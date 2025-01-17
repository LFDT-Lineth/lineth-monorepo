//go:build !race

package smartvectorsext

import (
	"fmt"
	"github.com/consensys/linea-monorepo/prover/maths/common/polyext"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/vectorext"
	"github.com/consensys/linea-monorepo/prover/maths/fft/fastpolyext"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/fft"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRuffini(t *testing.T) {

	testCases := []struct {
		q           fext.Element
		p           smartvectors.SmartVector
		expectedQuo smartvectors.SmartVector
		expectedRem fext.Element
	}{
		{
			q:           fext.NewElement(1, 0),
			p:           ForTestFromPairs(3, 0, 0, 0, 1, 0),
			expectedQuo: ForTestFromPairs(1, 0, 1, 0),
			expectedRem: fext.NewElement(4, 0),
		},
		{
			// 3 = 0 * (X - 1) + 3
			q:           fext.NewElement(1, 0),
			p:           ForTestFromPairs(3, 0),
			expectedQuo: NewConstantExt(fext.Zero(), 1),
			expectedRem: fext.NewElement(3, 0),
		},
		{
			// -α^2 - 3 α + α x^3 + x^3 - α^2 x^2 - 2 α x^2 - x^2 + α x + 2 x + 3 =
			// (x-(1+alpha))(x^2*(1+alpha)+(2+alpha))+5
			// alpha is a square root used to build the extension field, i.e. alpha^2=fext.RootPowers[1]
			q:           fext.NewElement(1, 1),
			p:           ForTestFromPairs(-fext.RootPowers[1]+3, -3, 2, 1, -1-fext.RootPowers[1], -2, 1, 1),
			expectedQuo: ForTestFromPairs(2, 1, 0, 0, 1, 1),
			expectedRem: fext.NewElement(5, 0),
		},
	}

	for _, testCase := range testCases {

		quo, rem := RuffiniQuoRem(testCase.p, testCase.q)
		require.Equal(t, testCase.expectedQuo.Pretty(), quo.Pretty())
		require.Equal(t, testCase.expectedRem.String(), rem.String())
	}

}

func TestFuzzPolynomial(t *testing.T) {

	for i := 0; i < smartvectors.FuzzIteration; i++ {

		// We reuse the test-case generator of lincomb but we only
		// use the first generated edge-case for each. The fact that
		// we use two test-cases gives a and b of different length.
		tcaseA := newTestBuilder(2 * i).NewTestCaseForLinComb()
		tcaseB := newTestBuilder(2*i + 1).NewTestCaseForLinComb()

		success := t.Run(fmt.Sprintf("fuzz-poly-%v", i), func(t *testing.T) {

			a := tcaseA.svecs[0]
			b := tcaseB.svecs[0]

			// Try interpolating by one (should return the first element)
			xa := Interpolate(a, fext.One())
			expecteda0 := a.GetExt(0)
			assert.Equal(t, xa.String(), expecteda0.String())

			// Get a random x to use as an evaluation point to check polynomial
			// identities
			var x fext.Element
			x.SetRandom()
			aX := EvalCoeff(a, x)
			bX := EvalCoeff(b, x)

			// Get the evaluations of a-n, b-a, a+b
			var aSubBx, bSubAx, aPlusBx fext.Element
			aSubBx.Sub(&aX, &bX)
			bSubAx.Sub(&bX, &aX)
			aPlusBx.Add(&aX, &bX)

			// And evaluate the corresponding polynomials to compare
			// with the above values
			aSubb := PolySub(a, b)
			bSuba := PolySub(b, a)
			aPlusb := PolyAdd(a, b)
			bPlusa := PolyAdd(b, a)

			aSubBxActual := EvalCoeff(aSubb, x)
			bSubAxActual := EvalCoeff(bSuba, x)
			aPlusbxActual := EvalCoeff(aPlusb, x)
			bPlusaxActual := EvalCoeff(bPlusa, x)

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

func TestBivariatePolynomial(t *testing.T) {

	testCases := []struct {
		v         smartvectors.SmartVector
		x         fext.Element
		y         fext.Element
		numCoeffX int
		res       fext.Element
	}{
		{
			// P(X) = P1(X)+Y*P2(X) = (1+4)+(3+8)Y = 5+11Y = 5+33 = 38
			v:         ForTestExt(1, 2, 3, 4),
			x:         fext.NewElement(2, 0),
			y:         fext.NewElement(3, 0),
			numCoeffX: 2,
			res:       fext.NewElement(38, 0),
		},
		{
			// P(X) = P1(X)+Y*P2(X)
			v:         ForTestFromPairs(1, 1, 2, 2, 3, 3, 4, 4),
			x:         fext.NewElement(2, 1),
			y:         fext.NewElement(3, 2),
			numCoeffX: 2,
			res: *new(fext.Element).
				SetInt64Pair(
					int64(44*fext.RootPowers[1]+38),
					int64(8*fext.RootPowers[1]+74)),
		},
	}

	for i, testCase := range testCases {

		t.Run(fmt.Sprintf("case-%v", i), func(t *testing.T) {
			res := EvalCoeffBivariate(
				testCase.v,
				testCase.x,
				testCase.numCoeffX,
				testCase.y,
			)

			require.Equal(t, testCase.res.String(), res.String())
		})
	}

}

func TestBatchInterpolationWithConstantVector(t *testing.T) {
	n := 4
	randPoly := vectorext.ForTest(1, 2, 3, 4)
	randPoly2 := vectorext.ForTest(1, 1, 1, 1)

	x := fext.NewElement(51, fieldPaddingInt())

	expectedY := polyext.EvalUnivariate(randPoly, x)
	expectedY2 := polyext.EvalUnivariate(randPoly2, x)
	domain := fft.NewDomain(n).WithCoset()

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
	fft.BitReverseExt(polys[0])
	fft.BitReverseExt(polys[1])

	yOnRoots := fastpolyext.BatchInterpolate(polys, x)
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
	fft.BitReverseExt(onCosets[0])
	fft.BitReverseExt(onCosets[1])

	yOnCosets := fastpolyext.BatchInterpolate(onCosets, x, true)
	require.Equal(t, expectedY.String(), yOnCosets[0].String())
	require.Equal(t, expectedY2.String(), yOnCosets[1].String())

}

func TestBatchInterpolateOnlyConstantVector(t *testing.T) {
	n := 4
	randPoly := vectorext.ForTest(1, 1, 1, 1)
	randPoly2 := vectorext.ForTest(2, 2, 2, 2)
	x := fext.NewElement(51, fieldPaddingInt())

	expectedY := polyext.EvalUnivariate(randPoly, x)
	expectedY2 := polyext.EvalUnivariate(randPoly2, x)
	domain := fft.NewDomain(n).WithCoset()

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
	fft.BitReverseExt(polys[0])
	fft.BitReverseExt(polys[1])

	yOnRoots := fastpolyext.BatchInterpolate(polys, x)
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
	fft.BitReverseExt(onCosets[0])
	fft.BitReverseExt(onCosets[1])

	yOnCosets := fastpolyext.BatchInterpolate(onCosets, x, true)
	require.Equal(t, expectedY.String(), yOnCosets[0].String())
	require.Equal(t, expectedY2.String(), yOnCosets[1].String())
}

// three vectors to see if range check and continue statement
// for edge cases works as expected
func TestBatchInterpolationThreeVectors(t *testing.T) {
	n := 4
	randPoly := vectorext.ForTest(1, 2, 3, 4)
	randPoly2 := vectorext.ForTest(1, 1, 1, 1)
	randPoly3 := vectorext.ForTest(1, 2, 3, 4)

	x := fext.NewElement(51, fieldPaddingInt())

	expectedY := polyext.EvalUnivariate(randPoly, x)
	expectedY2 := polyext.EvalUnivariate(randPoly2, x)
	expectedY3 := polyext.EvalUnivariate(randPoly3, x)
	domain := fft.NewDomain(n).WithCoset()

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
	fft.BitReverseExt(polys[0])
	fft.BitReverseExt(polys[1])
	fft.BitReverseExt(polys[2])

	yOnRoots := fastpolyext.BatchInterpolate(polys, x)
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
	fft.BitReverseExt(onCosets[0])
	fft.BitReverseExt(onCosets[1])
	fft.BitReverseExt(onCosets[2])

	yOnCosets := fastpolyext.BatchInterpolate(onCosets, x, true)
	require.Equal(t, expectedY.String(), yOnCosets[0].String())
	require.Equal(t, expectedY2.String(), yOnCosets[1].String())
	require.Equal(t, expectedY3.String(), yOnCosets[2].String())

}
