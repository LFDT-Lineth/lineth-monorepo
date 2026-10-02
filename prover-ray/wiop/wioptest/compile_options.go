package wioptest

import (
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
)

// TestingCompileOptions selects two FRI queries for full-pipeline scenarios.
// It leaves messagebus shared randomness at its default; bus-bearing scenarios
// whose columns are on round 0 must explicitly opt out. The reduced query
// count must not be used in production.
func TestingCompileOptions() []compilers.Option {
	return []compilers.Option{
		compilers.WithPCSOption(pcs.WithFRINumQueries(2)),
	}
}
