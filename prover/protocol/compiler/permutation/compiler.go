package permutation

import (
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/protocol/wizardutils"
	"github.com/consensys/linea-monorepo/prover/symbolic"
)

// CompileGrandProduct scans `comp`, looking for [query.Permutation] queries and
// compiles them using the GrandProduct argument technique. All the queries are
// compiled independently and the technique relies on computing a column Z
// accumulating the fractions (A[i] + Beta) / (B[i] + Beta)
func CompileGrandProduct(comp *wizard.CompiledIOP) {

	var (
		allProverActions = make([]proverTaskAtRound, comp.NumRounds()+1)
		// zCatalog stores a mapping (round, size) into ZCtx and helps finding
		// which Z context should be used to handle a part of a given permutation
		// query.
		zCatalog = map[[2]int]*ZCtx{}
	)

	for _, qName := range comp.QueriesNoParams.AllUnignoredKeys() {

		// Filter out non permutation queries
		permutation, ok := comp.QueriesNoParams.Data(qName).(query.Permutation)
		if !ok {
			continue
		}

		// This ensures that the lookup query is not used again in the
		// compilation process. We know that the query was already ignored at
		// the beginning because we are iterating over the unignored keys.
		comp.QueriesNoParams.MarkAsIgnored(qName)
		round := comp.QueriesNoParams.Round(qName)

		dispatchPermutation(comp, zCatalog, round, permutation)
	}

	for entry, zC := range zCatalog {
		zC.compile(comp)
		round := entry[0]
		allProverActions[round] = append(allProverActions[round], zC)
	}

	for round := range allProverActions {
		if len(allProverActions[round]) > 0 {
			comp.RegisterProverAction(round, allProverActions[round])
			comp.RegisterVerifierAction(round, &VerifierCtx{Ctxs: allProverActions[round]})
		}
	}

}

// dispatchPermutation applies the grand product argument compilation over
// a specific [query.Permutation]
func dispatchPermutation(
	comp *wizard.CompiledIOP,
	zCatalog map[[2]int]*ZCtx,
	round int,
	q query.Permutation,
) {

	var (
		isMultiColumn = len(q.A[0]) > 1
		alpha         coin.Info
		beta          = comp.InsertCoin(round+1, deriveName[coin.Name](q, "BETA"), coin.Field)
	)

	if isMultiColumn {
		alpha = comp.InsertCoin(round+1, deriveName[coin.Name](q, "ALPHA"), coin.Field)
	}

	for k, aOrB := range [2][][]ifaces.Column{q.A, q.B} {
		for frag := range aOrB {
			var (
				numRow = aOrB[frag][0].Size()
				factor = symbolic.NewVariable(aOrB[frag][0])
			)

			if isMultiColumn {
				factor = wizardutils.RandLinCombColSymbolic(alpha, aOrB[frag])
			}

			factor = symbolic.Add(factor, beta)

			catalogEntry := [2]int{round + 1, numRow}
			if _, ok := zCatalog[catalogEntry]; !ok {
				zCatalog[catalogEntry] = &ZCtx{
					Size:  numRow,
					Round: round + 1,
				}
			}

			ctx := zCatalog[catalogEntry]

			switch {
			case k == 0:
				ctx.NumeratorFactors = append(ctx.NumeratorFactors, factor)
			case k == 1:
				ctx.DenominatorFactors = append(ctx.DenominatorFactors, factor)
			default:
				panic("invalid k")
			}
		}
	}
}
