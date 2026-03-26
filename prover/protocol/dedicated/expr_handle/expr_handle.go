package expr_handle

import (
	"reflect"

	sv "github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/variables"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/protocol/wizardutils"
	"github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/sirupsen/logrus"
)

const exprHandlePragma = "expr_handle"

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
	domainSize := a.DomainSize()

	logrus.Tracef("running the expr handle assignment for %v, (round %v)", a.HandleName, a.MaxRound)
	metadatas := boarded.ListVariableMetadata()

	for _, metadataInterface := range metadatas {
		if handle, ok := metadataInterface.(ifaces.Column); ok {
			witness := handle.GetColAssignment(run)
			if witness.Len() != domainSize {
				utils.Panic("Query %v - Witness of %v has size %v which is below %v",
					a.HandleName, handle.String(), witness.Len(), domainSize)
			}
		}
	}

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
			if meta.IsBase() {
				utils.Panic("unsupported, coins are always over field extensions")

			} else {
				x := run.GetRandomCoinFieldExt(meta.Name)
				evalInputs[k] = sv.NewConstantExt(x, domainSize)
			}
		case variables.X:
			evalInputs[k] = meta.EvalCoset(domainSize, 0, 1, false)
		case variables.PeriodicSample:
			evalInputs[k] = meta.EvalCoset(domainSize, 0, 1, false)
		case ifaces.Accessor:
			if metadataInterface.IsBase() {
				elem, errFetch := meta.GetValBase(run)
				if errFetch != nil {
					utils.Panic("failed to fetch base accessor %v for query %v: %v", meta.String(), a.HandleName, errFetch)
				}
				evalInputs[k] = sv.NewConstant(elem, domainSize)
			} else {
				evalInputs[k] = sv.NewConstantExt(meta.GetValExt(run), domainSize)
			}

		default:
			utils.Panic("Not a variable type %v in query %v", reflect.TypeOf(metadataInterface), a.HandleName)
		}
	}

	resWitness := boarded.Evaluate(evalInputs)
	run.AssignColumn(a.HandleName, resWitness)
}

// Create a handle from an expression. Auto-registers the corresponding prover
// action.
func ExprHandle(comp *wizard.CompiledIOP, expr *symbolic.Expression, handleName string) ifaces.Column {
	res, proverAction := makeExprHandleCol(comp, expr, handleName)
	maxRound := proverAction.(*ExprHandleProverAction).MaxRound
	comp.RegisterProverAction(maxRound, proverAction)
	return res
}

// ExprHandleWithoutProverAction creates a handle from a global constraint but
// does not register the assignment prover action. The prover action is instead
// written in the pragmas of the column.
func ExprHandleWithoutProverAction(comp *wizard.CompiledIOP, expr *symbolic.Expression, handleName string) ifaces.Column {
	res, _ := makeExprHandleCol(comp, expr, handleName)
	return res
}

// GetExprHandleAssignment returns the assignment of a column assigned via
// ExprHandle. If the column has not been assigned it will attempt using the
// expr_handle pragma.
func GetExprHandleAssignment(run *wizard.ProverRuntime, colI ifaces.Column) sv.SmartVector {

	if r, ok := run.Columns.TryGet(colI.GetColID()); ok {
		return r
	}

	// This is asserted to succeed. The column must also have the pragma
	// attached.
	col := colI.(column.Natural)

	if proverAction, ok := col.GetPragma(exprHandlePragma); ok {
		proverAction.(*ExprHandleProverAction).Run(run)
		return col.GetColAssignment(run)
	}

	utils.Panic("failed to get assignment for %v", col.String())
	return nil
}

func makeExprHandleCol(comp *wizard.CompiledIOP, expr *symbolic.Expression, handleName string) (ifaces.Column, wizard.ProverAction) {

	var (
		boarded  = expr.Board()
		maxRound = wizardutils.LastRoundToEval(expr)
		length   = column.ExprIsOnSameLengthHandles(&boarded)
		res      = comp.InsertCommit(maxRound, ifaces.ColID(handleName), length, expr.IsBase).(column.Natural)
	)

	comp.InsertGlobal(maxRound, ifaces.QueryID(handleName), expr.Sub(ifaces.ColumnAsVariable(res)))

	proverAction := &ExprHandleProverAction{
		Expr:       expr,
		HandleName: ifaces.ColID(handleName),
		MaxRound:   maxRound,
	}

	res.SetPragma(exprHandlePragma, proverAction)

	return res, proverAction
}
