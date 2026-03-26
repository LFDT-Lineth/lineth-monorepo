package innerproduct

import (
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/symbolic"
)

// ContextForSize stores the compilation context of a pass of the inner-product
// cryptographic compiler. In particular it stores the protocol items added to
// the protocol by the compilation pass (coins, columns, queries etc...)
// relating to a particular size of column.
type ContextForSize struct {

	// Collapsed contains the linear combination of the product pairs covered
	// by the context. Says the compilation context compiles the inner-product
	// of the pairs: (a_i, b_i) for i=0..n, then Collapsed is assigned as
	//
	// Collapsed = \sum_i a_i * b_i * BatchinCoin^i
	Collapsed *symbolic.Expression

	// CollapsedBoard is as Collapsed and stores the ExpressionBoard
	// corresponding to the expression.
	CollapsedBoard symbolic.ExpressionBoard

	// Summation column is built by accumulating the sum of all the sub-product
	// terms.
	Summation ifaces.Column

	// SummationOpening stores the local opening query pointing to the last
	// entry of [Summation]. It is compared to the alleged inner-product values
	// by the verifier to finalize the compilation step.s
	SummationOpening query.LocalOpening

	//  Round after compilation
	Round int
}

// compileForSize applies the compilation step on a range of queries such that
// they all relate to column of the same size. The function expects a non-empty
// list of queries.
//
// It returns the compilation context of the query
// the round indicate the round of the last inner-product query, independent of its size.
func compileForSize(
	comp *wizard.CompiledIOP,
	round int,
	queries []query.InnerProduct,
) *ContextForSize {

	var (
		hasMoreThan1Pair = len(queries) > 1 || len(queries[0].Bs) > 1
		size             = queries[0].A.Size()
		ctx              = &ContextForSize{}
		// batchingCoin is used to collapse all the inner-product queries
		// into a batched inner-product query so that we only need to
		// commit to a single `Summation` column for all theses.
		batchingCoin coin.Info
	)

	if hasMoreThan1Pair {
		round = round + 1
	}
	//set the round
	ctx.Round = round

	ctx.Summation = comp.InsertCommit(
		round,
		deriveName[ifaces.ColID]("SUMMATION", size, comp.SelfRecursionCount),
		size,
		false,
	)

	if hasMoreThan1Pair {

		var (
			pairProduct = []*symbolic.Expression{}
		)

		batchingCoin = comp.InsertCoin(
			round,
			deriveName[coin.Name]("BATCHING_COIN", size, comp.SelfRecursionCount),
			coin.FieldExt,
		)

		for _, q := range queries {
			for _, b := range q.Bs {
				pairProduct = append(pairProduct, symbolic.Mul(q.A, b))
			}
		}

		// random linear combination over the pairs of inner-product.
		res := symbolic.NewPolyEval(batchingCoin.AsVariable(), pairProduct)

		ctx.Collapsed = res
		ctx.CollapsedBoard = ctx.Collapsed.Board()
	}

	if !hasMoreThan1Pair {
		ctx.Collapsed = symbolic.Mul(queries[0].A, queries[0].Bs[0])
		ctx.CollapsedBoard = ctx.Collapsed.Board()
	}

	// This constraints set the recurrent property of summation
	// when Summation size == 1, no need for the recurrence relation
	if ctx.Summation.Size() > 1 {
		comp.InsertGlobal(
			round,
			deriveName[ifaces.QueryID]("SUMMATION_CONSISTENCY", size, comp.SelfRecursionCount),
			symbolic.Sub(
				ctx.Summation,
				column.Shift(ctx.Summation, -1),
				ctx.Collapsed,
			),
		)
	}

	// This constraint ensures that summation has the correct initial value
	comp.InsertLocal(
		round,
		deriveName[ifaces.QueryID]("SUMMATION_INIT", size, comp.SelfRecursionCount),
		symbolic.Sub(ctx.Collapsed, ctx.Summation),
	)

	// The opening of the final position of ctx.Summation should be equal to
	// the linear combinations of the alleged openings of the inner-products.
	ctx.SummationOpening = comp.InsertLocalOpening(
		round,
		deriveName[ifaces.QueryID]("SUMMATION_END", size, comp.SelfRecursionCount),
		column.Shift(ctx.Summation, -1),
	)

	lastRound := comp.NumRounds() - 1
	comp.RegisterVerifierAction(lastRound, &VerifierForSize{
		Queries:          queries,
		SummationOpening: ctx.SummationOpening,
		BatchOpening:     batchingCoin,
	})

	return ctx
}
