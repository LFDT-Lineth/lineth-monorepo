package localcs

import (
	"github.com/consensys/linea-monorepo/prover/protocol/accessors"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/symbolic"
)

// Compile applies the local openings compilation pass over `comp`.
func compileOpeningsToConstraints(comp *wizard.CompiledIOP) {

	for _, qName := range comp.QueriesParams.AllUnignoredKeys() {
		q, ok := comp.QueriesParams.Data(qName).(query.LocalOpening)
		if !ok {
			// not an inner-product query
			continue
		}

		comp.QueriesParams.MarkAsIgnored(qName)
		round := comp.QueriesParams.Round(qName)

		comp.InsertLocal(
			round,
			qName+"_AS_LOCAL_CS",
			symbolic.Sub(q.Pol, accessors.NewLocalOpeningAccessor(q, round)),
		)
	}
}
