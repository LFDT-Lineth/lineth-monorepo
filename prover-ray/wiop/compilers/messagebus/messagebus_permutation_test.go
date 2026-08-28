package messagebus_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/global"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/grandproduct"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/localvanishing"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/messagebus"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// compilePermutationBus runs the two passes a permutation-reduced message bus
// needs: messagebus lowers each handle into a GrandProduct, and grandproduct
// discharges it into Z columns, the Result assignment, and the
// final-product/in-shard verifier actions.
//
// It deliberately stops short of the PCS pass, so the columns never make it into
// the Fiat-Shamir transcript and the α/β coins are constants. That is fine for
// the completeness tests, and for soundness tests whose rejection does not
// depend on the coins being unpredictable. Anything that folds several columns
// into one tuple does depend on it -- with α = 0 a tuple collapses onto its
// first column -- and must use [compilePermutationBusWithPCS] instead.
func compilePermutationBus(sys *wiop.System) {
	messagebus.Compile(sys)
	grandproduct.Compile(sys)
}

// compilePermutationBusWithPCS runs the message-bus pipeline all the way through
// the polynomial commitment pass. The PCS pass is what binds the witness into
// the Fiat-Shamir transcript (it absorbs one commitment per committed round in
// place of that round's columns), which is what makes the α/β coins depend on
// the witness and therefore unpredictable to the prover.
//
// The local-vanishing and global passes are not optional here: PCS opens
// LagrangeEval claims, and those two passes are what turn the grand-product
// vanishings into such claims.
func compilePermutationBusWithPCS(sys *wiop.System) {
	compilePermutationBus(sys)
	localvanishing.Compile(sys)
	global.Compile(sys)
	pcs.Compile(sys)
}

// TestCompile_Permutation_Balanced: one handle, one permutation Send and one
// permutation Receive whose row multisets coincide (B is a reordering of A).
// The shard's product accumulator is one and the verifier accepts.
func TestCompile_Permutation_Balanced(t *testing.T) {
	runWithAndWithoutHook(t, func(t *testing.T, sys *wiop.System, r0 *wiop.Round) {
		t.Helper()
		modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionNone)
		modB := sys.NewSizedModule(sys.Context.Childf("modB"), 4, wiop.PaddingDirectionNone)
		colA := modA.NewColumn(sys.Context.Childf("A"), r0)
		colB := modB.NewColumn(sys.Context.Childf("B"), r0)

		sys.NewMessageBusSend(
			sys.Context.Childf("send-A"), "shard", "route", wiop.NewTable(colA.View()))
		sys.NewMessageBusReceive(
			sys.Context.Childf("recv-B"), "shard", "route", wiop.NewTable(colB.View()))

		compilePermutationBus(sys)

		rt := wiop.NewRuntime(sys)
		rt.AssignColumn(colA, makeVec(10, 20, 30, 40))
		rt.AssignColumn(colB, makeVec(40, 10, 30, 20)) // a reordering of A

		drive(rt)
		require.NoError(t, checkAllVerifierActions(rt),
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
		colA := modA.NewColumn(sys.Context.Childf("A"), r0)
		colB := modB.NewColumn(sys.Context.Childf("B"), r0)

		sys.NewMessageBusSend(
			sys.Context.Childf("send-A"), "shard", "route", wiop.NewTable(colA.View()))
		sys.NewMessageBusReceive(
			sys.Context.Childf("recv-B"), "shard", "route", wiop.NewTable(colB.View()))

		compilePermutationBus(sys)

		rt := wiop.NewRuntime(sys)
		rt.AssignColumn(colA, makeVec(10, 20, 30, 40))
		rt.AssignColumn(colB, makeVec(40, 10, 30, 99)) // 20 replaced by 99

		drive(rt)
		assert.Error(t, checkAllVerifierActions(rt),
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
	colA := modA.NewColumn(sys.Context.Childf("A"), r0)
	selA := modA.NewColumn(sys.Context.Childf("selA"), r0)
	colB := modB.NewColumn(sys.Context.Childf("B"), r0)
	selB := modB.NewColumn(sys.Context.Childf("selB"), r0)

	sys.NewMessageBusSend(
		sys.Context.Childf("send-A"), "shard", "route",
		wiop.NewFilteredTable(selA.View(), colA.View()))
	sys.NewMessageBusReceive(
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

	drive(rt)
	require.NoError(t, checkAllVerifierActions(rt),
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
	colA := modA.NewColumn(sys.Context.Childf("A"), r0)
	selA := modA.NewColumn(sys.Context.Childf("selA"), r0)
	colB := modB.NewColumn(sys.Context.Childf("B"), r0)
	selB := modB.NewColumn(sys.Context.Childf("selB"), r0)

	sys.NewMessageBusSend(
		sys.Context.Childf("send-A"), "shard", "route",
		wiop.NewFilteredTable(selA.View(), colA.View()))
	sys.NewMessageBusReceive(
		sys.Context.Childf("recv-B"), "shard", "route",
		wiop.NewFilteredTable(selB.View(), colB.View()))

	compilePermutationBus(sys)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(colA, makeVec(10, 20, 30, 40))
	rt.AssignColumn(selA, makeVec(1, 1, 0, 0)) // selected {10, 20}
	rt.AssignColumn(colB, makeVec(10, 30, 20, 40))
	rt.AssignColumn(selB, makeVec(1, 1, 0, 0)) // selected {10, 30} ≠ {10, 20}

	drive(rt)
	assert.Error(t, checkAllVerifierActions(rt),
		"a filtered permutation with mismatched selected multisets must be rejected")
}

// TestNewMessageBus_SetsDirectionAndAllowsSelector checks the constructors set
// the right direction and permit a selector (a permutation may have only some
// rows active).
func TestNewMessageBus_SetsDirectionAndAllowsSelector(t *testing.T) {
	sys := wiop.NewSystemf("mb-perm-ctor")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("c"), r0)
	sel := mod.NewColumn(sys.Context.Childf("sel"), r0)

	send := sys.NewMessageBusSend(
		sys.Context.Childf("send"), "shard", "route", wiop.NewTable(col.View()))
	assert.Equal(t, wiop.BusSend, send.Direction)

	recv := sys.NewMessageBusReceive(
		sys.Context.Childf("recv"), "shard", "route",
		wiop.NewFilteredTable(sel.View(), col.View()))
	assert.Equal(t, wiop.BusReceive, recv.Direction)
	assert.NotNil(t, recv.Tab.Selector, "a selector is allowed on a permutation entry")
}
