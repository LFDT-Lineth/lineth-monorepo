package messagebus

import "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"

// ensureCoinRound returns the round on which [Compile] declares the shared α
// and β coins, allocating it if it does not exist yet.
//
// A sharded protocol needs this round *before* [Compile] runs, so it can
// register a [wiop.Round.RegisterPreSamplingHook] that seeds the Fiat-Shamir
// state with the shared randomness every shard agrees on. [Compile] calls this
// same function rather than repeating the lookup, so the round a caller
// pre-allocated and the round α and β land on are the same by construction.
//
// The result is one past the last bus-impacting round. In the sharded RISC-V
// layout that means: round 0 commits the program verification data, round 1
// commits the columns the message bus reads, and the coins therefore land on
// round 2 — after everything the bus binds, and before the shard-specific data
// that must not influence the shared challenges.
//
// Call it after every [wiop.MessageBus] entry has been declared and before
// [Compile]. The round is derived from the participant columns, so an entry
// declared afterwards can move Compile's choice and leave the hook stranded on
// a round that no longer carries the coins — a divergence that produces
// mismatched challenges across shards rather than an error.
func ensureCoinRound(sys *wiop.System) *wiop.Round {
	return ensureRoundAfter(sys, latestUnreducedParticipantRound(sys))
}

// latestUnreducedParticipantRound returns the highest-ID round touched by any
// unreduced [wiop.MessageBus] entry in sys, or nil if no such entry exists.
// It mirrors the logic of [latestParticipantRound] but operates directly on
// sys.MessageBuses rather than on a pre-built by-handle map, so it can be
// called before [Compile] has grouped entries.
func latestUnreducedParticipantRound(sys *wiop.System) *wiop.Round {
	var best *wiop.Round
	for _, mb := range sys.MessageBuses {
		if mb.IsReduced() {
			continue
		}
		if r := mb.Round(); r != nil && (best == nil || r.ID > best.ID) {
			best = r
		}
	}
	return best
}
