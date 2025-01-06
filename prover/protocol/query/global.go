package query

import (
	"fmt"
	"math"
	"reflect"

	"github.com/consensys/gnark/frontend"
	sv "github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/fft"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/variables"
	"github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/gnarkutil"
	"github.com/sirupsen/logrus"
)

/*
A global constraint is an arithmetic constraint that is applied on several vectors
For instance A[i - 1] * B[i] = A[i] for all i \in 0..1000. The expression can also
use random coins as variables.
*/
type GlobalConstraint struct {
	/*
		Symbolic expression representing the global constraint
	*/
	*symbolic.Expression
	/*
		ifaces.QueryID of the coinstraint
	*/
	ID ifaces.QueryID
	/*
		Domain over which the constraints applies
	*/
	DomainSize int
	/*
		Optional field: prevents the constraint to be cancelled on the
		bound. This is used in the "permutation" global constraint for
		instance. False by default
	*/
	NoBoundCancel bool
}

/*
Constructor for global constraints

  - getSize is a function that gives the size of a commitment from its ifaces.QueryID.
    It is aimed at abstracting the "CompiledIOP" from this package to avoid
    circular dependencies. It used mainly for validating the constraint and
    computing the size of the domain on which the constraint applies.
*/
func NewGlobalConstraint(id ifaces.QueryID, expr *symbolic.Expression, noBoundCancel ...bool) GlobalConstraint {

	/*
		Sanity-check : the querie's ifaces.QueryID cannot be empty or nil
	*/
	if len(id) <= 0 {
		utils.Panic("Given an empty ifaces.QueryID for global constraint query")
	}

	expr.AssertValid()

	res := GlobalConstraint{
		Expression: expr,
		ID:         id,
	}

	if len(noBoundCancel) > 0 {
		utils.Require(len(noBoundCancel) == 1, "there should be only 1 bound cancel got %v", len(noBoundCancel))
		res.NoBoundCancel = noBoundCancel[0]
	}

	// performs validation for the
	res.DomainSize = res.validatedDomainSize()
	return res
}

// Name implements the [ifaces.Query] interface
func (cs GlobalConstraint) Name() ifaces.QueryID {
	return cs.ID
}

/*
Test a polynomial identity relation
*/
func (cs GlobalConstraint) Check(run ifaces.Runtime) error {

	logrus.Debugf("checking global : %v\n", cs.ID)

	boarded := cs.Board()
	metadatas := boarded.ListVariableMetadata()

	/*
		Sanity-check : All witnesses should have a size at least
		larger than end.
	*/
	for _, metadataInterface := range metadatas {
		if handle, ok := metadataInterface.(ifaces.Column); ok {
			witness := handle.GetColAssignment(run)
			if witness.Len() != cs.DomainSize {
				utils.Panic(
					"Query %v - Witness of %v has size %v  which is below %v",
					cs.ID, handle.GetColID(), witness.Len(), cs.DomainSize,
				)
			}
		}
	}

	/*
		Collects the relevant datas into a slice for the evaluation
	*/
	evalInputs := make([]sv.SmartVector, len(metadatas))

	/*
		Omega is a root of unity which generates the domain of evaluation
		of the constraint. Its size coincide with the size of the domain
		of evaluation. For each value of `i`, X will evaluate to omega^i.
	*/
	omega := fft.GetOmega(cs.DomainSize)
	omegaI := field.One()

	// precomputations of the powers of omega, can be optimized if useful
	omegas := make([]field.Element, cs.DomainSize)
	for i := 0; i < cs.DomainSize; i++ {
		omegas[i] = omegaI
		omegaI.Mul(&omegaI, &omega)
	}

	/*
		Collect the relevants inputs for evaluating the constraint
	*/
	for k, metadataInterface := range metadatas {
		switch meta := metadataInterface.(type) {
		case ifaces.Column:
			w := meta.GetColAssignment(run)
			evalInputs[k] = w
		case coin.Info:
			evalInputs[k] = sv.NewConstant(run.GetRandomCoinField(meta.Name), cs.DomainSize)
		case variables.X:
			evalInputs[k] = meta.EvalCoset(cs.DomainSize, 0, 1, false)
		case variables.PeriodicSample:
			evalInputs[k] = meta.EvalCoset(cs.DomainSize, 0, 1, false)
		case ifaces.Accessor:
			evalInputs[k] = sv.NewConstant(meta.GetVal(run), cs.DomainSize)
		default:
			utils.Panic("Not a variable type %v in query %v", reflect.TypeOf(metadataInterface), cs.ID)
		}
	}

	// This panics if the global constraints doesn't use any commitment
	res := boarded.Evaluate(evalInputs)

	offsetRange := cs.MinMaxOffset()

	start, stop := 0, res.Len()
	if !cs.NoBoundCancel {
		start -= offsetRange.Min
		stop -= offsetRange.Max
	}

	start = max(start, 0)
	stop = min(stop, cs.DomainSize)

	for i := start; i < stop; i++ {

		resx := res.Get(i)
		// The proper test
		if !resx.IsZero() {
			s := ""

			for j := utils.Max(start, i-15); j < utils.Min(stop, i+15); j++ {
				debugMap := make(map[string]string)
				for k, metadataInterface := range metadatas {
					inpx := evalInputs[k].Get(j)
					debugMap[string(metadataInterface.String())] = fmt.Sprintf("%v", inpx.String())
				}
				if j == i {
					s += "\n"
				}
				s += fmt.Sprintf("%v: %v\n", j, debugMap)
				if j == i {
					s += "\n"
				}
			}

			return fmt.Errorf("the global constraint check failed at row %v \n\tinput details : %v \n\tres: %v\n\t", i, s, resx.String())
		}
	}

	// Update the value of omega^i
	omegaI.Mul(&omegaI, &omega)

	// Nil indicate the test passes
	return nil
}

// validatedDomainSize scans the expression of the global constraints and more
// specifically its inputs and looks for the followings:
//   - the expression must use at least one [ifaces.Column] as input variable
//   - all the ifaces.Column input variables must have the same Size
//
// If all these checks passes the function returns the size of the [ifaces.Column]
// inputs that it found.
func (cs *GlobalConstraint) validatedDomainSize() int {

	var (
		boarded   = cs.Board()
		metadatas = boarded.ListVariableMetadata()
		// foundAny flags wether the expression has any [ifaces.Column] as input
		// variables. If there are None, this is invalid and the function will
		// panic.
		foundAny   = false
		domainSize = 0
		// firstColumnFound stores the name of the first column found in the
		// expression. This will be used to print more detailled error message
		// by showing the first column found and the first one that does not
		// have the same size in the expression.
		firstColumnFound ifaces.ColID
	)

	// From the min/max offset
	for _, metadataInterface := range metadatas {
		if handle, ok := metadataInterface.(ifaces.Column); ok {
			// All domains should be the same length
			if !foundAny {
				foundAny = true
				domainSize = handle.Size()
				firstColumnFound = handle.GetColID()
			}

			// validation of the metadata
			if handle.Size() != domainSize {
				utils.Panic(
					"found a column `%v` with domain size %v, but also found `%v` with domainSize %v",
					handle.GetColID(), handle.Size(), firstColumnFound, domainSize,
				)
			}

			if handle.Size() == 0 {
				utils.Panic("size 0 is forbidden (%v)", handle.GetColID())
			}
		}
	}

	// that would be a global constraint on nothing
	if !foundAny {
		utils.Panic("query %v - Could not find any commitment in the metadatas: %v", cs.ID, metadatas)
	}

	return domainSize
}

// Returns the min and max offset happening in the expression
func (cs *GlobalConstraint) MinMaxOffset() utils.Range {

	minOffset := math.MaxInt
	maxOffset := math.MinInt

	/*
		Flag detecting if we indeed found at least one correct metadata
		There for sanity-checks
	*/
	foundAny := false

	exprBoard := cs.Expression.Board()

	for _, metadataUncasted := range exprBoard.ListVariableMetadata() {
		if handle, ok := metadataUncasted.(ifaces.Column); ok {
			foundAny = true

			offset := column.StackOffsets(handle)
			minOffset = utils.Min(minOffset, offset)
			maxOffset = utils.Max(maxOffset, offset)
		}
	}

	if !foundAny {
		panic("did not find any")
	}

	res := utils.Range{Min: minOffset, Max: maxOffset}

	return res
}

/*
Test a polynomial identity relation
*/
func (cs GlobalConstraint) CheckGnark(api frontend.API, run ifaces.GnarkRuntime) {
	boarded := cs.Board()
	metadatas := boarded.ListVariableMetadata()

	/*
		Sanity-check : All witnesses should have a size at least
		larger than end.
	*/
	for _, metadataInterface := range metadatas {
		if handle, ok := metadataInterface.(ifaces.Column); ok {
			witness := handle.GetColAssignmentGnark(run)
			if len(witness) != cs.DomainSize {
				utils.Panic(
					"Query %v - Witness of %v has size %v which is below %v",
					cs.ID, handle.GetColID(), len(witness), cs.DomainSize,
				)
			}
		}
	}

	/*
		Collects the relevant datas into a slice for the evaluation
	*/
	evalInputs := make([][]frontend.Variable, len(metadatas))

	/*
		Omega is a root of unity which generates the domain of evaluation
		of the constraint. Its size coincide with the size of the domain
		of evaluation. For each value of `i`, X will evaluate to omega^i.
	*/
	omega := fft.GetOmega(cs.DomainSize)
	omegaI := field.One()

	// precomputations of the powers of omega, can be optimized if useful
	omegas := make([]frontend.Variable, cs.DomainSize)
	for i := 0; i < cs.DomainSize; i++ {
		omegas[i] = omegaI
		omegaI.Mul(&omegaI, &omega)
	}

	/*
		Collect the relevants inputs for evaluating the constraint
	*/
	for k, metadataInterface := range metadatas {
		switch meta := metadataInterface.(type) {
		case ifaces.Column:
			w := meta.GetColAssignmentGnark(run)
			evalInputs[k] = w
		case coin.Info:
			evalInputs[k] = gnarkutil.RepeatedVariable(run.GetRandomCoinField(meta.Name), cs.DomainSize)
		case variables.X:
			evalInputs[k] = meta.GnarkEvalNoCoset(cs.DomainSize)
		case variables.PeriodicSample:
			evalInputs[k] = meta.GnarkEvalNoCoset(cs.DomainSize)
		case ifaces.Accessor:
			evalInputs[k] = gnarkutil.RepeatedVariable(meta.GetFrontendVariable(api, run), cs.DomainSize)
		default:
			utils.Panic("Not a variable type %v in query %v", reflect.TypeOf(metadataInterface), cs.ID)
		}
	}

	offsetRange := cs.MinMaxOffset()

	start, stop := 0, cs.DomainSize
	if !cs.NoBoundCancel {
		start -= offsetRange.Min
		stop -= offsetRange.Max
	}

	for i := start; i < stop; i++ {
		// This panics if the global constraints doesn't use any commitment
		inputs := make([]frontend.Variable, len(evalInputs))
		for j := range inputs {
			inputs[j] = evalInputs[j][i]
		}
		res := boarded.GnarkEval(api, inputs)
		api.AssertIsEqual(res, 0)
	}

	// Update the value of omega^i
	omegaI.Mul(&omegaI, &omega)
}
