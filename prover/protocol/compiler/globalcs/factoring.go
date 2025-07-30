package globalcs

import (
	"sync"

	"github.com/consensys/linea-monorepo/prover/protocol/accessors"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/symbolic/simplify"
)

// factorExpressionList applies [factorExpression] over a list of expressions
func factorExpressionList(comp *wizard.CompiledIOP, exprList []*symbolic.Expression) []*symbolic.Expression {
	res := make([]*symbolic.Expression, len(exprList))
	var wg sync.WaitGroup

	for i, expr := range exprList {
		wg.Add(1)
		go func(i int, expr *symbolic.Expression) {
			defer wg.Done()
			res[i] = factorExpression(comp, expr)
		}(i, expr)
	}

	wg.Wait()
	return res
}

// factorExpression factors expr and returns the factored expression. The
// resulting factored expression is cached in the file system as this is a
// compute intensive operation.
func factorExpression(comp *wizard.CompiledIOP, expr *symbolic.Expression) *symbolic.Expression {
	flattenedExpr := flattenExpr(expr)
	return simplify.AutoSimplify(flattenedExpr)
}

// flattenExpr returns an expression equivalent to expr where the
// [accessors.FromExprAccessor] are inlined
func flattenExpr(expr *symbolic.Expression) *symbolic.Expression {
	return expr.ReconstructBottomUp(func(e *symbolic.Expression, children []*symbolic.Expression) (new *symbolic.Expression) {

		v, isVar := e.Operator.(symbolic.Variable)
		if !isVar {
			return e.SameWithNewChildren(children)
		}

		fea, isFEA := v.Metadata.(*accessors.FromExprAccessor)

		if isFEA {
			return flattenExpr(fea.Expr)
		}

		return e
	})
}
