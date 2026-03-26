package column_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/utils/collection"
	"github.com/stretchr/testify/require"
)

func TestVariableMetaData(t *testing.T) {
	store := column.NewStore()
	V := store.AddToRound(0, ifaces.ColIDf("V"), 4, column.Committed, false)
	v := ifaces.ColumnAsVariable(V)
	vBoard := v.Board()

	// This will panic if the casting does not work
	var _ ifaces.Column = vBoard.ListVariableMetadata()[0].(ifaces.Column)

	require.True(t, v.IsVariable())

	a := column.Shift(V, 1)
	b := column.Shift(V, 1)
	require.True(t, a == b)
}

func TestReprAndDerivation(t *testing.T) {

	store := column.NewStore()
	V := store.AddToRound(0, ifaces.ColID("V"), 4, column.Committed, false)
	v := smartvectors.ForTest(1, 2, 3, 4)
	x := fext.RandomElement()

	// Test for shifting
	{
		shifted1 := smartvectors.ForTest(2, 3, 4, 1)
		Shifted1 := column.Shift(V, 1)

		expectedY := smartvectors.EvaluateBasePolyLagrange(shifted1, x)

		cachedXs := collection.NewMapping[string, fext.Element]()
		cachedXs.InsertNew("", x)

		var (
			x_       = column.DeriveEvaluationPoint(Shifted1, "", cachedXs, x)
			dsBranch = column.DownStreamBranch(Shifted1)
			_        = column.RootParents(Shifted1)
		)

		// Should find the downstreams in the cached map
		require.Equal(t, x_, cachedXs.MustGet(dsBranch))

		// Evaluate the derived claim : should equal the expected Y
		derivedY := smartvectors.EvaluateBasePolyLagrange(v, x_)

		finalYs := collection.NewMapping[string, fext.Element]()
		finalYs.InsertNew(column.DerivedYRepr(dsBranch, V), derivedY)

		// Test that we recovered
		recoveredY := column.VerifyYConsistency(Shifted1, "", cachedXs, finalYs)
		require.Equal(t, expectedY.String(), recoveredY.String())
		require.Equal(t, expectedY.String(), derivedY.String())
	}
}
