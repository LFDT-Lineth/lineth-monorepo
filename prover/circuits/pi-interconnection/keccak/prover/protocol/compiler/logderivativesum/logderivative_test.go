package logderivativesum_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/dummy"
	logderiv "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/logderivativesum"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
	"github.com/stretchr/testify/require"
)

// It tests that the given expression for the LogDerivativeSum adds up to the given parameter.
func TestLogDerivativeSum(t *testing.T) {

	define := func(b *wizard.Builder) {
		var (
			comp = b.CompiledIOP
		)

		size := 4

		p0 := b.RegisterCommit("Num_0", size)
		p1 := b.RegisterCommit("Num_1", size)
		p2 := b.RegisterCommit("Num_2", size)

		q0 := b.RegisterCommit("Den_0", size)
		q1 := b.RegisterCommit("Den_1", size)
		q2 := b.RegisterCommit("Den_2", size)

		inputs := []query.LogDerivativeSumPart{
			{
				Size: size,
				Name: "Part_0",
				Num:  symbolic.Mul(p0, -1),
				Den:  ifaces.ColumnAsVariable(q0),
			},
			{
				Size: size,
				Name: "Part_1",
				Num:  ifaces.ColumnAsVariable(p1),
				Den:  ifaces.ColumnAsVariable(q1),
			},
			{
				Size: size,
				Name: "Part_2",
				Num:  symbolic.Mul(p2, p0, 2),
				Den:  ifaces.ColumnAsVariable(q2),
			},
		}

		comp.InsertLogDerivativeSum(0, "LogDerivSum_Test", query.LogDerivativeSumInput{
			Parts: inputs,
		})

	}

	prover := func(run *wizard.ProverRuntime) {

		run.AssignColumn("Num_0", smartvectors.ForTest(1, 1, 1, 1))
		run.AssignColumn("Num_1", smartvectors.ForTest(2, 3, 7, 9))
		run.AssignColumn("Num_2", smartvectors.ForTest(5, 6, 1, 1))

		run.AssignColumn("Den_0", smartvectors.ForTest(1, 1, 1, 1))
		run.AssignColumn("Den_1", smartvectors.ForTest(2, 3, 7, 9))
		run.AssignColumn("Den_2", smartvectors.ForTest(5, 6, 1, 1))

		run.AssignLogDerivSum("LogDerivSum_Test", field.NewElement(8))

	}

	compiled := wizard.Compile(define, logderiv.CompileLogDerivativeSum, dummy.Compile)
	proof := wizard.Prove(compiled, prover)
	valid := wizard.Verify(compiled, proof)
	require.NoError(t, valid)
}
