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

func TestManualShifter(t *testing.T) {
	testCases := []struct {
		Define     func(*wizard.Builder)
		Prove      func(*wizard.ProverRuntime)
		Title      string
		ShouldPass bool
	}{
		{
			Define: func(builder *wizard.Builder) {
				a := builder.RegisterCommit("A", 16)
				b := builder.RegisterCommit("B", 16)
				b_1 := column.Shift(b, 1)
				builder.Permutation("PERM", []ifaces.Column{a}, []ifaces.Column{b_1})
			},

			Prove: func(run *wizard.ProverRuntime) {
				run.AssignColumn("A", smartvectors.ForTest(0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15))
				run.AssignColumn("B", smartvectors.ForTest(0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15))
			},

			Title:      "permutation",
			ShouldPass: true,
		},
		{
			Define: func(builder *wizard.Builder) {
				// Create columns for simple inclusion test (without shifted columns)
				a := builder.RegisterCommit("A_incl", 16)
				b := builder.RegisterCommit("B_incl", 16)
				b_1 := column.Shift(b, 1)
				// Simple inclusion query without shifts
				builder.Inclusion("INCL", []ifaces.Column{a}, []ifaces.Column{b_1})
			},

			Prove: func(run *wizard.ProverRuntime) {
				// A contains values that should be in B
				run.AssignColumn("A_incl", smartvectors.ForTest(0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15))
				// B contains the same values
				run.AssignColumn("B_incl", smartvectors.ForTest(0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15))
			},

			Title:      "inclusion",
			ShouldPass: true,
		},
		{
			Define: func(builder *wizard.Builder) {
				// Create columns for simple projection test
				a := builder.RegisterCommit("A_proj", 8)
				b := builder.RegisterCommit("B_proj", 16)
				// Create filter columns (no shifts)
				filterA := builder.RegisterCommit("FilterA_proj", 8)
				filterAShifted := column.Shift(filterA, 1)
				filterB := builder.RegisterCommit("FilterB_proj", 16)

				// Projection query: filtered values from A should match filtered values from B
				builder.InsertProjection("PROJ",
					query.ProjectionInput{
						ColumnA: []ifaces.Column{a},
						ColumnB: []ifaces.Column{b},
						FilterA: filterAShifted,
						FilterB: filterB,
					},
				)
			},

			Prove: func(run *wizard.ProverRuntime) {
				// A has values [1, 2, 3, 4, 5, 6, 7, 8]
				run.AssignColumn("A_proj", smartvectors.ForTest(1, 2, 3, 4, 5, 6, 7, 8))
				// B has values [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16]
				run.AssignColumn("B_proj", smartvectors.ForTest(1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16))
				// FilterA selects all rows: [1, 1, 1, 1, 1, 1, 1, 1]
				// After ManuallyShifted(filterA, +1) with zero-padding, the
				// effective filter becomes [1, 1, 1, 1, 1, 1, 1, 0] — 7 rows.
				filterAVals := make([]field.Element, 8)
				for i := 0; i < 8; i++ {
					filterAVals[i] = field.One()
				}
				run.AssignColumn("FilterA_proj", smartvectors.NewRegular(filterAVals))
				// FilterB must select the same number of rows (7) to match
				filterBVals := make([]field.Element, 16)
				for i := 0; i < 7; i++ {
					filterBVals[i] = field.One()
				}
				run.AssignColumn("FilterB_proj", smartvectors.NewRegular(filterBVals))
			},

			Title:      "projection",
			ShouldPass: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Title, func(t *testing.T) {
			comp := wizard.Compile(testCase.Define, CompileManualShifter)
			err := auditInitialWizard(comp)
			if err != nil {
				t.Fatalf("audit failed: %v", err.Error())
			}
			dummy.Compile(comp)
			proof := wizard.Prove(comp, testCase.Prove)
			if err := wizard.Verify(comp, proof); err != nil && testCase.ShouldPass {
				t.Fatalf("verifier did not pass: %v", err.Error())
			}
			if err := wizard.Verify(comp, proof); err == nil && !testCase.ShouldPass {
				t.Fatalf("verifier is passing for a false claim")
			}
		})
	}
}
