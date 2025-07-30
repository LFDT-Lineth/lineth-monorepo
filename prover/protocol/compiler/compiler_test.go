package compiler_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/globalcs"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/horner"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/innerproduct"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/localcs"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/logderivativesum"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/mimc"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/mpts"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/permutation"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/plonkinwizard"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/specialqueries"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/stitchsplit"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/univariates"
	"github.com/consensys/linea-monorepo/prover/protocol/internal/testtools"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/sirupsen/logrus"
)

var totalSuite = []func(comp *wizard.CompiledIOP){
	mimc.CompileMiMC,
	plonkinwizard.Compile,
	specialqueries.RangeProof,
	specialqueries.CompileFixedPermutations,
	permutation.CompileIntoGdProduct,
	permutation.CompileGrandProduct,
	logderivativesum.LookupIntoLogDerivativeSum,
	logderivativesum.CompileLogDerivativeSum,
	horner.ProjectionToHorner,
	horner.CompileHorner,
	innerproduct.Compile(),
	stitchsplit.Stitcher(1, 8),
	stitchsplit.Splitter(8),
	localcs.Compile,
	globalcs.Compile,
	univariates.Naturalize,
	mpts.Compile(),
	dummy.Compile,
	// vortex.Compile(2, vortex.ReplaceSisByMimc(), vortex.ForceNumOpenedColumns(2)),
}

func TestCompilers(t *testing.T) {

	logrus.SetLevel(logrus.FatalLevel)

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
	runTestList(t, "logderivativesum", testtools.ListOfLogDerivativeSumTestcasePositive)
	runTestList(t, "logderivativesum", testtools.ListOfLogDerivativeSumTestcaseNegative)
	runTestList(t, "mimc", testtools.ListOfMiMCTestcase)
	runTestList(t, "fixed-permutation", testtools.ListOfFixedPermutationTestcasePositive)
}

func TestCompilersWithGnarkVerifier(t *testing.T) {

	logrus.SetLevel(logrus.FatalLevel)

	runTestListGnark(t, "global", testtools.ListOfGlobalTestcasePositive)
	runTestListGnark(t, "horner", testtools.ListOfHornerTestcasePositive)
	runTestListGnark(t, "grand-product", testtools.ListOfGrandProductTestcasePositive)
	runTestListGnark(t, "projection", testtools.ListOfProjectionTestcasePositive)
	runTestListGnark(t, "permutation", testtools.ListOfPermutationTestcasePositive)
	runTestListGnark(t, "logderivativesum", testtools.ListOfLogDerivativeSumTestcasePositive)
	runTestListGnark(t, "mimc", testtools.ListOfMiMCTestcase)
	runTestListGnark(t, "fixed-permutation", testtools.ListOfFixedPermutationTestcasePositive)
}

func runTestList[T testtools.Testcase](t *testing.T, prefix string, list []T) {

	t.Run(prefix, func(t *testing.T) {
		for _, tc := range list {
			t.Run(tc.Name(), func(t *testing.T) {
				testtools.RunTestcase(t, tc, totalSuite)
			})
		}
	})
}

func runTestListGnark[T testtools.Testcase](t *testing.T, prefix string, list []T) {

	t.Run(prefix, func(t *testing.T) {
		for _, tc := range list {
			t.Run(tc.Name(), func(t *testing.T) {
				testtools.RunTestShouldPassWithGnark(t, tc, totalSuite)
			})
		}
	})
}
