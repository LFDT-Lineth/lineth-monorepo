//go:build !fuzzlight

package selfrecursion_test

import (
	"fmt"
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/backend/files"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/ringsis"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/logdata"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/mimc"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/selfrecursion"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/vortex"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/serialization"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
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
func generateProtocol(tc TestCase) (define func(*wizard.Builder), prove func(*wizard.ProverRuntime)) {

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

	// the prove function assignes the univariate evaluation
	// and the columns with random values
	prove = func(run *wizard.ProverRuntime) {
		// the evaluation point
		x := field.NewElement(42)
		var ys []field.Element
		if tc.IsCommitPrecomp {
			ys = make([]field.Element, (tc.Numpoly + tc.NumPrecomp))
		} else {
			ys = make([]field.Element, tc.Numpoly)
		}
		numColPerRound := tc.Numpoly / tc.NumRound

		// Handle the precomputeds at the beginning
		if tc.IsCommitPrecomp {
			for i := 0; i < tc.NumPrecomp; i++ {
				p := run.Spec.Precomputed.MustGet(precompColName(i))
				ys[i] = smartvectors.Interpolate(p, x)
			}
		}

		for round := 0; round < tc.NumRound; round++ {
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

			// assigns each column to a random value and evalutes it at x
			for i := start; i < stop; i++ {
				v := smartvectors.Rand(tc.PolSize)
				run.AssignColumn(dummyColName(i), v)
				ys[i] = smartvectors.Interpolate(v, x)
			}

			if round < tc.NumRound-1 {
				_ = run.GetRandomCoinField(dummyCoinName(round))
			}
		}

		run.AssignUnivariate(QNAME, x, ys...)
	}
	return define, prove
}

func TestSelfRecursionRandom(t *testing.T) {

	logrus.SetLevel(logrus.FatalLevel)

	for _, tc := range testcases {
		t.Run(fmt.Sprintf("testcase-%++v", tc), func(subT *testing.T) {
			define, prove := generateProtocol(tc)
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

			proof := wizard.Prove(
				comp,
				prove,
			)

			err := wizard.Verify(comp, proof)
			require.NoError(subT, err)
		})
	}
}

func TestSelfRecursionMultiLayered(t *testing.T) {

	logrus.SetLevel(logrus.FatalLevel)

	tc := TestCase{Numpoly: 32, NumRound: 3, PolSize: 32, NumOpenCol: 16, SisInstance: sisInstances[0]}
	t.Run(fmt.Sprintf("testcase-%++v", tc), func(subT *testing.T) {
		define, prove := generateProtocol(tc)

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

		proof := wizard.Prove(
			comp,
			prove,
		)

		err := wizard.Verify(comp, proof)
		require.NoError(subT, err)
	})
}

// Test for committing to the precomputed polynomials
func TestSelfRecursionCommitPrecomputed(t *testing.T) {

	logrus.SetLevel(logrus.FatalLevel)

	for _, tc := range testcases_precomp {
		t.Run(fmt.Sprintf("testcase-%++v", tc), func(subT *testing.T) {
			define, prove := generateProtocol(tc)
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

			proof := wizard.Prove(
				comp,
				prove,
			)

			err := wizard.Verify(comp, proof)
			require.NoError(subT, err)
		})
	}
}

// Test for precomputed polys with multilayered self recursion
func TestSelfRecursionPrecompMultiLayered(t *testing.T) {

	logrus.SetLevel(logrus.FatalLevel)

	tc := TestCase{Numpoly: 32, NumRound: 3, PolSize: 32, NumOpenCol: 16, SisInstance: sisInstances[0],
		NumPrecomp: 4, IsCommitPrecomp: true}
	t.Run(fmt.Sprintf("testcase-%++v", tc), func(subT *testing.T) {
		define, prove := generateProtocol(tc)

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

		proof := wizard.Prove(
			comp,
			prove,
		)

		err := wizard.Verify(comp, proof)
		require.NoError(subT, err)
	})
}

// Test the compiler of self-recursion with really many layers for a sample
// dummy protocol.
func TestSelfRecursionManyLayers(t *testing.T) {

	define, prove := generateProtocol(testcases[0])
	n := 6

	comp := wizard.Compile(
		define,
		vortex.Compile(
			8,
			vortex.ForceNumOpenedColumns(32),
			vortex.WithSISParams(&ringsis.StdParams),
			vortex.WithOptionalSISHashingThreshold(64),
		),
	)

	for i := 0; i < n; i++ {
		comp = wizard.ContinueCompilation(
			comp,
			selfrecursion.SelfRecurse,
			mimc.CompileMiMC,
			compiler.Arcane(
				compiler.WithTargetColSize(1<<13),
			),
			// logdata.Log("before-vortex"),
			logdata.GenCSV(files.MustOverwrite(fmt.Sprintf("selfrecursion-%v.csv", i)), logdata.IncludeAllFilter),
			vortex.Compile(
				8,
				vortex.ForceNumOpenedColumns(32),
				vortex.WithSISParams(&ringsis.StdParams),
				vortex.WithOptionalSISHashingThreshold(64),
			),
		)
	}

	proof := wizard.Prove(comp, prove)
	err := wizard.Verify(comp, proof)
	require.NoError(t, err)
}

// Test the compiler of self-recursion with really many layers for a sample
// dummy protocol.
func TestSelfRecursionManyLayersWithSerde(t *testing.T) {

	define, prove := generateProtocol(testcases[0])
	n := 6

	comp := wizard.Compile(
		define,
		vortex.Compile(
			8,
			vortex.ForceNumOpenedColumns(32),
			vortex.WithSISParams(&ringsis.StdParams),
			vortex.WithOptionalSISHashingThreshold(64),
		),
	)

	for i := 0; i < n; i++ {
		comp = wizard.ContinueCompilation(
			comp,
			selfrecursion.SelfRecurse,
			mimc.CompileMiMC,
			compiler.Arcane(
				compiler.WithTargetColSize(1<<13),
			),
			vortex.Compile(
				8,
				vortex.ForceNumOpenedColumns(32),
				vortex.WithSISParams(&ringsis.StdParams),
				vortex.WithOptionalSISHashingThreshold(64),
			),
		)
	}

	buf, err := serialization.Serialize(comp)
	if err != nil {
		t.Fatal(err)
	}

	comp2 := &wizard.CompiledIOP{}
	err = serialization.Deserialize(buf, &comp2)
	if err != nil {
		t.Fatal(err)
	}

	proof := wizard.Prove(comp2, prove)
	err = wizard.Verify(comp2, proof)
	require.NoError(t, err)
}
