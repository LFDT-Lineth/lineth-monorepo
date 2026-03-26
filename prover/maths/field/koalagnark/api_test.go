package koalagnark

import (
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/field/koalabear"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/stretchr/testify/assert"
)

// TestVarCircuit tests basic Var operations
type TestVarCircuit struct {
	A, B  Element
	MulAB Element
	AddAB Element
	SubAB Element
	DivAB Element
	NegA  Element
}

func (c *TestVarCircuit) Define(api frontend.API) error {
	f := NewAPI(api)

	tmp := f.Mul(c.A, c.B)
	f.AssertIsEqual(tmp, c.MulAB)

	tmp = f.Add(c.A, c.B)
	f.AssertIsEqual(tmp, c.AddAB)

	tmp = f.Sub(c.A, c.B)
	f.AssertIsEqual(tmp, c.SubAB)

	tmp = f.Div(c.A, c.B)
	f.AssertIsEqual(tmp, c.DivAB)

	tmp = f.Neg(c.A)
	f.AssertIsEqual(tmp, c.NegA)

	return nil
}

func getVarWitness() TestVarCircuit {
	var a, b, mulab, addab, subab, divab, nega field.Element
	a.SetRandom()
	b.SetRandom()
	mulab.Mul(&a, &b)
	addab.Add(&a, &b)
	subab.Sub(&a, &b)
	divab.Div(&a, &b)
	nega.Neg(&a)

	return TestVarCircuit{
		A:     NewElementFromKoala(a),
		B:     NewElementFromKoala(b),
		MulAB: NewElementFromKoala(mulab),
		AddAB: NewElementFromKoala(addab),
		SubAB: NewElementFromKoala(subab),
		DivAB: NewElementFromKoala(divab),
		NegA:  NewElementFromKoala(nega),
	}
}

func TestVarNative(t *testing.T) {
	witness := getVarWitness()
	var circuit TestVarCircuit

	ccs, err := frontend.CompileU32(koalabear.Modulus(), scs.NewBuilder, &circuit)
	assert.NoError(t, err)

	fullWitness, err := frontend.NewWitness(&witness, koalabear.Modulus())
	assert.NoError(t, err)

	err = ccs.IsSolved(fullWitness)
	assert.NoError(t, err)
}

func TestVarEmulated(t *testing.T) {
	witness := getVarWitness()
	var circuit TestVarCircuit

	ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &circuit)
	assert.NoError(t, err)

	fullWitness, err := frontend.NewWitness(&witness, ecc.BLS12_377.ScalarField())
	assert.NoError(t, err)

	err = ccs.IsSolved(fullWitness)
	assert.NoError(t, err)
}

// TestExtCircuit tests Ext operations
type TestExtCircuit struct {
	A, B    Ext
	AddAB   Ext
	SubAB   Ext
	MulAB   Ext
	SquareA Ext
	DivAB   Ext
	InvA    Ext
}

func (c *TestExtCircuit) Define(api frontend.API) error {
	f := NewAPI(api)

	addAB := f.AddExt(c.A, c.B)
	f.AssertIsEqualExt(addAB, c.AddAB)

	subAB := f.SubExt(c.A, c.B)
	f.AssertIsEqualExt(subAB, c.SubAB)

	mulAB := f.MulExt(c.A, c.B)
	f.AssertIsEqualExt(mulAB, c.MulAB)

	squareA := f.SquareExt(c.A)
	f.AssertIsEqualExt(squareA, c.SquareA)

	divAB := f.DivExt(c.A, c.B)
	f.AssertIsEqualExt(divAB, c.DivAB)

	invA := f.InverseExt(c.A)
	f.AssertIsEqualExt(invA, c.InvA)

	return nil
}

func getExtWitness() *TestExtCircuit {
	var a, b, addab, subab, mulab, squarea, inva, divab fext.Element
	a.SetRandom()
	b.SetRandom()
	addab.Add(&a, &b)
	subab.Sub(&a, &b)
	mulab.Mul(&a, &b)
	squarea.Square(&a)
	divab.Div(&a, &b)
	inva.Inverse(&a)

	return &TestExtCircuit{
		A:       NewExt(a),
		B:       NewExt(b),
		AddAB:   NewExt(addab),
		SubAB:   NewExt(subab),
		MulAB:   NewExt(mulab),
		SquareA: NewExt(squarea),
		DivAB:   NewExt(divab),
		InvA:    NewExt(inva),
	}
}

func TestExtNative(t *testing.T) {
	witness := getExtWitness()
	var circuit TestExtCircuit

	ccs, err := frontend.CompileU32(koalabear.Modulus(), scs.NewBuilder, &circuit)
	assert.NoError(t, err)

	fullWitness, err := frontend.NewWitness(witness, koalabear.Modulus())
	assert.NoError(t, err)

	err = ccs.IsSolved(fullWitness)
	assert.NoError(t, err)
}

func TestExtEmulated(t *testing.T) {
	witness := getExtWitness()
	var circuit TestExtCircuit

	ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &circuit)
	assert.NoError(t, err)

	fullWitness, err := frontend.NewWitness(witness, ecc.BLS12_377.ScalarField())
	assert.NoError(t, err)

	err = ccs.IsSolved(fullWitness)
	assert.NoError(t, err)
}
