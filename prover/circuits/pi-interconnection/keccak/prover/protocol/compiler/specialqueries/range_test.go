package specialqueries

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/logderivativesum"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/stretchr/testify/require"
)

func TestRangWithLogDerivCompiler(t *testing.T) {

	define := func(b *wizard.Builder) {
		cola := b.RegisterCommit("A", 8)
		colb := b.RegisterCommit("B", 8)

		b.Range("RANG1", cola, 1<<4)
		b.Range("RANG2", colb, 1<<4)
	}

	prover := func(run *wizard.ProverRuntime) {
		// assign a and b
		cola := smartvectors.ForTest(15, 0, 12, 14, 9, 6, 2, 1)

		colb := smartvectors.ForTest(0, 6, 9, 1, 1, 5, 11, 1)

		run.AssignColumn("A", cola)
		run.AssignColumn("B", colb)
	}

	comp := wizard.Compile(define, RangeProof, logderivativesum.CompileLookups, dummy.Compile)
	proof := wizard.Prove(comp, prover)
	err := wizard.Verify(comp, proof)
	require.NoError(t, err)
}
