package messagebus

import (
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/preflight"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// RegisterPreflightSeed wires a cross-shard shared-randomness seed into the
// message-bus coin round. Call it after declaring all [wiop.MessageBus] entries
// and before [Compile].
//
// It finds the latest round touched by any unreduced MessageBus participant
// column, pre-allocates the coin round immediately after it (via the same
// [ensureRoundAfter] logic [Compile] uses), and registers a
// [wiop.Round.RegisterPreSamplingHook] that calls [wiop.Runtime.SetFSState]
// with the additive hash of the cross-shard column sets. Because
// [wiop.Runtime.AdvanceRound] fires hooks on both the prover and the verifier,
// α and β are derived from the same seeded state on every participating shard
// without any extra verifier action.
//
// When [Compile] subsequently runs, its [ensureRoundAfter] call finds the
// pre-allocated tail round and reuses it, so α and β land on the hooked round.
//
// The returned round is the pre-allocated coin round; callers rarely need it,
// but it is exposed for inspection or additional hook registration.
func RegisterPreflightSeed[P any](
	sys *wiop.System,
	sets []preflight.BusInputSet,
	hasher preflight.AdditiveHasher[P],
) *wiop.Round {
	coinRound := ensureRoundAfter(sys, latestUnreducedParticipantRound(sys))
	coinRound.RegisterPreSamplingHook(&PreflightSeedHook[P]{sets: sets, hasher: hasher})
	return coinRound
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

// PreflightSeedHook is a [wiop.ProverAction] that fires as a PreSamplingHook
// on the coin round. It recomputes the shared Fiat-Shamir seed from the
// pre-distributed cross-shard column sets and injects it via SetFSState so
// that the subsequent α and β coins are identical on every participating shard.
// Because [wiop.Runtime.AdvanceRound] runs PreSamplingHooks on both the prover
// and the verifier, no separate verifier action is needed.
type PreflightSeedHook[P any] struct {
	sets   []preflight.BusInputSet
	hasher preflight.AdditiveHasher[P]
}

// Run implements [wiop.ProverAction].
func (h *PreflightSeedHook[P]) Run(rt *wiop.Runtime) {
	rt.SetFSState(preflight.Run(h.sets, h.hasher))
}
