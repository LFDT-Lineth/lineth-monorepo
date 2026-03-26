package plonkinwizard

import (
	"github.com/consensys/gnark/backend/witness"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// CircAssignment is the prover step responsible for assigning the plonk circuit
type CircAssignment struct{ *Context }

// AssignSelOpening is the prover step responsible for assigning the selector opening
type AssignSelOpening struct{ *Context }

func (a *CircAssignment) Run(run *wizard.ProverRuntime) {
	a.PlonkProverAction.Run(run, a.getWitnesses(run))
	a.StackedCircuitData.Run(run)
}

func (a CircAssignment) getWitnesses(run *wizard.ProverRuntime) []witness.Witness {

	var (
		data           = a.Q.Data.GetColAssignment(run).IntoRegVecSaveAlloc()
		sel            = a.Q.Selector.GetColAssignment(run).IntoRegVecSaveAlloc()
		nbPublic       = a.NbPublicVariable
		nbPublicPadded = utils.NextPowerOfTwo(nbPublic)
		witnesses      = make([]witness.Witness, 0, a.Q.Data.Size()/nbPublicPadded)
	)

	for i := 0; i < len(sel) && !sel[i].IsZero(); i += nbPublicPadded {

		var (
			locPubInputs  = data[i : i+nbPublic]
			witness, _    = witness.New(field.Modulus())
			witnessFiller = make(chan any, nbPublic)
		)

		for currPos := 0; currPos < nbPublic; currPos++ {
			witnessFiller <- locPubInputs[currPos]
		}

		// closing the channel is necessary to prevent leaking and
		// also to let the witness "know" it is complete.
		close(witnessFiller)
		if err := witness.Fill(nbPublic, 0, witnessFiller); err != nil {
			utils.Panic("[witness.Fill] : %v", err.Error())
		}

		witnesses = append(witnesses, witness)
	}

	return witnesses
}

func (a AssignSelOpening) Run(run *wizard.ProverRuntime) {

	var (
		maxNbInstances    = len(a.SelOpenings)
		nbPubInputsPadded = utils.NextPowerOfTwo(a.Q.GetNbPublicInputs())
	)

	for i := 0; i < maxNbInstances; i++ {

		var (
			openedPos = i * nbPubInputsPadded
			openedVal = a.Q.Selector.GetColAssignment(run).Get(openedPos)
		)

		run.AssignLocalPoint(a.SelOpenings[i].ID, openedVal)
	}
}
