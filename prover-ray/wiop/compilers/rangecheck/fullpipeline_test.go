package rangecheck_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/wioptest"
	"github.com/stretchr/testify/require"
)

// TestFullPipeline_RangeCheckScenarios runs the full pipeline on every
// [wioptest.RangeCheckCompilerScenarios] fixture. Every step contributes:
// rangecheck → lookup → log-derivative → recurrence vanishings → global
// quotient.
func TestFullPipeline_RangeCheckScenarios(t *testing.T) {
	for _, build := range wioptest.RangeCheckCompilerScenarios() {
		sc := build()
		t.Run(sc.Name, func(t *testing.T) {
			require.NoError(t, compilers.CompileFull(sc.Sys, wioptest.TestingCompileOptions()...))
			proof, pub := sc.Sys.Prove(sc.AssignWitness)
			require.NoError(t, sc.Sys.Verify(proof, pub),
				"full pipeline must accept an honest witness")
		})
	}
}
