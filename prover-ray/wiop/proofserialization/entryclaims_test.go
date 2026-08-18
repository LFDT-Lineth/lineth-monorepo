package proofserialization_test

import (
	"testing"

	ps "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/proofserialization"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/wioptest"
	"github.com/stretchr/testify/require"
)

// TestEntryClaims_AreDerivedFromTheProof checks what prover-ray is able to check
// about the derived PCS entry claims: that they exist, that there is one entry
// per committed column, and that every value is a claim cell the proof already
// carries rather than something invented by the derivation.
//
// It deliberately does NOT check the entry ORDER. That ordering is owned by
// verifier-ray (its codegen's pcsEntryOrder, mirroring the Zig verifier's
// pcs.reconstruct), and neither authority can be imported here — they live in a
// module that depends on this one. The cross-check for the order belongs in
// verifier-ray, comparing this against codegen.ExtractPcsOpening for the same
// proof. Until that exists the ordering is unverified, which is recorded in
// deriveEntryClaims's own comment.
func TestEntryClaims_AreDerivedFromTheProof(t *testing.T) {
	for _, build := range wioptest.VanishingScenarios()[:8] {
		sc := build()
		t.Run(sc.Name, func(t *testing.T) {
			compileFullPipeline(sc.Sys)
			proof, pub := sc.Sys.Prove(sc.AssignHonest)
			require.NoError(t, sc.Sys.Verify(proof, pub))

			projected, err := ps.Project(sc.Sys, proof, pub)
			require.NoError(t, err)
			claims := projected.Proof.PcsOpening.EntryClaims

			// Non-vacuous: a PCS-compiled proof opens something. Without this the
			// tests below would all pass on an empty derivation.
			require.NotEmpty(t, claims,
				"a PCS-compiled proof must open at least one column")

			committed := 0
			for _, r := range sc.Sys.Rounds {
				committed += len(r.Columns)
			}
			require.Len(t, claims, committed,
				"one entry per committed column, since every committed column is opened")

			// Every claim value must be a value the proof already carries as a
			// LagrangeEval claim cell. This is the property that matters: the
			// derivation reorders existing data, it does not produce new data.
			fromCells := map[ps.Ext]int{}
			opened := 0
			for _, le := range sc.Sys.LagrangeEvals {
				for _, cell := range le.EvaluationClaims {
					v, ok := proof.Cells[cell.Context.ID]
					require.True(t, ok, "claim cell %q must be in the proof", cell.Context.Path())
					fromCells[ps.ExtFrom(v.Ext)]++
					opened++
				}
			}

			derived := 0
			for _, entry := range claims {
				for _, v := range entry {
					require.Positive(t, fromCells[v],
						"claim value %v is not any LagrangeEval cell in the proof", v)
					fromCells[v]--
					derived++
				}
			}
			require.Equal(t, opened, derived,
				"every opened (column, shift) must appear exactly once across the entries")
		})
	}
}

// TestEntryClaims_Deterministic guards the derivation against Go map iteration
// order leaking into the entry order, which would make the image non-reproducible
// and break the committed cross-language fixture at random.
func TestEntryClaims_Deterministic(t *testing.T) {
	sc := wioptest.VanishingScenarios()[13]() // MultiModule: several sizes and batches
	compileFullPipeline(sc.Sys)
	proof, pub := sc.Sys.Prove(sc.AssignHonest)

	first, err := ps.Project(sc.Sys, proof, pub)
	require.NoError(t, err)

	for range 8 {
		again, err := ps.Project(sc.Sys, proof, pub)
		require.NoError(t, err)
		require.Equal(t, first.Proof.PcsOpening.EntryClaims, again.Proof.PcsOpening.EntryClaims,
			"the entry order must not depend on map iteration order")
	}
}
