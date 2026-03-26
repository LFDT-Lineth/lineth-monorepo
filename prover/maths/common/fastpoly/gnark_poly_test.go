package fastpoly

import (
	"testing"

	"github.com/consensys/gnark-crypto/field/koalabear"
	"github.com/consensys/gnark-crypto/field/koalabear/vortex"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
	"github.com/stretchr/testify/assert"
)

type EvaluateLagrangeCircuit struct {
	X    koalagnark.Ext       // point of evaluation
	Poly []koalagnark.Element // poly in Lagrange form
	R    koalagnark.Ext       // expected result
}

func (c *EvaluateLagrangeCircuit) Define(api frontend.API) error {

	r := EvaluateLagrangeGnarkMixed(api, c.Poly, c.X)

	koalaAPI := koalagnark.NewAPI(api)
	koalaAPI.AssertIsEqualExt(c.R, r)

	return nil
}

func getWitnessAndCircuit(t koalagnark.VType) (EvaluateLagrangeCircuit, EvaluateLagrangeCircuit) {

	// sample random poly and random point
	size := 8
	poly := make([]field.Element, size)
	for i := 0; i < size; i++ {
		poly[i].SetRandom()
	}
	var x fext.Element
	x.SetRandom()

	// eval lagrange
	r, err := vortex.EvalBasePolyLagrange(poly, x)
	if err != nil {
		panic(err)
	}
	// test circuit
	var witness, ckt EvaluateLagrangeCircuit
	ckt.Poly = make([]koalagnark.Element, size)
	witness.Poly = make([]koalagnark.Element, size)
	for i := 0; i < size; i++ {
		witness.Poly[i] = koalagnark.NewElementFromKoala(poly[i])
	}
	witness.R = koalagnark.NewExt(r)
	witness.X = koalagnark.NewExt(x)

	return ckt, witness

}

func TestEvaluateLagrangeGnark(t *testing.T) {

	{
		ckt, witness := getWitnessAndCircuit(koalagnark.Native)
		ccs, err := frontend.CompileU32(koalabear.Modulus(), scs.NewBuilder, &ckt)
		assert.NoError(t, err)

		fullWitness, err := frontend.NewWitness(&witness, koalabear.Modulus())
		assert.NoError(t, err)
		err = ccs.IsSolved(fullWitness)
		assert.NoError(t, err)
	}

	{
		ckt, witness := getWitnessAndCircuit(koalagnark.Emulated)
		ccs, err := frontend.CompileU32(koalabear.Modulus(), scs.NewBuilder, &ckt)
		assert.NoError(t, err)

		fullWitness, err := frontend.NewWitness(&witness, koalabear.Modulus())
		assert.NoError(t, err)
		err = ccs.IsSolved(fullWitness)
		assert.NoError(t, err)
	}
}
