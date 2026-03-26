package byte32cmp

import (
	"fmt"
	"math"
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/stretchr/testify/require"
)

func TestDecompose(t *testing.T) {

	var (
		max16BitsVal = (1 << 16) - 1
	)

	testCases := []struct {
		C smartvectors.SmartVector
	}{
		{
			C: smartvectors.ForTest(0, 1, 2, 3),
		},
		{
			C: smartvectors.ForTest(0, 0, 0, 0),
		},
		{
			C: smartvectors.ForTest(max16BitsVal, max16BitsVal, max16BitsVal, max16BitsVal),
		},
		{
			C: smartvectors.ForTest(math.MaxUint64/2, math.MaxUint64/2, math.MaxUint64/2, math.MaxUint64/2),
		},
	}

	for i := range testCases {
		t.Run(fmt.Sprintf("test-case-%v", i), func(t *testing.T) {

			var pa wizard.ProverAction

			define := func(b *wizard.Builder) {
				p := b.RegisterCommit("C", 4)
				_, pa = Decompose(b.CompiledIOP, p, 4, 16)

			}

			prover := func(run *wizard.ProverRuntime) {
				run.AssignColumn("C", testCases[i].C)
				pa.Run(run)
			}

			comp := wizard.Compile(define, dummy.Compile)
			proof := wizard.Prove(comp, prover)
			err := wizard.Verify(comp, proof)
			require.NoError(t, err)
		})
	}

}
