package messagebus_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/fri"
	multisethashing "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/multiset_hashing"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/preflight"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/messagebus"
	"github.com/stretchr/testify/require"
)

// sharedBusInputSets returns a minimal but concrete []preflight.BusInputSet.
// It contains a single set with one base column of one element, committed via
// a rate-4 RS encoder. The exact values are unimportant — what matters is that
// every shard calling this function gets the same slice and therefore produces
// the same preflight seed.
func sharedBusInputSets() []preflight.BusInputSet {
	enc := fri.NewEncoder(4, 1) // codeword size 4, plaintext size 1
	var elem field.Element
	elem.SetUint64(42)
	table := fri.MultiSizeTable{fri.SizedTable{Base: [][]field.Element{{elem}}}}
	return []preflight.BusInputSet{{Table: table, Encoders: []*fri.RSEncoder{&enc}}}
}

// buildPreflightShard constructs a single-direction shard (Send or Receive)
// over four rows, registers the preflight seed hook, compiles, and drives the
// prover to completion. The returned runtime has all coins sampled and all
// prover actions executed.
//
// SkipInShardCheck is set to true because a single-direction shard is
// intentionally unbalanced on its own; the cross-shard join (product of all
// shards == 1) is checked by the caller.
func buildPreflightShard(
	t *testing.T,
	name, originShard string,
	dir wiop.BusDirection,
	vals []uint64,
	sets []preflight.BusInputSet,
) (*wiop.Runtime, *wiop.System) {
	t.Helper()

	sys := wiop.NewSystemf("%s", name)
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), len(vals), wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("c"), r0)

	tab := wiop.NewTable(col.View())
	var mb *wiop.MessageBus
	switch dir {
	case wiop.BusSend:
		mb = sys.NewMessageBusSend(sys.Context.Childf("entry"), originShard, "handle", tab)
	case wiop.BusReceive:
		mb = sys.NewMessageBusReceive(sys.Context.Childf("entry"), originShard, "handle", tab)
	default:
		t.Fatalf("unexpected direction %v", dir)
	}
	mb.SkipInShardCheck = true

	messagebus.RegisterPreflightSeed(sys, sets, multisethashing.Hasher{})
	compilePermutationBus(sys)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(col, makeVec(vals...))
	drive(rt)

	return rt, sys
}

// TestPreflightSeedHook_SameCoinsOnTwoShards is the positive test: two
// independent prover instances, each holding one side of a cross-shard
// permutation, use the same BusInputSet. The test asserts that:
//
//  1. α and β are byte-for-byte identical on both shards — the hook replaces
//     the local Fiat-Shamir state with the shared seed before the coins are
//     sampled.
//  2. The cross-shard grand-product (product of the two partial accumulators)
//     equals one — the correct balance condition when the sent and received
//     multisets coincide.
func TestPreflightSeedHook_SameCoinsOnTwoShards(t *testing.T) {
	sets := sharedBusInputSets()

	// Shard 1 sends rows [10,20,30,40]; shard 2 receives them in a different
	// order. Both use the same sets so the hook produces the same seed.
	rt1, sys1 := buildPreflightShard(t, "shard-1", "shard-1", wiop.BusSend,
		[]uint64{10, 20, 30, 40}, sets)
	rt2, sys2 := buildPreflightShard(t, "shard-2", "shard-2", wiop.BusReceive,
		[]uint64{40, 30, 20, 10}, sets)

	// The coin round is sys.Rounds[1]; coins are α (index 0) then β (index 1)
	// in declaration order from messagebus.Compile.
	coinRound1 := sys1.Rounds[1]
	coinRound2 := sys2.Rounds[1]

	alpha1 := rt1.GetCoinValue(coinRound1.Coins[0])
	alpha2 := rt2.GetCoinValue(coinRound2.Coins[0])
	diffAlpha := alpha1.Sub(alpha2)
	require.True(t, diffAlpha.IsZero(),
		"α must be identical on both shards when they share the same BusInputSet")

	beta1 := rt1.GetCoinValue(coinRound1.Coins[1])
	beta2 := rt2.GetCoinValue(coinRound2.Coins[1])
	diffBeta := beta1.Sub(beta2)
	require.True(t, diffBeta.IsZero(),
		"β must be identical on both shards when they share the same BusInputSet")

	// Extra sanity: the cross-shard grand product equals one when the multisets
	// match and both shards use the same β.
	p1 := rt1.GetCellValue(sys1.GrandProducts[0].Result)
	p2 := rt2.GetCellValue(sys2.GrandProducts[0].Result)
	crossProduct := p1.Mul(p2)
	diffCross := crossProduct.Sub(field.ElemOne())
	require.True(t, diffCross.IsZero(),
		"cross-shard product must equal one when sent and received multisets coincide")
}

// TestPreflightSeedHook_DifferentCoinsWithoutHook is the negative test: when
// the preflight hook is absent, each shard's coins are derived from its local
// Fiat-Shamir transcript alone. Because the two shards hold different column
// data, their transcripts diverge and α/β differ, breaking the cross-shard
// balance.
func TestPreflightSeedHook_DifferentCoinsWithoutHook(t *testing.T) {
	// Build shard 1 WITH the preflight hook.
	sets := sharedBusInputSets()
	rt1, sys1 := buildPreflightShard(t, "shard-1", "shard-1", wiop.BusSend,
		[]uint64{10, 20, 30, 40}, sets)

	// Build shard 2 WITHOUT the preflight hook: coins are purely FS-derived
	// from whatever columns were absorbed into the transcript on round 0.
	sys2 := wiop.NewSystemf("shard-2")
	r0 := sys2.NewRound()
	mod2 := sys2.NewSizedModule(sys2.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col2 := mod2.NewColumn(sys2.Context.Childf("c"), r0)
	mb2 := sys2.NewMessageBusReceive(sys2.Context.Childf("entry"), "shard-2", "handle",
		wiop.NewTable(col2.View()))
	mb2.SkipInShardCheck = true
	compilePermutationBus(sys2) // no RegisterPreflightSeed

	rt2 := wiop.NewRuntime(sys2)
	rt2.AssignColumn(col2, makeVec(40, 30, 20, 10))
	drive(rt2)

	// The column data absorbed into FS on each shard differs, so without the
	// hook the resulting coins will diverge (with overwhelming probability).
	beta1 := rt1.GetCoinValue(sys1.Rounds[1].Coins[1])
	beta2 := rt2.GetCoinValue(sys2.Rounds[1].Coins[1])
	diffBeta := beta1.Sub(beta2)
	require.False(t, diffBeta.IsZero(),
		"β must differ between shards when the preflight hook is absent and column data differs")
}
