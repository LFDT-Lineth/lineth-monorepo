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
// The Z-recurrence and the L_0 initial condition are ordinary vanishing
// constraints (registered by the logderivativesum compiler), so they are
// discharged by the vanishing sub-verifier. What remains for this sub-verifier
// is the boundary identity the compiler's verifier action enforces:
//
//	Σ_entries Z[n-1] == Result        (and, for lookups, Result == 0)
//
// All operands are cell references (round, index) that the Zig verifier reads
// from ctx.rounds at verify time, so the check is against the adversary's
// transcript rather than baked-in honest-prover values.
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

// LogDerivQuery is one reduced LogDerivativeSum query: the transcript positions
// of Z[n-1] for each Z column and the claimed Result.
type LogDerivQuery struct {
	SourceName string
	// ZFinalRefs are the (round, index) positions of Z[n-1] for each Z column.
	ZFinalRefs []ScalarCellRef
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
// the Z-recurrence and L_0 initial condition are ordinary vanishing constraints
// that global.Compile discharges — without it those constraints go unchecked
// and the verifier is unsound. Calling global.Compile also appends
// quotientRound and evalRound after the logderivsum result round, which ensures
// result/ZFinal cells are never in the last wiop slot (protocol.replay excludes
// the last round from ctx.rounds). The check below turns a silent Zig
// out-of-bounds into a clear Go error at codegen time if global.Compile was
// accidentally omitted.
func BuildLogDerivSystem(sys *wiop.System) (LogDerivSystem, error) {
	out := LogDerivSystem{SourceName: sys.Context.Path()}
	lastSlot := len(sys.Rounds) - 1

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
				ZFinalRefs:   make([]ScalarCellRef, len(va.Entries)),
				ResultIsZero: resultMustBeZero[va.LogDerivativeSum],
			}
			for i, e := range va.Entries {
				zSlot := e.ZFinal.Context.ID.Slot()
				if err := checkNotLastSlot("z_final", e.ZFinal.Context.Path(), zSlot, lastSlot); err != nil {
					return LogDerivSystem{}, err
				}
				query.ZFinalRefs[i] = ScalarCellRef{Round: zSlot, Index: e.ZFinal.Context.ID.Position()}
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
