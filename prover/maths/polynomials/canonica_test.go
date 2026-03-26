package polynomials

import (
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/field/koalabear"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
	"github.com/stretchr/testify/assert"
)

type EvalCanonicalCircuit struct {
	Poly []koalagnark.Element
	X    koalagnark.Ext
	Y    koalagnark.Ext
}

func (c *EvalCanonicalCircuit) Define(api frontend.API) error {

	y := GnarkEvalCanonical(api, c.Poly, c.X)
	koalaAPI := koalagnark.NewAPI(api)
	koalaAPI.AssertIsEqual(c.Y.B0.A0, y.B0.A0)
	koalaAPI.AssertIsEqual(c.Y.B0.A1, y.B0.A1)
	koalaAPI.AssertIsEqual(c.Y.B1.A0, y.B1.A0)
	koalaAPI.AssertIsEqual(c.Y.B1.A1, y.B1.A1)

	return nil
}

func TestGnarkEvalCanonical(t *testing.T) {

	size := 10
	poly := make([]fext.Element, size)
	for i := 0; i < size; i++ {
		poly[i].B0.A0.SetRandom()
	}
	var x fext.Element
	x.SetRandom()
	y := eval(poly, x)

	{
		var ckt, witness EvalCanonicalCircuit
		ckt.Poly = make([]koalagnark.Element, size)
		witness.Poly = make([]koalagnark.Element, size)
		for i := 0; i < size; i++ {
			witness.Poly[i] = koalagnark.NewElementFromKoala(poly[i].B0.A0)
		}
		witness.X = koalagnark.NewExt(x)
		witness.Y = koalagnark.NewExt(y)

		ccs, err := frontend.CompileU32(koalabear.Modulus(), scs.NewBuilder, &ckt)
		assert.NoError(t, err)

		fullWitness, err := frontend.NewWitness(&witness, koalabear.Modulus())
		assert.NoError(t, err)
		err = ccs.IsSolved(fullWitness)
		assert.NoError(t, err)
	}
	{
		var ckt, witness EvalCanonicalCircuit
		ckt.Poly = make([]koalagnark.Element, size)
		witness.Poly = make([]koalagnark.Element, size)
		for i := 0; i < size; i++ {
			witness.Poly[i] = koalagnark.NewElementFromKoala(poly[i].B0.A0)
		}
		witness.X = koalagnark.NewExt(x)
		witness.Y = koalagnark.NewExt(y)

		ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &ckt)
		assert.NoError(t, err)

		fullWitness, err := frontend.NewWitness(&witness, ecc.BLS12_377.ScalarField())
		assert.NoError(t, err)
		err = ccs.IsSolved(fullWitness)
		assert.NoError(t, err)
	}
}
