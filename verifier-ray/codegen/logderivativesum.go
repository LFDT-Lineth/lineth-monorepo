package codegen

import (
	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
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
// All operands are concrete field extension values extracted from the honest
// prover run, so no ctx.rounds lookup is needed at verify time.
type LogDerivSystem struct {
	SourceName string
	Queries    []LogDerivQuery
}

// LogDerivQuery is one reduced LogDerivativeSum query: the concrete extension
// values of Z[n-1] for each Z column and the claimed Result.
type LogDerivQuery struct {
	SourceName string
	// ZFinals are the honest prover's Z[n-1] values for each Z column.
	ZFinals []field.Ext
	// Result is the honest prover's claimed aggregated value.
	Result field.Ext
	// ResultIsZero is set for lookup-reduced queries, whose Result must be 0.
	ResultIsZero bool
}

// BuildLogDerivSystem extracts the LogDerivativeSum verifier actions registered
// on sys and resolves their cell values from rt into a LogDerivSystem. Queries
// are collected in round/registration order so the output is deterministic.
//
// The error return is kept for API symmetry with BuildVanishingSystem (which
// can fail on unsupported expression types). BuildLogDerivSystem itself never
// returns a non-nil error because LogDerivativeSum operands are always cell
// references, which require no expression compilation.
func BuildLogDerivSystem(sys *wiop.System, rt wiop.Runtime) (LogDerivSystem, error) {
	out := LogDerivSystem{SourceName: sys.Context.Path()}

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
			query := LogDerivQuery{
				SourceName:   va.Ld.Context().Path(),
				Result:       rt.GetCellValue(va.Ld.Result).AsExt(),
				ZFinals:      make([]field.Ext, len(va.Entries)),
				ResultIsZero: resultMustBeZero[va.Ld],
			}
			for i, e := range va.Entries {
				query.ZFinals[i] = rt.GetCellValue(e.ZFinal).AsExt()
			}
			out.Queries = append(out.Queries, query)
		}
	}

	return out, nil
}
