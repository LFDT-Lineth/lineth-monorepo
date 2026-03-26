package symbolic

import (
	"testing"

	sv "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/stretchr/testify/require"
)

func TestSimpleAddition(t *testing.T) {
	x := NewDummyVar("x")
	y := NewDummyVar("y")

	SIZE := 2048

	t.Run("x+y", func(t *testing.T) {
		// Simple case : x + y
		expr := x.Add(y)
		b := expr.Board()

		t.Run("const-const", func(t *testing.T) {
			// 2 + 3 = 5
			res := b.Evaluate([]sv.SmartVector{
				sv.NewConstant(field.NewElement(2), 1),
				sv.NewConstant(field.NewElement(3), 1),
			}).(*sv.Constant).Val()

			require.Equal(t, res.String(), "5")
		})

		t.Run("const-vec", func(t *testing.T) {
			// 2 + 1 = 3
			// 2 + 5 = 7
			res := b.Evaluate([]sv.SmartVector{
				sv.NewConstant(field.NewElement(2), 2),
				sv.ForTest(1, 5),
			}).(*sv.Regular)

			require.Equal(t, (*res)[0].String(), "3")
			require.Equal(t, (*res)[1].String(), "7")
		})

		t.Run("vec-vec", func(t *testing.T) {
			// For large vectors
			res := b.Evaluate([]sv.SmartVector{
				sv.NewRegular(vector.Repeat(field.NewElement(2), SIZE)),
				sv.NewRegular(vector.Repeat(field.NewElement(3), SIZE)),
			}).(*sv.Regular)

			for i := range *res {
				require.Equal(t, (*res)[i].String(), "5", "at position %v", i)
			}
		})
	})

}

func TestPythagoras(t *testing.T) {
	x := NewDummyVar("x")
	y := NewDummyVar("y")

	// Pythagoras
	expr := x.Square().Add(y.Square())
	b := expr.Board()

	{
		// 2^2 + 3^2 = 13
		res := b.Evaluate([]sv.SmartVector{
			sv.NewConstant(field.NewElement(2), 1),
			sv.NewConstant(field.NewElement(3), 1),
		}).(*sv.Constant).Val()

		require.Equal(t, res.String(), "13")
	}

	{
		// A vector and a scalar
		res := b.Evaluate([]sv.SmartVector{
			sv.NewConstant(field.NewElement(2), 1024),
			sv.NewRegular(vector.Repeat(field.NewElement(3), 1024)),
		}).(*sv.Regular)

		require.Equal(t, res.Len(), 1024)
		for i := range *res {
			require.Equal(t, (*res)[i].String(), "13")
		}
	}

	{
		// Two vectors
		res := b.Evaluate([]sv.SmartVector{
			sv.NewRegular(vector.Repeat(field.NewElement(2), 8192)),
			sv.NewRegular(vector.Repeat(field.NewElement(3), 8192)),
		}).(*sv.Regular)

		require.Equal(t, res.Len(), 8192)
		for i := range *res {
			require.Equal(t, (*res)[i].String(), "13", "at position i = %v", i)
		}
	}
}

func TestMulAdd(t *testing.T) {
	x := NewDummyVar("x")
	y := NewDummyVar("y")
	// (x + x)(y + y)
	expr := x.Add(x).Mul(y.Add(y))
	b := expr.Board()

	{
		// (2+2) * (3+3) = 24
		res := b.Evaluate([]sv.SmartVector{
			sv.NewConstant(field.NewElement(2), 1),
			sv.NewConstant(field.NewElement(3), 1),
		}).(*sv.Constant).Val()

		require.Equal(t, res.String(), "24")
	}

	{
		// A vector and a scalar
		res := b.Evaluate([]sv.SmartVector{
			sv.NewConstant(field.NewElement(2), 1024),
			sv.NewRegular(vector.Repeat(field.NewElement(3), 1024)),
		}).(*sv.Regular)

		require.Equal(t, res.Len(), 1024)
		for i := range *res {
			require.Equal(t, (*res)[i].String(), "24", "at position i = %v", i)
		}
	}

	{
		// Two vectors
		res := b.Evaluate([]sv.SmartVector{
			sv.NewRegular(vector.Repeat(field.NewElement(2), 8192)),
			sv.NewRegular(vector.Repeat(field.NewElement(3), 8192)),
		}).(*sv.Regular)

		require.Equal(t, res.Len(), 8192)
		for i := range *res {
			require.Equal(t, (*res)[i].String(), "24", "at position i = %v", i)
		}
	}
}

func TestBinaryConstraintWithLargeWindows(t *testing.T) {

	v := NewDummyVar("v")

	expr2 := v.Mul(NewConstant("1").Sub(v))
	boarded := expr2.Board()

	res := boarded.Evaluate([]sv.SmartVector{
		sv.NewPaddedCircularWindow(
			vector.ForTest(0, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0),
			field.Zero(),
			1,
			1<<13,
		),
	})

	for i := 0; i < res.Len(); i++ {
		resx := res.Get(i)
		require.Equal(t, "0", resx.String(), "position %v", i)
	}
}

func TestExpressionsContainAllCases(t *testing.T) {

	// The all in one expression
	expr := ExpressionContainingAllCases()
	b := expr.Board()

	valA := sv.NewRegular(vector.Repeat(field.NewElement(1), 8192))
	valA0 := sv.NewConstant(field.NewElement(1), 8192)
	valAW := sv.NewPaddedCircularWindow(vector.Repeat(field.One(), 1000), field.One(), 0, 8192)
	valAWShifted := sv.NewPaddedCircularWindow(vector.Repeat(field.One(), 1000), field.One(), 1, 8192)
	valB := sv.NewConstant(field.NewElement(3), 8192)

	/*
		Catch potential errors arising from a change in the ordering of the
		metadatas
	*/
	require.Equal(t, b.ListVariableMetadata(), []Metadata{StringVar("a"), StringVar("b")})

	res := b.Evaluate([]sv.SmartVector{valA, valB}).(*sv.Regular).Get(0)
	res0 := b.Evaluate([]sv.SmartVector{valA0, valB}).Get(0)
	resW := b.Evaluate([]sv.SmartVector{valAW, valB}).Get(0)
	resWShifted := b.Evaluate([]sv.SmartVector{valAWShifted, valB}).Get(0)

	/*
		Compare the result of the two evaluations
	*/
	require.Equal(t, res0.String(), res.String())
	require.Equal(t, resW.String(), res.String())
	require.Equal(t, resWShifted.String(), res.String())
}

func TestDegree(t *testing.T) {
	x := NewDummyVar("x")
	y := NewDummyVar("y")

	// Function that gives a degree 1024 to all variables
	getdeg := func(interface{}) int { return 1024 }

	{
		// Simple case : x + y
		expr := x.Add(y)
		b := expr.Board()

		require.Equal(t, 1024, b.Degree(getdeg))
	}

	{
		// Pythagoras
		expr := x.Square().Add(y.Square())
		b := expr.Board()
		require.Equal(t, 2048, b.Degree(getdeg))
	}

	{
		// (x + x)(y + y)
		expr := x.Add(x).Mul(y.Add(y))
		b := expr.Board()
		require.Equal(t, 2048, b.Degree(getdeg))
	}

	{
		// The all in one expression
		expr := ExpressionContainingAllCases()
		b := expr.Board()
		require.Equal(t, 2048, b.Degree(getdeg))
	}

}

func ExpressionContainingAllCases() *Expression {

	/*
		Essentially, a will be a vector and b, a non-vector
		And we create an expression that goes through all
		possible combinations
	*/
	a := NewDummyVar("a")
	b := NewDummyVar("b")
	c := NewConstant(36)

	// For the addition
	expr := a.Add(a)
	expr = expr.Add(a.Add(b))
	expr = expr.Add(a.Add(c))
	expr = expr.Add(b.Add(a))
	expr = expr.Add(b.Add(b))
	expr = expr.Add(b.Add(c))
	expr = expr.Add(c.Add(a))
	expr = expr.Add(c.Add(b))
	expr = expr.Add(c.Add(c))

	// Substraction
	expr = expr.Add(a.Sub(a))
	expr = expr.Add(a.Sub(b))
	expr = expr.Add(a.Sub(c))
	expr = expr.Add(b.Sub(a))
	expr = expr.Add(b.Sub(b))
	expr = expr.Add(b.Sub(c))
	expr = expr.Add(c.Sub(a))
	expr = expr.Add(c.Sub(b))
	expr = expr.Add(c.Sub(c))

	// Multiplication
	expr = expr.Add(a.Mul(a))
	expr = expr.Add(a.Mul(b))
	expr = expr.Add(a.Mul(c))
	expr = expr.Add(b.Mul(a))
	expr = expr.Add(b.Mul(b))
	expr = expr.Add(b.Mul(c))
	expr = expr.Add(c.Mul(a))
	expr = expr.Add(c.Mul(b))
	expr = expr.Add(c.Mul(c))

	// Negation
	expr = expr.Add(a.Neg())
	expr = expr.Add(b.Neg())
	expr = expr.Add(c.Neg())

	return expr
}
