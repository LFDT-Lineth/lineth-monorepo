package vortex

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/ringsis"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/stretchr/testify/require"
)

// Evaluating a polynomial or its LDE yields the same result
func TestReedSolomonDoesNotChangeEvaluation(t *testing.T) {

	polySize := 1 << 10
	_nPolys := 15
	_blowUpFactor := 2

	x := field.NewElement(478)

	params := NewParams(_blowUpFactor, polySize, _nPolys, ringsis.StdParams)
	vec := smartvectors.Rand(1 << 10)
	rsEncoded := params.rsEncode(vec)

	err := params.isCodeword(rsEncoded)
	require.NoError(t, err)

	y0 := smartvectors.Interpolate(vec, x)
	y1 := smartvectors.Interpolate(rsEncoded, x)

	require.Equal(t, y0.String(), y1.String())
}

// Evaluating and testing for constants
func TestReedSolomonConstant(t *testing.T) {

	polySize := 1 << 10
	_nPolys := 15
	_blowUpFactor := 2

	x := field.NewElement(478)

	params := NewParams(_blowUpFactor, polySize, _nPolys, ringsis.StdParams)
	vec := smartvectors.NewConstant(field.NewElement(42), polySize)
	rsEncoded := params.rsEncode(vec)

	err := params.isCodeword(rsEncoded)
	require.NoError(t, err)

	y0 := smartvectors.Interpolate(vec, x)
	y1 := smartvectors.Interpolate(rsEncoded, x)

	require.Equal(t, y0.String(), y1.String())

}
