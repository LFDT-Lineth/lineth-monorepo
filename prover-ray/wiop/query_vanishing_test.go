package wiop_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/linea-monorepo/prover-ray/wiop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVanishing_Module_Round(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("vanCol"), wiop.VisibilityOracle, r0)
	v := mod.NewVanishing(sys.Context.Childf("van"), col.View())

	assert.Equal(t, mod, v.Module())
	assert.Equal(t, r0, v.Round())
}

func TestVanishing_String(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("strCol"), wiop.VisibilityOracle, r0)
	ctx := sys.Context.Childf("strV")
	v := mod.NewVanishing(ctx, col.View())
	s := v.String()
	assert.Contains(t, s, "Vanishing")
	assert.Contains(t, s, "strV")
}

func TestVanishing_Check_Scalar_Zero(t *testing.T) {
	// scalar constant zero vanishes
	sys, r0, _, _ := newTestSystem(t)
	rt := wiop.NewRuntime(sys)
	cell := r0.NewCell(sys.Context.Childf("scZero"), false)
	rt.AssignCell(cell, field.ElemZero())

	// zero - zero == 0
	expr := wiop.Sub(cell, cell)
	mod := sys.NewSizedModule(sys.Context.Childf("modScZero"), 1, wiop.PaddingDirectionNone)
	v := mod.NewVanishing(sys.Context.Childf("scVanish"), expr)
	require.NoError(t, v.Check(rt))
}

func TestVanishing_Check_Scalar_NonZero(t *testing.T) {
	sys, r0, _, _ := newTestSystem(t)
	rt := wiop.NewRuntime(sys)
	cell := r0.NewCell(sys.Context.Childf("scNZ"), false)
	rt.AssignCell(cell, field.ElemFromBase(field.NewFromString("1")))

	mod := sys.NewSizedModule(sys.Context.Childf("modScNZ"), 1, wiop.PaddingDirectionNone)
	v := mod.NewVanishing(sys.Context.Childf("scNZV"), cell)
	err := v.Check(rt)
	assert.Error(t, err)
}

func TestVanishing_Check_Vector_AllZero(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("allZCol"), wiop.VisibilityOracle, r0)
	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(col, baseVec(4, 0))

	v := mod.NewVanishing(sys.Context.Childf("allZVan"), col.View())
	require.NoError(t, v.Check(rt))
}

func TestVanishing_Check_Vector_NonZero(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("nzVCol"), wiop.VisibilityOracle, r0)
	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(col, baseVec(4, 1))

	v := mod.NewVanishing(sys.Context.Childf("nzVVan"), col.View())
	err := v.Check(rt)
	assert.Error(t, err)
}

func TestVanishing_Check_Vector_CancelledPositions(t *testing.T) {
	// col = [1,1,1,1]; cancel rows 0,1,2,3 → effectively all cancelled → should pass
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("cancelCol"), wiop.VisibilityOracle, r0)
	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(col, baseVec(4, 1))

	// negative indices: -4 = row 0, -3 = row 1, -2 = row 2, -1 = row 3
	v := mod.NewVanishingManual(sys.Context.Childf("cancelVan"), col.View(), -4, -3, -2, -1)
	require.NoError(t, v.Check(rt))
}

func TestVanishing_NewVanishing_NilCtxPanic(t *testing.T) {
	_, _, _, mod := newTestSystem(t)
	k := wiop.NewConstantField(field.NewFromString("0"))
	assert.Panics(t, func() { mod.NewVanishing(nil, k) })
}

func TestVanishing_NewVanishing_NilExprPanic(t *testing.T) {
	sys, _, _, mod := newTestSystem(t)
	assert.Panics(t, func() { mod.NewVanishing(sys.Context.Childf("nilExpr"), nil) })
}

func TestVanishing_NewVanishingManual_NilCtxPanic(t *testing.T) {
	_, _, _, mod := newTestSystem(t)
	k := wiop.NewConstantField(field.NewFromString("0"))
	assert.Panics(t, func() { mod.NewVanishingManual(nil, k) })
}

func TestVanishing_NewVanishingManual_NilExprPanic(t *testing.T) {
	sys, _, _, mod := newTestSystem(t)
	assert.Panics(t, func() { mod.NewVanishingManual(sys.Context.Childf("nilExpr2"), nil) })
}

func TestVanishing_AutoCancelledPositions_PositiveShift(t *testing.T) {
	// shift +2 → last 2 rows cancelled: -2, -1
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("shiftPosCol"), wiop.VisibilityOracle, r0)
	// col.View().Shift(2) triggers auto-cancel of last 2 rows
	v := mod.NewVanishing(sys.Context.Childf("shiftPosV"), col.View().Shift(2))
	assert.Len(t, v.CancelledPositions, 2)
}

func TestVanishing_AutoCancelledPositions_NegativeShift(t *testing.T) {
	// shift -1 → first row cancelled: 0
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("shiftNegCol"), wiop.VisibilityOracle, r0)
	v := mod.NewVanishing(sys.Context.Childf("shiftNegV"), col.View().Shift(-1))
	assert.Len(t, v.CancelledPositions, 1)
	assert.Equal(t, 0, v.CancelledPositions[0])
}

func TestVanishing_CheckGnark_Panics(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("vgCol"), wiop.VisibilityOracle, r0)
	v := mod.NewVanishing(sys.Context.Childf("vg"), col.View())
	assert.Panics(t, func() { v.CheckGnark(nil, nil) })
}

// ---- maxRoundInExpr (via Vanishing.Round) ----

func TestMaxRoundInExpr_MultipleRounds(t *testing.T) {
	sys := wiop.NewSystemf("s")
	r0 := sys.NewRound()
	r1 := sys.NewRound()
	mod0 := sys.NewSizedModule(sys.Context.Childf("m0"), 4, wiop.PaddingDirectionNone)
	mod1 := sys.NewSizedModule(sys.Context.Childf("m1"), 4, wiop.PaddingDirectionNone)
	col0 := mod0.NewColumn(sys.Context.Childf("c0"), wiop.VisibilityOracle, r0)
	col1 := mod1.NewColumn(sys.Context.Childf("c1"), wiop.VisibilityOracle, r1)

	// Add two vector expressions from different modules is valid structurally;
	// Vanishing.Round walks the tree and picks the max.
	expr := wiop.Add(col0.View(), col1.View())
	// Use a scratch module just to hang the Vanishing on
	mod := sys.NewSizedModule(sys.Context.Childf("scratch"), 4, wiop.PaddingDirectionNone)
	v := mod.NewVanishing(sys.Context.Childf("v"), expr)
	assert.Equal(t, r1, v.Round())
}

func TestVanishing_Check_Vector_ExtZero(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	extCol := mod.NewExtensionColumn(sys.Context.Childf("extZeroCol"), wiop.VisibilityOracle, r0)
	rt := wiop.NewRuntime(sys)

	// All-zero extension vector
	var zeroExt field.Ext
	elems := make([]field.Ext, 4)
	for i := range elems {
		elems[i] = zeroExt
	}
	rt.AssignColumn(extCol, &wiop.ConcreteVector{Plain: field.VecFromExt(elems)})

	v := mod.NewVanishing(sys.Context.Childf("extZeroVan"), extCol.View())
	require.NoError(t, v.Check(rt))
}
