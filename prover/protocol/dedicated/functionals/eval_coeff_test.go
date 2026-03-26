package functionals_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/functionals"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/stretchr/testify/require"
)

func TestEvalCoeff(t *testing.T) {

	var (
		x            coin.Info
		p            ifaces.Column
		acc          ifaces.Accessor
		savedRuntime *wizard.ProverRuntime
	)

	wp := smartvectors.Rand(32)

	definer := func(b *wizard.Builder) {
		p = b.RegisterCommit("P", wp.Len())
		x = b.RegisterRandomCoin("X", coin.FieldExt)
		acc = functionals.CoeffEval(b.CompiledIOP, "EVAL", x, p)

	}

	prover := func(run *wizard.ProverRuntime) {
		// Save the pointer toward the prover
		savedRuntime = run
		run.AssignColumn("P", wp)
	}

	compiled := wizard.Compile(definer,
		dummy.Compile,
	)

	proof := wizard.Prove(compiled, prover)

	xVal := savedRuntime.GetRandomCoinFieldExt(x.Name)
	accY := acc.GetValExt(savedRuntime)
	expectedY := smartvectors.EvalCoeffExt(wp, xVal)

	require.Equal(t, accY, expectedY)

	err := wizard.Verify(compiled, proof)
	require.NoError(t, err)
}
