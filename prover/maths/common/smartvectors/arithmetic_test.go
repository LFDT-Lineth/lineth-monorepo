//go:build !race

package smartvectors

import (
	"fmt"
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/common/mempool"
	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFuzzProduct(t *testing.T) {

	for i := 0; i < FuzzIteration; i++ {
		tcase := newTestBuilder(i).NewTestCaseForProd()

		success := t.Run(tcase.name, func(t *testing.T) {
			actualProd := Product(tcase.coeffs, tcase.svecs)
			require.Equal(t, tcase.expectedValue.Pretty(), actualProd.Pretty(), "product failed")

			// And let us do it a second time for idempotency
			actualProd = Product(tcase.coeffs, tcase.svecs)
			require.Equal(t, tcase.expectedValue.Pretty(), actualProd.Pretty(), "product failed")
		})

		if !success {
			t.Logf("TEST CASE %v\n", tcase.String())
			t.FailNow()
		}
	}

}

func TestFuzzLinComb(t *testing.T) {
	for i := 0; i < FuzzIteration; i++ {
		tcase := newTestBuilder(i).NewTestCaseForLinComb()

		success := t.Run(tcase.name, func(t *testing.T) {

			actualLinComb := LinComb(tcase.coeffs, tcase.svecs)
			require.Equal(t, tcase.expectedValue.Pretty(), actualLinComb.Pretty(), "linear combination failed")

			// And a second time for idempotency
			actualLinComb = LinComb(tcase.coeffs, tcase.svecs)
			require.Equal(t, tcase.expectedValue.Pretty(), actualLinComb.Pretty(), "linear combination failed")
		})

		if !success {
			t.Logf("TEST CASE %v\n", tcase.String())
			t.FailNow()
		}
	}
}

func TestFuzzPolyEval(t *testing.T) {
	for i := 0; i < FuzzIteration; i++ {
		tcase := newTestBuilder(i).NewTestCaseForPolyEval()

		success := t.Run(tcase.name, func(t *testing.T) {

			actualRes := PolyEval(tcase.svecs, tcase.evaluationPoint)
			require.Equal(t, tcase.expectedValue.Pretty(), actualRes.Pretty(), "linear combination failed")

			// and a second time to ensure idempotency
			actualRes = PolyEval(tcase.svecs, tcase.evaluationPoint)
			require.Equal(t, tcase.expectedValue.Pretty(), actualRes.Pretty(), "linear combination failed")

		})

		if !success {
			t.Logf("TEST CASE %v\n", tcase.String())
			t.FailNow()
		}
	}
}

func TestFuzzProductWithPool(t *testing.T) {

	for i := 0; i < FuzzIteration; i++ {
		tcase := newTestBuilder(i).NewTestCaseForProd()

		success := t.Run(tcase.name, func(t *testing.T) {

			pool := mempool.CreateFromSyncPool(tcase.svecs[0].Len())

			t.Logf("TEST CASE %v\n", tcase.String())

			prodWithPool := Product(tcase.coeffs, tcase.svecs, pool)
			require.Equal(t, tcase.expectedValue.Pretty(), prodWithPool.Pretty(), "product with pool failed")

			// And let us do it a second time for idempotency
			prodWithPool = Product(tcase.coeffs, tcase.svecs, pool)
			require.Equal(t, tcase.expectedValue.Pretty(), prodWithPool.Pretty(), "product with pool failed")
		})

		if !success {
			t.Logf("TEST CASE %v\n", tcase.String())
			t.FailNow()
		}
	}

}

func TestFuzzProductWithPoolCompare(t *testing.T) {

	for i := 0; i < FuzzIteration; i++ {
		tcase := newTestBuilder(i).NewTestCaseForProd()

		success := t.Run(tcase.name, func(t *testing.T) {

			pool := mempool.CreateFromSyncPool(tcase.svecs[0].Len())

			t.Logf("TEST CASE %v\n", tcase.String())

			// Product() with pool
			prodWithPool := Product(tcase.coeffs, tcase.svecs, pool)
			require.Equal(t, tcase.expectedValue.Pretty(), prodWithPool.Pretty(), "Product() with pool failed")

			// Product() without pool
			prod := Product(tcase.coeffs, tcase.svecs)

			// check if Product() with pool = Product() without pool
			require.Equal(t, prodWithPool.Pretty(), prod.Pretty(), "Product() w/ and w/o pool are different")
		})

		if !success {
			t.Logf("TEST CASE %v\n", tcase.String())
			t.FailNow()
		}
	}

}

func TestFuzzLinCombWithPool(t *testing.T) {

	for i := 0; i < FuzzIteration; i++ {
		tcase := newTestBuilder(i).NewTestCaseForLinComb()

		success := t.Run(tcase.name, func(t *testing.T) {

			pool := mempool.CreateFromSyncPool(tcase.svecs[0].Len())

			t.Logf("TEST CASE %v\n", tcase.String())

			linCombWithPool := LinComb(tcase.coeffs, tcase.svecs, pool)
			require.Equal(t, tcase.expectedValue.Pretty(), linCombWithPool.Pretty(), "LinComb() with pool failed")

			// And let us do it a second time for idempotency
			linCombWithPool = LinComb(tcase.coeffs, tcase.svecs, pool)
			require.Equal(t, tcase.expectedValue.Pretty(), linCombWithPool.Pretty(), "LinComb() with pool failed")
		})

		if !success {
			t.Logf("TEST CASE %v\n", tcase.String())
			t.FailNow()
		}
	}
}

func TestFuzzLinCombWithPoolCompare(t *testing.T) {

	for i := 0; i < FuzzIteration; i++ {
		tcase := newTestBuilder(i).NewTestCaseForLinComb()

		success := t.Run(tcase.name, func(t *testing.T) {

			pool := mempool.CreateFromSyncPool(tcase.svecs[0].Len())

			t.Logf("TEST CASE %v\n", tcase.String())

			// LinComb() with pool
			linCombWithPool := LinComb(tcase.coeffs, tcase.svecs, pool)
			require.Equal(t, tcase.expectedValue.Pretty(), linCombWithPool.Pretty(), "LinComb() with pool failed")

			// LinComb() without pool
			linComb := LinComb(tcase.coeffs, tcase.svecs)

			// check if LinComb() with pool = LinComb() without pool
			require.Equal(t, linCombWithPool.Pretty(), linComb.Pretty(), "LinComb() w/ and w/o pool are different")
		})

		if !success {
			t.Logf("TEST CASE %v\n", tcase.String())
			t.FailNow()
		}
	}
}

func TestOpBasicEdgeCases(t *testing.T) {

	two := field.NewElement(2)

	testCases := []struct {
		explainer   string
		inputs      []SmartVector
		expectedRes SmartVector
		fn          func(...SmartVector) SmartVector
	}{
		{
			explainer: "full-covering windows and a constant",
			inputs: []SmartVector{
				NewConstant(two, 16),
				LeftPadded(vector.Repeat(two, 12), two, 16),
				RightPadded(vector.Repeat(two, 12), two, 16),
			},
			expectedRes: NewRegular(vector.Repeat(field.NewElement(6), 16)),
			fn:          Add,
		},
		{
			explainer: "full-covering windows and a constant (mul)",
			inputs: []SmartVector{
				NewConstant(two, 16),
				LeftPadded(vector.Repeat(two, 12), two, 16),
				RightPadded(vector.Repeat(two, 12), two, 16),
			},
			expectedRes: NewRegular(vector.Repeat(field.NewElement(8), 16)),
			fn:          Mul,
		},
		{
			explainer: "full-covering windows, a regular and a constant",
			inputs: []SmartVector{
				NewConstant(two, 16),
				LeftPadded(vector.Repeat(two, 12), two, 16),
				RightPadded(vector.Repeat(two, 12), two, 16),
				NewRegular(vector.Repeat(two, 16)),
			},
			expectedRes: NewRegular(vector.Repeat(field.NewElement(8), 16)),
			fn:          Add,
		},
	}

	for i, testCase := range testCases {
		t.Run(fmt.Sprintf("case-%v", i), func(t *testing.T) {
			t.Logf("test-case details: %v", testCase.explainer)
			res := testCase.fn(testCase.inputs...).(*Pooled)
			actual := NewRegular(res.Regular)
			require.Equal(t, testCase.expectedRes, actual, "expectedRes=%v\nres=%v", testCase.expectedRes.Pretty(), res.Pretty())
		})
	}
}

func TestInnerProduct(t *testing.T) {
	testCases := []struct {
		a, b SmartVector
		y    field.Element
	}{
		{
			a: ForTest(1, 2, 1, 2, 1),
			b: ForTest(1, -1, 2, -1, 2),
			y: field.NewElement(1),
		},
	}

	for i, testCase := range testCases {
		t.Run(fmt.Sprintf("case-%v", i), func(t *testing.T) {
			y := InnerProduct(testCase.a, testCase.b)
			assert.Equal(t, testCase.y.String(), y.String())
		})
	}
}

func TestScalarMul(t *testing.T) {
	testCases := []struct {
		a SmartVector
		b field.Element
		y SmartVector
	}{
		{
			a: ForTest(1, 2, 1, 2, 1),
			b: field.NewElement(3),
			y: ForTest(3, 6, 3, 6, 3),
		},
	}

	for i, testCase := range testCases {
		t.Run(fmt.Sprintf("case-%v", i), func(t *testing.T) {
			y := ScalarMul(testCase.a, testCase.b)
			assert.Equal(t, testCase.y.Pretty(), y.Pretty())
		})
	}
}

func TestFuzzPolyEvalWithPool(t *testing.T) {
	for i := 0; i < FuzzIteration; i++ {
		tcase := newTestBuilder(i).NewTestCaseForPolyEval()

		success := t.Run(tcase.name, func(t *testing.T) {

			pool := mempool.CreateFromSyncPool(tcase.svecs[0].Len())

			// PolyEval() with pool
			polyEvalWithPool := PolyEval(tcase.svecs, tcase.evaluationPoint, pool)
			require.Equal(t, tcase.expectedValue.Pretty(), polyEvalWithPool.Pretty(), "linear combination with pool failed")

			// and a second time to ensure idempotency
			polyEvalWithPool = PolyEval(tcase.svecs, tcase.evaluationPoint, pool)
			require.Equal(t, tcase.expectedValue.Pretty(), polyEvalWithPool.Pretty(), "linear combination with pool failed")

		})

		if !success {
			t.Logf("TEST CASE %v\n", tcase.String())
			t.FailNow()
		}
	}
}

func TestFuzzPolyEvalWithPoolCompare(t *testing.T) {
	for i := 0; i < FuzzIteration; i++ {
		tcase := newTestBuilder(i).NewTestCaseForPolyEval()

		success := t.Run(tcase.name, func(t *testing.T) {

			pool := mempool.CreateFromSyncPool(tcase.svecs[0].Len())

			// PolyEval() with pool
			polyEvalWithPool := PolyEval(tcase.svecs, tcase.evaluationPoint, pool)
			require.Equal(t, tcase.expectedValue.Pretty(), polyEvalWithPool.Pretty(), "PolyEval() with pool failed")

			// PolyEval() without pool
			polyEval := PolyEval(tcase.svecs, tcase.evaluationPoint)

			// check if PolyEval() with pool = PolyEval() without pool
			require.Equal(t, polyEvalWithPool.Pretty(), polyEval.Pretty(), "PolyEval() w/ and w/o pool are different")

		})

		if !success {
			t.Logf("TEST CASE %v\n", tcase.String())
			t.FailNow()
		}
	}
}
