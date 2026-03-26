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

		require.Equal(t, len(b.Nodes), 2)
		require.Equal(t, len(b.Nodes[0]), 2)
		require.Equal(t, len(b.Nodes[1]), 1)

		metadatas := b.ListVariableMetadata()
		require.Equal(t, len(metadatas), 2)
		require.Equal(t, metadatas[0].String(), "x")
		require.Equal(t, metadatas[1].String(), "y")
	}

	{
		// Pythagoras
		expr := x.Square().Add(y.Square())
		b := expr.Board()

		require.Equal(t, len(b.Nodes), 3)
		require.Equal(t, len(b.Nodes[0]), 2)
		require.Equal(t, len(b.Nodes[1]), 2)
		require.Equal(t, len(b.Nodes[2]), 1)

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

		require.Equal(t, len(b.Nodes), 4)
		require.Equal(t, len(b.Nodes[0]), 2)
		require.Equal(t, len(b.Nodes[1]), 2)
		require.Equal(t, len(b.Nodes[2]), 1)
		require.Equal(t, len(b.Nodes[3]), 1)

		metadatas := b.ListVariableMetadata()
		require.Equal(t, len(metadatas), 2)
		require.Equal(t, metadatas[0].String(), "x")
		require.Equal(t, metadatas[1].String(), "y")
	}

}

func TestNodeID(t *testing.T) {

	levels := []int{0, 0, 1, 2, 15, 64, 1 << 10}
	pos := []int{0, 1, 1, 0, 4, 12, 1 << 10, 42}

	for i := range levels {
		nodeID := newNodeID(levels[i], pos[i])

		require.Equal(t, levels[i], nodeID.level(), "nodeid %v", nodeID)
		require.Equal(t, pos[i], nodeID.posInLevel(), "nodeid %v", nodeID)
	}

}
