package grandproduct_test

import (
	"errors"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/wioptest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
			require.NoError(t, compilers.CompileFull(sc.Sys, wioptest.TestingCompileOptions()...))
			proof, pub := sc.Sys.Prove(sc.AssignHonest)
			require.NoError(t, sc.Sys.Verify(proof, pub),
				"full pipeline must accept an honest permutation witness")
		})

		t.Run(sc.Name+"/Soundness", func(t *testing.T) {
			sc := build()
			require.NoError(t, compilers.CompileFull(sc.Sys, wioptest.TestingCompileOptions()...))
			proof, pub := sc.Sys.Prove(sc.AssignInvalid)
			assert.Error(t, sc.Sys.Verify(proof, pub),
				"full pipeline must reject a non-permutation witness")
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
//
// The pre-PCS snapshot identifies the extension Z columns; a PCS pre-hook
// then registers a prover-side tamper on an interior Z row before the PCS
// pass commits it. The endpoint (last row) and Result cell are left intact,
// so the grandproduct-local verifier actions still pass; only the recurrence
// — discharged by the global quotient — rejects the tampered interior row.
func TestFullPipeline_PermutationTamperedZ(t *testing.T) {
	sc := wioptest.NewPermutationSingleColumnScenario()

	before := snapshotModuleColumns(sc.Sys)
	options := append(wioptest.TestingCompileOptions(),
		compilers.WithPreHook(compilers.PCS, func(sys *wiop.System) error {
			zCols := newExtensionColumns(sys, before)
			if len(zCols) == 0 {
				return errors.New("grandproduct must add Z columns")
			}
			wioptest.Mutator{Column: zCols[0], Row: 1, Tweak: wioptest.AddOne}.Compile(sys)
			return nil
		}),
	)
	require.NoError(t, compilers.CompileFull(sc.Sys, options...),
		"full pipeline with Z tamper pre-hook must compile")

	proof, pub := sc.Sys.Prove(sc.AssignHonest)
	assert.Error(t, sc.Sys.Verify(proof, pub),
		"the full pipeline must reject a Z column whose interior recurrence is violated")
}
