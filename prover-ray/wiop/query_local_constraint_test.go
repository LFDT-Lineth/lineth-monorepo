package wiop_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/linea-monorepo/prover-ray/wiop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// indexedBaseVec returns a length-n ConcreteVector whose i-th entry is the
// uint64 returned by f(i).
func indexedBaseVec(n int, f func(i int) uint64) *wiop.ConcreteVector {
	elems := make([]field.Element, n)
	for i := range n {
		elems[i].SetUint64(f(i))
	}
	return &wiop.ConcreteVector{Plain: field.VecFromBase(elems)}
}

// ---- the three supported positions ----

func TestLocalConstraint_Position0_PicksFirstRow(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("p0Col"), wiop.VisibilityOracle, r0)

	rt := wiop.NewRuntime(sys)
	// col = [0, 9, 9, 9]; position 0 reads col[0] = 0.
	rt.AssignColumn(col, indexedBaseVec(4, func(i int) uint64 {
		if i == 0 {
			return 0
		}
		return 9
	}))

	v := mod.NewLocalConstraint(sys.Context.Childf("p0"), col.View(), 0)
	require.NoError(t, v.Check(rt))
}

func TestLocalConstraint_Position1_PicksSecondRow(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("p1Col"), wiop.VisibilityOracle, r0)

	rt := wiop.NewRuntime(sys)
	// col = [9, 0, 9, 9]; position 1 reads col[1] = 0.
	rt.AssignColumn(col, indexedBaseVec(4, func(i int) uint64 {
		if i == 1 {
			return 0
		}
		return 9
	}))

	v := mod.NewLocalConstraint(sys.Context.Childf("p1"), col.View(), 1)
	require.NoError(t, v.Check(rt))
}

func TestLocalConstraint_PositionMinus1_PicksLastRow(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("pNeg1Col"), wiop.VisibilityOracle, r0)

	rt := wiop.NewRuntime(sys)
	// col = [9, 9, 9, 0] on a size-4 module; position -1 reads col[3] = 0.
	rt.AssignColumn(col, indexedBaseVec(4, func(i int) uint64 {
		if i == 3 {
			return 0
		}
		return 9
	}))

	v := mod.NewLocalConstraint(sys.Context.Childf("pNeg1"), col.View(), -1)
	require.NoError(t, v.Check(rt))
}

func TestLocalConstraint_Position0_FailsWhenFirstRowNonZero(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("p0FailCol"), wiop.VisibilityOracle, r0)

	rt := wiop.NewRuntime(sys)
	// col[0] = 7 (non-zero) ⇒ Check at position 0 must fail.
	rt.AssignColumn(col, baseVec(4, 7))

	v := mod.NewLocalConstraint(sys.Context.Childf("p0Fail"), col.View(), 0)
	assert.Error(t, v.Check(rt))
}

func TestLocalConstraint_Position1_FailsWhenSecondRowNonZero(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("p1FailCol"), wiop.VisibilityOracle, r0)

	rt := wiop.NewRuntime(sys)
	// col = [0, 7, 0, 0]; position 1 reads col[1] = 7 ⇒ fails.
	rt.AssignColumn(col, indexedBaseVec(4, func(i int) uint64 {
		if i == 1 {
			return 7
		}
		return 0
	}))

	v := mod.NewLocalConstraint(sys.Context.Childf("p1Fail"), col.View(), 1)
	assert.Error(t, v.Check(rt))
}

func TestLocalConstraint_PositionMinus1_FailsWhenLastRowNonZero(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("pNeg1FailCol"), wiop.VisibilityOracle, r0)

	rt := wiop.NewRuntime(sys)
	// col = [0, 0, 0, 7]; position -1 reads col[3] = 7 ⇒ fails.
	rt.AssignColumn(col, indexedBaseVec(4, func(i int) uint64 {
		if i == 3 {
			return 7
		}
		return 0
	}))

	v := mod.NewLocalConstraint(sys.Context.Childf("pNeg1Fail"), col.View(), -1)
	assert.Error(t, v.Check(rt))
}

// ---- composition with column shifts ----

func TestLocalConstraint_PositionComposesWithShift(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("shiftComp"), wiop.VisibilityOracle, r0)

	rt := wiop.NewRuntime(sys)
	// col = [9, 9, 0, 9]; col.View().Shift(1) at position 1 reads col[2] = 0.
	rt.AssignColumn(col, indexedBaseVec(4, func(i int) uint64 {
		if i == 2 {
			return 0
		}
		return 9
	}))

	v := mod.NewLocalConstraint(sys.Context.Childf("shiftComp"), col.View().Shift(1), 1)
	require.NoError(t, v.Check(rt))
}

// ---- arithmetic / non-column leaves ----

func TestLocalConstraint_ArithmeticOverTwoColumns(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	a := mod.NewColumn(sys.Context.Childf("lcA"), wiop.VisibilityOracle, r0)
	b := mod.NewColumn(sys.Context.Childf("lcB"), wiop.VisibilityOracle, r0)

	rt := wiop.NewRuntime(sys)
	// a[0] = b[0] = 5 ⇒ a - b vanishes at row 0.
	rt.AssignColumn(a, baseVec(4, 5))
	rt.AssignColumn(b, baseVec(4, 5))

	expr := wiop.Sub(a.View(), b.View())
	v := mod.NewLocalConstraint(sys.Context.Childf("lcArith"), expr, 0)
	require.NoError(t, v.Check(rt))
}

func TestLocalConstraint_ScalarOnly_CellExpression(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	cell := r0.NewCell(sys.Context.Childf("lcCell"), false)

	rt := wiop.NewRuntime(sys)
	rt.AssignCell(cell, field.ElemZero())

	// No columns at all; position is irrelevant but must still be valid.
	v := mod.NewLocalConstraint(sys.Context.Childf("lcScalar"), cell, 0)
	require.NoError(t, v.Check(rt))
}

func TestLocalConstraint_VectorConstantCollapsedToScalar(t *testing.T) {
	sys, _, _, mod := newTestSystem(t)
	c := wiop.NewConstantVector(mod, field.NewFromString("0"))

	rt := wiop.NewRuntime(sys)
	v := mod.NewLocalConstraint(sys.Context.Childf("lcVecK"), c, 0)
	require.NoError(t, v.Check(rt))
}

// ---- registration / metadata ----

func TestLocalConstraint_RegistersOnModuleVanishings(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("lcRegCol"), wiop.VisibilityOracle, r0)

	before := len(mod.Vanishings)
	v := mod.NewLocalConstraint(sys.Context.Childf("lcReg"), col.View(), 0)
	assert.Len(t, mod.Vanishings, before+1)
	assert.Same(t, v, mod.Vanishings[before])
	// "Local" constraints never auto-cancel any rows.
	assert.Empty(t, v.CancelledPositions)
}

func TestLocalConstraint_RoundFollowsColumn(t *testing.T) {
	sys, _, r1, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("lcRoundCol"), wiop.VisibilityOracle, r1)
	v := mod.NewLocalConstraint(sys.Context.Childf("lcRound"), col.View(), 0)
	assert.Equal(t, r1, v.Round())
}

// ---- panics ----

func TestLocalConstraint_NilCtx_Panics(t *testing.T) {
	_, _, _, mod := newTestSystem(t)
	k := wiop.NewConstantField(field.NewFromString("0"))
	assert.Panics(t, func() { mod.NewLocalConstraint(nil, k, 0) })
}

func TestLocalConstraint_NilExpr_Panics(t *testing.T) {
	sys, _, _, mod := newTestSystem(t)
	assert.Panics(t, func() { mod.NewLocalConstraint(sys.Context.Childf("nilExprLC"), nil, 0) })
}

func TestLocalConstraint_InvalidPosition_Panics(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("badPosCol"), wiop.VisibilityOracle, r0)
	for _, bad := range []int{-2, 2, 3, -3, 100} {
		bad := bad
		t.Run("", func(t *testing.T) {
			assert.Panics(t, func() {
				mod.NewLocalConstraint(sys.Context.Childf("badPos"), col.View(), bad)
			})
		})
	}
}

func TestLocalConstraint_PositionMinus1_UnsizedModule_Panics(t *testing.T) {
	sys := wiop.NewSystemf("lcUnsized")
	r0 := sys.NewRound()
	// Unsized static module: SetSize has not been called.
	mod := sys.NewModule(sys.Context.Childf("unsizedMod"), wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("unsizedCol"), wiop.VisibilityOracle, r0)
	assert.Panics(t, func() {
		mod.NewLocalConstraint(sys.Context.Childf("lcNegUnsized"), col.View(), -1)
	})
}
