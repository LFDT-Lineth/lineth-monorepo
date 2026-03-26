package smartvectors

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFuzzProductExt(t *testing.T) {

	for i := 0; i < 1; i++ {
		tcase := newTestBuilderExt(i).NewTestCaseForProdExt()

		success := t.Run(tcase.name, func(t *testing.T) {
			actualProd := ProductExt(tcase.coeffs, tcase.svecs)
			require.Equal(t, tcase.expectedValue.Pretty(), actualProd.Pretty(), "product failed")

			// And let us do it a second time for idempotency
			actualProd = ProductExt(tcase.coeffs, tcase.svecs)
			require.Equal(t, tcase.expectedValue.Pretty(), actualProd.Pretty(), "product failed")
		})

		if !success {
			t.Logf("TEST CASE %v\n", tcase.String())
			t.FailNow()
		}
	}

}

func TestFuzzLinCombExt(t *testing.T) {
	for i := 0; i < FuzzIteration; i++ {
		tcase := newTestBuilderExt(i).NewTestCaseForLinCombExt()

		success := t.Run(tcase.name, func(t *testing.T) {

			actualLinComb := LinCombExt(tcase.coeffs, tcase.svecs)
			require.Equal(t, tcase.expectedValue.Pretty(), actualLinComb.Pretty(), "linear combination failed")

			// And a second time for idempotency
			actualLinComb = LinCombExt(tcase.coeffs, tcase.svecs)
			require.Equal(t, tcase.expectedValue.Pretty(), actualLinComb.Pretty(), "linear combination failed")
		})

		if !success {
			t.Logf("TEST CASE %v\n", tcase.String())
			t.FailNow()
		}
	}
}

func TestFuzzLinearCombinationBase(t *testing.T) {
	for i := 0; i < FuzzIteration; i++ {
		tcase := newTestBuilder(i).NewTestCaseForLinearCombination()

		fmt.Printf("TEST CASE %v\n", tcase.svecs[0].Pretty())
		success := t.Run(tcase.name, func(t *testing.T) {

			actualRes := LinearCombination(tcase.svecs, tcase.evaluationPoint)
			require.Equal(t, tcase.expectedValue.Pretty(), actualRes.Pretty(), "polynomial evaluation failed")

			// and a second time to ensure idempotency
			actualRes = LinearCombination(tcase.svecs, tcase.evaluationPoint)
			require.Equal(t, tcase.expectedValue.Pretty(), actualRes.Pretty(), "polynomial evaluation failed")

		})

		if !success {
			t.Logf("TEST CASE %v\n", tcase.String())
			t.FailNow()
		}
	}
}

func TestFuzzLinearCombinationMixed(t *testing.T) {

	for i := 0; i < FuzzIteration; i++ {
		tcase := newTestBuilder(i).NewTestCaseForLinearCombinationMixed()

		fmt.Printf("TEST CASE %v\n", tcase.svecs[0].Pretty())
		success := t.Run(tcase.name, func(t *testing.T) {

			actualRes := LinearCombinationMixed(tcase.svecs, tcase.evaluationPoint)
			require.Equal(t, tcase.expectedValue.Pretty(), actualRes.Pretty(), "linear combination failed")

			// and a second time to ensure idempotency
			actualRes = LinearCombinationMixed(tcase.svecs, tcase.evaluationPoint)
			require.Equal(t, tcase.expectedValue.Pretty(), actualRes.Pretty(), "linear combination failed")

		})

		if !success {
			t.Logf("TEST CASE %v\n", tcase.String())
			t.FailNow()
		}
	}
}
