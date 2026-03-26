package symbolic

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/collection"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReplayExpression(t *testing.T) {

	// Original dummy variable
	a, b, c, x := NewDummyVar("a"), NewDummyVar("b"), NewDummyVar("c"), NewDummyVar("x")
	a_, b_, c_, x_ := NewDummyVar("a_"), NewDummyVar("b_"), NewDummyVar("c_"), NewDummyVar("x_")

	expressions := []*Expression{
		a.Add(b),
		a.Add(a).Mul(b),
		a.Neg().Add(b).Neg().Mul(c).Add(a),
		a.Sub(b).Mul(c),
		a.Mul(a).Mul(b).Mul(a).Mul(c).Mul(c),
		a.Mul(NewPolyEval(x, []*Expression{a, b, a, a.Add(c)})),
	}

	// Random constants for the polyEvals
	var r field.Element
	r.SetRandom()

	witnesses := map[string]smartvectors.SmartVector{
		"a": smartvectors.ForTest(1, 2, 3, 4, 5, 6, 7, 8),
		"b": smartvectors.ForTest(8, 16, 32, 64, 128, 256, 512, 1024),
		"c": smartvectors.ForTest(1, 1, 2, 3, 5, 8, 13, 21),
		"x": smartvectors.NewConstant(r, 8),
	}

	witnesses_ := map[string]smartvectors.SmartVector{
		"a_": witnesses["a"].SubVector(0, 4),
		"b_": witnesses["b"].SubVector(0, 4),
		"c_": witnesses["c"].SubVector(0, 4),
		"x_": witnesses["x"].SubVector(0, 4),
	}

	translationMap := collection.NewMapping[string, *Expression]()
	translationMap.InsertNew("a", a_)
	translationMap.InsertNew("b", b_)
	translationMap.InsertNew("c", c_)
	translationMap.InsertNew("x", x_)

	for _, expr := range expressions {
		board := expr.Board()
		metadatas := board.ListVariableMetadata()
		inputs := make([]smartvectors.SmartVector, len(metadatas))
		for i := range metadatas {
			inputs[i] = witnesses[metadatas[i].String()]
		}

		replayed := expr.Replay(translationMap)

		// Check that the expression is valid
		replayed.AssertValid()

		replayedBoard := replayed.Board()
		replayedMetadata := replayedBoard.ListVariableMetadata()
		require.Len(t, replayedMetadata, len(metadatas))

		inputs_ := make([]smartvectors.SmartVector, len(metadatas))
		for i := range replayedMetadata {
			inputs_[i] = witnesses_[replayedMetadata[i].String()]
		}

		// Should be the same thing as the old board
		oldBoard := expr.Board()
		oldMetadata := oldBoard.ListVariableMetadata()
		require.Len(t, oldMetadata, len(metadatas))

		for i := range metadatas {
			require.Equal(t, metadatas[i].String(), oldMetadata[i].String())
			require.Equal(t, metadatas[i].String()+"_", replayedMetadata[i].String())
		}

		eval := board.Evaluate(inputs)
		replayedEval := replayedBoard.Evaluate(inputs_)
		oldEval := oldBoard.Evaluate(inputs)

		// The oldEval and eval should be consistent
		require.Equal(t, eval.Pretty(), oldEval.Pretty())
		require.Equal(t, eval.SubVector(0, 4).Pretty(), replayedEval.Pretty())
	}

}

func TestLCConstruction(t *testing.T) {

	x := NewDummyVar("x")
	y := NewDummyVar("y")

	t.Run("simple-addition", func(t *testing.T) {
		/*
			Test t a simple case of addition
		*/
		expr1 := x.Add(y)

		require.Equal(t, 2, len(expr1.Children))
		require.Equal(t, expr1.Children[0], x)
		require.Equal(t, expr1.Children[1], y)
		require.Equal(t, 2, len(expr1.Operator.(LinComb).Coeffs))
		require.Equal(t, expr1.Operator.(LinComb).Coeffs[0], 1)
		require.Equal(t, expr1.Operator.(LinComb).Coeffs[1], 1)
	})

	t.Run("x-y-x", func(t *testing.T) {
		/*
			Adding y then substracting x should give back (y)
		*/
		expr1 := x.Add(y).Sub(x)
		require.Equal(t, expr1, y)
	})

	t.Run("(-x)+x+y", func(t *testing.T) {
		/*
			Same thing when using Neg
		*/
		expr := x.Neg().Add(x).Add(y)
		assert.Equal(t, expr, y)
	})

}

func TestProductConstruction(t *testing.T) {

	x := NewDummyVar("x")
	y := NewDummyVar("y")

	t.Run("x * y", func(t *testing.T) {
		/*
			Test t a simple case of addition
		*/
		expr1 := x.Mul(y)

		require.Equal(t, 2, len(expr1.Children))
		require.Equal(t, expr1.Children[0], x)
		require.Equal(t, expr1.Children[1], y)
		require.Equal(t, 2, len(expr1.Operator.(Product).Exponents))
		require.Equal(t, expr1.Operator.(Product).Exponents[0], 1)
		require.Equal(t, expr1.Operator.(Product).Exponents[1], 1)
	})

	t.Run("x * y * x", func(t *testing.T) {
		/*
			Adding y then substracting x should give back (y)
		*/
		expr1 := x.Mul(y).Mul(x)
		require.Equal(t, 2, len(expr1.Children))
		require.Equal(t, expr1.Children[0], x)
		require.Equal(t, expr1.Children[1], y)
		require.Equal(t, 2, len(expr1.Operator.(Product).Exponents))
		require.Equal(t, expr1.Operator.(Product).Exponents[0], 2)
		require.Equal(t, expr1.Operator.(Product).Exponents[1], 1)
	})

	t.Run("x^2", func(t *testing.T) {
		/*
			When we square
		*/
		expr := x.Mul(x)
		require.Equal(t, 1, len(expr.Children))
		require.Equal(t, expr.Children[0], x)
		require.Equal(t, 1, len(expr.Operator.(Product).Exponents))
		require.Equal(t, expr.Operator.(Product).Exponents[0], 2)
	})

}
