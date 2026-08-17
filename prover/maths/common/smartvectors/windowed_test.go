package smartvectors

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/field"
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

// TestCoWindowRange checks that the range covers the union of every window,
// independently of argument order, and that constants are skipped.
func TestCoWindowRange(t *testing.T) {
	const size = 16
	a := NewPaddedCircularWindow(vector.Rand(3), field.Zero(), 2, size) // [2, 5)
	b := NewPaddedCircularWindow(vector.Rand(4), field.Zero(), 8, size) // [8, 12)
	c := NewConstant(field.Zero(), size)

	start, stop := CoWindowRange(a, b)
	require.Equal(t, 2, start)
	require.Equal(t, 12, stop)

	start, stop = CoWindowRange(b, a)
	require.Equal(t, 2, start)
	require.Equal(t, 12, stop)

	start, stop = CoWindowRange(c, a, c)
	require.Equal(t, 2, start)
	require.Equal(t, 5, stop)

	start, stop = CoWindowRange(NewRegular(vector.Rand(size)), a)
	require.Equal(t, 0, start)
	require.Equal(t, size, stop)
}
