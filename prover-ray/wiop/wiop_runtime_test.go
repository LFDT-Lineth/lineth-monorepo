package wiop_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/linea-monorepo/prover-ray/wiop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestSystem is a helper that builds a minimal two-round system with one
// sized module. It returns the system, both rounds, and the module.
func newTestSystem(t *testing.T) (*wiop.System, *wiop.Round, *wiop.Round, *wiop.Module) {
	t.Helper()
	sys := wiop.NewSystemf("test")
	r0 := sys.NewRound()
	r1 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	return sys, r0, r1, mod
}

// baseVec builds a PaddingDirectionNone ConcreteVector of length n where each
// element equals the provided uint64 value.
func baseVec(n int, val uint64) *wiop.ConcreteVector {
	elems := make([]field.Element, n)
	v := field.NewFromString(string(rune('0' + val)))
	_ = v // zero for val==0 is the zero element
	var e field.Element
	e.SetUint64(val)
	for i := range elems {
		elems[i] = e
	}
	return &wiop.ConcreteVector{Plain: []field.Vec{field.VecFromBase(elems)}}
}

// ---- Column/Module methods ----

func TestColumn_Round(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	assert.Equal(t, r0, col.Round())
}

func TestColumn_Degree(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	// module is sized to 4 → degree == 3
	assert.Equal(t, 3, col.Degree())
}

func TestColumn_Degree_UnsizedPanic(t *testing.T) {
	sys, r0, _, _ := newTestSystem(t)
	unsized := sys.NewModule(sys.Context.Childf("unsized"), wiop.PaddingDirectionNone)
	col := unsized.NewColumn(sys.Context.Childf("col2"), wiop.VisibilityOracle, r0)
	assert.Panics(t, func() { col.Degree() })
}

func TestModule_NewColumn_NilRoundPanic(t *testing.T) {
	sys, _, _, mod := newTestSystem(t)
	assert.Panics(t, func() { mod.NewColumn(sys.Context.Childf("c"), wiop.VisibilityOracle, nil) })
}

func TestModule_NewColumn_NilCtxPanic(t *testing.T) {
	_, r0, _, mod := newTestSystem(t)
	assert.Panics(t, func() { mod.NewColumn(nil, wiop.VisibilityOracle, r0) })
}

func TestModule_NewColumn_ReusedCtxPanic(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	ctx := sys.Context.Childf("col")
	mod.NewColumn(ctx, wiop.VisibilityOracle, r0) // first use is fine
	assert.Panics(t, func() {
		mod.NewColumn(ctx, wiop.VisibilityOracle, r0) // re-using same ctx must panic
	})
}

func TestModule_NewExtensionColumn(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewExtensionColumn(sys.Context.Childf("ext"), wiop.VisibilityOracle, r0)
	assert.True(t, col.IsExtension)
}

// ---- Cell / CoinField methods ----

func TestCell_Properties(t *testing.T) {
	sys, r0, _, _ := newTestSystem(t)
	cell := r0.NewCell(sys.Context.Childf("cell"), false)
	extCell := r0.NewCell(sys.Context.Childf("extcell"), true)

	assert.Equal(t, r0, cell.Round())
	assert.False(t, cell.IsExtension())
	assert.True(t, extCell.IsExtension())
	assert.False(t, cell.IsMultiValued())
	assert.Equal(t, 0, cell.Degree())
	assert.Nil(t, cell.Module())
	assert.Equal(t, wiop.VisibilityPublic, cell.Visibility())

	// scalar-only panics
	assert.Panics(t, func() { cell.Size() })
	assert.Panics(t, func() { cell.IsSized() })
}

func TestCoinField_Properties(t *testing.T) {
	sys, r0, _, _ := newTestSystem(t)
	coin := r0.NewCoinField(sys.Context.Childf("coin"))

	assert.Equal(t, r0, coin.Round())
	assert.True(t, coin.IsExtension())
	assert.False(t, coin.IsMultiValued())
	assert.Equal(t, 0, coin.Degree())
	assert.Nil(t, coin.Module())
	assert.Equal(t, wiop.VisibilityPublic, coin.Visibility())

	assert.Panics(t, func() { coin.Size() })
	assert.Panics(t, func() { coin.IsSized() })
}

func TestRound_NewCell_NilCtxPanic(t *testing.T) {
	_, r0, _, _ := newTestSystem(t)
	assert.Panics(t, func() { r0.NewCell(nil, false) })
}

func TestRound_NewCoinField_NilCtxPanic(t *testing.T) {
	_, r0, _, _ := newTestSystem(t)
	assert.Panics(t, func() { r0.NewCoinField(nil) })
}

// ---- Runtime: column assignment ----

func TestRuntime_AssignAndGetColumn(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	rt := wiop.NewRuntime(sys)

	assert.False(t, rt.HasColumnAssignment(col))

	v := baseVec(4, 7)
	rt.AssignColumn(col, v)

	assert.True(t, rt.HasColumnAssignment(col))
	got := rt.GetColumnAssignment(col)
	assert.Equal(t, v, got)
}

func TestRuntime_AssignColumn_WrongRoundPanic(t *testing.T) {
	sys, _, r1, mod := newTestSystem(t)
	// col belongs to r1 but runtime starts at r0
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r1)
	rt := wiop.NewRuntime(sys)
	assert.Panics(t, func() { rt.AssignColumn(col, baseVec(4, 0)) })
}

func TestRuntime_AssignColumn_DoublePanic(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(col, baseVec(4, 1))
	assert.Panics(t, func() { rt.AssignColumn(col, baseVec(4, 2)) })
}

func TestRuntime_GetColumnAssignment_UnassignedPanic(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	rt := wiop.NewRuntime(sys)
	assert.Panics(t, func() { rt.GetColumnAssignment(col) })
}

// ---- Runtime: cell assignment ----

func TestRuntime_AssignAndGetCell(t *testing.T) {
	sys, r0, _, _ := newTestSystem(t)
	cell := r0.NewCell(sys.Context.Childf("cell"), false)
	rt := wiop.NewRuntime(sys)

	assert.False(t, rt.HasCellValue(cell))

	v := field.ElemFromBase(field.NewFromString("5"))
	rt.AssignCell(cell, v)

	assert.True(t, rt.HasCellValue(cell))
	got := rt.GetCellValue(cell)
	assert.Equal(t, v, got)
}

func TestRuntime_AssignCell_WrongRoundPanic(t *testing.T) {
	sys, _, r1, _ := newTestSystem(t)
	cell := r1.NewCell(sys.Context.Childf("cell"), false)
	rt := wiop.NewRuntime(sys)
	assert.Panics(t, func() { rt.AssignCell(cell, field.ElemZero()) })
}

func TestRuntime_AssignCell_DoublePanic(t *testing.T) {
	sys, r0, _, _ := newTestSystem(t)
	cell := r0.NewCell(sys.Context.Childf("cell"), false)
	rt := wiop.NewRuntime(sys)
	rt.AssignCell(cell, field.ElemZero())
	assert.Panics(t, func() { rt.AssignCell(cell, field.ElemZero()) })
}

func TestRuntime_GetCellValue_UnassignedPanic(t *testing.T) {
	sys, r0, _, _ := newTestSystem(t)
	cell := r0.NewCell(sys.Context.Childf("cell"), false)
	rt := wiop.NewRuntime(sys)
	assert.Panics(t, func() { rt.GetCellValue(cell) })
}

// ---- Runtime: state bag ----

func TestRuntime_State(t *testing.T) {
	sys, _, _, _ := newTestSystem(t)
	rt := wiop.NewRuntime(sys)

	_, ok := rt.GetState("k")
	assert.False(t, ok)

	rt.SetState("k", "hello")
	v, ok := rt.GetState("k")
	require.True(t, ok)
	assert.Equal(t, "hello", v)
}

// ---- Runtime: AdvanceRound and coins ----

func TestRuntime_AdvanceRound_Basic(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	rt := wiop.NewRuntime(sys)
	assert.Equal(t, r0, rt.CurrentRound())

	rt.AssignColumn(col, baseVec(4, 3))
	rt.AdvanceRound()
	assert.Equal(t, sys.Rounds[1], rt.CurrentRound())
}

func TestRuntime_AdvanceRound_WithCoinSampling(t *testing.T) {
	sys, r0, r1, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	coin := r1.NewCoinField(sys.Context.Childf("coin"))
	rt := wiop.NewRuntime(sys)

	rt.AssignColumn(col, baseVec(4, 1))
	rt.AdvanceRound()

	// coin must be available after advancing into r1
	v := rt.GetCoinValue(coin)
	// just assert it's deterministic (call twice with same state → same coin)
	assert.Equal(t, v, rt.GetCoinValue(coin))
}

func TestRuntime_GetCoinValue_NotSampledPanic(t *testing.T) {
	sys, r0, r1, _ := newTestSystem(t)
	_ = r0
	coin := r1.NewCoinField(sys.Context.Childf("coin"))
	rt := wiop.NewRuntime(sys)
	// we have not advanced yet — coin is from r1
	assert.Panics(t, func() { rt.GetCoinValue(coin) })
}

func TestRuntime_AdvanceRound_LastRoundPanic(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(col, baseVec(4, 0))
	rt.AdvanceRound() // now at r1 (last round)
	assert.Panics(t, func() { rt.AdvanceRound() })
}

func TestRuntime_AdvanceRound_UnassignedOraclePanic(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	_ = mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0) // not assigned
	rt := wiop.NewRuntime(sys)
	assert.Panics(t, func() { rt.AdvanceRound() })
}

func TestRuntime_AdvanceRound_UnassignedCellPanic(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	_ = r0.NewCell(sys.Context.Childf("cell"), false) // not assigned
	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(col, baseVec(4, 0))
	assert.Panics(t, func() { rt.AdvanceRound() })
}

func TestNewRuntime_NoRoundsPanic(t *testing.T) {
	sys := wiop.NewSystemf("empty")
	assert.Panics(t, func() { wiop.NewRuntime(sys) })
}

// ---- HasCellAssignment ----

func TestRuntime_HasCellAssignment(t *testing.T) {
	sys, r0, _, _ := newTestSystem(t)
	cell := r0.NewCell(sys.Context.Childf("cell"), false)
	rt := wiop.NewRuntime(sys)
	assert.False(t, rt.HasCellAssignment(cell))
	rt.AssignCell(cell, field.ElemZero())
	assert.True(t, rt.HasCellAssignment(cell))
}
