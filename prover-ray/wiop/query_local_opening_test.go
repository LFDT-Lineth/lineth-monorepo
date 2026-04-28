package wiop_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/linea-monorepo/prover-ray/wiop"
	"github.com/consensys/linea-monorepo/prover-ray/wiop/wioptest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalOpening_Round_IsAlreadyAssigned_SelfAssign_Check(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("loCol"), wiop.VisibilityOracle, r0)
	lo := col.At(2).Open(sys.Context.Childf("lo"))

	require.NotNil(t, lo)
	assert.Equal(t, r0, lo.Round())

	rt := wiop.NewRuntime(sys)
	// Assign [0,1,2,3]; lo picks index 2 → expected 2
	elems := make([]field.Element, 4)
	for i := range 4 {
		elems[i].SetUint64(uint64(i))
	}
	rt.AssignColumn(col, &wiop.ConcreteVector{Plain: field.VecFromBase(elems)})

	assert.False(t, lo.IsAlreadyAssigned(rt))
	lo.SelfAssign(rt)
	assert.True(t, lo.IsAlreadyAssigned(rt))

	require.NoError(t, lo.Check(rt))
}

func TestLocalOpening_Check_Mismatch(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("loMisCol"), wiop.VisibilityOracle, r0)
	lo := col.At(1).Open(sys.Context.Childf("loMis"))

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(col, baseVec(4, 5))
	// manually assign wrong value to result cell
	rt.AssignCell(lo.Result, field.ElemFromBase(field.NewFromString("9")))

	err := lo.Check(rt)
	assert.Error(t, err)
}

func TestLocalOpening_Check_ColumnNotAssigned(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("loUnassigned"), wiop.VisibilityOracle, r0)
	lo := col.At(0).Open(sys.Context.Childf("loUnassignedQ"))

	rt := wiop.NewRuntime(sys)
	// don't assign column
	rt.AssignCell(lo.Result, field.ElemZero())
	err := lo.Check(rt)
	assert.Error(t, err)
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

// ---- Soundness ----

func TestLocalOpening_Soundness_Completeness(t *testing.T) {
	sc := wioptest.NewLocalOpeningScenario()
	rt := wiop.NewRuntime(sc.Sys)
	sc.RunHonest(&rt)
	require.NoError(t, sc.Query.Check(rt), "honest witness must pass Check")
}

func TestLocalOpening_Soundness_InvalidWitness(t *testing.T) {
	sc := wioptest.NewLocalOpeningScenario()
	rt := wiop.NewRuntime(sc.Sys)
	sc.RunInvalid(&rt)
	assert.Error(t, sc.Query.Check(rt), "invalid witness must be rejected by Check")
}
