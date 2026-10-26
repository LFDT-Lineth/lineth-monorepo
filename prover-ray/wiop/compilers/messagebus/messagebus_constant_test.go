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

func TestCompile_Permutation_ConstantEntry_Balanced(t *testing.T) {
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

func TestCompile_Permutation_ConstantEntry_Unbalanced(t *testing.T) {
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
// placement guard skips constant entries: they have no round, and must not be
// reported as living off the coin round.
func TestSharedRandomness_ConstantEntryIsNotMisplaced(t *testing.T) {
	sys := wiop.NewSystemf("mb-const-placement")
	sys.NewRound()
	r1 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	one := wiop.NewConstantVector(mod, field.One())

	send := sys.NewMessageBusSend(
		sys.Context.Childf("send"), "shard-1", "route",
		wiop.NewTableEntries(mod, nil, mod.NewColumn(sys.Context.Childf("a"), r1).View(), one))
	recv := sys.NewMessageBusReceive(
		sys.Context.Childf("recv"), "shard-1", "route",
		wiop.NewTableEntries(mod, nil, one, mod.NewColumn(sys.Context.Childf("b"), r1).View()))
	send.SkipInShardCheck = true
	recv.SkipInShardCheck = true

	require.NotPanics(t, func() { messagebus.Compile(sys) })
}
