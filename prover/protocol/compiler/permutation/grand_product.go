package permutation

import (
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
)

// CompileGrandProduct compiles [query.GrandProduct] queries and
func CompileGrandProduct(comp *wizard.CompiledIOP) {

	for _, qName := range comp.QueriesParams.AllUnignoredKeys() {

		// Filter out non grand product queries
		grandproduct, ok := comp.QueriesParams.Data(qName).(query.GrandProduct)
		if !ok {
			continue
		}

		// This ensures that the grand product query is not used again in the
		// compilation process. We know that the query was not already ignored at the beginning
		// because we are iterating over the unignored keys.
		comp.QueriesParams.MarkAsIgnored(qName)

		var (
			round     = comp.QueriesParams.Round(qName)
			zctxs     = NewZCtxFromGrandProduct(grandproduct)
			verAction = FinalProductCheck{
				GrandProductID: qName,
			}
			allProverActions = ProverTaskAtRound{}
		)

		for _, zctx := range zctxs {
			zctx.Compile(comp)
			verAction.ZOpenings = append(verAction.ZOpenings, zctx.ZOpenings...)
			allProverActions = append(allProverActions, zctx)
		}

		comp.RegisterVerifierAction(round, &verAction)
		comp.RegisterProverAction(round, allProverActions)
	}
}
