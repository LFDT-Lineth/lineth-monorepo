package localcs

import (
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/variables"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/symbolic"
)

/*
Reduce a local query -> convert it into a global constraint for the same round

Basically,

P(g^5) + Q(g^-3) = 0 => L_1(x) [ P(g^-5 . x) + Q(g^3 . x) ] for x \in D
*/
func ReduceLocalConstraint(comp *wizard.CompiledIOP, q query.LocalConstraint, round int) {
	/*
		Compile down the query
	*/
	comp.QueriesNoParams.MarkAsIgnored(q.ID)

	domainSize := 0

	metadata := q.ListBoardVariableMetadata()

	for _, m := range metadata {
		if metadata, ok := m.(ifaces.Column); ok {
			domainSize = metadata.Size()
			break
		}
	}

	var (
		min      = query.MinMaxOffsetOfExpression(q.Expression).Min
		lagrange = variables.Lagrange(domainSize, min)
		newExpr  = symbolic.Mul(
			column.ShiftExpr(q.Expression, -min),
			lagrange,
		)
		newName = deriveName[ifaces.QueryID]("LOCAL", q.ID, "QUERY")
	)

	// Make sure to never cancel the bounds here. It is a waste of
	// arithmetic degree + it wrongfully cancels on the last entry
	// if the local constraints involves a `-1` position
	comp.InsertGlobal(round, newName, newExpr, true)
}
