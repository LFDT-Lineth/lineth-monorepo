package bigrange_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/dedicated/bigrange"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBigRangeFullField(t *testing.T) {

	define := func(b *wizard.Builder) {
		P := b.RegisterCommit("P", 16)
		bigrange.BigRange(b.CompiledIOP, ifaces.ColumnAsVariable(P), 16, 16, "BIGRANGE")
	}

	prover := func(run *wizard.ProverRuntime) {
		run.AssignColumn("P", smartvectors.Rand(16))
	}

	comp := wizard.Compile(define, dummy.Compile)
	proof := wizard.Prove(comp, prover)
	err := wizard.Verify(comp, proof)
	require.NoError(t, err)

}

func TestBigRangeNegative(t *testing.T) {

	assignments := []smartvectors.SmartVector{
		smartvectors.Rand(16),
		smartvectors.RightPadded(
			vector.Repeat(
				field.NewFromString("0x10000000000000000000000000"), 10),
			field.Zero(),
			16,
		),
	}

	define := func(b *wizard.Builder) {
		P := b.RegisterCommit("P", 16)
		bigrange.BigRange(b.CompiledIOP, ifaces.ColumnAsVariable(P), 1, 16, "BIGRANGE")
	}

	comp := wizard.Compile(define, dummy.Compile)

	for _, v := range assignments {
		prover := func(run *wizard.ProverRuntime) {
			run.AssignColumn("P", v)
		}

		// This should not pass since we assigned a random field element and the
		// constraint is that the field should have less than 16 bits.
		assert.Panics(t, func() {
			_ = wizard.Prove(comp, prover)
		})
	}
}
