package grandproduct_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/grandproduct"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// condPermSystem builds the standard conditional-permutation fixture: a
// 4-row filtered A side against a 2-row unfiltered B side, single column.
// Only the two selected A rows must match B as a multiset.
func condPermSystem(t *testing.T) (*wiop.System, [3]*wiop.Column) {
	t.Helper()
	sys := wiop.NewSystemf("gp-cond")
	r0 := sys.NewRound()
	modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionNone)
	modB := sys.NewSizedModule(sys.Context.Childf("modB"), 2, wiop.PaddingDirectionNone)
	colA := modA.NewColumn(sys.Context.Childf("A"), wiop.VisibilityOracle, r0)
	selA := modA.NewColumn(sys.Context.Childf("selA"), wiop.VisibilityOracle, r0)
	colB := modB.NewColumn(sys.Context.Childf("B"), wiop.VisibilityOracle, r0)
	sys.NewPermutation(sys.Context.Childf("perm"),
		[]wiop.Table{wiop.NewFilteredTable(selA.View(), colA.View())},
		[]wiop.Table{wiop.NewTable(colB.View())})
	grandproduct.Compile(sys)
	return sys, [3]*wiop.Column{colA, selA, colB}
}

// TestCompile_ConditionalPermutation_Completeness: the selected A rows are a
// reordering of B; the masked rows hold junk that must drop out of the grand
// product via the neutral factor.
func TestCompile_ConditionalPermutation_Completeness(t *testing.T) {
	sys, cols := condPermSystem(t)
	colA, selA, colB := cols[0], cols[1], cols[2]
	proof, pub := sys.Prove(func(rt *wiop.Runtime) {
		rt.AssignColumn(colA, makeVecU64(10, 99, 20, 98)) // 99, 98 masked
		rt.AssignColumn(selA, makeVecU64(1, 0, 1, 0))
		rt.AssignColumn(colB, makeVecU64(20, 10))
	})
	require.NoError(t, sys.Verify(proof, pub),
		"a filtered permutation with matching selected rows must be accepted")
}

// TestCompile_ConditionalPermutation_SelectedMismatchRejected: a masked A row
// holds the value B expects; the neutral factor must keep it out of the
// product, so the grand product differs from one and the verifier rejects.
func TestCompile_ConditionalPermutation_SelectedMismatchRejected(t *testing.T) {
	sys, cols := condPermSystem(t)
	colA, selA, colB := cols[0], cols[1], cols[2]
	proof, pub := sys.Prove(func(rt *wiop.Runtime) {
		rt.AssignColumn(colA, makeVecU64(10, 20, 30, 0)) // 20 masked, 30 selected
		rt.AssignColumn(selA, makeVecU64(1, 0, 1, 0))
		rt.AssignColumn(colB, makeVecU64(20, 10))
	})
	assert.Error(t, sys.Verify(proof, pub),
		"a masked-out row must not satisfy the permutation")
}

// TestCompile_ConditionalPermutation_NoBinarityConstraintEmitted pins down the
// division of responsibility: the compiler does NOT constrain selectors to
// {0,1}, because selectors are typically already constrained where they are
// built and shared across queries, so emitting one here would duplicate an
// existing constraint. Binarity is a documented caller obligation
// ([wiop.System.NewPermutation]) and it is load-bearing for soundness: the
// fold is 1 + sel·(β + RLC(row) − 1), so an unconstrained selector lets a
// prover scale a row's contribution arbitrarily.
func TestCompile_ConditionalPermutation_NoBinarityConstraintEmitted(t *testing.T) {
	_, cols := condPermSystem(t)
	selA := cols[1]
	for _, v := range selA.Module.Vanishings {
		assert.NotContains(t, v.Context().Path(), "selector-binary",
			"the permutation compiler must leave selector binarity to the caller")
	}
}

// TestCompile_ConditionalPermutation_CallerBinarityConstraintRejects is the
// other half of that contract: the caller-side sel·(sel−1) = 0 constraint,
// once registered, is what rejects a non-binary selector. Asserted on the
// Vanishing directly rather than through Prove/Verify, because a witness with
// sel = 2 also skews the grand product — so an end-to-end rejection would not
// show that the binarity constraint is the thing doing the work.
func TestCompile_ConditionalPermutation_CallerBinarityConstraintRejects(t *testing.T) {
	sys := wiop.NewSystemf("gp-cond-caller-binary")
	r0 := sys.NewRound()
	modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionNone)
	modB := sys.NewSizedModule(sys.Context.Childf("modB"), 2, wiop.PaddingDirectionNone)
	colA := modA.NewColumn(sys.Context.Childf("A"), wiop.VisibilityOracle, r0)
	selA := modA.NewColumn(sys.Context.Childf("selA"), wiop.VisibilityOracle, r0)
	colB := modB.NewColumn(sys.Context.Childf("B"), wiop.VisibilityOracle, r0)

	// The caller's obligation, discharged where the selector is built.
	sel := wiop.Expression(selA.View())
	one := wiop.NewConstantField(field.One())
	binary := modA.NewVanishing(sys.Context.Childf("selA-binary"),
		wiop.Mul(sel, wiop.Sub(sel, one)))

	sys.NewPermutation(sys.Context.Childf("perm"),
		[]wiop.Table{wiop.NewFilteredTable(selA.View(), colA.View())},
		[]wiop.Table{wiop.NewTable(colB.View())})
	grandproduct.Compile(sys)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(colA, makeVecU64(10, 99, 20, 98))
	rt.AssignColumn(selA, makeVecU64(1, 0, 1, 0))
	rt.AssignColumn(colB, makeVecU64(20, 10))
	require.NoError(t, binary.Check(rt), "a {0,1} selector must satisfy the constraint")

	rt2 := wiop.NewRuntime(sys)
	rt2.AssignColumn(colA, makeVecU64(10, 99, 20, 98))
	rt2.AssignColumn(selA, makeVecU64(2, 0, 1, 0)) // 2 is not binary
	rt2.AssignColumn(colB, makeVecU64(20, 10))
	assert.Error(t, binary.Check(rt2),
		"a non-binary selector must be rejected by the caller's vanishing constraint")
}

// TestCompile_ConditionalPermutation_CardinalityImbalanceRejected: the
// selected values on each side agree wherever they pair up, but A selects
// three rows and B holds only two — the leftover β+30 factor must leave the
// grand product different from one. This is the failure mode the static
// balance check catches for unfiltered permutations and structurally cannot
// catch here: with selectors the participating row count is a witness
// property, so this runtime rejection is the only defense.
func TestCompile_ConditionalPermutation_CardinalityImbalanceRejected(t *testing.T) {
	sys, cols := condPermSystem(t)
	colA, selA, colB := cols[0], cols[1], cols[2]
	proof, pub := sys.Prove(func(rt *wiop.Runtime) {
		rt.AssignColumn(colA, makeVecU64(10, 20, 30, 0)) // selected: {10, 20, 30}
		rt.AssignColumn(selA, makeVecU64(1, 1, 1, 0))
		rt.AssignColumn(colB, makeVecU64(20, 10)) // only {20, 10}
	})
	assert.Error(t, sys.Verify(proof, pub),
		"three selected A rows can never be a permutation of two B rows")
}

// paddedCondPermSystem builds the padding fixture: a right-padded 4-row A
// module (so short assignments are padded up to 4 rows with each column's
// per-assignment Padding value) against a 2-row unfiltered B module.
func paddedCondPermSystem(t *testing.T) (*wiop.System, [3]*wiop.Column) {
	t.Helper()
	sys := wiop.NewSystemf("gp-cond-pad")
	r0 := sys.NewRound()
	modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionRight)
	modB := sys.NewSizedModule(sys.Context.Childf("modB"), 2, wiop.PaddingDirectionNone)
	colA := modA.NewColumn(sys.Context.Childf("A"), wiop.VisibilityOracle, r0)
	selA := modA.NewColumn(sys.Context.Childf("selA"), wiop.VisibilityOracle, r0)
	colB := modB.NewColumn(sys.Context.Childf("B"), wiop.VisibilityOracle, r0)
	sys.NewPermutation(sys.Context.Childf("perm"),
		[]wiop.Table{wiop.NewFilteredTable(selA.View(), colA.View())},
		[]wiop.Table{wiop.NewTable(colB.View())})
	grandproduct.Compile(sys)
	return sys, [3]*wiop.Column{colA, selA, colB}
}

// TestCompile_ConditionalPermutation_PaddingMasked pins down the intended
// padding semantics: on a right-padded module the selector's padding value is
// 0 (the ConcreteVector default), so the two synthetic padding rows are
// masked and only the two real rows participate. This is the contract the
// messagebus documents informally ("selector zero on padding rows") applied
// to conditional permutations.
func TestCompile_ConditionalPermutation_PaddingMasked(t *testing.T) {
	sys, cols := paddedCondPermSystem(t)
	colA, selA, colB := cols[0], cols[1], cols[2]
	proof, pub := sys.Prove(func(rt *wiop.Runtime) {
		// Two real rows; rows 2-3 are padding (colA pads with 0, selA with 0).
		rt.AssignColumn(colA, makeVecU64(10, 20))
		rt.AssignColumn(selA, makeVecU64(1, 1))
		rt.AssignColumn(colB, makeVecU64(20, 10))
	})
	require.NoError(t, sys.Verify(proof, pub),
		"padding rows with selector padding 0 must be masked out of the multiset")
}

// TestCompile_ConditionalPermutation_PaddingSelectedRejected is the flip side:
// a selector whose padding value is 1 pulls the data column's padding rows
// (value 0) INTO the A multiset, leaving the sides unbalanced. The verifier
// must reject — i.e. selecting padding is not silently ignored, it counts.
// Callers wanting the usual semantics must pad selectors with 0.
func TestCompile_ConditionalPermutation_PaddingSelectedRejected(t *testing.T) {
	sys, cols := paddedCondPermSystem(t)
	colA, selA, colB := cols[0], cols[1], cols[2]
	proof, pub := sys.Prove(func(rt *wiop.Runtime) {
		rt.AssignColumn(colA, makeVecU64(10, 20))
		// Selector padded with 1: the two padding rows (colA value 0) become
		// selected members of the A multiset.
		one := field.Element{}
		one.SetUint64(1)
		rt.AssignColumn(selA, &wiop.ConcreteVector{
			Plain:   makeVecU64(1, 1).Plain,
			Padding: one,
		})
		rt.AssignColumn(colB, makeVecU64(20, 10))
	})
	assert.Error(t, sys.Verify(proof, pub),
		"selector padding 1 must pull the padding rows into the multiset and unbalance it")
}

// TestCompile_ConditionalPermutation_EmptySelectionAccepted: with every row
// masked on both sides, both products consist solely of neutral factors, so
// the identity holds vacuously and the verifier accepts.
func TestCompile_ConditionalPermutation_EmptySelectionAccepted(t *testing.T) {
	sys := wiop.NewSystemf("gp-cond-empty")
	r0 := sys.NewRound()
	modA := sys.NewSizedModule(sys.Context.Childf("modA"), 2, wiop.PaddingDirectionNone)
	modB := sys.NewSizedModule(sys.Context.Childf("modB"), 2, wiop.PaddingDirectionNone)
	colA := modA.NewColumn(sys.Context.Childf("A"), wiop.VisibilityOracle, r0)
	selA := modA.NewColumn(sys.Context.Childf("selA"), wiop.VisibilityOracle, r0)
	colB := modB.NewColumn(sys.Context.Childf("B"), wiop.VisibilityOracle, r0)
	selB := modB.NewColumn(sys.Context.Childf("selB"), wiop.VisibilityOracle, r0)
	sys.NewPermutation(sys.Context.Childf("perm"),
		[]wiop.Table{wiop.NewFilteredTable(selA.View(), colA.View())},
		[]wiop.Table{wiop.NewFilteredTable(selB.View(), colB.View())})
	grandproduct.Compile(sys)

	proof, pub := sys.Prove(func(rt *wiop.Runtime) {
		rt.AssignColumn(colA, makeVecU64(10, 20))
		rt.AssignColumn(selA, makeVecU64(0, 0))
		rt.AssignColumn(colB, makeVecU64(30, 40))
		rt.AssignColumn(selB, makeVecU64(0, 0))
	})
	require.NoError(t, sys.Verify(proof, pub),
		"an all-masked permutation holds vacuously")
}

// TestCompile_ConditionalPermutation_EmptyVsNonEmptyRejected: everything
// masked on the A side against one selected B row — the lone β+RLC factor in
// the denominator must leave the product different from one.
func TestCompile_ConditionalPermutation_EmptyVsNonEmptyRejected(t *testing.T) {
	sys := wiop.NewSystemf("gp-cond-empty-vs")
	r0 := sys.NewRound()
	modA := sys.NewSizedModule(sys.Context.Childf("modA"), 2, wiop.PaddingDirectionNone)
	modB := sys.NewSizedModule(sys.Context.Childf("modB"), 2, wiop.PaddingDirectionNone)
	colA := modA.NewColumn(sys.Context.Childf("A"), wiop.VisibilityOracle, r0)
	selA := modA.NewColumn(sys.Context.Childf("selA"), wiop.VisibilityOracle, r0)
	colB := modB.NewColumn(sys.Context.Childf("B"), wiop.VisibilityOracle, r0)
	selB := modB.NewColumn(sys.Context.Childf("selB"), wiop.VisibilityOracle, r0)
	sys.NewPermutation(sys.Context.Childf("perm"),
		[]wiop.Table{wiop.NewFilteredTable(selA.View(), colA.View())},
		[]wiop.Table{wiop.NewFilteredTable(selB.View(), colB.View())})
	grandproduct.Compile(sys)

	proof, pub := sys.Prove(func(rt *wiop.Runtime) {
		rt.AssignColumn(colA, makeVecU64(10, 20))
		rt.AssignColumn(selA, makeVecU64(0, 0)) // nothing selected
		rt.AssignColumn(colB, makeVecU64(10, 40))
		rt.AssignColumn(selB, makeVecU64(1, 0)) // {10} selected
	})
	assert.Error(t, sys.Verify(proof, pub),
		"an empty selection cannot match a non-empty one")
}

// TestCompile_ConditionalPermutation_BothSidesFiltered: selectors on both
// sides, with different masked-row counts; only the selected multisets must
// coincide.
func TestCompile_ConditionalPermutation_BothSidesFiltered(t *testing.T) {
	sys := wiop.NewSystemf("gp-cond-both")
	r0 := sys.NewRound()
	modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionNone)
	modB := sys.NewSizedModule(sys.Context.Childf("modB"), 4, wiop.PaddingDirectionNone)
	colA := modA.NewColumn(sys.Context.Childf("A"), wiop.VisibilityOracle, r0)
	selA := modA.NewColumn(sys.Context.Childf("selA"), wiop.VisibilityOracle, r0)
	colB := modB.NewColumn(sys.Context.Childf("B"), wiop.VisibilityOracle, r0)
	selB := modB.NewColumn(sys.Context.Childf("selB"), wiop.VisibilityOracle, r0)
	sys.NewPermutation(sys.Context.Childf("perm"),
		[]wiop.Table{wiop.NewFilteredTable(selA.View(), colA.View())},
		[]wiop.Table{wiop.NewFilteredTable(selB.View(), colB.View())})
	grandproduct.Compile(sys)

	proof, pub := sys.Prove(func(rt *wiop.Runtime) {
		rt.AssignColumn(colA, makeVecU64(10, 77, 20, 30))
		rt.AssignColumn(selA, makeVecU64(1, 0, 1, 1)) // selected: {10, 20, 30}
		rt.AssignColumn(colB, makeVecU64(30, 10, 20, 88))
		rt.AssignColumn(selB, makeVecU64(1, 1, 1, 0)) // selected: {30, 10, 20}
	})
	require.NoError(t, sys.Verify(proof, pub),
		"both-sides-filtered permutation with matching selected multisets must be accepted")
}

// TestCompile_ConditionalPermutation_MixedFragments: a query mixing a filtered
// multi-column fragment with an unfiltered width-1 fragment on the A side.
// Exercises the selector fold composed with the α^w sentinel RLC and the
// unfiltered path within one query.
func TestCompile_ConditionalPermutation_MixedFragments(t *testing.T) {
	sys := wiop.NewSystemf("gp-cond-mixed")
	r0 := sys.NewRound()
	modA2 := sys.NewSizedModule(sys.Context.Childf("modA2"), 4, wiop.PaddingDirectionNone)
	modA1 := sys.NewSizedModule(sys.Context.Childf("modA1"), 2, wiop.PaddingDirectionNone)
	modB2 := sys.NewSizedModule(sys.Context.Childf("modB2"), 2, wiop.PaddingDirectionNone)
	modB1 := sys.NewSizedModule(sys.Context.Childf("modB1"), 2, wiop.PaddingDirectionNone)
	keyA := modA2.NewColumn(sys.Context.Childf("keyA"), wiop.VisibilityOracle, r0)
	valA := modA2.NewColumn(sys.Context.Childf("valA"), wiop.VisibilityOracle, r0)
	selA := modA2.NewColumn(sys.Context.Childf("selA"), wiop.VisibilityOracle, r0)
	colA1 := modA1.NewColumn(sys.Context.Childf("A1"), wiop.VisibilityOracle, r0)
	keyB := modB2.NewColumn(sys.Context.Childf("keyB"), wiop.VisibilityOracle, r0)
	valB := modB2.NewColumn(sys.Context.Childf("valB"), wiop.VisibilityOracle, r0)
	colB1 := modB1.NewColumn(sys.Context.Childf("B1"), wiop.VisibilityOracle, r0)

	sys.NewPermutation(sys.Context.Childf("perm"),
		[]wiop.Table{
			wiop.NewFilteredTable(selA.View(), keyA.View(), valA.View()), // width 2, filtered
			wiop.NewTable(colA1.View()),                                  // width 1, plain
		},
		[]wiop.Table{
			wiop.NewTable(keyB.View(), valB.View()), // width 2
			wiop.NewTable(colB1.View()),             // width 1
		})
	grandproduct.Compile(sys)

	proof, pub := sys.Prove(func(rt *wiop.Runtime) {
		rt.AssignColumn(keyA, makeVecU64(1, 7, 2, 8))
		rt.AssignColumn(valA, makeVecU64(10, 70, 20, 80))
		rt.AssignColumn(selA, makeVecU64(1, 0, 1, 0)) // selected pairs: (1,10), (2,20)
		rt.AssignColumn(colA1, makeVecU64(5, 6))
		rt.AssignColumn(keyB, makeVecU64(2, 1))
		rt.AssignColumn(valB, makeVecU64(20, 10))
		rt.AssignColumn(colB1, makeVecU64(6, 5))
	})
	require.NoError(t, sys.Verify(proof, pub),
		"mixed filtered/unfiltered fragments must compose with the width sentinel")
}
