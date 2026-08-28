package grandproduct

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rowLimitVecU64 builds a ConcreteVector holding vals in row order. Duplicated
// from the external test file, which lives in package grandproduct_test and so
// cannot be reached from these in-package tests.
func rowLimitVecU64(vals ...uint64) *wiop.ConcreteVector {
	elems := make([]field.Element, len(vals))
	for i, v := range vals {
		elems[i].SetUint64(v)
	}
	return &wiop.ConcreteVector{Plain: field.VecFromBase(elems)}
}

// buildRowLimitSystem builds a single-column permutation A ~ B with the given
// module sizes and returns it together with its witness round, the permutation
// query, and a Runtime on which every dynamic module already has a size.
//
// A size of 0 means a dynamic module, mirroring newPermutationSystem in the
// external test file: NewPermutation's static balance check rejects two sized
// sides of different heights, so the deliberately lopsided systems below need
// the small side to be dynamic. Dynamic modules are then given a 4-row witness
// on the Runtime, because RuntimeSize panics on a dynamic module that has no
// size yet.
//
// Over-limit sizes cost nothing to declare: they are module metadata, and
// RuntimeSize reads a sized module's height directly, so no 2^58-row vector is
// ever materialised.
func buildRowLimitSystem(t *testing.T, aSize, bSize int) (*wiop.System, *wiop.Round, *wiop.TableRelationQuery, *wiop.Runtime) {
	t.Helper()

	sys := wiop.NewSystemf("gp-rowlimit")
	r0 := sys.NewRound()

	newModule := func(name string, size int) *wiop.Module {
		if size == 0 {
			return sys.NewDynamicModule(sys.Context.Childf("%s", name), wiop.PaddingDirectionRight)
		}
		return sys.NewSizedModule(sys.Context.Childf("%s", name), size, wiop.PaddingDirectionRight)
	}
	modA, modB := newModule("modA", aSize), newModule("modB", bSize)
	colA := modA.NewColumn(sys.Context.Childf("A"), r0)
	colB := modB.NewColumn(sys.Context.Childf("B"), r0)
	sys.NewPermutation(
		sys.Context.Childf("perm"),
		[]wiop.Table{wiop.NewTable(colA.View())},
		[]wiop.Table{wiop.NewTable(colB.View())},
	)

	var q *wiop.TableRelationQuery
	for _, tr := range sys.TableRelations {
		if tr.Kind == wiop.KindPermutation {
			q = tr
		}
	}
	require.NotNil(t, q, "the permutation query must be registered")

	rt := wiop.NewRuntime(sys)
	if aSize == 0 {
		rt.AssignColumn(colA, rowLimitVecU64(10, 20, 30, 40))
	}
	if bSize == 0 {
		rt.AssignColumn(colB, rowLimitVecU64(30, 10, 40, 20))
	}
	return sys, r0, q, rt
}

// TestRowLimitAction_RegisteredOnWitnessRound pins the wiring: compilePermutations
// must attach the row-limit check to the witness round on BOTH sides of the
// protocol, with the full [wiop.MaxPermutationRows] budget and pointing at the
// permutation query it guards.
//
// This is a structural assertion rather than an end-to-end one on purpose. The
// runtime check can no longer be reached through [wiop.System.Verify]: Verify
// caps every claimed dynamic-module size at [wiop.ColumnSizeMaxSupported] (2^22)
// and PrecheckRowLimit already counts dynamic modules at that same maximum, so a
// permutation that compiles cannot reach the budget for any runtime assignment.
// Without this test, deleting both Register* calls in compilePermutations goes
// unnoticed by the entire suite.
func TestRowLimitAction_RegisteredOnWitnessRound(t *testing.T) {
	sys, r0, q, _ := buildRowLimitSystem(t, 4, 4)
	Compile(sys)

	var proverHits, verifierHits int
	for _, a := range r0.ProverActions {
		if rl, ok := a.(*rowLimitAction); ok {
			proverHits++
			assert.Same(t, q, rl.query, "the action must guard the permutation query")
			assert.Equal(t, wiop.MaxPermutationRows, rl.limit,
				"the per-side budget must be MaxPermutationRows, undivided by the packing arity or the permutation count")
		}
	}
	for _, a := range r0.VerifierActions {
		if _, ok := a.(*rowLimitAction); ok {
			verifierHits++
		}
	}

	assert.Equal(t, 1, proverHits, "exactly one row-limit prover action must be registered on the witness round")
	assert.Equal(t, 1, verifierHits, "exactly one row-limit verifier action must be registered on the witness round")
}

// TestRowLimitAction_RejectsOverLimitASide drives the action directly with an A
// side declared at the budget. Constructing the action by hand — rather than
// through Compile, which rejects the same system at its compile-time precheck
// before any action is registered — is what makes the runtime path reachable.
func TestRowLimitAction_RejectsOverLimitASide(t *testing.T) {
	_, _, q, rt := buildRowLimitSystem(t, 1<<58, 0) // A at the budget; B dynamic and tiny.
	action := &rowLimitAction{query: q, limit: wiop.MaxPermutationRows}

	err := action.Check(rt)
	require.Error(t, err, "verifier must reject an over-limit A side")
	require.ErrorContains(t, err, "A side")
	assert.Panics(t, func() { action.Run(rt) }, "prover must panic on an over-limit A side")
}

// TestRowLimitAction_RejectsOverLimitBSide is the B-side counterpart: the two
// sides are bounded independently, so a tiny A side does not excuse an
// over-limit B side.
func TestRowLimitAction_RejectsOverLimitBSide(t *testing.T) {
	_, _, q, rt := buildRowLimitSystem(t, 0, 1<<58) // B at the budget; A dynamic and tiny.
	action := &rowLimitAction{query: q, limit: wiop.MaxPermutationRows}

	err := action.Check(rt)
	require.Error(t, err, "verifier must reject an over-limit B side")
	require.ErrorContains(t, err, "B side")
	assert.Panics(t, func() { action.Run(rt) }, "prover must panic on an over-limit B side")
}

// TestRowLimitAction_AcceptsWithinLimit confirms the check is silent for an
// in-budget permutation: Check returns nil and Run does not panic.
func TestRowLimitAction_AcceptsWithinLimit(t *testing.T) {
	_, _, q, rt := buildRowLimitSystem(t, 4, 4)
	action := &rowLimitAction{query: q, limit: wiop.MaxPermutationRows}

	require.NoError(t, action.Check(rt), "an in-budget permutation must pass the verifier check")
	assert.NotPanics(t, func() { action.Run(rt) }, "an in-budget permutation must not panic the prover")
}
