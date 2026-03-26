package reedsolomon

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/stretchr/testify/require"
)

// Evaluating a polynomial or its LDE yields the same result
func TestReedSolomonDoesNotChangeEvaluation(t *testing.T) {

	// rate = 2
	{
		polySize := 1 << 10
		rate := 2

		x := fext.RandomElement()

		params := NewRsParams(polySize, rate)
		vec := smartvectors.Rand(1 << 10)
		rsEncoded := params.RsEncodeBase(vec)

		err := params.IsCodeword(rsEncoded)
		require.NoError(t, err)

		y0 := smartvectors.EvaluateBasePolyLagrange(vec, x)
		y1 := smartvectors.EvaluateBasePolyLagrange(rsEncoded, x)

		require.True(t, y0.B0.A0.Equal(&y1.B0.A0))
		require.True(t, y0.B0.A1.Equal(&y1.B0.A1))
		require.True(t, y0.B1.A0.Equal(&y1.B1.A0))
		require.True(t, y0.B1.A1.Equal(&y1.B1.A1))
	}

	// rate = 4
	{
		polySize := 1 << 10
		rate := 4

		x := fext.RandomElement()

		params := NewRsParams(polySize, rate)
		vec := smartvectors.Rand(1 << 10)
		rsEncoded := params.RsEncodeBase(vec)

		err := params.IsCodeword(rsEncoded)
		require.NoError(t, err)

		y0 := smartvectors.EvaluateBasePolyLagrange(vec, x)
		y1 := smartvectors.EvaluateBasePolyLagrange(rsEncoded, x)

		require.True(t, y0.B0.A0.Equal(&y1.B0.A0))
		require.True(t, y0.B0.A1.Equal(&y1.B0.A1))
		require.True(t, y0.B1.A0.Equal(&y1.B1.A0))
		require.True(t, y0.B1.A1.Equal(&y1.B1.A1))
	}
}

// Evaluating and testing for constants
func TestReedSolomonConstant(t *testing.T) {

	polySize := 1 << 10
	_blowUpFactor := 2

	x := fext.RandomElement()

	params := NewRsParams(_blowUpFactor, polySize)
	vec := smartvectors.NewConstant(field.NewElement(42), polySize)
	rsEncoded := params.RsEncodeBase(vec)

	err := params.IsCodeword(rsEncoded)
	require.NoError(t, err)

	y0 := smartvectors.EvaluateBasePolyLagrange(vec, x)
	y1 := smartvectors.EvaluateBasePolyLagrange(rsEncoded, x)

	require.Equal(t, y0.String(), y1.String())

}
