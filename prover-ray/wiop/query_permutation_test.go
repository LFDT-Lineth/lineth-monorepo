package wiop_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newPermutationSystem builds a single-round system with two equal-sized
// modules, one oracle column each, for single-column permutation tests.
func newPermutationSystem(t *testing.T) (*wiop.System, *wiop.Column, *wiop.Column) {
	t.Helper()
	sys := wiop.NewSystemf("permSys")
	r0 := sys.NewRound()
	modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionNone)
	modB := sys.NewSizedModule(sys.Context.Childf("modB"), 4, wiop.PaddingDirectionNone)
	colA := modA.NewColumn(sys.Context.Childf("A"), wiop.VisibilityOracle, r0)
	colB := modB.NewColumn(sys.Context.Childf("B"), wiop.VisibilityOracle, r0)
	return sys, colA, colB
}

func TestNewPermutation_Basic(t *testing.T) {
	sys, colA, colB := newPermutationSystem(t)
	p := sys.NewPermutation(
		sys.Context.Childf("perm"),
		[]wiop.Table{wiop.NewTable(colA.View())},
		[]wiop.Table{wiop.NewTable(colB.View())},
	)
	require.NotNil(t, p)
	assert.Equal(t, wiop.KindPermutation, p.Kind)
	require.Len(t, sys.TableRelations, 1)
	assert.Same(t, p, sys.TableRelations[0])
	assert.Equal(t, sys.Rounds[0], p.Round())
}

// TestPermutation_Check_Completeness: B is a reordering of A, so Check accepts.
func TestPermutation_Check_Completeness(t *testing.T) {
	sys, colA, colB := newPermutationSystem(t)
	p := sys.NewPermutation(
		sys.Context.Childf("perm"),
		[]wiop.Table{wiop.NewTable(colA.View())},
		[]wiop.Table{wiop.NewTable(colB.View())},
	)
	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(colA, makeVecU64(10, 20, 30, 40))
	rt.AssignColumn(colB, makeVecU64(30, 10, 40, 20))
	require.NoError(t, p.Check(rt), "a reordering of A must pass the permutation Check")
}

// TestPermutation_Check_Failure: B differs from A as a multiset.
func TestPermutation_Check_Failure(t *testing.T) {
	sys, colA, colB := newPermutationSystem(t)
	p := sys.NewPermutation(
		sys.Context.Childf("perm"),
		[]wiop.Table{wiop.NewTable(colA.View())},
		[]wiop.Table{wiop.NewTable(colB.View())},
	)
	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(colA, makeVecU64(10, 20, 30, 40))
	rt.AssignColumn(colB, makeVecU64(30, 10, 40, 99))
	assert.Error(t, p.Check(rt), "a non-permutation must be rejected by Check")
}

func TestNewPermutation_NilCtxPanic(t *testing.T) {
	sys, colA, colB := newPermutationSystem(t)
	assert.Panics(t, func() {
		sys.NewPermutation(nil,
			[]wiop.Table{wiop.NewTable(colA.View())},
			[]wiop.Table{wiop.NewTable(colB.View())})
	})
}

func TestNewPermutation_EmptySidePanic(t *testing.T) {
	sys, colA, _ := newPermutationSystem(t)
	assert.Panics(t, func() {
		sys.NewPermutation(sys.Context.Childf("perm"),
			[]wiop.Table{wiop.NewTable(colA.View())},
			nil)
	})
}

func TestNewPermutation_WidthMismatchPanic(t *testing.T) {
	sys := wiop.NewSystemf("permW")
	r0 := sys.NewRound()
	modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionNone)
	modB := sys.NewSizedModule(sys.Context.Childf("modB"), 4, wiop.PaddingDirectionNone)
	a0 := modA.NewColumn(sys.Context.Childf("a0"), wiop.VisibilityOracle, r0)
	a1 := modA.NewColumn(sys.Context.Childf("a1"), wiop.VisibilityOracle, r0)
	b0 := modB.NewColumn(sys.Context.Childf("b0"), wiop.VisibilityOracle, r0)
	assert.Panics(t, func() {
		sys.NewPermutation(sys.Context.Childf("perm"),
			[]wiop.Table{wiop.NewTable(a0.View(), a1.View())}, // width 2
			[]wiop.Table{wiop.NewTable(b0.View())})            // width 1
	})
}

func TestNewPermutation_SelectorRejectedPanic(t *testing.T) {
	sys := wiop.NewSystemf("permSel")
	r0 := sys.NewRound()
	modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionNone)
	modB := sys.NewSizedModule(sys.Context.Childf("modB"), 4, wiop.PaddingDirectionNone)
	colA := modA.NewColumn(sys.Context.Childf("A"), wiop.VisibilityOracle, r0)
	selA := modA.NewColumn(sys.Context.Childf("selA"), wiop.VisibilityOracle, r0)
	colB := modB.NewColumn(sys.Context.Childf("B"), wiop.VisibilityOracle, r0)
	assert.Panics(t, func() {
		sys.NewPermutation(sys.Context.Childf("perm"),
			[]wiop.Table{wiop.NewFilteredTable(selA.View(), colA.View())},
			[]wiop.Table{wiop.NewTable(colB.View())})
	}, "permutation queries must reject selectors")
}

func TestLookupKind_String(t *testing.T) {
	assert.Equal(t, "Inclusion", wiop.KindInclusion.String())
	assert.Equal(t, "Permutation", wiop.KindPermutation.String())
	assert.Equal(t, "LookupKind(7)", wiop.LookupKind(7).String())
}
