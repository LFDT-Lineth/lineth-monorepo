package byte32cmp_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/dedicated/byte32cmp"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
	"github.com/stretchr/testify/require"
)

func TestByte32CmpSimple(t *testing.T) {

	define := func(b *wizard.Builder) {
		colA := b.RegisterCommit("C_A", 2)
		colB := b.RegisterCommit("C_B", 2)
		activeRow := symbolic.NewConstant(1)
		byte32cmp.Bytes32Cmp(b.CompiledIOP, 16, 16, "TEST", colA, colB, activeRow)
	}

	prover := func(run *wizard.ProverRuntime) {
		run.AssignColumn("C_A", smartvectors.ForTest(6, 7))
		run.AssignColumn("C_B", smartvectors.ForTest(4, 5))
	}

	comp := wizard.Compile(define, dummy.Compile)
	proof := wizard.Prove(comp, prover)
	err := wizard.Verify(comp, proof)
	require.NoError(t, err)
}

func TestByte32CmpRandom(t *testing.T) {

	define := func(b *wizard.Builder) {
		colA := b.RegisterCommit("C_A", 16)
		colB := b.RegisterCommit("C_B", 16)
		activeRow := symbolic.NewConstant(1)
		byte32cmp.Bytes32Cmp(b.CompiledIOP, 16, 16, "TEST", colA, colB, activeRow)
	}

	prover := func(run *wizard.ProverRuntime) {
		smallVal := make([]field.Element, 16)
		largeVal := make([]field.Element, 16)
		for i := range smallVal {
			var x, y field.Element
			// We neglect the negligible chance of x being q-1
			x.SetRandom()
			y = field.One()
			smallVal[i] = x
			largeVal[i] = *x.Add(&x, &y)
		}
		run.AssignColumn("C_A", smartvectors.NewRegular(largeVal))
		run.AssignColumn("C_B", smartvectors.NewRegular(smallVal))
	}

	comp := wizard.Compile(define, dummy.Compile)
	proof := wizard.Prove(comp, prover)
	err := wizard.Verify(comp, proof)
	require.NoError(t, err)
}
