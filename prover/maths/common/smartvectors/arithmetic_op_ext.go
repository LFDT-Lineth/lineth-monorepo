package smartvectors

import (
	"math/big"

	"github.com/consensys/linea-monorepo/prover/maths/common/vectorext"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
)

func (linCombOp) constExtIntoConstExt(res, x *fext.Element, coeff int) {
	switch coeff {
	case 1:
		res.Add(res, x)
	case -1:
		res.Sub(res, x)
	case 2:
		res.Add(res, x).Add(res, x)
	case -2:
		res.Sub(res, x).Sub(res, x)
	default:
		var c fext.Element
		fext.SetFromIntBase(&c, int64(coeff))
		c.Mul(&c, x)
		res.Add(res, &c)
	}
}

func (linCombOp) vecExtIntoVecExt(res, x []fext.Element, coeff int) {
	// Sanity-check
	assertHasLength(len(res), len(x))
	switch coeff {
	case 1:
		vectorext.Vector(res).Add(res, x)
	case -1:
		vectorext.Vector(res).Sub(res, x)
	case 2:
		vectorext.Vector(res).Add(res, x)
		vectorext.Vector(res).Add(res, x)
	case -2:
		vectorext.Vector(res).Sub(res, x)
		vectorext.Vector(res).Sub(res, x)
	default:
		var c, tmp fext.Element
		fext.SetFromIntBase(&c, int64(coeff))
		for i := range res {
			tmp.Mul(&c, &x[i])
			res[i].Add(&res[i], &tmp)
		}
	}
}

func (linCombOp) constExtIntoVecExt(res []fext.Element, val *fext.Element, coeff int) {
	var term fext.Element
	linCombOp.constExtIntoTermExt(linCombOp{}, &term, val, coeff)
	linCombOp.constTermExtIntoVecExt(linCombOp{}, res, &term)
}

func (linCombOp) vecExtIntoTermExt(term, x []fext.Element, coeff int) {
	switch coeff {
	case 1:
		copy(term, x)
	case -1:
		for i := range term {
			term[i].Neg(&x[i])
		}
	case 2:
		vectorext.Vector(term).Add(x, x)
	case -2:
		for i := range term {
			term[i].Add(&x[i], &x[i]).Neg(&term[i])
		}
	default:
		var c fext.Element
		fext.SetFromIntBase(&c, int64(coeff))
		for i := range term {
			term[i].Mul(&c, &x[i])
		}
	}
}

func (linCombOp) constExtIntoTermExt(term, x *fext.Element, coeff int) {
	switch coeff {
	case 1:
		term.Set(x)
	case -1:
		term.Neg(x)
	case 2:
		term.Add(x, x)
	case -2:
		term.Add(x, x).Neg(term)
	default:
		var c fext.Element
		fext.SetFromIntBase(&c, int64(coeff))
		term.Mul(&c, x)
	}
}

func (linCombOp) constTermExtIntoConstExt(res, term *fext.Element) {
	res.Add(res, term)
}

func (linCombOp) vecTermExtIntoVecExt(res, term []fext.Element) {
	vectorext.Vector(res).Add(res, term)
}

func (linCombOp) constTermExtIntoVecExt(res []fext.Element, term *fext.Element) {
	for i := range res {
		res[i].Add(&res[i], term)
	}
}

// res *= x ^coeff where both res and x are constants
func (productOp) constExtIntoConstExt(res, x *fext.Element, coeff int) {
	switch coeff {
	case 0:
		// Nothing to do
	case 1:
		res.Mul(res, x)
	case 2:
		res.Mul(res, x).Mul(res, x)
	case 3:
		var tmp fext.Element
		tmp.Square(x)
		tmp.Mul(&tmp, x)
		res.Mul(res, &tmp)
	default:
		var tmp fext.Element
		tmp.Exp(*x, big.NewInt(int64(coeff)))
		res.Mul(res, &tmp)
	}
}

// res *= x ^coeff where both res and x are vectors
func (productOp) vecExtIntoVecExt(res, x []fext.Element, coeff int) {

	// Sanity-check
	assertHasLength(len(res), len(x))

	switch coeff {
	case 0:
		// Nothing to do
	case 1:
		vectorext.Vector(res).Mul(res, x)
	case 2:
		vectorext.Vector(res).Mul(res, x)
		vectorext.Vector(res).Mul(res, x)
	case 3:
		vectorext.Vector(res).Mul(res, x)
		vectorext.Vector(res).Mul(res, x)
		vectorext.Vector(res).Mul(res, x)
	default:
		var tmp fext.Element
		for i := range res {
			fext.ExpToInt(&tmp, x[i], coeff)
			res[i].Mul(&res[i], &tmp)
		}
	}
}

// res *= x ^coeff where res is a vector and x is a constant
func (productOp) constExtIntoVecExt(res []fext.Element, x *fext.Element, coeff int) {
	var term fext.Element
	productOp.constExtIntoTermExt(productOp{}, &term, x, coeff)
	productOp.constTermExtIntoVecExt(productOp{}, res, &term)
}

// res = x ^ coeff where x is a constant
func (productOp) constExtIntoTermExt(res, x *fext.Element, coeff int) {
	switch coeff {
	case 0:
		res.SetOne()
	case 1:
		res.Set(x)
	case 2:
		res.Square(x)
	case 3:
		var tmp fext.Element
		tmp.Square(x)
		res.Mul(&tmp, x)
	default:
		res.Exp(*x, big.NewInt(int64(coeff)))
	}
}

// res = x * coeff or res = x ^ coeff where x is a vector
func (productOp) vecExtIntoTermExt(res, x []fext.Element, coeff int) {
	vectorext.Vector(res).Exp(x, int64(coeff))
}

// res += term or res *= coeff for constants
func (productOp) constTermExtIntoConstExt(res, term *fext.Element) {
	res.Mul(res, term)
}

// res += term for vectors
func (productOp) vecTermExtIntoVecExt(res, term []fext.Element) {
	vectorext.Vector(res).Mul(res, term)
}

// res += term where res is a vector and term is a constant
func (productOp) constTermExtIntoVecExt(res []fext.Element, term *fext.Element) {
	vectorext.Vector(res).ScalarMul(res, term)
}
