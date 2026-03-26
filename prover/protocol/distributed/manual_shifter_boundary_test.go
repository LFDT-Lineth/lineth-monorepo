package distributed

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
)

// TestManualShifterMultipleOffsets verifies that CompileManualShifter handles
// multiple column.Shift calls on the same base column with different offsets.
// Before the naming fix, these would collide because the generated column IDs
// did not include the offset.
func TestManualShifterMultipleOffsets(t *testing.T) {
	define := func(builder *wizard.Builder) {
		a := builder.RegisterCommit("A_multi", 16)
		b := builder.RegisterCommit("B_multi", 16)
		c := builder.RegisterCommit("C_multi", 16)

		// Two different shifts of the same column used in permutations.
		// Each creates a ManuallyShifted committed column. Before the fix,
		// both would get the same column ID, causing a panic.
		a_plus1 := column.Shift(a, 1)
		a_minus1 := column.Shift(a, -1)

		// Two permutations using different offsets of the same base column
		builder.Permutation("PERM_multi_plus1", []ifaces.Column{a_plus1}, []ifaces.Column{b})
		builder.Permutation("PERM_multi_minus1", []ifaces.Column{a_minus1}, []ifaces.Column{c})
	}

	prove := func(run *wizard.ProverRuntime) {
		// a = [0, 1, 2, ..., 15]
		// ManuallyShifted(a, +1) = [1, 2, ..., 15, 0] (zero-padded at end)
		// ManuallyShifted(a, -1) = [0, 0, 1, ..., 14] (zero-padded at front)
		vals := make([]field.Element, 16)
		for i := range vals {
			vals[i] = field.NewElement(uint64(i))
		}
		run.AssignColumn("A_multi", smartvectors.NewRegular(vals))

		// b must be a permutation of [1, 2, ..., 15, 0]
		run.AssignColumn("B_multi", smartvectors.NewRegular(vals))

		// c must be a permutation of [0, 0, 1, ..., 14]
		cVals := make([]field.Element, 16)
		cVals[0] = field.Zero()
		for i := 1; i < 16; i++ {
			cVals[i] = field.NewElement(uint64(i - 1))
		}
		run.AssignColumn("C_multi", smartvectors.NewRegular(cVals))
	}

	comp := wizard.Compile(define, CompileManualShifter)
	if err := auditInitialWizard(comp); err != nil {
		t.Fatalf("audit failed: %v", err.Error())
	}
	dummy.Compile(comp)

	proof := wizard.Prove(comp, prove)
	if err := wizard.Verify(comp, proof); err != nil {
		t.Fatalf("verifier failed: %v", err.Error())
	}
}

// TestManualShifterZeroPaddedColumn verifies that ManuallyShifted columns
// created by CompileManualShifter produce zero-padded assignments (not cyclic
// rotations). Cyclic rotation at boundaries corrupts the distributed
// system's segment boundary detection.
func TestManualShifterZeroPaddedColumn(t *testing.T) {
	define := func(builder *wizard.Builder) {
		a := builder.RegisterCommit("A_zp", 8)
		b := builder.RegisterCommit("B_zp", 8)

		// Shift +1: ManuallyShifted = [20,30,40,50,60,70,80,0]
		// b must be a permutation of this
		a_plus1 := column.Shift(a, 1)
		builder.Permutation("PERM_zp", []ifaces.Column{a_plus1}, []ifaces.Column{b})
	}

	comp := wizard.Compile(define, CompileManualShifter)
	if err := auditInitialWizard(comp); err != nil {
		t.Fatalf("audit failed: %v", err.Error())
	}
	dummy.Compile(comp)

	// a = [10, 20, 30, 40, 50, 60, 70, 80]
	// ManuallyShifted(a, +1) = [20, 30, 40, 50, 60, 70, 80, 0]
	// b must be a permutation: [0, 20, 30, 40, 50, 60, 70, 80]
	proof := wizard.Prove(comp, func(run *wizard.ProverRuntime) {
		run.AssignColumn("A_zp", smartvectors.ForTest(10, 20, 30, 40, 50, 60, 70, 80))
		run.AssignColumn("B_zp", smartvectors.ForTest(0, 20, 30, 40, 50, 60, 70, 80))
	})

	if err := wizard.Verify(comp, proof); err != nil {
		t.Fatalf("verifier failed: %v", err.Error())
	}
}

// TestManualShifterNegativeOffset verifies CompileManualShifter for negative
// offsets (backward shift). The ManuallyShifted column should have zeros
// prepended and the last elements dropped.
func TestManualShifterNegativeOffset(t *testing.T) {
	define := func(builder *wizard.Builder) {
		a := builder.RegisterCommit("A_neg", 8)
		b := builder.RegisterCommit("B_neg", 8)

		a_minus1 := column.Shift(a, -1)
		builder.Permutation("PERM_neg", []ifaces.Column{a_minus1}, []ifaces.Column{b})
	}

	prove := func(run *wizard.ProverRuntime) {
		// a = [10, 20, 30, 40, 50, 60, 70, 80]
		// ManuallyShifted(a, -1) = [0, 10, 20, 30, 40, 50, 60, 70]
		// b must be a permutation of this
		run.AssignColumn("A_neg", smartvectors.ForTest(10, 20, 30, 40, 50, 60, 70, 80))
		run.AssignColumn("B_neg", smartvectors.ForTest(0, 10, 20, 30, 40, 50, 60, 70))
	}

	comp := wizard.Compile(define, CompileManualShifter)
	if err := auditInitialWizard(comp); err != nil {
		t.Fatalf("audit failed: %v", err.Error())
	}
	dummy.Compile(comp)

	proof := wizard.Prove(comp, prove)
	if err := wizard.Verify(comp, proof); err != nil {
		t.Fatalf("verifier failed: %v", err.Error())
	}
}

// TestManualShifterProjectionWithShift verifies that CompileManualShifter
// correctly handles column.Shift in projection queries. Projections are
// compiled into Horner evaluations and require careful boundary handling.
func TestManualShifterProjectionWithShift(t *testing.T) {
	define := func(builder *wizard.Builder) {
		a := builder.RegisterCommit("A_proj_shift", 8)
		b := builder.RegisterCommit("B_proj_shift", 16)
		filterA := builder.RegisterCommit("FA_shift", 8)
		filterB := builder.RegisterCommit("FB_shift", 16)

		a_shifted := column.Shift(a, 1)

		builder.InsertProjection("PROJ_shift",
			query.ProjectionInput{
				ColumnA: []ifaces.Column{a_shifted},
				ColumnB: []ifaces.Column{b},
				FilterA: filterA,
				FilterB: filterB,
			},
		)
	}

	prove := func(run *wizard.ProverRuntime) {
		// a = [1, 2, 3, 4, 5, 6, 7, 8]
		// a_shifted(+1) = [2, 3, 4, 5, 6, 7, 8, 0] with zero padding
		run.AssignColumn("A_proj_shift", smartvectors.ForTest(1, 2, 3, 4, 5, 6, 7, 8))

		// b must contain all filtered a_shifted values
		bVals := make([]field.Element, 16)
		// First 7 positions have the shifted values (filterA selects first 7)
		for i := 0; i < 7; i++ {
			bVals[i] = field.NewElement(uint64(i + 2)) // a_shifted = [2,3,4,5,6,7,8]
		}
		run.AssignColumn("B_proj_shift", smartvectors.NewRegular(bVals))

		// filterA: select first 7 rows (skip the zero-padded last position)
		filterAVals := make([]field.Element, 8)
		for i := 0; i < 7; i++ {
			filterAVals[i] = field.One()
		}
		run.AssignColumn("FA_shift", smartvectors.NewRegular(filterAVals))

		// filterB: select first 7 rows
		filterBVals := make([]field.Element, 16)
		for i := 0; i < 7; i++ {
			filterBVals[i] = field.One()
		}
		run.AssignColumn("FB_shift", smartvectors.NewRegular(filterBVals))
	}

	comp := wizard.Compile(define, CompileManualShifter)
	if err := auditInitialWizard(comp); err != nil {
		t.Fatalf("audit failed: %v", err.Error())
	}
	dummy.Compile(comp)

	proof := wizard.Prove(comp, prove)
	if err := wizard.Verify(comp, proof); err != nil {
		t.Fatalf("verifier failed: %v", err.Error())
	}
}
