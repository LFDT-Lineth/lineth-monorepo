package univariates

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

func TestNaturalize(t *testing.T) {

	var (
		P1   ifaces.ColID   = "P1"
		P2   ifaces.ColID   = "P2"
		P3   ifaces.ColID   = "P3"
		P4   ifaces.ColID   = "P4"
		EVAL ifaces.QueryID = "EVAL"
		P1S2 ifaces.Column
	)

	definer := func(build *wizard.Builder) {
		P1 := build.RegisterCommit(P1, 4) // overshadowing
		P2 := build.RegisterCommit(P2, 4) // overshadowing
		P3 := build.RegisterCommit(P3, 8) // overshadowing
		P4 := build.RegisterCommit(P4, 8) // overshadowing
		P1S2 = column.Shift(P1, 2)
		build.UnivariateEval(EVAL, P1, P1S2, P2, P3, P4)
	}

	comp := wizard.Compile(
		definer,
		Naturalize,
		dummy.Compile,
	)

	require.Equal(t, len(comp.QueriesParams.AllKeysAt(0)), 3)

	hLProver := func(assi *wizard.ProverRuntime) {
		p1 := smartvectors.ForTest(1, 2, 3, 4)
		p2 := smartvectors.ForTest(3, 4, 1, 2)
		p3 := smartvectors.ForTest(1, 2, 3, 4, 1, 2, 3, 4)
		p4 := smartvectors.ForTest(2, 3, 4, 1, 2, 3, 4, 1)

		assi.AssignColumn(P1, p1)
		assi.AssignColumn(P2, p2)
		assi.AssignColumn(P3, p3)
		assi.AssignColumn(P4, p4)

		x := field.NewElement(5)

		y1 := smartvectors.Interpolate(p1, x)
		y2 := smartvectors.Interpolate(p2, x)
		y3 := smartvectors.Interpolate(p3, x)
		y4 := smartvectors.Interpolate(p4, x)

		p1s2 := P1S2.GetColAssignment(assi)

		require.Equal(t, p1s2.Pretty(), p2.Pretty())
		assi.AssignUnivariate(EVAL, x, y1, y2, y2, y3, y4)
	}

	proof := wizard.Prove(comp, hLProver)
	err := wizard.Verify(comp, proof)
	require.NoError(t, err)

}
