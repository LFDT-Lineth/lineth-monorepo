package dictionary

import (
	"bytes"
	"errors"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark-crypto/hash"
	"github.com/consensys/gnark/std/compress"
	"github.com/consensys/linea-monorepo/prover/lib/compressor/blob/encode"
	"os"
)

// Checksum according to the given spec version
func Checksum(dict []byte, version uint16) ([]byte, error) {
	switch version {
	case 1:
		return encode.MiMCChecksumPackedData(dict, 8)
	case 0:
		return compress.ChecksumPaddedBytes(dict, len(dict), hash.MIMC_BLS12_377.New(), fr.Bits), nil
	}
	return nil, errors.New("unsupported version")
}

type Store []map[string][]byte

func NewStore(paths ...string) Store {
	res := make(Store, 2)
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

func (s Store) Load(paths ...string) error {
	loadVsn := func(vsn uint16) error {
		for _, path := range paths {
			dict, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			checksum, err := Checksum(dict, vsn)
			if err != nil {
				return err
			}
			key := string(checksum)
			existing, exists := s[vsn][key]
			if exists && !bytes.Equal(dict, existing) { // should be incredibly unlikely
				return errors.New("unmatching dictionary found")
			}
			s[vsn][key] = dict
		}
		return nil
	}

	return errors.Join(loadVsn(0), loadVsn(1))
}

func (s Store) Get(checksum []byte, version uint16) ([]byte, error) {
	if int(version) > len(s) {
		return nil, errors.New("unrecognized blob version")
	}
	res, ok := s[version][string(checksum)]
	if !ok {
		return nil, errors.New("dictionary not found")
	}
	return res, nil
}
