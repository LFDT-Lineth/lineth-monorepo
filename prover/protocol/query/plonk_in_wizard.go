package query

import (
	"errors"
	"fmt"
	"sync"

	"github.com/consensys/gnark-crypto/field/koalabear"

	"github.com/consensys/gnark/backend/witness"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/gnarkutil"
	"github.com/google/uuid"
)

var _ ifaces.Query = &PlonkInWizard{}

// PlonkInWizard is a non-parametric query enforcing that a Plonk circuit
// whose public inputs are provided in the input columns is satisfiable.
// To be more precise the input column can specify several instances of
// of the same circuits, the number of instances is dynamic can be read
// for the Selector column. The input data should be formatted as follows:
//
//	|	DATA					|	SELECTOR 		|
//	| <inputs (instance 0)>		| 1 1 1 1 ...		| // Laying out the actual public inputs of the first instance
//	| <zero padding>			| 1 1 1 1 ...		| // Padding to the next power of two positions
//	| <inputs (instance 1)>		| 1 1 1 1 ...		|
//	| <zero padding>			| 1 1 1 1 ...		|
//	| <inputs (instance 2)>		| 1 1 0 0 ...		|
//	| <zero padding>			| 0 0 0 0 ...		|
//	| 0 0 0 0 ... 				| 0 0 0 0 ...		| // Both the selector and the data columns are right-padded with zeroes to highlight that no more instances are expected.
//
// Of course, both the Data and the Selector columns must have the same length
// and the maximal number of instances is given by:
//
// MaxNbInstance := len(data) / NextPowTwo()
//
// The Data column should always be filled in such a way that the *complete* witness
// witness of a circuit instance is provided: we can't have the last circuit instance
// be only-partially provided.
//
// To understand if an instance of the circuit is active or, the query looks at the
// first row of the Selector column for the corresponding instance. 0 indicates the
// instance is not used and 1 indicates it is. The query does not enforce any
// constraints on the Selector column.
type PlonkInWizard struct {
	// ID is the unique identifier of the query
	ID ifaces.QueryID
	// Data is the column storing the values to provide as the public inputs of
	// the circuit instance.
	Data ifaces.Column
	// Selector is the binary-decreasing column indicating which portion of the
	// rows of [PlonkInWizard.Data] corresponds to actual public inputs to the
	// circuit to satisfy.
	Selector ifaces.Column
	// Circuit is the circuit to satisfy. The circuit must have zero "secret" values
	// meaning that it should be fully assignable just from the public inputs.
	Circuit frontend.Circuit

	// PlonkOptions are optional options to pass to the circuit when building it
	PlonkOptions []PlonkOption

	// nbPublicInput is a lazily-loaded variable representing the number of public
	// inputs in the circuit provided by the query. The variable is computed the
	// first time [PlonkInWizard.GetNbPublicInputs] is called and saved there.
	nbPublicInputs int `serde:"omit"`

	// nbPublicInputs loaded is a flag indicating whether we need to compute the
	// number of public input. It is not using [sync.Once] that way we don't need
	// to initialize the value.
	nbPublicInputsLoaded bool      `serde:"omit"`
	uuid                 uuid.UUID `serde:"omit"`
}

func NewPlonkInWizard(
	ID ifaces.QueryID,
	Data ifaces.Column,
	Selector ifaces.Column,
	Circuit frontend.Circuit,
	PlonkOptions []PlonkOption,
) *PlonkInWizard {

	return &PlonkInWizard{
		ID:           ID,
		Data:         Data,
		Selector:     Selector,
		Circuit:      Circuit,
		PlonkOptions: PlonkOptions,
		uuid:         uuid.New(),
	}
}

// PlonkOption represents a an option for the compilation of the circuit. One
// option is the use of a custom range-checker (based on external-lookups)
// instead of one based on in-circuit lookups.
type PlonkOption struct {
	RangeCheckNbBits               int
	RangeCheckNbLimbs              int
	RangeCheckAddGateForRangeCheck bool
}

// WithRangecheck allows bridging range checking from gnark into Wizard. The
// total of bits being range-checked are nbBits*nbLimbs. If addGateForRangeCheck
// is true, then new gates are added for wires not present in existing gates.
func PlonkRangeCheckOption(nbBits, nbLimbs int, addGateForRangeCheck bool) PlonkOption {
	return PlonkOption{
		RangeCheckNbBits:               nbBits,
		RangeCheckNbLimbs:              nbLimbs,
		RangeCheckAddGateForRangeCheck: addGateForRangeCheck,
	}
}

// Name implements the [ifaces.Query] interface
func (piw *PlonkInWizard) Name() ifaces.QueryID {
	return piw.ID
}

// Check implements the [ifaces.Query] interface
func (piw *PlonkInWizard) Check(run ifaces.Runtime) error {

	var (
		data, errData = piw.Data.GetColAssignment(run).IntoRegVecSaveAllocBase()
		sel, errSel   = piw.Selector.GetColAssignment(run).IntoRegVecSaveAllocBase()
	)

	if errData != nil {
		return fmt.Errorf("Error while getting the data %v", errData)
	}

	if errSel != nil {
		return fmt.Errorf("Error while getting the selector %v", errSel)
	}
	var (
		builder             = gnarkutil.NewMockBuilder(scs.NewBuilder[constraint.U32])
		ccs, compErr        = frontend.CompileU32(koalabear.Modulus(), builder, piw.Circuit)
		numEffInstances int = 0
	)

	if compErr != nil {
		return fmt.Errorf("while compiling the circuit: %w", compErr)
	}

	var (
		nbPublic       = ccs.GetNbPublicVariables()
		nbPublicPadded = utils.NextPowerOfTwo(nbPublic)
		wg             = &sync.WaitGroup{}
		errLock        = &sync.Mutex{}
		errSolver      error
		// pushErr is a convenience function to join an error to errSolver
		// in a thread-safe way.
		pushErr = func(err error) {
			errLock.Lock()
			errSolver = errors.Join(errSolver, err)
			errLock.Unlock()
		}
	)

	for i := 0; i < len(sel) && !sel[i].IsZero(); i += nbPublicPadded {

		numEffInstances++

		// This local variable (which could equivalently be defined as
		// 'numEffInstance - 1') is needed because the value of numEffInstance
		// might change when we go to the next iteration of the loop and print
		// error messages from the goroutine.
		currInstance := i / nbPublicPadded

		wg.Add(1)

		go func(i int) {

			defer wg.Done()

			var (
				locPubInputs  = data[i : i+nbPublicPadded]
				locSelector   = sel[i : i+nbPublicPadded]
				witness, _    = witness.New(koalabear.Modulus())
				witnessFiller = make(chan any, nbPublic)
			)

			for currPos := 0; currPos < nbPublicPadded; currPos++ {

				// NB: this will make the dummy verifier fail but not the
				// actual one as this is not checked by the query. Still,
				// if it happens it legitimately means there is a bug.
				if currPos == 0 && locSelector[currPos].IsZero() {
					pushErr(fmt.Errorf("[plonkInWizard] incomplete assignment"))
					return
				}

				if currPos < nbPublic {
					witnessFiller <- locPubInputs[currPos]
				}

				if currPos >= nbPublic && !locPubInputs[currPos].IsZero() {
					pushErr(fmt.Errorf("[plonkInWizard] public input is not zero in padding area: %v", locPubInputs[currPos].String()))
				}
			}

			// closing the channel is necessary to prevent leaking and
			// also to let the witness "know" it is complete.
			close(witnessFiller)

			// Note: having an error here is completely unexpected so this could
			// be a panic instead. It bubbling up means there is a bug in the
			// current function.
			if err := witness.Fill(nbPublic, 0, witnessFiller); err != nil {
				pushErr(fmt.Errorf("error in witness filler instance=%v err=%w", currInstance, err))
				return
			}

			if err := ccs.IsSolved(witness); err != nil {
				pushErr(fmt.Errorf("error in solver instance=%v err=%w", currInstance, err))
				return
			}

		}(i)
	}

	wg.Wait()

	if errSolver != nil {
		return fmt.Errorf("plonk-in-wizard verifier error: %w", errSolver)
	}

	return nil
}

// CheckGnark implements the [ifaces.Query] interface and will panic in this
// construction because we do not have a good way to check the query within a
// circuit
func (piw *PlonkInWizard) CheckGnark(api frontend.API, run ifaces.GnarkRuntime) {
	utils.Panic("UNSUPPORTED : can't check a PlonkInWizard query directly into the circuit, query-name=%v", piw.Name())
}

// GetNbPublicInputs returns the number of public inputs of the circuit provided
// by the query.
func (piw *PlonkInWizard) GetNbPublicInputs() int {
	// The lazy loading does not need to be thread-safe as (1) it is not
	// meant to be run concurrently and (2) the initialization is idempotent
	// anyway.
	if !piw.nbPublicInputsLoaded {
		piw.nbPublicInputsLoaded = true
		nbPub, _ := gnarkutil.CountVariables(piw.Circuit)
		piw.nbPublicInputs = nbPub
	}
	return piw.nbPublicInputs
}

// GetMaxNbCircuitInstances returns the maximum number of circuit instances
// that can be covered by the query.
func (piw *PlonkInWizard) GetMaxNbCircuitInstances() int {
	return piw.Data.Size() / utils.NextPowerOfTwo(piw.GetNbPublicInputs())
}

// GetRound returns the round number at which both [PlonkInWizard.Data] and
// [PlonkInWizard.Selector] are available.
func (piw *PlonkInWizard) GetRound() int {
	return max(piw.Data.Round(), piw.Selector.Round())
}

// CheckMask checks if the [PlonkInWizard.CircuitMask] is consistent with the
// provided [PlonkInWizard.Circuit]. It returns an error if not.
func (piw *PlonkInWizard) CheckMask(mask smartvectors.SmartVector) error {

	var (
		size                 = piw.Data.Size()
		nbPublicInputs       = piw.GetNbPublicInputs()
		nbPublicInputsPadded = utils.NextPowerOfTwo(nbPublicInputs)
	)

	for i := 0; i < size; i += nbPublicInputsPadded {
		for k := 0; k < nbPublicInputsPadded; k++ {

			val := mask.Get(i + k)

			if k < nbPublicInputs && !val.IsOne() {
				return fmt.Errorf("mask is not consistent with the circuit: mask[%v] = %v but expected 1, k=%v", i+k, val.String(), k)
			}

			if k >= nbPublicInputs && !val.IsZero() {
				return fmt.Errorf("mask is not consistent with the circuit: mask[%v] = %v but expected 0, k=%v", i+k, val.String(), k)
			}
		}
	}

	return nil
}

func (piw *PlonkInWizard) UUID() uuid.UUID {
	return piw.uuid
}
