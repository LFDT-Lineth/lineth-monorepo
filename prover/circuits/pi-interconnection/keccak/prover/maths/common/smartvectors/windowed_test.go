package smartvectors

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/stretchr/testify/require"
)

// This is a simple error case we have faces in the past, the test ensures that
// it does go through.
func TestProcessWindowed(_ *testing.T) {

	a := NewPaddedCircularWindow(
		vector.Rand(5),
		field.Zero(),
		0,
		16,
	)

	b := NewPaddedCircularWindow(
		vector.Rand(12),
		field.Zero(),
		4,
		16,
	)

	_, _ = processWindowedOnly(
		linCombOp{},
		[]SmartVector{b, a},
		[]int{1, 1},
	)
}

func TestEdgeCases(t *testing.T) {
	require.PanicsWithValue(t, "zero length subvector is forbidden", func() {
		NewPaddedCircularWindow(
			vector.Rand(5),
			field.Zero(),
			0,
			16,
		).SubVector(0, 0)
	},
		"SubVector should panic with 'zero length subvector is forbidden' message")
	require.PanicsWithValue(t, "Subvector of zero lengths are not allowed", func() {
		NewRegular([]field.Element{field.Zero()}).SubVector(0, 0)
	},
		"SubVector should panic with 'Subvector of zero lengths are not allowed' message")
	require.PanicsWithValue(t, "zero or negative length are not allowed", func() {
		NewConstant(field.Zero(), 0)
	},
		"NewConstant should panic with 'zero or negative length are not allowed' message")
	require.PanicsWithValue(t, "zero or negative length are not allowed", func() {
		NewConstant(field.Zero(), -1)
	},
		"NewConstant should panic with 'zero or negative length are not allowed' message")
	require.PanicsWithValue(t, "negative length are not allowed", func() {
		NewConstant(field.Zero(), 10).SubVector(3, 1)
	},
		"NewConstant.Subvector should panic with 'negative length are not allowed' message")
	require.PanicsWithValue(t, "zero length are not allowed", func() {
		NewConstant(field.Zero(), 10).SubVector(3, 3)
	},
		"NewConstant.Subvector should panic with 'zero length are not allowed' message")
}
