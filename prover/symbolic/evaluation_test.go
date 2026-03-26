package symbolic

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/common/vectorext"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"

	sv "github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
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
				sv.NewConstantExt(fext.NewFromUintBase(2), 1),
				sv.NewConstantExt(fext.NewFromUintBase(3), 1),
			}).(*sv.ConstantExt).Val()

			require.Equal(t, res.String(), "5+0*u+(0+0*u)*v")
		})

		t.Run("const-vec", func(t *testing.T) {
			// 2 + 1 = 3
			// 2 + 5 = 7
			res := b.Evaluate([]sv.SmartVector{
				sv.NewConstantExt(fext.NewFromUintBase(2), 2),
				sv.ForTestExt(1, 5),
			}).(*sv.RegularExt)

			require.Equal(t, (*res)[0].String(), "3+0*u+(0+0*u)*v")
			require.Equal(t, (*res)[1].String(), "7+0*u+(0+0*u)*v")
		})

		t.Run("vec-vec", func(t *testing.T) {
			// For large vectors
			res := b.Evaluate([]sv.SmartVector{
				sv.NewRegularExt(vectorext.Repeat(fext.NewFromUintBase(2), SIZE)),
				sv.NewRegularExt(vectorext.Repeat(fext.NewFromUintBase(3), SIZE)),
			}).(*sv.RegularExt)

			for i := range *res {
				require.Equal(t, (*res)[i].String(), "5+0*u+(0+0*u)*v", "at position %v", i)
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
			sv.NewConstantExt(fext.NewFromUintBase(2), 1),
			sv.NewConstantExt(fext.NewFromUintBase(3), 1),
		}).(*sv.ConstantExt).Val()

		require.Equal(t, res.String(), "13+0*u+(0+0*u)*v")
	}

	{
		// A vector and a scalar
		res := b.Evaluate([]sv.SmartVector{
			sv.NewConstantExt(fext.NewFromUintBase(2), 1024),
			sv.NewRegularExt(vectorext.Repeat(fext.NewFromUintBase(3), 1024)),
		}).(*sv.RegularExt)

		require.Equal(t, res.Len(), 1024)
		for i := range *res {
			require.Equal(t, (*res)[i].String(), "13+0*u+(0+0*u)*v")
		}
	}

	{
		// Two vectors
		res := b.Evaluate([]sv.SmartVector{
			sv.NewRegularExt(vectorext.Repeat(fext.NewFromUintBase(2), 8192)),
			sv.NewRegularExt(vectorext.Repeat(fext.NewFromUintBase(3), 8192)),
		}).(*sv.RegularExt)

		require.Equal(t, res.Len(), 8192)
		for i := range *res {
			require.Equal(t, (*res)[i].String(), "13+0*u+(0+0*u)*v", "at position i = %v", i)
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
			sv.NewConstantExt(fext.NewFromUintBase(2), 1),
			sv.NewConstantExt(fext.NewFromUintBase(3), 1),
		}).(*sv.ConstantExt).Val()

		require.Equal(t, res.String(), "24+0*u+(0+0*u)*v")
	}

	{
		// A vector and a scalar
		res := b.Evaluate([]sv.SmartVector{
			sv.NewConstantExt(fext.NewFromUintBase(2), 1024),
			sv.NewRegularExt(vectorext.Repeat(fext.NewFromUintBase(3), 1024)),
		}).(*sv.RegularExt)

		require.Equal(t, res.Len(), 1024)
		for i := range *res {
			require.Equal(t, (*res)[i].String(), "24+0*u+(0+0*u)*v", "at position i = %v", i)
		}
	}

	{
		// Two vectors
		res := b.Evaluate([]sv.SmartVector{
			sv.NewRegularExt(vectorext.Repeat(fext.NewFromUintBase(2), 8192)),
			sv.NewRegularExt(vectorext.Repeat(fext.NewFromUintBase(3), 8192)),
		}).(*sv.RegularExt)

		require.Equal(t, res.Len(), 8192)
		for i := range *res {
			require.Equal(t, (*res)[i].String(), "24+0*u+(0+0*u)*v", "at position i = %v", i)
		}
	}
}

func TestBinaryConstraintWithLargeWindows(t *testing.T) {

	v := NewDummyVar("v")

	expr2 := v.Mul(NewConstant("1").Sub(v))
	boarded := expr2.Board()

	res := boarded.Evaluate([]sv.SmartVector{
		sv.NewPaddedCircularWindowExt(
			vectorext.ForTest(0, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0),
			fext.Zero(),
			1,
			1<<13,
		),
	})

	for i := 0; i < res.Len(); i++ {
		resx := res.GetExt(i)
		require.Equal(t, "0+0*u+(0+0*u)*v", resx.String(), "position %v", i)
	}
}

func TestExpressionsContainAllCases(t *testing.T) {

	// The all in one expression
	expr := ExpressionContainingAllCases()
	b := expr.Board()

	valA := sv.NewRegularExt(vectorext.Repeat(fext.NewFromUintBase(1), 8192))
	valA0 := sv.NewConstantExt(fext.NewFromUintBase(1), 8192)
	valAW := sv.NewPaddedCircularWindowExt(vectorext.Repeat(fext.One(), 1000), fext.One(), 0, 8192)
	valAWShifted := sv.NewPaddedCircularWindowExt(vectorext.Repeat(fext.One(), 1000), fext.One(), 1, 8192)
	valB := sv.NewConstantExt(fext.NewFromUintBase(3), 8192)

	/*
		Catch potential errors arising from a change in the ordering of the
		metadatas
	*/
	require.Equal(t, b.ListVariableMetadata(), []Metadata{StringVar("a"), StringVar("b")})

	res := b.Evaluate([]sv.SmartVector{valA, valB}).(*sv.RegularExt).GetExt(0)
	res0 := b.Evaluate([]sv.SmartVector{valA0, valB}).GetExt(0)
	resW := b.Evaluate([]sv.SmartVector{valAW, valB}).GetExt(0)
	resWShifted := b.Evaluate([]sv.SmartVector{valAWShifted, valB}).GetExt(0)

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

func TestEvaluatePanics(t *testing.T) {
	x := NewDummyVar("x")
	expr := x.Add(x)
	b := expr.Board()

	t.Run("no input", func(t *testing.T) {
		require.Panics(t, func() {
			b.Evaluate([]sv.SmartVector{})
		})
	})

	t.Run("mismatch size", func(t *testing.T) {
		require.Panics(t, func() {
			b.Evaluate([]sv.SmartVector{
				sv.NewConstantExt(fext.NewFromUintBase(2), 10),
				sv.NewConstantExt(fext.NewFromUintBase(3), 11),
			})
		})
	})

	t.Run("zero size", func(t *testing.T) {
		require.Panics(t, func() {
			b.Evaluate([]sv.SmartVector{
				sv.NewConstantExt(fext.NewFromUintBase(2), 0),
			})
		})
	})

	t.Run("odd size works", func(t *testing.T) {
		// totalSize=33 should work (chunk size falls back to 1)
		require.NotPanics(t, func() {
			b.Evaluate([]sv.SmartVector{
				sv.NewConstantExt(fext.NewFromUintBase(2), 33),
			})
		})
	})
}

func TestPolyEval(t *testing.T) {
	x := NewDummyVar("x")
	c0 := NewDummyVar("c0")
	c1 := NewDummyVar("c1")

	// P(x) = c0 + c1*x
	expr := NewPolyEval(x, []*Expression{c0, c1})
	b := expr.Board()

	// Evaluate with constants
	// x=2, c0=3, c1=4 -> 3 + 4*2 = 11
	res := b.Evaluate([]sv.SmartVector{
		sv.NewConstantExt(fext.NewFromUintBase(2), 1),
		sv.NewConstantExt(fext.NewFromUintBase(3), 1),
		sv.NewConstantExt(fext.NewFromUintBase(4), 1),
	}).(*sv.ConstantExt).Val()

	require.Equal(t, res.String(), "11+0*u+(0+0*u)*v")
}

func TestLinCombEdgeCases(t *testing.T) {
	x := NewDummyVar("x")
	y := NewDummyVar("y")

	// 2x + 2y
	expr2 := x.Add(x).Add(y.Add(y))
	b2 := expr2.Board()
	res2 := b2.Evaluate([]sv.SmartVector{
		sv.NewConstantExt(fext.NewFromUintBase(2), 1),
		sv.NewConstantExt(fext.NewFromUintBase(3), 1),
	}).(*sv.ConstantExt).Val()
	// 2*2 + 2*3 = 4 + 6 = 10
	require.Equal(t, res2.String(), "10+0*u+(0+0*u)*v")

	// -x - y
	exprNeg := x.Neg().Sub(y)
	bNeg := exprNeg.Board()
	resNeg := bNeg.Evaluate([]sv.SmartVector{
		sv.NewConstantExt(fext.NewFromUintBase(2), 1),
		sv.NewConstantExt(fext.NewFromUintBase(3), 1),
	}).(*sv.ConstantExt).Val()
	// -2 - 3 = -5
	// In field arithmetic, -5 is P-5.
	expectedNeg := fext.NewFromUintBase(2)
	expectedNeg.Neg(&expectedNeg)
	val3 := fext.NewFromUintBase(3)
	expectedNeg.Sub(&expectedNeg, &val3)
	require.Equal(t, resNeg.String(), expectedNeg.String())
}

func TestRotatedInputs(t *testing.T) {
	x := NewDummyVar("x")
	expr := x.Add(x)
	b := expr.Board()

	// Create a rotated vector
	// [1, 2, 3, 4] rotated by 1 -> [2, 3, 4, 1]
	vec := []field.Element{
		field.NewElement(1),
		field.NewElement(2),
		field.NewElement(3),
		field.NewElement(4),
	}
	rot := sv.NewRotated(vec, 1)

	// Evaluate x + x with rotated input
	// [2, 3, 4, 1] + [2, 3, 4, 1] = [4, 6, 8, 2]
	res := b.Evaluate([]sv.SmartVector{rot}).(*sv.Regular)

	require.Equal(t, 4, res.Len())
	require.Equal(t, (*res)[0].String(), "4")
	require.Equal(t, (*res)[1].String(), "6")
	require.Equal(t, (*res)[2].String(), "8")
	require.Equal(t, (*res)[3].String(), "2")
}

func TestRotatedExtInputs(t *testing.T) {
	x := NewDummyVar("x")
	expr := x.Add(x)
	b := expr.Board()

	// Create a rotated ext vector
	vec := []fext.Element{
		fext.NewFromUintBase(1),
		fext.NewFromUintBase(2),
		fext.NewFromUintBase(3),
		fext.NewFromUintBase(4),
	}
	rot := sv.NewRotatedExt(vec, 1)

	// Evaluate x + x with rotated input
	res := b.Evaluate([]sv.SmartVector{rot}).(*sv.RegularExt)

	require.Equal(t, 4, res.Len())
	require.Equal(t, (*res)[0].String(), "4+0*u+(0+0*u)*v")
	require.Equal(t, (*res)[1].String(), "6+0*u+(0+0*u)*v")
	require.Equal(t, (*res)[2].String(), "8+0*u+(0+0*u)*v")
	require.Equal(t, (*res)[3].String(), "2+0*u+(0+0*u)*v")
}

func TestProductExponents(t *testing.T) {
	x := NewDummyVar("x")
	// x^3
	expr := x.Mul(x).Mul(x)
	b := expr.Board()

	// x=2 -> 2^3 = 8
	res := b.Evaluate([]sv.SmartVector{
		sv.NewConstantExt(fext.NewFromUintBase(2), 1),
	}).(*sv.ConstantExt).Val()

	require.Equal(t, res.String(), "8+0*u+(0+0*u)*v")
}

func TestProductMixedExponents(t *testing.T) {
	x := NewDummyVar("x")
	y := NewDummyVar("y")
	// x^2 * y
	expr := x.Square().Mul(y)
	b := expr.Board()

	// x=2, y=3 -> 4 * 3 = 12
	res := b.Evaluate([]sv.SmartVector{
		sv.NewConstantExt(fext.NewFromUintBase(2), 1),
		sv.NewConstantExt(fext.NewFromUintBase(3), 1),
	}).(*sv.ConstantExt).Val()

	require.Equal(t, res.String(), "12+0*u+(0+0*u)*v")
}
