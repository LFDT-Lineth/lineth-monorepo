package codegen

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver/risc5"
)

// ProgramVKSystem is the compiled metadata for a [risc5.ProgramVKChecker]
// registered on a wiop.System, in the form the Zig program-VK sub-verifier
// consumes.
//
// The checker compares, limb by limb, the public-input cells claiming to be the
// program verification key against the PCS commitment to the program round.
// Both halves are plain transcript coordinates, so the Zig verifier enforces
// the identity against the adversary's transcript; no baked-in honest-prover
// value is trusted.
//
// Absent (no VKRefs) when sys carries no program VK, i.e. when
// [risc5.RegisterProgramVK] was not called — see [BuildProgramVKSystem].
type ProgramVKSystem struct {
	SourceName string
	// CommitmentRound is the round whose commitment is the program VK:
	// prover-ray's programCommitment reads rt.Commitments[zkcdriver.ProgramRound].
	CommitmentRound CommitmentRoundCtx
	// VKRefs are the (round, index) transcript positions of the
	// [risc5.ProgramVKPI] cells, one per limb of the commitment, in limb order.
	VKRefs []ScalarCellRef
}

// BuildProgramVKSystem extracts the [risc5.ProgramVKChecker] verifier action
// registered on sys, if any, and records the round whose commitment it reads
// plus the program-VK public-input cell refs it needs. Returns a zero-value
// ProgramVKSystem (no error) when sys carries no such action.
//
// The round is read off the action's own [wiop.Round.ID], so it can never
// drift from the round whose commitment the prover publishes.
func BuildProgramVKSystem(sys *wiop.System) (ProgramVKSystem, error) {
	out := ProgramVKSystem{SourceName: sys.Context.Path()}

	var programRound *wiop.Round
	for _, round := range sys.Rounds {
		for _, action := range round.VerifierActions {
			if _, ok := action.(*risc5.ProgramVKChecker); ok {
				programRound = round
				break
			}
		}
		if programRound != nil {
			break
		}
	}
	if programRound == nil {
		return out, nil
	}

	out.CommitmentRound = CommitmentRoundCtx{
		RoundIndex:    programRound.ID,
		HasCommitment: programRound.HasCommitment,
	}

	for i := range risc5.NumProgramVK {
		cell, pos := sys.LookupPublicInputByTag(risc5.ProgramVKPI, i)
		if pos < 0 {
			return ProgramVKSystem{}, fmt.Errorf(
				"codegen: BuildProgramVKSystem: missing program-vk-%d public input, "+
					"despite a ProgramVKChecker being registered", i)
		}
		out.VKRefs = append(out.VKRefs, ScalarCellRef{
			Round: cell.Context.ID.Slot(),
			Index: cell.Context.ID.Position(),
		})
	}

	return out, nil
}
