package poseidon2

import (
	"math/rand/v2"
	"testing"

	"github.com/consensys/gnark-crypto/field/koalabear/vortex"

	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/stretchr/testify/require"
)

func TestPoseidon2BlockCompressionConsistency(t *testing.T) {
	// This test is to check the consistency with vortex.CompressPoseidon2
	var rng = rand.New(utils.NewRandSource(0)) // #nosec G404 -- test only

	var old, block [blockSize]field.Element
	for i := 0; i < 100; i++ {
		for i := 0; i < blockSize; i++ {
			old[i] = field.PseudoRand(rng)
			block[i] = field.PseudoRand(rng)
		}
		state := poseidon2BlockCompression(old, block)
		gnarkState := vortex.CompressPoseidon2(old, block)

		for i := 0; i < blockSize; i++ {
			require.Equal(t, gnarkState[i].String(), state[i].String(), "Poseidon2 compression functions should produce the same state")
		}
	}
}
