package smartvectors_mixed

import (
	"testing"

	"github.com/consensys/gnark-crypto/field/koalabear/fft"
	"github.com/consensys/gnark-crypto/field/koalabear/vortex"
	"github.com/consensys/gnark-crypto/utils"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/stretchr/testify/require"
)

func eval(p []field.Element, x fext.Element) fext.Element {
	var res fext.Element
	for i := len(p) - 1; i >= 0; i-- {
		res.Mul(&res, &x)
		res.B0.A0.Add(&res.B0.A0, &p[i])
	}
	return res
}

func TestComputeLagrangeBasisAtX(t *testing.T) {

	n := 64
	domain := fft.NewDomain(uint64(n))

	var x fext.Element
	x.SetRandom()
	r := computeLagrangeBasisAtX(n, x)

	_r := make([]fext.Element, n)
	for i := 0; i < n; i++ {
		m := make([]field.Element, n)
		m[i].SetOne()
		domain.FFTInverse(m, fft.DIF)
		utils.BitReverse(m)
		_r[i] = eval(m, x)
	}

	for i := 0; i < n; i++ {
		if !_r[i].Equal(&r[i]) {
			t.Fatal("error computeLagrangeBasisAtX")
		}
	}

}

func TestBatchEvaluateLagrange(t *testing.T) {

	x := fext.RandomElement()

	nbPoly := 8
	size := 64

	polys := make([][]field.Element, nbPoly)
	for i := 0; i < nbPoly; i++ {
		polys[i] = make([]field.Element, size)
		for j := 0; j < size; j++ {
			polys[i][j].SetRandom()
		}
	}

	evalCan := make([]fext.Element, nbPoly)
	for i := 0; i < nbPoly; i++ {
		evalCan[i] = vortex.EvalBasePolyHorner(polys[i], x)
	}

	d := fft.NewDomain(uint64(size))
	polyLagranges := make([][]field.Element, nbPoly)
	copy(polyLagranges, polys)

	polyLagrangeSv := make([]smartvectors.SmartVector, nbPoly)
	for i := 0; i < nbPoly; i++ {
		d.FFT(polyLagranges[i], fft.DIF)
		utils.BitReverse(polyLagranges[i])
		polyLagrangeSv[i] = smartvectors.NewRegular(polyLagranges[i])

	}
	evalLag := BatchEvaluateLagrange(polyLagrangeSv, x)

	// check the result
	for i := 0; i < nbPoly; i++ {
		require.Equal(t, evalLag[i].String(), evalCan[i].String())
	}

}
