package gnarkutil

import (
	"bytes"
	"testing"

	fr377 "github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	"github.com/stretchr/testify/require"
)

// python style
func _range(n int) []byte {
	res := make([]byte, n)
	for i := range n {
		res[i] = byte(i)
	}
	return res
}

func TestPackLoose(t *testing.T) {
	data := _range(32)
	var (
		bb       bytes.Buffer
		expected [64]byte
	)
	copy(expected[1:], data)
	expected[32] = 0
	expected[33] = 31
	require.NoError(t, PackLoose(&bb, data, fr377.Bytes, 1))
	require.Equal(t, expected[:], bb.Bytes())
}
