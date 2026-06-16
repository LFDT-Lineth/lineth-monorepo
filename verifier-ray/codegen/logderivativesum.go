package codegen

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover-ray/wiop"
	"github.com/consensys/linea-monorepo/prover-ray/wiop/compilers/logderivativesum"
	"github.com/consensys/linea-monorepo/prover-ray/wiop/compilers/lookuptologderivsum"
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
// Returns an error if any cell (ZFinal or Result) lives in sys.Rounds[len-1].
// protocol.replay only carries len(sys.Rounds)-1 rounds into ctx.rounds, so a
// cell in the last round would produce an out-of-bounds index in the Zig
// verifier. The fix is to run global.Compile after logderivativesum.Compile so
// that two rounds are appended after the logderivsum result round.
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
				resultMustBeZero[la.Ld] = true
			}
		}
	}

	for _, round := range sys.Rounds {
		for _, action := range round.VerifierActions {
			va, ok := action.(*logderivativesum.VerifierAction)
			if !ok {
				continue
			}

			resultSlot := va.Ld.Result.Context.ID.Slot()
			if resultSlot == lastSlot {
				return LogDerivSystem{}, fmt.Errorf(
					"codegen: logderivsum result cell %q is in the last wiop round (slot %d); "+
						"ctx.rounds excludes the last round — run global.Compile after logderivativesum.Compile",
					va.Ld.Result.Context.Path(), lastSlot,
				)
			}

			query := LogDerivQuery{
				SourceName:   va.Ld.Context().Path(),
				ResultRef:    ScalarCellRef{Round: resultSlot, Index: va.Ld.Result.Context.ID.Position()},
				ZFinalRefs:   make([]ScalarCellRef, len(va.Entries)),
				ResultIsZero: resultMustBeZero[va.Ld],
			}
			for i, e := range va.Entries {
				zSlot := e.ZFinal.Context.ID.Slot()
				if zSlot == lastSlot {
					return LogDerivSystem{}, fmt.Errorf(
						"codegen: logderivsum z_final cell %q is in the last wiop round (slot %d); "+
							"ctx.rounds excludes the last round — run global.Compile after logderivativesum.Compile",
						e.ZFinal.Context.Path(), lastSlot,
					)
				}
				query.ZFinalRefs[i] = ScalarCellRef{Round: zSlot, Index: e.ZFinal.Context.ID.Position()}
			}
			out.Queries = append(out.Queries, query)
		}
	}

	return out, nil
}
