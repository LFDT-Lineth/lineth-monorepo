package byte32cmp

import (
	"fmt"
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
)

func TestMultiLimbCmp(t *testing.T) {

	var (
		limbBits = 16
		maxVal   = (1 << limbBits) - 1
	)

	testCases := []struct {
		A, B []smartvectors.SmartVector
	}{
		{
			A: []smartvectors.SmartVector{
				smartvectors.ForTest(0, 0, 0, 0),
				smartvectors.ForTest(0, 0, 0, 0),
				smartvectors.ForTest(0, 0, 0, 0),
			},
			B: []smartvectors.SmartVector{
				smartvectors.ForTest(0, 0, 0, 0),
				smartvectors.ForTest(0, 0, 0, 0),
				smartvectors.ForTest(0, 0, 0, 0),
			},
		},
		{
			A: []smartvectors.SmartVector{
				smartvectors.ForTest(1, 1, 1, 1),
				smartvectors.ForTest(0, 0, 0, 0),
				smartvectors.ForTest(0, 0, 0, 0),
			},
			B: []smartvectors.SmartVector{
				smartvectors.ForTest(0, 0, 0, 0),
				smartvectors.ForTest(1, 1, 1, 1),
				smartvectors.ForTest(0, 0, 0, 0),
			},
		},
		{
			A: []smartvectors.SmartVector{
				smartvectors.ForTest(1, 1, 1, 1),
				smartvectors.ForTest(0, 0, 0, 0),
				smartvectors.ForTest(15, 40, 0, 19),
			},
			B: []smartvectors.SmartVector{
				smartvectors.ForTest(0, 0, 0, 0),
				smartvectors.ForTest(1, maxVal, maxVal, 1),
				smartvectors.ForTest(16, 16, 16, 16),
			},
		},
	}

	for i := range testCases {
		t.Run(fmt.Sprintf("test-case-%v", i), func(t *testing.T) {

			var pa wizard.ProverAction

			define := func(builder *wizard.Builder) {
				var (
					a = LimbColumns{
						Limbs: []ifaces.Column{
							builder.RegisterCommit("A1", 4),
							builder.RegisterCommit("A2", 4),
							builder.RegisterCommit("A3", 4),
						},
						LimbBitSize: 16,
					}
					b = LimbColumns{
						Limbs: []ifaces.Column{
							builder.RegisterCommit("B1", 4),
							builder.RegisterCommit("B2", 4),
							builder.RegisterCommit("B3", 4),
						},
						LimbBitSize: 16,
					}
				)

				_, _, _, pa = CmpMultiLimbs(builder.CompiledIOP, a, b)
			}

			prove := func(run *wizard.ProverRuntime) {
				run.AssignColumn("A1", testCases[i].A[0])
				run.AssignColumn("A2", testCases[i].A[1])
				run.AssignColumn("A3", testCases[i].A[2])
				run.AssignColumn("B1", testCases[i].B[0])
				run.AssignColumn("B2", testCases[i].B[1])
				run.AssignColumn("B3", testCases[i].B[2])
				pa.Run(run)
			}

			comp := wizard.Compile(define, dummy.Compile)
			proof := wizard.Prove(comp, prove)

			if err := wizard.Verify(comp, proof); err != nil {
				t.Fatalf("verification failed: %v", err.Error())
			}

		})
	}

}
