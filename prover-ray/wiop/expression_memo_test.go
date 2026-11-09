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
	require.True(t, got.IsBase(), "a sum of base constants must stay in the base field")
	assert.Equal(t, want, got.AsBase(), "x_64 must equal 3·2^64")

	// The compiled program is reused across calls.
	assert.Equal(t, got, x.EvaluateSingle(nil).Value, "a second EvaluateSingle on the compiled program must give the same value")
}

func TestMetadata_IsLinearOnDAGs(t *testing.T) {
	sys := wiop.NewSystemf("metadata-memo")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 8, wiop.PaddingDirectionNone)
	col := mod.NewExtensionColumn(sys.Context.Childf("col"), r0)

	x := doublingDAG(col.View(), 64)
	assert.True(t, x.IsExtension(), "a DAG over an extension column must be an extension")
	assert.Equal(t, 1, x.DegreeFactor(), "sums of a degree-1 column keep degree factor 1")

	sq := doublingDAG(wiop.Square(col.View()), 64)
	assert.Equal(t, 2, sq.DegreeFactor(), "sums of a squared column keep degree factor 2")
}

// A DegreeFactor call that panics must not leave a cached value behind: a
// later call has to panic again rather than return a silent zero.
func TestDegreeFactor_PanicIsNotCached(t *testing.T) {
	sys := wiop.NewSystemf("degree-factor-panic")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 8, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), r0)

	div := wiop.Div(col.View(), col.View())
	assert.Panics(t, func() { div.DegreeFactor() }, "DegreeFactor of a Div must panic")
	assert.Panics(t, func() { div.DegreeFactor() }, "a second DegreeFactor of a Div must panic again, not return a cached zero")
}

// Degree and IsSized are not cached on the node, but each call visits a
// distinct node once.
func TestDegreeAndIsSized_AreLinearOnDAGs(t *testing.T) {
	sys := wiop.NewSystemf("degree-sized-dag")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 8, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), r0)

	x := doublingDAG(col.View(), 64)
	assert.Equal(t, col.View().Degree(), x.Degree(), "sums of a column keep the column's degree")
	assert.True(t, x.IsSized(), "a DAG over a sized column must be sized")

	sq := doublingDAG(wiop.Mul(col.View(), col.View()), 64)
	assert.Equal(t, 2*col.View().Degree(), sq.Degree(), "sums of a squared column keep the squared degree")
}

// A chain of 64 squarings has degree factor 2^64, above math.MaxInt: it must
// panic instead of wrapping around, and keep panicking (nothing is cached).
func TestDegreeFactor_OverflowPanics(t *testing.T) {
	sys := wiop.NewSystemf("degree-factor-overflow")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 8, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), r0)

	var x wiop.Expression = col.View()
	for range 64 {
		x = wiop.Mul(x, x)
	}
	assert.Panics(t, func() { x.DegreeFactor() }, "a degree factor of 2^64 must panic, not wrap")
	assert.Panics(t, func() { x.DegreeFactor() }, "an overflowing DegreeFactor must not be cached")
	assert.Panics(t, func() { x.Degree() }, "a degree above math.MaxInt must panic, not wrap")
}
