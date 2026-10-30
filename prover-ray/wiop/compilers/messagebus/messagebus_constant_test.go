package messagebus_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/messagebus"
	"github.com/stretchr/testify/require"
)

// buildConstantBus declares a send port (selA; kA, 7) and a receive port
// (kB, vB): the sender's value position is a constant, never committed.
func buildConstantBus(t *testing.T) (sys *wiop.System, kA, selA, kB, vB *wiop.Column) {
	t.Helper()
	sys = wiop.NewSystemf("mb-const")
	r0 := sys.NewRound()
	modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionNone)
	modB := sys.NewSizedModule(sys.Context.Childf("modB"), 2, wiop.PaddingDirectionNone)
	kA = modA.NewColumn(sys.Context.Childf("kA"), r0)
	selA = modA.NewColumn(sys.Context.Childf("selA"), r0)
	kB = modB.NewColumn(sys.Context.Childf("kB"), r0)
	vB = modB.NewColumn(sys.Context.Childf("vB"), r0)

	sys.NewMessageBusSend(
		sys.Context.Childf("send-A"), "shard", "kv",
		wiop.NewTableEntries(modA, selA.View(), kA.View(), wiop.NewConstantVector(modA, field.NewElement(7))))
	sys.NewMessageBusReceive(
		sys.Context.Childf("recv-B"), "shard", "kv",
		wiop.NewTable(kB.View(), vB.View()))
	return sys, kA, selA, kB, vB
}

func TestCompile_Bus_ConstantEntry_Balanced(t *testing.T) {
	sys, kA, selA, kB, vB := buildConstantBus(t)
	compilePermutationBusWithPCS(sys)

	proof, pub := sys.Prove(func(rt *wiop.Runtime) {
		rt.AssignColumn(kA, makeVec(1, 2, 3, 4))
		rt.AssignColumn(selA, makeVec(0, 1, 0, 1))
		rt.AssignColumn(kB, makeVec(4, 2))
		rt.AssignColumn(vB, makeVec(7, 7))
	})
	require.NoError(t, sys.Verify(proof, pub),
		"the sent tuples (2, 7), (4, 7) are exactly the received ones")
}

func TestCompile_Bus_ConstantEntry_Unbalanced(t *testing.T) {
	sys, kA, selA, kB, vB := buildConstantBus(t)
	compilePermutationBusWithPCS(sys)

	proof, pub := sys.Prove(func(rt *wiop.Runtime) {
		rt.AssignColumn(kA, makeVec(1, 2, 3, 4))
		rt.AssignColumn(selA, makeVec(0, 1, 0, 1))
		rt.AssignColumn(kB, makeVec(4, 2))
		rt.AssignColumn(vB, makeVec(7, 8)) // (2, 8) was never sent
	})
	require.Error(t, sys.Verify(proof, pub),
		"a received value differing from the sender's constant must be rejected")
}

// TestSharedRandomness_ConstantEntryIsNotMisplaced checks that the coin-round
// placement guard skips constant entries: they have no round, so they must
// neither be reported as living off the coin round nor hide a real column that
// is.
func TestSharedRandomness_ConstantEntryIsNotMisplaced(t *testing.T) {
	// build declares a send port (one, a) and a receive port (one, b), with a
	// on round aRound and b on the coin round (round 1). Round 0 is the seed
	// round, so the coin round is round 1.
	build := func(aRound int) (*wiop.System, *wiop.Column) {
		sys := wiop.NewSystemf("mb-const-placement")
		sys.NewRound()
		sys.NewRound()
		mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
		one := wiop.NewConstantVector(mod, field.One())
		a := mod.NewColumn(sys.Context.Childf("a"), sys.Rounds[aRound])
		b := mod.NewColumn(sys.Context.Childf("b"), sys.Rounds[1])

		send := sys.NewMessageBusSend(
			sys.Context.Childf("send"), "shard-1", "route",
			wiop.NewTableEntries(mod, nil, one, a.View()))
		recv := sys.NewMessageBusReceive(
			sys.Context.Childf("recv"), "shard-1", "route",
			wiop.NewTableEntries(mod, nil, one, b.View()))
		send.SkipInShardCheck = true
		recv.SkipInShardCheck = true
		return sys, a
	}

	t.Run("constants alone are not misplaced", func(t *testing.T) {
		sys, _ := build(1)
		require.NotPanics(t, func() { messagebus.Compile(sys) })
	})

	t.Run("a misplaced column is reported, not the constant", func(t *testing.T) {
		sys, a := build(0)
		defer func() {
			r := recover()
			require.NotNil(t, r, "the column on round 0 must trip the placement guard")
			msg, ok := r.(string)
			require.True(t, ok, "the guard panics with a string, got %T", r)
			require.Contains(t, msg, "must live on the coin round")
			require.Contains(t, msg, a.Context.Path(),
				"the panic must name the misplaced column, ahead of which sits a constant")
			require.Contains(t, msg, "round 0")
		}()
		messagebus.Compile(sys)
	})
}
