package wiop_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/linea-monorepo/prover-ray/wiop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- ConcreteVector.ElementAt ----

func TestElementAt_PaddingNone(t *testing.T) {
	cv := baseVec(4, 7) // all-7 vector, PaddingNone via ConcreteVector
	mod := wiop.NewSystemf("s").NewSizedModule(wiop.NewRootFramef("m"), 4, wiop.PaddingDirectionNone)
	for i := range 4 {
		elem := cv.ElementAt(mod, i)
		var want field.Element
		want.SetUint64(7)
		assert.Equal(t, field.ElemFromBase(want), elem)
	}
}

func TestElementAt_PaddingLeft(t *testing.T) {
	// plain has 2 elements [3,3], module size 4, padding left → [pad,pad,3,3]
	var e field.Element
	e.SetUint64(3)
	plain := wiop.ConcreteVector{Plain: []field.Vec{field.VecFromBase([]field.Element{e, e})}}

	sys := wiop.NewSystemf("s")
	sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("m"), 4, wiop.PaddingDirectionLeft)

	var pad field.Element
	pad.SetUint64(0)
	plain.Padding = pad

	// rows 0,1 are padding (0), rows 2,3 are 3
	elem0 := plain.ElementAt(mod, 0)
	assert.Equal(t, field.ElemFromBase(pad), elem0)
	elem2 := plain.ElementAt(mod, 2)
	assert.Equal(t, field.ElemFromBase(e), elem2)
}

func TestElementAt_PaddingRight(t *testing.T) {
	// plain has 2 elements [5,5], module size 4, padding right → [5,5,pad,pad]
	var e field.Element
	e.SetUint64(5)
	plain := wiop.ConcreteVector{Plain: []field.Vec{field.VecFromBase([]field.Element{e, e})}}

	sys := wiop.NewSystemf("s")
	sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("m"), 4, wiop.PaddingDirectionRight)

	var pad field.Element
	pad.SetUint64(0)
	plain.Padding = pad

	elem1 := plain.ElementAt(mod, 1)
	assert.Equal(t, field.ElemFromBase(e), elem1)
	elem3 := plain.ElementAt(mod, 3)
	assert.Equal(t, field.ElemFromBase(pad), elem3)
}

func TestElementAt_OutOfBoundsPanic(t *testing.T) {
	cv := baseVec(4, 1)
	sys := wiop.NewSystemf("s")
	sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("m"), 4, wiop.PaddingDirectionNone)
	assert.Panics(t, func() { cv.ElementAt(mod, -1) })
	assert.Panics(t, func() { cv.ElementAt(mod, 4) })
}

// ---- ColumnView ----

func TestColumnView_Properties(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("cvCol"), wiop.VisibilityOracle, r0)
	cv := col.View()

	assert.Equal(t, col, cv.Column)
	assert.Equal(t, 0, cv.ShiftingOffset)
	assert.Equal(t, r0, cv.Round())
	assert.Equal(t, mod, cv.Module())
	assert.False(t, cv.IsExtension())
	assert.True(t, cv.IsMultiValued())
	assert.True(t, cv.IsSized())
	assert.Equal(t, 4, cv.Size())
	assert.Equal(t, 3, cv.Degree())
	assert.Equal(t, wiop.VisibilityOracle, cv.Visibility())
}

func TestColumnView_NilPanic(t *testing.T) {
	var col *wiop.Column
	assert.Panics(t, func() { col.View() })
}

func TestColumnView_Shift(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("shiftCol"), wiop.VisibilityOracle, r0)
	cv := col.View().Shift(2)
	assert.Equal(t, 2, cv.ShiftingOffset)

	cv2 := cv.Shift(-1)
	assert.Equal(t, 1, cv2.ShiftingOffset)
	// original unmodified
	assert.Equal(t, 2, cv.ShiftingOffset)
}

func TestColumnView_Shift_NilPanic(t *testing.T) {
	assert.Panics(t, func() {
		var cv *wiop.ColumnView
		cv.Shift(1)
	})
}

func TestColumnView_EvaluateSinglePanic(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("cvEvalCol"), wiop.VisibilityOracle, r0)
	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(col, baseVec(4, 1))
	assert.Panics(t, func() { col.View().EvaluateSingle(rt) })
}

func TestColumnView_EvaluateVector_Identity(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("cvEvCol"), wiop.VisibilityOracle, r0)
	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(col, baseVec(4, 3))

	result := col.View().EvaluateVector(rt)
	require.Len(t, result.Plain, 1)
	var want field.Element
	want.SetUint64(3)
	for i := range 4 {
		assert.Equal(t, want, result.Plain[0].AsBase()[i])
	}
}

func TestColumnView_EvaluateVector_Shifted(t *testing.T) {
	// Assign [0,1,2,3]; shift by 1 → [1,2,3,0]
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("cvShiftEv"), wiop.VisibilityOracle, r0)
	rt := wiop.NewRuntime(sys)

	elems := make([]field.Element, 4)
	for i := range 4 {
		elems[i].SetUint64(uint64(i))
	}
	rt.AssignColumn(col, &wiop.ConcreteVector{Plain: []field.Vec{field.VecFromBase(elems)}})

	result := col.View().Shift(1).EvaluateVector(rt)
	require.Len(t, result.Plain, 1)
	for i := range 4 {
		var want field.Element
		want.SetUint64(uint64((i + 1) % 4))
		assert.Equal(t, want, result.Plain[0].AsBase()[i])
	}
}

// ---- ColumnPosition ----

func TestColumnPosition_Properties(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("cpCol"), wiop.VisibilityOracle, r0)
	cp := col.At(2)

	assert.Equal(t, col, cp.Column)
	assert.Equal(t, 2, cp.Position)
	assert.False(t, cp.IsMultiValued())
	assert.False(t, cp.IsExtension())
	assert.Equal(t, 0, cp.Degree())
	assert.Equal(t, r0, cp.Round())
	assert.Nil(t, cp.Module())
	assert.Equal(t, wiop.VisibilityOracle, cp.Visibility())

	assert.Panics(t, func() { cp.IsSized() })
	assert.Panics(t, func() { cp.Size() })
	assert.Panics(t, func() { cp.EvaluateVector(wiop.NewRuntime(sys)) })
}

func TestColumnPosition_NilPanic(t *testing.T) {
	var col *wiop.Column
	assert.Panics(t, func() { col.At(0) })
}

func TestColumnPosition_EvaluateSingle(t *testing.T) {
	// Assign [0,1,2,3]; At(2) → 2
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("cpEvCol"), wiop.VisibilityOracle, r0)
	rt := wiop.NewRuntime(sys)

	elems := make([]field.Element, 4)
	for i := range 4 {
		elems[i].SetUint64(uint64(i))
	}
	rt.AssignColumn(col, &wiop.ConcreteVector{Plain: []field.Vec{field.VecFromBase(elems)}})

	result := col.At(2).EvaluateSingle(rt)
	var want field.Element
	want.SetUint64(2)
	assert.Equal(t, field.ElemFromBase(want), result.Value)
}

// ---- Cell.EvaluateSingle / EvaluateVector ----

func TestCell_EvaluateSingle(t *testing.T) {
	sys, r0, _, _ := newTestSystem(t)
	cell := r0.NewCell(sys.Context.Childf("cellEv"), false)
	rt := wiop.NewRuntime(sys)

	v := field.ElemFromBase(field.NewFromString("9"))
	rt.AssignCell(cell, v)

	result := cell.EvaluateSingle(rt)
	assert.Equal(t, v, result.Value)
}

func TestCell_EvaluateVector_Panics(t *testing.T) {
	sys, r0, _, _ := newTestSystem(t)
	cell := r0.NewCell(sys.Context.Childf("cellEvPanic"), false)
	rt := wiop.NewRuntime(sys)
	rt.AssignCell(cell, field.ElemZero())
	assert.Panics(t, func() { cell.EvaluateVector(rt) })
}

// ---- CoinField.EvaluateSingle / EvaluateVector ----

func TestCoinField_EvaluateSingle(t *testing.T) {
	sys, r0, r1, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("coinEvCol"), wiop.VisibilityOracle, r0)
	coin := r1.NewCoinField(sys.Context.Childf("coinEv"))
	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(col, baseVec(4, 1))
	rt.AdvanceRound()

	result := coin.EvaluateSingle(rt)
	// deterministic: calling again gives the same value
	result2 := coin.EvaluateSingle(rt)
	assert.Equal(t, result.Value, result2.Value)
}

func TestCoinField_EvaluateVector_Panics(t *testing.T) {
	sys, r0, r1, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("coinPanicCol"), wiop.VisibilityOracle, r0)
	coin := r1.NewCoinField(sys.Context.Childf("coinPanic"))
	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(col, baseVec(4, 1))
	rt.AdvanceRound()
	assert.Panics(t, func() { coin.EvaluateVector(rt) })
}

// ---- NewPrecomputedColumn ----

func TestModule_NewPrecomputedColumn(t *testing.T) {
	sys := wiop.NewSystemf("sys")
	sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)

	assignment := baseVec(4, 5)
	col := mod.NewPrecomputedColumn(sys.Context.Childf("pre"), wiop.VisibilityPublic, assignment)
	require.NotNil(t, col)
	assert.Equal(t, wiop.VisibilityPublic, col.Visibility)
	assert.False(t, col.IsExtension)
}

func TestModule_NewPrecomputedColumn_NilCtxPanic(t *testing.T) {
	sys := wiop.NewSystemf("sys")
	sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	assert.Panics(t, func() { mod.NewPrecomputedColumn(nil, wiop.VisibilityPublic, baseVec(4, 1)) })
}

func TestColumnView_Degree_UnsizedPanic(t *testing.T) {
	sys, r0, _, _ := newTestSystem(t)
	unsized := sys.NewModule(sys.Context.Childf("unsized"), wiop.PaddingDirectionNone)
	col := unsized.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	assert.Panics(t, func() { col.View().Degree() })
}
