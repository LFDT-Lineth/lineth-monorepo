package dedicated

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/symbolic"
)

// TernaryCtx represents a column that is constructed as a ternary operation
// from two columns acting as the if-true and if-false branches and a column
// acting as the boolean condition.
//
// The struct implements the [wizard.ProverAction] interface to assign
// the resulting ternary column.
type TernaryCtx struct {
	Condition ifaces.Column
	IfTrue    ifaces.Column
	IfFalse   ifaces.Column
	Result    ifaces.Column
}

// Ternary construct a new column that is constructed as the result of a ternary
// operation from two columns acting as the if-true and if-false branches and
// a column acting as the boolean condition.
//
// The function does not add constraints to checks that the condition is boolean
// and it assumes this is already checked.
//
// The constraints and generated columns are given dummy names of the form.
// "TERNARY_<Some number>"
func Ternary(comp *wizard.CompiledIOP, condition, ifTrue, ifFalse ifaces.Column) *TernaryCtx {

	var (
		round = max(max(condition.Round(), ifTrue.Round()), ifFalse.Round())
		name  = fmt.Sprintf("TERNARY_%v", comp.Columns.NumEntriesTotal())
		ctx   = &TernaryCtx{
			Condition: condition,
			IfTrue:    ifTrue,
			IfFalse:   ifFalse,
		}
	)

	ctx.Result = comp.InsertCommit(
		round,
		ifaces.ColID(name),
		ifTrue.Size(),
		true,
	)

	comp.InsertGlobal(
		round,
		ifaces.QueryID(name),
		symbolic.Sub(
			ctx.Result,
			symbolic.Mul(condition, ifTrue),
			symbolic.Mul(symbolic.Sub(1, condition), ifFalse),
		),
	)

	return ctx
}

// Run implements the [wizard.ProverAction] interface and assigns the
// Result column.
func (ctx *TernaryCtx) Run(run *wizard.ProverRuntime) {

	var (
		condition   = ctx.Condition.GetColAssignment(run)
		ifTrue      = ctx.IfTrue.GetColAssignment(run)
		ifFalse     = ctx.IfFalse.GetColAssignment(run)
		fullSize    = ctx.IfTrue.Size()
		start, stop = smartvectors.CoCompactRange(ifTrue, ifFalse, condition)
		res         = make([]field.Element, 0, stop-start)
	)

	for i := start; i <= stop; i++ {

		c := condition.Get(i)

		if c.IsZero() {
			res = append(res, ifFalse.Get(i))
		} else {
			res = append(res, ifTrue.Get(i))
		}
	}

	run.AssignColumn(
		ctx.Result.GetColID(),
		smartvectors.FromCompactWithRange(res, start, stop, fullSize))
}
