package poly

import (
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils"
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

// TODO remove this, only used in /protocol/compiler/univariates/multi_to_single_point.go (otherwise used in tests only)
// Mul returns (∑_i a[i]xⁱ)*(∑_i b[i]xⁱ) using schoolbook algo
func Mul(a, b []field.Element) (res []field.Element) {

	if len(a) == 0 || len(b) == 0 {
		return []field.Element{}
	}

	res = make([]field.Element, len(a)+len(b)-1)

	for i := 0; i < len(a); i++ {
		for j := 0; j < len(b); j++ {
			var tmp field.Element
			tmp.Mul(&a[i], &b[j])
			res[i+j].Add(&res[i+j], &tmp)
		}
	}

	return res
}

// Add adds two polynomials in coefficient form of possibly distinct degree.
// The returned slice has length = max(len(a), len(b)).
// The empty slice is understood as the zero polynomial and if both a and b are
// empty, the function returns the empty slice.
func Add(a, b []field.Element) (res []field.Element) {

	res = make([]field.Element, utils.Max(len(a), len(b)))
	copy(res, a)
	for i := range b {
		res[i].Add(&res[i], &b[i])
	}

	return res
}
