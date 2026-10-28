package codegen

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/logderivativesum"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/lookuptologderivsum"
)

// LogDerivSystem is the compiled metadata for every LogDerivativeSum query in a
// wiop.System, in the form the Zig logderivativesum sub-verifier consumes.
//
// The cyclic Z-recurrence Z[i] − Z[i−1] = v[i] − C (row 0 reading Z[n−1]) is
// an ordinary vanishing constraint (registered by the logderivativesum
// compiler), so it is discharged by the vanishing sub-verifier; summed over
// the rows, it makes n·C the column's total. What remains for this
// sub-verifier is the identity the compiler's verifier action enforces:
//
//	Σ_entries n·C == Result        (and, for lookups, Result == 0)
//
// C and Result are cell references (round, index) that the Zig verifier reads
// from ctx.rounds at verify time, so the check is against the adversary's
// transcript rather than baked-in honest-prover values; n is the static size
// or the proof-supplied size of a dynamic module.
type LogDerivSystem struct {
	SourceName string
	Queries    []LogDerivQuery
}

// ScalarCellRef locates a cell in the proof transcript by its (round, index)
// coordinates. Round is the proof.rounds index (0-based); Index is the
// position within that round's cells slice. This mirrors the ObjectID.Slot() /
// .Position() encoding used throughout the vanishing codegen.
type ScalarCellRef struct {
	Round int
	Index int
}

// LogDerivEntry is one Z column: the transcript position of its average cell C
// and the size n of its module.
type LogDerivEntry struct {
	AverageRef ScalarCellRef
	Size       ModuleSize
}

// LogDerivQuery is one reduced LogDerivativeSum query: its Z columns and the
// claimed Result.
type LogDerivQuery struct {
	SourceName string
	// Entries are the Z columns, in the order of the verifier action's entries.
	Entries []LogDerivEntry
	// ResultRef is the (round, index) position of the claimed aggregated value.
	ResultRef ScalarCellRef
	// ResultIsZero is set for lookup-reduced queries, whose Result must be 0.
	ResultIsZero bool
}

// BuildLogDerivSystem extracts the LogDerivativeSum verifier actions registered
// on sys and records their cell-reference coordinates in a LogDerivSystem.
// Queries are collected in round/registration order so the output is
// deterministic.
//
// Requires global.Compile to have been called after logderivativesum.Compile:
// the cyclic Z-recurrences are ordinary vanishing constraints that
// global.Compile discharges — without it those constraints go unchecked and
// the verifier is unsound. Calling global.Compile also appends quotientRound
// and evalRound after the logderivsum result round, which ensures result and
// average cells are never in the last wiop slot (protocol.replay excludes
// the last round from ctx.rounds). The check below turns a silent Zig
// out-of-bounds into a clear Go error at codegen time if global.Compile was
// accidentally omitted.
func BuildLogDerivSystem(sys *wiop.System) (LogDerivSystem, error) {
	out := LogDerivSystem{SourceName: sys.Context.Path()}
	lastSlot := len(sys.Rounds) - 1
	dynamicIndices := DynamicModuleIndex(sys)

	// First pass: collect the LogDerivativeSum queries that a lookup reduction
	// requires to be zero (lookuptologderivsum registers a ResultIsZero action
	// alongside the logderivativesum reduction).
	resultMustBeZero := map[*wiop.LogDerivativeSum]bool{}
	for _, round := range sys.Rounds {
		for _, action := range round.VerifierActions {
			if la, ok := action.(*lookuptologderivsum.ResultIsZeroVerifierAction); ok {
				resultMustBeZero[la.LogDerivativeSum] = true
			}
		}
	}

	for _, round := range sys.Rounds {
		for _, action := range round.VerifierActions {
			va, ok := action.(*logderivativesum.VerifierAction)
			if !ok {
				continue
			}

			resultSlot := va.LogDerivativeSum.Result.Context.ID.Slot()
			if err := checkNotLastSlot("result", va.LogDerivativeSum.Result.Context.Path(), resultSlot, lastSlot); err != nil {
				return LogDerivSystem{}, err
			}

			query := LogDerivQuery{
				SourceName:   va.LogDerivativeSum.Context().Path(),
				ResultRef:    ScalarCellRef{Round: resultSlot, Index: va.LogDerivativeSum.Result.Context.ID.Position()},
				Entries:      make([]LogDerivEntry, len(va.Entries)),
				ResultIsZero: resultMustBeZero[va.LogDerivativeSum],
			}
			for i, e := range va.Entries {
				cSlot := e.Average.Context.ID.Slot()
				if err := checkNotLastSlot("average", e.Average.Context.Path(), cSlot, lastSlot); err != nil {
					return LogDerivSystem{}, err
				}
				size, err := moduleSizeOf(e.Module(), dynamicIndices)
				if err != nil {
					return LogDerivSystem{}, err
				}
				query.Entries[i] = LogDerivEntry{
					AverageRef: ScalarCellRef{Round: cSlot, Index: e.Average.Context.ID.Position()},
					Size:       size,
				}
			}
			out.Queries = append(out.Queries, query)
		}
	}

	return out, nil
}

// checkNotLastSlot is a defence-in-depth guard used by BuildLogDerivSystem.
// It returns a clear error when a cell sits in the last wiop round — a slot
// that protocol.replay excludes from ctx.rounds. In a correctly assembled
// system (global.Compile called after logderivativesum.Compile) this is never
// triggered; it exists solely to surface a forgotten global.Compile call at
// codegen time rather than as a silent out-of-bounds in the Zig verifier.
func checkNotLastSlot(kind, path string, slot, lastSlot int) error {
	if slot != lastSlot {
		return nil
	}
	return fmt.Errorf(
		"codegen: logderivsum %s cell %q is in the last wiop round (slot %d); "+
			"global.Compile must be called after logderivativesum.Compile — "+
			"it is required for soundness and also ensures cells never land in the last round",
		kind, path, slot,
	)
}
