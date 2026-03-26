package plonkinwizard

import (
	"fmt"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
)

// CheckActivatorAndMask is an implementation of [wizard.VerifierAction] and is
// used to embody the verifier checks added by [checkActivators].
type CheckActivatorAndMask struct {
	*Context
	skipped bool `serde:"omit"`
}

func (c *CheckActivatorAndMask) Run(run wizard.Runtime) error {
	for i := range c.SelOpenings {
		var (
			localOpening = run.GetLocalPointEvalParams(c.SelOpenings[i].ID)
			valOpened    = localOpening.Y
			valActiv     = c.Activators[i].GetColAssignment(run).Get(0)
		)

		if valOpened != valActiv {
			return fmt.Errorf(
				"%v: activator does not match the circMask %v (activator=%v, mask=%v)",
				c.Q.ID, i, valActiv.String(), valOpened.String(),
			)
		}
	}

	return nil
}

func (c *CheckActivatorAndMask) RunGnark(api frontend.API, run wizard.GnarkRuntime) {
	for i := range c.SelOpenings {
		var (
			valOpened = run.GetLocalPointEvalParams(c.SelOpenings[i].ID).Y
			valActiv  = c.Activators[i].GetColAssignmentGnarkAt(run, 0)
		)

		api.AssertIsEqual(valOpened, valActiv)
	}
}

func (c *CheckActivatorAndMask) Skip() {
	c.skipped = true
}

func (c *CheckActivatorAndMask) IsSkipped() bool {
	return c.skipped
}
