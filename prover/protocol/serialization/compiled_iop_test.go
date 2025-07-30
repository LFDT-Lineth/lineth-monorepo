package serialization_test

import (
	"fmt"
	"testing"

	"github.com/consensys/linea-monorepo/prover/crypto/ringsis"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/cleanup"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/globalcs"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/localcs"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/logderivativesum"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/mimc"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/mpts"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/plonkinwizard"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/recursion"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/selfrecursion"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/univariates"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/vortex"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/sirupsen/logrus"
)

// returns a dummy column name
func dummyColName(i int) ifaces.ColID {
	return ifaces.ColIDf("POLY_%v", i)
}

// returns a precomputed column name
func precompColName(i int) ifaces.ColID {
	return ifaces.ColIDf("PRE_COMP_POLY_%v", i)
}

// returns a dummy column name
func dummyCoinName(i int) coin.Name {
	return coin.Namef("A%v", i)
}

// name of the evaluation query
const QNAME ifaces.QueryID = "EVAL"

// sis instances
var sisInstances = []ringsis.Params{
	{LogTwoBound: 8, LogTwoDegree: 1},
	{LogTwoBound: 8, LogTwoDegree: 2},
	{LogTwoBound: 8, LogTwoDegree: 3},
	{LogTwoBound: 8, LogTwoDegree: 6},
	{LogTwoBound: 8, LogTwoDegree: 5},
	{LogTwoBound: 16, LogTwoDegree: 6},
}

// testcase type
type TestCase struct {
	Numpoly, NumRound, PolSize, NumOpenCol, NumPrecomp int
	SisInstance                                        ringsis.Params
	IsCommitPrecomp                                    bool
}

// tests-cases for all tests
var testcases []TestCase = []TestCase{
	{Numpoly: 32, NumRound: 1, PolSize: 32, NumOpenCol: 16, SisInstance: sisInstances[0]},
	{Numpoly: 32, NumRound: 2, PolSize: 32, NumOpenCol: 16, SisInstance: sisInstances[0]},
	{Numpoly: 2, NumRound: 2, PolSize: 32, NumOpenCol: 2, SisInstance: sisInstances[0]},
	{Numpoly: 32, NumRound: 3, PolSize: 32, NumOpenCol: 16, SisInstance: sisInstances[0]},
	{Numpoly: 32, NumRound: 1, PolSize: 16, NumOpenCol: 16, SisInstance: sisInstances[1]},
	{Numpoly: 32, NumRound: 3, PolSize: 32, NumOpenCol: 16, SisInstance: sisInstances[2]},
	{Numpoly: 27, NumRound: 1, PolSize: 32, NumOpenCol: 8, SisInstance: sisInstances[0]},
	{Numpoly: 32, NumRound: 1, PolSize: 32, NumOpenCol: 16, SisInstance: sisInstances[3]},
	{Numpoly: 27, NumRound: 3, PolSize: 32, NumOpenCol: 16, SisInstance: sisInstances[3]},
	{Numpoly: 29, NumRound: 1, PolSize: 32, NumOpenCol: 16, SisInstance: sisInstances[3]},
}

var testcases_precomp []TestCase = []TestCase{
	{Numpoly: 32, NumRound: 1, PolSize: 32, NumOpenCol: 16, SisInstance: sisInstances[0], NumPrecomp: 4, IsCommitPrecomp: true},
	{Numpoly: 32, NumRound: 2, PolSize: 32, NumOpenCol: 16, SisInstance: sisInstances[0], NumPrecomp: 4, IsCommitPrecomp: true},
	{Numpoly: 2, NumRound: 2, PolSize: 32, NumOpenCol: 2, SisInstance: sisInstances[0], NumPrecomp: 2, IsCommitPrecomp: true},
	{Numpoly: 32, NumRound: 3, PolSize: 32, NumOpenCol: 16, SisInstance: sisInstances[0], NumPrecomp: 4, IsCommitPrecomp: true},
	{Numpoly: 32, NumRound: 1, PolSize: 16, NumOpenCol: 16, SisInstance: sisInstances[1], NumPrecomp: 4, IsCommitPrecomp: true},
	{Numpoly: 32, NumRound: 3, PolSize: 32, NumOpenCol: 16, SisInstance: sisInstances[2], NumPrecomp: 4, IsCommitPrecomp: true},
	{Numpoly: 27, NumRound: 1, PolSize: 32, NumOpenCol: 8, SisInstance: sisInstances[0], NumPrecomp: 4, IsCommitPrecomp: true},
	{Numpoly: 32, NumRound: 1, PolSize: 32, NumOpenCol: 16, SisInstance: sisInstances[3], NumPrecomp: 4, IsCommitPrecomp: true},
	{Numpoly: 27, NumRound: 3, PolSize: 32, NumOpenCol: 16, SisInstance: sisInstances[3], NumPrecomp: 4, IsCommitPrecomp: true},
	{Numpoly: 29, NumRound: 1, PolSize: 32, NumOpenCol: 16, SisInstance: sisInstances[3], NumPrecomp: 4, IsCommitPrecomp: true},
}

// generate a testcase protocol with given parameters
func generateProtocol(tc TestCase) (define func(*wizard.Builder)) {

	// the define function creates a dummy protocol
	// with only univariate evaluations
	define = func(b *wizard.Builder) {
		var cols []ifaces.Column
		if tc.IsCommitPrecomp {
			cols = make([]ifaces.Column, (tc.Numpoly + tc.NumPrecomp))
		} else {
			cols = make([]ifaces.Column, tc.Numpoly)
		}
		numColPerRound := tc.Numpoly / tc.NumRound
		// register the precomputed polynomials at the beginning of cols
		if tc.IsCommitPrecomp {
			for i := 0; i < tc.NumPrecomp; i++ {
				p := smartvectors.Rand(tc.PolSize)
				cols[i] = b.RegisterPrecomputed(precompColName(i), p)
			}
		}
		logrus.Printf("Registered precomp polynomials")
		for round := 0; round < tc.NumRound; round++ {
			logrus.Printf("round = %d", round)
			// determine which columns should be declared for each round
			start, stop := round*numColPerRound, (round+1)*numColPerRound
			// Consider the precomputed polys
			if tc.IsCommitPrecomp {
				start, stop = round*numColPerRound+tc.NumPrecomp, (round+1)*numColPerRound+tc.NumPrecomp
			}
			if round == tc.NumRound-1 && tc.IsCommitPrecomp {
				stop = tc.Numpoly + tc.NumPrecomp
			}
			if round == tc.NumRound-1 && !tc.IsCommitPrecomp {
				stop = tc.Numpoly
			}

			for i := start; i < stop; i++ {
				cols[i] = b.RegisterCommit(dummyColName(i), tc.PolSize)
			}

			if round < tc.NumRound-1 {
				b.RegisterRandomCoin(dummyCoinName(round), coin.Field)
			}
		}

		b.UnivariateEval(QNAME, cols...)
	}

	return define
}

func TestSerdeIOP1(t *testing.T) {

	for _, tc := range testcases {
		t.Run(fmt.Sprintf("testcase-%++v", tc), func(subT *testing.T) {
			define := generateProtocol(tc)
			sisInstances := tc.SisInstance

			comp := wizard.Compile(
				define,
				vortex.Compile(
					2,
					vortex.ForceNumOpenedColumns(16),
					vortex.WithSISParams(&sisInstances),
				),
				selfrecursion.SelfRecurse,
				dummy.Compile,
			)

			runSerdeTest(t, comp, "iop1", true, false)
		})
	}
}

func TestSerdeIOP2(t *testing.T) {

	tc := TestCase{Numpoly: 32, NumRound: 3, PolSize: 32, NumOpenCol: 16, SisInstance: sisInstances[0]}
	t.Run(fmt.Sprintf("testcase-%++v", tc), func(subT *testing.T) {
		define := generateProtocol(tc)

		comp := wizard.Compile(
			define,
			vortex.Compile(
				2,
				vortex.ForceNumOpenedColumns(tc.NumOpenCol),
				vortex.WithSISParams(&tc.SisInstance),
			),
			selfrecursion.SelfRecurse,
			mimc.CompileMiMC,
			compiler.Arcane(
				compiler.WithTargetColSize(1<<10)),
			vortex.Compile(
				2,
				vortex.ForceNumOpenedColumns(tc.NumOpenCol),
				vortex.WithSISParams(&tc.SisInstance),
			),
			selfrecursion.SelfRecurse,
			mimc.CompileMiMC,
			compiler.Arcane(
				compiler.WithTargetColSize(1<<13)),
			vortex.Compile(
				2,
				vortex.ForceNumOpenedColumns(tc.NumOpenCol),
				vortex.WithSISParams(&tc.SisInstance),
			),
			dummy.Compile,
		)

		runSerdeTest(t, comp, "iop2", true, false)
	})
}

// Test for committing to the precomputed polynomials
func TestSerdeIOP3(t *testing.T) {

	for _, tc := range testcases_precomp {
		t.Run(fmt.Sprintf("testcase-%++v", tc), func(subT *testing.T) {
			define := generateProtocol(tc)
			sisInstances := tc.SisInstance
			comp := wizard.Compile(
				define,
				vortex.Compile(
					2,
					vortex.ForceNumOpenedColumns(16),
					vortex.WithSISParams(&sisInstances),
				),
				selfrecursion.SelfRecurse,
				dummy.Compile,
			)

			runSerdeTest(t, comp, "iop3", true, false)
		})
	}
}

// Test for precomputed polys with multilayered self recursion
func TestSerdeIOP4(t *testing.T) {

	logrus.SetLevel(logrus.FatalLevel)

	tc := TestCase{Numpoly: 32, NumRound: 3, PolSize: 32, NumOpenCol: 16, SisInstance: sisInstances[0],
		NumPrecomp: 4, IsCommitPrecomp: true}
	t.Run(fmt.Sprintf("testcase-%++v", tc), func(subT *testing.T) {
		define := generateProtocol(tc)

		comp := wizard.Compile(
			define,
			vortex.Compile(
				2,
				vortex.ForceNumOpenedColumns(16),
				vortex.WithSISParams(&tc.SisInstance),
			),
			selfrecursion.SelfRecurse,
			mimc.CompileMiMC,
			compiler.Arcane(
				compiler.WithTargetColSize(1<<10)),
			vortex.Compile(
				2,
				vortex.ForceNumOpenedColumns(16),
				vortex.WithSISParams(&tc.SisInstance),
			),
			selfrecursion.SelfRecurse,
			mimc.CompileMiMC,
			compiler.Arcane(
				compiler.WithTargetColSize(1<<13)),
			vortex.Compile(
				2,
				vortex.ForceNumOpenedColumns(16),
				vortex.WithSISParams(&tc.SisInstance),
			),
			dummy.Compile,
		)

		runSerdeTest(t, comp, "iop4", true, false)
	})
}

const lppMerkleRootPublicInput = "LPP_COLUMNS_MERKLE_ROOTS"

func TestSerdeIOP5(t *testing.T) {

	numRow := 1 << 10
	tc := distributeTestCase{numRow: numRow}
	sisInstance := ringsis.Params{LogTwoBound: 16, LogTwoDegree: 6}

	t.Run(fmt.Sprintf("testcase-%++v", tc), func(subT *testing.T) {

		comp := wizard.Compile(
			func(build *wizard.Builder) {
				tc.define(build.CompiledIOP)
			},
			mimc.CompileMiMC,
			plonkinwizard.Compile,
			compiler.Arcane(
				compiler.WithTargetColSize(1<<17),
				compiler.WithDebugMode("conglomeration"),
			),
			vortex.Compile(
				2,
				vortex.ForceNumOpenedColumns(256),
				vortex.WithSISParams(&sisInstance),
				vortex.AddMerkleRootToPublicInputs(lppMerkleRootPublicInput, []int{0}),
			),
			selfrecursion.SelfRecurse,
			cleanup.CleanUp,
			mimc.CompileMiMC,
			compiler.Arcane(
				compiler.WithTargetColSize(1<<15),
			),
			vortex.Compile(
				8,
				vortex.ForceNumOpenedColumns(64),
				vortex.WithSISParams(&sisInstance),
			),
			selfrecursion.SelfRecurse,
			cleanup.CleanUp,
			mimc.CompileMiMC,
			compiler.Arcane(
				compiler.WithTargetColSize(1<<13),
			),
			vortex.Compile(
				8,
				vortex.ForceNumOpenedColumns(64),
				vortex.WithOptionalSISHashingThreshold(1<<20),
			),
		)

		runSerdeTest(t, comp, "iop5", true, false)
	})
}

func TestSerdeIOP6(t *testing.T) {

	define1 := func(bui *wizard.Builder) {

		var (
			a = bui.RegisterCommit("A", 8)
			b = bui.RegisterCommit("B", 8)
		)

		bui.Inclusion("Q", []ifaces.Column{a}, []ifaces.Column{b})
	}

	suites := [][]func(*wizard.CompiledIOP){
		{
			logderivativesum.CompileLookups,
			localcs.Compile,
			globalcs.Compile,
			univariates.Naturalize,
			mpts.Compile(),
			vortex.Compile(
				2,
				vortex.ForceNumOpenedColumns(4),
				vortex.WithSISParams(&ringsis.StdParams),
				vortex.PremarkAsSelfRecursed(),
				vortex.WithOptionalSISHashingThreshold(0),
			),
		},
	}

	for i, s := range suites {

		t.Run(fmt.Sprintf("case-%v", i), func(t *testing.T) {
			comp1 := wizard.Compile(define1, s...)
			runSerdeTest(t, comp1, fmt.Sprintf("iop6-recursion-comp1-%v", i), true, false)
			define2 := func(build2 *wizard.Builder) {
				recursion.DefineRecursionOf(build2.CompiledIOP, comp1, recursion.Parameters{
					Name:        "test",
					WithoutGkr:  true,
					MaxNumProof: 1,
				})
			}
			comp2 := wizard.Compile(define2, dummy.CompileAtProverLvl())
			runSerdeTest(t, comp2, fmt.Sprintf("iop6-recursion-comp2-%v", i), true, false)
		})
	}
}
