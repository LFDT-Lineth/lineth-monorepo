package messagebus

import "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"

// latestRound returns the highest-ID round among rounds, skipping nil entries,
// or nil when every entry is nil. It is how [Compile] combines the two lower
// bounds on its result round — the coin rounds and the last participant round —
// neither of which dominates the other across the layouts callers use.
func latestRound(rounds ...*wiop.Round) *wiop.Round {
	var best *wiop.Round
	for _, r := range rounds {
		if r != nil && (best == nil || r.ID > best.ID) {
			best = r
		}
	}
	return best
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
