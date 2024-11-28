//go:build !race

package smartvectors

import (
	"fmt"
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/common/poly"
	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/fft"
	"github.com/consensys/linea-monorepo/prover/maths/fft/fastpoly"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRuffini(t *testing.T) {

	testCases := []struct {
		q           field.Element
		p           SmartVector
		expectedQuo SmartVector
		expectedRem field.Element
	}{
		{
			q:           field.NewElement(1),
			p:           ForTest(3, 0, 1),
			expectedQuo: ForTest(1, 1),
			expectedRem: field.NewElement(4),
		},
		{
			// 3 = 0 * (X - 1) + 3
			q:           field.NewElement(1),
			p:           ForTest(3),
			expectedQuo: NewConstant(field.Zero(), 1),
			expectedRem: field.NewElement(3),
		},
	}

	for _, testCase := range testCases {

		quo, rem := RuffiniQuoRem(testCase.p, testCase.q)
		require.Equal(t, testCase.expectedQuo.Pretty(), quo.Pretty())
		require.Equal(t, testCase.expectedRem.String(), rem.String())
	}

}

func TestFuzzPolynomial(t *testing.T) {

	for i := 0; i < fuzzIteration; i++ {

		// We reuse the test-case generator of lincomb but we only
		// use the first generated edge-case for each. The fact that
		// we use two test-cases gives a and b of different length.
		tcaseA := newTestBuilder(2 * i).NewTestCaseForLinComb()
		tcaseB := newTestBuilder(2*i + 1).NewTestCaseForLinComb()

		success := t.Run(fmt.Sprintf("fuzz-poly-%v", i), func(t *testing.T) {

			a := tcaseA.svecs[0]
			b := tcaseB.svecs[0]

			// Try interpolating by one (should return the first element)
			xa := Interpolate(a, field.One())
			expecteda0 := a.Get(0)
			assert.Equal(t, xa.String(), expecteda0.String())

			// Get a random x to use as an evaluation point to check polynomial
			// identities
			var x field.Element
			x.SetRandom()
			aX := EvalCoeff(a, x)
			bX := EvalCoeff(b, x)

			// Get the evaluations of a-n, b-a, a+b
			var aSubBx, bSubAx, aPlusBx field.Element
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
		v         SmartVector
		x         field.Element
		y         field.Element
		numCoeffX int
		res       field.Element
	}{
		{
			v:         ForTest(1, 2, 3, 4),
			x:         field.NewElement(2),
			y:         field.NewElement(3),
			numCoeffX: 2,
			res:       field.NewElement(38),
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
	randPoly := vector.ForTest(1, 2, 3, 4)
	randPoly2 := vector.ForTest(1, 1, 1, 1)

	x := field.NewElement(51)

	expectedY := poly.EvalUnivariate(randPoly, x)
	expectedY2 := poly.EvalUnivariate(randPoly2, x)
	domain := fft.NewDomain(n).WithCoset()

	/*
		Test without coset
	*/
	onRoots := vector.DeepCopy(randPoly)
	onRoots2 := vector.DeepCopy(randPoly2)
	polys := make([][]field.Element, 2)
	polys[0] = onRoots
	polys[1] = onRoots2

	domain.FFT(polys[0], fft.DIF)
	domain.FFT(polys[1], fft.DIF)
	fft.BitReverse(polys[0])
	fft.BitReverse(polys[1])

	yOnRoots := fastpoly.BatchInterpolate(polys, x)
	require.Equal(t, expectedY.String(), yOnRoots[0].String())
	require.Equal(t, expectedY2.String(), yOnRoots[1].String())

	/*
		Test with coset
	*/
	onCoset := vector.DeepCopy(randPoly)
	onCoset2 := vector.DeepCopy(randPoly2)
	onCosets := make([][]field.Element, 2)
	onCosets[0] = onCoset
	onCosets[1] = onCoset2

	domain.FFT(onCosets[0], fft.DIF, fft.OnCoset())
	domain.FFT(onCosets[1], fft.DIF, fft.OnCoset())
	fft.BitReverse(onCosets[0])
	fft.BitReverse(onCosets[1])

	yOnCosets := fastpoly.BatchInterpolate(onCosets, x, true)
	require.Equal(t, expectedY.String(), yOnCosets[0].String())
	require.Equal(t, expectedY2.String(), yOnCosets[1].String())

}

func TestBatchInterpolateOnlyConstantVector(t *testing.T) {
	n := 4
	randPoly := vector.ForTest(1, 1, 1, 1)
	randPoly2 := vector.ForTest(2, 2, 2, 2)
	x := field.NewElement(51)

	expectedY := poly.EvalUnivariate(randPoly, x)
	expectedY2 := poly.EvalUnivariate(randPoly2, x)
	domain := fft.NewDomain(n).WithCoset()

	/*
		Test without coset
	*/
	onRoots := vector.DeepCopy(randPoly)
	onRoots2 := vector.DeepCopy(randPoly2)
	polys := make([][]field.Element, 2)
	polys[0] = onRoots
	polys[1] = onRoots2

	domain.FFT(polys[0], fft.DIF)
	domain.FFT(polys[1], fft.DIF)
	fft.BitReverse(polys[0])
	fft.BitReverse(polys[1])

	yOnRoots := fastpoly.BatchInterpolate(polys, x)
	require.Equal(t, expectedY.String(), yOnRoots[0].String())
	require.Equal(t, expectedY2.String(), yOnRoots[1].String())

	/*
		Test with coset
	*/
	onCoset := vector.DeepCopy(randPoly)
	onCoset2 := vector.DeepCopy(randPoly2)
	onCosets := make([][]field.Element, 2)
	onCosets[0] = onCoset
	onCosets[1] = onCoset2

	domain.FFT(onCosets[0], fft.DIF, fft.OnCoset())
	domain.FFT(onCosets[1], fft.DIF, fft.OnCoset())
	fft.BitReverse(onCosets[0])
	fft.BitReverse(onCosets[1])

	yOnCosets := fastpoly.BatchInterpolate(onCosets, x, true)
	require.Equal(t, expectedY.String(), yOnCosets[0].String())
	require.Equal(t, expectedY2.String(), yOnCosets[1].String())
}

// three vectors to see if range check and continue statement
// for edge cases works as expected
func TestBatchInterpolationThreeVectors(t *testing.T) {
	n := 4
	randPoly := vector.ForTest(1, 2, 3, 4)
	randPoly2 := vector.ForTest(1, 1, 1, 1)
	randPoly3 := vector.ForTest(1, 2, 3, 4)

	x := field.NewElement(51)

	expectedY := poly.EvalUnivariate(randPoly, x)
	expectedY2 := poly.EvalUnivariate(randPoly2, x)
	expectedY3 := poly.EvalUnivariate(randPoly3, x)
	domain := fft.NewDomain(n).WithCoset()

	/*
		Test without coset
	*/
	onRoots := vector.DeepCopy(randPoly)
	onRoots2 := vector.DeepCopy(randPoly2)
	onRoots3 := vector.DeepCopy(randPoly3)
	polys := make([][]field.Element, 3)
	polys[0] = onRoots
	polys[1] = onRoots2
	polys[2] = onRoots3

	domain.FFT(polys[0], fft.DIF)
	domain.FFT(polys[1], fft.DIF)
	domain.FFT(polys[2], fft.DIF)
	fft.BitReverse(polys[0])
	fft.BitReverse(polys[1])
	fft.BitReverse(polys[2])

	yOnRoots := fastpoly.BatchInterpolate(polys, x)
	require.Equal(t, expectedY.String(), yOnRoots[0].String())
	require.Equal(t, expectedY2.String(), yOnRoots[1].String())
	require.Equal(t, expectedY3.String(), yOnRoots[2].String())

	/*
		Test with coset
	*/
	onCoset := vector.DeepCopy(randPoly)
	onCoset2 := vector.DeepCopy(randPoly2)
	onCoset3 := vector.DeepCopy(randPoly3)
	onCosets := make([][]field.Element, 3)
	onCosets[0] = onCoset
	onCosets[1] = onCoset2
	onCosets[2] = onCoset3

	domain.FFT(onCosets[0], fft.DIF, fft.OnCoset())
	domain.FFT(onCosets[1], fft.DIF, fft.OnCoset())
	domain.FFT(onCosets[2], fft.DIF, fft.OnCoset())
	fft.BitReverse(onCosets[0])
	fft.BitReverse(onCosets[1])
	fft.BitReverse(onCosets[2])

	yOnCosets := fastpoly.BatchInterpolate(onCosets, x, true)
	require.Equal(t, expectedY.String(), yOnCosets[0].String())
	require.Equal(t, expectedY2.String(), yOnCosets[1].String())
	require.Equal(t, expectedY3.String(), yOnCosets[2].String())

}
