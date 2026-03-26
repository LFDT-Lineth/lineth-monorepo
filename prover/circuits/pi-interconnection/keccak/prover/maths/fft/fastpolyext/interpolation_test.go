package fastpolyext

import (
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/polyext"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/vectorext"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field/fext"
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/fft"
	"github.com/stretchr/testify/require"
)

func TestInterpolation(t *testing.T) {
	n := 4
	randPoly := vectorext.ForTest(1, 2, 3, 4)
	x := fext.NewElement(51, 0)
	expectedY := polyext.EvalUnivariate(randPoly, x)
	domain := fft.NewDomain(n).WithCoset()

	/*
		Test without coset
	*/
	onRoots := vectorext.DeepCopy(randPoly)
	domain.FFTExt(onRoots, fft.DIF)

	fft.BitReverseExt(onRoots)
	yOnRoots := Interpolate(onRoots, x)
	require.Equal(t, expectedY.String(), yOnRoots.String())

	/*
		Test with coset
	*/
	onCoset := vectorext.DeepCopy(randPoly)
	domain.FFTExt(onCoset, fft.DIF, fft.OnCoset())
	fft.BitReverseExt(onCoset)
	yOnCoset := Interpolate(onCoset, x, true)
	require.Equal(t, expectedY.String(), yOnCoset.String())

}

func TestBatchInterpolation(t *testing.T) {
	n := 4
	randPoly := vectorext.ForTest(1, 2, 3, 4)
	randPoly2 := vectorext.ForTest(5, 6, 7, 8)
	x := fext.NewElement(51, 0)

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

	yOnRoots := BatchInterpolate(polys, x)
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

	yOnCosets := BatchInterpolate(onCosets, x, true)
	require.Equal(t, expectedY.String(), yOnCosets[0].String())
	require.Equal(t, expectedY2.String(), yOnCosets[1].String())

}

// edge-case : x is a root of unity of the domain. In this case, we can just return
// the associated value for poly
func TestBatchInterpolationRootOfUnity(t *testing.T) {
	n := 4
	randPoly := vectorext.ForTest(1, 2, 3, 4)
	randPoly2 := vectorext.ForTest(5, 6, 7, 8)

	// define x as a root of unity
	x := fext.One()

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

	yOnRoots := BatchInterpolate(polys, x)
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

	yOnCosets := BatchInterpolate(onCosets, x, true)
	require.Equal(t, expectedY.String(), yOnCosets[0].String())
	require.Equal(t, expectedY2.String(), yOnCosets[1].String())

}
