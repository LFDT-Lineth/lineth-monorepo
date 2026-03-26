package common

import (
	"fmt"
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
)

func TestHashing(t *testing.T) {

	numRow := 4

	testCase := [][]smartvectors.SmartVector{
		{
			smartvectors.NewConstant(field.Zero(), numRow),
			smartvectors.NewConstant(field.One(), numRow),
			smartvectors.NewConstant(field.NewElement(2), numRow),
		},
	}

	for i, tc := range testCase {

		t.Run(fmt.Sprintf("testcase-%v", i), func(t *testing.T) {

			var pa wizard.ProverAction

			define := func(b *wizard.Builder) {
				cols := [][NbElemPerHash]ifaces.Column{
					{
						b.RegisterCommit("A1", numRow),
						b.RegisterCommit("B1", numRow),
						b.RegisterCommit("C1", numRow),
						b.RegisterCommit("D1", numRow),
						b.RegisterCommit("E1", numRow),
						b.RegisterCommit("F1", numRow),
						b.RegisterCommit("G1", numRow),
						b.RegisterCommit("H1", numRow),
					},
					{
						b.RegisterCommit("A2", numRow),
						b.RegisterCommit("B2", numRow),
						b.RegisterCommit("C2", numRow),
						b.RegisterCommit("D2", numRow),
						b.RegisterCommit("E2", numRow),
						b.RegisterCommit("F2", numRow),
						b.RegisterCommit("G2", numRow),
						b.RegisterCommit("H2", numRow),
					},
					{
						b.RegisterCommit("A3", numRow),
						b.RegisterCommit("B3", numRow),
						b.RegisterCommit("C3", numRow),
						b.RegisterCommit("D3", numRow),
						b.RegisterCommit("E3", numRow),
						b.RegisterCommit("F3", numRow),
						b.RegisterCommit("G3", numRow),
						b.RegisterCommit("H3", numRow),
					},
				}

				_, pa = HashOf(b.CompiledIOP, cols)
			}

			prove := func(run *wizard.ProverRuntime) {
				run.AssignColumn("A1", tc[0])
				run.AssignColumn("B1", tc[0])
				run.AssignColumn("C1", tc[0])
				run.AssignColumn("D1", tc[0])
				run.AssignColumn("E1", tc[0])
				run.AssignColumn("F1", tc[0])
				run.AssignColumn("G1", tc[0])
				run.AssignColumn("H1", tc[0])

				run.AssignColumn("A2", tc[1])
				run.AssignColumn("B2", tc[1])
				run.AssignColumn("C2", tc[1])
				run.AssignColumn("D2", tc[1])
				run.AssignColumn("E2", tc[1])
				run.AssignColumn("F2", tc[1])
				run.AssignColumn("G2", tc[1])
				run.AssignColumn("H2", tc[1])

				run.AssignColumn("A3", tc[2])
				run.AssignColumn("B3", tc[2])
				run.AssignColumn("C3", tc[2])
				run.AssignColumn("D3", tc[2])
				run.AssignColumn("E3", tc[2])
				run.AssignColumn("F3", tc[2])
				run.AssignColumn("G3", tc[2])
				run.AssignColumn("H3", tc[2])

				pa.Run(run)
			}

			comp := wizard.Compile(define, dummy.Compile)
			proof := wizard.Prove(comp, prove)
			err := wizard.Verify(comp, proof)

			if err != nil {
				t.Fatalf("verification failed: %v", err)
			}
		})
	}

}
