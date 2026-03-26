package byte32cmp

import (
	"fmt"
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
)

func TestOneLimbCmp(t *testing.T) {

	testCases := []struct {
		A, B smartvectors.SmartVector
	}{
		{
			A: smartvectors.NewConstant(field.Zero(), 8),
			B: smartvectors.NewConstant(field.Zero(), 8),
		},
		{
			A: smartvectors.NewConstant(field.NewElement((1<<16)-1), 8),
			B: smartvectors.NewConstant(field.NewElement((1<<16)-1), 8),
		},
		{
			A: smartvectors.NewConstant(field.NewElement((1<<16)-1), 8),
			B: smartvectors.NewConstant(field.Zero(), 8),
		},
		{
			A: smartvectors.NewConstant(field.Zero(), 8),
			B: smartvectors.NewConstant(field.NewElement((1<<16)-1), 8),
		},
		{
			A: smartvectors.ForTest(0, 2000, 1, 1, 67, 98, 8192, 12),
			B: smartvectors.ForTest(2000, 1, 1, 67, 98, 8192, 12, 0),
		},
		{
			A: smartvectors.NewConstant(field.Zero(), 8),
			B: smartvectors.NewConstant(field.One(), 8),
		},
	}

	for i := range testCases {
		t.Run(fmt.Sprintf("test-case-%v", i), func(t *testing.T) {

			var pa wizard.ProverAction

			define := func(builder *wizard.Builder) {
				a := builder.RegisterCommit("A", 8)
				b := builder.RegisterCommit("B", 8)

				_, _, _, pa = CmpSmallCols(builder.CompiledIOP, a, b, 16)
			}

			prove := func(run *wizard.ProverRuntime) {
				run.AssignColumn("A", testCases[i].A)
				run.AssignColumn("B", testCases[i].B)
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
