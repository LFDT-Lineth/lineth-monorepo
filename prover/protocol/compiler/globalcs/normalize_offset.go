package globalcs

import (
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
)

// NormalizeGlobalOffset is a small compilation routine which shifts all the
// global constraints expression so that their minimal offset is zero.
func NormalizeGlobalOffset(comp *wizard.CompiledIOP) {

	queries := comp.QueriesNoParams.AllUnignoredKeys()

	for i := range queries {

		q, ok := comp.QueriesNoParams.Data(queries[i]).(query.GlobalConstraint)
		if !ok {
			continue
		}

		comp.QueriesNoParams.MarkAsIgnored(queries[i])

		var (
			round   = comp.QueriesNoParams.Round(queries[i])
			offset  = query.MinMaxOffset(q.Expression).Min
			newExpr = column.ShiftExpr(q.Expression, -offset)
			name    = queries[i] + "_NORMALIZED_GLOBAL_OFFSET"
		)

		comp.InsertGlobal(round, name, newExpr)
	}
}
