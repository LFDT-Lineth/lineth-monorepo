package poseidon2

import (
	"testing"

	"github.com/consensys/gnark-crypto/field/koalabear"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/stretchr/testify/require"
)

//---------------------------------------
// native variables

type GnarkMDHasherCircuit struct {
	Inputs []frontend.Variable
	Output GnarkOctuplet
}

func (ghc *GnarkMDHasherCircuit) Define(api frontend.API) error {

	h, err := NewGnarkMDHasher(api)
	if err != nil {
		return err
	}

	// write elmts
	h.Write(ghc.Inputs...)

	// sum
	res := h.Sum()

	// check the result
	for i := 0; i < 8; i++ {
		api.AssertIsEqual(ghc.Output[i], res[i])
	}

	return nil
}

func getGnarkMDHasherCircuitWitness() (*GnarkMDHasherCircuit, *GnarkMDHasherCircuit) {

	// values to hash
	nbElmts := 16
	vals := make([]field.Element, nbElmts)
	for i := 0; i < nbElmts; i++ {
		if _, err := vals[i].SetRandom(); err != nil {
			panic(err)
		}
	}

	// sum
	phasher := NewMDHasher()
	phasher.WriteElements(vals...)
	res := phasher.SumElement()

	// create witness and circuit
	var circuit, witness GnarkMDHasherCircuit
	circuit.Inputs = make([]frontend.Variable, nbElmts)
	witness.Inputs = make([]frontend.Variable, nbElmts)
	for i := 0; i < nbElmts; i++ {
		witness.Inputs[i] = vals[i].String()
	}
	for i := 0; i < 8; i++ {
		witness.Output[i] = res[i].String()
	}

	return &circuit, &witness

}

func TestCircuit(t *testing.T) {

	circuit, witness := getGnarkMDHasherCircuitWitness()

	ccs, err := frontend.CompileU32(koalabear.Modulus(), scs.NewBuilder, circuit)
	require.NoError(t, err)

	fullWitness, err := frontend.NewWitness(witness, koalabear.Modulus())
	require.NoError(t, err)
	err = ccs.IsSolved(fullWitness)
	require.NoError(t, err)

}
