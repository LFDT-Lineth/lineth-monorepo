package poly

import (
	"github.com/consensys/linea-monorepo/prover/maths/field"
)

// Eval returns ∑_i pol[i]xⁱ
func Eval(pol []field.Element, x field.Element) field.Element {
	var res field.Element
	for i := len(pol) - 1; i >= 0; i-- {
		res.Mul(&res, &x)
		res.Add(&res, &pol[i])
	}
	return res
}
