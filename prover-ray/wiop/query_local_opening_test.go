package wiop_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestColumnPosition_Open_RegistersScalarVanishing asserts that Open lowers an
// opening into a single scalar vanishing on the column's module and returns a
// cell living in the column's round.
func TestColumnPosition_Open_RegistersScalarVanishing(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("loCol"), wiop.VisibilityOracle, r0)

	vansBefore := len(mod.Vanishings)
	result := col.At(2).Open(sys.Context.Childf("lo"))

	require.NotNil(t, result)
	assert.Equal(t, r0, result.Round())
	require.Len(t, mod.Vanishings, vansBefore+1,
		"Open must register exactly one vanishing")
	v := mod.Vanishings[len(mod.Vanishings)-1]
	assert.False(t, v.Expression.IsMultiValued(),
		"the opening vanishing must be scalar so the local-vanishing compiler discharges it")
}

// TestColumnPosition_Open_LazyCellResolvesToColumnValue asserts that the result
// cell is lazy: it is unassigned until read, then resolves to Column[Position].
func TestColumnPosition_Open_LazyCellResolvesToColumnValue(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("loCol"), wiop.VisibilityOracle, r0)
	result := col.At(2).Open(sys.Context.Childf("lo"))

	rt := wiop.NewRuntime(sys)
	// Assign [0,1,2,3]; position 2 → expected 2.
	elems := make([]field.Element, 4)
	for i := range elems {
		elems[i].SetUint64(uint64(i))
	}
	rt.AssignColumn(col, &wiop.ConcreteVector{Plain: field.VecFromBase(elems)})

	assert.False(t, rt.HasCellValue(result), "lazy cell must be unassigned before first read")

	got := rt.GetCellValue(result).AsBase()
	var want field.Element
	want.SetUint64(2)
	assert.True(t, got.Equal(&want), "lazy cell must resolve to Column[2] == 2")
	assert.True(t, rt.HasCellValue(result), "lazy cell must be cached after first read")
}

// TestColumnPosition_Open_ExplicitAssignmentWins asserts that an explicit cell
// assignment takes precedence over the lazy assigner (the basis for malicious-
// prover / mutator scenarios that inject a wrong opening value).
func TestColumnPosition_Open_ExplicitAssignmentWins(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("loCol"), wiop.VisibilityOracle, r0)
	result := col.At(2).Open(sys.Context.Childf("lo"))

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(col, baseVec(4, 7))

	var wrong field.Element
	wrong.SetUint64(9)
	rt.AssignCell(result, field.ElemFromBase(wrong))

	got := rt.GetCellValue(result).AsBase()
	assert.True(t, got.Equal(&wrong), "explicit assignment must win over the lazy assigner")
}

func TestColumnPosition_Open_NilReceiverPanic(t *testing.T) {
	sys := wiop.NewSystemf("s")
	sys.NewRound()
	var cp *wiop.ColumnPosition
	assert.Panics(t, func() { cp.Open(sys.Context.Childf("p")) })
}

func TestColumnPosition_Open_NilCtxPanic(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("openNilCtx"), wiop.VisibilityOracle, r0)
	assert.Panics(t, func() { col.At(0).Open(nil) })
}
