package messagebus_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/messagebus"
	"github.com/stretchr/testify/require"
)

// buildShardWithBusOnRound declares one send and one receive column on round
// colRoundID of a system with numRounds rounds, and returns the system
// uncompiled. Round 0 is the seed round, so the coin round is round 1 and
// colRoundID is what decides whether the layout is the supported one.
func buildShardWithBusOnRound(t *testing.T, numRounds, colRoundID int) *wiop.System {
	t.Helper()
	require.Less(t, colRoundID, numRounds, "the column round must exist")

	sys := wiop.NewSystemf("round-placement-%d-of-%d", colRoundID, numRounds)
	for range numRounds {
		sys.NewRound()
	}
	r := sys.Rounds[colRoundID]
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)

	send := sys.NewMessageBusSend(
		sys.Context.Childf("send"), "shard-1", "route",
		wiop.NewTable(mod.NewColumn(sys.Context.Childf("a"), r).View()))
	recv := sys.NewMessageBusReceive(
		sys.Context.Childf("recv"), "shard-1", "route",
		wiop.NewTable(mod.NewColumn(sys.Context.Childf("b"), r).View()))
	send.SkipInShardCheck = true
	recv.SkipInShardCheck = true

	return sys
}

// requireGuardPanic runs f and asserts it panicked through the coin-round
// placement guard, rather than through some unrelated panic that happened to
// fire first.
func requireGuardPanic(t *testing.T, f func()) {
	t.Helper()
	defer func() {
		r := recover()
		require.NotNil(t, r, "expected the placement guard to panic")
		msg, ok := r.(string)
		require.True(t, ok, "the guard panics with a string, got %T", r)
		require.Contains(t, msg, "must live on the coin round",
			"the panic must come from the placement guard")
	}()
	f()
}

// TestSharedRandomness_BusColumnsMustBeOnTheCoinRound pins the layout the seeded
// path depends on. γ occupies round 0 and the coins are drawn on the way into
// round 1, so a bus column anywhere but round 1 breaks the argument in a way no
// proof would report
func TestSharedRandomness_BusColumnsMustBeOnTheCoinRound(t *testing.T) {
	compile := func(sys *wiop.System) func() {
		return func() {
			messagebus.Compile(sys, messagebus.CompileOptions{SharedRandomness: true})
		}
	}

	t.Run("before the coin round", func(t *testing.T) {
		requireGuardPanic(t, compile(buildShardWithBusOnRound(t, 1, 0)))
	})

	t.Run("after the coin round", func(t *testing.T) {
		requireGuardPanic(t, compile(buildShardWithBusOnRound(t, 3, 2)))
	})

	t.Run("on the coin round", func(t *testing.T) {
		require.NotPanics(t, compile(buildShardWithBusOnRound(t, 2, 1)),
			"bus columns on the coin round are the supported layout")
	})
}
