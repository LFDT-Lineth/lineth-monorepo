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
// The checker recomputes the multiset hash of the message-bus columns,
// PCS commitment and compares it, limb by limb, against public-input cells
// claiming to be this shard's contribution to the cross-shard shared
// randomness. Both halves — the commitment's round and the claimed-digest cell
// refs — are plain transcript coordinates, so the Zig verifier can enforce the
// identity against the adversary's transcript the same way every other
// sub-verifier does; no baked-in honest-prover value is trusted.
//
// Absent (no ContributionRefs) when sys was not compiled with
// [messagebus.CompileOptions.SharedRandomness] — see
// [BuildSharedRandomnessSystem].
type SharedRandomnessSystem struct {
	SourceName string
	// CommitmentRound is the round whose commitment is the whole sponge
	// preimage: prover-ray's sharedRandomnessContribution hashes
	// rt.Commitments[rt.CurrentRound().ID] and nothing else. The preflight
	// columns are assumed isolated on that round — an assumption prover-ray's
	// backend enforces.
	CommitmentRound CommitmentRoundCtx
	// ContributionRefs are the (round, index) transcript positions of the
	// [messagebus.SharedRandomnessSeedContributionPI] cells, one per limb of the
	// multiset-hash digest, in limb order.
	ContributionRefs []ScalarCellRef
}

// CommitmentRoundCtx names the round whose commitment feeds the hash,
type CommitmentRoundCtx struct {
	// RoundIndex is the wiop Round.ID / proof.rounds index this entry describes.
	RoundIndex int
	// HasCommitment mirrors wiop.Round.HasCommitment for this round. When
	// false the prover hashes a zero Octuplet — a Go map miss on
	// rt.Commitments — so the Zig checker must do the same to stay in step.
	HasCommitment bool
}

// BuildSharedRandomnessSystem extracts the
// [messagebus.SharedRandomnessContributionChecker] verifier action registered
// on sys, if any, and records the round whose commitment it hashes plus the
// contribution public-input cell refs it needs. Returns a zero-value
// SharedRandomnessSystem (no error) when sys carries no such action — a system
// compiled without [messagebus.CompileOptions.SharedRandomness] has nothing for
// this sub-verifier to check.
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

	out.CommitmentRound = CommitmentRoundCtx{
		RoundIndex:    coinRound.ID,
		HasCommitment: coinRound.HasCommitment,
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
