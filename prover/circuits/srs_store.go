package circuits

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/kzg"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/gnark/constraint"
	"github.com/sirupsen/logrus"

	kzg377 "github.com/consensys/gnark-crypto/ecc/bls12-377/kzg"
	kzg254 "github.com/consensys/gnark-crypto/ecc/bn254/kzg"
	kzgbw6 "github.com/consensys/gnark-crypto/ecc/bw6-761/kzg"
)

type SRSStore struct {
	entries map[ecc.ID][]fsEntry
}

type fsEntry struct {
	isCanonical bool
	size        int
	path        string
}

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
			continue
		}

		isCanonical := matches[2] == "canonical"
		size, _ := strconv.Atoi(matches[3])
		var curveID ecc.ID
		switch matches[4] {
		case "bls12377":
			curveID = ecc.BLS12_377
		case "bn254":
			curveID = ecc.BN254
		case "bw6761":
			curveID = ecc.BW6_761
		default:
			return nil, errors.New("curve not supported")
		}

		srsStore.entries[curveID] = append(srsStore.entries[curveID], fsEntry{
			isCanonical: isCanonical,
			size:        size,
			path:        filepath.Join(rootDir, fileName),
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

func (store *SRSStore) GetSRS(ctx context.Context, ccs constraint.ConstraintSystem) (kzg.SRS, kzg.SRS, error) {
	sizeCanonical, sizeLagrange := plonk.SRSSize(ccs)
	curveID := fieldToCurve(ccs.Field())

	// find the canonical srs
	var canonicalSRS kzg.SRS
	for _, entry := range store.entries[curveID] {
		if entry.isCanonical && entry.size >= sizeCanonical {
			canonicalSRS = kzg.NewSRS(curveID)
			data, err := os.ReadFile(entry.path)
			if err != nil {
				return nil, nil, err
			}
			if err := canonicalSRS.ReadDump(bytes.NewReader(data), sizeCanonical); err != nil {
				return nil, nil, err
			}
			break
		}
	}

	if canonicalSRS == nil {
		return nil, nil, fmt.Errorf("could not find canonical SRS for curve %s and size %d", curveID, sizeCanonical)
	}

	// find the lagrange srs
	var lagrangeSRS kzg.SRS
	for _, entry := range store.entries[curveID] {
		if !entry.isCanonical && entry.size == sizeLagrange {
			lagrangeSRS = kzg.NewSRS(curveID)
			data, err := os.ReadFile(entry.path)
			if err != nil {
				return nil, nil, err
			}
			if err := lagrangeSRS.ReadDump(bytes.NewReader(data)); err != nil {
				return nil, nil, err
			}
			break
		}
	}

	if lagrangeSRS == nil {
		// we can compute it from the canonical one.
		if sizeCanonical < sizeLagrange {
			panic("canonical SRS is smaller than lagrange SRS")
		}
		logrus.Debugf("computing lagrange SRS from canonical SRS %d -> %d\n", sizeCanonical, sizeLagrange)
		var err error
		lagrangeSRS, err = toLagrange(canonicalSRS, sizeLagrange)
		if err != nil {
			return nil, nil, err
		}
	}

	return canonicalSRS, lagrangeSRS, nil
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
