package circuits

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/kzg"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/parallel"
	"github.com/sirupsen/logrus"

	kzg377 "github.com/consensys/gnark-crypto/ecc/bls12-377/kzg"
	kzg254 "github.com/consensys/gnark-crypto/ecc/bn254/kzg"
	kzgbw6 "github.com/consensys/gnark-crypto/ecc/bw6-761/kzg"
)

type SRSStore struct {
	// mu guards entries; a file is safe to read unlocked because it is only
	// registered after being atomically published, and a re-publish renames a
	// new file over the path — rename(2) swaps the directory entry, so an
	// in-flight read keeps its old inode and never sees a mix.
	mu      sync.RWMutex
	entries map[ecc.ID][]fsEntry
	rootDir string
}

type fsEntry struct {
	isCanonical bool
	size        int
	path        string
	source      string // the provenance tag in the file name: aleo, aztec, celo or derived
}

// derivedSourceTag is the provenance tag for a Lagrange basis this process
// computed locally, as opposed to one distributed from a ceremony.
//
// A derived basis is only as trustworthy as the canonical SRS it was computed
// from, so it must not inherit that file's ceremony tag. Whoever later lists
// the store, restores a backup, or bakes the directory into an image has to be
// able to tell computed material from attested material, and a file name is
// the only signal they get. Nothing in the store validates that a file tagged
// "aztec" descends from that ceremony, so a tag written by code is a
// provenance claim the code cannot support: never write one.
//
// The tag is only meaningful on a lagrange basis: the store never computes
// canonical material, so a canonical name claiming it is ignored.
const derivedSourceTag = "derived"

// orphanTempMaxAge is how long a temp file's last write must lie in the past
// before provisioning deletes it; a live writer keeps refreshing its temp's
// mtime while streaming the dump.
const orphanTempMaxAge = time.Hour

// curveFileNames maps a curve ID to the token naming it in SRS file names.
var curveFileNames = map[ecc.ID]string{
	ecc.BLS12_377: "bls12377",
	ecc.BN254:     "bn254",
	ecc.BW6_761:   "bw6761",
}

// curveIDsByFileName is the inverse of curveFileNames.
var curveIDsByFileName = func() map[string]ecc.ID {
	m := make(map[string]ecc.ID, len(curveFileNames))
	for id, name := range curveFileNames {
		m[name] = id
	}
	return m
}()

// NewSRSStore creates a new SRSStore
func NewSRSStore(rootDir string) (*SRSStore, error) {
	// list all the files in rootDir
	// for each file, make a fsEntry but do not load the SRS (lazy loaded on demand)
	// store the fsEntry in map[string]fsEntry, with the key being the file name

	dir, err := os.ReadDir(rootDir)
	if err != nil {
		return nil, err
	}

	srsStore := &SRSStore{
		entries: make(map[ecc.ID][]fsEntry),
		rootDir: rootDir,
	}
	srsStore.entries[ecc.BLS12_377] = []fsEntry{}
	srsStore.entries[ecc.BN254] = []fsEntry{}
	srsStore.entries[ecc.BW6_761] = []fsEntry{}

	// the trailing group is the provenance tag: one of the three ceremonies the
	// store may be seeded from, or derivedSourceTag for a locally computed basis
	srsRegexp := regexp.MustCompile(`^(kzg_srs)_(canonical|lagrange)_(\d+)_(bls12377|bn254|bw6761)_(aleo|aztec|celo|` + derivedSourceTag + `)\.memdump$`)

	for _, entry := range dir {
		if entry.IsDir() {
			continue
		}
		// parse the file name
		// create a fsEntry
		// store it in the map

		fileName := entry.Name()
		matches := srsRegexp.FindStringSubmatch(fileName)
		if matches == nil {
			continue
		}

		isCanonical := matches[2] == "canonical"
		size, _ := strconv.Atoi(matches[3])
		source := matches[5]
		// only a lagrange basis can be locally computed: a canonical dump
		// carrying the derived tag is not a name anything writes, so treat
		// it as noise rather than trusted, fatal-if-bad ceremony material
		if isCanonical && source == derivedSourceTag {
			logrus.Warnf("ignoring %s: a canonical SRS cannot carry the %q tag", fileName, derivedSourceTag)
			continue
		}
		curveID, ok := curveIDsByFileName[matches[4]]
		if !ok {
			return nil, errors.New("curve not supported")
		}

		srsStore.entries[curveID] = append(srsStore.entries[curveID], fsEntry{
			isCanonical: isCanonical,
			size:        size,
			path:        filepath.Join(rootDir, fileName),
			source:      source,
		})

	}

	// sort the entries by size
	for _, entries := range srsStore.entries {
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].size < entries[j].size
		})
	}

	return srsStore, nil
}

// GetSRS returns the canonical and Lagrange SRS for the circuit.
//
// It only reads. When no loadable Lagrange dump is on disk the basis is derived
// in memory and discarded with the process, exactly as it was before the store
// learned to cache. Populating the store is DeriveAndPersistLagrange's job, so
// that a prove-time read can never mutate the SRS directory: an operator can
// mount it read-only and still be sure the fast path is available, because
// whether the dump exists was decided at provisioning time, not by whichever
// process happened to ask first. A miss at a real circuit size is loud:
// hitting the derivation at prove time recurs on every start, so the warning
// names the command and flag that make it stop.
func (store *SRSStore) GetSRS(ctx context.Context, ccs constraint.ConstraintSystem) (kzg.SRS, kzg.SRS, error) {
	canonicalSRS, lagrangeSRS, _, err := store.resolveSRS(ccs, false)
	return canonicalSRS, lagrangeSRS, err
}

// lagrangeSizeWarnThreshold separates real circuit sizes from the tiny dummy
// circuits that pass through GetSRS during setup: below it a derivation costs
// milliseconds and is not worth an operator-facing warning. A var rather than
// a const only so tests can lower it to reach the warning path without a
// million-point derivation; production code never writes it.
var lagrangeSizeWarnThreshold = 1 << 20

// DeriveAndPersistLagrange makes the Lagrange basis for ccs available on disk,
// so later runs load it instead of spending hours re-deriving it. It is the
// only path in the store that writes.
//
// Without force, it is as cheap as what there is to do: a dump of the right
// size already in the index counts as done without loading it (whether it
// loads is re-checked wherever it is read; repairing one that does not is
// force's job), and an unwritable directory is detected by a probe before the
// expensive derivation rather than after it.
//
// Best-effort is the caller's choice here rather than the store's: the error is
// returned so an explicit provisioning step can report it, where a prove-time
// read had to swallow it.
func (store *SRSStore) DeriveAndPersistLagrange(ctx context.Context, ccs constraint.ConstraintSystem, force bool) error {
	// reclaim crash leftovers before the cheap-skip: after a crashed --force
	// re-write a valid dump still exists, so no later step would ever run
	store.sweepOrphanTemps()

	_, sizeLagrange := plonk.SRSSize(ccs)
	curveID := fieldToCurve(ccs.Field())

	if !force {
		for _, entry := range store.entriesSnapshot(curveID) {
			if !entry.isCanonical && entry.size == sizeLagrange {
				// a dump of the right size is already on disk: done, without
				// paying a multi-GiB load just to conclude there is nothing to
				// write
				return nil
			}
		}
	}

	// probe writability before deriving: failing at the write would waste the
	// hours-long derivation this call exists to save. The probe name matches
	// the orphan-temp sweep pattern, so a crash leftover is reclaimed.
	probe, err := os.CreateTemp(store.rootDir, "kzg_srs_probe.memdump.tmp")
	if err != nil {
		return fmt.Errorf("SRS directory %s is not writable, not deriving a basis that could not be persisted: %w", store.rootDir, err)
	}
	probe.Close()
	os.Remove(probe.Name())

	_, lagrangeSRS, derived, err := store.resolveSRS(ccs, true)
	if err != nil {
		return err
	}
	if !derived {
		// a loadable dump is already on disk; nothing to publish
		return nil
	}
	return store.persistLagrange(lagrangeSRS, sizeLagrange, curveID)
}

// sweepOrphanTemps deletes temp files orphaned by a crash mid-write (e.g. an
// OOM-kill during a multi-GiB dump) once their last write is more than an
// hour old — a live writer's temp is always newer than that. Only the
// provisioning path calls it: constructing or reading the store never mutates
// the directory.
func (store *SRSStore) sweepOrphanTemps() {
	dir, err := os.ReadDir(store.rootDir)
	if err != nil {
		return
	}
	for _, entry := range dir {
		fileName := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(fileName, "kzg_srs_") || !strings.Contains(fileName, ".memdump.tmp") {
			continue
		}
		if info, err := entry.Info(); err == nil && time.Since(info.ModTime()) > orphanTempMaxAge {
			if err := os.Remove(filepath.Join(store.rootDir, fileName)); err != nil {
				logrus.Warnf("could not remove orphaned srs temp file %s: %v", fileName, err)
			} else {
				logrus.Infof("removed orphaned srs temp file %s", fileName)
			}
		}
	}
}

// resolveSRS loads the canonical SRS and either loads or derives the matching
// Lagrange basis, reporting whether it had to derive. It never writes.
// provisioning tells it who is asking, for the derivation warning's sake: a
// prove-time caller is told the setup command that makes the derivation stop
// recurring, a provisioning caller is already running it.
//
// Concurrent callers requesting the same missing size each derive independently
// (the store is race-safe but does not deduplicate the work); every in-repo
// caller is sequential today.
func (store *SRSStore) resolveSRS(ccs constraint.ConstraintSystem, provisioning bool) (kzg.SRS, kzg.SRS, bool, error) {
	sizeCanonical, sizeLagrange := plonk.SRSSize(ccs)
	curveID := fieldToCurve(ccs.Field())

	entries := store.entriesSnapshot(curveID)

	// find the canonical srs
	var canonicalSRS kzg.SRS
	for _, entry := range entries {
		if entry.isCanonical && entry.size >= sizeCanonical {
			canonicalSRS = kzg.NewSRS(curveID)
			data, err := os.ReadFile(entry.path)
			if err != nil {
				return nil, nil, false, err
			}
			if err := canonicalSRS.ReadDump(bytes.NewReader(data), sizeCanonical); err != nil {
				return nil, nil, false, err
			}
			break
		}
	}

	if canonicalSRS == nil {
		return nil, nil, false, fmt.Errorf("could not find canonical SRS for curve %s and size %d", curveID, sizeCanonical)
	}

	// find the lagrange srs
	var lagrangeSRS kzg.SRS
	for _, entry := range entries {
		if entry.isCanonical || entry.size != sizeLagrange {
			continue
		}
		srs := kzg.NewSRS(curveID)
		data, err := os.ReadFile(entry.path)
		if err == nil {
			// catches a truncated or unparseable dump
			err = srs.ReadDump(bytes.NewReader(data))
		}
		if err == nil && pkG1Len(srs) != sizeLagrange {
			// catches a wrong-size dump here, where it re-derives, rather than
			// letting plonk.Setup reject it later as an unrecoverable error
			err = fmt.Errorf("dump has %d points, want %d", pkG1Len(srs), sizeLagrange)
		}
		if err == nil {
			// catches a dump from a different setup: the KZG verifying key is
			// basis-independent, so a lagrange dump derived from this canonical
			// SRS must carry the same Vk
			err = utils.WriterstoEqual(srsVk(canonicalSRS), srsVk(srs))
		}
		if err == nil {
			// catches corrupted point data — ReadDump copies bytes without
			// validating them
			err = pkG1OnCurve(srs)
		}
		if err != nil {
			// a lagrange dump is reconstructible (unlike the canonical SRS): log
			// and fall back to deriving. The rejected file is left exactly as it
			// is: a derived basis is published under derivedSourceTag, so this
			// process never overwrites a file it did not write.
			logrus.Warnf("could not load lagrange SRS %s, re-deriving it: %v", entry.path, err)
			continue
		}
		lagrangeSRS = srs
		break
	}

	if lagrangeSRS == nil {
		// we can compute it from the canonical one.
		if sizeCanonical < sizeLagrange {
			panic("canonical SRS is smaller than lagrange SRS")
		}
		if sizeLagrange >= lagrangeSizeWarnThreshold {
			// warn, not debug: at real circuit sizes this branch costs hours, and
			// the operator deserves to know before the wait, not after — remedy
			// included, because a killed process never reaches a post-hoc hint
			if provisioning {
				logrus.Warnf("computing lagrange SRS from canonical SRS %d -> %d — this can take hours",
					sizeCanonical, sizeLagrange)
			} else {
				logrus.Warnf("computing lagrange SRS from canonical SRS %d -> %d in memory — this can take hours "+
					"and recurs on every start; run `prover setup` with persist_derived_srs on (the default) to "+
					"persist it, adding --force if a dump was reported unloadable above",
					sizeCanonical, sizeLagrange)
			}
		} else {
			logrus.Debugf("computing lagrange SRS from canonical SRS %d -> %d", sizeCanonical, sizeLagrange)
		}
		lagrangeSRS, err := toLagrange(canonicalSRS, sizeLagrange)
		if err != nil {
			return nil, nil, false, err
		}
		return canonicalSRS, lagrangeSRS, true, nil
	}

	return canonicalSRS, lagrangeSRS, false, nil
}

// entriesSnapshot returns a copy of the entries for curveID, safe to iterate unlocked.
func (store *SRSStore) entriesSnapshot(curveID ecc.ID) []fsEntry {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return append([]fsEntry(nil), store.entries[curveID]...)
}

// register adds a lagrange entry to the index unless an equivalent one exists.
func (store *SRSStore) register(curveID ecc.ID, newEntry fsEntry) {
	store.mu.Lock()
	defer store.mu.Unlock()
	for _, entry := range store.entries[curveID] {
		if !entry.isCanonical && entry.size == newEntry.size && entry.source == newEntry.source {
			return
		}
	}
	store.entries[curveID] = append(store.entries[curveID], newEntry)
	sort.Slice(store.entries[curveID], func(i, j int) bool {
		return store.entries[curveID][i].size < store.entries[curveID][j].size
	})
}

// persistLagrange writes a derived Lagrange SRS into the store's directory using
// the naming scheme NewSRSStore parses, and registers it in the index. The dump
// is fsync'd and renamed into place atomically, so a torn write cannot appear
// under a trusted name. On load, framing, size, setup-identity (Vk) and
// point-validity (on-curve) errors are all caught and re-derived; substitution
// of validly-encoded points is out of scope, as for every file in the store.
//
// The published name always carries derivedSourceTag, so the only file this can
// ever replace is one it wrote itself: a ceremony dump on the same path is
// impossible by construction, and a stale derived dump is safe to supersede.
func (store *SRSStore) persistLagrange(lagrangeSRS kzg.SRS, sizeLagrange int, curveID ecc.ID) error {
	curveName, ok := curveFileNames[curveID]
	if !ok {
		return fmt.Errorf("curve not supported: %s", curveID)
	}

	fileName := fmt.Sprintf("kzg_srs_lagrange_%d_%s_%s.memdump", sizeLagrange, curveName, derivedSourceTag)
	finalPath := filepath.Join(store.rootDir, fileName)

	f, err := os.CreateTemp(store.rootDir, fileName+".tmp")
	if err != nil {
		return err
	}
	// one cleanup for every failure path: until the rename publishes the dump,
	// returning is what removes the temp
	published := false
	defer func() {
		if !published {
			f.Close()
			os.Remove(f.Name())
		}
	}()

	if err := lagrangeSRS.WriteDump(f); err != nil {
		return fmt.Errorf("writing srs dump: %w", err)
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	// ceremony files in the store are world-readable; match them so the cache
	// stays loadable when a later run uses a different uid
	if err := os.Chmod(f.Name(), 0o644); err != nil {
		return err
	}
	if err := os.Rename(f.Name(), finalPath); err != nil {
		return err
	}
	published = true
	syncDir(store.rootDir)

	store.register(curveID, fsEntry{isCanonical: false, size: sizeLagrange, path: finalPath, source: derivedSourceTag})

	logrus.Infof("persisted derived lagrange SRS to %s", finalPath)
	return nil
}

// pkG1OnCurve fails if any proving-key point is off-curve. ReadDump is a raw
// copy with no point validation, so this is what actually catches a corrupted
// dump: a random bit-flip virtually never lands on the curve. Runs over points
// already in memory, in parallel — seconds, against hours of re-derivation.
func pkG1OnCurve(srs kzg.SRS) error {
	var bad atomic.Int64
	bad.Store(-1)
	check := func(isOnCurve func(i int) bool, n int) {
		parallel.Execute(n, func(start, stop int) {
			for i := start; i < stop; i++ {
				if !isOnCurve(i) {
					bad.Store(int64(i))
					return
				}
			}
		})
	}
	switch s := srs.(type) {
	case *kzg254.SRS:
		check(func(i int) bool { return s.Pk.G1[i].IsOnCurve() }, len(s.Pk.G1))
	case *kzg377.SRS:
		check(func(i int) bool { return s.Pk.G1[i].IsOnCurve() }, len(s.Pk.G1))
	case *kzgbw6.SRS:
		check(func(i int) bool { return s.Pk.G1[i].IsOnCurve() }, len(s.Pk.G1))
	default:
		return fmt.Errorf("unsupported SRS type %T", srs)
	}
	if i := bad.Load(); i >= 0 {
		return fmt.Errorf("proving-key point %d is not on the curve", i)
	}
	return nil
}

// srsVk exposes the SRS verifying key for equality checks. The Vk is
// basis-independent, so a canonical SRS and a lagrange dump derived from it
// carry the same one.
func srsVk(srs kzg.SRS) io.WriterTo {
	switch s := srs.(type) {
	case *kzg254.SRS:
		return &s.Vk
	case *kzg377.SRS:
		return &s.Vk
	case *kzgbw6.SRS:
		return &s.Vk
	default:
		return nil
	}
}

// pkG1Len returns the number of G1 proving-key points in the SRS, or -1.
func pkG1Len(srs kzg.SRS) int {
	switch srs := srs.(type) {
	case *kzg254.SRS:
		return len(srs.Pk.G1)
	case *kzg377.SRS:
		return len(srs.Pk.G1)
	case *kzgbw6.SRS:
		return len(srs.Pk.G1)
	default:
		return -1
	}
}

// syncDir best-effort fsyncs a directory so a just-renamed file survives a crash.
func syncDir(dir string) {
	d, err := os.Open(dir)
	if err != nil {
		logrus.Warnf("could not open %s to sync it: %v", dir, err)
		return
	}
	if err := d.Sync(); err != nil {
		logrus.Warnf("could not sync %s: %v", dir, err)
	}
	d.Close()
}

func toLagrange(srs kzg.SRS, sizeLagrange int) (kzg.SRS, error) {
	var err error
	// the verifying key is basis-independent: carry it over so the derived SRS
	// (and any dump written from it) is faithful, not just its proving key
	switch srs := srs.(type) {
	case *kzg254.SRS:
		lagrange := &kzg254.SRS{Vk: srs.Vk}
		lagrange.Pk.G1, err = kzg254.ToLagrangeG1(srs.Pk.G1[:sizeLagrange])
		return lagrange, err
	case *kzg377.SRS:
		lagrange := &kzg377.SRS{Vk: srs.Vk}
		lagrange.Pk.G1, err = kzg377.ToLagrangeG1(srs.Pk.G1[:sizeLagrange])
		return lagrange, err
	case *kzgbw6.SRS:
		lagrange := &kzgbw6.SRS{Vk: srs.Vk}
		lagrange.Pk.G1, err = kzgbw6.ToLagrangeG1(srs.Pk.G1[:sizeLagrange])
		return lagrange, err
	default:
		panic("unknown SRS type")
	}
}
