package wiop_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/rangecheck"
	"github.com/stretchr/testify/require"
)

// TestRangeCheckChecksDynamicModule is the regression test for a defect in
// which [wiop.RangeCheck.Check] performed no range test at all on a dynamic
// module: it accepted every assignment, however far outside [0, B).
//
// MECHANISM. Check used to size its row loop with Module.Size():
//
//	n := m.Size()
//	for row := range n { ... }
//
// Module.Size() returns 0 for a dynamic module -- the per-Runtime height lives
// in Module.RuntimeSize(rt). So n == 0, the loop body never executed, and Check
// returned nil unconditionally. The same wrong size reached
// ConcreteVector.ElementAt, resolving padding against Size() rather than the
// runtime height. The fix is n := m.RuntimeSize(rt) with
// cv.ElementAtN(m.Padding, n, row).
//
// IMPACT. Check is the reference oracle for a not-yet-compiled protocol
// (ProveOptions.CheckUnreducedQueries) and for the wioptest soundness
// harnesses. The zkcdriver places every non-static ZkC module on a dynamic
// module and registers each air.RangeConstraint on those columns, so in
// practice every real range constraint landed on the vacuous path.
//
// SCOPE. The defect was confined to the reference checker: the third subtest
// shows the reduction emitted by rangecheck.Compile enforces the bound on the
// same dynamic witness, so no proof was forgeable through it.
func TestRangeCheckChecksDynamicModule(t *testing.T) {
	// Row 3 holds 99, far outside the declared range [0, 4). Reused verbatim by
	// every subtest so the only variable is which module kind carries it.
	const bound = 4
	witness := func() *wiop.ConcreteVector { return vecOf(0, 1, 2, 99) }

	t.Run("out-of-range value on a dynamic module is rejected", func(t *testing.T) {
		sys := wiop.NewSystemf("rc-dyn")
		r0 := sys.NewRound()
		mod := sys.NewDynamicModule(sys.Context.Childf("dyn"), wiop.PaddingDirectionRight)
		col := mod.NewColumn(sys.Context.Childf("col"), r0)
		rc := mod.NewRangeCheck(sys.Context.Childf("rc"), col, bound)

		rt := wiop.NewRuntime(sys)
		rt.AssignColumn(col, witness())

		// The precondition that used to empty the loop: Size() is 0, while the
		// size Check must read is the runtime height.
		require.Equal(t, 0, mod.Size(),
			"precondition: Module.Size() is 0 for a dynamic module")
		require.Equal(t, 4, mod.RuntimeSize(rt),
			"precondition: the runtime height is 4, so Check must inspect 4 rows")

		err := rc.Check(rt)
		require.Error(t, err,
			"REGRESSION: 99 passed a RangeCheck with bound 4 -- Check looped over "+
				"Module.Size() == 0 rows instead of RuntimeSize == 4")
		require.Contains(t, err.Error(), "out of range [0, 4)")
		t.Logf("rejected as expected: %v", err)
	})

	t.Run("control: same witness on a sized module is rejected", func(t *testing.T) {
		sys := wiop.NewSystemf("rc-static")
		r0 := sys.NewRound()
		mod := sys.NewSizedModule(sys.Context.Childf("static"), 4, wiop.PaddingDirectionRight)
		col := mod.NewColumn(sys.Context.Childf("col"), r0)
		rc := mod.NewRangeCheck(sys.Context.Childf("rc"), col, bound)

		rt := wiop.NewRuntime(sys)
		rt.AssignColumn(col, witness())

		// Isolates the defect to the size resolution: with a non-zero Size() the
		// bound comparison and the field decoding do their job.
		err := rc.Check(rt)
		require.Error(t, err)
		t.Logf("rejected as expected: %v", err)
	})

	t.Run("control: the compiled reduction rejects the dynamic witness", func(t *testing.T) {
		sys := wiop.NewSystemf("rc-dyn-compiled")
		r0 := sys.NewRound()
		mod := sys.NewDynamicModule(sys.Context.Childf("dyn"), wiop.PaddingDirectionRight)
		col := mod.NewColumn(sys.Context.Childf("col"), r0)
		mod.NewRangeCheck(sys.Context.Childf("rc"), col, bound)

		rangecheck.Compile(sys)
		require.Len(t, sys.TableRelations, 1)

		rt := wiop.NewRuntime(sys)
		rt.AssignColumn(col, witness())

		// Scope bound: the Inclusion the pass emits does resolve the runtime
		// height, so the compiled argument still binds. Only the reference
		// checker is blind.
		err := sys.TableRelations[0].Check(rt)
		require.Error(t, err)
		t.Logf("compiled reduction rejects it: %v", err)
	})
}

func vecOf(vals ...uint64) *wiop.ConcreteVector {
	elems := make([]field.Element, len(vals))
	for i, v := range vals {
		elems[i].SetUint64(v)
	}
	return &wiop.ConcreteVector{Plain: field.VecFromBase(elems)}
}
