package wioptest

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
)

// RSBlowupBenchConfig pairs a Reed-Solomon blowup with the FRI query count
// that targets the same security level, so benchmarks compare blowups at equal
// security.
type RSBlowupBenchConfig struct {
	Blowup     int
	NumQueries int
}

// RSBlowupBenchConfigs returns the blowup/query pairings shared by the PCS and
// zkcdriver blowup benchmarks.
func RSBlowupBenchConfigs() []RSBlowupBenchConfig {
	return []RSBlowupBenchConfig{
		{Blowup: 2, NumQueries: 229},
		{Blowup: 4, NumQueries: 115},
		{Blowup: 8, NumQueries: 77},
		{Blowup: 16, NumQueries: 58},
	}
}

// Name is the benchmark sub-name of c.
func (c RSBlowupBenchConfig) Name() string {
	return fmt.Sprintf("blowup=%d/queries=%d", c.Blowup, c.NumQueries)
}

// PCSOptions returns the [pcs.Compile] options selecting c.
func (c RSBlowupBenchConfig) PCSOptions() []pcs.Option {
	return []pcs.Option{pcs.WithRSBlowup(c.Blowup), pcs.WithFRINumQueries(c.NumQueries)}
}

// CompileOptions returns the [compilers.CompileFull] options selecting c.
func (c RSBlowupBenchConfig) CompileOptions() []compilers.Option {
	return []compilers.Option{
		compilers.WithPCSOption(pcs.WithRSBlowup(c.Blowup)),
		compilers.WithPCSOption(pcs.WithFRINumQueries(c.NumQueries)),
	}
}
