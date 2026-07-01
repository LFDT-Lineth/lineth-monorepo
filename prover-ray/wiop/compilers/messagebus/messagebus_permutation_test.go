package messagebus_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/grandproduct"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/messagebus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// compilePermutationBus runs the two passes a permutation-reduced message bus
// needs: messagebus lowers each handle into a GrandProduct, and grandproduct
// discharges it into Z columns, the Result assignment, and the
// final-product/in-shard verifier actions.
func compilePermutationBus(sys *wiop.System) {
	messagebus.Compile(sys)
	grandproduct.Compile(sys)
}

// TestCompile_Permutation_Balanced: one handle, one permutation Send and one
// permutation Receive whose row multisets coincide (B is a reordering of A).
// The shard's product accumulator is one and the verifier accepts.
func TestCompile_Permutation_Balanced(t *testing.T) {
	runWithAndWithoutHook(t, func(t *testing.T, sys *wiop.System, r0 *wiop.Round) {
		t.Helper()
		modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionNone)
		modB := sys.NewSizedModule(sys.Context.Childf("modB"), 4, wiop.PaddingDirectionNone)
		colA := modA.NewColumn(sys.Context.Childf("A"), wiop.VisibilityOracle, r0)
		colB := modB.NewColumn(sys.Context.Childf("B"), wiop.VisibilityOracle, r0)

		sys.NewMessageBusPermutationSend(
			sys.Context.Childf("send-A"), "shard", "route", wiop.NewTable(colA.View()))
		sys.NewMessageBusPermutationReceive(
			sys.Context.Childf("recv-B"), "shard", "route", wiop.NewTable(colB.View()))

		compilePermutationBus(sys)

		rt := wiop.NewRuntime(sys)
		rt.AssignColumn(colA, makeVec(10, 20, 30, 40))
		rt.AssignColumn(colB, makeVec(40, 10, 30, 20)) // a reordering of A

		drive(&rt)
		require.NoError(t, checkAllVerifierActions(&rt),
			"a balanced permutation must be accepted")
	})
}

// TestCompile_Permutation_Unbalanced: the receive multiset differs from the
// send multiset on one row, so the product accumulator is not one and the
// in-shard check rejects.
func TestCompile_Permutation_Unbalanced(t *testing.T) {
	runWithAndWithoutHook(t, func(t *testing.T, sys *wiop.System, r0 *wiop.Round) {
		t.Helper()
		modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionNone)
		modB := sys.NewSizedModule(sys.Context.Childf("modB"), 4, wiop.PaddingDirectionNone)
		colA := modA.NewColumn(sys.Context.Childf("A"), wiop.VisibilityOracle, r0)
		colB := modB.NewColumn(sys.Context.Childf("B"), wiop.VisibilityOracle, r0)

		sys.NewMessageBusPermutationSend(
			sys.Context.Childf("send-A"), "shard", "route", wiop.NewTable(colA.View()))
		sys.NewMessageBusPermutationReceive(
			sys.Context.Childf("recv-B"), "shard", "route", wiop.NewTable(colB.View()))

		compilePermutationBus(sys)

		rt := wiop.NewRuntime(sys)
		rt.AssignColumn(colA, makeVec(10, 20, 30, 40))
		rt.AssignColumn(colB, makeVec(40, 10, 30, 99)) // 20 replaced by 99

		drive(&rt)
		assert.Error(t, checkAllVerifierActions(&rt),
			"a non-permutation must be rejected")
	})
}

// TestCompile_Permutation_WithSelectorBalanced: selectors restrict
// participation to a subset of rows; the selected multisets coincide, so the
// verifier accepts even though the unselected rows differ.
func TestCompile_Permutation_WithSelectorBalanced(t *testing.T) {
	sys := wiop.NewSystemf("mb-perm-sel-ok")
	r0 := sys.NewRound()
	modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionNone)
	modB := sys.NewSizedModule(sys.Context.Childf("modB"), 4, wiop.PaddingDirectionNone)
	colA := modA.NewColumn(sys.Context.Childf("A"), wiop.VisibilityOracle, r0)
	selA := modA.NewColumn(sys.Context.Childf("selA"), wiop.VisibilityOracle, r0)
	colB := modB.NewColumn(sys.Context.Childf("B"), wiop.VisibilityOracle, r0)
	selB := modB.NewColumn(sys.Context.Childf("selB"), wiop.VisibilityOracle, r0)

	sys.NewMessageBusPermutationSend(
		sys.Context.Childf("send-A"), "shard", "route",
		wiop.NewFilteredTable(selA.View(), colA.View()))
	sys.NewMessageBusPermutationReceive(
		sys.Context.Childf("recv-B"), "shard", "route",
		wiop.NewFilteredTable(selB.View(), colB.View()))

	compilePermutationBus(sys)

	rt := wiop.NewRuntime(sys)
	// Selected A rows = {10, 20}; unselected rows carry junk that must not matter.
	rt.AssignColumn(colA, makeVec(10, 20, 77, 88))
	rt.AssignColumn(selA, makeVec(1, 1, 0, 0))
	// Selected B rows = {20, 10}; unselected rows differ from A's.
	rt.AssignColumn(colB, makeVec(66, 20, 10, 55))
	rt.AssignColumn(selB, makeVec(0, 1, 1, 0))

	drive(&rt)
	require.NoError(t, checkAllVerifierActions(&rt),
		"a filtered permutation with matching selected multisets must be accepted")
}

// TestCompile_Permutation_WithSelectorUnbalanced: the selected multisets
// differ, so the verifier rejects even though the full (unfiltered) columns
// would match.
func TestCompile_Permutation_WithSelectorUnbalanced(t *testing.T) {
	sys := wiop.NewSystemf("mb-perm-sel-bad")
	r0 := sys.NewRound()
	modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionNone)
	modB := sys.NewSizedModule(sys.Context.Childf("modB"), 4, wiop.PaddingDirectionNone)
	colA := modA.NewColumn(sys.Context.Childf("A"), wiop.VisibilityOracle, r0)
	selA := modA.NewColumn(sys.Context.Childf("selA"), wiop.VisibilityOracle, r0)
	colB := modB.NewColumn(sys.Context.Childf("B"), wiop.VisibilityOracle, r0)
	selB := modB.NewColumn(sys.Context.Childf("selB"), wiop.VisibilityOracle, r0)

	sys.NewMessageBusPermutationSend(
		sys.Context.Childf("send-A"), "shard", "route",
		wiop.NewFilteredTable(selA.View(), colA.View()))
	sys.NewMessageBusPermutationReceive(
		sys.Context.Childf("recv-B"), "shard", "route",
		wiop.NewFilteredTable(selB.View(), colB.View()))

	compilePermutationBus(sys)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(colA, makeVec(10, 20, 30, 40))
	rt.AssignColumn(selA, makeVec(1, 1, 0, 0)) // selected {10, 20}
	rt.AssignColumn(colB, makeVec(10, 30, 20, 40))
	rt.AssignColumn(selB, makeVec(1, 1, 0, 0)) // selected {10, 30} ≠ {10, 20}

	drive(&rt)
	assert.Error(t, checkAllVerifierActions(&rt),
		"a filtered permutation with mismatched selected multisets must be rejected")
}

// TestCompile_Permutation_MixedReductionSameHandlePanics: a handle cannot mix a
// log-derivative entry and a permutation entry — the compiler rejects it.
func TestCompile_Permutation_MixedReductionSameHandlePanics(t *testing.T) {
	sys := wiop.NewSystemf("mb-mixed")
	r0 := sys.NewRound()
	modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionNone)
	modB := sys.NewSizedModule(sys.Context.Childf("modB"), 4, wiop.PaddingDirectionNone)
	colA := modA.NewColumn(sys.Context.Childf("A"), wiop.VisibilityOracle, r0)
	colB := modB.NewColumn(sys.Context.Childf("B"), wiop.VisibilityOracle, r0)

	sys.NewMessageBusSend( // LogUp
		sys.Context.Childf("send-A"), "shard", "route", wiop.NewTable(colA.View()))
	sys.NewMessageBusPermutationReceive( // Permutation — same handle
		sys.Context.Childf("recv-B"), "shard", "route", wiop.NewTable(colB.View()))

	assert.Panics(t, func() { messagebus.Compile(sys) },
		"mixing reductions within a handle must panic")
}

// TestNewMessageBusPermutation_SetsReductionAndAllowsSelector checks the
// constructors set ReducePermutation and permit a selector (a permutation may
// have only some rows active).
func TestNewMessageBusPermutation_SetsReductionAndAllowsSelector(t *testing.T) {
	sys := wiop.NewSystemf("mb-perm-ctor")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("c"), wiop.VisibilityOracle, r0)
	sel := mod.NewColumn(sys.Context.Childf("sel"), wiop.VisibilityOracle, r0)

	send := sys.NewMessageBusPermutationSend(
		sys.Context.Childf("send"), "shard", "route", wiop.NewTable(col.View()))
	assert.Equal(t, wiop.ReducePermutation, send.Reduction)
	assert.Equal(t, wiop.BusSend, send.Direction)

	recv := sys.NewMessageBusPermutationReceive(
		sys.Context.Childf("recv"), "shard", "route",
		wiop.NewFilteredTable(sel.View(), col.View()))
	assert.Equal(t, wiop.ReducePermutation, recv.Reduction)
	assert.Nil(t, recv.Multiplicity, "permutation receive carries no multiplicity")
	assert.NotNil(t, recv.Tab.Selector, "a selector is allowed on a permutation entry")
}
