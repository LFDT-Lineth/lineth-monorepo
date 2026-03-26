package symbolic

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExp(t *testing.T) {

	getdeg := func(interface{}) int { return 1 }
	x := NewDummyVar("x")
	y := NewDummyVar("y")

	t.Run("mul constant", func(t *testing.T) {
		// x time y
		expr := x.Mul(NewConstant(1))
		board := expr.Board()
		deg := board.Degree(getdeg)
		require.Equal(t, deg, 1)
	})

	t.Run("mul x y", func(t *testing.T) {
		// x time y
		expr := x.Mul(y)
		board := expr.Board()
		deg := board.Degree(getdeg)
		require.Equal(t, deg, 2)
	})

	t.Run("x squared", func(t *testing.T) {
		// x^2
		expr := x.Square()
		fmt.Printf("expr = %++v\n", expr)
		board := expr.Board()
		deg := board.Degree(getdeg)
		require.Len(t, board.Nodes, 2)
		require.Len(t, board.Nodes[1].Children, 1)
		require.Equal(t, deg, 2)
	})

	t.Run("x cube", func(t *testing.T) {
		// x^3
		expr := x.Square().Mul(x)
		board := expr.Board()
		deg := board.Degree(getdeg)
		require.Equal(t, deg, 3)
	})

	t.Run("x to the 4th", func(t *testing.T) {
		// x^4
		expr := x.Square().Mul(x.Square())
		expr.AssertValid()
		board := expr.Board()
		deg := board.Degree(getdeg)
		require.Equal(t, deg, 4)
	})

	t.Run("x to the 4th (2)", func(t *testing.T) {
		// x^4
		expr := x.Mul(x).Mul(x).Mul(x)
		expr.AssertValid()
		board := expr.Board()
		deg := board.Degree(getdeg)
		require.Equal(t, deg, 4)
	})

	t.Run("x to the 5th", func(t *testing.T) {
		// x^5
		expr := x.Pow(5)
		expr.AssertValid()
		require.Lenf(t, expr.Children, 1, "for input %v\n", 5)
		require.IsType(t, expr.Operator, Product{})
		board := expr.Board()
		deg := board.Degree(getdeg)
		require.Equal(t, deg, 5)
	})

	t.Run("powers", func(t *testing.T) {
		for i := 2; i < 10; i++ {
			expr := x.Pow(i)
			expr.AssertValid()
			require.Lenf(t, expr.Children, 1, "for input %v\n", i)
			require.IsType(t, expr.Operator, Product{})
			board := expr.Board()
			deg := board.Degree(getdeg)
			require.Equal(t, deg, i)
		}
	})

	t.Run("prod with zero coeffs", func(t *testing.T) {
		expr := NewProduct([]*Expression{x, y}, []int{0, 0})
		require.Equal(t, NewConstant(1), expr)
	})

}
