package query_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/internal/testtools"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
)

// TestQuery runs all the testtools.Testcases over the [dummy.Compile]
// compiler. This aims at testing the [Check] function of the queries
// and ensuring the implementations are both complete and sound.
func TestQuery(t *testing.T) {
	runTestList(t, "global", testtools.ListOfGlobalTestcasePositive)
	runTestList(t, "global", testtools.ListOfGlobalTestcaseNegative)
	runTestList(t, "horner", testtools.ListOfHornerTestcasePositive)
	runTestList(t, "horner", testtools.ListOfHornerTestcaseNegative)
	runTestList(t, "grand-product", testtools.ListOfGrandProductTestcasePositive)
	runTestList(t, "grand-product", testtools.ListOfGrandProductTestcaseNegative)
	runTestList(t, "projection", testtools.ListOfProjectionTestcasePositive)
	runTestList(t, "projection", testtools.ListOfProjectionTestcaseNegative)
	runTestList(t, "permutation", testtools.ListOfPermutationTestcasePositive)
	runTestList(t, "permutation", testtools.ListOfPermutationTestcaseNegative)
	runTestList(t, "log-derivative", testtools.ListOfLogDerivativeSumTestcasePositive)
	runTestList(t, "log-derivative", testtools.ListOfLogDerivativeSumTestcaseNegative)
	runTestList(t, "poseidon2", testtools.ListOfPoseidon2Testcase)

	// runTestList(t, "fixed-permutation", testtools.ListOfFixedPermutationTestcasePositive)
}

func runTestList[T testtools.Testcase](t *testing.T, prefix string, list []T) {

	dummySuite := []func(comp *wizard.CompiledIOP){
		dummy.Compile,
	}

	t.Run(prefix, func(t *testing.T) {
		for _, tc := range list {
			t.Run(tc.Name(), func(t *testing.T) {
				testtools.RunTestcase(t, tc, dummySuite)
			})
		}
	})
}
