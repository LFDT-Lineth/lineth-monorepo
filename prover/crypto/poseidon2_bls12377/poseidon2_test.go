package poseidon2_bls12377

import (
	"strconv"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	_ "github.com/consensys/gnark/std/hash/all" // Register all hash function getters
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
	"github.com/stretchr/testify/assert"
)

// ------------------------------------------------------------
// hashing bls12377 elmts

type GnarkMDHasherCircuit struct {
	Inputs []frontend.Variable
	Ouput  frontend.Variable
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
	api.AssertIsEqual(ghc.Ouput, res)

	return nil
}

func getGnarkMDHasherCircuitWitness(nbElmts int) (*GnarkMDHasherCircuit, *GnarkMDHasherCircuit) {

	// values to hash
	vals := make([]fr.Element, nbElmts)
	for i := 0; i < nbElmts; i++ {
		// vals[i].SetRandom()
		vals[i].SetRandom()
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
	witness.Ouput = res.String()

	return &circuit, &witness

}

func TestCircuit(t *testing.T) {

	// Define all the sizes you want to test here
	testSizes := []int{3, 6, 444}

	for _, size := range testSizes {
		// Run a sub-test for each size
		t.Run("Size_"+strconv.Itoa(size), func(t *testing.T) {

			// Pass the size to the helper
			circuit, witness := getGnarkMDHasherCircuitWitness(size)

			// Compile (Must happen for every new size)
			ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, circuit)
			assert.NoError(t, err)

			fullWitness, err := frontend.NewWitness(witness, ecc.BLS12_377.ScalarField())
			assert.NoError(t, err)

			err = ccs.IsSolved(fullWitness)
			assert.NoError(t, err)
		})
	}

}

// ------------------------------------------------------------
// hashing koalabear elmts

type GnarkMDHasherCircuitKoalabear struct {
	Inputs []koalagnark.Element
	Ouput  frontend.Variable
}

func (ghc *GnarkMDHasherCircuitKoalabear) Define(api frontend.API) error {

	h, err := NewGnarkMDHasher(api)
	if err != nil {
		return err
	}

	// write elmts
	h.WriteWVs(ghc.Inputs...)

	// sum
	res := h.Sum()

	// check the result
	api.AssertIsEqual(ghc.Ouput, res)

	return nil
}

func getGnarkMDHasherCircuitKoalabearWitness() (*GnarkMDHasherCircuitKoalabear, *GnarkMDHasherCircuitKoalabear) {

	// values to hash
	nbElmts := 2
	vals := make([]field.Element, nbElmts)
	for i := 0; i < nbElmts; i++ {
		// vals[i].SetRandom()
		vals[i].SetUint64(uint64(10 + i))
	}

	// sum
	phasher := NewMDHasher()
	phasher.WriteKoalabearElements(vals...)
	res := phasher.SumElement()

	// create witness and circuit
	var circuit, witness GnarkMDHasherCircuitKoalabear
	circuit.Inputs = make([]koalagnark.Element, nbElmts)
	witness.Inputs = make([]koalagnark.Element, nbElmts)
	for i := 0; i < nbElmts; i++ {
		witness.Inputs[i] = koalagnark.NewElementFromKoala(vals[i])
	}
	witness.Ouput = res.String()

	return &circuit, &witness

}

func TestCircuitKoalabear(t *testing.T) {

	circuit, witness := getGnarkMDHasherCircuitKoalabearWitness()

	ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, circuit)
	assert.NoError(t, err)

	fullWitness, err := frontend.NewWitness(witness, ecc.BLS12_377.ScalarField())
	assert.NoError(t, err)
	err = ccs.IsSolved(fullWitness)
	assert.NoError(t, err)

}
