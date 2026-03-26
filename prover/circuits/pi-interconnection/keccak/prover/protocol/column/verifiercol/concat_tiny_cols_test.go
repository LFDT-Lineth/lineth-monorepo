package verifiercol_test

import (
	"fmt"
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/sirupsen/logrus"
)

func TestConcatTinyColRange(t *testing.T) {

	// mute the logs
	logrus.SetLevel(logrus.FatalLevel)

	testcases := []struct {
		NumCols, Split int
		Shift          int
	}{
		{NumCols: 16, Split: 8, Shift: 1},
		{NumCols: 16, Split: 8, Shift: 0},
		{NumCols: 16, Split: 16, Shift: 1},
		{NumCols: 16, Split: 16, Shift: 0},
		{NumCols: 16, Split: 32, Shift: 1},
		{NumCols: 16, Split: 32, Shift: 0},
	}

	for _, tc := range testcases {

		t.Run(
			fmt.Sprintf("testcase-%++v", tc),
			func(subT *testing.T) {

				var (
					QUERY ifaces.QueryID = "QUERY"
				)

				cols := make([]ifaces.Column, tc.NumCols)
				define := func(b *wizard.Builder) {

					// Registers the tiny columns
					for i := range cols {
						cols[i] = b.CompiledIOP.InsertColumn(
							0,
							ifaces.ColIDf("P%v", i),
							1,            // size of the column (must be 1 because of CTC)
							column.Proof, // must be a public column
						)
					}

					ctc := verifiercol.NewConcatTinyColumns(
						b.CompiledIOP,
						tc.NumCols,
						field.Element{},
						cols...)

					// We apply a shift to force the application of the
					// naturalization compiler over our column
					if tc.Shift > 0 {
						ctc = column.Shift(ctc, tc.Shift)
					}

					b.Range(QUERY, ctc, tc.NumCols)
				}

				prove := func(run *wizard.ProverRuntime) {
					for i := range cols {
						// the assignment value must respect the range check constraint
						run.AssignColumn(cols[i].GetColID(), smartvectors.ForTest(i))
					}
				}

				// Compile with the full suite
				compiled := wizard.Compile(define,
					dummy.Compile,
				)

				proof := wizard.Prove(compiled, prove)

				if err := wizard.Verify(compiled, proof); err != nil {
					subT.Logf("verifier failed : %v", err)
					subT.FailNow()
				}

			},
		)
	}

}

func TestConcatTinyColWithPaddingRange(t *testing.T) {

	// mute the logs
	logrus.SetLevel(logrus.FatalLevel)

	testcases := []struct {
		NumCols, PaddedSize, Split int
		Shift                      int
	}{
		{NumCols: 12, PaddedSize: 16, Split: 16, Shift: 1},
		{NumCols: 12, PaddedSize: 16, Split: 16, Shift: 0},
		{NumCols: 12, PaddedSize: 16, Split: 32, Shift: 1},
		{NumCols: 12, PaddedSize: 16, Split: 32, Shift: 0},
	}

	for _, tc := range testcases {

		t.Run(
			fmt.Sprintf("testcase-%++v", tc),
			func(subT *testing.T) {

				var (
					QUERY ifaces.QueryID = "QUERY"
				)

				cols := make([]ifaces.Column, tc.NumCols)

				define := func(b *wizard.Builder) {

					for i := range cols {
						cols[i] = b.CompiledIOP.InsertColumn(
							0,
							ifaces.ColIDf("P%v", i),
							1,            // size of the column (must be 1 because of CTC)
							column.Proof, // must be a public column
						)
					}

					ctc := verifiercol.NewConcatTinyColumns(
						b.CompiledIOP,
						tc.PaddedSize,
						field.Element{},
						cols...)

					// We apply a shift to force the application of the
					// naturalization compiler over our column
					if tc.Shift > 0 {
						ctc = column.Shift(ctc, tc.Shift)
					}

					b.Range(QUERY, ctc, tc.NumCols)
				}

				prove := func(run *wizard.ProverRuntime) {
					for i := range cols {
						// the assignment value must respect the range check constraint
						run.AssignColumn(cols[i].GetColID(), smartvectors.ForTest(i))
					}
				}

				// Compile with the full suite
				compiled := wizard.Compile(define,
					compiler.Arcane(
						compiler.WithTargetColSize(tc.Split),
						compiler.WithStitcherMinSize(tc.Split),
						compiler.WithoutMpts(),
					),
					dummy.Compile,
				)

				proof := wizard.Prove(compiled, prove)

				if err := wizard.Verify(compiled, proof); err != nil {
					subT.Logf("verifier failed : %v", err)
					subT.FailNow()
				}

			},
		)
	}
}
