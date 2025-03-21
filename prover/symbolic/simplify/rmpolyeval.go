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

		// Precompute powers of x
		monomialTerms := make([]any, len(cs))
		for i := 0; i < len(cs); i++ {
			// We don't use the default constructor because it will collapse the
			// intermediate terms into a single term. The intermediates are useful because
			// they tell the evaluator to reuse the intermediate terms instead of
			// computing x^i for every term.
			monomialTerms[i] = any(sym.NewProduct([]*sym.Expression{cs[i], x}, []int{1, i}))
		}

		acc := sym.Add(monomialTerms...)

		if oldExpr.ESHash != acc.ESHash {
			panic("ESH was altered")
		}

		return acc
	}

	return e.ReconstructBottomUp(constructor)
}
