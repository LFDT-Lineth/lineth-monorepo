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
	colA := modA.NewColumn(sys.Context.Childf("A"), r0)
	colB := modB.NewColumn(sys.Context.Childf("B"), r0)
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

// TestNewPermutation_MixedWidthAllowed documents that mixed-width fragments are
// now accepted at construction: the grandproduct compiler's α^w length sentinel
// keeps a width-2 row from aliasing a width-1 row, so the query is well-defined.
func TestNewPermutation_MixedWidthAllowed(t *testing.T) {
	sys := wiop.NewSystemf("permW")
	r0 := sys.NewRound()
	modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionNone)
	modB := sys.NewSizedModule(sys.Context.Childf("modB"), 4, wiop.PaddingDirectionNone)
	a0 := modA.NewColumn(sys.Context.Childf("a0"), r0)
	a1 := modA.NewColumn(sys.Context.Childf("a1"), r0)
	b0 := modB.NewColumn(sys.Context.Childf("b0"), r0)
	assert.NotPanics(t, func() {
		sys.NewPermutation(sys.Context.Childf("perm"),
			[]wiop.Table{wiop.NewTable(a0.View(), a1.View())}, // width 2
			[]wiop.Table{wiop.NewTable(b0.View())})            // width 1
	})
}

// newFilteredPermutationSystem builds a single-round system for conditional
// permutation tests: a filtered A side (colA with selA) against an unfiltered
// B side (colB), with modA twice the size of modB so only the balance-check
// skip for selectors makes the query constructible.
func newFilteredPermutationSystem(t *testing.T) (*wiop.System, *wiop.TableRelationQuery, [3]*wiop.Column) {
	t.Helper()
	sys := wiop.NewSystemf("permSel")
	r0 := sys.NewRound()
	modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionNone)
	modB := sys.NewSizedModule(sys.Context.Childf("modB"), 2, wiop.PaddingDirectionNone)
	colA := modA.NewColumn(sys.Context.Childf("A"), r0)
	selA := modA.NewColumn(sys.Context.Childf("selA"), r0)
	colB := modB.NewColumn(sys.Context.Childf("B"), r0)
	q := sys.NewPermutation(sys.Context.Childf("perm"),
		[]wiop.Table{wiop.NewFilteredTable(selA.View(), colA.View())},
		[]wiop.Table{wiop.NewTable(colB.View())})
	return sys, q, [3]*wiop.Column{colA, selA, colB}
}

// TestNewPermutation_SelectorAllowed documents that selectors are accepted on
// permutation fragments (conditional permutations), and that the static
// balance check is skipped for them: the sides here have 4 vs 2 raw rows.
func TestNewPermutation_SelectorAllowed(t *testing.T) {
	assert.NotPanics(t, func() { newFilteredPermutationSystem(t) },
		"a filtered, statically unbalanced permutation must be constructible")
}

// TestPermutation_Check_WithSelector_Completeness: the selected rows of A are a
// reordering of B; masked A rows hold junk that must be ignored.
func TestPermutation_Check_WithSelector_Completeness(t *testing.T) {
	sys, q, cols := newFilteredPermutationSystem(t)
	colA, selA, colB := cols[0], cols[1], cols[2]
	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(colA, makeVecU64(10, 99, 20, 98)) // 99, 98 masked out
	rt.AssignColumn(selA, makeVecU64(1, 0, 1, 0))
	rt.AssignColumn(colB, makeVecU64(20, 10))
	require.NoError(t, q.Check(rt),
		"selected rows of A form a permutation of B; masked junk must be ignored")
}

// TestPermutation_Check_WithSelector_Failure: a masked A row holds the value B
// expects; masking must not let it count.
func TestPermutation_Check_WithSelector_Failure(t *testing.T) {
	sys, q, cols := newFilteredPermutationSystem(t)
	colA, selA, colB := cols[0], cols[1], cols[2]
	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(colA, makeVecU64(10, 20, 30, 0)) // 20 masked out, 30 selected instead
	rt.AssignColumn(selA, makeVecU64(1, 0, 1, 0))
	rt.AssignColumn(colB, makeVecU64(20, 10))
	assert.Error(t, q.Check(rt),
		"a masked-out row must not satisfy the multiset equality")
}

// TestNewPermutation_UnbalancedStaticPanic: when every fragment is statically
// sized, a mismatch in the two sides' total row counts is caught at
// construction — such a permutation can never hold.
func TestNewPermutation_UnbalancedStaticPanic(t *testing.T) {
	sys := wiop.NewSystemf("permUnbalanced")
	r0 := sys.NewRound()
	modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionNone)
	modB := sys.NewSizedModule(sys.Context.Childf("modB"), 8, wiop.PaddingDirectionNone)
	colA := modA.NewColumn(sys.Context.Childf("A"), r0)
	colB := modB.NewColumn(sys.Context.Childf("B"), r0)
	assert.PanicsWithValue(t,
		"wiop: System.NewPermutation: the two sides have different total row counts (4 vs 8); "+
			"a permutation between multisets of different cardinalities can never hold",
		func() {
			sys.NewPermutation(sys.Context.Childf("perm"),
				[]wiop.Table{wiop.NewTable(colA.View())},
				[]wiop.Table{wiop.NewTable(colB.View())})
		})
}

// TestNewPermutation_BalancedFragmentsAllowed: fragments of different sizes are
// fine as long as the per-side totals match (4 + 4 == 8).
func TestNewPermutation_BalancedFragmentsAllowed(t *testing.T) {
	sys := wiop.NewSystemf("permBalancedFrags")
	r0 := sys.NewRound()
	modA0 := sys.NewSizedModule(sys.Context.Childf("modA0"), 4, wiop.PaddingDirectionNone)
	modA1 := sys.NewSizedModule(sys.Context.Childf("modA1"), 4, wiop.PaddingDirectionNone)
	modB := sys.NewSizedModule(sys.Context.Childf("modB"), 8, wiop.PaddingDirectionNone)
	a0 := modA0.NewColumn(sys.Context.Childf("a0"), r0)
	a1 := modA1.NewColumn(sys.Context.Childf("a1"), r0)
	b := modB.NewColumn(sys.Context.Childf("b"), r0)
	assert.NotPanics(t, func() {
		sys.NewPermutation(sys.Context.Childf("perm"),
			[]wiop.Table{wiop.NewTable(a0.View()), wiop.NewTable(a1.View())},
			[]wiop.Table{wiop.NewTable(b.View())})
	})
}

// TestNewPermutation_DynamicModuleSkipsBalanceCheck: a dynamic module's height
// is only known at runtime, so the static balance check must not fire even
// though the other side's static size cannot match "unknown".
func TestNewPermutation_DynamicModuleSkipsBalanceCheck(t *testing.T) {
	sys := wiop.NewSystemf("permDyn")
	r0 := sys.NewRound()
	modA := sys.NewDynamicModule(sys.Context.Childf("modA"), wiop.PaddingDirectionRight)
	modB := sys.NewSizedModule(sys.Context.Childf("modB"), 8, wiop.PaddingDirectionNone)
	colA := modA.NewColumn(sys.Context.Childf("A"), r0)
	colB := modB.NewColumn(sys.Context.Childf("B"), r0)
	assert.NotPanics(t, func() {
		sys.NewPermutation(sys.Context.Childf("perm"),
			[]wiop.Table{wiop.NewTable(colA.View())},
			[]wiop.Table{wiop.NewTable(colB.View())})
	})
}

func TestLookupKind_String(t *testing.T) {
	assert.Equal(t, "Inclusion", wiop.KindInclusion.String())
	assert.Equal(t, "Permutation", wiop.KindPermutation.String())
	assert.Equal(t, "LookupKind(7)", wiop.LookupKind(7).String())
}
