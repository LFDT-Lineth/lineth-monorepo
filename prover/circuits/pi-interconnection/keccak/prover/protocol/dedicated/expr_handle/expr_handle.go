package expr_handle

import (
	"fmt"
	"reflect"

	sv "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/variables"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizardutils"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/sirupsen/logrus"
)

type ExprHandleProverAction struct {
	Expr       *symbolic.Expression
	HandleName ifaces.ColID
	MaxRound   int
}

func (a *ExprHandleProverAction) DomainSize() int {
	_, _, n := wizardutils.AsExpr(a.Expr)
	return n
}

func (a *ExprHandleProverAction) Run(run *wizard.ProverRuntime) {

	boarded := a.Expr.Board()
	logrus.Tracef("running the expr handle assignment for %v, (round %v)", a.HandleName, a.MaxRound)
	metadatas := boarded.ListVariableMetadata()

	domainSize := a.DomainSize()

	evalInputs := make([]sv.SmartVector, len(metadatas))

	for k, metadataInterface := range metadatas {
		switch meta := metadataInterface.(type) {
		case ifaces.Column:
			w := meta.GetColAssignment(run)
			if w.Len() != domainSize {
				utils.Panic("Query %v - Witness of %v has size %v which is below %v",
					a.HandleName, meta.String(), w.Len(), domainSize)
			}
			evalInputs[k] = w
		case coin.Info:
			x := run.GetRandomCoinField(meta.Name)
			evalInputs[k] = sv.NewConstant(x, domainSize)
		case variables.X:
			evalInputs[k] = meta.EvalCoset(domainSize, 0, 1, false)
		case variables.PeriodicSample:
			evalInputs[k] = meta.EvalCoset(domainSize, 0, 1, false)
		case ifaces.Accessor:
			evalInputs[k] = sv.NewConstant(meta.GetVal(run), domainSize)
		default:
			utils.Panic("Not a variable type %v in query %v", reflect.TypeOf(metadataInterface), a.HandleName)
		}
	}

	resWitness := boarded.Evaluate(evalInputs)
	run.AssignColumn(a.HandleName, resWitness)
}

// Create a handle from an expression. The name is
// optional, if not set a generic name will be derived
// from the ESH of the expression.
func ExprHandle(comp *wizard.CompiledIOP, expr *symbolic.Expression, name ...string) ifaces.Column {

	var (
		boarded    = expr.Board()
		maxRound   = wizardutils.LastRoundToEval(expr)
		length     = column.ExprIsOnSameLengthHandles(&boarded)
		handleName = fmt.Sprintf("SYMBOLIC_%v", expr.ESHash.String())
	)

	if len(name) > 0 {
		handleName = name[0]
	}

	res := comp.InsertCommit(maxRound, ifaces.ColID(handleName), length)
	comp.InsertGlobal(maxRound, ifaces.QueryID(handleName), expr.Sub(ifaces.ColumnAsVariable(res)))

	// prover := func(run *wizard.ProverRuntime) {

	// 	logrus.Tracef("running the expr handle assignment for %v, (round %v)", handleName, maxRound)

	// 	metadatas := boarded.ListVariableMetadata()

	// 	/*
	// 		Sanity-check : All witnesses should have the same size as the expression
	// 	*/
	// 	for _, metadataInterface := range metadatas {
	// 		if handle, ok := metadataInterface.(ifaces.Column); ok {
	// 			witness := handle.GetColAssignment(run)
	// 			if witness.Len() != cs.DomainSize {
	// 				utils.Panic(
	// 					"Query %v - Witness of %v has size %v  which is below %v",
	// 					cs.ID, handle.String(), witness.Len(), cs.DomainSize,
	// 				)
	// 			}
	// 		}
	// 	}

	// 	/*
	// 		Collects the relevant datas into a slice for the evaluation
	// 	*/
	// 	evalInputs := make([]sv.SmartVector, len(metadatas))

	// 	/*
	// 		Omega is a root of unity which generates the domain of evaluation
	// 		of the constraint. Its size coincide with the size of the domain
	// 		of evaluation. For each value of `i`, X will evaluate to omega^i.
	// 	*/
	// 	omega := fft.GetOmega(cs.DomainSize)
	// 	omegaI := field.One()

	// 	// precomputations of the powers of omega, can be optimized if useful
	// 	omegas := make([]field.Element, cs.DomainSize)
	// 	for i := 0; i < cs.DomainSize; i++ {
	// 		omegas[i] = omegaI
	// 		omegaI.Mul(&omegaI, &omega)
	// 	}

	// 	/*
	// 		Collect the relevants inputs for evaluating the constraint
	// 	*/
	// 	for k, metadataInterface := range metadatas {
	// 		switch meta := metadataInterface.(type) {
	// 		case ifaces.Column:
	// 			w := meta.GetColAssignment(run)
	// 			evalInputs[k] = w
	// 		case coin.Info:
	// 			// Implicitly, the coin has to be a field element in the expression
	// 			// It will panic if not
	// 			x := run.GetRandomCoinField(meta.Name)
	// 			evalInputs[k] = sv.NewConstant(x, length)
	// 		case variables.X:
	// 			evalInputs[k] = meta.EvalCoset(length, 0, 1, false)
	// 		case variables.PeriodicSample:
	// 			evalInputs[k] = meta.EvalCoset(length, 0, 1, false)
	// 		case ifaces.Accessor:
	// 			evalInputs[k] = sv.NewConstant(meta.GetVal(run), length)
	// 		default:
	// 			utils.Panic("Not a variable type %v in query %v", reflect.TypeOf(metadataInterface), cs.ID)
	// 		}
	// 	}

	// 	// This panics if the global constraints doesn't use any commitment
	// 	resWitness := boarded.Evaluate(evalInputs)

	// 	run.AssignColumn(ifaces.ColID(handleName), resWitness)
	// }
	//comp.SubProvers.AppendToInner(maxRound, prover)

	comp.RegisterProverAction(maxRound, &ExprHandleProverAction{
		Expr:       expr,
		HandleName: ifaces.ColID(handleName),
		MaxRound:   maxRound,
	})
	return res
}
