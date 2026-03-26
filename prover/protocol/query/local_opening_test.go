package query_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/stretchr/testify/require"
)

func TestLocalOpening(t *testing.T) {

	n := 16

	define := func(b *wizard.Builder) {
		P := b.RegisterCommit("P", 16)
		for i := 0; i < n; i++ {
			b.LocalOpening(ifaces.QueryIDf("Q_%v", i), column.Shift(P, i))
		}
	}

	prover := func(run *wizard.ProverRuntime) {
		p := make([]field.Element, n)
		for i := range p {
			p[i].SetUint64(uint64(i))
		}
		run.AssignColumn("P", smartvectors.NewRegular(p))

		for i := range p {
			run.AssignLocalPoint(ifaces.QueryIDf("Q_%v", i), p[i])
		}
	}

	compiled := wizard.Compile(define, dummy.Compile)
	proof := wizard.Prove(compiled, prover, false)
	valid := wizard.Verify(compiled, proof, false)
	require.NoError(t, valid)
}
