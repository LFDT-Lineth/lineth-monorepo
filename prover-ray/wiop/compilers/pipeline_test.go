package compilers_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/global"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/grandproduct"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/localvanishing"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/logderivativesum"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/lookuptologderivsum"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/rangecheck"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/wioptest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// compileFullPipeline runs every wiop compilation pass in the canonical
// order so that each pass can consume the previous one's output:
//
//  1. rangecheck:           RangeCheck → Inclusion TableRelation
//  2. lookuptologderivsum:  Inclusion → LogDerivativeSum
//  3. logderivativesum:     LogDerivativeSum → recurrence Vanishings + endpoint openings
//  4. localvanishing:       scalar Vanishings → multi-valued Vanishings via the Lagrange lift
//  5. global:               multi-valued Vanishings → quotient shares + LagrangeEval claims
//
// Each pass is a no-op when its input queries are absent, so this ordering
// is safe to apply uniformly to every wioptest scenario regardless of which
// pass the scenario is primarily exercising.
func compileFullPipeline(sys *wiop.System) {
	rangecheck.Compile(sys)
	lookuptologderivsum.Compile(sys)
	grandproduct.Compile(sys)
	logderivativesum.Compile(sys)
	localvanishing.Compile(sys)
	global.Compile(sys)
}

// These tests drive every scenario through the full
// range → lookup → logderivative → local → global pipeline using the explicit
// prover/verifier split: sys.Prove(assign) produces a strict, public-only
// [wiop.Proof], and sys.Verify(proof, pub) re-checks it without access to the oracle
// witness columns. Because the Proof carries only public columns, cells, and
// coins, these tests fail loudly if any verifier action reads an oracle or
// internal column.

// TestFullPipeline_VanishingScenarios runs the full pipeline on every
// [wioptest.VanishingScenarios] fixture. These scenarios start with
// multi-valued [wiop.Vanishing] constraints; the local-vanishing pass is a
// no-op and the global pass discharges them through the quotient argument.
func TestFullPipeline_VanishingScenarios(t *testing.T) {
	for _, build := range wioptest.VanishingScenarios() {
		sc := build()
		t.Run(sc.Name, func(t *testing.T) {
			compileFullPipeline(sc.Sys)
			proof, pub := sc.Sys.Prove(sc.AssignHonest)
			require.NoError(t, sc.Sys.Verify(proof, pub),
				"full pipeline must accept an honest witness")
		})

		// Each soundness case rebuilds a fresh scenario so it doesn't share
		// compilation state with the completeness case above.
		t.Run(sc.Name+"/Soundness", func(t *testing.T) {
			sc := build()
			compileFullPipeline(sc.Sys)
			proof, pub := sc.Sys.Prove(sc.AssignInvalid)
			assert.Error(t, sc.Sys.Verify(proof, pub),
				"full pipeline must reject an invalid witness")
		})
	}
}

// TestFullPipeline_LocalVanishingScenarios runs the full pipeline on every
// [wioptest.LocalVanishingScenarios] fixture. The local-vanishing pass
// lifts each scalar [wiop.Vanishing] into a multi-valued one; the global
// pass then discharges it.
func TestFullPipeline_LocalVanishingScenarios(t *testing.T) {
	for _, build := range wioptest.LocalVanishingScenarios() {
		sc := build()
		t.Run(sc.Name, func(t *testing.T) {
			compileFullPipeline(sc.Sys)
			proof, pub := sc.Sys.Prove(sc.AssignHonest)
			require.NoError(t, sc.Sys.Verify(proof, pub),
				"full pipeline must accept an honest witness")
		})

		t.Run(sc.Name+"/Soundness", func(t *testing.T) {
			sc := build()
			compileFullPipeline(sc.Sys)
			proof, pub := sc.Sys.Prove(sc.AssignInvalid)
			assert.Error(t, sc.Sys.Verify(proof, pub),
				"full pipeline must reject an invalid witness")
		})
	}
}

// TestFullPipeline_LogDerivativeSumScenarios runs the full pipeline on
// every [wioptest.LogDerivativeSumCompilerScenarios] fixture. The
// log-derivative pass emits one recurrence Vanishing per Z column (plus
// LocalOpenings for the endpoints), and the global pass then discharges the
// recurrence.
func TestFullPipeline_LogDerivativeSumScenarios(t *testing.T) {
	for _, build := range wioptest.LogDerivativeSumCompilerScenarios() {
		sc := build()
		t.Run(sc.Name, func(t *testing.T) {
			compileFullPipeline(sc.Sys)
			proof, pub := sc.Sys.Prove(sc.AssignWitness)
			require.NoError(t, sc.Sys.Verify(proof, pub),
				"full pipeline must accept an honest witness")
		})
	}
}

// TestFullPipeline_LookupScenarios runs the full pipeline on every
// [wioptest.LookupScenarios] fixture. The pipeline reduces each Inclusion
// through the log-derivative + recurrence chain into quotient queries that
// the global pass discharges.
func TestFullPipeline_LookupScenarios(t *testing.T) {
	for _, build := range wioptest.LookupScenarios() {
		sc := build()
		t.Run(sc.Name, func(t *testing.T) {
			compileFullPipeline(sc.Sys)
			proof, pub := sc.Sys.Prove(sc.AssignWitness)
			require.NoError(t, sc.Sys.Verify(proof, pub),
				"full pipeline must accept an honest witness")
		})
	}
}

// TestFullPipeline_PermutationScenarios runs the full pipeline on every
// [wioptest.PermutationScenarios] fixture. The grandproduct pass reduces each
// permutation into a grand product and then into running-product Z columns
// (recurrence + local + endpoint openings) that the local-vanishing and global
// passes discharge; the honest witness must verify and the invalid witness
// (A and B not equal as multisets) must be rejected.
func TestFullPipeline_PermutationScenarios(t *testing.T) {
	for _, build := range wioptest.PermutationScenarios() {
		sc := build()
		t.Run(sc.Name, func(t *testing.T) {
			compileFullPipeline(sc.Sys)
			proof := sc.Sys.Prove(sc.AssignHonest)
			require.NoError(t, sc.Sys.Verify(proof),
				"full pipeline must accept an honest permutation witness")
		})

		t.Run(sc.Name+"/Soundness", func(t *testing.T) {
			sc := build()
			compileFullPipeline(sc.Sys)
			proof := sc.Sys.Prove(sc.AssignInvalid)
			assert.Error(t, sc.Sys.Verify(proof),
				"full pipeline must reject a non-permutation witness")
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
	compileFullPipeline(sc.Sys)

	proof := sc.Sys.Prove(sc.AssignWitness)
	require.NoError(t, sc.Sys.Verify(proof),
		"sanity: honest log-derivative proof must verify")

	require.NotEmpty(t, sc.Sys.LogDerivativeSums,
		"scenario must contain a LogDerivativeSum after compilation")
	result := sc.Sys.LogDerivativeSums[0].Result
	proof.Cells[result.Context.ID] = field.ElemFromBase(field.NewFromString("123456"))

	assert.Error(t, sc.Sys.Verify(proof),
		"a tampered LogDerivativeSum Result must be rejected by the full pipeline")
}

// TestFullPipeline_LogDerivativeSumTamperedZ is the running-sum analogue of
// TestFullPipeline_PermutationTamperedZ: a Z column corrupted at an interior
// row — endpoint left intact — is rejected only because the full pipeline
// discharges the running-sum recurrence.
//
// The logderivativesum pass's own final-sum verifier action reads the
// endpoint-opening cells and the Result cell, all untouched by an interior
// corruption, so it still accepts. It is the recurrence Vanishing — lifted by
// local-vanishing and discharged by the global quotient — that pins every
// interior row of Z, so only the assembled pipeline catches it.
func TestFullPipeline_LogDerivativeSumTamperedZ(t *testing.T) {
	sc := wioptest.NewLDSSingleFractionAllOnesScenario()

	before := snapshotModuleColumns(sc.Sys)
	compileFullPipeline(sc.Sys)
	zCols := newExtensionColumns(sc.Sys, before)
	require.NotEmpty(t, zCols, "logderivativesum must add Z columns")

	proof := sc.Sys.Prove(sc.AssignWitness)
	require.NoError(t, sc.Sys.Verify(proof),
		"honest log-derivative witness must verify through the full pipeline")

	z := zCols[0]
	cv := proof.Columns[z.Context.ID]
	require.NotNil(t, cv, "the Z column must be captured in the proof")
	ext := cv.Plain.AsExt()
	require.GreaterOrEqual(t, len(ext), 3, "need at least one interior row to tamper")
	one := field.OneExt()
	ext[1].Add(&ext[1], &one) // interior row 1; endpoint (last row) left intact

	assert.Error(t, sc.Sys.Verify(proof),
		"the full pipeline must reject a Z column whose interior recurrence is violated")
}

// TestFullPipeline_PermutationTamperedZ shows that a Z column corrupted at an
// interior row — but with its endpoint left intact — is rejected only because
// the full pipeline discharges the running-product recurrence.
//
// The grandproduct pass's own verifier actions cannot see this tamper: both
// CheckResultIsOne (reads the Result cell) and FinalProductCheck (reads the
// endpoint-opening cells) operate on values that the interior corruption leaves
// untouched. It is the recurrence Vanishing — lifted by local-vanishing and
// discharged by the global quotient — that pins every interior row of Z, so
// only the assembled pipeline catches the corruption.
func TestFullPipeline_PermutationTamperedZ(t *testing.T) {
	sc := wioptest.NewPermutationSingleColumnScenario()

	before := snapshotModuleColumns(sc.Sys)
	compileFullPipeline(sc.Sys)
	zCols := newExtensionColumns(sc.Sys, before)
	require.NotEmpty(t, zCols, "grandproduct must add Z columns")

	// Sanity: the honest proof verifies.
	proof := sc.Sys.Prove(sc.AssignHonest)
	require.NoError(t, sc.Sys.Verify(proof),
		"honest permutation witness must verify through the full pipeline")

	// Corrupt one interior row of a Z column, leaving its endpoint (last row)
	// untouched. The endpoint openings and Result cell are unchanged, so the
	// grandproduct-local verifier actions still pass; the recurrence does not.
	z := zCols[0]
	cv := proof.Columns[z.Context.ID]
	require.NotNil(t, cv, "the Z column must be captured in the proof")
	ext := cv.Plain.AsExt()
	require.GreaterOrEqual(t, len(ext), 3, "need at least one interior row to tamper")
	one := field.OneExt()
	ext[1].Add(&ext[1], &one) // interior row 1; endpoint (last row) left intact

	assert.Error(t, sc.Sys.Verify(proof),
		"the full pipeline must reject a Z column whose interior recurrence is violated")
}

// TestFullPipeline_RangeCheckScenarios runs the full pipeline on every
// [wioptest.RangeCheckCompilerScenarios] fixture. Every step contributes:
// rangecheck → lookup → log-derivative → recurrence vanishings → global
// quotient.
func TestFullPipeline_RangeCheckScenarios(t *testing.T) {
	for _, build := range wioptest.RangeCheckCompilerScenarios() {
		sc := build()
		t.Run(sc.Name, func(t *testing.T) {
			compileFullPipeline(sc.Sys)
			proof, pub := sc.Sys.Prove(sc.AssignWitness)
			require.NoError(t, sc.Sys.Verify(proof, pub),
				"full pipeline must accept an honest witness")
		})
	}
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
