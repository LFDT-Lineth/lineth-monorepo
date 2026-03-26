package mimc_test

import (
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/mimc"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/stretchr/testify/require"
)

// circuit
type Circuit struct {
	Block frontend.Variable
	Old   frontend.Variable
	New   frontend.Variable
}

func (circuit *Circuit) Define(api frontend.API) error {
	res := mimc.GnarkBlockCompression(api, circuit.Old, circuit.Block)
	api.AssertIsEqual(res, circuit.New)
	return nil
}

func TestGnarkCompression(t *testing.T) {

	r1cs, err := frontend.Compile(
		ecc.BLS12_377.ScalarField(),
		r1cs.NewBuilder,
		&Circuit{},
	)
	require.NoError(t, err)

	assignment := Circuit{
		Block: 1,
		Old:   2,
		New:   mimc.BlockCompression(field.NewElement(2), field.NewElement(1)),
	}

	witness, err := frontend.NewWitness(&assignment, ecc.BLS12_377.ScalarField())
	require.NoError(t, err)

	err = r1cs.IsSolved(witness)
	require.NoError(t, err)
}
