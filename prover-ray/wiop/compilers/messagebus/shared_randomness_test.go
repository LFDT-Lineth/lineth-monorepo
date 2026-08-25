package messagebus_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/grandproduct"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/messagebus"
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
// replaced each shard's local state with γ before sampling. Their accumulators
// must then multiply to one, the cross-shard balance condition.
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
// challenge while still appearing to share randomness.
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
