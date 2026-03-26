//go:build !fuzzlight

package selfrecursion_test

import (
	"fmt"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/test"
	"github.com/consensys/linea-monorepo/prover/backend/files"
	"github.com/consensys/linea-monorepo/prover/crypto/ringsis"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/logdata"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/poseidon2"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/selfrecursion"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/vortex"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/serde"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
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
	{Numpoly: 2, NumRound: 1, PolSize: 2, NumOpenCol: 1, SisInstance: sisInstances[0]},
	{Numpoly: 1024, NumRound: 2, PolSize: 32, NumOpenCol: 16, SisInstance: sisInstances[0]},
	{Numpoly: 2, NumRound: 2, PolSize: 32, NumOpenCol: 2, SisInstance: sisInstances[0]},
	{Numpoly: 1024, NumRound: 3, PolSize: 32, NumOpenCol: 16, SisInstance: sisInstances[0]},
	{Numpoly: 1024, NumRound: 1, PolSize: 16, NumOpenCol: 16, SisInstance: sisInstances[1]},
	{Numpoly: 1024, NumRound: 3, PolSize: 32, NumOpenCol: 16, SisInstance: sisInstances[2]},
	{Numpoly: 27, NumRound: 1, PolSize: 32, NumOpenCol: 8, SisInstance: sisInstances[0]},
	{Numpoly: 1024, NumRound: 1, PolSize: 32, NumOpenCol: 16, SisInstance: sisInstances[3]},
	{Numpoly: 27, NumRound: 3, PolSize: 32, NumOpenCol: 16, SisInstance: sisInstances[3]},
	{Numpoly: 29, NumRound: 1, PolSize: 32, NumOpenCol: 16, SisInstance: sisInstances[3]},
}

var testcases_precomp []TestCase = []TestCase{
	{Numpoly: 2, NumRound: 1, PolSize: 2, NumOpenCol: 1, SisInstance: sisInstances[0], NumPrecomp: 2, IsCommitPrecomp: true},
	{Numpoly: 1024, NumRound: 2, PolSize: 32, NumOpenCol: 16, SisInstance: sisInstances[0], NumPrecomp: 4, IsCommitPrecomp: true},
	{Numpoly: 2, NumRound: 2, PolSize: 32, NumOpenCol: 2, SisInstance: sisInstances[0], NumPrecomp: 2, IsCommitPrecomp: true},
	{Numpoly: 1024, NumRound: 3, PolSize: 32, NumOpenCol: 16, SisInstance: sisInstances[0], NumPrecomp: 4, IsCommitPrecomp: true},
	{Numpoly: 1024, NumRound: 1, PolSize: 16, NumOpenCol: 16, SisInstance: sisInstances[1], NumPrecomp: 4, IsCommitPrecomp: true},
	{Numpoly: 1024, NumRound: 3, PolSize: 32, NumOpenCol: 16, SisInstance: sisInstances[2], NumPrecomp: 4, IsCommitPrecomp: true},
	{Numpoly: 27, NumRound: 1, PolSize: 32, NumOpenCol: 8, SisInstance: sisInstances[0], NumPrecomp: 4, IsCommitPrecomp: true},
	{Numpoly: 1024, NumRound: 1, PolSize: 32, NumOpenCol: 16, SisInstance: sisInstances[3], NumPrecomp: 4, IsCommitPrecomp: true},
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
				b.RegisterRandomCoin(dummyCoinName(round), coin.FieldExt)
			}
		}

		b.UnivariateEval(QNAME, cols...)
	}

	// the prove function assignes the univariate evaluation
	// and the columns with random values
	prove = func(run *wizard.ProverRuntime) {
		// the evaluation point
		x := fext.RandomElement()
		var ys []fext.Element
		if tc.IsCommitPrecomp {
			ys = make([]fext.Element, (tc.Numpoly + tc.NumPrecomp))
		} else {
			ys = make([]fext.Element, tc.Numpoly)
		}
		numColPerRound := tc.Numpoly / tc.NumRound

		// Handle the precomputeds at the beginning
		if tc.IsCommitPrecomp {
			for i := 0; i < tc.NumPrecomp; i++ {
				p := run.Spec.Precomputed.MustGet(precompColName(i))
				ys[i] = smartvectors.EvaluateBasePolyLagrange(p, x)
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
				ys[i] = smartvectors.EvaluateBasePolyLagrange(v, x)
			}

			if round < tc.NumRound-1 {
				_ = run.GetRandomCoinFieldExt(dummyCoinName(round))
			}
		}

		run.AssignUnivariateExt(QNAME, x, ys...)
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
					false,
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
				false,
				vortex.ForceNumOpenedColumns(tc.NumOpenCol),
				vortex.WithSISParams(&tc.SisInstance),
			),
			selfrecursion.SelfRecurse,
			poseidon2.CompilePoseidon2,
			compiler.Arcane(
				compiler.WithTargetColSize(1<<10)),
			vortex.Compile(
				2,
				false,
				vortex.ForceNumOpenedColumns(tc.NumOpenCol),
				vortex.WithSISParams(&tc.SisInstance),
			),
			selfrecursion.SelfRecurse,
			poseidon2.CompilePoseidon2,
			compiler.Arcane(
				compiler.WithTargetColSize(1<<13)),
			vortex.Compile(
				2,
				false,
				vortex.ForceNumOpenedColumns(tc.NumOpenCol),
				vortex.WithSISParams(&tc.SisInstance),
			),
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
					false,
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
				false,
				vortex.ForceNumOpenedColumns(16),
				vortex.WithSISParams(&tc.SisInstance),
			),
			selfrecursion.SelfRecurse,
			poseidon2.CompilePoseidon2,
			compiler.Arcane(
				compiler.WithTargetColSize(1<<10)),
			vortex.Compile(
				2,
				false,
				vortex.ForceNumOpenedColumns(16),
				vortex.WithSISParams(&tc.SisInstance),
			),
			selfrecursion.SelfRecurse,
			poseidon2.CompilePoseidon2,
			compiler.Arcane(
				compiler.WithTargetColSize(1<<13)),
			vortex.Compile(
				2,
				false,
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
	// don't increase too much so that it does not increase too much the runtime
	// of the test.
	n := 2

	comp := wizard.Compile(
		define,
		vortex.Compile(
			8,
			false,
			vortex.ForceNumOpenedColumns(32),
			vortex.WithSISParams(&ringsis.StdParams),
			vortex.WithOptionalSISHashingThreshold(64),
		),
	)

	for i := 0; i < n; i++ {

		fmt.Printf("layer %v\n", i)

		comp = wizard.ContinueCompilation(
			comp,
			selfrecursion.SelfRecurse,
			poseidon2.CompilePoseidon2,
			compiler.Arcane(
				compiler.WithTargetColSize(1<<13),
			),
			// logdata.Log("before-vortex"),
			// logdata.GenCSV(files.MustOverwrite(fmt.Sprintf("selfrecursion-%v.csv", i)), logdata.IncludeAllFilter),
			vortex.Compile(
				8,
				false,
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

func TestGnarkSelfRecursionManyLayers(t *testing.T) {
	const isBLS = true

	define, prove := generateProtocol(testcases[0])
	// don't increase too much so that it does not increase too much the runtime
	// of the test.
	n := 1 // TODO@yao: set back to 2?

	comp := wizard.Compile(
		define,
		vortex.Compile(
			2,
			false,
			vortex.ForceNumOpenedColumns(16),
			vortex.WithSISParams(&ringsis.StdParams),
			vortex.WithOptionalSISHashingThreshold(64),
		),
	)

	for i := 0; i < n; i++ {

		fmt.Printf("layer %v\n", i)

		if i == n-1 {
			comp = wizard.ContinueCompilation(
				comp,
				selfrecursion.SelfRecurse,
				poseidon2.CompilePoseidon2,
				compiler.Arcane(
					compiler.WithTargetColSize(1<<9),
				),
				// logdata.Log("before-vortex"),
				logdata.GenCSV(files.MustOverwrite(fmt.Sprintf("selfrecursion-%v.csv", i)), logdata.IncludeAllFilter),
				vortex.Compile(
					2,
					isBLS,
					vortex.ForceNumOpenedColumns(16),
					vortex.WithOptionalSISHashingThreshold(1<<20),
				),
			)
		} else {
			comp = wizard.ContinueCompilation(
				comp,
				selfrecursion.SelfRecurse,
				poseidon2.CompilePoseidon2,
				compiler.Arcane(
					compiler.WithTargetColSize(1<<9),
				),
				// logdata.Log("before-vortex"),
				logdata.GenCSV(files.MustOverwrite(fmt.Sprintf("selfrecursion-%v.csv", i)), logdata.IncludeAllFilter),
				vortex.Compile(
					2,
					false,
					vortex.ForceNumOpenedColumns(16),
					vortex.WithSISParams(&ringsis.StdParams),
					vortex.WithOptionalSISHashingThreshold(64),
				),
			)
		}
	}

	proof := wizard.Prove(comp, prove, isBLS)
	err := wizard.Verify(comp, proof, isBLS)
	require.NoError(t, err)

	circuit := verifierCircuit{}
	{
		c := wizard.AllocateWizardCircuit(comp, comp.NumRounds(), isBLS)
		circuit.C = *c
	}

	csc, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &circuit, frontend.IgnoreUnconstrainedInputs())

	if err != nil {
		t.Fatal(err)
	}

	assignment := &verifierCircuit{
		C: *wizard.AssignVerifierCircuit(comp, proof, comp.NumRounds(), isBLS),
	}

	witness, err := frontend.NewWitness(assignment, ecc.BLS12_377.ScalarField())
	if err != nil {
		t.Fatal(err)
	}

	// Check if solved using the pre-compiled SCS
	err = csc.IsSolved(witness)
	if err != nil {
		// When the error string is too large `require.NoError` does not print
		// the error.
		t.Logf("circuit solving failed : %v. Retrying with test engine\n", err)

		errDetail := test.IsSolved(
			assignment,
			assignment,
			csc.Field(),
		)

		t.Logf("while running the plonk prover: %v", errDetail)

		t.FailNow()
	}

}

type verifierCircuit struct {
	C wizard.VerifierCircuit
}

func (c *verifierCircuit) Define(api frontend.API) error {
	c.C.Verify(api)
	return nil
}

// // Test the compiler of self-recursion with really many layers for a sample
// // dummy protocol.
func TestSelfRecursionManyLayersWithSerde(t *testing.T) {

	define, _ := generateProtocol(testcases[0])
	n := 1

	comp := wizard.Compile(
		define,
		vortex.Compile(
			2,
			false,
			vortex.ForceNumOpenedColumns(16),
			vortex.WithSISParams(&ringsis.StdParams),
			vortex.WithOptionalSISHashingThreshold(64),
		),
	)

	for i := 0; i < n; i++ {

		//fmt.Printf("layer %v\n", i)

		if i == n-1 {
			comp = wizard.ContinueCompilation(
				comp,
				selfrecursion.SelfRecurse,
				poseidon2.CompilePoseidon2,
				compiler.Arcane(
					compiler.WithTargetColSize(1<<9),
				),
				// logdata.Log("before-vortex"),
				//logdata.GenCSV(files.MustOverwrite(fmt.Sprintf("selfrecursion-%v.csv", i)), logdata.IncludeAllFilter),
				vortex.Compile(
					2,
					true,
					vortex.ForceNumOpenedColumns(16),
					vortex.WithOptionalSISHashingThreshold(1<<20),
				),
			)
		} else {
			comp = wizard.ContinueCompilation(
				comp,
				selfrecursion.SelfRecurse,
				poseidon2.CompilePoseidon2,
				compiler.Arcane(
					compiler.WithTargetColSize(1<<9),
				),
				// logdata.Log("before-vortex"),
				//logdata.GenCSV(files.MustOverwrite(fmt.Sprintf("selfrecursion-%v.csv", i)), logdata.IncludeAllFilter),
				vortex.Compile(
					2,
					false,
					vortex.ForceNumOpenedColumns(16),
					vortex.WithSISParams(&ringsis.StdParams),
					vortex.WithOptionalSISHashingThreshold(64),
				),
			)
		}
	}

	buf, err := serde.Serialize(comp)
	if err != nil {
		t.Fatal(err)
	}

	logrus.Info("serialized succesfully.")

	comp2 := &wizard.CompiledIOP{}
	err = serde.Deserialize(buf, &comp2)
	if err != nil {
		t.Fatal(err)
	}

	if !serde.DeepCmp(comp, comp2, true) {
		t.Fatal("comp and comp2 are not equal")
	}
}
