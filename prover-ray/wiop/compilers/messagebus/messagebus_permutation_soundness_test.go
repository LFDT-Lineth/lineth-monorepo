package messagebus_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/messagebus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This file ports the completeness and soundness coverage that used to live on
// the (now-removed) log-derivative message-bus path to the grand-product
// (permutation) path. The multiplicity-specific scenarios have no permutation
// analogue — the grand product proves multiset equality and every selected row
// counts exactly once — so they are intentionally absent.

// TestCompile_Permutation_NoMessageBusesIsNoOp asserts that running the
// compiler against a system with no message-bus queries is a no-op: no rounds,
// coins, or verifier actions are appended.
func TestCompile_Permutation_NoMessageBusesIsNoOp(t *testing.T) {
	sys := wiop.NewSystemf("mb-perm-empty")
	sys.NewRound()

	roundsBefore := len(sys.Rounds)
	messagebus.Compile(sys)
	assert.Len(t, sys.Rounds, roundsBefore,
		"Compile must not append rounds when there are no MessageBus queries")
}

// TestCompile_Permutation_MixedOriginShardPanics asserts that Compile rejects a
// system containing unreduced MessageBus entries from more than one shard.
// [messagebus.Compile] is a per-shard operation; mixing OriginShard values in a
// single invocation is a misuse the compiler catches before emitting artefacts.
func TestCompile_Permutation_MixedOriginShardPanics(t *testing.T) {
	sys := wiop.NewSystemf("mb-perm-mixed-shards")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 2, wiop.PaddingDirectionNone)
	colA := mod.NewColumn(sys.Context.Childf("A"), wiop.VisibilityOracle, r0)
	colB := mod.NewColumn(sys.Context.Childf("B"), wiop.VisibilityOracle, r0)

	sys.NewMessageBusSend(
		sys.Context.Childf("send-shardA"), "shardA", "h", wiop.NewTable(colA.View()))
	sys.NewMessageBusSend(
		sys.Context.Childf("send-shardB"), "shardB", "h", wiop.NewTable(colB.View()))

	assert.Panics(t, func() { messagebus.Compile(sys) },
		"Compile must panic when MessageBus entries straddle different OriginShard values")
}

// TestCompile_Permutation_MultiColumnTuples covers width-2 tables: (key, value)
// tuples sent and received. The compiler must allocate an α coin (in addition
// to β) and fold each tuple via Horner. A reordering of the send multiset on
// the receive side keeps the product at one.
func TestCompile_Permutation_MultiColumnTuples(t *testing.T) {
	runWithAndWithoutHook(t, func(t *testing.T, sys *wiop.System, r0 *wiop.Round) {
		t.Helper()
		modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionNone)
		modB := sys.NewSizedModule(sys.Context.Childf("modB"), 4, wiop.PaddingDirectionNone)
		keyA := modA.NewColumn(sys.Context.Childf("kA"), wiop.VisibilityOracle, r0)
		valA := modA.NewColumn(sys.Context.Childf("vA"), wiop.VisibilityOracle, r0)
		keyB := modB.NewColumn(sys.Context.Childf("kB"), wiop.VisibilityOracle, r0)
		valB := modB.NewColumn(sys.Context.Childf("vB"), wiop.VisibilityOracle, r0)

		sys.NewMessageBusSend(
			sys.Context.Childf("send-A"), "shard", "kv",
			wiop.NewTable(keyA.View(), valA.View()))
		sys.NewMessageBusReceive(
			sys.Context.Childf("recv-B"), "shard", "kv",
			wiop.NewTable(keyB.View(), valB.View()))

		compilePermutationBus(sys)

		rt := wiop.NewRuntime(sys)
		rt.AssignColumn(keyA, makeVec(1, 2, 3, 4))
		rt.AssignColumn(valA, makeVec(10, 20, 30, 40))
		// B is a reordering of A's (key, value) tuples.
		rt.AssignColumn(keyB, makeVec(3, 1, 4, 2))
		rt.AssignColumn(valB, makeVec(30, 10, 40, 20))

		drive(&rt)
		require.NoError(t, checkAllVerifierActions(&rt),
			"a balanced width-2 permutation must be accepted")
	})
}

// TestCompile_Permutation_MultiColumnTuples_Unbalanced is the soundness
// counterpart: one receive tuple's value disagrees with every send tuple, so
// the folded factors differ and the product is not one.
func TestCompile_Permutation_MultiColumnTuples_Unbalanced(t *testing.T) {
	runWithAndWithoutHook(t, func(t *testing.T, sys *wiop.System, r0 *wiop.Round) {
		t.Helper()
		modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionNone)
		modB := sys.NewSizedModule(sys.Context.Childf("modB"), 4, wiop.PaddingDirectionNone)
		keyA := modA.NewColumn(sys.Context.Childf("kA"), wiop.VisibilityOracle, r0)
		valA := modA.NewColumn(sys.Context.Childf("vA"), wiop.VisibilityOracle, r0)
		keyB := modB.NewColumn(sys.Context.Childf("kB"), wiop.VisibilityOracle, r0)
		valB := modB.NewColumn(sys.Context.Childf("vB"), wiop.VisibilityOracle, r0)

		sys.NewMessageBusSend(
			sys.Context.Childf("send-A"), "shard", "kv",
			wiop.NewTable(keyA.View(), valA.View()))
		sys.NewMessageBusReceive(
			sys.Context.Childf("recv-B"), "shard", "kv",
			wiop.NewTable(keyB.View(), valB.View()))

		compilePermutationBus(sys)

		rt := wiop.NewRuntime(sys)
		rt.AssignColumn(keyA, makeVec(1, 2, 3, 4))
		rt.AssignColumn(valA, makeVec(10, 20, 30, 40))
		rt.AssignColumn(keyB, makeVec(1, 2, 3, 4))
		rt.AssignColumn(valB, makeVec(10, 21, 30, 40)) // (2,21) is not a sent tuple

		drive(&rt)
		assert.Error(t, checkAllVerifierActions(&rt),
			"a width-2 permutation with a mismatched tuple must be rejected")
	})
}

// TestCompile_Permutation_TamperedValueFailsInShardCheck exercises the in-shard
// product rejection through a tampered VALUE column: the send and receive
// multisets disagree on a single row's value, so the folded factors differ and
// the per-handle product lands off one.
func TestCompile_Permutation_TamperedValueFailsInShardCheck(t *testing.T) {
	runWithAndWithoutHook(t, func(t *testing.T, sys *wiop.System, r0 *wiop.Round) {
		t.Helper()
		modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionNone)
		modB := sys.NewSizedModule(sys.Context.Childf("modB"), 4, wiop.PaddingDirectionNone)
		colA := modA.NewColumn(sys.Context.Childf("A"), wiop.VisibilityOracle, r0)
		colB := modB.NewColumn(sys.Context.Childf("B"), wiop.VisibilityOracle, r0)

		sys.NewMessageBusSend(
			sys.Context.Childf("send-A"), "shard", "ping", wiop.NewTable(colA.View()))
		sys.NewMessageBusReceive(
			sys.Context.Childf("recv-B"), "shard", "ping", wiop.NewTable(colB.View()))

		compilePermutationBus(sys)

		rt := wiop.NewRuntime(sys)
		rt.AssignColumn(colA, makeVec(10, 20, 30, 40))
		rt.AssignColumn(colB, makeVec(10, 21, 30, 40)) // wrong: row 1 holds 21, not 20

		drive(&rt)
		assert.Error(t, checkAllVerifierActions(&rt),
			"verifier must reject when a receive row's value does not appear in the send multiset")
	})
}

// TestCompile_Permutation_TamperedFilterFailsInShardCheck exercises the
// in-shard product rejection through an ASYMMETRIC selector: the send side
// filters out a row that the receive side still claims, so the multisets have
// different cardinalities and the per-handle product is not one.
func TestCompile_Permutation_TamperedFilterFailsInShardCheck(t *testing.T) {
	runWithAndWithoutHook(t, func(t *testing.T, sys *wiop.System, r0 *wiop.Round) {
		t.Helper()
		modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionNone)
		modB := sys.NewSizedModule(sys.Context.Childf("modB"), 4, wiop.PaddingDirectionNone)
		colA := modA.NewColumn(sys.Context.Childf("A"), wiop.VisibilityOracle, r0)
		selA := modA.NewColumn(sys.Context.Childf("selA"), wiop.VisibilityOracle, r0)
		colB := modB.NewColumn(sys.Context.Childf("B"), wiop.VisibilityOracle, r0)

		sys.NewMessageBusSend(
			sys.Context.Childf("send-A"), "shard", "ping",
			wiop.NewFilteredTable(selA.View(), colA.View()))
		sys.NewMessageBusReceive(
			sys.Context.Childf("recv-B"), "shard", "ping",
			wiop.NewTable(colB.View()))

		compilePermutationBus(sys)

		rt := wiop.NewRuntime(sys)
		rt.AssignColumn(colA, makeVec(10, 20, 30, 40))
		rt.AssignColumn(selA, makeVec(1, 0, 1, 1)) // sender filters out row 1 (value 20)
		rt.AssignColumn(colB, makeVec(10, 20, 30, 40))

		drive(&rt)
		assert.Error(t, checkAllVerifierActions(&rt),
			"verifier must reject when the send-side selector drops a row the receive side still claims")
	})
}

// TestCompile_Permutation_MultipleSendersOneReceiver verifies that several Send
// entries on the same handle balance against a single Receive entry when their
// combined multisets coincide. Sender 1 emits [10, 20]; sender 2 emits
// [30, 40]; the receiver holds all four (reordered). All entries share a shard.
func TestCompile_Permutation_MultipleSendersOneReceiver(t *testing.T) {
	runWithAndWithoutHook(t, func(t *testing.T, sys *wiop.System, r0 *wiop.Round) {
		t.Helper()
		modS1 := sys.NewSizedModule(sys.Context.Childf("modS1"), 2, wiop.PaddingDirectionNone)
		modS2 := sys.NewSizedModule(sys.Context.Childf("modS2"), 2, wiop.PaddingDirectionNone)
		modR := sys.NewSizedModule(sys.Context.Childf("modR"), 4, wiop.PaddingDirectionNone)
		colS1 := modS1.NewColumn(sys.Context.Childf("S1"), wiop.VisibilityOracle, r0)
		colS2 := modS2.NewColumn(sys.Context.Childf("S2"), wiop.VisibilityOracle, r0)
		colR := modR.NewColumn(sys.Context.Childf("R"), wiop.VisibilityOracle, r0)

		sys.NewMessageBusSend(
			sys.Context.Childf("send-S1"), "shard", "bus", wiop.NewTable(colS1.View()))
		sys.NewMessageBusSend(
			sys.Context.Childf("send-S2"), "shard", "bus", wiop.NewTable(colS2.View()))
		sys.NewMessageBusReceive(
			sys.Context.Childf("recv-R"), "shard", "bus", wiop.NewTable(colR.View()))

		compilePermutationBus(sys)

		rt := wiop.NewRuntime(sys)
		rt.AssignColumn(colS1, makeVec(10, 20))
		rt.AssignColumn(colS2, makeVec(30, 40))
		rt.AssignColumn(colR, makeVec(40, 10, 30, 20)) // union of both senders, reordered

		drive(&rt)
		require.NoError(t, checkAllVerifierActions(&rt),
			"two senders balancing one receiver on the union multiset must be accepted")
	})
}

// TestCompile_Permutation_TwoHandlesIndependent verifies that two unrelated
// handles in the same system are checked independently: they share the global
// (α, β) coins but each handle still gets its own GrandProduct cells and its
// own verifier action, so tampering one handle cannot mask the other.
func TestCompile_Permutation_TwoHandlesIndependent(t *testing.T) {
	runWithAndWithoutHook(t, func(t *testing.T, sys *wiop.System, r0 *wiop.Round) {
		t.Helper()
		modA := sys.NewSizedModule(sys.Context.Childf("modA"), 2, wiop.PaddingDirectionNone)
		modB := sys.NewSizedModule(sys.Context.Childf("modB"), 2, wiop.PaddingDirectionNone)
		modC := sys.NewSizedModule(sys.Context.Childf("modC"), 2, wiop.PaddingDirectionNone)
		modD := sys.NewSizedModule(sys.Context.Childf("modD"), 2, wiop.PaddingDirectionNone)
		colA := modA.NewColumn(sys.Context.Childf("A"), wiop.VisibilityOracle, r0)
		colB := modB.NewColumn(sys.Context.Childf("B"), wiop.VisibilityOracle, r0)
		colC := modC.NewColumn(sys.Context.Childf("C"), wiop.VisibilityOracle, r0)
		colD := modD.NewColumn(sys.Context.Childf("D"), wiop.VisibilityOracle, r0)

		sys.NewMessageBusSend(
			sys.Context.Childf("send-A"), "shard", "alpha", wiop.NewTable(colA.View()))
		sys.NewMessageBusReceive(
			sys.Context.Childf("recv-B"), "shard", "alpha", wiop.NewTable(colB.View()))
		sys.NewMessageBusSend(
			sys.Context.Childf("send-C"), "shard", "beta", wiop.NewTable(colC.View()))
		sys.NewMessageBusReceive(
			sys.Context.Childf("recv-D"), "shard", "beta", wiop.NewTable(colD.View()))

		compilePermutationBus(sys)

		rt := wiop.NewRuntime(sys)
		rt.AssignColumn(colA, makeVec(7, 8))
		rt.AssignColumn(colB, makeVec(8, 7)) // reordering of alpha
		rt.AssignColumn(colC, makeVec(100, 200))
		rt.AssignColumn(colD, makeVec(200, 100)) // reordering of beta

		drive(&rt)
		require.NoError(t, checkAllVerifierActions(&rt),
			"two independent balanced handles sharing (α, β) must both be accepted")
	})
}

// TestCompile_Permutation_MixedWidth_Balanced covers participants of DIFFERENT
// widths on a single handle. A width-2 send/receive sub-group and a width-1
// send/receive sub-group each balance internally; the α^w length sentinel keeps
// the two sub-groups from interfering, so the product is one and the verifier
// accepts.
func TestCompile_Permutation_MixedWidth_Balanced(t *testing.T) {
	runWithAndWithoutHook(t, func(t *testing.T, sys *wiop.System, r0 *wiop.Round) {
		t.Helper()
		// Width-2 sub-group.
		modS2 := sys.NewSizedModule(sys.Context.Childf("modS2"), 2, wiop.PaddingDirectionNone)
		modR2 := sys.NewSizedModule(sys.Context.Childf("modR2"), 2, wiop.PaddingDirectionNone)
		keyS := modS2.NewColumn(sys.Context.Childf("keyS"), wiop.VisibilityOracle, r0)
		valS := modS2.NewColumn(sys.Context.Childf("valS"), wiop.VisibilityOracle, r0)
		keyR := modR2.NewColumn(sys.Context.Childf("keyR"), wiop.VisibilityOracle, r0)
		valR := modR2.NewColumn(sys.Context.Childf("valR"), wiop.VisibilityOracle, r0)

		// Width-1 sub-group.
		modS1 := sys.NewSizedModule(sys.Context.Childf("modS1"), 2, wiop.PaddingDirectionNone)
		modR1 := sys.NewSizedModule(sys.Context.Childf("modR1"), 2, wiop.PaddingDirectionNone)
		colS1 := modS1.NewColumn(sys.Context.Childf("S1"), wiop.VisibilityOracle, r0)
		colR1 := modR1.NewColumn(sys.Context.Childf("R1"), wiop.VisibilityOracle, r0)

		sys.NewMessageBusSend(
			sys.Context.Childf("send-w2"), "shard", "mixed",
			wiop.NewTable(keyS.View(), valS.View()))
		sys.NewMessageBusReceive(
			sys.Context.Childf("recv-w2"), "shard", "mixed",
			wiop.NewTable(keyR.View(), valR.View()))
		sys.NewMessageBusSend(
			sys.Context.Childf("send-w1"), "shard", "mixed",
			wiop.NewTable(colS1.View()))
		sys.NewMessageBusReceive(
			sys.Context.Childf("recv-w1"), "shard", "mixed",
			wiop.NewTable(colR1.View()))

		compilePermutationBus(sys)

		rt := wiop.NewRuntime(sys)
		rt.AssignColumn(keyS, makeVec(1, 2))
		rt.AssignColumn(valS, makeVec(10, 20))
		rt.AssignColumn(keyR, makeVec(2, 1)) // reordering of the width-2 tuples
		rt.AssignColumn(valR, makeVec(20, 10))
		rt.AssignColumn(colS1, makeVec(5, 6))
		rt.AssignColumn(colR1, makeVec(6, 5)) // reordering of the width-1 rows

		drive(&rt)
		require.NoError(t, checkAllVerifierActions(&rt),
			"a handle mixing width-1 and width-2 participants that balance internally must be accepted")
	})
}

// TestCompile_Permutation_MixedWidth_SentinelPreventsAliasing is the soundness
// counterpart: a width-1 send of value v must NOT be cancellable by a width-2
// receive of (0, v). Without the α^w length sentinel both fold to β + v and the
// product would spuriously equal one; the sentinel gives the width-1 row
// β + α + v and the width-2 row β + α² + v, so the product is off one and the
// verifier rejects.
func TestCompile_Permutation_MixedWidth_SentinelPreventsAliasing(t *testing.T) {
	runWithAndWithoutHook(t, func(t *testing.T, sys *wiop.System, r0 *wiop.Round) {
		t.Helper()
		modS1 := sys.NewSizedModule(sys.Context.Childf("modS1"), 2, wiop.PaddingDirectionNone)
		modR2 := sys.NewSizedModule(sys.Context.Childf("modR2"), 2, wiop.PaddingDirectionNone)
		colS1 := modS1.NewColumn(sys.Context.Childf("S1"), wiop.VisibilityOracle, r0)
		hiR := modR2.NewColumn(sys.Context.Childf("hiR"), wiop.VisibilityOracle, r0)
		loR := modR2.NewColumn(sys.Context.Childf("loR"), wiop.VisibilityOracle, r0)

		sys.NewMessageBusSend(
			sys.Context.Childf("send-w1"), "shard", "alias", wiop.NewTable(colS1.View()))
		// A width-2 receive that zero-pads the leading column, trying to consume
		// the width-1 send as (0, v).
		sys.NewMessageBusReceive(
			sys.Context.Childf("recv-w2"), "shard", "alias",
			wiop.NewTable(hiR.View(), loR.View()))

		compilePermutationBus(sys)

		rt := wiop.NewRuntime(sys)
		rt.AssignColumn(colS1, makeVec(5, 6))
		rt.AssignColumn(hiR, makeVec(0, 0))
		rt.AssignColumn(loR, makeVec(5, 6))

		drive(&rt)
		assert.Error(t, checkAllVerifierActions(&rt),
			"the length sentinel must stop a width-2 (0,v) receive from aliasing a width-1 v send")
	})
}

// TestCompile_Permutation_DynamicModule_Balanced is the completeness case on
// dynamic modules: the participating modules' sizes are established at runtime
// by the first column assignment. The same compiled System is re-driven across
// two runtime sizes to confirm size-agnostic compilation.
func TestCompile_Permutation_DynamicModule_Balanced(t *testing.T) {
	sys := wiop.NewSystemf("mb-perm-dyn-balanced")
	r0 := sys.NewRound()
	setupMessageBusHook(sys)

	modA := sys.NewDynamicModule(sys.Context.Childf("modA"), wiop.PaddingDirectionRight)
	modB := sys.NewDynamicModule(sys.Context.Childf("modB"), wiop.PaddingDirectionRight)
	colA := modA.NewColumn(sys.Context.Childf("A"), wiop.VisibilityOracle, r0)
	colB := modB.NewColumn(sys.Context.Childf("B"), wiop.VisibilityOracle, r0)

	sys.NewMessageBusSend(
		sys.Context.Childf("send-A"), "shard", "ping", wiop.NewTable(colA.View()))
	sys.NewMessageBusReceive(
		sys.Context.Childf("recv-B"), "shard", "ping", wiop.NewTable(colB.View()))

	compilePermutationBus(sys)

	cases := []struct {
		name     string
		send     []uint64
		recvPerm []uint64
	}{
		{"size-4", []uint64{10, 20, 30, 40}, []uint64{40, 10, 30, 20}},
		{"size-8", []uint64{1, 2, 3, 4, 5, 6, 7, 8}, []uint64{8, 1, 6, 3, 4, 5, 2, 7}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rt := wiop.NewRuntime(sys)
			rt.AssignColumn(colA, makeVec(tc.send...))
			rt.AssignColumn(colB, makeVec(tc.recvPerm...))

			drive(&rt)
			require.NoError(t, checkAllVerifierActions(&rt),
				"a balanced permutation on a dynamic module must be accepted at any runtime size")
		})
	}
}

// TestCompile_Permutation_DynamicModule_TamperedValueFails is the soundness
// counterpart on a dynamic module: a receive value absent from the send
// multiset must be rejected regardless of the runtime size.
func TestCompile_Permutation_DynamicModule_TamperedValueFails(t *testing.T) {
	sys := wiop.NewSystemf("mb-perm-dyn-tampered")
	r0 := sys.NewRound()
	setupMessageBusHook(sys)

	modA := sys.NewDynamicModule(sys.Context.Childf("modA"), wiop.PaddingDirectionRight)
	modB := sys.NewDynamicModule(sys.Context.Childf("modB"), wiop.PaddingDirectionRight)
	colA := modA.NewColumn(sys.Context.Childf("A"), wiop.VisibilityOracle, r0)
	colB := modB.NewColumn(sys.Context.Childf("B"), wiop.VisibilityOracle, r0)

	sys.NewMessageBusSend(
		sys.Context.Childf("send-A"), "shard", "ping", wiop.NewTable(colA.View()))
	sys.NewMessageBusReceive(
		sys.Context.Childf("recv-B"), "shard", "ping", wiop.NewTable(colB.View()))

	compilePermutationBus(sys)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(colA, makeVec(10, 20, 30, 40))
	rt.AssignColumn(colB, makeVec(10, 21, 30, 40)) // 21 is not a sent value

	drive(&rt)
	assert.Error(t, checkAllVerifierActions(&rt),
		"verifier must reject a tampered receive value on a dynamic-module permutation bus")
}

// TestCompile_Permutation_DynamicModule_TamperedFilterFails is the dynamic
// counterpart of the asymmetric-selector soundness case: the send side filters
// out a row the receive side still claims, so the multisets differ and the
// in-shard product check must reject.
func TestCompile_Permutation_DynamicModule_TamperedFilterFails(t *testing.T) {
	sys := wiop.NewSystemf("mb-perm-dyn-tampered-filter")
	r0 := sys.NewRound()
	setupMessageBusHook(sys)

	modA := sys.NewDynamicModule(sys.Context.Childf("modA"), wiop.PaddingDirectionRight)
	modB := sys.NewDynamicModule(sys.Context.Childf("modB"), wiop.PaddingDirectionRight)
	colA := modA.NewColumn(sys.Context.Childf("A"), wiop.VisibilityOracle, r0)
	selA := modA.NewColumn(sys.Context.Childf("selA"), wiop.VisibilityOracle, r0)
	colB := modB.NewColumn(sys.Context.Childf("B"), wiop.VisibilityOracle, r0)

	sys.NewMessageBusSend(
		sys.Context.Childf("send-A"), "shard", "ping",
		wiop.NewFilteredTable(selA.View(), colA.View()))
	sys.NewMessageBusReceive(
		sys.Context.Childf("recv-B"), "shard", "ping", wiop.NewTable(colB.View()))

	compilePermutationBus(sys)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(colA, makeVec(10, 20, 30, 40))
	rt.AssignColumn(selA, makeVec(1, 0, 1, 1)) // sender filters out row 1 (value 20)
	rt.AssignColumn(colB, makeVec(10, 20, 30, 40))

	drive(&rt)
	assert.Error(t, checkAllVerifierActions(&rt),
		"verifier must reject an asymmetric send-side selector on a dynamic-module permutation bus")
}

// TestCheckHandleSumInShard_Permutation_Expected exercises the Expected field
// on [messagebus.CheckHandleSumInShard] for the grand-product path. The
// compile-time path always sets Expected to one (single-shard semantics); the
// other-value branch is intended for the cross-shard layer that constructs this
// action itself. This test:
//
//  1. Builds a balanced single-handle pipeline so the GrandProduct Result cell
//     — this shard's product on the handle — is guaranteed to be one.
//  2. Suppresses Compile's auto-registered action via
//     [wiop.System.MessageBusSkipInShardCheck].
//  3. Constructs CheckHandleSumInShard directly with two Expected values:
//     one (matches the actual product) and zero (does not), and asserts Check
//     accepts/rejects accordingly.
func TestCheckHandleSumInShard_Permutation_Expected(t *testing.T) {
	runWithAndWithoutHook(t, func(t *testing.T, sys *wiop.System, r0 *wiop.Round) {
		t.Helper()
		// Own the in-shard check ourselves so Compile doesn't pre-register one.
		sys.MessageBusSkipInShardCheck = true

		modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionNone)
		modB := sys.NewSizedModule(sys.Context.Childf("modB"), 4, wiop.PaddingDirectionNone)
		colA := modA.NewColumn(sys.Context.Childf("A"), wiop.VisibilityOracle, r0)
		colB := modB.NewColumn(sys.Context.Childf("B"), wiop.VisibilityOracle, r0)

		sys.NewMessageBusSend(
			sys.Context.Childf("send-A"), "shard", "ping", wiop.NewTable(colA.View()))
		sys.NewMessageBusReceive(
			sys.Context.Childf("recv-B"), "shard", "ping", wiop.NewTable(colB.View()))

		compilePermutationBus(sys)

		rt := wiop.NewRuntime(sys)
		rt.AssignColumn(colA, makeVec(10, 20, 30, 40))
		rt.AssignColumn(colB, makeVec(40, 10, 30, 20)) // reordering → product one

		drive(&rt)

		// The single GrandProduct query is this shard's product on the handle.
		require.Len(t, sys.GrandProducts, 1, "single-shard Compile must emit exactly one GrandProduct per handle")
		product := sys.GrandProducts[0].Result

		// Happy path: Expected matches the actual product (one for a balanced bus).
		happy := &messagebus.CheckHandleSumInShard{
			Handle:   "ping",
			Cell:     product,
			Path:     "test/ping/expected-one",
			Expected: field.ElemOne(),
		}
		require.NoError(t, happy.Check(rt),
			"Check must accept when Expected matches the actual product")

		// Sad path: Expected differs from the actual product, so Check must reject.
		sad := &messagebus.CheckHandleSumInShard{
			Handle:   "ping",
			Cell:     product,
			Path:     "test/ping/expected-zero",
			Expected: field.ElemZero(),
		}
		err := sad.Check(rt)
		require.Error(t, err,
			"Check must reject when Expected differs from the actual product")
		assert.Contains(t, err.Error(), `handle "ping"`,
			"error must name the handle for diagnostics")
		assert.Contains(t, err.Error(), "expected",
			"error must include the expected-value context for diagnostics")
	})
}
