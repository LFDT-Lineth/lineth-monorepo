package codegen

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/recursion"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// E6CheckSystem is the compiled metadata for every [recursion.E6Check]
// registered on a wiop.System, in the form the Zig e6check sub-verifier
// consumes.
//
// A recursion program exposes an E6 value as one extension public input whose
// only tie to the proof is the E6Check: wiop has no extension constant, so the
// recomposition of the six base lanes cannot be a constraint, and only the
// lanes are bound by their openings. Both halves are plain transcript cells,
// so the Zig verifier recomposes the lanes and compares them with the public
// input against the adversary's transcript, like every other sub-verifier.
//
// Empty (no Checks) when sys exposes no E6 public input.
type E6CheckSystem struct {
	SourceName string
	Checks     []E6CheckEntry
}

// E6CheckEntry is one E6Check: Cell must equal the E6 value of Lanes, lanes in
// the order (B0.A0, B0.A1, B1.A0, B1.A1, B2.A0, B2.A1) of [recursion.CellE6].
type E6CheckEntry struct {
	Cell  ScalarCellRef
	Lanes [recursion.KindE6]ScalarCellRef
}

// BuildE6CheckSystem extracts every [recursion.E6Check] registered on sys, in
// round then registration order, and records the transcript positions of its
// extension cell and lane cells. It rejects an action whose shape the Zig
// sub-verifier could not check as the Go one does — a lane count other than
// six, an extension lane, a base public-input cell, a cell in the last wiop
// round (which the Zig verifier does not replay, so every proof, honest or not,
// would fail with CellRefOutOfRange) — rather than emit a system that drops,
// weakens or breaks the check.
func BuildE6CheckSystem(sys *wiop.System) (E6CheckSystem, error) {
	out := E6CheckSystem{SourceName: sys.Context.Path()}
	lastSlot := len(sys.Rounds) - 1
	for _, round := range sys.Rounds {
		for _, action := range round.VerifierActions {
			check, ok := action.(*recursion.E6Check)
			if !ok {
				continue
			}
			entry, err := buildE6CheckEntry(check, lastSlot)
			if err != nil {
				return E6CheckSystem{}, fmt.Errorf("codegen: BuildE6CheckSystem: round %d: %w", round.ID, err)
			}
			out.Checks = append(out.Checks, entry)
		}
	}
	return out, nil
}

func buildE6CheckEntry(check *recursion.E6Check, lastSlot int) (E6CheckEntry, error) {
	if check.Cell == nil {
		return E6CheckEntry{}, fmt.Errorf("E6Check has no cell")
	}
	path := check.Cell.Context.Path()
	if !check.Cell.IsExtension() {
		return E6CheckEntry{}, fmt.Errorf("E6Check %q: cell is not an extension cell", path)
	}
	if len(check.Lanes) != recursion.KindE6 {
		return E6CheckEntry{}, fmt.Errorf("E6Check %q: %d lanes, want %d", path, len(check.Lanes), recursion.KindE6)
	}
	if err := checkE6NotLastRound("cell", path, check.Cell, lastSlot); err != nil {
		return E6CheckEntry{}, err
	}
	entry := E6CheckEntry{Cell: cellRef(check.Cell)}
	for l, lane := range check.Lanes {
		if lane == nil {
			return E6CheckEntry{}, fmt.Errorf("E6Check %q: lane %d is nil", path, l)
		}
		if lane.IsExtension() {
			return E6CheckEntry{}, fmt.Errorf("E6Check %q: lane %d is an extension cell", path, l)
		}
		if err := checkE6NotLastRound(fmt.Sprintf("lane %d", l), path, lane, lastSlot); err != nil {
			return E6CheckEntry{}, err
		}
		entry.Lanes[l] = cellRef(lane)
	}
	return entry, nil
}

// checkE6NotLastRound rejects a cell in the last wiop round: verifier-ray
// replays rounds [0, lastSlot) only, so ctx.rounds has no such round.
func checkE6NotLastRound(kind, path string, c *wiop.Cell, lastSlot int) error {
	if slot := c.Context.ID.Slot(); slot >= lastSlot {
		return fmt.Errorf("E6Check %q: %s is in the last wiop round (slot %d), which verifier-ray does not replay",
			path, kind, slot)
	}
	return nil
}

func cellRef(c *wiop.Cell) ScalarCellRef {
	return ScalarCellRef{Round: c.Context.ID.Slot(), Index: c.Context.ID.Position()}
}
