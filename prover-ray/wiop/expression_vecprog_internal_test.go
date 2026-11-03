package wiop

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/internal/vecprog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sharedDoubling returns x_depth where x_0 = leaf and x_{i+1} = x_i + x_i,
// one pointer per level: depth+1 distinct nodes, but a tree of 2^depth
// leaves.
func sharedDoubling(leaf Expression, depth int) Expression {
	x := leaf
	for range depth {
		x = Add(x, x)
	}
	return x
}

// Lowering a depth-64 shared DAG must visit each distinct node once: a walk
// re-descending into shared operands would not terminate. The program it
// builds must still compute 2^64 · col.
func TestRowLowering_IsLinearOnDAGs(t *testing.T) {
	const n = 8
	sys := NewSystemf("row-lowering-dag")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), n, PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), r0)

	rt := NewRuntime(sys)
	vals := make([]field.Element, n)
	for i := range vals {
		vals[i].SetUint64(uint64(i + 1))
	}
	rt.AssignColumn(col, &ConcreteVector{Plain: field.VecFromBase(vals)})

	l := NewRowLowering(rt, n)
	got := make([]field.Ext, n)
	l.B.Store(l.Lower(sharedDoubling(col.View(), 64)), got)
	l.B.Compile(n, 1, 1).Run(nil, nil, 1)

	var pow field.Element // 2^64
	pow.SetOne()
	for range 64 {
		pow.Double(&pow)
	}
	for i := range n {
		var want field.Element
		want.Mul(&vals[i], &pow)
		assert.Equal(t, field.Lift(want), got[i], "row %d", i)
	}
}

// A subexpression shared between the expressions of one program -- the
// vanishings of a quotient bucket -- must be lowered once, not once per
// expression: the second Lower only visits the node it adds.
func TestExpressionLowering_SharesAcrossCalls(t *testing.T) {
	sys := NewSystemf("lowering-across-calls")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 8, PaddingDirectionNone)
	a := mod.NewColumn(sys.Context.Childf("a"), r0)
	c := mod.NewColumn(sys.Context.Childf("c"), r0)

	b := vecprog.NewBuilder()
	table := make([]field.Element, 8)
	leaves := 0
	l := NewExpressionLowering(nil, b, func(Expression) int {
		leaves++
		return b.Base(table, leaves)
	})

	// Operations are memoised, leaves are not: shared reaches two leaves.
	shared := Mul(Square(a.View()), a.View())
	first := l.Lower(Add(shared, shared))
	require.Equal(t, 2, leaves, "a shared node is lowered once within an expression")

	second := l.Lower(Sub(shared, c.View()))
	assert.Equal(t, 3, leaves, "the second expression must only lower its new leaf")
	assert.NotEqual(t, first, second)
}
