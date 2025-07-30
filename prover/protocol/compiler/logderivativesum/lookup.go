package logderivativesum

import "github.com/consensys/linea-monorepo/prover/protocol/wizard"

// CompileLookups compiles the [query.Inclusion] queries that are present
// in comp using the [LogDerivativeSum] approach. First, the queries are
// grouped into a big [LogDerivativeSum] query and then, the query is
// compiled using global-constraints and local-constraints.
func CompileLookups(comp *wizard.CompiledIOP) {
	LookupIntoLogDerivativeSum(comp)
	CompileLogDerivativeSum(comp)
}
