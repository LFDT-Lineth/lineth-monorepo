package codegen

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/messagebus"
)

// SharedRandomnessSystem is the compiled metadata for a
// [messagebus.SharedRandomnessContributionChecker] registered on a wiop.System,
// in the form the Zig shared-randomness sub-verifier consumes.
//
// The checker recomputes a Poseidon2 Merkle-Damgård sponge hash over every
// round preceding the message-bus coin round that carries a PCS commitment,
// and compares the resulting digest, limb by limb, against public-input cells
// claiming to be this shard's contribution to the cross-shard shared
// randomness. Both halves — the ordered per-round commitment octuplets and the
// claimed-digest cell refs — are plain (round, index) transcript coordinates,
// so the Zig verifier can enforce the identity against the adversary's
// transcript the same way every other sub-verifier does; no baked-in
// honest-prover value is trusted.
//
// Absent (empty Rounds and ContributionRefs) when sys was not compiled with
// [messagebus.CompileOptions.SharedRandomness] — see
// [BuildSharedRandomnessSystem].
type SharedRandomnessSystem struct {
	SourceName string
	// Rounds is every round index in [0, coinRound.ID), in order, mirroring the
	// Go verifier's `for i := range rt.CurrentRound().ID` loop exactly. A round
	// with HasCommitment == false is skipped by the Zig checker, just as the Go
	// loop's `continue` skips it — its Commitment field is meaningless in that
	// case and left zero.
	Rounds []SharedRandomnessRound
	// ContributionRefs are the (round, index) transcript positions of the
	// [messagebus.SharedRandomnessSeedContributionPI] cells, one per limb of the
	// multiset-hash digest, in limb order.
	ContributionRefs []ScalarCellRef
}

// SharedRandomnessRound is one round's commitment status and, when present,
// its committed Octuplet — the same (round, index) coordinate style used by
// every other sub-verifier's cell references, except a round's commitment
// lives outside the cells slice (see verifier-ray's protocol.RoundMessage).
type SharedRandomnessRound struct {
	// RoundIndex is the wiop Round.ID / proof.rounds index this entry describes.
	RoundIndex int
	// HasCommitment mirrors wiop.Round.HasCommitment for this round. When
	// false, the Zig checker skips this round entirely, exactly as the Go
	// reference implementation's `continue` does.
	HasCommitment bool
}

// BuildSharedRandomnessSystem extracts the
// [messagebus.SharedRandomnessContributionChecker] verifier action registered
// on sys, if any, and records the ordered round list plus the contribution
// public-input cell refs it needs. Returns a zero-value SharedRandomnessSystem
// (no error) when sys carries no such action — a system compiled without
// [messagebus.CompileOptions.SharedRandomness] has nothing for this
// sub-verifier to check.
//
// sys must have been compiled with messagebus.Compile(sys,
// messagebus.CompileOptions{SharedRandomness: true}); the coin round the
// checker was registered on is read directly off the action via
// [wiop.Round.ID], so it can never drift from the round the prover's
// [messagebus.SharedRandomnessContributionAssigner] ran on.
func BuildSharedRandomnessSystem(sys *wiop.System) (SharedRandomnessSystem, error) {
	out := SharedRandomnessSystem{SourceName: sys.Context.Path()}

	var coinRound *wiop.Round
	for _, round := range sys.Rounds {
		for _, action := range round.VerifierActions {
			if _, ok := action.(*messagebus.SharedRandomnessContributionChecker); ok {
				coinRound = round
				break
			}
		}
		if coinRound != nil {
			break
		}
	}
	if coinRound == nil {
		return out, nil
	}

	out.Rounds = make([]SharedRandomnessRound, coinRound.ID)
	for i := 0; i < coinRound.ID; i++ {
		out.Rounds[i] = SharedRandomnessRound{
			RoundIndex:    i,
			HasCommitment: sys.Rounds[i].HasCommitment,
		}
	}

	for i := range messagebus.NumSharedRandomnessContribution {
		cell, pos := sys.LookupPublicInputByTag(messagebus.SharedRandomnessSeedContributionPI, i)
		if pos < 0 {
			return SharedRandomnessSystem{}, fmt.Errorf(
				"codegen: BuildSharedRandomnessSystem: missing contribution-%d public input, "+
					"despite a SharedRandomnessContributionChecker being registered", i)
		}
		out.ContributionRefs = append(out.ContributionRefs, ScalarCellRef{
			Round: cell.Context.ID.Slot(),
			Index: cell.Context.ID.Position(),
		})
	}

	return out, nil
}
