package lookup

import (
	"fmt"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
)

// finalEvaluationCheck implements the [wizard.VerifierAction] interface. It
// represents the consistency check between SigmaT and the SigmaS[i]. This
// corresponds in the check #1 in the doc, where the verifier ensures that
// summing the ending values of the SigmaS[i]s yields the same result as the
// ending value of SigmaT.
//
// The current implementation is for packed Zs
type finalEvaluationCheck struct {
	// the name of a lookupTable in the pack, this can help for debugging.
	Name string
	// ZOpenings lists all the openings of all the zCtx
	ZOpenings []query.LocalOpening
	skipped   bool
}

// Run implements the [wizard.VerifierAction]
func (f *finalEvaluationCheck) Run(run *wizard.VerifierRuntime) error {

	// zSum stores the sum of the ending values of the zs as queried
	// in the protocol via the local opening queries.
	zSum := field.Zero()
	for k := range f.ZOpenings {
		temp := run.GetLocalPointEvalParams(f.ZOpenings[k].ID).Y
		zSum.Add(&zSum, &temp)
	}

	if zSum != field.Zero() {
		return fmt.Errorf("log-derivate lookup, the final evaluation check failed for %v,", f.Name)
	}

	return nil
}

// RunGnark implements the [wizard.VerifierAction]
func (f *finalEvaluationCheck) RunGnark(api frontend.API, run *wizard.WizardVerifierCircuit) {

	// SigmaSKSum stores the sum of the ending values of the SigmaSs as queried
	// in the protocol via the
	zSum := frontend.Variable(field.Zero())
	for k := range f.ZOpenings {
		temp := run.GetLocalPointEvalParams(f.ZOpenings[k].ID).Y
		zSum = api.Add(zSum, temp)
	}

	api.AssertIsEqual(zSum, 0)
}

func (f *finalEvaluationCheck) Skip() {
	f.skipped = true
}

func (f *finalEvaluationCheck) IsSkipped() bool {
	return f.skipped
}
