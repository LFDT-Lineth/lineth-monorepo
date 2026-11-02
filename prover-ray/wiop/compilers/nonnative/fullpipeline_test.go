package nonnative_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/wioptest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFullPipeline_NonNativeScenarios runs the full pipeline on every
// [wioptest.NonNativeScenarios] fixture. The nonnative pass reduces each
// [wiop.NonNative] query to a multi-valued [wiop.Vanishing] identity checked at
// a shared random point; this is then checked by global compiler using quotient
// argument.
func TestFullPipeline_NonNativeScenarios(t *testing.T) {
	for _, build := range wioptest.NonNativeScenarios() {
		sc := build()
		t.Run(sc.Name, func(t *testing.T) {
			require.NoError(t, compilers.CompileFull(sc.Sys, wioptest.TestingCompileOptions()...))
			proof, pub := sc.Sys.Prove(sc.AssignHonest)
			require.NoError(t, sc.Sys.Verify(proof, pub),
				"full pipeline must accept an honest witness")
		})

		t.Run(sc.Name+"/Soundness", func(t *testing.T) {
			sc := build()
			require.NoError(t, compilers.CompileFull(sc.Sys, wioptest.TestingCompileOptions()...))
			proof, pub := sc.Sys.Prove(sc.AssignInvalid)
			assert.Error(t, sc.Sys.Verify(proof, pub),
				"full pipeline must reject an invalid witness")
		})
	}
}
