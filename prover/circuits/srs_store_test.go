package circuits

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/kzg"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/test/unsafekzg"
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
			assert.NoError(store.cacheLagrange(lagrange, 8, tc.curveID, "aztec"))

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
				errs <- store.cacheLagrange(lagrange, 8, ecc.BN254, "aztec")
			}()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			assert.NoError(err, "concurrent cacheLagrange calls must all succeed")
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
		assert.NoError(store.cacheLagrange(lagrange, 8, ecc.BN254, "aztec"))

		leftovers, err := filepath.Glob(filepath.Join(dir, "*.tmp*"))
		assert.NoError(err)
		assert.Empty(leftovers, "cacheLagrange must not leave temp files behind")
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
		assert.NoError(os.Mkdir(filepath.Join(dir, "kzg_srs_lagrange_8_bn254_aztec.memdump"), 0o700))
		assert.Error(store.cacheLagrange(lagrange, 8, ecc.BN254, "aztec"), "publish must fail when the final name is taken by a directory")

		leftovers, err := filepath.Glob(filepath.Join(dir, "*.tmp*"))
		assert.NoError(err)
		assert.Empty(leftovers, "a failed publish must not leave temp files behind")
	})
}

func TestSRSStore_GetSRS_PersistsDerivedLagrange(t *testing.T) {
	assert := require.New(t)
	dir := t.TempDir()

	cs, err := frontend.Compile(ecc.BN254.ScalarField(), scs.NewBuilder, &circuit{make([]frontend.Variable, 1)})
	assert.NoError(err)
	canonicalSize, lagrangeSize := plonk.SRSSize(cs)
	canonical, _, err := unsafekzg.NewSRS(cs)
	assert.NoError(err)

	// seed the store with ONLY the canonical dump: GetSRS must derive the
	// lagrange SRS and persist it
	dumpToFile(t, canonical, filepath.Join(dir, fmt.Sprintf("kzg_srs_canonical_%d_bn254_aleo.memdump", canonicalSize)))

	store, err := NewSRSStore(dir)
	assert.NoError(err)
	_, lagrangeSRS, err := store.GetSRS(context.TODO(), cs)
	assert.NoError(err)
	assert.NotNil(lagrangeSRS)

	cachedPath := filepath.Join(dir, fmt.Sprintf("kzg_srs_lagrange_%d_bn254_aleo.memdump", lagrangeSize))
	info, err := os.Stat(cachedPath)
	assert.NoError(err, "GetSRS must persist the derived lagrange SRS")
	assert.Equal(os.FileMode(0o644), info.Mode().Perm(), "the cached dump must be world-readable like the ceremony files")

	// a fresh store must serve the same request from the cached file, without
	// republishing it
	fresh, err := NewSRSStore(dir)
	assert.NoError(err)
	_, lagrangeSRS, err = fresh.GetSRS(context.TODO(), cs)
	assert.NoError(err)
	assert.NotNil(lagrangeSRS)
	after, err := os.Stat(cachedPath)
	assert.NoError(err)
	assert.Equal(info.ModTime(), after.ModTime(), "a cache hit must not rewrite the file")

	for _, bad := range []struct {
		name    string
		content []byte
	}{
		{"garbage", []byte("garbage")},
		// a valid but wrong-size dump: catches the pkG1Len length guard
		{"undersized", dumpBytes(t, mustToLagrange(t, canonical, lagrangeSize/2))},
	} {
		t.Run("re_derives_over_unloadable_cache/"+bad.name, func(t *testing.T) {
			assert := require.New(t)
			// an unloadable lagrange dump beside a valid canonical: GetSRS must
			// warn, re-derive, and overwrite the bad file in place
			subDir := t.TempDir()
			dumpToFile(t, canonical, filepath.Join(subDir, fmt.Sprintf("kzg_srs_canonical_%d_bn254_aleo.memdump", canonicalSize)))
			badPath := filepath.Join(subDir, fmt.Sprintf("kzg_srs_lagrange_%d_bn254_aleo.memdump", lagrangeSize))
			assert.NoError(os.WriteFile(badPath, bad.content, 0o600))

			broken, err := NewSRSStore(subDir)
			assert.NoError(err)
			_, lagrangeSRS, err := broken.GetSRS(context.TODO(), cs)
			assert.NoError(err, "an unloadable cached lagrange SRS must not fail GetSRS")
			assert.NotNil(lagrangeSRS)

			// the bad file was repaired in place with a loadable, correct-size dump
			reloaded := kzg.NewSRS(ecc.BN254)
			data, err := os.ReadFile(badPath)
			assert.NoError(err)
			assert.NoError(reloaded.ReadDump(bytes.NewReader(data)), "the repaired cache file must be loadable")
			assert.Equal(lagrangeSize, pkG1Len(reloaded), "the repaired cache file must have the right size")
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

		otherCanonical, _, err := unsafekzg.NewSRS(cs)
		assert.NoError(err)
		otherLagrange, err := toLagrange(otherCanonical, lagrangeSize)
		assert.NoError(err)
		lagrangePath := filepath.Join(subDir, fmt.Sprintf("kzg_srs_lagrange_%d_bn254_aleo.memdump", lagrangeSize))
		dumpToFile(t, otherLagrange, lagrangePath)

		store, err := NewSRSStore(subDir)
		assert.NoError(err)
		_, lagrangeSRS, err := store.GetSRS(context.TODO(), cs)
		assert.NoError(err)
		assert.NotNil(lagrangeSRS)

		// the foreign dump must have been re-derived over: its Vk now matches
		// this canonical
		reloaded := kzg.NewSRS(ecc.BN254)
		data, err := os.ReadFile(lagrangePath)
		assert.NoError(err)
		assert.NoError(reloaded.ReadDump(bytes.NewReader(data)))
		assertSameVk(t, canonical, reloaded)
	})

	t.Run("read_only_store_dir_is_best_effort", func(t *testing.T) {
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
		_, err = os.Stat(filepath.Join(roDir, fmt.Sprintf("kzg_srs_lagrange_%d_bn254_aleo.memdump", lagrangeSize)))
		assert.True(os.IsNotExist(err), "nothing must be published to a read-only directory")
	})
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
