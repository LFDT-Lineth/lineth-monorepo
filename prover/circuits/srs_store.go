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
	source      string // the ceremony tag in the file name: aleo, aztec or celo
}

// orphanTempMaxAge is how old a temp file must be before store construction
// sweeps it; a live writer refreshes its temp's mtime while streaming the dump.
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

	srsRegexp := regexp.MustCompile(`^(kzg_srs)_(canonical|lagrange)_(\d+)_(bls12377|bn254|bw6761)_(aleo|aztec|celo)\.memdump$`)

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
			// a crash mid-write (e.g. an OOM-kill during a multi-GiB dump)
			// orphans a temp file that nothing indexes or reclaims; sweep it
			// once it is old enough that no live writer can still own it
			if strings.HasPrefix(fileName, "kzg_srs_") && strings.Contains(fileName, ".memdump.tmp") {
				if info, err := entry.Info(); err == nil && time.Since(info.ModTime()) > orphanTempMaxAge {
					if err := os.Remove(filepath.Join(rootDir, fileName)); err != nil {
						logrus.Warnf("could not remove orphaned srs temp file %s: %v", fileName, err)
					} else {
						logrus.Infof("removed orphaned srs temp file %s", fileName)
					}
				}
			}
			continue
		}

		isCanonical := matches[2] == "canonical"
		size, _ := strconv.Atoi(matches[3])
		source := matches[5]
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

// GetSRS returns the canonical and Lagrange SRS for the circuit, deriving and
// persisting the Lagrange form when no loadable dump is on disk. Concurrent
// callers requesting the same missing size each derive independently (the
// store is race-safe but does not deduplicate the work); every in-repo caller
// is sequential today.
func (store *SRSStore) GetSRS(ctx context.Context, ccs constraint.ConstraintSystem) (kzg.SRS, kzg.SRS, error) {
	sizeCanonical, sizeLagrange := plonk.SRSSize(ccs)
	curveID := fieldToCurve(ccs.Field())

	entries := store.entriesSnapshot(curveID)

	// find the canonical srs
	var canonicalSRS kzg.SRS
	var canonicalEntry fsEntry
	for _, entry := range entries {
		if entry.isCanonical && entry.size >= sizeCanonical {
			canonicalSRS = kzg.NewSRS(curveID)
			data, err := os.ReadFile(entry.path)
			if err != nil {
				return nil, nil, err
			}
			if err := canonicalSRS.ReadDump(bytes.NewReader(data), sizeCanonical); err != nil {
				return nil, nil, err
			}
			canonicalEntry = entry
			break
		}
	}

	if canonicalSRS == nil {
		return nil, nil, fmt.Errorf("could not find canonical SRS for curve %s and size %d", curveID, sizeCanonical)
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
			// and fall back to deriving, which re-persists over this same path
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
		logrus.Debugf("computing lagrange SRS from canonical SRS %d -> %d", sizeCanonical, sizeLagrange)
		var err error
		lagrangeSRS, err = toLagrange(canonicalSRS, sizeLagrange)
		if err != nil {
			return nil, nil, err
		}
		// Persist the derived Lagrange SRS so subsequent runs load it from disk
		// instead of re-deriving it. Best-effort: a failed write must not fail
		// the caller.
		if err := store.cacheLagrange(lagrangeSRS, sizeLagrange, curveID, canonicalEntry.source); err != nil {
			logrus.Warnf("could not persist derived lagrange SRS (continuing): %v", err)
		}
	}

	return canonicalSRS, lagrangeSRS, nil
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

// cacheLagrange writes a derived Lagrange SRS into the store's directory using
// the naming scheme NewSRSStore parses, and registers it in the index. The dump
// is fsync'd and renamed into place atomically, so a torn write cannot appear
// under a trusted name. On load, framing, size, setup-identity (Vk) and
// point-validity (on-curve) errors are all caught and re-derived; substitution
// of validly-encoded points is out of scope, as for every file in the store.
func (store *SRSStore) cacheLagrange(lagrangeSRS kzg.SRS, sizeLagrange int, curveID ecc.ID, source string) error {
	curveName, ok := curveFileNames[curveID]
	if !ok {
		return fmt.Errorf("curve not supported: %s", curveID)
	}

	fileName := fmt.Sprintf("kzg_srs_lagrange_%d_%s_%s.memdump", sizeLagrange, curveName, source)
	finalPath := filepath.Join(store.rootDir, fileName)

	f, err := os.CreateTemp(store.rootDir, fileName+".tmp")
	if err != nil {
		return err
	}
	if err := lagrangeSRS.WriteDump(f); err != nil {
		f.Close()
		os.Remove(f.Name())
		return fmt.Errorf("writing srs dump: %w", err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(f.Name())
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return err
	}
	// ceremony files in the store are world-readable; match them so the cache
	// stays loadable when a later run uses a different uid
	if err := os.Chmod(f.Name(), 0o644); err != nil {
		os.Remove(f.Name())
		return err
	}
	if err := os.Rename(f.Name(), finalPath); err != nil {
		os.Remove(f.Name())
		return err
	}
	syncDir(store.rootDir)

	store.register(curveID, fsEntry{isCanonical: false, size: sizeLagrange, path: finalPath, source: source})

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
