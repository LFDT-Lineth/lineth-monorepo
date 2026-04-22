package reedsolomon

import (
	"strconv"
	"testing"

	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/polynomials"
	"github.com/stretchr/testify/require"
)

// Evaluating a polynomial or its LDE yields the same result
func TestReedSolomonDoesNotChangeEvaluation(t *testing.T) {

	for _, rate := range []int{2, 4, 8} {
		t.Run("rate="+strconv.Itoa(rate), func(t *testing.T) {

			var (
				polySize  = 1 << 10
				x         = field.RandomElemExt()
				params    = NewRsParams(polySize, rate)
				vec       = field.VecRandomBase(1 << 10)
				rsEncoded = params.RsEncodeBase(vec)
				err       = params.IsCodeword(rsEncoded)
			)

			require.NoError(t, err)

			y0 := polynomials.EvalLagrange(field.VecFromBase(vec), x)
			y1 := polynomials.EvalLagrange(field.VecFromBase(rsEncoded), x)

			require.True(t, y0.Equal(&y1.Ext))
		})
	}
}

// Evaluating and testing for constants
func TestReedSolomonConstant(t *testing.T) {

	var (
		polySize     = 1 << 10
		blowUpFactor = 2
		x            = field.RandomElemExt()
		params       = NewRsParams(polySize, blowUpFactor)
		vec          = field.VecRepeatBase(field.NewElement(42), polySize)
		rsEncoded    = params.RsEncodeBase(vec)
		err          = params.IsCodeword(rsEncoded)
	)

	require.NoError(t, err)

	y0 := polynomials.EvalLagrange(field.VecFromBase(vec), x)
	y1 := polynomials.EvalLagrange(field.VecFromBase(rsEncoded), x)

	require.Equal(t, y0.String(), y1.String())
}

// Evaluating a polynomial or its LDE yields the same result
func TestReedSolomonExtDoesNotChangeEvaluation(t *testing.T) {

	var (
		polySize      = 1 << 10
		_blowUpFactor = 2
		x             = field.RandomElemBase()
		params        = NewRsParams(polySize, _blowUpFactor)
		vec           = field.VecRandomExt(1 << 10)
		rsEncoded     = params.rsEncodeExt(vec)
		err           = params.IsCodewordExt(rsEncoded)
	)

	require.NoError(t, err)

	y0 := polynomials.EvalLagrange(field.VecFromExt(vec), x)
	y1 := polynomials.EvalLagrange(field.VecFromExt(rsEncoded), x)

	require.Equal(t, y0.String(), y1.String())
}

// Evaluating and testing for constants
func TestReedSolomonExtConstant(t *testing.T) {

	var (
		polySize      = 1 << 10
		_blowUpFactor = 2
		x             = field.RandomElemExt()
		params        = NewRsParams(polySize, _blowUpFactor)
		vec           = field.VecRepeatExt(field.RandomElementExt(), polySize)
		rsEncoded     = params.rsEncodeExt(vec)
		err           = params.IsCodewordExt(rsEncoded)
	)

	require.NoError(t, err)

	y0 := polynomials.EvalLagrange(field.VecFromExt(vec), x)
	y1 := polynomials.EvalLagrange(field.VecFromExt(rsEncoded), x)

	require.Equal(t, y0.String(), y1.String())
}
