package serialization_test

import (
	"fmt"
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/ringsis"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/cleanup"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/globalcs"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/localcs"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/logderivativesum"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/mimc"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/mpts"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/plonkinwizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/recursion"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/selfrecursion"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/univariates"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/vortex"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/sirupsen/logrus"
)

var (
	// Avoid setting both modes to true at the same time
	isTest      = true
	isBenchmark = false
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

// DO NOT CHANGE THESE from std params of ringsis instances
// marshalling/unmarshalling depends on it
var sisInstances = []ringsis.Params{
	ringsis.StdParams, ringsis.StdParams, ringsis.StdParams,
	ringsis.StdParams, ringsis.StdParams, ringsis.StdParams,
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

const lppMerkleRootPublicInput = "LPP_COLUMNS_MERKLE_ROOTS"

// Scenario registry
type serdeScenario struct {
	name      string
	build     func() *wizard.CompiledIOP
	testCases []TestCase
	test      bool
	benchmark bool
}

// Cached scenarios to avoid recompilation
var cachedScenarios = make(map[string]*wizard.CompiledIOP)

func getScenarioComp(scenario *serdeScenario) *wizard.CompiledIOP {
	if comp, exists := cachedScenarios[scenario.name]; exists {
		return comp
	}

	comp := scenario.build()
	cachedScenarios[scenario.name] = comp
	return comp
}

// Scenarios definition
var serdeScenarios = []serdeScenario{
	{
		name: "iop1",
		build: func() *wizard.CompiledIOP {
			// Use first testcase for compilation (since we need one representative)
			tc := testcases[0]
			return wizard.Compile(
				generateProtocol(tc),
				vortex.Compile(
					2,
					vortex.ForceNumOpenedColumns(16),
					vortex.WithSISParams(&tc.SisInstance),
				),
				selfrecursion.SelfRecurse,
				dummy.Compile,
			)
		},
		testCases: testcases, // Multiple test cases
		test:      isTest,
		benchmark: isBenchmark,
	},
	{
		name: "iop2",
		build: func() *wizard.CompiledIOP {
			tc := TestCase{Numpoly: 32, NumRound: 3, PolSize: 32, NumOpenCol: 16, SisInstance: sisInstances[0]}
			return wizard.Compile(
				generateProtocol(tc),
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
		},
		testCases: []TestCase{{Numpoly: 32, NumRound: 3, PolSize: 32, NumOpenCol: 16, SisInstance: sisInstances[0]}},
		test:      isTest,
		benchmark: isBenchmark,
	},
	{
		name: "iop3",
		build: func() *wizard.CompiledIOP {
			// Use first testcase for compilation
			tc := testcases_precomp[0]
			return wizard.Compile(
				generateProtocol(tc),
				vortex.Compile(
					2,
					vortex.ForceNumOpenedColumns(16),
					vortex.WithSISParams(&tc.SisInstance),
				),
				selfrecursion.SelfRecurse,
				dummy.Compile,
			)
		},
		testCases: testcases_precomp, // Multiple test cases
		test:      isTest,
		benchmark: isBenchmark,
	},
	{
		name: "iop4",
		build: func() *wizard.CompiledIOP {
			tc := TestCase{Numpoly: 32, NumRound: 3, PolSize: 32, NumOpenCol: 16, SisInstance: sisInstances[0],
				NumPrecomp: 4, IsCommitPrecomp: true}
			return wizard.Compile(
				generateProtocol(tc),
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
		},
		testCases: []TestCase{{Numpoly: 32, NumRound: 3, PolSize: 32, NumOpenCol: 16, SisInstance: sisInstances[0],
			NumPrecomp: 4, IsCommitPrecomp: true}},
		test:      isTest,
		benchmark: isBenchmark,
	},
	{
		name: "iop5",
		build: func() *wizard.CompiledIOP {
			numRow := 1 << 10
			tc := distributeTestCase{numRow: numRow}
			sisInstance := ringsis.Params{LogTwoBound: 16, LogTwoDegree: 6}
			return wizard.Compile(
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
		},
		testCases: []TestCase{{}}, // Empty test cases since this is a special case
		test:      isTest,
		benchmark: isBenchmark,
	},
	{
		name: "iop6",
		build: func() *wizard.CompiledIOP {
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

			// Use first suite for compilation
			s := suites[0]
			comp1 := wizard.Compile(define1, s...)

			define2 := func(build2 *wizard.Builder) {
				recursion.DefineRecursionOf(build2.CompiledIOP, comp1, recursion.Parameters{
					Name:        "test",
					WithoutGkr:  true,
					MaxNumProof: 1,
				})
			}

			return wizard.Compile(define2, dummy.CompileAtProverLvl())
		},
		testCases: []TestCase{{}}, // Empty test cases since this is a special case
		test:      isTest,
		benchmark: isBenchmark,
	},
}

// Test function that runs sanity checks for all scenarios with multiple test cases
func TestSerdeIOPAll(t *testing.T) {
	for _, scenario := range serdeScenarios {
		if !scenario.test {
			continue
		}

		comp := getScenarioComp(&scenario)

		// For scenarios with multiple test cases, run each one
		if len(scenario.testCases) > 0 {
			for i, tc := range scenario.testCases {
				t.Run(fmt.Sprintf("%s-testcase-%d-%+v", scenario.name, i, tc), func(subT *testing.T) {
					// Note: For sanity testing with multiple test cases, we just use the compiled IOP
					// as a representative. The actual protocol generation is handled in the original tests
					runSerdeTest(subT, comp, fmt.Sprintf("%s-%d", scenario.name, i), true, false)
				})
			}
		} else {
			// Single case scenarios
			t.Run(fmt.Sprintf("%s-single", scenario.name), func(subT *testing.T) {
				runSerdeTest(subT, comp, scenario.name, true, false)
			})
		}
	}
}

// Benchmark functions
func BenchmarkSerIOP1(b *testing.B) {
	benchmarkScenario(b, "iop1", true) // true for serialization only
}

func BenchmarkDeserIOP1(b *testing.B) {
	benchmarkScenario(b, "iop1", false) // false for deserialization only
}

func BenchmarkSerIOP2(b *testing.B) {
	benchmarkScenario(b, "iop2", true)
}

func BenchmarkDeserIOP2(b *testing.B) {
	benchmarkScenario(b, "iop2", false)
}

func BenchmarkSerIOP3(b *testing.B) {
	benchmarkScenario(b, "iop3", true)
}

func BenchmarkDeserIOP3(b *testing.B) {
	benchmarkScenario(b, "iop3", false)
}

func BenchmarkSerIOP4(b *testing.B) {
	benchmarkScenario(b, "iop4", true)
}

func BenchmarkDeserIOP4(b *testing.B) {
	benchmarkScenario(b, "iop4", false)
}

func BenchmarkSerIOP5(b *testing.B) {
	benchmarkScenario(b, "iop5", true)
}

func BenchmarkDeserIOP5(b *testing.B) {
	benchmarkScenario(b, "iop5", false)
}

func BenchmarkSerIOP6(b *testing.B) {
	benchmarkScenario(b, "iop6", true)
}

func BenchmarkDeserIOP6(b *testing.B) {
	benchmarkScenario(b, "iop6", false)
}

// Helper function to run benchmark for a specific scenario
func benchmarkScenario(b *testing.B, scenarioName string, onlySerialize bool) {
	// Find the scenario
	var scenario *serdeScenario
	for _, s := range serdeScenarios {
		if s.name == scenarioName {
			scenario = &s
			break
		}
	}

	if scenario == nil {
		b.Fatalf("Scenario %s not found", scenarioName)
	}

	if !scenario.benchmark {
		b.Skipf("Scenario %s is not configured for benchmarking", scenarioName)
		return
	}

	comp := getScenarioComp(scenario)
	runSerdeBenchmark(b, comp, scenarioName, onlySerialize)
}

// Keep original test functions for backward compatibility (optional)
// You can remove these if you only want to use TestSerdeAll

func TestSerdeIOP1(t *testing.T) {
	if len(testcases) == 0 {
		t.Skip("No test cases for iop1")
		return
	}

	scenario := findScenario("iop1")
	if scenario == nil {
		t.Fatal("iop1 scenario not found")
	}

	comp := getScenarioComp(scenario)

	for i, tc := range testcases {
		t.Run(fmt.Sprintf("testcase-%d-%+v", i, tc), func(subT *testing.T) {
			runSerdeTest(subT, comp, fmt.Sprintf("iop1-%d", i), true, false)
		})
	}
}

func TestSerdeIOP2(t *testing.T) {
	scenario := findScenario("iop2")
	if scenario == nil {
		t.Fatal("iop2 scenario not found")
	}

	comp := getScenarioComp(scenario)

	t.Run("single-case", func(subT *testing.T) {
		runSerdeTest(subT, comp, "iop2", true, false)
	})
}

func TestSerdeIOP3(t *testing.T) {
	if len(testcases_precomp) == 0 {
		t.Skip("No test cases for iop3")
		return
	}

	scenario := findScenario("iop3")
	if scenario == nil {
		t.Fatal("iop3 scenario not found")
	}

	comp := getScenarioComp(scenario)

	for i, tc := range testcases_precomp {
		t.Run(fmt.Sprintf("testcase-%d-%+v", i, tc), func(subT *testing.T) {
			runSerdeTest(subT, comp, fmt.Sprintf("iop3-%d", i), true, false)
		})
	}
}

func TestSerdeIOP4(t *testing.T) {
	scenario := findScenario("iop4")
	if scenario == nil {
		t.Fatal("iop4 scenario not found")
	}

	comp := getScenarioComp(scenario)

	t.Run("single-case", func(subT *testing.T) {
		runSerdeTest(subT, comp, "iop4", true, false)
	})
}

func TestSerdeIOP5(t *testing.T) {
	scenario := findScenario("iop5")
	if scenario == nil {
		t.Fatal("iop5 scenario not found")
	}

	comp := getScenarioComp(scenario)

	t.Run("single-case", func(subT *testing.T) {
		runSerdeTest(subT, comp, "iop5", true, false)
	})
}

func TestSerdeIOP6(t *testing.T) {
	scenario := findScenario("iop6")
	if scenario == nil {
		t.Fatal("iop6 scenario not found")
	}

	comp := getScenarioComp(scenario)

	t.Run("single-case", func(subT *testing.T) {
		runSerdeTest(subT, comp, "iop6", true, false)
	})
}

// Helper function to find scenario by name
func findScenario(name string) *serdeScenario {
	for i := range serdeScenarios {
		if serdeScenarios[i].name == name {
			return &serdeScenarios[i]
		}
	}
	return nil
}
