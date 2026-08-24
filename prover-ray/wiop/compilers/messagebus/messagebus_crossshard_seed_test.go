package messagebus_test

import (
	"sort"
	"testing"

	multisethashing "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/multiset_hashing"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/grandproduct"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/messagebus"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
	"github.com/stretchr/testify/require"
)

// This file covers shared randomness reconstruction from contributions: γ is
// not handed to the shards as a literal, it is *derived from what the shards
// publish*. Each shard commits its bus columns through the real PCS commit
// action, publishes the multiset hash of that commitment as its contribution,
// and γ is the group sum of those contributions compressed by
// [multisethashing.ToSeed] — the same construction
// [github.com/LFDT-Lineth/lineth-monorepo/prover-ray/preflight.Run] performs in
// the orchestrator.
//
// seededShardAssignment pairs a bus column with the rows to write into it.
type seededShardAssignment struct {
	col  *wiop.Column
	vals []uint64
}

// seededShard is a bidirectional shard compiled with real shared randomness and
// real commitments, re-runnable under different γ.
type seededShard struct {
	sys     *wiop.System
	assign  []seededShardAssignment
	handles []string // alphabetical, matching Compile's public-input order
}

// buildSeededBidirectionalShard mirrors [buildBidirectionalShard] — same traffic
// shape, one sent and one received column per handle on round 0, every entry
// carrying SkipInShardCheck so the cross-shard layer owns the balance check —
// with two deliberate differences.
//
// First, it compiles with [messagebus.CompileOptions.SharedRandomness], which is
// what declares the γ cells and the contribution cells and registers the
// assigner and checker.
//
// Second, it runs [pcs.Compile]. It registers the per-round commit action that populates
// [wiop.Runtime.Commitments], and the contribution is a hash of exactly those
// values.
func buildSeededBidirectionalShard(
	t *testing.T,
	name, originShard string,
	traffic []busTraffic,
) *seededShard {
	t.Helper()

	sys := wiop.NewSystemf("%s", name)
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)

	toAssign := make([]seededShardAssignment, 0, 2*len(traffic))
	handles := make([]string, 0, len(traffic))

	for _, tr := range traffic {
		colA := mod.NewColumn(sys.Context.Childf("a-%s", tr.handle), r0)
		colB := mod.NewColumn(sys.Context.Childf("b-%s", tr.handle), r0)

		send := sys.NewMessageBusSend(
			sys.Context.Childf("send-%s", tr.handle), originShard, tr.handle,
			wiop.NewTable(colA.View()))
		recv := sys.NewMessageBusReceive(
			sys.Context.Childf("recv-%s", tr.handle), originShard, tr.handle,
			wiop.NewTable(colB.View()))
		send.SkipInShardCheck = true
		recv.SkipInShardCheck = true

		toAssign = append(toAssign,
			seededShardAssignment{colA, tr.sent},
			seededShardAssignment{colB, tr.received})
		handles = append(handles, tr.handle)
	}

	messagebus.Compile(sys, messagebus.CompileOptions{SharedRandomness: true})
	grandproduct.Compile(sys)
	pcs.Compile(sys)

	sort.Strings(handles)
	return &seededShard{sys: sys, assign: toAssign, handles: handles}
}

// seededCoinRoundID is where messagebus.Compile puts α and β for this layout:
// every bus column sits on round 0, so the coin round is the one after it.
const seededCoinRoundID = 1

// run drives the prover under seed g and returns the runtime together with the
// contribution the shard published and the Merkle root it committed on round 0.
//
// It stops after the result round's actions: that is far enough for the
// contribution cells (assigned on the coin round) and the per-handle
// accumulators (assigned on the result round) to hold their values, and short of
// the opening round, which this pipeline cannot discharge and does not need to.
func (s *seededShard) run(
	t *testing.T,
	g field.Octuplet,
) (rt *wiop.Runtime, contribution multisethashing.MSetHash, busRoot field.Octuplet) {
	t.Helper()

	require.Len(t, s.sys.Rounds[seededCoinRoundID].Coins, 2,
		"α and β must be declared on round %d", seededCoinRoundID)

	rt = wiop.NewRuntime(s.sys)
	for _, a := range s.assign {
		rt.AssignColumn(a.col, makeVec(a.vals...))
	}
	messagebus.AssignSharedRandomnessSeed(rt, g)

	// Round 0: the PCS commit action fills rt.Commitments[0]. Advancing then
	// absorbs it and fires the shared-randomness hook, so α and β derive from γ.
	for rt.CurrentRound().ID < seededCoinRoundID {
		runRound(rt)
		rt.AdvanceRound()
	}

	// Coin round: SharedRandomnessContributionAssigner writes the contribution.
	runRound(rt)

	contribution = s.contribution(t, rt)
	busRoot = rt.Commitments[0]

	// Run the verifier action Verify would normally run, while the runtime still
	// sits on the coin round so that it recomputes over exactly the rounds the
	// assigner covered. This is what ties the published contribution to the
	// shard's own commitments rather than to an arbitrary value.
	require.NoError(t, (&messagebus.SharedRandomnessContributionChecker{}).Check(rt),
		"the published contribution must match the shard's own commitments")

	// Result round: the grand-product actions assign the per-handle accumulators.
	rt.AdvanceRound()
	runRound(rt)

	return rt, contribution, busRoot
}

// contribution reads the shard's contribution out of its public-input cells.
func (s *seededShard) contribution(t *testing.T, rt *wiop.Runtime) multisethashing.MSetHash {
	t.Helper()
	var c multisethashing.MSetHash
	for i := range messagebus.NumSharedRandomnessContribution {
		cell, pos := s.sys.LookupPublicInputByTag(
			messagebus.SharedRandomnessSeedContributionPI, i)
		require.GreaterOrEqual(t, pos, 0, "contribution limb %d must be a public input", i)
		c[i] = rt.GetCellValue(cell).AsBase()
	}
	return c
}

// product returns handle i's accumulator, in the alphabetical order Compile
// numbers the MessageBus public inputs by.
func (s *seededShard) product(rt *wiop.Runtime, i int) field.Gen {
	return rt.GetCellValue(s.sys.GrandProducts[i].Result)
}

// TestCrossShard_SeedDerivedFromContributions closes the shared-randomness loop
// in the direction no other test covers: γ is computed *from* the shards rather
// than given *to* them.
//
// Two bidirectional shards commit their bus columns, publish their contributions,
// and γ is taken as ToSeed(Combine(c1, c2)). Feeding that γ back must leave the
// contributions untouched and reproduce the same γ — the fixed point that lets an
// orchestrator hand out a seed the shards can be held to.
//
// The reconstruction is well defined only because a contribution cannot depend on
// γ: γ lives in cells, while a contribution hashes round commitments and
// commitments cover columns only (see commitToRound, which walks round.Columns).
// That independence is asserted rather than assumed, since a contribution that
// moved with γ would make the fixed point circular and the test meaningless.
func TestCrossShard_SeedDerivedFromContributions(t *testing.T) {
	shard1 := buildSeededBidirectionalShard(
		t, "shard-1-bidir-seeded", "shard-1", crossShardTrafficShard1)
	shard2 := buildSeededBidirectionalShard(
		t, "shard-2-bidir-seeded", "shard-2", crossShardTrafficShard2)

	// Pass one: harvest the contributions under a throwaway seed.
	throwaway := gamma(1)
	_, c1, root1 := shard1.run(t, throwaway)
	_, c2, root2 := shard2.run(t, throwaway)

	require.NotEqual(t, c1, c2,
		"the two shards hold different bus traffic, so their contributions must differ; "+
			"equal contributions mean the PCS pass dropped out and both degenerated to Hash(0)")

	// A contribution is the multiset hash of the shard's own bus-column
	// commitment. Pinning this is what keeps the construction anchored to the
	// orchestrator's: preflight.Run accumulates exactly Hash(root) per shard.
	require.Equal(t, multisethashing.Hash(root1), c1,
		"shard 1's contribution must be the multiset hash of its bus-column Merkle root")
	require.Equal(t, multisethashing.Hash(root2), c2,
		"shard 2's contribution must be the multiset hash of its bus-column Merkle root")

	g := multisethashing.ToSeed(multisethashing.Combine(c1, c2))
	require.Equal(t, g, multisethashing.ToSeed(multisethashing.Combine(c2, c1)),
		"the group operation is commutative, so no shard ordering may be imposed")

	// Pass two: bind the shards to the γ their own contributions produced.
	rt1, c1Bound, _ := shard1.run(t, g)
	rt2, c2Bound, _ := shard2.run(t, g)

	require.Equal(t, c1, c1Bound,
		"a contribution must not move with γ: it hashes commitments, and γ is a cell")
	require.Equal(t, c2, c2Bound,
		"a contribution must not move with γ: it hashes commitments, and γ is a cell")

	require.Equal(t, g, multisethashing.ToSeed(multisethashing.Combine(c1Bound, c2Bound)),
		"γ must be recoverable from the contributions of the shards it was handed to")

	// Each shard is unbalanced alone, and the pair settles exactly. This is also
	// the check that α and β really did agree: had the seeding failed, the two
	// shards would fold their rows under different challenges and the products
	// would not be inverses.
	require.Len(t, shard1.handles, len(crossShardHandles))
	for i, h := range shard1.handles {
		t.Run(h, func(t *testing.T) {
			p1 := shard1.product(rt1, i)
			p2 := shard2.product(rt2, i)

			require.False(t, equal(p1, field.ElemOne()),
				"shard 1 carries a net position on %q and must not balance alone", h)
			require.False(t, equal(p2, field.ElemOne()),
				"shard 2 carries the mirror position on %q and must not balance alone", h)
			require.True(t, equal(p1.Mul(p2), field.ElemOne()),
				"the shards' net positions on %q must be inverses under the shared α and β", h)
		})
	}
}

// TestCrossShard_SeedFromWrongContributions_Rejected is the soundness
// counterpart, following the balanced/unbalanced pattern the rest of this file
// uses. A shard handed a γ that its own contribution did not help produce is
// still internally consistent — the contribution checker recomputes from
// commitments and passes — but the reconstruction no longer closes, which is the
// discrepancy an aggregator is there to catch.
func TestCrossShard_SeedFromWrongContributions_Rejected(t *testing.T) {
	shard1 := buildSeededBidirectionalShard(
		t, "shard-1-bidir-seeded", "shard-1", crossShardTrafficShard1)
	shard2 := buildSeededBidirectionalShard(
		t, "shard-2-bidir-seeded", "shard-2", crossShardTrafficShard2)

	_, c1, _ := shard1.run(t, gamma(1))
	_, c2, _ := shard2.run(t, gamma(1))

	// γ built from only one shard's contribution: the sibling's traffic is not
	// bound into the seed at all.
	partial := multisethashing.ToSeed(c1)
	full := multisethashing.ToSeed(multisethashing.Combine(c1, c2))
	require.NotEqual(t, full, partial,
		"a γ omitting a shard's contribution must differ from the complete one")

	// The shards still prove and still pass their own contribution checks under
	// the wrong γ — the local checks say nothing about which γ was used.
	_, c1Wrong, _ := shard1.run(t, partial)
	_, c2Wrong, _ := shard2.run(t, partial)

	require.NotEqual(t, partial,
		multisethashing.ToSeed(multisethashing.Combine(c1Wrong, c2Wrong)),
		"reconstruction must not reproduce a γ that was not built from every shard")
}
