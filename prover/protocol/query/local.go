package query

import (
	"fmt"
	"reflect"

	"github.com/consensys/gnark/frontend"
	sv "github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/variables"
	"github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/google/uuid"
)

// A local constraint is an arithmetic relation between prespecified
// polynomial commitment openings and random coin.
// the local constraint is evaluated at 0
// in order to obtain evaluations at different points, the vector should be shifted first
// and the constraint applied after
type LocalConstraint struct {
	*symbolic.Expression
	ID         ifaces.QueryID
	DomainSize int
	uuid       uuid.UUID `serde:"omit"`
}

// Construct a new local constraint
func NewLocalConstraint(id ifaces.QueryID, expr *symbolic.Expression) LocalConstraint {

	if len(id) == 0 {
		panic("Local constraint with an empty ID")
	}

	expr.AssertValid()

	domainSize := 0
	foundAny := false

	/*
		Upon construct, checks that all the offsets are compatible with
		the domain size and sizes
	*/
	board := expr.Board()
	metadatas := board.ListVariableMetadata()
	var firstColumn ifaces.Column
	for _, metadataInterface := range metadatas {
		if metadata, ok := metadataInterface.(ifaces.Column); ok {
			if !foundAny {
				foundAny = true
				domainSize = metadata.Size()
				firstColumn = metadata
			}
			if metadata.Size() != domainSize {
				utils.Panic(
					"Unsupported : Local constraints with heterogeneous domain size; %v has size %v but %v has size %v",
					firstColumn.GetColID(), domainSize, metadata.GetColID(), metadata.Size(),
				)
			}
		}
	}

	if !foundAny {
		utils.Panic("No commitment found in the local constraint, query id %v", id)
	}

	if domainSize == 0 {
		utils.Panic("All commitment given had a length of zero")
	}

	res := LocalConstraint{Expression: expr, ID: id, DomainSize: domainSize, uuid: uuid.New()}
	return res
}

// Name implements the [ifaces.Query] interface
func (r LocalConstraint) Name() ifaces.QueryID {
	return r.ID
}

// Test the polynomial identity
func (cs LocalConstraint) Check(run ifaces.Runtime) error {
	board := cs.Board()
	metadatas := board.ListVariableMetadata()
	/*
		Collects the relevant datas into a slice for the evaluation
	*/
	inputs := make([]sv.SmartVector, len(metadatas))
	for i, metadataInterface := range metadatas {
		switch metadata := metadataInterface.(type) {
		case ifaces.Column:
			/*
				The offsets are already tested to be in range
					- should be between 0 and N-1 (included)
						where N is the size of the polynomials
			*/
			if metadata.IsBase() {
				val, _ := metadata.GetColAssignmentAtBase(run, 0)
				inputs[i] = sv.NewConstant(val, 1)
			} else {
				val := metadata.GetColAssignmentAtExt(run, 0)
				inputs[i] = sv.NewConstantExt(val, 1)
			}
		case coin.Info:
			if metadata.IsBase() {
				utils.Panic("unsupported, coins are always over field extensions")

			} else {
				inputs[i] = sv.NewConstantExt(run.GetRandomCoinFieldExt(metadata.Name), 1)
			}
		case variables.PeriodicSample:
			v := field.One()
			if metadata.Offset != 0 {
				v.SetZero()
			}
			inputs[i] = sv.NewConstant(metadata.EvalAtOnDomain(0), 1)
		case variables.X:
			utils.Panic("In local constraint %v, Local constraints using X are not handled so far", cs.ID)
		case ifaces.Accessor:
			if metadata.IsBase() {
				val, _ := metadata.GetValBase(run)
				inputs[i] = sv.NewConstant(val, 1)
			} else {
				inputs[i] = sv.NewConstantExt(metadata.GetValExt(run), 1)
			}

		default:
			utils.Panic("Unknown variable type %v in local constraint %v", reflect.TypeOf(metadataInterface), cs.ID)
		}
	}
	/*
		Sanity-check : n (the number of element used for the evaluation)
		should be equal to the length of metadata
	*/
	evalRes := board.Evaluate(inputs)
	res := sv.GetGenericElemOfSmartvector(evalRes, 0)
	/*
		If the query is satisfied, the result should be zero. In
		this case, we pretty print an error.
	*/
	if !res.IsZero() {
		debugMap := make(map[string]string)

		for k, metadataInterface := range metadatas {
			inpx := sv.GetGenericElemOfSmartvector(inputs[k], 0)
			debugMap[string(metadataInterface.String())] = fmt.Sprintf("%v", inpx.String())
		}

		return fmt.Errorf("the local constraint %v check failed \n\tinput details : %v \n\tres: %v\n\t", cs.ID, debugMap, res.String())
	}
	/*
		Nil to indicate the test passed
	*/
	return nil
}

// Test the polynomial identity in a circuit setting
func (cs LocalConstraint) CheckGnark(api frontend.API, run ifaces.GnarkRuntime) {
	koalaAPI := koalagnark.NewAPI(api)
	board := cs.Board()
	metadatas := board.ListVariableMetadata()
	/*
		Collects the relevant datas into a slice for the evaluation
	*/
	inputs := make([]any, len(metadatas))
	for i, metadataInterface := range metadatas {
		switch metadata := metadataInterface.(type) {
		case ifaces.Column:
			/*
				The offsets are already tested to be in range
					- should be between 0 and N-1 (included)
						where N is the size of the polynomials
			*/
			val := metadata.GetColAssignmentGnarkAtExt(run, 0)
			inputs[i] = val
		case coin.Info:
			if metadata.IsBase() {
				utils.Panic("unsupported, coins are always over field extensions")
			} else {
				// TODO @thomas fixme
				inputs[i] = run.GetRandomCoinFieldExt(metadata.Name)
			}
		case variables.X, variables.PeriodicSample:
			utils.Panic("In local constraint %v, Local constraints using X are not handled so far", cs.ID)
		case ifaces.Accessor:
			inputs[i] = metadata.GetFrontendVariableExt(api, run)
		default:
			utils.Panic("Unknown variable type %v in local constraint %v", reflect.TypeOf(metadataInterface), cs.ID)
		}
	}
	/*
		Sanity-check : n (the number of element used for the evaluation)
		should be equal to the length of metadata
	*/
	res := board.GnarkEvalExt(api, inputs)
	zero := koalaAPI.ZeroExt()
	koalaAPI.AssertIsEqualExt(res, zero)

}

func (cs LocalConstraint) UUID() uuid.UUID {
	return cs.uuid
}
