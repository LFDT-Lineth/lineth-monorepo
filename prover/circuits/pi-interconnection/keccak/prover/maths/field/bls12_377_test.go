package field

import (
	"math/big"
	"testing"

	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMulRInv(t *testing.T) {
	computedR := big.NewInt(2)

	computedR.Exp(computedR, big.NewInt(fr.Limbs*64), Modulus())
	assert.Equal(t, computedR.String(), r.String(), "bad R")

	computedRInv := new(big.Int).ModInverse(computedR, Modulus())
	assert.Equal(t, computedRInv.String(), rInv.String(), "bad RInv")
}

func TestRootOfUnity(t *testing.T) {
	var shouldBeOne, shouldNotBeOne Element
	shouldBeOne.Exp(RootOfUnity, big.NewInt(1<<int64(RootOfUnityOrder)))
	shouldNotBeOne.Exp(RootOfUnity, big.NewInt(1<<int64(RootOfUnityOrder-1)))

	assert.Equal(t, "1", shouldBeOne.String(), "not a root of unity")
	assert.Equal(t, "-1", shouldNotBeOne.Text(10), "wrong order")
}

func TestParBatchInvert(t *testing.T) {

	vec := make([]Element, 1024)
	for i := range vec {
		vec[i].SetRandom()
	}

	vec1 := BatchInvert(vec)
	vec2 := ParBatchInvert(vec, 16)

	for i := range vec {
		require.Equalf(t, vec1[i].String(), vec2[i].String(), "at position %v", i)
	}
}
