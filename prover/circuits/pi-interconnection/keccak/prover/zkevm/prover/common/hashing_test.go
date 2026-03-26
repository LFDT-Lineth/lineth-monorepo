package common

import (
	"fmt"
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
)

func TestHashing(t *testing.T) {

	testCase := [][]smartvectors.SmartVector{
		{
			smartvectors.NewConstant(field.Zero(), 4),
			smartvectors.NewConstant(field.One(), 4),
			smartvectors.NewConstant(field.NewElement(2), 4),
		},
	}

	for i, tc := range testCase {

		t.Run(fmt.Sprintf("testcase-%v", i), func(t *testing.T) {

			var pa wizard.ProverAction

			define := func(b *wizard.Builder) {
				cols := []ifaces.Column{
					b.RegisterCommit("A1", 4),
					b.RegisterCommit("A2", 4),
					b.RegisterCommit("A3", 4),
				}

				_, pa = HashOf(b.CompiledIOP, cols)
			}

			prove := func(run *wizard.ProverRuntime) {
				run.AssignColumn("A1", tc[0])
				run.AssignColumn("A2", tc[1])
				run.AssignColumn("A3", tc[2])
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
