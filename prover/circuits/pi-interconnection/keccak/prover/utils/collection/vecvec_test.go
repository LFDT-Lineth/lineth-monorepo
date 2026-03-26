package collection_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/collection"
	"github.com/stretchr/testify/require"
)

func TestVecVecLenOf(t *testing.T) {
	vecvec := collection.NewVecVec[int]()
	res := vecvec.LenOf(1)
	require.Equal(t, 0, res)
}
