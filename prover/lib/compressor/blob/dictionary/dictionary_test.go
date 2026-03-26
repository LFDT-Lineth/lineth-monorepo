package dictionary

import (
	"hash"
	"os"
	"testing"

	"github.com/consensys/linea-monorepo/prover/lib/compressor/blob/encode"
	"github.com/stretchr/testify/require"
)

func checksum(hsh hash.Hash, data []byte) []byte {
	hsh.Reset()
	hsh.Write(data)
	return hsh.Sum(nil)
}

func TestLoadDict(t *testing.T) {
	const (
		dict1Path = "../../compressor_dict.bin"
		dict2Path = "../../dict/25-04-21.bin"
	)
	dict1, err := os.ReadFile(dict1Path)
	require.NoError(t, err)
	dict2, err := os.ReadFile(dict2Path)
	require.NoError(t, err)

	dict1MiMC, err := encode.MiMCChecksumPackedData(dict1, 8)
	require.NoError(t, err)

	dict2MiMC, err := encode.MiMCChecksumPackedData(dict2, 8)
	require.NoError(t, err)

	dict1Poseidon2, err := encode.Poseidon2ChecksumPackedData(dict1, 8)
	require.NoError(t, err)

	dict2Poseidon2, err := encode.Poseidon2ChecksumPackedData(dict2, 8)
	require.NoError(t, err)

	store := NewStore(dict1Path, dict2Path)

	// load version 2
	res, err := store.Get(dict1Poseidon2, 2)
	require.NoError(t, err)
	require.Equal(t, dict1, res)

	res, err = store.Get(dict2Poseidon2, 2)
	require.NoError(t, err)
	require.Equal(t, dict2, res)

	res, err = store.Get(dict1MiMC, 2)
	require.Error(t, err)

	res, err = store.Get(dict2MiMC, 2)
	require.Error(t, err)

	// load version 1
	res, err = store.Get(dict1MiMC, 1)
	require.NoError(t, err)
	require.Equal(t, dict1, res)

	res, err = store.Get(dict2MiMC, 1)
	require.NoError(t, err)
	require.Equal(t, dict2, res)

	res, err = store.Get(dict1Poseidon2, 1)
	require.Error(t, err)

	res, err = store.Get(dict2Poseidon2, 1)
	require.Error(t, err)
}
