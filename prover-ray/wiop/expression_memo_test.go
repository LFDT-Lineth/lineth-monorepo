package wiop_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// doublingDAG returns x_depth where x_0 = leaf and x_{i+1} = x_i + x_i, built
// with a single pointer per level: depth distinct nodes, but a tree of
// 2^depth leaves. Any walk that re-descends into shared operands does not
// terminate in practice at depth 64.
func doublingDAG(leaf wiop.Expression, depth int) wiop.Expression {
	x := leaf
	for range depth {
		x = wiop.Add(x, x)
	}
	return x
}

func TestEvaluateSingle_IsLinearOnDAGs(t *testing.T) {
	const depth = 64
	x := doublingDAG(wiop.NewConstantField(field.NewElement(3)), depth)

	// x_64 = 3 · 2^64.
	var want, two field.Element
	want.SetUint64(3)
	two.SetUint64(2)
	for range depth {
		want.Mul(&want, &two)
	}
	got := x.EvaluateSingle(nil).Value
	require.True(t, got.IsBase())
	assert.Equal(t, want, got.AsBase())

	// The compiled program is reused across calls.
	assert.Equal(t, got, x.EvaluateSingle(nil).Value)
}

func TestMetadata_IsLinearOnDAGs(t *testing.T) {
	sys := wiop.NewSystemf("metadata-memo")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 8, wiop.PaddingDirectionNone)
	col := mod.NewExtensionColumn(sys.Context.Childf("col"), r0)

	x := doublingDAG(col.View(), 64)
	assert.True(t, x.IsExtension())
	assert.Equal(t, 1, x.DegreeFactor())

	sq := doublingDAG(wiop.Square(col.View()), 64)
	assert.Equal(t, 2, sq.DegreeFactor())
}

// A DegreeFactor call that panics must not leave a cached value behind: a
// later call has to panic again rather than return a silent zero.
func TestDegreeFactor_PanicIsNotCached(t *testing.T) {
	sys := wiop.NewSystemf("degree-factor-panic")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 8, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), r0)

	div := wiop.Div(col.View(), col.View())
	assert.Panics(t, func() { div.DegreeFactor() })
	assert.Panics(t, func() { div.DegreeFactor() })
}
