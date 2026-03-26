package symbolic

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLinComb(t *testing.T) {

	x := NewDummyVar("x")
	y := NewDummyVar("y")

	{
		// x + x = 2x
		expr := x.Add(x)
		require.Len(t, expr.Children, 1)
	}

	{
		// x + y
		expr := x.Add(y)
		require.Len(t, expr.Children, 2)
	}

	{
		// (x + x) + (x + x)
		expr := x.Add(x).Add(x.Add(x))
		require.Len(t, expr.Children, 1)
	}

	{
		// ((x + x) + x) + x
		expr := x.Add(x).Add(x).Add(x)
		require.Len(t, expr.Children, 1)
	}

	{
		expr := NewLinComb([]*Expression{x, y}, []int{0, 0})
		require.Equal(t, NewConstant(0), expr)
	}

}
