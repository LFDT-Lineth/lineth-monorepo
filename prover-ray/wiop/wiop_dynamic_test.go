package wiop_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/linea-monorepo/prover-ray/wiop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newDynamicTestSystem builds a two-round system with one dynamic module.
func newDynamicTestSystem(t *testing.T) (*wiop.System, *wiop.Round, *wiop.Round, *wiop.Module) {
	t.Helper()
	sys := wiop.NewSystemf("dyn-test")
	r0 := sys.NewRound()
	r1 := sys.NewRound()
	mod := sys.NewDynamicModule(sys.Context.Childf("dynmod"), wiop.PaddingDirectionRight)
	return sys, r0, r1, mod
}

// makeVec builds a ConcreteVector of length n with all elements set to val.
func makeVec(n int, val uint64) *wiop.ConcreteVector {
	elems := make([]field.Element, n)
	var e field.Element
	e.SetUint64(val)
	for i := range elems {
		elems[i] = e
	}
	return &wiop.ConcreteVector{Plain: field.VecFromBase(elems)}
}

// TestDynamicModule_IsDynamic verifies the flag is set correctly on each factory.
func TestDynamicModule_IsDynamic(t *testing.T) {
	sys := wiop.NewSystemf("test")
	sys.NewRound()
	static := sys.NewModule(sys.Context.Childf("static"), wiop.PaddingDirectionNone)
	dyn := sys.NewDynamicModule(sys.Context.Childf("dyn"), wiop.PaddingDirectionRight)

	assert.False(t, static.IsDynamic())
	assert.True(t, dyn.IsDynamic())
}

// TestDynamicModule_StaticAPI confirms Size()==0 and IsSized()==false for a
// dynamic module, consistent with the "permanently unsized" static view.
func TestDynamicModule_StaticAPI(t *testing.T) {
	sys := wiop.NewSystemf("test")
	sys.NewRound()
	dyn := sys.NewDynamicModule(sys.Context.Childf("dyn"), wiop.PaddingDirectionRight)

	assert.False(t, dyn.IsSized())
	assert.Equal(t, 0, dyn.Size())
}

// TestDynamicModule_SetSizePanic confirms SetSize panics on a dynamic module.
func TestDynamicModule_SetSizePanic(t *testing.T) {
	sys := wiop.NewSystemf("test")
	sys.NewRound()
	dyn := sys.NewDynamicModule(sys.Context.Childf("dyn"), wiop.PaddingDirectionRight)

	assert.Panics(t, func() { dyn.SetSize(8) })
}

// TestDynamicModule_AutoSizeOnFirstAssign verifies that the first AssignColumn
// to a dynamic module records the data length as the module's domain size.
func TestDynamicModule_AutoSizeOnFirstAssign(t *testing.T) {
	sys, r0, _, dyn := newDynamicTestSystem(t)
	col := dyn.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(col, makeVec(8, 1))

	// After assignment, RuntimeSize should return 8.
	assert.Equal(t, 8, dyn.RuntimeSize(rt))
}

// TestDynamicModule_OverflowSecondColumnPanic verifies that a second column
// whose data length exceeds the module's recorded size causes a panic.
func TestDynamicModule_GrowOnLargerColumn(t *testing.T) {
	sys, r0, _, dyn := newDynamicTestSystem(t)
	colA := dyn.NewColumn(sys.Context.Childf("A"), wiop.VisibilityOracle, r0)
	colB := dyn.NewColumn(sys.Context.Childf("B"), wiop.VisibilityOracle, r0)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(colA, makeVec(4, 1)) // sets domain size = 4
	rt.AssignColumn(colB, makeVec(8, 2)) // 8 > 4 → auto-grows to 8
	assert.Equal(t, 8, dyn.RuntimeSize(rt))
}

// TestDynamicModule_StaticOverflowPanic verifies that assigning a column whose
// data length exceeds a static module's declared size causes a panic.
func TestDynamicModule_StaticOverflowPanic(t *testing.T) {
	sys := wiop.NewSystemf("test")
	r0 := sys.NewRound()
	sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)

	rt := wiop.NewRuntime(sys)
	assert.Panics(t, func() {
		rt.AssignColumn(col, makeVec(8, 1)) // 8 > 4 → overflow
	})
}

// TestDynamicModule_MissingSizePanic verifies that RuntimeSize panics when no
// column of the module has been assigned yet.
func TestDynamicModule_MissingSizePanic(t *testing.T) {
	sys, _, _, dyn := newDynamicTestSystem(t)

	rt := wiop.NewRuntime(sys)
	assert.Panics(t, func() { dyn.RuntimeSize(rt) })
}

// TestDynamicModule_VanishingCheck verifies that a vanishing constraint on a
// dynamic module evaluates correctly, and that the same System can be reused
// across two Runtimes with different module sizes.
func TestDynamicModule_VanishingCheck(t *testing.T) {
	sys, r0, _, dyn := newDynamicTestSystem(t)

	colA := dyn.NewColumn(sys.Context.Childf("A"), wiop.VisibilityOracle, r0)
	colB := dyn.NewColumn(sys.Context.Childf("B"), wiop.VisibilityOracle, r0)

	// Constraint: A - B == 0
	dyn.NewVanishing(sys.Context.Childf("eq"), wiop.Sub(colA.View(), colB.View()))

	// Same System, two Runtimes with different sizes.
	for _, n := range []int{4, 8} {
		rt := wiop.NewRuntime(sys)
		rt.AssignColumn(colA, makeVec(n, 3)) // sets domain size = n
		rt.AssignColumn(colB, makeVec(n, 3))

		for _, v := range dyn.Vanishings {
			require.NoError(t, v.Check(rt), "n=%d", n)
		}
	}
}

// TestDynamicModule_VanishingCheckFailure verifies that a failing constraint is
// detected correctly for a dynamic module.
func TestDynamicModule_VanishingCheckFailure(t *testing.T) {
	sys, r0, _, dyn := newDynamicTestSystem(t)

	colA := dyn.NewColumn(sys.Context.Childf("A"), wiop.VisibilityOracle, r0)
	colB := dyn.NewColumn(sys.Context.Childf("B"), wiop.VisibilityOracle, r0)
	dyn.NewVanishing(sys.Context.Childf("eq"), wiop.Sub(colA.View(), colB.View()))

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(colA, makeVec(4, 1))
	rt.AssignColumn(colB, makeVec(4, 2)) // mismatch → constraint fails

	for _, v := range dyn.Vanishings {
		require.Error(t, v.Check(rt))
	}
}
