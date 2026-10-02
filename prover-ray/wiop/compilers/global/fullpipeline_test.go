package global_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/wioptest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFullPipeline_VanishingScenarios runs the full pipeline on every
// [wioptest.VanishingScenarios] fixture. These scenarios start with
// multi-valued [wiop.Vanishing] constraints; the local-vanishing pass is a
// no-op and the global pass discharges them through the quotient argument.
func TestFullPipeline_VanishingScenarios(t *testing.T) {
	for _, build := range wioptest.VanishingScenarios() {
		sc := build()
		t.Run(sc.Name, func(t *testing.T) {
			require.NoError(t, compilers.CompileFull(sc.Sys, wioptest.TestingCompileOptions()...))
			proof, pub := sc.Sys.Prove(sc.AssignHonest)
			require.NoError(t, sc.Sys.Verify(proof, pub),
				"full pipeline must accept an honest witness")
		})

		// Each soundness case rebuilds a fresh scenario so it doesn't share
		// compilation state with the completeness case above.
		t.Run(sc.Name+"/Soundness", func(t *testing.T) {
			sc := build()
			require.NoError(t, compilers.CompileFull(sc.Sys, wioptest.TestingCompileOptions()...))
			proof, pub := sc.Sys.Prove(sc.AssignInvalid)
			assert.Error(t, sc.Sys.Verify(proof, pub),
				"full pipeline must reject an invalid witness")
		})
	}
}
