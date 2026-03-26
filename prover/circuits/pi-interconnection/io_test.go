package pi_interconnection

import (
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	fr381 "github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/test"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/stretchr/testify/assert"
)

func TestFr377EncodedFr381ToBytes(t *testing.T) {
	var circuit fr377EncodedFr381ToBytesCircuit

	for i := 0; i < 10; i++ {
		var x fr381.Element
		_, err := x.SetRandom()
		assert.NoError(t, err)
		xBytes := x.Bytes()
		encoded := types.AsBls12377Fr(xBytes[:])
		assert.NoError(t, err)

		assignment := fr377EncodedFr381ToBytesCircuit{
			Encoded: [2]frontend.Variable{encoded[:16], encoded[16:]},
		}

		b := x.Bytes()
		for j := range b {
			assignment.ExpectedRecoded[j] = b[j]
		}

		assert.NoError(t, test.IsSolved(&circuit, &assignment, ecc.BLS12_377.ScalarField()))
	}
}

type fr377EncodedFr381ToBytesCircuit struct {
	Encoded         [2]frontend.Variable
	ExpectedRecoded [32]frontend.Variable
}

func (c *fr377EncodedFr381ToBytesCircuit) Define(api frontend.API) error {
	for i, b := range fr377EncodedFr381ToBytes(api, c.Encoded) {
		api.AssertIsEqual(b, c.ExpectedRecoded[i])
	}
	return nil
}
