// This file covers shared randomness in both directions.
//
// γ handed *to* the shards: the light pipeline below (no PCS) checks that a γ
// supplied from outside reaches α and β, that it reaches nothing else, and that
// it leaves the prover as a public input.
//
// γ derived *from* the shards: the committed pipeline further down runs the real
// PCS commit action, has each shard publish the multiset hash of its bus round's
// commitment as its contribution, and takes γ to be the group sum of those
// contributions compressed by [multisethashing.ToSeed] — the same construction
// [github.com/LFDT-Lineth/lineth-monorepo/prover-ray/preflight.Run] performs in
// the orchestrator.
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

// gamma builds a γ octuplet whose limbs are seed, seed+1, … so that two
// different seeds give two clearly different octuplets.
func gamma(seed uint64) field.Octuplet {
	var g field.Octuplet
	for i := range g {
		g[i].SetUint64(seed + uint64(i))
	}
	return g
}

// equal reports whether two field values coincide. It exists because IsZero is
// a pointer method, so the difference has to be named before it can be tested.
func equal(a, b field.Gen) bool {
	diff := a.Sub(b)
	return diff.IsZero()
}

// =============================================================================
// γ handed to the shards
// =============================================================================

// shard is one compiled single-direction shard together with the handles a test
// needs to drive and inspect it.
type shard struct {
	sys      *wiop.System
	col      *wiop.Column
	local    *wiop.Cell
	vals     []uint64
	localV   uint64
	withSeed bool
}

// buildShard declares a one-column message-bus shard in the given direction and
// compiles the permutation pipeline, seeding it with γ only if withSeed.
//
// localV is written into a plain round-0 cell. That cell exists to give the
// shard a *local* Fiat-Shamir transcript of its own: cells are absorbed by
// [wiop.Runtime.AdvanceRound], whereas column data is only bound into the
// transcript by the PCS pass, which this pipeline does not run. Two shards with
// different localV therefore reach the coin round in genuinely different
// Fiat-Shamir states — which is the precondition that makes seeding observable
// at all. TestSharedRandomness_UnseededShardsDisagree is the control that keeps
// this honest.
//
// SkipInShardCheck is set because a single-direction shard is unbalanced on its
// own by construction: the balance is the cross-shard product of the two
// shards' accumulators, which the caller checks.
func buildShard(
	t *testing.T,
	name string,
	dir wiop.BusDirection,
	vals []uint64,
	localV uint64,
	withSeed bool,
) *shard {
	t.Helper()

	sys := wiop.NewSystemf("%s", name)
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), len(vals), wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("c"), r0)
	local := r0.NewCell(sys.Context.Childf("local"), false)
	tab := wiop.NewTable(col.View())

	var mb *wiop.MessageBus
	switch dir {
	case wiop.BusSend:
		mb = sys.NewMessageBusSend(sys.Context.Childf("entry"), name, "handle", tab)
	case wiop.BusReceive:
		mb = sys.NewMessageBusReceive(sys.Context.Childf("entry"), name, "handle", tab)
	default:
		t.Fatalf("unexpected direction %v", dir)
	}
	mb.SkipInShardCheck = true

	s := &shard{sys: sys, col: col, local: local, vals: vals, localV: localV, withSeed: withSeed}

	messagebus.Compile(sys, messagebus.CompileOptions{SharedRandomness: withSeed})
	grandproduct.Compile(sys)

	return s
}

// assign writes every round-0 prover input: the local cell, γ when the shard is
// seeded, and the bus column.
func (s *shard) assign(rt *wiop.Runtime, g field.Octuplet) {
	var localVal field.Element
	localVal.SetUint64(s.localV)
	rt.AssignCell(s.local, field.ElemFromBase(localVal))

	if s.withSeed {
		messagebus.AssignSharedRandomnessSeed(rt, g)
	}

	rt.AssignColumn(s.col, makeVec(s.vals...))
}

// run drives the prover to completion against the given γ and returns the
// runtime with every coin sampled and every prover action executed.
func (s *shard) run(g field.Octuplet) *wiop.Runtime {
	rt := wiop.NewRuntime(s.sys)
	s.assign(rt, g)

	for {
		for _, a := range rt.CurrentRound().ProverActions {
			a.Run(rt)
		}
		if rt.CurrentRound().ID == len(s.sys.Rounds)-1 {
			return rt
		}
		rt.AdvanceRound()
	}
}

// coins returns the shard's α and β, which messagebus.Compile declares in that
// order on the coin round (round 1, the round after the participants).
func (s *shard) coins(rt *wiop.Runtime) (alpha, beta field.Gen) {
	coinRound := s.sys.Rounds[1]
	return rt.GetCoinValue(coinRound.Coins[0]), rt.GetCoinValue(coinRound.Coins[1])
}

// TestSharedRandomness_CoinsLandAfterTheLastBusRound pins the round layout the
// sharded RISC-V protocol depends on: round 0 commits the program verification
// data, round 1 commits the columns the message bus reads, and α/β must be
// sampled on round 2 — after every bus-impacting commitment, and before any
// shard-specific data that must not influence the shared challenges.
//
// If the coins ever slid to round 1 they would be drawn before the bus columns
// were committed; if they slid past round 2 they would absorb shard-specific
// data and shards would stop agreeing. Neither shows up as a failure in the
// other tests here, which use a single participant round, so the layout is
// asserted directly.
func TestSharedRandomness_CoinsLandAfterTheLastBusRound(t *testing.T) {
	sys := wiop.NewSystemf("shard")
	r0 := sys.NewRound() // program verification data
	r1 := sys.NewRound() // the columns the bus reads

	progMod := sys.NewSizedModule(sys.Context.Childf("prog"), 4, wiop.PaddingDirectionNone)
	progCol := progMod.NewColumn(sys.Context.Childf("prog-col"), r0)

	busMod := sys.NewSizedModule(sys.Context.Childf("bus"), 4, wiop.PaddingDirectionNone)
	busCol := busMod.NewColumn(sys.Context.Childf("bus-col"), r1)

	mb := sys.NewMessageBusSend(
		sys.Context.Childf("entry"), "shard", "handle", wiop.NewTable(busCol.View()))
	mb.SkipInShardCheck = true

	messagebus.Compile(sys, messagebus.CompileOptions{SharedRandomness: true})
	grandproduct.Compile(sys)

	require.Len(t, sys.Rounds[2].Coins, 2,
		"α and β must be declared on round 2, one past the last bus-impacting round")
	require.Empty(t, sys.Rounds[1].Coins,
		"no coin may be sampled on round 1, before the bus columns are committed")

	// The seed cells stay on round 0 regardless of how far out the coin round
	// sits, so they are always assigned before the hook reads them.
	cell, pos := sys.LookupPublicInputByTag(messagebus.SharedRandomnessSeedPI, 0)
	require.GreaterOrEqual(t, pos, 0, "γ must be registered as a public input")
	require.Equal(t, 0, cell.Round().ID, "γ cells must live on round 0")

	// The contribution cells go the other way: they cannot precede the
	// commitments they hash, so they belong on the coin round, where the prover
	// action that computes them runs.
	contrib, pos := sys.LookupPublicInputByTag(messagebus.SharedRandomnessSeedContributionPI, 0)
	require.GreaterOrEqual(t, pos, 0, "the contribution must be registered as a public input")
	require.Equal(t, 2, contrib.Round().ID, "contribution cells must live on the coin round")

	// Drive the prover to confirm the hook fires on the round that carries the
	// coins rather than panicking or seeding an empty round.
	rt := wiop.NewRuntime(sys)
	messagebus.AssignSharedRandomnessSeed(rt, gamma(7))
	rt.AssignColumn(progCol, makeVec(1, 2, 3, 4))
	for {
		// Each column is assigned while the runtime sits on its own round.
		if rt.CurrentRound().ID == 1 {
			rt.AssignColumn(busCol, makeVec(10, 20, 30, 40))
		}
		for _, a := range rt.CurrentRound().ProverActions {
			a.Run(rt)
		}
		if rt.CurrentRound().ID == len(sys.Rounds)-1 {
			break
		}
		rt.AdvanceRound()
	}

	alpha := rt.GetCoinValue(sys.Rounds[2].Coins[0])
	require.False(t, equal(alpha, field.Gen{}), "α must have been sampled")
}

// TestSharedRandomness_UnseededShardsDisagree is the control for
// TestSharedRandomness_SameGammaGivesSameCoins. Built without γ, the two shards
// derive their coins from their own transcripts, which differ — so they
// disagree.
//
// Without this control the positive test would be vacuous: if the two shards
// happened to reach the coin round in identical Fiat-Shamir states, their coins
// would match whether or not the hook did anything, and the positive test would
// pass against a hook that seeds nothing.
func TestSharedRandomness_UnseededShardsDisagree(t *testing.T) {
	send := buildShard(t, "shard-1", wiop.BusSend, []uint64{10, 20, 30, 40}, 111, false)
	recv := buildShard(t, "shard-2", wiop.BusReceive, []uint64{40, 30, 20, 10}, 222, false)

	alphaSend, betaSend := send.coins(send.run(field.Octuplet{}))
	alphaRecv, betaRecv := recv.coins(recv.run(field.Octuplet{}))

	require.False(t, equal(alphaSend, alphaRecv),
		"unseeded shards with different transcripts must derive different α")
	require.False(t, equal(betaSend, betaRecv),
		"unseeded shards with different transcripts must derive different β")
}

// TestSharedRandomness_SameGammaGivesSameCoins is the positive test. It is the
// same pair of shards as the control above — different column data, different
// local cells, therefore different local transcripts — but seeded with one
// shared γ. The coins must now agree, which they can only do because the hook
// replaced each shard's local state with γ before sampling.
//
// This is the only test that reads α and β out of the two shards and compares
// them directly. TestSharedRandomness_SeedDerivedFromContributions observes the
// same agreement, but only through its effect on the products, and only in the
// committed pipeline.
func TestSharedRandomness_SameGammaGivesSameCoins(t *testing.T) {
	g := gamma(7)

	send := buildShard(t, "shard-1", wiop.BusSend, []uint64{10, 20, 30, 40}, 111, true)
	recv := buildShard(t, "shard-2", wiop.BusReceive, []uint64{40, 30, 20, 10}, 222, true)

	rtSend, rtRecv := send.run(g), recv.run(g)

	alphaSend, betaSend := send.coins(rtSend)
	alphaRecv, betaRecv := recv.coins(rtRecv)

	require.True(t, equal(alphaSend, alphaRecv),
		"α must be identical on both shards when they are given the same γ")
	require.True(t, equal(betaSend, betaRecv),
		"β must be identical on both shards when they are given the same γ")

	prodSend := rtSend.GetCellValue(send.sys.GrandProducts[0].Result)
	prodRecv := rtRecv.GetCellValue(recv.sys.GrandProducts[0].Result)
	require.True(t, equal(prodSend.Mul(prodRecv), field.ElemOne()),
		"the cross-shard product must be one when the sent and received multisets coincide")
}

// TestSharedRandomness_DifferentGammaGivesDifferentCoins runs one shard twice,
// changing nothing but γ. The coins must move with it: a γ that did not reach
// the challenges would leave shards free to disagree on the permutation
// challenge while still appearing to share randomness. It is also what rules out
// a hook that seeds some constant and ignores γ — which every other test here
// would accept.
func TestSharedRandomness_DifferentGammaGivesDifferentCoins(t *testing.T) {
	s := buildShard(t, "shard-1", wiop.BusSend, []uint64{10, 20, 30, 40}, 111, true)

	alphaA, betaA := s.coins(s.run(gamma(7)))
	alphaB, betaB := s.coins(s.run(gamma(999)))

	require.False(t, equal(alphaA, alphaB),
		"α must change when γ changes, since γ is the state the coins are drawn from")
	require.False(t, equal(betaA, betaB),
		"β must change when γ changes, since γ is the state the coins are drawn from")
}

// TestSharedRandomness_SeedingIsScopedToAlphaBeta is the reason α and β are
// marked with [wiop.CoinField.MarkSeeded] instead of the seed covering the whole
// coin round: γ must reach the shared challenges without erasing the shard's own
// transcript.
//
// The coin round is not this pass's private property — the lookup and permutation
// passes anchor their coin rounds the way ensureCoinRound does and regularly land
// on it, declaring their own coins there, which is what the foreign coin below
// stands in for. The two shards are identical bar the round-0 local cell, so only
// that cell can make them diverge. α and β must still agree, being drawn from γ
// alone; the foreign coin and the Fiat-Shamir state must not, since the cell was
// absorbed before the coin round and has to stay bound into every later
// challenge. Under whole-round seeding both would have been drawn from γ too and
// stayed identical, silently losing that binding.
func TestSharedRandomness_SeedingIsScopedToAlphaBeta(t *testing.T) {
	g := gamma(7)

	// build compiles a seeded shard, then plants a foreign unmarked coin on the
	// round messagebus chose for α and β.
	build := func(name string, localV uint64) (*shard, *wiop.CoinField) {
		s := buildShard(t, name, wiop.BusSend, []uint64{10, 20, 30, 40}, localV, true)
		coinRound := s.sys.Rounds[1]
		require.Len(t, coinRound.Coins, 2, "α and β must be the round's only coins so far")
		return s, coinRound.NewCoinField(s.sys.Context.Childf("foreign"))
	}

	a, foreignA := build("shard-a", 111)
	b, foreignB := build("shard-b", 222)

	rtA, rtB := a.run(g), b.run(g)

	alphaA, betaA := a.coins(rtA)
	alphaB, betaB := b.coins(rtB)
	require.True(t, equal(alphaA, alphaB), "α is seeded and must agree across shards")
	require.True(t, equal(betaA, betaB), "β is seeded and must agree across shards")

	require.False(t, equal(rtA.GetCoinValue(foreignA), rtB.GetCoinValue(foreignB)),
		"an unmarked coin sharing the coin round must derive from its own shard's "+
			"transcript, so it must differ when the shards' round-0 data differs")
	require.NotEqual(t, rtA.GetFS().State(), rtB.GetFS().State(),
		"the seeding must not erase the local transcript: the round-0 cell has to stay "+
			"bound into every challenge drawn after the coin round")
}

// TestSharedRandomness_IsAPublicInput checks the property that lets the
// aggregation layer close the binding loop: γ leaves the prover in the
// public-input vector, at the positions carrying the SharedRandomnessSeed_i tags,
// where an aggregator can read it and compare it against a sibling's.
//
// TestSharedRandomness_CoinsLandAfterTheLastBusRound already asserts that the γ
// cells are registered and where they live; what this adds is that their
// *values* reach the vector [wiop.System.Prove] hands out, at those positions.
func TestSharedRandomness_IsAPublicInput(t *testing.T) {
	s := buildShard(t, "shard-1", wiop.BusSend, []uint64{10, 20, 30, 40}, 111, true)
	g := gamma(7)

	_, pub := s.sys.Prove(func(rt *wiop.Runtime) { s.assign(rt, g) })

	for i := range messagebus.NumSharedRandomness {
		cell, pos := s.sys.LookupPublicInputByTag(messagebus.SharedRandomnessSeedPI, i)
		require.NotNil(t, cell, "γ limb %d must be registered as a public input", i)
		require.Less(t, pos, len(pub), "γ limb %d must have a slot in the public-input vector", i)
		require.True(t, equal(pub[pos], field.ElemFromBase(g[i])),
			"public input at the SharedRandomnessSeed_%d position must carry γ limb %d", i, i)
	}
}

// =============================================================================
// γ derived from the shards
// =============================================================================

// seededShardAssignment pairs a column with the rows to write into it.
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
// shape, one sent and one received column per handle, every entry carrying
// SkipInShardCheck so the cross-shard layer owns the balance check — with three
// deliberate differences.
//
// First, it compiles with [messagebus.CompileOptions.SharedRandomness], which is
// what declares the γ cells and the contribution cells and registers the
// assigner and checker.
//
// Second, it runs [pcs.Compile]. It registers the per-round commit action that populates
// [wiop.Runtime.Commitments], and the contribution is a hash of exactly those
// values.
//
// Third, the bus columns sit on round 1, behind a round-0 column no bus entry
// reads — the layout [ensureCoinRound] describes, where round 0 commits the
// program verification data and round 1 commits what the bus reads. γ still
// lives on round 0, where [registerSharedRandomness] puts it. Keeping the bus
// columns off round 0 is what makes the test able to tell "the round before the
// coins" from "the first round": with everything on round 0 the two coincide,
// and a contribution hashing the wrong round would go unnoticed.
func buildSeededBidirectionalShard(
	t *testing.T,
	name, originShard string,
	traffic []busTraffic,
) *seededShard {
	t.Helper()

	sys := wiop.NewSystemf("%s", name)
	r0 := sys.NewRound()
	r1 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)

	toAssign := make([]seededShardAssignment, 0, 2*len(traffic)+1)
	handles := make([]string, 0, len(traffic))

	// The round-0 occupant. Same content on every shard, as a guest program would
	// be: two shards whose contributions came out equal would then be hashing this
	// round instead of their own bus columns, which the c1 != c2 assertion catches.
	progCol := mod.NewColumn(sys.Context.Childf("prog-col"), r0)
	toAssign = append(toAssign, seededShardAssignment{progCol, []uint64{7, 7, 7, 7}})

	for _, tr := range traffic {
		colA := mod.NewColumn(sys.Context.Childf("a-%s", tr.handle), r1)
		colB := mod.NewColumn(sys.Context.Childf("b-%s", tr.handle), r1)

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

const (
	// seededBusRoundID is where the bus columns sit; round 0 carries the
	// program column instead.
	seededBusRoundID = 1
	// seededCoinRoundID is where messagebus.Compile puts α and β: one past the
	// last round any bus entry reads.
	seededCoinRoundID = seededBusRoundID + 1
)

// run drives the prover under seed g and returns the runtime together with the
// contribution the shard published and the Merkle root it committed on the bus
// round.
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
	messagebus.AssignSharedRandomnessSeed(rt, g)

	// Each committed round in turn: assign its columns, let its PCS commit action
	// fill rt.Commitments[ID], then advance — which absorbs that root, and on the
	// step into the coin round fires the hook that seeds α and β from γ. Columns
	// are assigned round by round because AssignColumn rejects a column that does
	// not belong to the round the runtime is on.
	for rt.CurrentRound().ID < seededCoinRoundID {
		for _, a := range s.assign {
			if a.col.Round() == rt.CurrentRound() {
				rt.AssignColumn(a.col, makeVec(a.vals...))
			}
		}
		runRound(rt)
		rt.AdvanceRound()
	}

	// Coin round: SharedRandomnessContributionAssigner writes the contribution.
	runRound(rt)

	contribution = s.contribution(t, rt)
	busRoot = rt.Commitments[seededBusRoundID]

	require.NotEqual(t, rt.Commitments[0], busRoot,
		"round 0 and the bus round must commit different data, or the caller's "+
			"contribution assertions cannot tell which of the two was hashed")

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

// TestSharedRandomness_SeedDerivedFromContributions closes the shared-randomness
// loop in the direction no other test covers: γ is computed *from* the shards
// rather than given *to* them.
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
func TestSharedRandomness_SeedDerivedFromContributions(t *testing.T) {
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
			"equal contributions mean the PCS pass dropped out and both degenerated to "+
			"Hash(0), or that the contribution hashed round 0, whose program column is "+
			"the same on both shards")

	// A contribution is the multiset hash of the shard's own bus-round
	// commitment — the bus round being round 1 here, not round 0, so this also
	// pins that the contribution follows the bus columns rather than the first
	// committed round. Pinning it is what keeps the construction anchored to the
	// orchestrator's: preflight.Run accumulates exactly Hash(root) per shard.
	require.Equal(t, multisethashing.Hash(root1), c1,
		"shard 1's contribution must be the multiset hash of its bus-round Merkle root")
	require.Equal(t, multisethashing.Hash(root2), c2,
		"shard 2's contribution must be the multiset hash of its bus-round Merkle root")

	g := multisethashing.ToSeed(multisethashing.Combine(c1, c2))
	require.Equal(t, g, multisethashing.ToSeed(multisethashing.Combine(c2, c1)),
		"the group operation is commutative, so no shard ordering may be imposed")

	// A γ built from a subset of the contributions binds only that subset. It must
	// come out different, because that discrepancy is the only thing an aggregator
	// has to go on: a shard handed such a γ stays internally consistent — its own
	// contribution checker recomputes from its own commitments and passes — so the
	// local checks say nothing about which γ was used.
	require.NotEqual(t, g, multisethashing.ToSeed(c1),
		"a γ omitting a shard's contribution must differ from the complete one")

	// Pass two: bind the shards to the γ their own contributions produced.
	rt1, c1Bound, _ := shard1.run(t, g)
	rt2, c2Bound, _ := shard2.run(t, g)

	require.Equal(t, c1, c1Bound,
		"a contribution must not move with γ: it hashes commitments, and γ is a cell")
	require.Equal(t, c2, c2Bound,
		"a contribution must not move with γ: it hashes commitments, and γ is a cell")

	require.Equal(t, g, multisethashing.ToSeed(multisethashing.Combine(c1Bound, c2Bound)),
		"γ must be recoverable from the contributions of the shards it was handed to")

	// The pair settles exactly, per handle. TestCrossShard_Bidirectional_Balanced
	// asserts the same balance on the same traffic, but under a fixed-seed test
	// hook; this is the only place the shards fold their rows under α and β
	// actually derived from a γ, with the PCS commitments in the transcript. Had
	// that seeding failed here, the two shards would fold under different
	// challenges and the products would not be inverses.
	require.Len(t, shard1.handles, len(crossShardHandles))
	for i, h := range shard1.handles {
		t.Run(h, func(t *testing.T) {
			p1 := shard1.product(rt1, i)
			p2 := shard2.product(rt2, i)

			require.False(t, equal(p1, field.ElemOne()),
				"shard 1 carries a net position on %q, so a product of one would mean the "+
					"folds degenerated and the inverse check below is vacuous", h)
			require.True(t, equal(p1.Mul(p2), field.ElemOne()),
				"the shards' net positions on %q must be inverses under the shared α and β", h)
		})
	}
}
