//go:build !fuzzlight

package gkrmimc

import (
	"fmt"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/mimc"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/stretchr/testify/require"
)

// simple hashing circuit
type SimpleHashingCircuit struct {
	Input  []frontend.Variable
	Digest frontend.Variable
}

type SimpleHashingCircuitWithAPI SimpleHashingCircuit

// just hash the input using the factory's hasher and return the result
func (c SimpleHashingCircuit) Define(api frontend.API) error {
	factory := NewHasherFactory(api)
	hasher := factory.NewHasher()

	hasher.Write(c.Input[:]...)
	d := hasher.Sum()

	api.AssertIsEqual(d, c.Digest)
	return nil
}

func TestFactory(t *testing.T) {

	scs, err := frontend.Compile(
		ecc.BLS12_377.ScalarField(),
		scs.NewBuilder,
		&SimpleHashingCircuit{Input: make([]frontend.Variable, 4)},
	)
	require.NoError(t, err)

	assignment := SimpleHashingCircuit{
		Input:  []frontend.Variable{0, 1, 2, 3},
		Digest: mimc.HashVec(vector.ForTest(0, 1, 2, 3)),
	}

	witness, err := frontend.NewWitness(&assignment, ecc.BLS12_377.ScalarField())
	require.NoError(t, err)

	err = scs.IsSolved(witness)
	require.NoError(t, err)
}

func TestFactoryWithPadding(t *testing.T) {

	scs, err := frontend.Compile(
		ecc.BLS12_377.ScalarField(),
		scs.NewBuilder,
		&SimpleHashingCircuit{Input: make([]frontend.Variable, 3)},
	)
	require.NoError(t, err)

	assignment := SimpleHashingCircuit{
		Input:  []frontend.Variable{0, 1, 2},
		Digest: mimc.HashVec(vector.ForTest(0, 1, 2)),
	}

	witness, err := frontend.NewWitness(&assignment, ecc.BLS12_377.ScalarField())
	require.NoError(t, err)

	err = scs.IsSolved(witness)
	require.NoError(t, err)
}

func TestFactoryManySizes(t *testing.T) {

	for _, size := range []int{1, 2, 3, 12, 37, 64, 110, 128,
		129, 221, 240, 256, 260,
	} {
		t.Run(fmt.Sprintf("%v-hashes", size), func(t *testing.T) {
			scs, err := frontend.Compile(
				ecc.BLS12_377.ScalarField(),
				scs.NewBuilder,
				&SimpleHashingCircuit{Input: make([]frontend.Variable, size)},
			)
			require.NoError(t, err)

			vals := make([]field.Element, size)
			inputs := make([]frontend.Variable, size)
			for i := range inputs {
				inputs[i] = field.NewElement(uint64(i))
				vals[i] = field.NewElement(uint64(i))
			}

			assignment := SimpleHashingCircuit{
				Input:  inputs,
				Digest: mimc.HashVec(vals),
			}

			witness, err := frontend.NewWitness(&assignment, ecc.BLS12_377.ScalarField())
			require.NoError(t, err)

			err = scs.IsSolved(witness)
			require.NoError(t, err)
		})
	}

}
