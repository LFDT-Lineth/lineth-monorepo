// The simplify package exposes a list of functions aiming at simplifying
// symbolic expressions.
package simplify

import (
	sym "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

var (
	autoFactorize = func(e *sym.Expression) *sym.Expression {
		// The choice of 16 is empirical
		return factorizeExpression(e, 16)
	}
)

// AutoSimplify automatically runs a handfull of automatic simplification
// routines aiming a simplifying the input expression.
func AutoSimplify(expr *sym.Expression) *sym.Expression {

	steps := []func(*sym.Expression) *sym.Expression{
		removePolyEval,
		autoFactorize,
	}

	// To ensure at every stage that the ESHash is never altered.
	initESH := expr.ESHash

	res := expr
	for i, step := range steps {
		res = step(res)

		if err := res.Validate(); err != nil {
			utils.Panic("the simplification step generated an invalid expression: %v", err.Error())
		}

		if res.ESHash != initESH {
			utils.Panic("esh was altered at step %v", i)
		}
	}

	return res
}
