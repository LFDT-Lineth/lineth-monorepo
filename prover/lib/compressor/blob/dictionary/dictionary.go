package dictionary

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark-crypto/hash"
	_ "github.com/consensys/gnark-crypto/hash/all"
	"github.com/consensys/gnark/std/compress"
	"github.com/consensys/linea-monorepo/prover/lib/compressor/blob/encode"
)

// Checksum according to the given spec version
func Checksum(dict []byte, version uint16) ([]byte, error) {
	switch version {
	case 2:
		return encode.Poseidon2ChecksumPackedData(dict, 8)
	case 1:
		return encode.MiMCChecksumPackedData(dict, 8)
	case 0:
		return compress.ChecksumPaddedBytes(dict, len(dict), hash.MIMC_BLS12_377.New(), fr.Bits), nil
	}
	return nil, errors.New("unsupported version")
}

type Store []map[string][]byte

func NewStore(paths ...string) Store {
	res := make(Store, 3)
	for i := range res {
		res[i] = make(map[string][]byte)
	}
	if err := res.Load(paths...); err != nil {
		panic(err)
	}
	return res
}

func SingletonStore(dict []byte, version uint16) (Store, error) {
	s := make(Store, version+1)
	key, err := Checksum(dict, version)
	s[version] = make(map[string][]byte, 1)
	s[version][string(key)] = dict
	return s, err
}

func (s Store) add(dict []byte, version uint16) error {
	checksum, err := Checksum(dict, version)
	if err != nil {
		return err
	}
	key := string(checksum)
	existing, exists := s[version][key]
	if exists && !bytes.Equal(dict, existing) { // should be incredibly unlikely
		return errors.New("unmatching dictionary found")
	}
	s[version][key] = dict
	return nil
}

func (s Store) Load(paths ...string) error {
	// by default load for the most recent version.
	vsn := uint16(len(s)) - 1

	for _, path := range paths {
		dict, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err = s.add(dict, vsn); err != nil {
			return err
		}
	}
	return nil

}

func (s Store) Get(checksum []byte, version uint16) ([]byte, error) {
	if int(version) >= len(s) {
		return nil, fmt.Errorf("unrecognized blob version %d - dictionary store initialized for maximum version %d", version, len(s)-1)
	}
	res, ok := s[version][string(checksum)]
	if !ok {
		// All dictionaries are by default loaded for the most recent blob version.
		// If not found, iterate through the entire store of the most recent version
		// to try and find it.
		if len(s[version]) != len(s[len(s)-1]) {
			// Any previous version's store is a subset of the most recent one's.
			// If their sizes are the same, they are identical, so no point in searching.
			// If NOT the same (i.e. proper subset), go through them all
			// and cache along the way.
			// The first run of this condition is slow, but subsequent ones are fast.
			for _, dict := range s[len(s)-1] {
				if err := s.add(dict, version); err != nil {
					return nil, err
				}
			}
			return s.Get(checksum, version)
		}
		return nil, fmt.Errorf("blob v%d: no dictionary found in store with checksum %x", version, checksum)
	}

	return res, nil
}
