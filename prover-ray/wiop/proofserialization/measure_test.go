package proofserialization_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/proofserialization"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/wioptest"
	"github.com/stretchr/testify/require"
)

// TestMeasure checks that Measure agrees with the proof it is given, on a real
// PCS-compiled proof built entirely from wiop primitives — no ZKC program, no
// arithmetization, nothing to compile out of tree.
func TestMeasure(t *testing.T) {
	for _, build := range wioptest.VanishingScenarios() {
		sc := build()
		t.Run(sc.Name, func(t *testing.T) {
			require.NoError(t, compilers.CompileFull(sc.Sys))
			proof, pub := sc.Sys.Prove(sc.AssignHonest)
			require.NoError(t, sc.Sys.Verify(proof, pub), "measured proof must verify")

			s := proofserialization.Measure(sc.Sys, proof, pub)

			require.Equal(t, max(len(sc.Sys.Rounds)-1, 0), s.Rounds, "round count must match replayed rounds (last excluded)")
			require.Equal(t, len(proof.Cells), s.Cells, "cell count must match the proof")
			require.Equal(t, len(proof.Cells), s.BaseCells+s.ExtCells,
				"every cell must be classified as base or ext")
			require.True(t, s.HasPCS, "the full pipeline ends with the PCS pass")
			require.Equal(t, pcs.FRINumQueries(sc.Sys), s.Queries,
				"one input query per FRI query")
			require.Positive(t, s.Total, "a PCS-compiled proof cannot have an empty image")
			require.GreaterOrEqual(t, s.Total, s.Payload,
				"payload is a subset of the image, so overhead cannot be negative")

			t.Logf("\n=== %s ===\n%s", sc.Name, s)
		})
	}
}

// TestMeasure_NoPCS documents what a non-PCS-compiled proof measures to: the
// round messages only, with the FRI fields left zero rather than guessed at.
func TestMeasure_NoPCS(t *testing.T) {
	sc := wioptest.VanishingScenarios()[0]()
	require.NoError(t, compilers.CompileFull(sc.Sys,
		compilers.WithoutPCS(),
	))

	proof, pub := sc.Sys.Prove(sc.AssignHonest)
	s := proofserialization.Measure(sc.Sys, proof, pub)

	require.False(t, s.HasPCS, "no PCS pass was run")
	require.Zero(t, s.Queries, "FRI counts must stay zero without an opening proof")
	require.Zero(t, s.LeafSlots, "FRI counts must stay zero without an opening proof")
	require.Positive(t, s.Total, "the round messages still occupy the image")
}
