package messagebus_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/stretchr/testify/require"
)

// buildSingleDirectionShard builds a self-contained shard whose single handle
// carries exactly one entry — either a Send or a Receive — and drives it to a
// product value.
//
// Because a shard that only sends (resp. only receives) can never balance on
// its own, the entry is marked [wiop.MessageBus.SkipInShardCheck] so the
// compiler does NOT register the in-shard "product == 1" assertion. The shard's
// GrandProduct Result is therefore left live and unasserted: it holds this
// shard's partial product on the handle, to be joined with sibling shards.
//
//   - A Send-only shard produces Result = ∏send folds (numerator only).
//   - A Receive-only shard produces Result = 1 / ∏recv folds (denominator only).
//
// Every shard registers [setupMessageBusHook], seeding Fiat–Shamir with the
// shared testMessageBusSeed so α and β are IDENTICAL across shards — the
// precondition that makes the per-shard folds comparable and their products
// joinable. This mirrors the production cross-shard layer, which feeds every
// shard the same shared randomness.
func buildSingleDirectionShard(
	t *testing.T,
	name, originShard string,
	dir wiop.BusDirection,
	vals []uint64,
) (*wiop.Runtime, *wiop.Cell) {
	t.Helper()
	sys := wiop.NewSystemf("%s", name)
	r0 := sys.NewRound()
	setupMessageBusHook(sys) // shared seed → same α, β as every sibling shard

	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("c"), wiop.VisibilityOracle, r0)

	var mb *wiop.MessageBus
	switch dir {
	case wiop.BusSend:
		mb = sys.NewMessageBusSend(
			sys.Context.Childf("send"), originShard, "route", wiop.NewTable(col.View()))
	case wiop.BusReceive:
		mb = sys.NewMessageBusReceive(
			sys.Context.Childf("recv"), originShard, "route", wiop.NewTable(col.View()))
	default:
		t.Fatalf("unexpected direction %v", dir)
	}
	// Defer the consistency check to the cross-shard layer: an isolated
	// send-only / receive-only shard is intentionally unbalanced.
	mb.SkipInShardCheck = true

	compilePermutationBus(sys)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(col, makeVec(vals...))
	drive(rt)

	require.Len(t, sys.GrandProducts, 1,
		"single-handle shard must emit exactly one GrandProduct")
	return rt, sys.GrandProducts[0].Result
}

// TestCrossShard_SendOnlyAndReceiveOnly_Balanced models the canonical
// cross-shard split: shard 1 ONLY sends a multiset of rows and shard 2 ONLY
// receives them (in a different order). Neither shard balances on its own — the
// send-only shard's product is ∏send and the receive-only shard's is 1/∏recv,
// each ≠ 1 — but because the received rows are a reordering of the sent rows,
// the product across shards is one. That cross-shard product is exactly the
// identity a downstream layer would assert in place of the suppressed in-shard
// checks.
func TestCrossShard_SendOnlyAndReceiveOnly_Balanced(t *testing.T) {
	rtSend, sendRes := buildSingleDirectionShard(
		t, "shard-1-send-only", "shard-1", wiop.BusSend, []uint64{10, 20, 30, 40})
	rtRecv, recvRes := buildSingleDirectionShard(
		t, "shard-2-recv-only", "shard-2", wiop.BusReceive, []uint64{40, 10, 30, 20})

	pSend := rtSend.GetCellValue(sendRes)
	pRecv := rtRecv.GetCellValue(recvRes)

	// Each shard alone is unbalanced — the whole point of deferring the check.
	sendResidual := pSend.Sub(field.ElemOne())
	recvResidual := pRecv.Sub(field.ElemOne())
	require.False(t, sendResidual.IsZero(),
		"send-only shard product must not be one in isolation")
	require.False(t, recvResidual.IsZero(),
		"receive-only shard product must not be one in isolation")

	// Cross-shard identity: ∏send · (1/∏recv) == 1 when the multisets coincide.
	crossShard := pSend.Mul(pRecv)
	crossResidual := crossShard.Sub(field.ElemOne())
	require.True(t, crossResidual.IsZero(),
		"cross-shard product must be one when the received rows are a permutation of the sent rows")
}

// TestCrossShard_SendOnlyAndReceiveOnly_Unbalanced is the soundness counterpart:
// shard 2 receives a multiset that is NOT what shard 1 sent (row 20 replaced by
// 21). With the in-shard checks suppressed on both shards, neither shard reports
// an error locally — the failure surfaces only at the cross-shard join, where
// the product is no longer one.
func TestCrossShard_SendOnlyAndReceiveOnly_Unbalanced(t *testing.T) {
	rtSend, sendRes := buildSingleDirectionShard(
		t, "shard-1-send-only", "shard-1", wiop.BusSend, []uint64{10, 20, 30, 40})
	rtRecv, recvRes := buildSingleDirectionShard(
		t, "shard-2-recv-only", "shard-2", wiop.BusReceive, []uint64{40, 10, 30, 21})

	pSend := rtSend.GetCellValue(sendRes)
	pRecv := rtRecv.GetCellValue(recvRes)

	crossShard := pSend.Mul(pRecv)
	crossResidual := crossShard.Sub(field.ElemOne())
	require.False(t, crossResidual.IsZero(),
		"cross-shard product must differ from one when the sent and received multisets disagree")
}
