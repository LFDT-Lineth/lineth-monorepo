package plonk

import (
	"fmt"
	"path"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/witness"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed/pragmas"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/exit"
	"github.com/consensys/linea-monorepo/prover/utils/gnarkutil"
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
	PlonkOptions []query.PlonkOption

	// InputFillerKey is the key used to retrieve the input filler from the
	// registry.
	InputFillerKey string

	// CircMaskOpenings are local opening queries over the ToCircuitMask that
	// we use to checks that the "activators" of the Plonk in Wizard are
	// correctly set w.r.t. circMaskOpening
	CircMaskOpenings []query.LocalOpening
}

// InputFiller is a function that is used to fill the public inputs of
// incomplete circuits.
type InputFiller func(circuitInstance, inputIndex int) field.Element

// plonkInputFillerRegistry is a registry for plonk input fillers.
var plonkInputFillerRegistry = map[string]func(circuitInstance, inputIndex int) field.Element{}

// witnessRuntimeKey is a key used to store and retrieve the plonk witnesses
// in the runtime's state.
var (
	witnessRuntimeKey       = "plonk-witnesses"
	numEffWitnessRuntimeKey = "plonk-num-eff-witnesses"
)

// RegisterInputFiller registers an input filler for the given key. Passing it
// the input filler to the register allows using it in a PlonkAlignedInput.
func RegisterInputFiller(key string, inpFiller InputFiller) {
	plonkInputFillerRegistry[key] = inpFiller
}

// retrieveInputFiler retrieves the input filler for the given key. If the key
// is the empty string then the default input filler is returned. The default
// input filler is a function that returns zero. Otherwise, the function
// returns the nil value.
func retrieveInputFiler(key string) InputFiller {
	if len(key) == 0 {
		return func(circuitInstance, inputIndex int) field.Element {
			return field.Zero()
		}
	}
	return plonkInputFillerRegistry[key]
}

func (ci *CircuitAlignmentInput) NbInstances() int {
	return ci.NbCircuitInstances
}

// prepareWitnesses prepares the witnesses for every circuit instance. It is
// called inside the Once so that we do not prepare the witnesses multiple
// times. Safe to call multiple times, it is idepotent after first call.
// The function checks how many instances of the circuit are called and panics
// if this uncovers an overflow.
func (ci *CircuitAlignmentInput) prepareWitnesses(run *wizard.ProverRuntime) {

	// This checks if the witness already exists
	if run.State.Exists(path.Join(ci.Name, witnessRuntimeKey)) {
		return
	}

	nbPublicInputs, _ := gnarkutil.CountVariables(ci.Circuit)

	// This call may panic or exit the program
	ci.checkNbCircuitInvocation(run)

	inputFiller := retrieveInputFiler(ci.InputFillerKey)
	if inputFiller == nil {
		panic(fmt.Sprintf("could not find input filler for key %s", ci.InputFillerKey))
	}

	// the number of inputs here we deduce -- we divide all masked values by the number of instances.
	dataCol := ci.DataToCircuit.GetColAssignment(run)
	maskCol := ci.DataToCircuitMask.GetColAssignment(run)
	// first we count how many inputs we actually have
	totalInputs := 0
	for i := 0; i < maskCol.Len() && totalInputs < nbPublicInputs*ci.NbCircuitInstances; i++ {
		selector := maskCol.Get(i)
		if selector.IsOne() {
			totalInputs++
		}
	}

	// prepare witness for every circuit instance NB! keep in mind that we only
	// have public inputs. So the public and private inputs match. Due to
	// interface definition we have to return both but in practice have only a
	// single one.
	witnesses := make([]witness.Witness, ci.NbCircuitInstances)
	witnessFillers := make([]chan any, ci.NbCircuitInstances)
	numEffWitnesses := 0
	var err error
	wg, ctx := errgroup.WithContext(context.Background())
	for i := range witnesses {
		ii := i // capture the value. Pre Go 1.22
		witnesses[i], err = witness.New(ecc.BLS12_377.ScalarField())
		if err != nil {
			utils.Panic("new witness: %v", err)
			return
		}
		witnessFillers[i] = make(chan any)
		wg.Go(func() error {
			return witnesses[ii].Fill(nbPublicInputs, 0, witnessFillers[ii])
		})
	}

	wg.Go(func() error {
		var filled int
		for j := 0; j < dataCol.Len(); j++ {
			mask := maskCol.Get(j)
			if mask.IsZero() {
				continue
			}
			data := dataCol.Get(j)
			select {
			case <-ctx.Done():
				return nil
			case witnessFillers[filled/nbPublicInputs] <- data:
			}
			filled++
			if filled%nbPublicInputs == 0 {
				close(witnessFillers[(filled-1)/nbPublicInputs])
			}
		}

		if filled > 0 {
			numEffWitnesses = utils.DivCeil(filled-1, nbPublicInputs)
		}

		for filled < nbPublicInputs*ci.NbCircuitInstances {
			select {
			case <-ctx.Done():
				return nil
			case witnessFillers[filled/nbPublicInputs] <- inputFiller(filled/nbPublicInputs, filled%nbPublicInputs):
			}
			filled++
			if filled%nbPublicInputs == 0 {
				close(witnessFillers[(filled-1)/nbPublicInputs])
			}
		}

		return nil
	})

	if err := wg.Wait(); err != nil {
		utils.Panic("fill witness: %v", err.Error())
		return
	}

	run.State.InsertNew(path.Join(ci.Name, witnessRuntimeKey), witnesses)
	run.State.InsertNew(path.Join(ci.Name, numEffWitnessRuntimeKey), numEffWitnesses)
}

// Assign returns the witness for the circuit for solving. The witness is read
// from the columns and if it is not long enough, then filled with dummy values.
// Implements [WitnessAssigner].
func (ci *CircuitAlignmentInput) Assign(run *wizard.ProverRuntime, i int) (private, public witness.Witness, err error) {
	ci.prepareWitnesses(run)
	witnesses := run.State.MustGet(path.Join(ci.Name, witnessRuntimeKey)).([]witness.Witness)
	return witnesses[i], witnesses[i], nil
}

// NumEffWitnesses returns the effective number of Plonk witnesses that are
// collected from the assignment of the AlignmentModule.
func (ci *CircuitAlignmentInput) NumEffWitnesses(run *wizard.ProverRuntime) int {
	ci.prepareWitnesses(run)
	return run.State.MustGet(path.Join(ci.Name, numEffWitnessRuntimeKey)).(int)
}

// checkNbCircuitInvocation checks that the number of time the circuit is called
// does not goes above the [maxNbInstance] limit and returns an error if it does.
func (ci *CircuitAlignmentInput) checkNbCircuitInvocation(run *wizard.ProverRuntime) error {

	var (
		mask              = ci.DataToCircuitMask.GetColAssignment(run).IntoRegVecSaveAlloc()
		count             = 0
		nbPublicInputs, _ = gnarkutil.CountVariables(ci.Circuit)
	)

	for i := range mask {
		if mask[i].IsOne() {
			count++
		}
	}

	if count > nbPublicInputs*ci.NbCircuitInstances {

		// This will either panic or exit the program.
		exit.OnLimitOverflow(
			nbPublicInputs*ci.NbCircuitInstances,
			count,
			fmt.Errorf(
				"[circuit-alignement] too many inputs circuit=%v nb-public-input-required=%v nb-public-input-per-circuit=%v nb-circuits-available=%v nb-circuit-required=%v",
				ci.Name, count, nbPublicInputs, ci.NbCircuitInstances, utils.DivCeil(count, nbPublicInputs),
			),
		)
	}

	return nil
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
	// ActualCircuitInputMask is an assigned column which masks public inputs
	// for the circuit coming from the alignment input.
	ActualCircuitInputMask *dedicated.RepeatedPattern
	// PlonkQuery is the query enforcing that the circuit is satisfied
	PlonkQuery *query.PlonkInWizard
}

// DefineAlignment allows to align data from a column with a mask to PI input
// column of PLONK-in-Wizard instance.
func DefineAlignment(comp *wizard.CompiledIOP, toAlign *CircuitAlignmentInput) *Alignment {

	// compute the constant mask
	nbPublicInputs, _ := gnarkutil.CountVariables(toAlign.Circuit)
	if nbPublicInputs == 0 {
		utils.Panic("cannot connect a circuit with no public inputs: %v", nbPublicInputs)
	}

	var (
		totalColumnSize = utils.NextPowerOfTwo(nbPublicInputs) * utils.NextPowerOfTwo(toAlign.NbCircuitInstances)
		isActive        = comp.InsertCommit(toAlign.Round, ifaces.ColIDf("%v_IS_ACTIVE", toAlign.Name), totalColumnSize)
		actualMask      = dedicated.NewRepeatedPattern(comp, toAlign.Round, getCircuitMaskValuePattern(nbPublicInputs), isActive)
		alignedData     = comp.InsertCommit(toAlign.Round, ifaces.ColIDf("%v_PI", toAlign.Name), totalColumnSize)

		// This has to be the first thing we declare as this runs [frontend.Compile]
		// internally.
		plonkInWizardQ = query.NewPlonkInWizard(
			ifaces.QueryID(toAlign.Name),
			alignedData,
			isActive,
			toAlign.Circuit,
			toAlign.PlonkOptions,
		)
	)

	pragmas.MarkRightPadded(isActive)

	comp.InsertPlonkInWizard(plonkInWizardQ)

	res := &Alignment{
		CircuitAlignmentInput:  toAlign,
		IsActive:               isActive,
		CircuitInput:           alignedData,
		ActualCircuitInputMask: actualMask,
		PlonkQuery:             plonkInWizardQ,
	}

	res.csProjection(comp)

	return res
}

// csProjection ensures the data in the [Alignment.Data] column is the same as
// the data provided by the [Alignment.CircuitInput].
func (a *Alignment) csProjection(comp *wizard.CompiledIOP) {
	comp.InsertProjection(ifaces.QueryIDf("%v_PROJECTION", a.Name), query.ProjectionInput{ColumnA: []ifaces.Column{a.DataToCircuit}, ColumnB: []ifaces.Column{a.CircuitInput}, FilterA: a.DataToCircuitMask, FilterB: a.ActualCircuitInputMask.Natural})
}

// Assign assigns the colums in the Alignment structure at runtime.
func (a *Alignment) Assign(run *wizard.ProverRuntime) {
	a.assignMasks(run)
	a.assignCircMaskOpenings(run)
	a.assignCircData(run)
}

// assignMasks assigns the [Alignment.IsActive] and the [Alignment.ActualCircuitInputMask]
// into `run`.
func (a *Alignment) assignMasks(run *wizard.ProverRuntime) {
	// we want to assign IS_ACTIVE and ACTUAL_MASK columns. We can construct
	// them at the same time from the precomputed mask and selector.
	var (
		totalSize            = a.IsActive.Size()
		dataToCircAssignment = a.DataToCircuitMask.GetColAssignment(run)
		// totalInputs stores the total number of public inputs to assign within
		// the assignment circuit.
		totalInputs = 0
		// totalAligned counts the number of public inputs that have been assigned
		// in the alignement module.
		totalAligned         = 0
		isActiveAssignment   = make([]field.Element, totalSize)
		nbPublicInputs, _    = gnarkutil.CountVariables(a.Circuit)
		nbPublicInputsPadded = utils.NextPowerOfTwo(nbPublicInputs)
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
	for i := 0; i < totalSize; i++ {

		if totalAligned >= totalInputs {
			isActiveAssignment = isActiveAssignment[:i:i]
			break
		}

		isActiveAssignment[i].SetOne()
		if i%nbPublicInputsPadded < nbPublicInputs {
			totalAligned++
		}
	}

	run.AssignColumn(a.IsActive.GetColID(), smartvectors.RightZeroPadded(isActiveAssignment, totalSize))
	a.ActualCircuitInputMask.Assign(run)
}

// assignCircData assigns the [Alignment.CircuitInput] column.
func (a *Alignment) assignCircData(run *wizard.ProverRuntime) {

	var (
		unalignedInputs   = a.CircuitAlignmentInput.DataToCircuit.GetColAssignment(run).IntoRegVecSaveAlloc()
		unalignedSelector = a.CircuitAlignmentInput.DataToCircuitMask.GetColAssignment(run).IntoRegVecSaveAlloc()
		nbInput           = a.PlonkQuery.GetNbPublicInputs()
		nbInputsPadded    = utils.NextPowerOfTwo(nbInput)
		maxNbInstances    = a.PlonkQuery.GetMaxNbCircuitInstances()
		res               = make([]field.Element, nbInputsPadded*maxNbInstances)
		dataChan          = make(chan field.Element, nbInputsPadded*maxNbInstances)
		nbActualData      = 0
	)

	for i := range unalignedInputs {
		if unalignedSelector[i].IsOne() {
			dataChan <- unalignedInputs[i]
			nbActualData++
		}
	}

	var (
		lastEffInstance    = nbActualData / nbInput
		nbDataLastInstance = nbActualData % nbInput
		inputFiller        = retrieveInputFiler(a.InputFillerKey)
	)

	if nbDataLastInstance > 0 {
		for i := nbDataLastInstance; i < nbInput; i++ {
			dataChan <- inputFiller(lastEffInstance, i)
		}
	}

	close(dataChan)

assignmentLoop:
	for i := 0; i < maxNbInstances*nbInputsPadded; i += nbInputsPadded {
		for k := 0; k < nbInput; k++ {
			x, ok := <-dataChan

			// if the channel is closed on the first position, then
			// we stop right here and do not start a new instance.
			if !ok && k == 0 {
				res = res[:i+k]
				break assignmentLoop
			}

			res[i+k] = x
		}
	}

	run.AssignColumn(a.CircuitInput.GetColID(), smartvectors.RightZeroPadded(res, nbInputsPadded*maxNbInstances))

}

// assignCircMaskOpenings assigns the openings queries over the actualCircMaskAssignment
func (a *Alignment) assignCircMaskOpenings(run *wizard.ProverRuntime) {
	for i := range a.CircMaskOpenings {
		v := a.CircMaskOpenings[i].Pol.GetColAssignmentAt(run, 0)
		run.AssignLocalPoint(a.CircMaskOpenings[i].ID, v)
	}
}

// getCircuitMaskValue returns a slices of the form 1 1 1 .. 1 1 0 0 .. 0 with
// [nbPublicInputnbPublicInputPerCircuit] 1s and zero-padded to the next power
// of two.
func getCircuitMaskValuePattern(nbPublicInputPerCircuit int) []field.Element {

	var (
		piLen     = utils.NextPowerOfTwo(nbPublicInputPerCircuit)
		maskValue = make([]field.Element, utils.NextPowerOfTwo(piLen))
	)

	for i := 0; i < nbPublicInputPerCircuit; i++ {
		maskValue[i] = field.One()
	}

	return maskValue
}
