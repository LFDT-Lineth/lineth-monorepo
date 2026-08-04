package circuits

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/kzg"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/test/unsafekzg"
	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/require"

	kzg377 "github.com/consensys/gnark-crypto/ecc/bls12-377/kzg"
	kzg254 "github.com/consensys/gnark-crypto/ecc/bn254/kzg"
	kzgbw6 "github.com/consensys/gnark-crypto/ecc/bw6-761/kzg"
)

func TestSRSStore(t *testing.T) {
	assert := require.New(t)

	srsStore, err := NewSRSStore("../prover-assets/kzgsrs")
	assert.NoError(err)

	assert.True(len(srsStore.entries) > 0)

	// log the entries
	for curveID, entries := range srsStore.entries {
		t.Logf("curveID %s\n", curveID)
		for _, entry := range entries {
			t.Logf("entry %v\n", entry)
		}
	}
}

// assertSameVk fails the test unless both SRSs carry the same verifying key.
func assertSameVk(t *testing.T, want, got kzg.SRS) {
	t.Helper()
	switch want := want.(type) {
	case *kzg254.SRS:
		require.Equal(t, want.Vk, got.(*kzg254.SRS).Vk, "verifying key must survive derivation and persistence")
	case *kzg377.SRS:
		require.Equal(t, want.Vk, got.(*kzg377.SRS).Vk, "verifying key must survive derivation and persistence")
	case *kzgbw6.SRS:
		require.Equal(t, want.Vk, got.(*kzgbw6.SRS).Vk, "verifying key must survive derivation and persistence")
	default:
		t.Fatalf("unsupported SRS type %T", want)
	}
}

// newTestCanonicalSRS returns a small test-only canonical SRS for curveID.
func newTestCanonicalSRS(t *testing.T, curveID ecc.ID, size uint64) kzg.SRS {
	t.Helper()
	var (
		srs kzg.SRS
		err error
	)
	switch curveID {
	case ecc.BN254:
		srs, err = kzg254.NewSRS(size, big.NewInt(42))
	case ecc.BLS12_377:
		srs, err = kzg377.NewSRS(size, big.NewInt(42))
	case ecc.BW6_761:
		srs, err = kzgbw6.NewSRS(size, big.NewInt(42))
	default:
		t.Fatalf("unsupported curve %s", curveID)
	}
	require.NoError(t, err)
	return srs
}

func TestSRSStore_PersistsDerivedLagrange(t *testing.T) {
	testCases := []struct {
		curveID   ecc.ID
		curveName string
	}{
		{ecc.BN254, "bn254"},
		{ecc.BLS12_377, "bls12377"},
		{ecc.BW6_761, "bw6761"},
	}

	for _, tc := range testCases {
		t.Run(tc.curveName, func(t *testing.T) {
			assert := require.New(t)
			dir := t.TempDir()

			// a small test-only canonical SRS, written with the store's naming scheme
			canonical := newTestCanonicalSRS(t, tc.curveID, 16)
			dumpToFile(t, canonical, filepath.Join(dir, fmt.Sprintf("kzg_srs_canonical_16_%s_aztec.memdump", tc.curveName)))

			store, err := NewSRSStore(dir)
			assert.NoError(err)

			lagrange, err := toLagrange(canonical, 8)
			assert.NoError(err)
			assert.NoError(store.persistLagrange(lagrange, 8, tc.curveID))

			// a fresh store must pick the cached file up and be able to load it
			fresh, err := NewSRSStore(dir)
			assert.NoError(err)
			found := false
			for _, entry := range fresh.entriesSnapshot(tc.curveID) {
				if !entry.isCanonical && entry.size == 8 {
					found = true
					reloaded := kzg.NewSRS(tc.curveID)
					data, err := os.ReadFile(entry.path)
					assert.NoError(err)
					assert.NoError(reloaded.ReadDump(bytes.NewReader(data)), "cached lagrange SRS must be loadable")
					assertSameVk(t, canonical, reloaded)
				}
			}
			assert.True(found, "derived lagrange SRS was not persisted under a loadable name")
		})
	}

	t.Run("concurrent_calls_register_once", func(t *testing.T) {
		assert := require.New(t)
		dir := t.TempDir()
		canonical := newTestCanonicalSRS(t, ecc.BN254, 16)
		dumpToFile(t, canonical, filepath.Join(dir, "kzg_srs_canonical_16_bn254_aztec.memdump"))
		store, err := NewSRSStore(dir)
		assert.NoError(err)
		lagrange, err := toLagrange(canonical, 8)
		assert.NoError(err)

		// concurrent snapshot readers and cache writers must be race-free
		// (run with -race) and must register exactly one entry
		errs := make(chan error, 8)
		var wg sync.WaitGroup
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = store.entriesSnapshot(ecc.BN254)
				errs <- store.persistLagrange(lagrange, 8, ecc.BN254)
			}()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			assert.NoError(err, "concurrent persistLagrange calls must all succeed")
		}
		count := 0
		for _, entry := range store.entriesSnapshot(ecc.BN254) {
			if !entry.isCanonical && entry.size == 8 {
				count++
			}
		}
		assert.Equal(1, count, "concurrent caching must register exactly one entry")
	})

	t.Run("leaves_no_temp_files", func(t *testing.T) {
		assert := require.New(t)
		dir := t.TempDir()
		canonical := newTestCanonicalSRS(t, ecc.BN254, 16)
		dumpToFile(t, canonical, filepath.Join(dir, "kzg_srs_canonical_16_bn254_aztec.memdump"))
		store, err := NewSRSStore(dir)
		assert.NoError(err)
		lagrange, err := toLagrange(canonical, 8)
		assert.NoError(err)
		assert.NoError(store.persistLagrange(lagrange, 8, ecc.BN254))

		leftovers, err := filepath.Glob(filepath.Join(dir, "*.tmp*"))
		assert.NoError(err)
		assert.Empty(leftovers, "persistLagrange must not leave temp files behind")
	})

	t.Run("failed_publish_cleans_up_its_temp", func(t *testing.T) {
		assert := require.New(t)
		dir := t.TempDir()
		canonical := newTestCanonicalSRS(t, ecc.BN254, 16)
		dumpToFile(t, canonical, filepath.Join(dir, "kzg_srs_canonical_16_bn254_aztec.memdump"))
		store, err := NewSRSStore(dir)
		assert.NoError(err)
		lagrange, err := toLagrange(canonical, 8)
		assert.NoError(err)

		// a directory squatting on the final name makes the rename fail
		assert.NoError(os.Mkdir(filepath.Join(dir, "kzg_srs_lagrange_8_bn254_derived.memdump"), 0o700))
		assert.Error(store.persistLagrange(lagrange, 8, ecc.BN254), "publish must fail when the final name is taken by a directory")

		leftovers, err := filepath.Glob(filepath.Join(dir, "*.tmp*"))
		assert.NoError(err)
		assert.Empty(leftovers, "a failed publish must not leave temp files behind")
	})

	t.Run("sweeps_only_aged_orphan_temps", func(t *testing.T) {
		assert := require.New(t)
		dir := t.TempDir()

		cs, err := frontend.Compile(ecc.BN254.ScalarField(), scs.NewBuilder, &circuit{make([]frontend.Variable, 1)})
		assert.NoError(err)
		canonicalSize, _ := plonk.SRSSize(cs)
		canonical, _, err := unsafekzg.NewSRS(cs)
		assert.NoError(err)
		dumpToFile(t, canonical, filepath.Join(dir, fmt.Sprintf("kzg_srs_canonical_%d_bn254_aztec.memdump", canonicalSize)))

		// a crash-orphaned temp (old), a crashed probe's leftover (old) and a
		// concurrent writer's temp (fresh)
		aged := filepath.Join(dir, "kzg_srs_lagrange_8_bn254_aztec.memdump.tmp111")
		agedProbe := filepath.Join(dir, "kzg_srs_probe.memdump.tmp123")
		fresh := filepath.Join(dir, "kzg_srs_lagrange_8_bn254_aztec.memdump.tmp222")
		assert.NoError(os.WriteFile(aged, []byte("dead"), 0o600))
		assert.NoError(os.WriteFile(agedProbe, []byte("dead"), 0o600))
		assert.NoError(os.WriteFile(fresh, []byte("live"), 0o600))
		assert.NoError(os.Chtimes(aged, time.Now().Add(-2*time.Hour), time.Now().Add(-2*time.Hour)))
		assert.NoError(os.Chtimes(agedProbe, time.Now().Add(-2*time.Hour), time.Now().Add(-2*time.Hour)))

		store, err := NewSRSStore(dir)
		assert.NoError(err)
		_, err = os.Stat(aged)
		assert.NoError(err, "constructing the store must not mutate the directory")

		// provisioning sweeps the aged orphan and spares the live writer's temp
		assert.NoError(store.DeriveAndPersistLagrange(context.TODO(), cs, false))
		_, err = os.Stat(aged)
		assert.True(os.IsNotExist(err), "an aged orphan temp must be swept")
		_, err = os.Stat(agedProbe)
		assert.True(os.IsNotExist(err), "a crashed probe's leftover must be swept")
		_, err = os.Stat(fresh)
		assert.NoError(err, "a fresh temp must be spared")
		for _, entry := range store.entriesSnapshot(ecc.BN254) {
			assert.NotContains(entry.path, ".memdump.tmp", "temp files must never be indexed")
		}
	})
}

func TestSRSStore_DeriveAndPersistLagrange(t *testing.T) {
	assert := require.New(t)
	dir := t.TempDir()

	cs, err := frontend.Compile(ecc.BN254.ScalarField(), scs.NewBuilder, &circuit{make([]frontend.Variable, 1)})
	assert.NoError(err)
	canonicalSize, lagrangeSize := plonk.SRSSize(cs)
	canonical, _, err := unsafekzg.NewSRS(cs)
	assert.NoError(err)

	// seed the store with ONLY the canonical dump
	dumpToFile(t, canonical, filepath.Join(dir, fmt.Sprintf("kzg_srs_canonical_%d_bn254_aleo.memdump", canonicalSize)))
	cachedPath := filepath.Join(dir, fmt.Sprintf("kzg_srs_lagrange_%d_bn254_derived.memdump", lagrangeSize))

	// reading must not write: GetSRS derives in memory and leaves no trace
	reader, err := NewSRSStore(dir)
	assert.NoError(err)
	_, lagrangeSRS, err := reader.GetSRS(context.TODO(), cs)
	assert.NoError(err)
	assert.NotNil(lagrangeSRS, "GetSRS must still derive the basis in memory")
	_, err = os.Stat(cachedPath)
	assert.True(os.IsNotExist(err), "GetSRS must not write into the SRS directory")
	assert.Equal([]string{fmt.Sprintf("kzg_srs_canonical_%d_bn254_aleo.memdump", canonicalSize)}, dirNames(t, dir),
		"a read must leave the directory exactly as it was")

	// provisioning writes, under the derived tag and world-readable
	store, err := NewSRSStore(dir)
	assert.NoError(err)
	assert.NoError(store.DeriveAndPersistLagrange(context.TODO(), cs, false))
	info, err := os.Stat(cachedPath)
	assert.NoError(err, "DeriveAndPersistLagrange must publish the derived lagrange SRS")
	_, err = os.Stat(filepath.Join(dir, fmt.Sprintf("kzg_srs_lagrange_%d_bn254_aleo.memdump", lagrangeSize)))
	assert.True(os.IsNotExist(err), "a derived basis must not be published under a ceremony tag")
	assert.Equal(os.FileMode(0o644), info.Mode().Perm(), "the cached dump must be world-readable like the ceremony files")

	// a fresh store serves the request from the cached file without rewriting it,
	// and a second provisioning call is a no-op
	fresh, err := NewSRSStore(dir)
	assert.NoError(err)
	_, lagrangeSRS, err = fresh.GetSRS(context.TODO(), cs)
	assert.NoError(err)
	assert.NotNil(lagrangeSRS)
	assert.NoError(fresh.DeriveAndPersistLagrange(context.TODO(), cs, false))
	after, err := os.Stat(cachedPath)
	assert.NoError(err)
	assert.Equal(info.ModTime(), after.ModTime(), "a cache hit must not rewrite the file")

	for _, bad := range []struct {
		name    string
		content []byte
	}{
		{"garbage", []byte("garbage")},
		// one flipped bit in the point region loads cleanly and passes the
		// length check, but leaves an off-curve point — must be re-derived
		{"bitflipped", bitflip(dumpBytes(t, mustToLagrange(t, canonical, lagrangeSize)))},
		// a valid but wrong-size dump: catches the pkG1Len length guard
		{"undersized", dumpBytes(t, mustToLagrange(t, canonical, lagrangeSize/2))},
	} {
		t.Run("re_derives_beside_unloadable_cache/"+bad.name, func(t *testing.T) {
			assert := require.New(t)
			// an unloadable lagrange dump beside a valid canonical: GetSRS must
			// warn, re-derive, and publish under the derived tag while leaving
			// the ceremony-tagged file it did not write completely alone
			subDir := t.TempDir()
			dumpToFile(t, canonical, filepath.Join(subDir, fmt.Sprintf("kzg_srs_canonical_%d_bn254_aleo.memdump", canonicalSize)))
			badPath := filepath.Join(subDir, fmt.Sprintf("kzg_srs_lagrange_%d_bn254_aleo.memdump", lagrangeSize))
			assert.NoError(os.WriteFile(badPath, bad.content, 0o600))

			broken, err := NewSRSStore(subDir)
			assert.NoError(err)
			_, lagrangeSRS, err := broken.GetSRS(context.TODO(), cs)
			assert.NoError(err, "an unloadable cached lagrange SRS must not fail GetSRS")
			assert.NotNil(lagrangeSRS)

			// without force, a right-size dump in the index counts as done — the
			// cheap path must not load it, so it cannot know it is bad
			derivedPath := filepath.Join(subDir, fmt.Sprintf("kzg_srs_lagrange_%d_bn254_derived.memdump", lagrangeSize))
			assert.NoError(broken.DeriveAndPersistLagrange(context.TODO(), cs, false))
			_, err = os.Stat(derivedPath)
			assert.True(os.IsNotExist(err), "without force an indexed dump must be trusted, not repaired")

			// force validates in full and repairs beside the bad file
			assert.NoError(broken.DeriveAndPersistLagrange(context.TODO(), cs, true))

			// the operator's file is untouched, byte for byte
			stillBad, err := os.ReadFile(badPath)
			assert.NoError(err, "the rejected dump must not be deleted")
			assert.Equal(bad.content, stillBad, "the rejected dump must not be overwritten")

			// and a loadable, correct-size, same-setup dump exists beside it
			reloaded := kzg.NewSRS(ecc.BN254)
			data, err := os.ReadFile(derivedPath)
			assert.NoError(err, "a derived dump must have been published")
			assert.NoError(reloaded.ReadDump(bytes.NewReader(data)), "the derived dump must be loadable")
			assert.Equal(lagrangeSize, pkG1Len(reloaded), "the derived dump must have the right size")
			assertSameVk(t, canonical, reloaded)
		})
	}

	t.Run("rejects_lagrange_from_a_different_setup", func(t *testing.T) {
		assert := require.New(t)
		// a same-size lagrange dump derived from a DIFFERENT canonical SRS (so a
		// different verifying key) must be rejected and re-derived, even though
		// its filename and point count both match
		subDir := t.TempDir()
		dumpToFile(t, canonical, filepath.Join(subDir, fmt.Sprintf("kzg_srs_canonical_%d_bn254_aleo.memdump", canonicalSize)))

		// unsafekzg.NewSRS memoises on (curve, size, toxic value), so calling it
		// again with no toxic value returns the very same SRS and the dump below
		// would carry a matching Vk: an explicit toxic value is what makes this
		// a different setup at all
		otherCanonical, _, err := unsafekzg.NewSRS(cs, unsafekzg.WithToxicValue(big.NewInt(7919)))
		assert.NoError(err)
		otherLagrange, err := toLagrange(otherCanonical, lagrangeSize)
		assert.NoError(err)
		lagrangePath := filepath.Join(subDir, fmt.Sprintf("kzg_srs_lagrange_%d_bn254_aleo.memdump", lagrangeSize))
		dumpToFile(t, otherLagrange, lagrangePath)
		foreignBefore, err := os.ReadFile(lagrangePath)
		assert.NoError(err)

		store, err := NewSRSStore(subDir)
		assert.NoError(err)
		_, lagrangeSRS, err := store.GetSRS(context.TODO(), cs)
		assert.NoError(err)
		assert.NotNil(lagrangeSRS)
		// force: only the full-validation path can tell a foreign dump apart
		assert.NoError(store.DeriveAndPersistLagrange(context.TODO(), cs, true))

		// the foreign dump is left alone; a matching basis is published beside it
		foreignAfter, err := os.ReadFile(lagrangePath)
		assert.NoError(err)
		assert.Equal(foreignBefore, foreignAfter, "a dump from another setup must not be overwritten")

		reloaded := kzg.NewSRS(ecc.BN254)
		data, err := os.ReadFile(filepath.Join(subDir, fmt.Sprintf("kzg_srs_lagrange_%d_bn254_derived.memdump", lagrangeSize)))
		assert.NoError(err, "a derived dump matching this canonical must have been published")
		assert.NoError(reloaded.ReadDump(bytes.NewReader(data)))
		assertSameVk(t, canonical, reloaded)
	})

	t.Run("read_only_dir_reads_fine_but_provisioning_reports", func(t *testing.T) {
		assert := require.New(t)
		if os.Geteuid() == 0 {
			t.Skip("directory permissions are not enforced for root")
		}
		roDir := t.TempDir()
		dumpToFile(t, canonical, filepath.Join(roDir, fmt.Sprintf("kzg_srs_canonical_%d_bn254_aleo.memdump", canonicalSize)))
		roStore, err := NewSRSStore(roDir)
		assert.NoError(err)
		assert.NoError(os.Chmod(roDir, 0o500))
		t.Cleanup(func() { _ = os.Chmod(roDir, 0o700) })

		_, lagrangeSRS, err := roStore.GetSRS(context.TODO(), cs)
		assert.NoError(err, "a read-only store directory must not fail GetSRS")
		assert.NotNil(lagrangeSRS)

		// provisioning returns the failure rather than swallowing it, so an
		// operator running it deliberately finds out that nothing was written —
		// and the wording pins that the probe reported it before deriving,
		// not persistLagrange after
		assert.ErrorContains(roStore.DeriveAndPersistLagrange(context.TODO(), cs, false),
			"not deriving a basis", "provisioning must report that it could not write, from the probe")
		_, err = os.Stat(filepath.Join(roDir, fmt.Sprintf("kzg_srs_lagrange_%d_bn254_derived.memdump", lagrangeSize)))
		assert.True(os.IsNotExist(err), "nothing must be published to a read-only directory")
	})
}

// TestSRSStore_DerivedDumpsNeverClaimACeremony pins the provenance rule: a
// locally computed basis is published under derivedSourceTag whatever ceremony
// the canonical SRS it came from was tagged with, and is loadable again from
// that name. The mirror also holds: a canonical dump claiming the derived tag
// is ignored, never indexed as ceremony material.
func TestSRSStore_DerivedDumpsNeverClaimACeremony(t *testing.T) {
	for _, ceremony := range []string{"aleo", "aztec", "celo"} {
		t.Run("canonical_"+ceremony, func(t *testing.T) {
			assert := require.New(t)
			dir := t.TempDir()

			canonical := newTestCanonicalSRS(t, ecc.BN254, 16)
			dumpToFile(t, canonical, filepath.Join(dir, fmt.Sprintf("kzg_srs_canonical_16_bn254_%s.memdump", ceremony)))

			store, err := NewSRSStore(dir)
			assert.NoError(err)
			lagrange, err := toLagrange(canonical, 8)
			assert.NoError(err)
			assert.NoError(store.persistLagrange(lagrange, 8, ecc.BN254))

			assert.FileExists(filepath.Join(dir, "kzg_srs_lagrange_8_bn254_derived.memdump"))
			_, err = os.Stat(filepath.Join(dir, fmt.Sprintf("kzg_srs_lagrange_8_bn254_%s.memdump", ceremony)))
			assert.True(os.IsNotExist(err), "a derived basis must never inherit the canonical file's ceremony tag")

			// the derived tag must round-trip through the store's own parser
			fresh, err := NewSRSStore(dir)
			assert.NoError(err)
			indexed := false
			for _, entry := range fresh.entriesSnapshot(ecc.BN254) {
				if !entry.isCanonical && entry.size == 8 {
					indexed = true
					assert.Equal(derivedSourceTag, entry.source)
				}
			}
			assert.True(indexed, "a derived dump must be indexed on the next construction")
		})
	}

	t.Run("canonical_never_carries_derived", func(t *testing.T) {
		assert := require.New(t)
		dir := t.TempDir()

		canonical := newTestCanonicalSRS(t, ecc.BN254, 16)
		dumpToFile(t, canonical, filepath.Join(dir, "kzg_srs_canonical_16_bn254_aztec.memdump"))
		// a name nothing writes: a canonical claiming to be locally computed
		// must be ignored, not indexed as trusted ceremony material
		dumpToFile(t, canonical, filepath.Join(dir, "kzg_srs_canonical_16_bn254_derived.memdump"))

		store, err := NewSRSStore(dir)
		assert.NoError(err)
		entries := store.entriesSnapshot(ecc.BN254)
		assert.Len(entries, 1, "a derived-tagged canonical must not be indexed")
		assert.Contains(entries[0].path, "aztec")
	})
}

// TestSRSStore_DerivationWarning pins the miss warning's gate and content: a
// dummy-size derivation stays quiet, a real-size prove-time miss warns once
// and names the remedy, provisioning is not told to run the command it
// already is, and once the dump is persisted the same read is silent.
// It lowers lagrangeSizeWarnThreshold and captures the global logger, so it
// must not run in parallel.
func TestSRSStore_DerivationWarning(t *testing.T) {
	assert := require.New(t)
	dir := t.TempDir()

	cs, err := frontend.Compile(ecc.BN254.ScalarField(), scs.NewBuilder, &circuit{make([]frontend.Variable, 1)})
	assert.NoError(err)
	canonicalSize, _ := plonk.SRSSize(cs)
	canonical, _, err := unsafekzg.NewSRS(cs)
	assert.NoError(err)
	dumpToFile(t, canonical, filepath.Join(dir, fmt.Sprintf("kzg_srs_canonical_%d_bn254_aleo.memdump", canonicalSize)))

	store, err := NewSRSStore(dir)
	assert.NoError(err)
	hook := logtest.NewGlobal()
	defer hook.Reset()

	// below the threshold a derivation is routine: no operator-facing warning
	_, lagrangeSRS, err := store.GetSRS(context.TODO(), cs)
	assert.NoError(err)
	assert.NotNil(lagrangeSRS)
	for _, e := range hook.AllEntries() {
		assert.Greater(e.Level, logrus.WarnLevel, "a dummy-size derivation must not warn: %s", e.Message)
	}

	oldThreshold := lagrangeSizeWarnThreshold
	lagrangeSizeWarnThreshold = 1
	t.Cleanup(func() { lagrangeSizeWarnThreshold = oldThreshold })

	// at a real size a prove-time miss is loud, once, and names the remedy
	hook.Reset()
	_, _, err = store.GetSRS(context.TODO(), cs)
	assert.NoError(err)
	warnings := 0
	for _, e := range hook.AllEntries() {
		if e.Level == logrus.WarnLevel {
			warnings++
			assert.Contains(e.Message, "prover setup", "the warning must name the command that stops the recurrence")
			assert.Contains(e.Message, "persist_derived_srs", "the warning must name the flag that gates the persist")
		}
	}
	assert.Equal(1, warnings, "a real-size prove-time miss must warn exactly once")

	// provisioning announces the wait but is not told to run itself
	hook.Reset()
	assert.NoError(store.DeriveAndPersistLagrange(context.TODO(), cs, false))
	warned := false
	for _, e := range hook.AllEntries() {
		if e.Level == logrus.WarnLevel {
			warned = true
			assert.NotContains(e.Message, "prover setup", "provisioning must not be told to run the command it already is")
		}
	}
	assert.True(warned, "a real-size provisioning derivation must still announce the wait")

	// the named remedy works: with the dump persisted, the same read is silent
	hook.Reset()
	_, lagrangeSRS, err = store.GetSRS(context.TODO(), cs)
	assert.NoError(err)
	assert.NotNil(lagrangeSRS)
	for _, e := range hook.AllEntries() {
		assert.Greater(e.Level, logrus.WarnLevel, "after setup persisted the dump a read must be quiet: %s", e.Message)
	}
}

// TestSRSStore_CanonicalLoadFailuresStayFatal pins the boundary of the
// lenient-load behaviour: a lagrange dump that fails to load is re-derived,
// but ceremony (canonical) material is not reconstructible, so a corrupt or
// missing canonical dump must fail GetSRS, never fall back.
func TestSRSStore_CanonicalLoadFailuresStayFatal(t *testing.T) {
	assert := require.New(t)

	cs, err := frontend.Compile(ecc.BN254.ScalarField(), scs.NewBuilder, &circuit{make([]frontend.Variable, 1)})
	assert.NoError(err)
	canonicalSize, _ := plonk.SRSSize(cs)

	t.Run("corrupt", func(t *testing.T) {
		assert := require.New(t)
		dir := t.TempDir()
		assert.NoError(os.WriteFile(
			filepath.Join(dir, fmt.Sprintf("kzg_srs_canonical_%d_bn254_aleo.memdump", canonicalSize)),
			[]byte("garbage"), 0o644))
		store, err := NewSRSStore(dir)
		assert.NoError(err)
		_, _, err = store.GetSRS(context.TODO(), cs)
		assert.Error(err, "a corrupt canonical dump must fail GetSRS, not fall back to deriving")
	})

	t.Run("missing", func(t *testing.T) {
		assert := require.New(t)
		store, err := NewSRSStore(t.TempDir())
		assert.NoError(err)
		_, _, err = store.GetSRS(context.TODO(), cs)
		assert.ErrorContains(err, "could not find canonical SRS")
	})
}

// dirNames lists the entry names in dir, sorted.
func dirNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}

// mustToLagrange derives a lagrange SRS or fails the test.
func mustToLagrange(t *testing.T, canonical kzg.SRS, size int) kzg.SRS {
	t.Helper()
	l, err := toLagrange(canonical, size)
	require.NoError(t, err)
	return l
}

// dumpBytes serializes an SRS to its on-disk dump form.
func dumpBytes(t *testing.T, srs kzg.SRS) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, srs.WriteDump(&buf))
	return buf.Bytes()
}

// bitflip flips one bit inside the point region of a serialized dump.
func bitflip(dump []byte) []byte {
	out := append([]byte(nil), dump...)
	out[len(out)-100] ^= 0x01
	return out
}
