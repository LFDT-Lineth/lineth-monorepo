package plonkinternal

import (
	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	"github.com/consensys/gnark/backend/witness"
	cs "github.com/consensys/gnark/constraint/bls12-377"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/mimc"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/parallel"
)

// PlonkInWizardProverAction is an interface representing prover runtime action
// to assign the Plonk circuit and run the gnark solver to generate the witness.
type PlonkInWizardProverAction interface {
	// Run is responsible for scheduling the assignment of the gnark circuit.
	Run(run *wizard.ProverRuntime, fullWitness []witness.Witness)
}

var (
	_ PlonkInWizardProverAction = PlonkNoCommitProverAction{}
)

type PlonkNoCommitProverAction struct {
	GenericPlonkProverAction
}

// Run is responsible for scheduling the assignment of the Wizard
// columns related to the currently compiled Plonk circuit. It is used
// specifically for when we do not wish to use BBS commitment as part of the
// circuit.
//
// In essence, the function works by computing the Plonk witness by calling the
// gnark solver over the circuit and assign the LRO columns from the resulting
// solution.
//
// It implements the [PlonkInWizardProverAction] interface.
func (pa PlonkNoCommitProverAction) Run(run *wizard.ProverRuntime, fullWitnesses []witness.Witness) {

	var (
		ctx             = pa
		maxNbInstance   = pa.MaxNbInstances
		numEffInstances = len(fullWitnesses)
	)

	if ctx.ExternalHasherOption.Enabled {
		solver.RegisterHint(mimc.MimcHintfunc)
	}

	parallel.Execute(maxNbInstance, func(start, stop int) {
		for i := start; i < stop; i++ {

			if i >= numEffInstances {
				run.AssignColumn(ctx.Columns.TinyPI[i].GetColID(), smartvectors.NewConstant(field.Zero(), ctx.Columns.TinyPI[i].Size()))
				run.AssignColumn(ctx.Columns.L[i].GetColID(), smartvectors.NewConstant(field.Zero(), ctx.Columns.L[0].Size()))
				run.AssignColumn(ctx.Columns.R[i].GetColID(), smartvectors.NewConstant(field.Zero(), ctx.Columns.R[0].Size()))
				run.AssignColumn(ctx.Columns.O[i].GetColID(), smartvectors.NewConstant(field.Zero(), ctx.Columns.O[0].Size()))
				run.AssignColumn(ctx.Columns.Activators[i].GetColID(), smartvectors.NewConstant(field.Zero(), 1))
				continue
			}

			// create the witness assignment
			pubWitness, err := fullWitnesses[i].Public()
			if err != nil {
				utils.Panic("[witness.Public] returned an error: %v", err)
			}

			if pa.NbPublicInputs > 0 {

				// Converts it as a smart-vector
				pubWitSV := smartvectors.RightZeroPadded(
					[]field.Element(pubWitness.Vector().(fr.Vector)),
					pa.NbPublicInputs,
				)

				// Assign the public witness
				run.AssignColumn(ctx.Columns.TinyPI[i].GetColID(), pubWitSV)
			}

			// Solve the circuit
			sol_, err := ctx.SPR.Solve(fullWitnesses[i])
			if err != nil {
				utils.Panic("Error in the solver, err=%v", err)
			}

			// And parse the solution into a witness. The solution returned by gnark
			// uses a padding value that is equal to the last value of the "actual"
			// solution. In case we are extending the size of the column thanks to the
			//
			solution := sol_.(*cs.SparseR1CSSolution)
			lastValue := solution.L[len(solution.L)-1]
			run.AssignColumn(ctx.Columns.L[i].GetColID(), smartvectors.RightPadded(solution.L, lastValue, ctx.Columns.L[i].Size()))
			run.AssignColumn(ctx.Columns.R[i].GetColID(), smartvectors.RightPadded(solution.R, lastValue, ctx.Columns.R[i].Size()))
			run.AssignColumn(ctx.Columns.O[i].GetColID(), smartvectors.RightPadded(solution.O, lastValue, ctx.Columns.O[i].Size()))
			run.AssignColumn(ctx.Columns.Activators[i].GetColID(), smartvectors.NewConstant(field.One(), 1))
		}
	})

	if ctx.RangeCheckOption.Enabled && !ctx.RangeCheckOption.WasCancelled {
		ctx.assignRangeChecked(run)
	}

	if ctx.ExternalHasherOption.Enabled {
		ctx.assignHashColumns(run)
	}
}
