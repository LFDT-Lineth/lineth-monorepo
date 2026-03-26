package wizard_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/stretchr/testify/require"
)

const (
	SIZE int = 4
)

func TestCompiler(t *testing.T) {

	// Creates names
	var (
		P    ifaces.ColID   = "P"
		U    ifaces.QueryID = "U"
		COIN coin.Name      = "R"
	)

	define := func(build *wizard.Builder) {
		// Commit to P
		// Sample a random alpha
		// Evaluates P in alpha (evaluation point not yet specified)
		P := build.RegisterCommit(P, SIZE) // Overshadows P with something not of the same type
		build.RegisterRandomCoin(COIN, coin.Field)
		build.UnivariateEval(U, P)
	}

	compiled := wizard.Compile(define, dummy.Compile)

	require.Equal(t, 2, compiled.Coins.NumRounds())
	require.Equal(t, 2, compiled.Columns.NumRounds())
	require.Equal(t, 2, compiled.QueriesParams.NumRounds())
	require.Equal(t, 2, compiled.QueriesNoParams.NumRounds())
	require.Equal(t, 2, compiled.NumRounds())

	compiled.Columns.MustBeInRound(P, 0)
	compiled.Coins.MustBeInRound(1, COIN)
	compiled.QueriesParams.MustBeInRound(1, U)

	prover := func(run *wizard.ProverRuntime) {
		p := smartvectors.ForTest(1, 2, 3, 3)
		run.AssignColumn(P, p)
		u := run.GetRandomCoinField(COIN)
		y := smartvectors.Interpolate(p, u)
		run.AssignUnivariate(U, u, y)
	}

	proof := wizard.Prove(compiled, prover)
	err := wizard.Verify(compiled, proof)
	require.NoError(t, err)
}

func TestChangingColumnStatus(t *testing.T) {

	comp := wizard.NewCompiledIOP()
	comp.InsertCommit(0, "P", 4)

	p := comp.Columns.GetHandle("P").(column.Natural)
	require.Equal(t, column.Committed, p.Status())
	comp.Columns.SetStatus("P", column.Ignored)
	require.Equal(t, column.Ignored, p.Status())
}
