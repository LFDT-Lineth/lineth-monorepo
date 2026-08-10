package wiop_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newGrandProductSystem builds a 2-round system with a single sized module and
// two oracle columns (numerator and denominator sources) committed in r0. The
// GrandProduct result lands in r1.
func newGrandProductSystem(t *testing.T) (*wiop.System, *wiop.Column, *wiop.Column) {
	t.Helper()
	sys := wiop.NewSystemf("gpSys")
	r0 := sys.NewRound()
	sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("gpMod"), 4, wiop.PaddingDirectionNone)
	num := mod.NewColumn(sys.Context.Childf("num"), r0)
	den := mod.NewColumn(sys.Context.Childf("den"), r0)
	return sys, num, den
}

// TestGrandProduct_Compute_IsOne: ∏num / ∏den with the same multiset on both
// sides evaluates to one and SelfAssign + Check accept it.
func TestGrandProduct_Compute_IsOne(t *testing.T) {
	sys, num, den := newGrandProductSystem(t)
	gp := sys.NewGrandProduct(
		sys.Context.Childf("gp"),
		[]wiop.Expression{num.View()},
		[]wiop.Expression{den.View()},
	)
	require.NotNil(t, gp)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(num, makeVecU64(2, 3, 4, 5))
	rt.AssignColumn(den, makeVecU64(5, 4, 3, 2))
	rt.AdvanceRound()

	assert.False(t, gp.IsAlreadyAssigned(rt))
	gp.SelfAssign(rt)
	assert.True(t, gp.IsAlreadyAssigned(rt))
	require.NoError(t, gp.Check(rt))
	assert.Equal(t, sys.Rounds[1], gp.Round())
}

// TestGrandProduct_Compute_NonOne: an all-ones denominator yields ∏num, a
// value distinct from one, which the prover-supplied Result must match.
func TestGrandProduct_Compute_NonOne(t *testing.T) {
	sys, num, den := newGrandProductSystem(t)
	gp := sys.NewGrandProduct(
		sys.Context.Childf("gp"),
		[]wiop.Expression{num.View()},
		[]wiop.Expression{den.View()},
	)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(num, makeVecU64(2, 2, 2, 2)) // ∏ = 16
	rt.AssignColumn(den, makeVecU64(1, 1, 1, 1)) // ∏ = 1
	rt.AdvanceRound()

	gp.SelfAssign(rt)
	require.NoError(t, gp.Check(rt))
}

// TestGrandProduct_Check_Mismatch: a Result cell pinned to the wrong value is
// rejected by Check.
func TestGrandProduct_Check_Mismatch(t *testing.T) {
	sys, num, den := newGrandProductSystem(t)
	gp := sys.NewGrandProduct(
		sys.Context.Childf("gp"),
		[]wiop.Expression{num.View()},
		[]wiop.Expression{den.View()},
	)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(num, makeVecU64(2, 2, 2, 2))
	rt.AssignColumn(den, makeVecU64(2, 2, 2, 2))
	rt.AdvanceRound()
	rt.AssignCell(gp.Result, field.ElemFromBase(field.NewFromString("99")))

	assert.Error(t, gp.Check(rt))
}

// TestGrandProduct_EmptyNumerator: an empty numerator side is the constant-one
// product, so the value is 1 / ∏den.
func TestGrandProduct_EmptyNumerator(t *testing.T) {
	sys, num, den := newGrandProductSystem(t)
	gp := sys.NewGrandProduct(
		sys.Context.Childf("gp"),
		nil,
		[]wiop.Expression{den.View()},
	)

	rt := wiop.NewRuntime(sys)
	// num is unused by this query but exists in r0, so it must still be assigned
	// before AdvanceRound folds the round into the transcript.
	rt.AssignColumn(num, makeVecU64(1, 1, 1, 1))
	rt.AssignColumn(den, makeVecU64(1, 1, 1, 1)) // ∏ = 1 ⇒ value = 1
	rt.AdvanceRound()
	gp.SelfAssign(rt)
	require.NoError(t, gp.Check(rt))
}

func TestNewGrandProduct_NilCtxPanic(t *testing.T) {
	sys, num, den := newGrandProductSystem(t)
	assert.Panics(t, func() {
		sys.NewGrandProduct(nil, []wiop.Expression{num.View()}, []wiop.Expression{den.View()})
	})
}

func TestNewGrandProduct_NoFactorsPanic(t *testing.T) {
	sys, _, _ := newGrandProductSystem(t)
	assert.Panics(t, func() {
		sys.NewGrandProduct(sys.Context.Childf("gp"), nil, nil)
	})
}

func TestNewGrandProduct_NilFactorPanic(t *testing.T) {
	sys, _, den := newGrandProductSystem(t)
	assert.Panics(t, func() {
		sys.NewGrandProduct(sys.Context.Childf("gp"), []wiop.Expression{nil}, []wiop.Expression{den.View()})
	})
}

func TestNewGrandProduct_ScalarFactorPanic(t *testing.T) {
	sys, _, den := newGrandProductSystem(t)
	scalar := wiop.NewConstantField(field.NewFromString("3"))
	assert.Panics(t, func() {
		sys.NewGrandProduct(sys.Context.Childf("gp"), []wiop.Expression{scalar}, []wiop.Expression{den.View()})
	})
}

// TestNewGrandProduct_NoRoundBearingPanic: every factor is a constant vector
// (vector-valued but round-free), so no result round can be chosen.
func TestNewGrandProduct_NoRoundBearingPanic(t *testing.T) {
	sys := wiop.NewSystemf("gpNoRound")
	sys.NewRound()
	sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("m"), 4, wiop.PaddingDirectionNone)
	num := wiop.NewConstantVector(mod, field.NewFromString("2"))
	den := wiop.NewConstantVector(mod, field.NewFromString("1"))
	assert.Panics(t, func() {
		sys.NewGrandProduct(sys.Context.Childf("gp"), []wiop.Expression{num}, []wiop.Expression{den})
	})
}

// TestNewGrandProduct_NoNextRoundPanic: factors live in the only round, so
// there is no round to hold the Result cell.
func TestNewGrandProduct_NoNextRoundPanic(t *testing.T) {
	sys := wiop.NewSystemf("gpNoNext")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("m"), 4, wiop.PaddingDirectionNone)
	num := mod.NewColumn(sys.Context.Childf("num"), r0)
	den := mod.NewColumn(sys.Context.Childf("den"), r0)
	assert.Panics(t, func() {
		sys.NewGrandProduct(sys.Context.Childf("gp"), []wiop.Expression{num.View()}, []wiop.Expression{den.View()})
	})
}
