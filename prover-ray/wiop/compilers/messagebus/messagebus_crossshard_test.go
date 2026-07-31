package messagebus_test

import (
	"sort"
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
) (*wiop.Runtime, *wiop.Cell, *wiop.Cell) {
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
	return rt, sys.GrandProducts[0].Result, sys.PublicInputs.MessageBus[0]
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
	rtSend, sendRes, mbPISend := buildSingleDirectionShard(
		t, "shard-1-send-only", "shard-1", wiop.BusSend, []uint64{10, 20, 30, 40})
	rtRecv, recvRes, mbPIRecv := buildSingleDirectionShard(
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

	// The accumulator each shard exposes as a public input is the same one the
	// cross-shard join above consumed.
	require.Equal(t, pSend, rtSend.GetCellValue(mbPISend),
		"MessageBus PublicInput has an unexpected value")
	require.Equal(t, pRecv, rtRecv.GetCellValue(mbPIRecv),
		"MessageBus PublicInput has an unexpected value")
}

// TestCrossShard_SendOnlyAndReceiveOnly_Unbalanced is the soundness counterpart:
// shard 2 receives a multiset that is NOT what shard 1 sent (row 20 replaced by
// 21). With the in-shard checks suppressed on both shards, neither shard reports
// an error locally — the failure surfaces only at the cross-shard join, where
// the product is no longer one.
func TestCrossShard_SendOnlyAndReceiveOnly_Unbalanced(t *testing.T) {
	rtSend, sendRes, mbPISend := buildSingleDirectionShard(
		t, "shard-1-send-only", "shard-1", wiop.BusSend, []uint64{10, 20, 30, 40})
	rtRecv, recvRes, mbPIRecv := buildSingleDirectionShard(
		t, "shard-2-recv-only", "shard-2", wiop.BusReceive, []uint64{40, 10, 30, 21})

	pSend := rtSend.GetCellValue(sendRes)
	pRecv := rtRecv.GetCellValue(recvRes)

	crossShard := pSend.Mul(pRecv)
	crossResidual := crossShard.Sub(field.ElemOne())
	require.False(t, crossResidual.IsZero(),
		"cross-shard product must differ from one when the sent and received multisets disagree")

	require.Equal(t, pSend, rtSend.GetCellValue(mbPISend),
		"MessageBus PublicInput has an unexpected value")
	require.Equal(t, pRecv, rtRecv.GetCellValue(mbPIRecv),
		"MessageBus PublicInput has an unexpected value")
}

// busTraffic is one handle's traffic within a single shard: the rows the shard
// sends into the handle and the rows it receives from it. Rows appearing in both
// are settled locally; the rest cross the shard boundary.
type busTraffic struct {
	handle   string
	sent     []uint64
	received []uint64
}

// buildBidirectionalShard builds a shard that BOTH sends and receives on every
// handle in traffic. Per handle, column a is sent and column b is received, and
// the two collapse into a single GrandProduct whose Result is
//
//	∏_a fold(row) / ∏_b fold(row)
//
// Rows present in both a and b cancel inside the shard; whatever is left over is
// the shard's net debt or credit on that handle, to be settled against its
// siblings. That is the difference from [buildSingleDirectionShard], where
// nothing can cancel locally because only one side is present.
//
// Every entry carries [wiop.MessageBus.SkipInShardCheck] — a handle must agree
// on it across its entries, and a net-nonzero shard would otherwise fail the
// in-shard "product == 1" assertion the cross-shard layer is meant to own.
func buildBidirectionalShard(
	t *testing.T,
	name, originShard string,
	traffic []busTraffic,
) (*wiop.Runtime, map[string]*wiop.Cell, []*wiop.Cell) {
	t.Helper()
	sys := wiop.NewSystemf("%s", name)
	r0 := sys.NewRound()
	setupMessageBusHook(sys) // shared seed → same α, β as every sibling shard

	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)

	type assignment struct {
		col  *wiop.Column
		vals []uint64
	}
	toAssign := make([]assignment, 0, 2*len(traffic)) // one send + one receive column per handle
	handles := make([]string, 0, len(traffic))

	for _, tr := range traffic {
		colA := mod.NewColumn(sys.Context.Childf("a-%s", tr.handle), wiop.VisibilityOracle, r0)
		colB := mod.NewColumn(sys.Context.Childf("b-%s", tr.handle), wiop.VisibilityOracle, r0)

		send := sys.NewMessageBusSend(
			sys.Context.Childf("send-%s", tr.handle), originShard, tr.handle,
			wiop.NewTable(colA.View()))
		recv := sys.NewMessageBusReceive(
			sys.Context.Childf("recv-%s", tr.handle), originShard, tr.handle,
			wiop.NewTable(colB.View()))
		send.SkipInShardCheck = true
		recv.SkipInShardCheck = true

		toAssign = append(toAssign, assignment{colA, tr.sent}, assignment{colB, tr.received})
		handles = append(handles, tr.handle)
	}

	compilePermutationBus(sys)

	rt := wiop.NewRuntime(sys)
	for _, a := range toAssign {
		rt.AssignColumn(a.col, makeVec(a.vals...))
	}
	drive(rt)

	require.Len(t, sys.GrandProducts, len(handles),
		"each handle's send and receive must collapse into one GrandProduct")
	require.Len(t, sys.PublicInputs.MessageBus, len(handles),
		"every handle's accumulator must be registered as a MessageBus public input")

	// Compile processes handles in alphabetical order, so GrandProducts[i] is
	// handles[i] once sorted. Assert it via the query path rather than trusting
	// the ordering silently — a change there would otherwise mis-key this map.
	sort.Strings(handles)
	resultByHandle := make(map[string]*wiop.Cell, len(handles))
	for i, h := range handles {
		gp := sys.GrandProducts[i]
		require.Contains(t, gp.Context().Path(), "handle-"+h,
			"GrandProduct %d must belong to handle %q (alphabetical order)", i, h)
		resultByHandle[h] = gp.Result
	}
	return rt, resultByHandle, sys.PublicInputs.MessageBus
}

// crossShardHandles is the traffic both bidirectional tests below share. Two
// independent handles, each with the same shape: every shard settles half its
// rows locally and carries the other half across the boundary.
//
//	route:  shard 1  sends {10, 20, 30, 40}      receives {10, 20, 50, 60}
//	        shard 2  sends {50, 60, 70, 80}      receives {30, 40, 70, 80}
//	wire:   shard 1  sends {110,120,130,140}     receives {110,120,150,160}
//	        shard 2  sends {150,160,170,180}     receives {130,140,170,180}
//
// On route, shard 1 cancels 10 and 20 locally, owes 30 and 40 to shard 2, and is
// owed 50 and 60 by it; shard 2 mirrors that, cancelling 70 and 80 locally. wire
// repeats the pattern on a disjoint value range. Each shard's per-handle product
// is therefore ≠ 1, and the two shards' products are exact inverses per handle.
//
// Declared route-then-wire, which is already alphabetical — the same order
// Compile registers the public inputs in, so index i lines up with handles[i].
var (
	crossShardHandles = []string{"route", "wire"}

	crossShardTrafficShard1 = []busTraffic{
		{handle: "route", sent: []uint64{10, 20, 30, 40}, received: []uint64{10, 20, 50, 60}},
		{handle: "wire", sent: []uint64{110, 120, 130, 140}, received: []uint64{110, 120, 150, 160}},
	}
	crossShardTrafficShard2 = []busTraffic{
		{handle: "route", sent: []uint64{50, 60, 70, 80}, received: []uint64{30, 40, 70, 80}},
		{handle: "wire", sent: []uint64{150, 160, 170, 180}, received: []uint64{130, 140, 170, 180}},
	}
)

// TestCrossShard_Bidirectional_Balanced is the realistic sharding shape: each
// shard both sends and receives on every handle, settles part of its traffic
// locally, and carries the remainder across the shard boundary. See
// [crossShardTrafficShard1] for the multisets.
//
// This is strictly stronger than [TestCrossShard_SendOnlyAndReceiveOnly_Balanced]:
// there, imbalance was unavoidable because each shard had a single direction.
// Here both directions are present, local cancellation genuinely happens, and
// the residual imbalance is what the cross-shard join has to absorb. Running two
// handles at once also pins down that they settle independently despite sharing
// α and β, and that each occupies its own public-input position.
func TestCrossShard_Bidirectional_Balanced(t *testing.T) {
	rt1, res1, mbPI1 := buildBidirectionalShard(
		t, "shard-1-bidir", "shard-1", crossShardTrafficShard1)
	rt2, res2, mbPI2 := buildBidirectionalShard(
		t, "shard-2-bidir", "shard-2", crossShardTrafficShard2)

	require.Len(t, mbPI1, len(crossShardHandles))
	require.Len(t, mbPI2, len(crossShardHandles))

	for i, h := range crossShardHandles {
		t.Run(h, func(t *testing.T) {
			p1 := rt1.GetCellValue(res1[h])
			p2 := rt2.GetCellValue(res2[h])

			// Local cancellation is not enough: each shard still owes the other.
			residual1 := p1.Sub(field.ElemOne())
			residual2 := p2.Sub(field.ElemOne())
			require.False(t, residual1.IsZero(),
				"shard 1 sends rows it does not receive and receives rows it does not send "+
					"on %q, so it must not balance alone", h)
			require.False(t, residual2.IsZero(),
				"shard 2 holds the mirror-image imbalance on %q and must not balance alone", h)

			// Union of sends == union of receives, so the shards settle exactly.
			crossResidual := p1.Mul(p2).Sub(field.ElemOne())
			require.True(t, crossResidual.IsZero(),
				"the two shards' net positions on %q are inverses, so the joint product must be one", h)

			// Position i refers to this same handle on BOTH shards — the invariant
			// that lets the cross-shard layer join by position alone.
			require.Same(t, res1[h], mbPI1[i],
				"shard 1 MessageBus[%d] must be handle %q's accumulator", i, h)
			require.Same(t, res2[h], mbPI2[i],
				"shard 2 MessageBus[%d] must be handle %q's accumulator", i, h)

			require.Equal(t, p1, rt1.GetCellValue(mbPI1[i]),
				"MessageBus PublicInput has an unexpected value")
			require.Equal(t, p2, rt2.GetCellValue(mbPI2[i]),
				"MessageBus PublicInput has an unexpected value")
		})
	}
}

// TestCrossShard_Bidirectional_Unbalanced is the soundness counterpart: on
// handle "route" shard 2 receives 41 where shard 1 sent 40, so that handle's
// union of receives no longer matches its union of sends. "wire" is left intact.
//
// Both shards suppress their in-shard checks, so each looks locally
// indistinguishable from the balanced case — the discrepancy surfaces only at
// the join, and only on the handle that was tampered with. That containment is
// the point: handles share α and β, so a break in one must not smear into the
// other.
func TestCrossShard_Bidirectional_Unbalanced(t *testing.T) {
	tampered := []busTraffic{
		{handle: "route", sent: []uint64{50, 60, 70, 80}, received: []uint64{30, 41, 70, 80}}, // 40 → 41
		{handle: "wire", sent: []uint64{150, 160, 170, 180}, received: []uint64{130, 140, 170, 180}},
	}

	rt1, res1, _ := buildBidirectionalShard(
		t, "shard-1-bidir", "shard-1", crossShardTrafficShard1)
	rt2, res2, _ := buildBidirectionalShard(
		t, "shard-2-bidir", "shard-2", tampered)

	routeResidual := rt1.GetCellValue(res1["route"]).
		Mul(rt2.GetCellValue(res2["route"])).Sub(field.ElemOne())
	require.False(t, routeResidual.IsZero(),
		"a row received on no shard leaves route's joint product different from one")

	wireResidual := rt1.GetCellValue(res1["wire"]).
		Mul(rt2.GetCellValue(res2["wire"])).Sub(field.ElemOne())
	require.True(t, wireResidual.IsZero(),
		"wire is untouched and must still settle, despite sharing α and β with route")
}
