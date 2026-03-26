package permutation

import (
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/protocol/wizardutils"
	"github.com/consensys/linea-monorepo/prover/symbolic"
)

// CompileViaGrandProduct scans `comp`, looking for [query.Permutation] queries and
// compiles them using the GrandProduct argument technique. All the queries are
// compiled independently and the technique relies on computing a column Z
// accumulating the fractions (A[i] + Beta) / (B[i] + Beta)
func CompileViaGrandProduct(comp *wizard.CompiledIOP) {
	CompileIntoGdProduct(comp)
	CompileGrandProduct(comp)
}

// CompileIntoGdProduct scans comp, looking for [query.Permutation] queries
// and compile them into global [query.GrandProduct]. One for every-round.
func CompileIntoGdProduct(comp *wizard.CompiledIOP) {

	var (
		// zCatalog stores a mapping (round, size) into ZCtx and helps finding
		// which Z context should be used to handle a part of a given permutation
		// query.
		gdProductInputs = map[int]*query.GrandProductInput{}
		numPub          = []*symbolic.Expression{}
		denPub          = []*symbolic.Expression{}
		maxRound        = 0
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
		maxRound = max(maxRound, round)

		_numPub, _denPub := dispatchPermutation(comp, gdProductInputs, round, permutation)
		numPub = append(numPub, _numPub...)
		denPub = append(denPub, _denPub...)
	}

	// This corresponds to the case where there are no queries to compile
	if len(gdProductInputs)+len(numPub)+len(denPub) == 0 {
		return
	}

	query := comp.InsertGrandProduct(maxRound+1, ifaces.QueryIDf("PERMUTATION_GD_PRODUCT_%v", comp.SelfRecursionCount), gdProductInputs)
	comp.RegisterProverAction(query.Round, &AssignPermutationGrandProduct{Query: &query, IsPartial: len(numPub)+len(denPub) > 0})
	comp.RegisterVerifierAction(query.Round, &CheckGrandProductIsOne{Query: &query, ExplicitNum: numPub, ExplicitDen: denPub})
}

// dispatchPermutation applies the grand product argument compilation over
// a specific [query.Permutation]
func dispatchPermutation(
	comp *wizard.CompiledIOP,
	permutationInputs map[int]*query.GrandProductInput,
	round int,
	q query.Permutation,
) (numPub, denPub []*symbolic.Expression) {

	var (
		isMultiColumn = len(q.A[0]) > 1
		alpha         coin.Info
		beta          = comp.InsertCoin(round+1, deriveName[coin.Name](q, "BETA"), coin.FieldExt)
	)

	if isMultiColumn {
		alpha = comp.InsertCoin(round+1, deriveName[coin.Name](q, "ALPHA"), coin.FieldExt)
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

			if _, ok := permutationInputs[numRow]; !ok {
				permutationInputs[numRow] = &query.GrandProductInput{
					Size: numRow,
				}
			}

			ctx := permutationInputs[numRow]

			isPublic := column.IsPublicExpression(factor)

			switch {
			case k == 0 && !isPublic:
				ctx.Numerators = append(ctx.Numerators, factor)
			case k == 1 && !isPublic:
				ctx.Denominators = append(ctx.Denominators, factor)
			case k == 0 && isPublic:
				numPub = append(numPub, factor)
			case k == 1 && isPublic:
				denPub = append(denPub, factor)
			default:
				panic("invalid k")
			}
		}
	}

	return numPub, denPub
}
