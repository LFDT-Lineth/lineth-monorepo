package symbolic

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBoarding(t *testing.T) {

	x := NewDummyVar("x")
	y := NewDummyVar("y")

	{
		// Simple case : x + y
		expr := x.Add(y)
		b := expr.Board()

		require.Equal(t, 3, len(b.Nodes))

		metadatas := b.ListVariableMetadata()
		require.Equal(t, len(metadatas), 2)
		require.Equal(t, metadatas[0].String(), "x")
		require.Equal(t, metadatas[1].String(), "y")
	}

	{
		// Pythagoras
		expr := x.Square().Add(y.Square())
		b := expr.Board()

		require.Equal(t, 5, len(b.Nodes))

		metadatas := b.ListVariableMetadata()
		require.Equal(t, len(metadatas), 2)
		require.Equal(t, metadatas[0].String(), "x")
		require.Equal(t, metadatas[1].String(), "y")
	}

	{
		// It should eliminate duplicate variables
		// (Concretely, it should reuse the x^2)
		expr := x.Square().Add(y.Square()).Mul(x.Square())
		b := expr.Board()

		require.Equal(t, 6, len(b.Nodes))

		metadatas := b.ListVariableMetadata()
		require.Equal(t, len(metadatas), 2)
		require.Equal(t, metadatas[0].String(), "x")
		require.Equal(t, metadatas[1].String(), "y")
	}

}
