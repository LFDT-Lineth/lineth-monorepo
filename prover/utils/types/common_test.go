package types_test

import (
	"bytes"
	"encoding/hex"
	"math/big"
	"math/rand/v2"
	"testing"

	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteBigInt(t *testing.T) {

	b := big.NewInt(10)

	buffer := &bytes.Buffer{}
	types.WriteBigIntOn64Bytes(buffer, b)

	// Converts to hex to simplify the reading
	hex := hex.EncodeToString(buffer.Bytes())
	assert.Equal(
		t,
		"0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000a",
		hex,
	)
}

func TestWriteInt64(t *testing.T) {

	tcases := []struct {
		N        int
		Expected string
	}{
		{
			N:        10,
			Expected: "0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000a",
		},
		{
			N:        7987979,
			Expected: "0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000790000e30b",
		},
	}

	for _, tc := range tcases {
		n := int64(tc.N)
		buffer := &bytes.Buffer{}
		types.WriteInt64On64Bytes(buffer, n)

		// Converts to hex to simplify the reading
		hex := hex.EncodeToString(buffer.Bytes())
		assert.Equal(
			t,
			tc.Expected,
			hex,
		)
	}
}

func TestReadWriteInt64(t *testing.T) {

	const nIterations = 100

	// #nosec G404 -- no need for a cryptographically strong PRNG for testing purposes
	rng := rand.New(rand.NewChaCha8([32]byte{}))

	for _i := 0; _i < nIterations; _i++ {
		n := rng.Int64()
		buffer := &bytes.Buffer{}
		types.WriteInt64On64Bytes(buffer, n)
		n2, _, err := types.ReadInt64On64Bytes(buffer)
		require.NoError(t, err)
		assert.Equal(t, n, n2)
	}
}

func TestReadWriteBigInt(t *testing.T) {

	const nIterations = 100

	// #nosec G404 -- no need for a cryptographically strong PRNG for testing purposes
	rng := rand.New(rand.NewChaCha8([32]byte{}))

	for _i := 0; _i < nIterations; _i++ {
		n := big.NewInt(rng.Int64())
		buffer := &bytes.Buffer{}
		types.WriteBigIntOn64Bytes(buffer, n)
		n2, err := types.ReadBigIntOn64Bytes(buffer)
		require.NoError(t, err)
		assert.Equal(t, n, n2)
	}

	// case where the big int is zero
	n := big.NewInt(0)
	buffer := &bytes.Buffer{}
	types.WriteBigIntOn64Bytes(buffer, n)
	n2, err := types.ReadBigIntOn64Bytes(buffer)
	require.NoError(t, err)
	assert.Equal(t, n, n2)
}
