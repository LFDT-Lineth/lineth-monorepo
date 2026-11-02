package logderivativesum_test

import (
	"errors"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/wioptest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFullPipeline_LogDerivativeSumScenarios runs the full pipeline on
// every [wioptest.LogDerivativeSumCompilerScenarios] fixture. The
// log-derivative pass emits one recurrence Vanishing per Z column (plus
// LocalOpenings for the endpoints), and the global pass then discharges the
// recurrence.
func TestFullPipeline_LogDerivativeSumScenarios(t *testing.T) {
	for _, build := range wioptest.LogDerivativeSumCompilerScenarios() {
		sc := build()
		t.Run(sc.Name, func(t *testing.T) {
			require.NoError(t, compilers.CompileFull(sc.Sys, wioptest.TestingCompileOptions()...))
			proof, pub := sc.Sys.Prove(sc.AssignWitness)
			require.NoError(t, sc.Sys.Verify(proof, pub),
				"full pipeline must accept an honest witness")
		})
	}
}

// TestFullPipeline_LogDerivativeSumTamperedResult is the running-sum
// soundness companion to the completeness-only
// TestFullPipeline_LogDerivativeSumScenarios: an honest proof whose claimed
// LogDerivativeSum Result cell is then corrupted must be rejected.
//
// A bare LogDerivativeSum self-computes its Result from the witness, so no
// round-0 witness alone is "invalid"; the failure mode is a wrong claimed
// Result. The Result lives in a round after the witness, so it is corrupted in
// the produced proof rather than through the round-0 assignment hook. The
// logderivativesum pass's final-sum verifier action then rejects the proof.
func TestFullPipeline_LogDerivativeSumTamperedResult(t *testing.T) {
	sc := wioptest.NewLDSSingleFractionAllOnesScenario()
	require.NoError(t, compilers.CompileFull(sc.Sys, wioptest.TestingCompileOptions()...))

	proof, pub := sc.Sys.Prove(sc.AssignWitness)
	require.NoError(t, sc.Sys.Verify(proof, pub),
		"sanity: honest log-derivative proof must verify")

	require.NotEmpty(t, sc.Sys.LogDerivativeSums,
		"scenario must contain a LogDerivativeSum after compilation")
	result := sc.Sys.LogDerivativeSums[0].Result
	proof.Cells[result.Context.ID] = field.ElemFromBase(field.NewFromString("123456"))

	assert.Error(t, sc.Sys.Verify(proof, pub),
		"a tampered LogDerivativeSum Result must be rejected by the full pipeline")
}

// snapshotModuleColumns records, per module, the set of columns present before
// a compiler pass runs. Pair with [newExtensionColumns] to identify the
// extension (Z) columns a pass adds.
func snapshotModuleColumns(sys *wiop.System) map[*wiop.Module]map[*wiop.Column]struct{} {
	before := make(map[*wiop.Module]map[*wiop.Column]struct{}, len(sys.Modules))
	for _, m := range sys.Modules {
		seen := make(map[*wiop.Column]struct{}, len(m.Columns))
		for _, c := range m.Columns {
			seen[c] = struct{}{}
		}
		before[m] = seen
	}
	return before
}

// newExtensionColumns returns the extension columns added to any module since
// the snapshot. These are the running-sum / running-product Z columns emitted
// by the logderivativesum and grandproduct passes.
func newExtensionColumns(sys *wiop.System, before map[*wiop.Module]map[*wiop.Column]struct{}) []*wiop.Column {
	var zCols []*wiop.Column
	for _, m := range sys.Modules {
		for _, c := range m.Columns {
			if _, existed := before[m][c]; existed {
				continue
			}
			if c.IsExtension {
				zCols = append(zCols, c)
			}
		}
	}
	return zCols
}

// TestFullPipeline_LogDerivativeSumTamperedZ is the running-sum analogue of
// grandproduct's TestFullPipeline_PermutationTamperedZ: a Z column corrupted
// at an interior row — endpoint left intact — is rejected only because the
// full pipeline discharges the running-sum recurrence.
//
// The logderivativesum pass's own final-sum verifier action reads the
// endpoint-opening cells and the Result cell, all untouched by an interior
// corruption, so it still accepts. It is the recurrence Vanishing — lifted by
// local-vanishing and discharged by the global quotient — that pins every
// interior row of Z, so only the assembled pipeline catches it.
//
// The pre-PCS snapshot identifies the extension Z columns; a PCS pre-hook
// then registers a prover-side tamper on an interior Z row before the PCS
// pass commits it. The tamper runs during Prove after Z is assigned but
// before PCS commits it, so the FRI commitment and opening bind the tampered
// column: the PCS-local checks pass and only the recurrence — discharged by
// the global quotient — rejects.
func TestFullPipeline_LogDerivativeSumTamperedZ(t *testing.T) {
	sc := wioptest.NewLDSSingleFractionAllOnesScenario()

	before := snapshotModuleColumns(sc.Sys)
	options := append(wioptest.TestingCompileOptions(),
		compilers.WithPreHook(compilers.PCS, func(sys *wiop.System) error {
			zCols := newExtensionColumns(sys, before)
			if len(zCols) == 0 {
				return errors.New("logderivativesum must add Z columns")
			}
			wioptest.Mutator{Column: zCols[0], Row: 1, Tweak: wioptest.AddOne}.Compile(sys)
			return nil
		}),
	)
	require.NoError(t, compilers.CompileFull(sc.Sys, options...),
		"full pipeline with Z tamper pre-hook must compile")

	proof, pub := sc.Sys.Prove(sc.AssignWitness)
	assert.Error(t, sc.Sys.Verify(proof, pub),
		"the full pipeline must reject a Z column whose interior recurrence is violated")
}
