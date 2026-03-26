package selfrecursion_test

import (
	"testing"

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
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/sirupsen/logrus"
)

type testCasePerf = struct {
	Explainer string
	Define    func(b *wizard.Builder)
	Prove     func(pr *wizard.ProverRuntime)
}

func testCasePerfGenerator() []testCasePerf {
	var (
		polSize = 1 << 13
		// variables for multi-round
		nPolSize        = 1 << 7 // size of each polynomial
		nPolsMultiRound = []int{nPolSize, nPolSize, nPolSize, nPolSize, nPolSize}
		numRounds       = len(nPolsMultiRound)
		rowsMultiRound  = make([][]ifaces.Column, numRounds)
		// variables for precomputed columns
		numPrecomputed = 20
	)
	tc := []testCasePerf{
		{
			Explainer: "Test Optional SIS Hash Feature With Multi-Round",
			Define: func(b *wizard.Builder) {
				for round := 0; round < numRounds; round++ {
					var offsetIndex = 0
					// trigger the creation of a new round by declaring a dummy coin
					if round != 0 {
						_ = b.RegisterRandomCoin(coin.Namef("COIN_%v", round), coin.Field)
						// Compute the offsetIndex
						for i := 0; i < round; i++ {
							offsetIndex += nPolsMultiRound[i]
						}
					}

					rowsMultiRound[round] = make([]ifaces.Column, nPolsMultiRound[round])
					if round == 0 {
						for i := 0; i < numPrecomputed; i++ {
							p := smartvectors.Rand(polSize)
							rowsMultiRound[round][i] = b.RegisterPrecomputed(ifaces.ColIDf("PRE_COMP_%v", i), p)
						}
						for i := numPrecomputed; i < nPolsMultiRound[round]; i++ {
							rowsMultiRound[round][i] = b.RegisterCommit(ifaces.ColIDf("P_%v", i), polSize)
						}
						continue
					}
					for i := range nPolsMultiRound[round] {
						rowsMultiRound[round][i] = b.RegisterCommit(ifaces.ColIDf("P_%v", offsetIndex+i), polSize)
					}
				}

				b.UnivariateEval("EVAL", utils.Join(rowsMultiRound...)...)
			},
			Prove: func(pr *wizard.ProverRuntime) {
				// Count the total number of polynomials
				numPolys := 0
				for i := range nPolsMultiRound {
					numPolys += nPolsMultiRound[i]
				}
				ys := make([]field.Element, numPolys)
				x := field.NewElement(57) // the evaluation point

				// assign the rows with random polynomials and collect the ys
				for round := range rowsMultiRound {
					var offsetIndex = 0
					if round != 0 {
						// let the prover know that it is free to go to the next
						// round by sampling the coin.
						_ = pr.GetRandomCoinField(coin.Namef("COIN_%v", round))
						// Compute the offsetIndex
						for i := 0; i < round; i++ {
							offsetIndex += nPolsMultiRound[i]
						}
					}

					for i, row := range rowsMultiRound[round] {
						// For round 0 we need (numPolys - numPrecomputeds) polys, as the precomputed are
						// assigned in the define phase
						if i < numPrecomputed && round == 0 {
							p := pr.Spec.Precomputed.MustGet(row.GetColID())
							ys[i] = smartvectors.Interpolate(p, x)
							continue
						}
						p := smartvectors.Rand(polSize)
						ys[offsetIndex+i] = smartvectors.Interpolate(p, x)
						pr.AssignColumn(row.GetColID(), p)
					}
				}

				pr.AssignUnivariate("EVAL", x, ys...)
			},
		},
	}
	return tc
}

// Test with varying forcedNumOpenedColumns and rates.
// We only generate the compiledIop object to obtain
// performance metrics for the Vortex with self recursion.
func TestOptionalSISHashingPerfVarOpenedCols(t *testing.T) {
	// Mute the logs
	// logrus.SetLevel(logrus.FatalLevel)
	var (
		// Params for 100 bits of security
		sisThreshold            = 64
		rate1                   = 2
		forcedNumOpenedColumns1 = 100
		rate2                   = 8
		forcedNumOpenedColumns2 = 34
		targetColSize           = 1 << 16
	)
	testCases := testCasePerfGenerator()
	for _, tc := range testCases {
		t.Run(tc.Explainer, func(t *testing.T) {
			logrus.Infof("Testing %s with SIS threshold 64", tc.Explainer)
			compiled := wizard.Compile(
				tc.Define,
				logdata.Log("Initially before Vortex: "),
				vortex.Compile(
					rate1,
					vortex.ForceNumOpenedColumns(forcedNumOpenedColumns1),
					vortex.WithOptionalSISHashingThreshold(sisThreshold),
				),
				selfrecursion.SelfRecurse,
				mimc.CompileMiMC,
				compiler.Arcane(
					compiler.WithTargetColSize(targetColSize)),
				logdata.Log("After 1st round of self recursion, MiMC, and Arcane, before Vortex: "),
				vortex.Compile(
					rate2,
					vortex.ForceNumOpenedColumns(forcedNumOpenedColumns2),
					vortex.WithOptionalSISHashingThreshold(sisThreshold),
				),
				selfrecursion.SelfRecurse,
				mimc.CompileMiMC,
				compiler.Arcane(
					compiler.WithTargetColSize(targetColSize)),
				logdata.Log("After 2nd round of self recursion, MiMC, and Arcane, before Vortex: "),
				vortex.Compile(
					rate2,
					vortex.ForceNumOpenedColumns(forcedNumOpenedColumns2),
					vortex.WithOptionalSISHashingThreshold(sisThreshold),
				),
				selfrecursion.SelfRecurse,
				mimc.CompileMiMC,
				compiler.Arcane(
					compiler.WithTargetColSize(targetColSize)),
				logdata.Log("After 3rd round of self recursion, MiMC, and Arcane, before Vortex: "),
				vortex.Compile(
					rate2,
					vortex.ForceNumOpenedColumns(forcedNumOpenedColumns1),
					vortex.WithOptionalSISHashingThreshold(sisThreshold),
				),
				dummy.Compile,
			)
			//logdata.GenCSV(files.MustOverwrite("./self-recusion-perf/1st-round.csv"),
			// 	logdata.IncludeColumnCSVFilter,
			// )(compiled)
			// Just a random line to make sure compiled is used and avoid unused variable errors
			compiled.Coins.AllKeys()
		})
	}
}
