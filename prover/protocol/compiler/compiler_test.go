package compiler_test

import (
	"runtime/debug"
	"testing"

	"github.com/consensys/linea-monorepo/prover/protocol/compiler"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/poseidon2"
	"github.com/consensys/linea-monorepo/prover/protocol/internal/testtools"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/sirupsen/logrus"
)

var totalSuite = []func(comp *wizard.CompiledIOP){
	poseidon2.CompilePoseidon2,

	compiler.Arcane(
		compiler.WithDebugMode("compiler-tests"),
		compiler.WithStitcherMinSize(2),
		compiler.WithTargetColSize(1024),
	),

	dummy.Compile,
}

func TestCompilers(t *testing.T) {

	logrus.SetLevel(logrus.FatalLevel)

	runTestList(t, "fixed-permutation", testtools.ListOfFixedPermutationTestcasePositive)
	runTestList(t, "global", testtools.ListOfGlobalTestcasePositive)
	runTestList(t, "global", testtools.ListOfGlobalTestcaseNegative)
	runTestList(t, "grand-product", testtools.ListOfGrandProductTestcasePositive)
	runTestList(t, "grand-product", testtools.ListOfGrandProductTestcaseNegative)
	runTestList(t, "horner", testtools.ListOfHornerTestcaseNegative)
	runTestList(t, "horner", testtools.ListOfHornerTestcasePositive)
	runTestList(t, "innerproduct", testtools.ListOfInnerProductTestcasePositive)
	runTestList(t, "logderivativesum", testtools.ListOfLogDerivativeSumTestcasePositive)
	runTestList(t, "logderivativesum", testtools.ListOfLogDerivativeSumTestcaseNegative)
	runTestList(t, "permutation", testtools.ListOfPermutationTestcasePositive)
	runTestList(t, "permutation", testtools.ListOfPermutationTestcaseNegative)
	runTestList(t, "projection", testtools.ListOfProjectionTestcasePositive)
	runTestList(t, "projection", testtools.ListOfProjectionTestcaseNegative)
	runTestList(t, "poseidon2", testtools.ListOfPoseidon2Testcase)
}

func TestCompilersWithGnarkVerifierBLS(t *testing.T) {

	logrus.SetLevel(logrus.FatalLevel)

	runTestListGnark(t, true, "fixed-permutation", testtools.ListOfFixedPermutationTestcasePositive)
	runTestListGnark(t, true, "global", testtools.ListOfGlobalTestcasePositive)
	runTestListGnark(t, true, "grand-product", testtools.ListOfGrandProductTestcasePositive)
	runTestListGnark(t, true, "horner", testtools.ListOfHornerTestcasePositive)
	runTestListGnark(t, true, "innerproduct", testtools.ListOfInnerProductTestcasePositive)
	runTestListGnark(t, true, "logderivativesum", testtools.ListOfLogDerivativeSumTestcasePositive)
	runTestListGnark(t, true, "permutation", testtools.ListOfPermutationTestcasePositive)
	runTestListGnark(t, true, "projection", testtools.ListOfProjectionTestcasePositive)
	runTestListGnark(t, true, "poseidon2", testtools.ListOfPoseidon2Testcase)
}

func TestCompilersWithGnarkVerifierKoala(t *testing.T) {

	logrus.SetLevel(logrus.FatalLevel)

	runTestListGnark(t, false, "fixed-permutation", testtools.ListOfFixedPermutationTestcasePositive)
	runTestListGnark(t, false, "global", testtools.ListOfGlobalTestcasePositive)
	runTestListGnark(t, false, "grand-product", testtools.ListOfGrandProductTestcasePositive)
	runTestListGnark(t, false, "horner", testtools.ListOfHornerTestcasePositive)
	runTestListGnark(t, false, "innerproduct", testtools.ListOfInnerProductTestcasePositive)
	runTestListGnark(t, false, "logderivativesum", testtools.ListOfLogDerivativeSumTestcasePositive)
	runTestListGnark(t, false, "permutation", testtools.ListOfPermutationTestcasePositive)
	runTestListGnark(t, false, "projection", testtools.ListOfProjectionTestcasePositive)
	runTestListGnark(t, false, "poseidon2", testtools.ListOfPoseidon2Testcase)
}

func runTestList[T testtools.Testcase](t *testing.T, prefix string, list []T) {

	t.Run(prefix, func(t *testing.T) {
		for _, tc := range list {
			t.Run(tc.Name(), func(t *testing.T) {

				defer func() {
					if r := recover(); r != nil {
						t.Errorf("Test got a panic: %v", r)
						debug.PrintStack()
					}
				}()

				testtools.RunTestcase(t, tc, totalSuite)
			})
		}
	})
}

func runTestListGnark[T testtools.Testcase](t *testing.T, withBLS bool, prefix string, list []T) {

	t.Run(prefix, func(t *testing.T) {
		for _, tc := range list {
			t.Run(tc.Name(), func(t *testing.T) {
				if withBLS {
					testtools.RunTestShouldPassWithGnarkBLS(t, tc, totalSuite)
				} else {
					testtools.RunTestShouldPassWithGnarkKoala(t, tc, totalSuite)
				}
			})
		}
	})
}
