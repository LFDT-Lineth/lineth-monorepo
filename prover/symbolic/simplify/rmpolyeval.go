package simplify

import (
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
)

// removePolyEval rewrites all the polyeval as an equivalent sums of products
func removePolyEval(e *sym.Expression) *sym.Expression {

	constructor := func(oldExpr *sym.Expression, newChildren []*sym.Expression) *sym.Expression {

		_, ok := oldExpr.Operator.(sym.PolyEval)
		if !ok {
			return oldExpr.SameWithNewChildren(newChildren)
		}

		x := newChildren[0]
		cs := newChildren[1:]

		if len(cs) == 0 {
			return oldExpr // Handle edge case where there are no coefficients
		}

		acc := cs[0]

		// Precompute powers of x
		powersOfX := make([]*sym.Expression, len(cs))
		powersOfX[0] = x
		for i := 1; i < len(cs); i++ {
			// We don't use the default constructor because it will collapse the
			// intermediate terms into a single term. The intermediates are useful because
			// they tell the evaluator to reuse the intermediate terms instead of
			// computing x^i for every term.
			powersOfX[i] = sym.NewProduct([]*sym.Expression{powersOfX[i-1], x}, []int{1, 1})
		}

		for i := 1; i < len(cs); i++ {
			// Here we want to use the default constructor to ensure that we
			// will have a merged sum at the end.
			acc = sym.Add(acc, sym.Mul(powersOfX[i-1], cs[i]))
		}

		if oldExpr.ESHash != acc.ESHash {
			panic("ESH was altered")
		}

		return acc
	}

	return e.ReconstructBottomUp(constructor)
}
