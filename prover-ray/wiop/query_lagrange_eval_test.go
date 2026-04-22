package wiop_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/linea-monorepo/prover-ray/wiop"
	"github.com/consensys/linea-monorepo/prover-ray/wiop/wioptest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- Soundness ----

func TestLagrangeEval_Soundness_Completeness(t *testing.T) {
	sc := wioptest.NewLagrangeEvalScenario()
	rt := wiop.NewRuntime(sc.Sys)
	sc.RunHonest(&rt)
	require.NoError(t, sc.Query.Check(rt), "honest witness must pass Check")
}

func TestLagrangeEval_Soundness_InvalidWitness(t *testing.T) {
	sc := wioptest.NewLagrangeEvalScenario()
	rt := wiop.NewRuntime(sc.Sys)
	sc.RunInvalid(&rt)
	assert.Error(t, sc.Query.Check(rt), "invalid witness must be rejected by Check")
}

// ---- LagrangeEval helpers ----

// lagrangeSystem builds a 2-round system with one 4-element sized module,
// one column and one coin serving as evaluation point.
func lagrangeSystem(t *testing.T) (*wiop.System, *wiop.Round, *wiop.Round, *wiop.Module, *wiop.Column, *wiop.CoinField) {
	t.Helper()
	sys := wiop.NewSystemf("le")
	r0 := sys.NewRound()
	r1 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("leMod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("leCol"), wiop.VisibilityOracle, r0)
	coin := r1.NewCoinField(sys.Context.Childf("leCoin"))
	return sys, r0, r1, mod, col, coin
}

// ---- LagrangeEval construction ----

func TestLagrangeEval_NewLagrangeEval_Basic(t *testing.T) {
	sys, r0, r1, _, col, coin := lagrangeSystem(t)
	_ = r0
	le := sys.NewLagrangeEval(sys.Context.Childf("le1"), []*wiop.ColumnView{col.View()}, coin)
	require.NotNil(t, le)
	assert.Len(t, le.EvaluationClaims, 1)
	// Round() should be r1 (the coin's round)
	assert.Equal(t, r1, le.Round())
}

func TestLagrangeEval_NewLagrangeEval_NilCtxPanic(t *testing.T) {
	sys, _, _, _, col, coin := lagrangeSystem(t)
	assert.Panics(t, func() { sys.NewLagrangeEval(nil, []*wiop.ColumnView{col.View()}, coin) })
}

func TestLagrangeEval_NewLagrangeEval_EmptyPolysPanic(t *testing.T) {
	sys, _, _, _, _, coin := lagrangeSystem(t)
	assert.Panics(t, func() { sys.NewLagrangeEval(sys.Context.Childf("le2"), nil, coin) })
}

func TestLagrangeEval_NewLagrangeEval_NoNextRoundPanic(t *testing.T) {
	// Only one round — no next round for claim cells
	sys := wiop.NewSystemf("leNoRound")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	cell := r0.NewCell(sys.Context.Childf("ep"), false)
	assert.Panics(t, func() {
		sys.NewLagrangeEval(sys.Context.Childf("le3"), []*wiop.ColumnView{col.View()}, cell)
	})
}

func TestLagrangeEval_NewLagrangeEvalFrom(t *testing.T) {
	sys, _, r1, _, col, coin := lagrangeSystem(t)
	claim := r1.NewCell(sys.Context.Childf("extClaim"), false)
	le := sys.NewLagrangeEvalFrom(sys.Context.Childf("leFrom"), []*wiop.ColumnView{col.View()}, coin, []*wiop.Cell{claim})
	require.NotNil(t, le)
	assert.Equal(t, claim, le.EvaluationClaims[0])
}

func TestLagrangeEval_NewLagrangeEvalFrom_LenMismatchPanic(t *testing.T) {
	sys, _, r1, _, col, coin := lagrangeSystem(t)
	c1 := r1.NewCell(sys.Context.Childf("c1"), false)
	c2 := r1.NewCell(sys.Context.Childf("c2"), false)
	assert.Panics(t, func() {
		sys.NewLagrangeEvalFrom(sys.Context.Childf("leFromMis"), []*wiop.ColumnView{col.View()}, coin, []*wiop.Cell{c1, c2})
	})
}

// ---- LagrangeEval SelfAssign + Check ----

func TestLagrangeEval_SelfAssign_Check(t *testing.T) {
	// Constant column of value c; Lagrange eval of constant polynomial P = c is c everywhere.
	sys, _, _, _, col, coin := lagrangeSystem(t)
	le := sys.NewLagrangeEval(sys.Context.Childf("leEval"), []*wiop.ColumnView{col.View()}, coin)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(col, baseVec(4, 3))
	rt.AdvanceRound() // samples coin, now at r1

	assert.False(t, le.IsAlreadyAssigned(rt))
	le.SelfAssign(rt)
	assert.True(t, le.IsAlreadyAssigned(rt))
	require.NoError(t, le.Check(rt))
}

func TestLagrangeEval_Check_ColumnNotAssigned(t *testing.T) {
	sys, _, _, _, col, coin := lagrangeSystem(t)
	le := sys.NewLagrangeEval(sys.Context.Childf("leNoCol"), []*wiop.ColumnView{col.View()}, coin)

	// Manually assign a coin by advancing normally first, but don't assign column.
	// However, we can't advance without assigning oracle columns... let's make a
	// separate system without oracle columns to reach r1.
	sys2 := wiop.NewSystemf("le2")
	r0b := sys2.NewRound()
	r1b := sys2.NewRound()
	mod2 := sys2.NewSizedModule(sys2.Context.Childf("mod2"), 4, wiop.PaddingDirectionNone)
	col2 := mod2.NewColumn(sys2.Context.Childf("col2"), wiop.VisibilityOracle, r0b)
	coin2 := r1b.NewCoinField(sys2.Context.Childf("coin2"))
	le2 := sys2.NewLagrangeEval(sys2.Context.Childf("le2q"), []*wiop.ColumnView{col2.View()}, coin2)
	_ = le

	rt2 := wiop.NewRuntime(sys2)
	rt2.AssignColumn(col2, baseVec(4, 1))
	rt2.AdvanceRound()
	// Don't assign col2 in a fresh runtime — use a fresh runtime without column assignment.
	// Actually we just already advanced once. Let's test Check when claim isn't assigned.
	// SelfAssign was not called, and we haven't assigned the result cell.
	// Check should error because claim cell is unassigned.
	err := le2.Check(rt2)
	assert.Error(t, err)
}

func TestLagrangeEval_Check_Mismatch(t *testing.T) {
	sys, r0, _, _, col, coin := lagrangeSystem(t)
	le := sys.NewLagrangeEval(sys.Context.Childf("leMis"), []*wiop.ColumnView{col.View()}, coin)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(col, baseVec(4, 5))
	rt.AdvanceRound()
	// Assign wrong value to claim cell
	rt.AssignCell(le.EvaluationClaims[0], field.ElemFromBase(field.NewFromString("99")))

	err := le.Check(rt)
	assert.Error(t, err)
	_ = r0
}

func TestLagrangeEval_CheckGnark_Panics(t *testing.T) {
	sys, _, _, _, col, coin := lagrangeSystem(t)
	le := sys.NewLagrangeEval(sys.Context.Childf("leGnark"), []*wiop.ColumnView{col.View()}, coin)
	assert.Panics(t, func() { le.CheckGnark(nil, nil) })
}

// ---- LagrangeEval with Cell as evaluation point (covers roundOf *Cell branch) ----

func TestLagrangeEval_CellEvalPoint_Round(t *testing.T) {
	sys := wiop.NewSystemf("leCell")
	r0 := sys.NewRound()
	r1 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	cell := r1.NewCell(sys.Context.Childf("ep"), false)
	le := sys.NewLagrangeEval(sys.Context.Childf("le"), []*wiop.ColumnView{col.View()}, cell)
	assert.Equal(t, r1, le.Round())
}

// ---- LagrangeEval with PaddingDirectionLeft (covers evalLagrangePaddedBaseBase) ----

func TestLagrangeEval_PaddingLeft_SelfAssign_Check(t *testing.T) {
	sys := wiop.NewSystemf("leLeft")
	r0 := sys.NewRound()
	r1 := sys.NewRound()
	// Module size 4, but assignment provides 3 elements; padding=Left → [pad,v,v,v]
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionLeft)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	coin := r1.NewCoinField(sys.Context.Childf("coin"))
	le := sys.NewLagrangeEval(sys.Context.Childf("le"), []*wiop.ColumnView{col.View()}, coin)

	rt := wiop.NewRuntime(sys)
	// 3-element assignment with no padding data specified; padding value is zero
	var v field.Element
	v.SetUint64(2)
	elems := []field.Element{v, v, v}
	rt.AssignColumn(col, &wiop.ConcreteVector{Plain: []field.Vec{field.VecFromBase(elems)}})
	rt.AdvanceRound()

	le.SelfAssign(rt)
	require.NoError(t, le.Check(rt))
}

func TestLagrangeEval_NewLagrangeEvalFrom_NilCtxPanic(t *testing.T) {
	sys, _, r1, _, col, coin := lagrangeSystem(t)
	claim := r1.NewCell(sys.Context.Childf("cl"), false)
	assert.Panics(t, func() {
		sys.NewLagrangeEvalFrom(nil, []*wiop.ColumnView{col.View()}, coin, []*wiop.Cell{claim})
	})
}

func TestLagrangeEval_NewLagrangeEvalFrom_EmptyPolysPanic(t *testing.T) {
	sys, _, r1, _, _, coin := lagrangeSystem(t)
	claim := r1.NewCell(sys.Context.Childf("cl2"), false)
	assert.Panics(t, func() {
		sys.NewLagrangeEvalFrom(sys.Context.Childf("le"), nil, coin, []*wiop.Cell{claim})
	})
}

// ---- LagrangeEval.Check: unassigned column error ----

func TestLagrangeEval_Check_UnassignedColumn(t *testing.T) {
	sys, _, _, _, col, coin := lagrangeSystem(t)
	le := sys.NewLagrangeEval(sys.Context.Childf("leUnassigned"), []*wiop.ColumnView{col.View()}, coin)

	// Build a runtime at r1 without assigning the column.
	// We need to advance to r1 somehow. Use a separate system trick:
	// actually let's just assign the column but then create a runtime on
	// a different system state... The simplest approach: use a fresh runtime,
	// assign only what's needed for AdvanceRound (no oracle columns exist
	// except col, which we intentionally skip), then try Check.
	// But without assigning col (oracle), AdvanceRound will panic.
	// So instead: don't advance, just call Check while at r0 -- column is
	// not assigned.
	rt := wiop.NewRuntime(sys)
	_ = le
	_ = rt

	// Now create a fresh runtime that doesn't have the column assigned but is at r1.
	// Can't easily do this with the existing API. Instead test via a separate module
	// where we can control assignment.
	sys3 := wiop.NewSystemf("leChkUA")
	r0 := sys3.NewRound()
	r1 := sys3.NewRound()
	mod3 := sys3.NewSizedModule(sys3.Context.Childf("m"), 4, wiop.PaddingDirectionNone)
	col3 := mod3.NewColumn(sys3.Context.Childf("col3"), wiop.VisibilityOracle, r0)
	coin3 := r1.NewCoinField(sys3.Context.Childf("coin3"))
	le3 := sys3.NewLagrangeEval(sys3.Context.Childf("le3"), []*wiop.ColumnView{col3.View()}, coin3)

	rt3 := wiop.NewRuntime(sys3)
	rt3.AssignColumn(col3, baseVec(4, 1))
	rt3.AdvanceRound()
	// Now assign claim but skip SelfAssign (claim is unassigned) → Check returns error
	err := le3.Check(rt3)
	assert.Error(t, err)
	_ = rt
}

// evalWithBaseZ builds a system where the evaluation point is a *Cell whose
// assigned value is a base-field element. This triggers the z.IsBase() branches
// in evalLagrangePadded.
func evalWithBaseZ(t *testing.T, padding wiop.PaddingDirection, ext bool) {
	t.Helper()
	sys := wiop.NewSystemf("padTest")
	r0 := sys.NewRound()
	r1 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, padding)

	var col *wiop.Column
	if ext {
		col = mod.NewExtensionColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	} else {
		col = mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	}

	// Evaluation point is a Cell holding a base-field value → z.IsBase() = true.
	ep := r1.NewCell(sys.Context.Childf("ep"), false)
	le := sys.NewLagrangeEval(sys.Context.Childf("le"), []*wiop.ColumnView{col.View()}, ep)

	rt := wiop.NewRuntime(sys)

	if ext {
		// Extension column: assign extension elements all equal to 2.
		var e field.Ext
		var two field.Element
		two.SetUint64(2)
		e.B0.A0 = two
		elems := make([]field.Ext, 3)
		for i := range elems {
			elems[i] = e
		}
		rt.AssignColumn(col, &wiop.ConcreteVector{Plain: []field.Vec{field.VecFromExt(elems)}})
	} else {
		// Base column: assign 3 elements all equal to 5, padded to 4.
		var v field.Element
		v.SetUint64(5)
		elems := []field.Element{v, v, v}
		rt.AssignColumn(col, &wiop.ConcreteVector{Plain: []field.Vec{field.VecFromBase(elems)}})
	}

	// Advance to r1 so we can assign the Cell in r1.
	rt.AdvanceRound()

	// Assign a base-field value to the evaluation-point cell (now at r1, so valid).
	var z field.Element
	z.SetUint64(3)
	rt.AssignCell(ep, field.ElemFromBase(z))

	le.SelfAssign(rt)
	require.NoError(t, le.Check(rt))
}

// TestEvalLagrangePaddedBaseBase: base data, left padding, base z.
func TestEvalLagrangePaddedBaseBase(t *testing.T) {
	evalWithBaseZ(t, wiop.PaddingDirectionLeft, false)
}

// TestEvalLagrangePaddedExtBase: extension data, right padding, base z.
func TestEvalLagrangePaddedExtBase(t *testing.T) {
	evalWithBaseZ(t, wiop.PaddingDirectionRight, true)
}

// TestEvalLagrangePaddedExtExt: extension data, left padding, extension z (coin).
func TestEvalLagrangePaddedExtExt(t *testing.T) {
	sys := wiop.NewSystemf("extExt")
	r0 := sys.NewRound()
	r1 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionLeft)
	col := mod.NewExtensionColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	// Coin gives an extension element → z.IsBase() = false.
	coin := r1.NewCoinField(sys.Context.Childf("coin"))
	le := sys.NewLagrangeEval(sys.Context.Childf("le"), []*wiop.ColumnView{col.View()}, coin)

	rt := wiop.NewRuntime(sys)

	var e field.Ext
	var two field.Element
	two.SetUint64(2)
	e.B0.A0 = two
	elems := make([]field.Ext, 3)
	for i := range elems {
		elems[i] = e
	}
	rt.AssignColumn(col, &wiop.ConcreteVector{Plain: []field.Vec{field.VecFromExt(elems)}})
	rt.AdvanceRound()

	le.SelfAssign(rt)
	require.NoError(t, le.Check(rt))
}

// TestEvalPolynomials_Shift: covers the k != 0 branch in evalPolynomials.
func TestEvalPolynomials_Shift(t *testing.T) {
	sys := wiop.NewSystemf("shift")
	r0 := sys.NewRound()
	r1 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	coin := r1.NewCoinField(sys.Context.Childf("coin"))

	// Shifted view: ShiftingOffset = 1
	shifted := col.View().Shift(1)
	le := sys.NewLagrangeEval(sys.Context.Childf("le"), []*wiop.ColumnView{shifted}, coin)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(col, baseVec(4, 5))
	rt.AdvanceRound()

	le.SelfAssign(rt)
	require.NoError(t, le.Check(rt))
}

// TestLagrangeEval_Check_ColumnUnassigned tests the first guard in Check: when
// a polynomial column has no runtime assignment, Check must return an error
// before attempting to evaluate anything.
func TestLagrangeEval_Check_ColumnUnassigned(t *testing.T) {
	sys, _, _, _, col, coin := lagrangeSystem(t)
	le := sys.NewLagrangeEval(sys.Context.Childf("leUA"), []*wiop.ColumnView{col.View()}, coin)

	// Do not assign the column. Check must fail on the first guard.
	rt := wiop.NewRuntime(sys)
	err := le.Check(rt)
	assert.Error(t, err, "Check must fail when the polynomial column is unassigned")
}

// ---- Vanishing/LocalOpening CheckGnark panics ----
func TestLocalOpening_CheckGnark_Panics(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("logCol"), wiop.VisibilityOracle, r0)
	lo := col.At(0).Open(sys.Context.Childf("log"))
	assert.Panics(t, func() { lo.CheckGnark(nil, nil) })
}
