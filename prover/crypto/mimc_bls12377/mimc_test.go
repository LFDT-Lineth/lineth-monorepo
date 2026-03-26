package mimc_test

import (
	"testing"

	field "github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	mimc "github.com/consensys/linea-monorepo/prover/crypto/mimc_bls12377"
	"github.com/stretchr/testify/require"
)

func TestMiMCBloc(t *testing.T) {

	for i := 0; i < 100; i++ {

		hasher := mimc.NewMiMC()

		// old is set to zero
		var x, old field.Element

		// s is set to a random value. Each run of the test will
		// generate a different value.
		x.SetRandom()
		xBytes := x.Bytes()

		newState := mimc.BlockCompression(old, x)

		hasher.Write(xBytes[:])
		newBytes := hasher.Sum(nil)
		var newFromHasher field.Element
		newFromHasher.SetBytes(newBytes)

		require.Equal(t, newFromHasher.String(), newState.String())
	}
}
