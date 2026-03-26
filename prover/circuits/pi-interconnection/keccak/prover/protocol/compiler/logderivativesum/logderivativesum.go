package logderivativesum

import (
	"fmt"
	"slices"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

// compile [query.LogDerivativeSum] query
func CompileLogDerivativeSum(comp *wizard.CompiledIOP) {

	// Collect all the logDerivativeSum queries
	for _, qName := range comp.QueriesParams.AllUnignoredKeys() {

		// Filter out non other types of queries
		logDeriv, ok := comp.QueriesParams.Data(qName).(query.LogDerivativeSum)
		if !ok {
			continue
		}

		// This ensures that the LogDerivativeSum query is not used again in the
		// compilation process. We know that the query was already ignored at
		// the beginning because we are iterating over the unignored keys.
		comp.QueriesParams.MarkAsIgnored(qName)

		var (
			zEntries = logDeriv.Inputs
			va       = FinalEvaluationCheck{
				LogDerivSumID: qName,
			}
			lastRound   = logDeriv.Round
			proverTasks = make([]ProverTaskAtRound, lastRound+1)
		)

		sizes := utils.MapFunc(zEntries.Parts, func(part query.LogDerivativeSumPart) int {
			return part.Size
		})

		slices.SortFunc(sizes, func(a, b int) int { return a - b }) // sorts the slice of integers
		sizes = slices.Compact(sizes)
		sizes = slices.Clip(sizes)

		for _, size := range sizes {

			if !utils.IsPowerOfTwo(size) {
				utils.Panic("the size of a log-derivative cannot be a non-power of two: %v", size)
			}

			// get the Numerator and Denominator from the input and prepare their compilation.
			zC := ZCtx{
				Round: lastRound,
				Size:  size,
			}

			// This accumulates all the parts whose size is equal to size in zC
			for _, part := range zEntries.Parts {
				if part.Size == size {
					zC.SigmaNumerator = append(zC.SigmaNumerator, part.Num)
					zC.SigmaDenominator = append(zC.SigmaDenominator, part.Den)
				}
			}

			// z-packing compile; it imposes the correct accumulation over Numerator and Denominator.
			zC.Compile(comp)

			// prover step; Z assignments
			zAssignmentTask := ZAssignmentTask(zC)
			proverTasks[zC.Round].pushZAssignment(zAssignmentTask)

			// collect all the zOpening for all the z columns
			va.ZOpenings = append(va.ZOpenings, zC.ZOpenings...)
		}

		for round, task := range proverTasks {
			if task.numTasks() > 0 {
				comp.RegisterProverAction(round, task)
			}
		}

		// verifer step
		comp.RegisterVerifierAction(lastRound, &va)
	}

}

type FinalEvaluationCheck struct {
	// ZOpenings lists all the openings of all the zCtx
	ZOpenings []query.LocalOpening
	// query ID
	LogDerivSumID ifaces.QueryID
	// skip verifier action
	skipped bool `serde:"omit"`
}

// Run implements the [wizard.VerifierAction]
func (f *FinalEvaluationCheck) Run(run wizard.Runtime) error {

	tmps := make([]field.Element, 0)

	// zSum stores the sum of the ending values of the zs as queried
	// in the protocol via the local opening queries.
	zSum := field.Zero()
	for k := range f.ZOpenings {
		temp := run.GetLocalPointEvalParams(f.ZOpenings[k].ID).Y
		tmps = append(tmps, temp)
		zSum.Add(&zSum, &temp)
	}

	claimedSum := run.GetLogDerivSumParams(f.LogDerivSumID).Sum
	if zSum != claimedSum {
		return fmt.Errorf("log-derivate-sum; the final evaluation check failed for %v\n"+
			"given %v but calculated %v\n"+
			"partial-sums=(len %v) %v",
			f.LogDerivSumID, claimedSum.String(), zSum.String(), len(tmps), vector.Prettify(tmps),
		)
	}

	return nil
}

// RunGnark implements the [wizard.VerifierAction]
func (f *FinalEvaluationCheck) RunGnark(api frontend.API, run wizard.GnarkRuntime) {

	claimedSum := run.GetLogDerivSumParams(f.LogDerivSumID).Sum
	// SigmaSKSum stores the sum of the ending values of the SigmaSs as queried
	// in the protocol via the
	zSum := frontend.Variable(field.Zero())
	for k := range f.ZOpenings {
		temp := run.GetLocalPointEvalParams(f.ZOpenings[k].ID).Y
		zSum = api.Add(zSum, temp)
	}

	api.AssertIsEqual(zSum, claimedSum)
}

func (f *FinalEvaluationCheck) Skip() {
	f.skipped = true
}

func (f *FinalEvaluationCheck) IsSkipped() bool {
	return f.skipped
}
