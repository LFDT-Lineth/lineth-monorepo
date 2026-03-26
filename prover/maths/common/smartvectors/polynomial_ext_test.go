//go:build !race

package smartvectors

import (
	"fmt"
	"testing"

	"github.com/consensys/gnark-crypto/field/koalabear/vortex"
	"github.com/consensys/gnark-crypto/utils"

	"github.com/consensys/linea-monorepo/prover/maths/common/vectorext"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"

	"github.com/consensys/gnark-crypto/field/koalabear/fft"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRuffiniExt(t *testing.T) {

	testCases := []struct {
		q           fext.Element
		p           SmartVector
		expectedQuo SmartVector
		expectedRem fext.Element
	}{
		{
			q:           fext.NewFromUint(1, 0, 0, 0),
			p:           ForTestExt(3, 0, 1),
			expectedQuo: ForTestExt(1, 1),

			expectedRem: fext.NewFromUint(4, 0, 0, 0),
		},
		{
			// 3 = 0 * (X - 1) + 3
			q:           fext.NewFromUint(1, 0, 0, 0),
			p:           ForTestFromQuads(3, 0, 0, 0),
			expectedQuo: NewConstantExt(fext.Zero(), 1),
			expectedRem: fext.NewFromUint(3, 0, 0, 0),
		},
		{
			// p = -α^2 - 3 α + α x^3 + x^3 - α^2 x^2 - 2 α x^2 - x^2 + α x + 2 x + 3 =
			// q*expectedQuo + expectedRem = (x-(1+alpha))(x^2*(1+alpha)+(2+alpha))+5
			// alpha is a square root used to build the extension field, i.e. alpha^2=fext.RootPowers[1]
			q:           fext.NewFromUint(1, 1, 0, 0),
			p:           ForTestFromQuads(-fext.RootPowers[1]+3, -3, 0, 0, 2, 1, 0, 0, -1-fext.RootPowers[1], -2, 0, 0, 1, 1, 0, 0),
			expectedQuo: ForTestFromQuads(2, 1, 0, 0, 0, 0, 0, 0, 1, 1, 0, 0),
			expectedRem: fext.NewFromUint(5, 0, 0, 0),
		},
	}

	for _, testCase := range testCases {

		quo, rem := RuffiniQuoRemExt(testCase.p, testCase.q)
		require.Equal(t, testCase.expectedQuo.Pretty(), quo.Pretty())
		require.Equal(t, testCase.expectedRem.String(), rem.String())
	}

}

func TestFuzzPolynomialExt(t *testing.T) {

	for i := 0; i < FuzzIteration; i++ {

		// We reuse the test-case generator of lincomb but we only
		// use the first generated edge-case for each. The fact that
		// we use two test-cases gives a and b of different length.
		tcaseA := newTestBuilderExt(2 * i).NewTestCaseForLinCombExt()
		tcaseB := newTestBuilderExt(2*i + 1).NewTestCaseForLinCombExt()

		success := t.Run(fmt.Sprintf("fuzz-poly-%v", i), func(t *testing.T) {

			a := tcaseA.svecs[0]
			b := tcaseB.svecs[0]

			// Try interpolating by one (should return the first element)
			xa := EvaluateFextPolyLagrange(a, fext.One())
			expecteda0 := a.GetExt(0)
			assert.Equal(t, xa.String(), expecteda0.String())

			// Get a random x to use as an evaluation point to check polynomial
			// identities
			var x fext.Element
			x.SetRandom()
			aX := EvalCoeffExt(a, x)
			bX := EvalCoeffExt(b, x)

			// Get the evaluations of a-n, b-a, a+b
			var aSubBx, bSubAx, aPlusBx fext.Element
			aSubBx.Sub(&aX, &bX)
			bSubAx.Sub(&bX, &aX)
			aPlusBx.Add(&aX, &bX)

			// And evaluate the corresponding polynomials to compare
			// with the above values
			aSubb := PolySubExt(a, b)
			bSuba := PolySubExt(b, a)
			aPlusb := PolyAddExt(a, b)
			bPlusa := PolyAddExt(b, a)

			aSubBxActual := EvalCoeffExt(aSubb, x)
			bSubAxActual := EvalCoeffExt(bSuba, x)
			aPlusbxActual := EvalCoeffExt(aPlusb, x)
			bPlusaxActual := EvalCoeffExt(bPlusa, x)

			t.Logf(
				"Len of a %v, b %v, a+b %v, a-b %v, b-a %v",
				a.Len(), b.Len(), aPlusb.Len(), aSubb.Len(), bSuba.Len(),
			)

			require.Equal(t, aSubBx.String(), aSubBxActual.String())
			require.Equal(t, bSubAx.String(), bSubAxActual.String())
			require.Equal(t, aPlusBx.String(), aPlusbxActual.String())
			require.Equal(t, aPlusBx.String(), bPlusaxActual.String())
		})

		if !success {
			t.Logf("TEST CASE %v\n", i)
			t.FailNow()
		}
	}
}

func TestBivariatePolynomialExt(t *testing.T) {

	testCases := []struct {
		v         SmartVector
		x         fext.Element
		y         fext.Element
		numCoeffX int
		res       fext.Element
	}{
		{
			// P(X) = P1(X)+Y*P2(X) = (1+2x)+(3+4x)Y = 5+11Y = 5+33 = 38
			v:         ForTestExt(1, 2, 3, 4),
			x:         fext.NewFromUint(2, 0, 0, 0),
			y:         fext.NewFromUint(3, 0, 0, 0),
			numCoeffX: 2,
			res:       fext.NewFromUint(38, 0, 0, 0),
		},
		{
			// P(X) = P1(X)+Y*P2(X)
			v:         ForTestFromQuads(1, 1, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3, 4, 4, 4, 4),
			x:         fext.NewFromUint(2, 0, 0, 0),
			y:         fext.NewFromUint(3, 0, 0, 0),
			numCoeffX: 2,
			res:       fext.NewFromUint(38, 38, 38, 38),
		},
	}

	for i, testCase := range testCases {

		t.Run(fmt.Sprintf("case-%v", i), func(t *testing.T) {
			res := EvalCoeffBivariateExt(
				testCase.v,
				testCase.x,
				testCase.numCoeffX,
				testCase.y,
			)

			require.Equal(t, testCase.res.String(), res.String())
		})
	}

}

func TestBatchEvaluateLagrangeExt(t *testing.T) {

	x := fext.RandomElement()

	nbPoly := 8
	size := 64

	polys := make([][]fext.Element, nbPoly)
	for i := 0; i < nbPoly; i++ {
		polys[i] = make([]fext.Element, size)
		for j := 0; j < size; j++ {
			polys[i][j].SetRandom()
		}
	}

	evalCan := make([]fext.Element, nbPoly)
	for i := 0; i < nbPoly; i++ {
		evalCan[i] = vortex.EvalFextPolyHorner(polys[i], x)
	}

	d := fft.NewDomain(uint64(size))
	polyLagranges := make([][]fext.Element, nbPoly)
	copy(polyLagranges, polys)

	polyLagrangeSv := make([]SmartVector, nbPoly)
	for i := 0; i < nbPoly; i++ {
		d.FFTExt(polyLagranges[i], fft.DIF)
		utils.BitReverse(polyLagranges[i])
		polyLagrangeSv[i] = NewRegularExt(polyLagranges[i])

	}
	evalLag := BatchEvaluateFextPolyLagrange(polyLagrangeSv, x)

	// check the result
	for i := 0; i < nbPoly; i++ {
		require.Equal(t, evalLag[i].String(), evalCan[i].String())
	}

}
func TestBatchInterpolationWithConstantVectorExt(t *testing.T) {
	n := 4
	randPoly := vectorext.ForTest(1, 2, 3, 4)
	randPoly2 := vectorext.ForTest(1, 1, 1, 1)

	x := fext.RandomElement()

	expectedY := vortex.EvalFextPolyHorner(randPoly, x)
	expectedY2 := vortex.EvalFextPolyHorner(randPoly2, x)
	domain := fft.NewDomain(uint64(n))

	/*
		Test without coset
	*/
	onRoots := vectorext.DeepCopy(randPoly)
	onRoots2 := vectorext.DeepCopy(randPoly2)
	polys := make([][]fext.Element, 2)
	polys[0] = onRoots
	polys[1] = onRoots2

	domain.FFTExt(polys[0], fft.DIF)
	domain.FFTExt(polys[1], fft.DIF)
	utils.BitReverse(polys[0])
	utils.BitReverse(polys[1])

	yOnRoots, err := vortex.BatchEvalFextPolyLagrange(polys, x)
	require.NoError(t, err)
	require.Equal(t, expectedY.String(), yOnRoots[0].String())
	require.Equal(t, expectedY2.String(), yOnRoots[1].String())

	/*
		Test with coset
	*/
	onCoset := vectorext.DeepCopy(randPoly)
	onCoset2 := vectorext.DeepCopy(randPoly2)
	onCosets := make([][]fext.Element, 2)
	onCosets[0] = onCoset
	onCosets[1] = onCoset2

	domain.FFTExt(onCosets[0], fft.DIF, fft.OnCoset())
	domain.FFTExt(onCosets[1], fft.DIF, fft.OnCoset())
	utils.BitReverse(onCosets[0])
	utils.BitReverse(onCosets[1])

	yOnCosets, err := vortex.BatchEvalFextPolyLagrange(onCosets, x, true)
	require.NoError(t, err)
	require.Equal(t, expectedY.String(), yOnCosets[0].String())
	require.Equal(t, expectedY2.String(), yOnCosets[1].String())

}

func TestBatchEvaluateLagrangeOnlyConstantVector(t *testing.T) {
	n := 4
	randPoly := vectorext.ForTest(1, 1, 1, 1)
	randPoly2 := vectorext.ForTest(2, 2, 2, 2)
	x := fext.RandomElement()

	expectedY := vortex.EvalFextPolyHorner(randPoly, x)
	expectedY2 := vortex.EvalFextPolyHorner(randPoly2, x)
	domain := fft.NewDomain(uint64(n))

	/*
		Test without coset
	*/
	onRoots := vectorext.DeepCopy(randPoly)
	onRoots2 := vectorext.DeepCopy(randPoly2)
	polys := make([][]fext.Element, 2)
	polys[0] = onRoots
	polys[1] = onRoots2

	domain.FFTExt(polys[0], fft.DIF)
	domain.FFTExt(polys[1], fft.DIF)
	utils.BitReverse(polys[0])
	utils.BitReverse(polys[1])

	yOnRoots, err := vortex.BatchEvalFextPolyLagrange(polys, x)
	require.NoError(t, err)
	require.Equal(t, expectedY.String(), yOnRoots[0].String())
	require.Equal(t, expectedY2.String(), yOnRoots[1].String())

	/*
		Test with coset
	*/
	onCoset := vectorext.DeepCopy(randPoly)
	onCoset2 := vectorext.DeepCopy(randPoly2)
	onCosets := make([][]fext.Element, 2)
	onCosets[0] = onCoset
	onCosets[1] = onCoset2

	domain.FFTExt(onCosets[0], fft.DIF, fft.OnCoset())
	domain.FFTExt(onCosets[1], fft.DIF, fft.OnCoset())
	utils.BitReverse(onCosets[0])
	utils.BitReverse(onCosets[1])

	yOnCosets, err := vortex.BatchEvalFextPolyLagrange(onCosets, x, true)
	require.NoError(t, err)
	require.Equal(t, expectedY.String(), yOnCosets[0].String())
	require.Equal(t, expectedY2.String(), yOnCosets[1].String())
}

// three vectors to see if range check and continue statement
// for edge cases works as expected
func TestBatchInterpolationThreeVectorsExt(t *testing.T) {
	n := 4
	randPoly := vectorext.ForTest(1, 2, 3, 4)
	randPoly2 := vectorext.ForTest(1, 1, 1, 1)
	randPoly3 := vectorext.ForTest(1, 2, 3, 4)

	x := fext.RandomElement()

	expectedY := vortex.EvalFextPolyHorner(randPoly, x)
	expectedY2 := vortex.EvalFextPolyHorner(randPoly2, x)
	expectedY3 := vortex.EvalFextPolyHorner(randPoly3, x)
	domain := fft.NewDomain(uint64(n))

	/*
		Test without coset
	*/
	onRoots := vectorext.DeepCopy(randPoly)
	onRoots2 := vectorext.DeepCopy(randPoly2)
	onRoots3 := vectorext.DeepCopy(randPoly3)
	polys := make([][]fext.Element, 3)
	polys[0] = onRoots
	polys[1] = onRoots2
	polys[2] = onRoots3

	domain.FFTExt(polys[0], fft.DIF)
	domain.FFTExt(polys[1], fft.DIF)
	domain.FFTExt(polys[2], fft.DIF)
	utils.BitReverse(polys[0])
	utils.BitReverse(polys[1])
	utils.BitReverse(polys[2])

	yOnRoots, err := vortex.BatchEvalFextPolyLagrange(polys, x)
	require.NoError(t, err)
	require.Equal(t, expectedY.String(), yOnRoots[0].String())
	require.Equal(t, expectedY2.String(), yOnRoots[1].String())
	require.Equal(t, expectedY3.String(), yOnRoots[2].String())

	/*
		Test with coset
	*/
	onCoset := vectorext.DeepCopy(randPoly)
	onCoset2 := vectorext.DeepCopy(randPoly2)
	onCoset3 := vectorext.DeepCopy(randPoly3)
	onCosets := make([][]fext.Element, 3)
	onCosets[0] = onCoset
	onCosets[1] = onCoset2
	onCosets[2] = onCoset3

	domain.FFTExt(onCosets[0], fft.DIF, fft.OnCoset())
	domain.FFTExt(onCosets[1], fft.DIF, fft.OnCoset())
	domain.FFTExt(onCosets[2], fft.DIF, fft.OnCoset())
	utils.BitReverse(onCosets[0])
	utils.BitReverse(onCosets[1])
	utils.BitReverse(onCosets[2])

	yOnCosets, err := vortex.BatchEvalFextPolyLagrange(onCosets, x, true)
	require.NoError(t, err)
	require.Equal(t, expectedY.String(), yOnCosets[0].String())
	require.Equal(t, expectedY2.String(), yOnCosets[1].String())
	require.Equal(t, expectedY3.String(), yOnCosets[2].String())

}
