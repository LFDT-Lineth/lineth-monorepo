package codegen

import (
	"fmt"
	"sort"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// PublicInputSystem is the transcript cell layout needed to rebuild
// prover-ray's split `(Proof, PublicInput)` wire format into verifier-visible
// round messages before Fiat-Shamir replay.
type PublicInputSystem struct {
	// RoundCellCounts[i] is the total verifier-visible cell count of replayed
	// round i (all cells in sys.Rounds[i], including ones promoted to public
	// inputs and therefore omitted from the proof's cell slices).
	RoundCellCounts []int
	// Refs records where each registered public input re-enters the transcript
	// cell stream. Entries are sorted by (Round, Index) so the Zig verifier can
	// merge them into the proof rounds in one pass while still indexing the flat
	// statement by StatementIndex (registration order).
	Refs []PublicInputRef
}

type PublicInputRef struct {
	StatementIndex int
	Round          int
	Index          int
}

// BuildPublicInputSystem extracts the replayed-round cell layout and the
// positions at which prover-ray's registered public inputs re-enter the shared
// transcript.
func BuildPublicInputSystem(sys *wiop.System) (PublicInputSystem, error) {
	out := PublicInputSystem{
		RoundCellCounts: make([]int, max(len(sys.Rounds)-1, 0)),
	}
	for i, round := range sys.Rounds {
		if i < len(sys.Rounds)-1 {
			out.RoundCellCounts[i] = len(round.Cells)
		}
	}

	lastReplayRound := len(sys.Rounds) - 1
	for statementIndex, cell := range sys.PublicInputs {
		round := cell.Context.ID.Slot()
		if round >= lastReplayRound {
			return PublicInputSystem{}, fmt.Errorf(
				"codegen: public input cell %q is in the last wiop round (slot %d); verifier-ray replays only rounds [0,%d)",
				cell.Context.Path(), round, lastReplayRound,
			)
		}

		ref := PublicInputRef{
			StatementIndex: statementIndex,
			Round:          round,
			Index:          cell.Context.ID.Position(),
		}
		if ref.Index >= out.RoundCellCounts[ref.Round] {
			return PublicInputSystem{}, fmt.Errorf(
				"codegen: public input cell %q has position %d outside round %d cell count %d",
				cell.Context.Path(), ref.Index, ref.Round, out.RoundCellCounts[ref.Round],
			)
		}
		out.Refs = append(out.Refs, ref)
	}

	sort.Slice(out.Refs, func(i, j int) bool {
		if out.Refs[i].Round != out.Refs[j].Round {
			return out.Refs[i].Round < out.Refs[j].Round
		}
		return out.Refs[i].Index < out.Refs[j].Index
	})

	return out, nil
}
