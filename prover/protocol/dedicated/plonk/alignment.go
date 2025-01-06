package plonk

import (
	"fmt"
	"sync"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/witness"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/projection"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
	"golang.org/x/net/context"
	"golang.org/x/sync/errgroup"
)

// CircuitAlignmentInput is the input structure for the alignment of the data to
// the circuit. It contains the circuit for which the data is aligned, the data
// to align and the mask which indicates which data should be given as input to
// the circuit.
//
// The alignment is done in a way that the data is padded to power of two length
// for every circuit instance.
type CircuitAlignmentInput struct {
	// Name is a unique name for the alignment for identification purposes.
	Name string
	// Round is the round at which we should call the PLONK solver.
	Round int
	// DataToCircuitMask is a binary vector which indicates which data should be
	// given as an input to a PLONK-in-Wizard instance as a public input.
	//
	// NB! We do not consider the padding to power of two here. If the input
	// data is not full length then we use InputFiller to compute the missing
	// values.
	DataToCircuitMask ifaces.Column
	// DataToCircuit is the actual data to provide as input to PLONK-in-Wizard
	// instance as public input. It is up to the caller to ensure that the
	// number of masked elements is nbPublicInputs*nbCircuitInstances, as it is
	// circuit specific. Most importantly, if the input data is less then the
	// caller must pad with valid inputs according to the circuit (dummy values,
	// zero values, indicator etc)!
	//
	// NB! See the comment for DataToCircuitMask.
	DataToCircuit ifaces.Column
	// Circuit is the gnark circuit for which we provide the public input.
	Circuit frontend.Circuit
	// NbCircuitInstances is the number of gnark circuit instances we call. We
	// have to consider that for every circuit instance the PI column length has
	// to be padded to a power of two.
	NbCircuitInstances int

	// PlonkOptions are optional options to the plonk-in-wizard checker. See [Option].
	PlonkOptions []Option

	// InputFiller returns an element to pad in the public input for the
	// circuit in case DataToCircuitMask is not full length of the circuit
	// input. If it is nil, then we use zero value.
	InputFiller func(circuitInstance, inputIndex int) field.Element

	witnesses       []witness.Witness
	witnessesOnce   sync.Once
	numEffWitnesses int

	// nbPublicInputs is the number of public inputs for the circuit. It is
	// computed from the circuit and then stored here for later use.
	nbPublicInputs int

	// circMaskOpenings are local opening queries over the ToCircuitMask that
	// we use to checks that the "activators" of the Plonk in Wizard are
	// correctly set w.r.t. circMaskOpening
	circMaskOpenings []query.LocalOpening
}

func (ci *CircuitAlignmentInput) NbInstances() int {
	return ci.NbCircuitInstances
}

// prepareWitnesses prepares the witnesses for every circuit instance. It is
// called inside the Once so that we do not prepare the witnesses multiple
// times. Safe to call multiple times, it is idepotent after first call.
func (ci *CircuitAlignmentInput) prepareWitnesses(run *wizard.ProverRuntime) {
	ci.witnessesOnce.Do(func() {
		if ci.InputFiller == nil {
			ci.InputFiller = func(_, _ int) field.Element { return field.Zero() }
		}
		// the number of inputs here we deduce -- we divide all masked values by the number of instances.
		dataCol := ci.DataToCircuit.GetColAssignment(run)
		maskCol := ci.DataToCircuitMask.GetColAssignment(run)
		// first we count how many inputs we actually have
		totalInputs := 0
		for i := 0; i < maskCol.Len() && totalInputs < ci.nbPublicInputs*ci.NbCircuitInstances; i++ {
			selector := maskCol.Get(i)
			if selector.IsOne() {
				totalInputs++
			}
		}
		// prepare witness for every circuit instance NB! keep in mind that we only
		// have public inputs. So the public and private inputs match. Due to
		// interface definition we have to return both but in practice have only a
		// single one.
		ci.witnesses = make([]witness.Witness, ci.NbCircuitInstances)
		witnessFillers := make([]chan any, ci.NbCircuitInstances)
		var err error
		wg, ctx := errgroup.WithContext(context.Background())
		for i := range ci.witnesses {
			ii := i // capture the value. Pre Go 1.22
			ci.witnesses[i], err = witness.New(ecc.BLS12_377.ScalarField())
			if err != nil {
				utils.Panic("new witness: %v", err)
				return
			}
			witnessFillers[i] = make(chan any)
			wg.Go(func() error {
				return ci.witnesses[ii].Fill(ci.nbPublicInputs, 0, witnessFillers[ii])
			})
		}
		go func() {
			var filled int
			for j := 0; j < dataCol.Len(); j++ {
				mask := maskCol.Get(j)
				if mask.IsZero() {
					continue
				}
				data := dataCol.Get(j)
				select {
				case <-ctx.Done():
					return
				case witnessFillers[filled/ci.nbPublicInputs] <- data:
				}
				filled++
				if filled%ci.nbPublicInputs == 0 {
					close(witnessFillers[(filled-1)/ci.nbPublicInputs])
				}
			}

			if filled > 0 {
				ci.numEffWitnesses = utils.DivCeil(filled-1, ci.nbPublicInputs)
			}

			for filled < ci.nbPublicInputs*ci.NbCircuitInstances {
				select {
				case <-ctx.Done():
					return
				case witnessFillers[filled/ci.nbPublicInputs] <- ci.InputFiller(filled/ci.nbPublicInputs, filled%ci.nbPublicInputs):
				}
				filled++
				if filled%ci.nbPublicInputs == 0 {
					close(witnessFillers[(filled-1)/ci.nbPublicInputs])
				}
			}
		}()
		if err := wg.Wait(); err != nil {
			utils.Panic("fill witness: %v", err.Error())
			return
		}
	})
}

// Assign returns the witness for the circuit for solving. The witness is read
// from the columns and if it is not long enough, then filled with dummy values.
// Implements [WitnessAssigner].
func (ci *CircuitAlignmentInput) Assign(run *wizard.ProverRuntime, i int) (private, public witness.Witness, err error) {
	// done inside Once, so can always call without overhead
	ci.prepareWitnesses(run)
	return ci.witnesses[i], ci.witnesses[i], nil
}

// NumEffWitnesses returns the effective number of Plonk witnesses that are
// collected from the assignment of the AlignmentModule.
func (ci *CircuitAlignmentInput) NumEffWitnesses(run *wizard.ProverRuntime) int {
	ci.prepareWitnesses(run)
	return ci.numEffWitnesses
}

// Alignment is the prepared structure where the Data field is aligned to gnark
// circuit PI column. It considers the cases where we call multiple instances of
// the circuit so that the inputs for every circuit is padded to power of two
// length.
type Alignment struct {
	*CircuitAlignmentInput
	// IsActive is a column which indicates that the row is active.
	// Can be used to perform constrain cancellation.
	IsActive ifaces.Column
	// CircuitInput is the aligned input to the circuit with every instance
	// input padded to power of two.
	CircuitInput ifaces.Column
	// FullCircuitInputMask is a precomputed column which masks all public inputs for the circuit.
	FullCircuitInputMask ifaces.Column
	// ActualCircuitInputMask is an assigned column which masks public inputs
	// for the circuit coming from the alignment input.
	ActualCircuitInputMask ifaces.Column
	// NbInput indicates the actual number of inputs for a circuit. Property of
	// this structure as we obtain it after compiling the circuit.
	NbInput int
	// totalSize is the total size of the concatenated padded PI columns for all
	// circuit instances.
	totalSize int
	// plonkInWizardCtx stores the Plonk-in-Wizard context and is used during the
	// assignment phase to ensure that PI gets assigned.
	plonkInWizardCtx compilationCtx
}

// DefineAlignment allows to align data from a column with a mask to PI input
// column of PLONK-in-Wizard instance.
func DefineAlignment(comp *wizard.CompiledIOP, toAlign *CircuitAlignmentInput) *Alignment {

	// This has to be the first thing we declare as this runs [frontend.Compile]
	// internally.
	plonkInWizardCtx := PlonkCheck(
		comp,
		toAlign.Name,
		toAlign.Round,
		toAlign.Circuit,
		toAlign.NbInstances(),
		toAlign.PlonkOptions...,
	)

	// compute the constant mask
	nbPublicInputs := plonkInWizardCtx.Plonk.SPR.GetNbPublicVariables()
	if nbPublicInputs == 0 {
		utils.Panic("cannot connect a circuit with no public inputs: %v", nbPublicInputs)
	}

	// zero value is zero
	if toAlign.nbPublicInputs == 0 {
		toAlign.nbPublicInputs = plonkInWizardCtx.Plonk.SPR.GetNbPublicVariables()
	}

	var (
		maskValue       = getCircuitMaskValue(toAlign.nbPublicInputs, toAlign.NbCircuitInstances)
		mask            = comp.InsertPrecomputed(ifaces.ColIDf("%v_FULL_PI_MASK", toAlign.Name), maskValue)
		totalColumnSize = maskValue.Len()
		isActive        = comp.InsertCommit(toAlign.Round, ifaces.ColIDf("%v_IS_ACTIVE", toAlign.Name), totalColumnSize)
		actualMask      = comp.InsertCommit(toAlign.Round, ifaces.ColIDf("%v_ACTUAL_PI_MASK", toAlign.Name), totalColumnSize)
		alignedData     = plonkInWizardCtx.ConcatenatedTinyPIs(totalColumnSize)
	)

	res := &Alignment{
		CircuitAlignmentInput:  toAlign,
		IsActive:               isActive,
		CircuitInput:           alignedData,
		FullCircuitInputMask:   mask,
		ActualCircuitInputMask: actualMask,
		NbInput:                nbPublicInputs,
		totalSize:              totalColumnSize,
		plonkInWizardCtx:       plonkInWizardCtx,
	}

	res.csIsActive(comp)
	res.csProjection(comp)
	res.csProjectionSelector(comp)
	res.checkActivators(comp)

	return res
}

// csIsActive adds the cosntraints ensuring that the [Alignment.IsActive] column
// is well-formed. Namely, that this is a sequence of 1s followed by a sequence
// of 0s.
func (a *Alignment) csIsActive(comp *wizard.CompiledIOP) {
	// IsActive is binary column
	comp.InsertGlobal(a.Round, ifaces.QueryIDf("%v_IS_ACTIVE_BINARY", a.Name), symbolic.Mul(a.IsActive, symbolic.Sub(a.IsActive, 1)))
	// IS_ACTIVE_{i} == 0 -> IS_ACTIVE_{i+1} == 0 and IS_ACTIVE_{i} == 1 -> IS_ACTIVE_{i+1} == or IS_ACTIVE_{i+1} == 0
	comp.InsertGlobal(a.Round, ifaces.QueryIDf("%v_IS_ACTIVE_SWITCH", a.Name), symbolic.Sub(a.IsActive, symbolic.Mul(a.IsActive, column.Shift(a.IsActive, -1))))
}

// csProjection ensures the data in the [Alignment.Data] column is the same as
// the data provided by the [Alignment.CircuitInput].
func (a *Alignment) csProjection(comp *wizard.CompiledIOP) {
	projection.InsertProjection(comp, ifaces.QueryIDf("%v_PROJECTION", a.Name), []ifaces.Column{a.DataToCircuit}, []ifaces.Column{a.CircuitInput}, a.DataToCircuitMask, a.ActualCircuitInputMask)
}

// csProjectionSelector constraints that the projection selection
// [Alignment.ActualCircuitInputMask] is well-formed. This ensures that the
// imported data are correctly imported "in-front" of the public inputs of the
// Plonk.
func (a *Alignment) csProjectionSelector(comp *wizard.CompiledIOP) {
	// ACTUAL_PI_MASK = IS_ACTIVE * STATIC_PI_MASK
	comp.InsertGlobal(a.Round, ifaces.QueryIDf("%v_ACTUAL_SUBSET", a.Name), symbolic.Sub(a.ActualCircuitInputMask, symbolic.Mul(a.IsActive, a.FullCircuitInputMask)))
}

// Assign assigns the colums in the Alignment structure at runtime.
func (a *Alignment) Assign(run *wizard.ProverRuntime) {
	a.plonkInWizardCtx.GetPlonkProverAction().Run(run, a.CircuitAlignmentInput)
	a.assignMasks(run)
	a.assignCircMaskOpenings(run)
}

// assignMasks assigns the [Alignment.IsActive] and the [Alignment.ActualCircuitInputMask]
// into `run`.
func (a *Alignment) assignMasks(run *wizard.ProverRuntime) {
	// we want to assign IS_ACTIVE and ACTUAL_MASK columns. We can construct
	// them at the same time from the precomputed mask and selector.
	var (
		fullCircMaskAssignment = a.FullCircuitInputMask.GetColAssignment(run)
		dataToCircAssignment   = a.DataToCircuitMask.GetColAssignment(run)
		// totalInputs stores the total number of public inputs to assign within
		// the assignment circuit.
		totalInputs = 0
		// totalAligned counts the number of public inputs that have been assigned
		// in the alignement module.
		totalAligned             = 0
		isActiveAssignment       = make([]field.Element, a.totalSize)
		actualCircMaskAssignment = make([]field.Element, a.totalSize)
	)

	for i := 0; i < dataToCircAssignment.Len(); i++ {
		selector := dataToCircAssignment.Get(i)
		if selector.IsOne() {
			totalInputs++
		}
	}

	// we have the number of 1 selector column elements. We must have
	// same number of ones in the ACTUAL_MASK column. And at the same time the
	// first time we have STATIC_MASK != ALIGNED_MASK, we set IS_ACTIVE to zero.
	for i := 0; i < a.totalSize && totalAligned < totalInputs; i++ {
		fullMask := fullCircMaskAssignment.Get(i)
		if fullMask.IsOne() {
			actualCircMaskAssignment[i].SetOne()
			totalAligned++
		}
		isActiveAssignment[i].SetOne()
	}

	run.AssignColumn(a.IsActive.GetColID(), smartvectors.NewRegular(isActiveAssignment))
	run.AssignColumn(a.ActualCircuitInputMask.GetColID(), smartvectors.NewRegular(actualCircMaskAssignment))
}

// assignCircMaskOpenings assigns the openings queries over the actualCircMaskAssignment
func (a *Alignment) assignCircMaskOpenings(run *wizard.ProverRuntime) {
	for i := range a.circMaskOpenings {
		v := a.circMaskOpenings[i].Pol.GetColAssignmentAt(run, 0)
		run.AssignLocalPoint(a.circMaskOpenings[i].ID, v)
	}
}

// getCircuitMaskValue returns the static assignment of the precomputed columns
// to be assigned to [Alignment.FullCircuitInputMask].
func getCircuitMaskValue(nbPublicInputPerCircuit, nbCircuitInstance int) smartvectors.SmartVector {

	var (
		piLen     = utils.NextPowerOfTwo(nbPublicInputPerCircuit)
		maskValue = make([]field.Element, utils.NextPowerOfTwo(piLen*nbCircuitInstance))
	)

	for i := 0; i < nbCircuitInstance; i++ {
		for j := 0; j < nbPublicInputPerCircuit; j++ {
			maskValue[piLen*i+j] = field.One()
		}
	}

	return smartvectors.NewRegular(maskValue)
}

// checkActivators adds the constraints checking the activators are well-set w.r.t
// to the circuit mask column. See [compilationCtx.Columns.Activators].
func (ci *Alignment) checkActivators(comp *wizard.CompiledIOP) {

	var (
		openings   = make([]query.LocalOpening, ci.NbCircuitInstances)
		mask       = ci.ActualCircuitInputMask
		offset     = utils.NextPowerOfTwo(ci.nbPublicInputs)
		activators = ci.plonkInWizardCtx.Columns.Activators
		round      = activators[0].Round()
	)

	for i := range openings {
		openings[i] = comp.InsertLocalOpening(
			round,
			ifaces.QueryIDf("%v_ACTIVATOR_LOCAL_OP_%v", ci.Name, i),
			column.Shift(mask, i*offset),
		)
	}

	ci.circMaskOpenings = openings

	comp.RegisterVerifierAction(ci.Round, &checkActivatorAndMask{Alignment: *ci})
}

// checkActivatorAndMask is an implementation of [wizard.VerifierAction] and is
// used to embody the verifier checks added by [checkActivators].
type checkActivatorAndMask struct {
	Alignment
	skipped bool
}

func (c *checkActivatorAndMask) Run(run *wizard.VerifierRuntime) error {
	for i := range c.circMaskOpenings {
		var (
			localOpening = run.GetLocalPointEvalParams(c.circMaskOpenings[i].ID)
			valOpened    = localOpening.Y
			valActiv     = c.plonkInWizardCtx.Columns.Activators[i].GetColAssignment(run).Get(0)
		)

		if valOpened != valActiv {
			return fmt.Errorf(
				"%v: activator does not match the circMask %v (activator=%v, mask=%v)",
				c.Name, i, valActiv.String(), valOpened.String(),
			)
		}
	}

	return nil
}

func (c *checkActivatorAndMask) RunGnark(api frontend.API, run *wizard.WizardVerifierCircuit) {
	for i := range c.circMaskOpenings {
		var (
			valOpened = run.GetLocalPointEvalParams(c.circMaskOpenings[i].ID).Y
			valActiv  = c.plonkInWizardCtx.Columns.Activators[i].GetColAssignmentGnarkAt(run, 0)
		)

		api.AssertIsEqual(valOpened, valActiv)
	}
}

func (c *checkActivatorAndMask) Skip() {
	c.skipped = true
}

func (c *checkActivatorAndMask) IsSkipped() bool {
	return c.skipped
}
