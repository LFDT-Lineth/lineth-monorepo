package messagebus_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/messagebus"
	"github.com/stretchr/testify/require"
)

// newBusPair declares one Send and one Receive on handle, each over its own
// fresh column, with the in-shard check skipped (these fixtures care about
// registration bookkeeping, not about the products balancing).
func newBusPair(sys *wiop.System, mod *wiop.Module, r0 *wiop.Round, handle string) {
	send := sys.NewMessageBusSend(
		sys.Context.Childf("send-%s", handle), "shard", handle,
		wiop.NewTable(mod.NewColumn(sys.Context.Childf("a-%s", handle), wiop.VisibilityOracle, r0).View()))
	recv := sys.NewMessageBusReceive(
		sys.Context.Childf("recv-%s", handle), "shard", handle,
		wiop.NewTable(mod.NewColumn(sys.Context.Childf("b-%s", handle), wiop.VisibilityOracle, r0).View()))
	send.SkipInShardCheck, recv.SkipInShardCheck = true, true
}

// TestCompileIsSingleInvocation checks the single-invocation contract. Compile
// numbers each handle's public-input tag by its index in that call's alphabetical
// order, so a second batch of entries would restart at MessageBus_0 and collide
// with the first batch — and the index would stop identifying the same handle
// across shards. Compile must therefore refuse the second batch up front, while
// still tolerating a repeat call that has nothing new to do.
func TestCompileIsSingleInvocation(t *testing.T) {
	newSys := func(name string) (*wiop.System, *wiop.Module, *wiop.Round) {
		sys := wiop.NewSystemf("single-invocation-%s", name)
		r0 := sys.NewRound()
		return sys, sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone), r0
	}

	// A second call carrying new entries is rejected.
	sys, mod, r0 := newSys("second-batch")
	newBusPair(sys, mod, r0, "alpha")
	messagebus.Compile(sys)
	require.Len(t, sys.PublicInputs, 1)

	newBusPair(sys, mod, r0, "beta")
	require.PanicsWithValue(t,
		"wiop/compilers/messagebus: Compile must be invoked at most once per system, but it is being "+
			"called again with new entries: \"single-invocation-second-batch/send-alpha\" (handle \"alpha\") "+
			"is already reduced while \"single-invocation-second-batch/send-beta\" (handle \"beta\") is not. "+
			"Declare every message-bus entry before the single Compile call.",
		func() { messagebus.Compile(sys) },
		"a second batch must be refused with the single-invocation message")

	// Declaring both handles up front is the supported way to get the same result.
	sysOK, modOK, r0OK := newSys("one-batch")
	newBusPair(sysOK, modOK, r0OK, "alpha")
	newBusPair(sysOK, modOK, r0OK, "beta")
	messagebus.Compile(sysOK)
	require.Equal(t, []string{"alpha", "beta"}, sysOK.MessageBusHandles())
	require.Len(t, sysOK.PublicInputs, 2)

	// A repeat call with nothing new stays a harmless no-op: no panic, and no
	// second registration of the same accumulators.
	require.NotPanics(t, func() { messagebus.Compile(sysOK) },
		"a repeat call with no new entries must be a no-op")
	require.Len(t, sysOK.PublicInputs, 2, "a no-op call must not register anything")
	require.Len(t, sysOK.GrandProducts, 2, "a no-op call must not emit new GrandProducts")
}

// TestMessageBusHandles pins the contract that ties [wiop.System.MessageBusHandles]
// to the public inputs the compiler registers: the handle list is deduplicated
// and sorted, it counts buses rather than participations, and its index i is the
// numeric suffix of the MessageBus_i public-input tag.
//
// This is the invariant a downstream consumer relies on to walk the message-bus
// public inputs — iterating sys.MessageBuses instead would overcount, because a
// handle has one entry per participant rather than one entry per bus.
func TestMessageBusHandles(t *testing.T) {
	sys := wiop.NewSystemf("handles")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)

	// Declare the handles out of alphabetical order, and give "zebra" three
	// participants (two sends, one receive) so the entry count and the bus count
	// genuinely differ.
	type participant struct {
		handle string
		send   bool
	}
	participants := []participant{
		{"zebra", true},
		{"apple", true},
		{"zebra", true},
		{"apple", false},
		{"zebra", false},
		{"mango", true},
		{"mango", false},
	}

	for i, p := range participants {
		col := mod.NewColumn(sys.Context.Childf("col-%d", i), wiop.VisibilityOracle, r0)
		tab := wiop.NewTable(col.View())
		var mb *wiop.MessageBus
		if p.send {
			mb = sys.NewMessageBusSend(sys.Context.Childf("s-%d", i), "shard", p.handle, tab)
		} else {
			mb = sys.NewMessageBusReceive(sys.Context.Childf("r-%d", i), "shard", p.handle, tab)
		}
		// The products are intentionally unbalanced; only the registration
		// bookkeeping is under test here, so the in-shard assertion is skipped.
		mb.SkipInShardCheck = true
	}

	handles := sys.MessageBusHandles()
	require.Equal(t, []string{"apple", "mango", "zebra"}, handles,
		"handles must be deduplicated and sorted")
	require.Len(t, sys.MessageBuses, len(participants),
		"MessageBuses counts participations, not buses")
	require.Less(t, len(handles), len(sys.MessageBuses),
		"this fixture must actually distinguish the two counts")

	messagebus.Compile(sys)

	// One public input per bus, not per participation.
	require.Len(t, sys.PublicInputs, len(handles))

	// Index i of the handle list is the suffix of the MessageBus_i tag, and it
	// resolves to that handle's GrandProduct accumulator.
	require.Len(t, sys.GrandProducts, len(handles))
	for i, h := range handles {
		cell, pos := sys.LookupPublicInputByTag(messagebus.PublicInputTag, i)
		require.NotNil(t, cell, "no public input tagged %s_%d", messagebus.PublicInputTag, i)
		require.Equal(t, i, pos)
		require.Contains(t, sys.GrandProducts[i].Context().Path(), "handle-"+h,
			"MessageBus_%d must be the accumulator of handle %q", i, h)
		require.Equal(t, sys.GrandProducts[i].Result, cell)
	}
}
