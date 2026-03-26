package query

import (
	"fmt"
	"reflect"

	"github.com/consensys/gnark/frontend"
	sv "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/variables"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
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
	metadatas := expr.BoardListVariableMetadata()
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
		utils.Panic("No commitment found in the local constraint")
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
	metadatas := cs.BoardListVariableMetadata()
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
			val := metadata.GetColAssignmentAt(run, 0)
			inputs[i] = sv.NewConstant(val, 1)
		case coin.Info:
			inputs[i] = sv.NewConstant(run.GetRandomCoinField(metadata.Name), 1)
		case variables.PeriodicSample:
			v := field.One()
			if metadata.Offset != 0 {
				v.SetZero()
			}
			inputs[i] = sv.NewConstant(metadata.EvalAtOnDomain(0), 1)
		case variables.X:
			utils.Panic("In local constraint %v, Local constraints using X are not handled so far", cs.ID)
		case ifaces.Accessor:
			inputs[i] = sv.NewConstant(metadata.GetVal(run), 1)
		default:
			utils.Panic("Unknown variable type %v in local constraint %v", reflect.TypeOf(metadataInterface), cs.ID)
		}
	}
	/*
		Sanity-check : n (the number of element used for the evaluation)
		should be equal to the length of metadata
	*/
	res := board.Evaluate(inputs).(*sv.Constant).Val()
	/*
		If the query is satisfied, the result should be zero. In
		this case, we pretty print an error.
	*/
	if !res.IsZero() {
		debugMap := make(map[string]string)

		for k, metadataInterface := range metadatas {
			inpx := inputs[k].Get(0)
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
	board := cs.Board()
	metadatas := board.ListVariableMetadata()
	/*
		Collects the relevant datas into a slice for the evaluation
	*/
	inputs := make([]frontend.Variable, len(metadatas))
	for i, metadataInterface := range metadatas {
		switch metadata := metadataInterface.(type) {
		case ifaces.Column:
			/*
				The offsets are already tested to be in range
					- should be between 0 and N-1 (included)
						where N is the size of the polynomials
			*/
			val := metadata.GetColAssignmentGnarkAt(run, 0)
			inputs[i] = val
		case coin.Info:
			inputs[i] = run.GetRandomCoinField(metadata.Name)
		case variables.X, variables.PeriodicSample:
			utils.Panic("In local constraint %v, Local constraints using X are not handled so far", cs.ID)
		case ifaces.Accessor:
			inputs[i] = metadata.GetFrontendVariable(api, run)
		default:
			utils.Panic("Unknown variable type %v in local constraint %v", reflect.TypeOf(metadataInterface), cs.ID)
		}
	}
	/*
		Sanity-check : n (the number of element used for the evaluation)
		should be equal to the length of metadata
	*/
	res := board.GnarkEval(api, inputs)
	api.AssertIsEqual(res, 0)
}

func (cs LocalConstraint) UUID() uuid.UUID {
	return cs.uuid
}
