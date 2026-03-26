package fiatshamir

import (
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/field/koalabear"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/linea-monorepo/prover/crypto/fiatshamir_bls12377"
	"github.com/consensys/linea-monorepo/prover/crypto/fiatshamir_koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
	"github.com/stretchr/testify/assert"
)

type FSCircuit struct {
	A                  [10]koalagnark.Element
	RandomA            koalagnark.Octuplet
	B                  [10]koalagnark.Ext
	RandomB            koalagnark.Octuplet
	RandomField        koalagnark.Octuplet
	RandomFieldExt     koalagnark.Ext
	RandomManyIntegers [10]frontend.Variable
	isKoala            bool
}

func assertEqualOctuplet(api *koalagnark.API, a, b koalagnark.Octuplet) {
	for i := 0; i < 8; i++ {
		api.AssertIsEqual(a[i], b[i])
	}
}

func (c *FSCircuit) Define(api frontend.API) error {

	var fs GnarkFS
	if c.isKoala {
		fs = NewGnarkFSKoalabear(api)
	} else {
		fs = NewGnarkFSBLS12377(api)
	}

	koalaAPI := koalagnark.NewAPI(api)

	fs.Update(c.A[:]...)
	randomA := fs.RandomField()
	assertEqualOctuplet(koalaAPI, randomA, c.RandomA)

	fs.UpdateExt(c.B[:]...)
	randomB := fs.RandomField()
	assertEqualOctuplet(koalaAPI, randomB, c.RandomB)

	randomField := fs.RandomField()

	assertEqualOctuplet(koalaAPI, randomField, c.RandomField)

	randomFieldExt := fs.RandomFieldExt()
	koalaAPI.AssertIsEqual(randomFieldExt.B0.A0, c.RandomFieldExt.B0.A0)
	koalaAPI.AssertIsEqual(randomFieldExt.B0.A1, c.RandomFieldExt.B0.A1)
	koalaAPI.AssertIsEqual(randomFieldExt.B1.A0, c.RandomFieldExt.B1.A0)
	koalaAPI.AssertIsEqual(randomFieldExt.B1.A1, c.RandomFieldExt.B1.A1)

	randomManyIntegers := fs.RandomManyIntegers(10, 16)
	for i := 0; i < 10; i++ {
		api.AssertIsEqual(randomManyIntegers[i], c.RandomManyIntegers[i])
	}

	return nil
}

func getWitnessCircuit(isKoala bool) (*FSCircuit, *FSCircuit) {

	var fs FS
	var circuit, witness FSCircuit
	circuit.isKoala = isKoala
	witness.isKoala = isKoala
	if isKoala {
		fs = fiatshamir_koalabear.NewFS()
	} else {
		fs = fiatshamir_bls12377.NewFS()
	}

	var A [10]field.Element
	for i := 0; i < 10; i++ {
		A[i].SetRandom()
	}
	fs.Update(A[:]...)
	RandomA := fs.RandomField()

	var B [10]fext.Element
	for i := 0; i < 10; i++ {
		B[i].SetRandom()
	}
	fs.UpdateExt(B[:]...)

	RandomB := fs.RandomField()

	RandomField := fs.RandomField()
	RandomFieldExt := fs.RandomFext()
	RandomManyIntegers := fs.RandomManyIntegers(10, 16)

	for i := 0; i < 10; i++ {
		witness.A[i] = koalagnark.NewElementFromKoala(A[i])
	}
	for i := 0; i < 8; i++ {
		witness.RandomA[i] = koalagnark.NewElementFromKoala(RandomA[i])
	}

	for i := 0; i < 10; i++ {
		witness.B[i].B0.A0 = koalagnark.NewElementFromKoala(B[i].B0.A0)
		witness.B[i].B0.A1 = koalagnark.NewElementFromKoala(B[i].B0.A1)
		witness.B[i].B1.A0 = koalagnark.NewElementFromKoala(B[i].B1.A0)
		witness.B[i].B1.A1 = koalagnark.NewElementFromKoala(B[i].B1.A1)
	}
	for i := 0; i < 8; i++ {
		witness.RandomB[i] = koalagnark.NewElementFromKoala(RandomB[i])
	}

	for i := 0; i < 8; i++ {
		witness.RandomField[i] = koalagnark.NewElementFromKoala(RandomField[i])
	}

	witness.RandomFieldExt.B0.A0 = koalagnark.NewElementFromKoala(RandomFieldExt.B0.A0)
	witness.RandomFieldExt.B0.A1 = koalagnark.NewElementFromKoala(RandomFieldExt.B0.A1)
	witness.RandomFieldExt.B1.A0 = koalagnark.NewElementFromKoala(RandomFieldExt.B1.A0)
	witness.RandomFieldExt.B1.A1 = koalagnark.NewElementFromKoala(RandomFieldExt.B1.A1)

	for i := 0; i < 10; i++ {
		witness.RandomManyIntegers[i] = RandomManyIntegers[i]
	}

	return &circuit, &witness
}

func TestFSCircuit(t *testing.T) {

	// compile on koala
	{
		circuit, witness := getWitnessCircuit(true)
		ccs, err := frontend.CompileU32(koalabear.Modulus(), scs.NewBuilder, circuit)
		assert.NoError(t, err)

		fullWitness, err := frontend.NewWitness(witness, koalabear.Modulus())
		assert.NoError(t, err)
		err = ccs.IsSolved(fullWitness)
		assert.NoError(t, err)
	}

	// compile on bls
	{
		circuit, witness := getWitnessCircuit(false)
		ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, circuit)
		assert.NoError(t, err)

		fullWitness, err := frontend.NewWitness(witness, ecc.BLS12_377.ScalarField())
		assert.NoError(t, err)
		err = ccs.IsSolved(fullWitness)
		assert.NoError(t, err)
	}

}
