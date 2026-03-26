package smartvectorsext

import (
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/vectorext"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field/fext"
	"testing"

	"github.com/stretchr/testify/require"
)

// This is a simple error case we have faces in the past, the test ensures that
// it does go through.
func TestProcessWindowed(_ *testing.T) {

	a := NewPaddedCircularWindowExt(
		vectorext.Rand(5),
		fext.Zero(),
		0,
		16,
	)

	b := NewPaddedCircularWindowExt(
		vectorext.Rand(12),
		fext.Zero(),
		4,
		16,
	)

	_, _ = processWindowedOnly(
		linCombOp{},
		[]smartvectors.SmartVector{b, a},
		[]int{1, 1},
	)
}

func TestEdgeCases(t *testing.T) {
	require.PanicsWithValue(t, "zero length subvector is forbidden", func() {
		NewPaddedCircularWindowExt(
			vectorext.Rand(5),
			fext.Zero(),
			0,
			16,
		).SubVector(0, 0)
	},
		"SubVector should panic with 'zero length subvector is forbidden' message")
	require.PanicsWithValue(t, "Subvector of zero lengths are not allowed", func() {
		NewRegularExt([]fext.Element{fext.Zero()}).SubVector(0, 0)
	},
		"SubVector should panic with 'Subvector of zero lengths are not allowed' message")
	require.PanicsWithValue(t, "zero or negative length are not allowed", func() {
		NewConstantExt(fext.Zero(), 0)
	},
		"NewConstant should panic with 'zero or negative length are not allowed' message")
	require.PanicsWithValue(t, "zero or negative length are not allowed", func() {
		NewConstantExt(fext.Zero(), -1)
	},
		"NewConstant should panic with 'zero or negative length are not allowed' message")
	require.PanicsWithValue(t, "negative length are not allowed", func() {
		NewConstantExt(fext.Zero(), 10).SubVector(3, 1)
	},
		"NewConstant.Subvector should panic with 'negative length are not allowed' message")
	require.PanicsWithValue(t, "zero length are not allowed", func() {
		NewConstantExt(fext.Zero(), 10).SubVector(3, 3)
	},
		"NewConstant.Subvector should panic with 'zero length are not allowed' message")
}
