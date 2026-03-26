package polynomials

import (
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/field/koalabear"
	"github.com/consensys/gnark-crypto/field/koalabear/fft"
	"github.com/consensys/gnark-crypto/utils"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
	"github.com/stretchr/testify/assert"
)

type LagrangeEvaluationCircuit struct {
	Poly   []koalagnark.Ext
	X      koalagnark.Ext
	Y      koalagnark.Ext
	Domain *fft.Domain
}

func (c *LagrangeEvaluationCircuit) Define(api frontend.API) error {

	y := GnarkEvaluateLagrangeExt(api, c.Poly, c.X, c.Domain.Generator, c.Domain.Cardinality)
	koalaAPI := koalagnark.NewAPI(api)
	koalaAPI.AssertIsEqual(c.Y.B0.A0, y.B0.A0)
	koalaAPI.AssertIsEqual(c.Y.B0.A1, y.B0.A1)
	koalaAPI.AssertIsEqual(c.Y.B1.A0, y.B1.A0)
	koalaAPI.AssertIsEqual(c.Y.B1.A1, y.B1.A1)

	return nil
}

func fftextinplace(poly []fext.Element, d *fft.Domain) {

	size := len(poly)

	// fft on each coordinates ... @thomas where is the fft ext ???
	buf := make([]field.Element, size)

	for i := 0; i < size; i++ {
		buf[i].Set(&poly[i].B0.A0)
	}
	d.FFT(buf, fft.DIF)
	utils.BitReverse(buf)
	for i := 0; i < size; i++ {
		poly[i].B0.A0.Set(&buf[i])
	}

	for i := 0; i < size; i++ {
		buf[i].Set(&poly[i].B0.A1)
	}
	d.FFT(buf, fft.DIF)
	utils.BitReverse(buf)
	for i := 0; i < size; i++ {
		poly[i].B0.A1.Set(&buf[i])
	}

	for i := 0; i < size; i++ {
		buf[i].Set(&poly[i].B1.A0)
	}
	d.FFT(buf, fft.DIF)
	utils.BitReverse(buf)
	for i := 0; i < size; i++ {
		poly[i].B1.A0.Set(&buf[i])
	}

	for i := 0; i < size; i++ {
		buf[i].Set(&poly[i].B1.A1)
	}
	d.FFT(buf, fft.DIF)
	utils.BitReverse(buf)
	for i := 0; i < size; i++ {
		poly[i].B1.A1.Set(&buf[i])
	}
}

func fftextInverseinplace(poly []fext.Element, d *fft.Domain) {

	size := len(poly)

	// fft on each coordinates ... @thomas where is the fft ext ???
	buf := make([]field.Element, size)

	for i := 0; i < size; i++ {
		buf[i].Set(&poly[i].B0.A0)
	}
	d.FFTInverse(buf, fft.DIF)
	utils.BitReverse(buf)
	for i := 0; i < size; i++ {
		poly[i].B0.A0.Set(&buf[i])
	}

	for i := 0; i < size; i++ {
		buf[i].Set(&poly[i].B0.A1)
	}
	d.FFTInverse(buf, fft.DIF)
	utils.BitReverse(buf)
	for i := 0; i < size; i++ {
		poly[i].B0.A1.Set(&buf[i])
	}

	for i := 0; i < size; i++ {
		buf[i].Set(&poly[i].B1.A0)
	}
	d.FFTInverse(buf, fft.DIF)
	utils.BitReverse(buf)
	for i := 0; i < size; i++ {
		poly[i].B1.A0.Set(&buf[i])
	}

	for i := 0; i < size; i++ {
		buf[i].Set(&poly[i].B1.A1)
	}
	d.FFTInverse(buf, fft.DIF)
	utils.BitReverse(buf)
	for i := 0; i < size; i++ {
		poly[i].B1.A1.Set(&buf[i])
	}
}

func TestLagrangeEvaluation(t *testing.T) {

	size := 64
	d := fft.NewDomain(uint64(size))
	poly := make([]fext.Element, size)
	for i := 0; i < size; i++ {
		poly[i].B0.A0.SetRandom()
	}

	var x fext.Element
	x.SetRandom()
	y := eval(poly, x)

	fftextinplace(poly, d)

	{
		var ckt, witness LagrangeEvaluationCircuit
		ckt.Poly = make([]koalagnark.Ext, size)
		ckt.Domain = d
		witness.Poly = make([]koalagnark.Ext, size)
		for i := 0; i < size; i++ {
			witness.Poly[i] = koalagnark.NewExt(poly[i])
			witness.X = koalagnark.NewExt(x)
			witness.Y = koalagnark.NewExt(y)
		}

		ccs, err := frontend.CompileU32(koalabear.Modulus(), scs.NewBuilder, &ckt)
		assert.NoError(t, err)

		fullWitness, err := frontend.NewWitness(&witness, koalabear.Modulus())
		assert.NoError(t, err)
		err = ccs.IsSolved(fullWitness)
		assert.NoError(t, err)
	}
	{
		var ckt, witness LagrangeEvaluationCircuit
		ckt.Poly = make([]koalagnark.Ext, size)
		ckt.Domain = d
		witness.Poly = make([]koalagnark.Ext, size)
		for i := 0; i < size; i++ {
			witness.Poly[i] = koalagnark.NewExt(poly[i])
			witness.X = koalagnark.NewExt(x)
			witness.Y = koalagnark.NewExt(y)
		}

		ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &ckt)
		assert.NoError(t, err)

		fullWitness, err := frontend.NewWitness(&witness, ecc.BLS12_377.ScalarField())
		assert.NoError(t, err)
		err = ccs.IsSolved(fullWitness)
		assert.NoError(t, err)
	}
}

type LagrangeAtZCircuit struct {
	X  koalagnark.Ext
	Li []koalagnark.Ext
	d  *fft.Domain
}

func (c *LagrangeAtZCircuit) Define(api frontend.API) error {
	li := gnarkComputeLagrangeAtZ(api, c.X, c.d.Generator, c.d.Cardinality)
	koalaAPI := koalagnark.NewAPI(api)
	for i := 0; i < len(li); i++ {
		koalaAPI.AssertIsEqual(li[i].B0.A0, c.Li[i].B0.A0)
		koalaAPI.AssertIsEqual(li[i].B0.A1, c.Li[i].B0.A1)
		koalaAPI.AssertIsEqual(li[i].B1.A0, c.Li[i].B1.A0)
		koalaAPI.AssertIsEqual(li[i].B1.A1, c.Li[i].B1.A1)
	}
	return nil
}

func TestGnarkComputeLagrangeAtZ(t *testing.T) {

	size := 8
	d := fft.NewDomain(uint64(size))
	li := make([]fext.Element, size)
	var x fext.Element
	x.SetRandom()
	for i := 0; i < size; i++ {
		buf := make([]fext.Element, size)
		buf[i].SetOne()
		fftextInverseinplace(buf, d)
		li[i] = eval(buf, x)
	}

	{
		var ckt, witness LagrangeAtZCircuit
		ckt.Li = make([]koalagnark.Ext, size)
		ckt.d = d
		witness.Li = make([]koalagnark.Ext, size)
		for i := 0; i < size; i++ {
			witness.Li[i] = koalagnark.NewExt(li[i])
		}
		witness.X = koalagnark.NewExt(x)
		ccs, err := frontend.CompileU32(koalabear.Modulus(), scs.NewBuilder, &ckt)
		assert.NoError(t, err)

		fullWitness, err := frontend.NewWitness(&witness, koalabear.Modulus())
		assert.NoError(t, err)
		err = ccs.IsSolved(fullWitness)
		assert.NoError(t, err)
	}

	{
		var ckt, witness LagrangeAtZCircuit
		ckt.Li = make([]koalagnark.Ext, size)
		ckt.d = d
		witness.Li = make([]koalagnark.Ext, size)
		for i := 0; i < size; i++ {
			witness.Li[i] = koalagnark.NewExt(li[i])
		}
		witness.X = koalagnark.NewExt(x)
		ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &ckt)
		assert.NoError(t, err)

		fullWitness, err := frontend.NewWitness(&witness, ecc.BLS12_377.ScalarField())
		assert.NoError(t, err)
		err = ccs.IsSolved(fullWitness)
		assert.NoError(t, err)
	}
}
