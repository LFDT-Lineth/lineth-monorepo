package wiop_test

import (
	"testing"
	"time"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// operand returns the i-th operand of the ArithmeticOperation e.
func operand(t *testing.T, e wiop.Expression, i int) wiop.Expression {
	t.Helper()
	op, ok := e.(*wiop.ArithmeticOperation)
	require.Truef(t, ok, "expected an ArithmeticOperation, got %T", e)
	return op.Operands[i]
}

// Two vanishings built from separate allocations of a·b share one a·b node.
func TestNewVanishing_SharesEqualSubexpressionsAcrossVanishings(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	a := mod.NewColumn(sys.Context.Childf("a"), r0)
	b := mod.NewColumn(sys.Context.Childf("b"), r0)
	c := mod.NewColumn(sys.Context.Childf("c"), r0)

	v1 := mod.NewVanishing(sys.Context.Childf("v1"), wiop.Sub(wiop.Mul(a.View(), b.View()), c.View()))
	v2 := mod.NewVanishing(sys.Context.Childf("v2"), wiop.Add(wiop.Mul(a.View(), b.View()), c.View()))

	ab1, ab2 := operand(t, v1.Expression, 0), operand(t, v2.Expression, 0)
	assert.Same(t, ab1, ab2, "a·b must be one node across both vanishings")
	assert.Same(t, operand(t, v1.Expression, 1), operand(t, v2.Expression, 1), "c must be one leaf")
}

// Repeated subtrees of one vanishing collapse onto one node.
func TestNewVanishing_SharesEqualSubexpressionsWithinVanishing(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	a := mod.NewColumn(sys.Context.Childf("a"), r0)
	coin := r0.NewCoinField(sys.Context.Childf("coin"))

	// (a + coin)·(a + coin) - a, the two factors allocated separately.
	expr := wiop.Sub(
		wiop.Mul(wiop.Add(a.View(), coin), wiop.Add(a.View(), coin)),
		a.View(),
	)
	v := mod.NewVanishing(sys.Context.Childf("v"), expr)

	prod := operand(t, v.Expression, 0)
	assert.Same(t, operand(t, prod, 0), operand(t, prod, 1), "the two a + coin factors must be one node")
	assert.Same(t, operand(t, operand(t, prod, 0), 0), operand(t, v.Expression, 1), "a must be one leaf")
}

// An already canonical expression is stored as passed.
func TestNewVanishing_KeepsCanonicalExpression(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	a := mod.NewColumn(sys.Context.Childf("a"), r0)
	b := mod.NewColumn(sys.Context.Childf("b"), r0)

	ab := wiop.Mul(a.View(), b.View())
	expr := wiop.Sub(ab, ab)
	v := mod.NewVanishing(sys.Context.Childf("v"), expr)
	assert.Same(t, expr, v.Expression)

	// A later vanishing reusing the same pointers keeps them too.
	expr2 := wiop.Add(ab, a.View())
	v2 := mod.NewVanishing(sys.Context.Childf("v2"), expr2)
	assert.Same(t, ab, operand(t, v2.Expression, 0))
}

// A new node on a non-canonical operand is rebuilt on the canonical one, and
// the caller's expression is left untouched.
func TestNewVanishing_RebuildsOnCanonicalOperands(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	a := mod.NewColumn(sys.Context.Childf("a"), r0)
	b := mod.NewColumn(sys.Context.Childf("b"), r0)

	first := a.View()
	mod.NewVanishing(sys.Context.Childf("v1"), first)

	second := a.View()
	expr := wiop.Mul(second, b.View())
	v := mod.NewVanishing(sys.Context.Childf("v2"), expr)

	assert.NotSame(t, expr, v.Expression, "the node must be rebuilt on the canonical leaf")
	assert.Same(t, first, operand(t, v.Expression, 0))
	assert.Same(t, second, operand(t, expr, 0), "the caller's expression must not be modified")
	assert.Equal(t, wiop.ArithmeticOperatorMul, v.Expression.(*wiop.ArithmeticOperation).Operator)
}

// Look-alike nodes that may differ are never merged.
func TestNewVanishing_KeepsDistinctValuesApart(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	a := mod.NewColumn(sys.Context.Childf("a"), r0)
	b := mod.NewColumn(sys.Context.Childf("b"), r0)
	other := sys.NewSizedModule(sys.Context.Childf("other"), 4, wiop.PaddingDirectionNone)
	one := field.One()

	cases := []struct {
		name string
		x, y wiop.Expression
	}{
		{"different shifts", a.View(), a.View().Shift(1)},
		{"different columns", a.View(), b.View()},
		{"different positions", a.At(0), a.At(1)},
		{"different constants", wiop.NewConstantField(one), wiop.NewConstantField(field.NewElement(2))},
		{"vector and scalar constant", wiop.NewConstantVector(mod, one), wiop.NewConstantField(one)},
		{"constants on different modules", wiop.NewConstantVector(mod, one), wiop.NewConstantVector(other, one)},
		{"different selectors", wiop.NewLagrangeSelector(mod, 0), wiop.NewLagrangeSelector(mod, 1)},
		{"selectors allocated separately", wiop.NewLagrangeSelector(mod, 2), wiop.NewLagrangeSelector(mod, 2)},
		{"different operators", wiop.Add(a.View(), b.View()), wiop.Sub(a.View(), b.View())},
		{"swapped operands", wiop.Sub(a.View(), b.View()), wiop.Sub(b.View(), a.View())},
		{"unary and binary", wiop.Negate(a.View()), wiop.Add(a.View(), a.View())},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := mod.NewVanishingManual(sys.Context.Childf("v%d", i), wiop.Sub(tc.x, tc.y))
			assert.NotSame(t, operand(t, v.Expression, 0), operand(t, v.Expression, 1))
		})
	}
}

// Separately allocated leaves of equal value are merged.
func TestNewVanishing_SharesEqualLeaves(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	a := mod.NewColumn(sys.Context.Childf("a"), r0)
	one := field.One()

	cases := []struct {
		name string
		x, y wiop.Expression
	}{
		{"column view", a.View().Shift(-1), a.View().Shift(-1)},
		{"column position", a.At(2), a.At(2)},
		{"scalar constant", wiop.NewConstantField(one), wiop.NewConstantField(one)},
		{"vector constant", wiop.NewConstantVector(mod, one), wiop.NewConstantVector(mod, one)},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.NotSame(t, tc.x, tc.y)
			v := mod.NewVanishingManual(sys.Context.Childf("v%d", i), wiop.Mul(tc.x, tc.y))
			assert.Same(t, operand(t, v.Expression, 0), operand(t, v.Expression, 1))
		})
	}
}

// x_{k+1} = x_k + x_k over 64 levels is 65 nodes but 2^65 - 1 as a tree:
// registering it only finishes if each distinct node is visited once.
func TestNewVanishing_InterningIsLinearOnDAGs(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	x := wiop.Expression(mod.NewColumn(sys.Context.Childf("x"), r0).View())
	for range 64 {
		x = wiop.Add(x, x)
	}

	done := make(chan *wiop.Vanishing, 1)
	go func() { done <- mod.NewVanishing(sys.Context.Childf("deep"), x) }()
	select {
	case v := <-done:
		assert.Same(t, x, v.Expression)
	case <-time.After(10 * time.Second):
		t.Fatal("interning a depth-64 DAG did not finish: it is walking the tree, not the DAG")
	}
}

// Equal subexpressions on two modules are not shared.
func TestNewVanishing_InterningIsPerModule(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	other := sys.NewSizedModule(sys.Context.Childf("other"), 4, wiop.PaddingDirectionNone)
	coin := r0.NewCoinField(sys.Context.Childf("coin"))
	a := mod.NewColumn(sys.Context.Childf("a"), r0)
	b := other.NewColumn(sys.Context.Childf("b"), r0)

	v1 := mod.NewVanishing(sys.Context.Childf("v1"), wiop.Mul(wiop.Square(coin), a.View()))
	v2 := other.NewVanishing(sys.Context.Childf("v2"), wiop.Mul(wiop.Square(coin), b.View()))
	assert.NotSame(t, operand(t, v1.Expression, 0), operand(t, v2.Expression, 0))
}
