package smartvectors

import (
	"math/big"

	"github.com/consensys/linea-monorepo/prover/maths/field/fext"

	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/field"
)

// operator represents a mathematical operation that can be performed between
// scalars and integers. It is implemented by [linCombOp] and [productOp]. The
// operator interface allows applying the operator in all the combination of
// scalars or vectors operands, in immutable version or assigning version.
//
// In the terminology of this interface:
//   - "const" means a scalar. Or equivalently, abstractly, a vector whose all
//     coordinates have the same value.
//   - "vec" means a slice of field element
//   - "term" is a couple (const|vec, coeff)
//   - "coeff" means either a linear combination coefficient or an exponent and
//     is always assumed to be reasonnably small
//
// The reason to resort to this interface is because applying n-ary mathematical
// operator to smart-vector comes with a lot of inherent complexity. This is
// mitigated that we have a single function [processOperator] owning all the
// "smartvector" logic and all the logic pertaining to doing additions,
// multiplication etc.. is implemented by the [operator] interface.
type operator interface {
	// constIntoConst applies the operator over `res` and `(c, coeff)` and sets
	// the result into res. This is specialized for the case where both res and
	// x are scalars.
	//
	// 		res += x * coeff or res *= x^coeff
	constIntoConst(res, x *field.Element, coeff int)
	// vecIntoVec applies the operator over `res` and `(c, coeff)` and sets
	// the result into res. This is specialized for the case where both res and
	// x are vectors.
	//
	// 		res += x * coeff or res *= x^coeff
	vecIntoVec(res, x []field.Element, coeff int)
	// VecIntoVec applies the operator over `res` and `(c, coeff)` and sets
	// the result into res. This is specialized for the case where res is a
	// vector and c is a constant.
	//
	// 		res += x * coeff or res *= x^coeff
	constIntoVec(res []field.Element, x *field.Element, coeff int)
	// constIntoTerm evaluates the operator over (x, coeff) and sets the result
	// into `res`, overwriting it.
	// It is specialized for the case where x and res are both scalars.
	//
	// 		res = x * coeff or res = x^coeff
	constIntoTerm(res, x *field.Element, coeff int)
	// vecIntoTerm evaluates the operator over (x, coeff) and sets the result
	// into `res`, overwriting it.
	// It is specialized for the case where x and res are both vectors.
	//
	// 		res = x * coeff or res = x^coeff where x is a vector
	vecIntoTerm(res, x []field.Element, coeff int)
	// constTermIntoConst updates applies the operator over res and term and
	// sets the result into res.
	// This function is specialized for the case where the term and res are
	// scalar.
	//
	// res += term or res *= term for constants
	constTermIntoConst(res, term *field.Element)
	// vecTermIntoVec updates applies the operator over res and term and
	// sets the result into res.
	// This function is specialized for the case where the term and res are
	// vector.
	//
	// res += term or res *= term
	vecTermIntoVec(res, term []field.Element)
	// constTermIntoVec updates a vector `res` by applying the operator over
	// it
	//
	// res += term or res *= term
	constTermIntoVec(res []field.Element, term *field.Element)

	// Same functions for extension vectors
	// constExtIntoConstExt applies the operator over `res` and `(c, coeff)` and sets
	// the result into res. This is specialized for the case where both res and
	// x are scalars.
	//
	// 		res += x * coeff or res *= x^coeff
	constExtIntoConstExt(res, x *fext.Element, coeff int)
	// vecExtIntoVecExt applies the operator over `res` and `(c, coeff)` and sets
	// the result into res. This is specialized for the case where both res and
	// x are vectors.
	//
	// 		res += x * coeff or res *= x^coeff
	vecExtIntoVecExt(res, x []fext.Element, coeff int)
	// VecIntoVec applies the operator over `res` and `(c, coeff)` and sets
	// the result into res. This is specialized for the case where res is a
	// vector and c is a constant.
	//
	// 		res += x * coeff or res *= x^coeff
	constExtIntoVecExt(res []fext.Element, x *fext.Element, coeff int)
	// constExtIntoTermExt evaluates the operator over (x, coeff) and sets the result
	// into `res`, overwriting it.
	// It is specialized for the case where x and res are both scalars.
	//
	// 		res = x * coeff or res = x^coeff
	constExtIntoTermExt(res, x *fext.Element, coeff int)
	// vecExtIntoTermExt evaluates the operator over (x, coeff) and sets the result
	// into `res`, overwriting it.
	// It is specialized for the case where x and res are both vectors.
	//
	// 		res = x * coeff or res = x^coeff where x is a vector
	vecExtIntoTermExt(res, x []fext.Element, coeff int)
	// constTermIntoConst updates applies the operator over res and term and
	// sets the result into res.
	// This function is specialized for the case where the term and res are
	// scalar.
	//
	// res += term or res *= term for constants
	constTermExtIntoConstExt(res, term *fext.Element)
	// vecTermExtIntoVecExt updates applies the operator over res and term and
	// sets the result into res.
	// This function is specialized for the case where the term and res are
	// vector.
	//
	// res += term or res *= term
	vecTermExtIntoVecExt(res, term []fext.Element)
	// constTermExtIntoVecExt updates a vector `res` by applying the operator over
	// it
	//
	// res += term or res *= term
	constTermExtIntoVecExt(res []fext.Element, term *fext.Element)
}

// linCompOp is an implementation of the [operator] interface. It represents a
// linear combination with coefficients.
type linCombOp struct{}

func (linCombOp) constIntoConst(res, x *field.Element, coeff int) {
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
		var c field.Element
		c.SetInt64(int64(coeff))
		c.Mul(&c, x)
		res.Add(res, &c)
	}
}

func (linCombOp) vecIntoVec(res, x []field.Element, coeff int) {
	// Sanity-check
	assertHasLength(len(res), len(x))
	switch coeff {
	case 1:
		vector.Add(res, res, x)
	case -1:
		vector.Sub(res, res, x)
	case 2:
		for i := range res {
			res[i].Add(&res[i], &x[i]).Add(&res[i], &x[i])
		}
	case -2:
		for i := range res {
			res[i].Sub(&res[i], &x[i]).Sub(&res[i], &x[i])
		}
	default:
		var c, tmp field.Element
		c.SetInt64(int64(coeff))
		for i := range res {
			tmp.Mul(&c, &x[i])
			res[i].Add(&res[i], &tmp)
		}
	}
}

func (linCombOp) constIntoVec(res []field.Element, val *field.Element, coeff int) {
	var term field.Element
	linCombOp.constIntoTerm(linCombOp{}, &term, val, coeff)
	linCombOp.constTermIntoVec(linCombOp{}, res, &term)
}

func (linCombOp) vecIntoTerm(term, x []field.Element, coeff int) {
	switch coeff {
	case 1:
		copy(term, x)
	case -1:
		for i := range term {
			term[i].Neg(&x[i])
		}
	case 2:
		vector.Add(term, x, x)
	case -2:
		for i := range term {
			term[i].Add(&x[i], &x[i]).Neg(&term[i])
		}
	default:
		var c field.Element
		c.SetInt64(int64(coeff))
		for i := range term {
			term[i].Mul(&c, &x[i])
		}
	}
}

func (linCombOp) constIntoTerm(term, x *field.Element, coeff int) {
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
		var c field.Element
		c.SetInt64(int64(coeff))
		term.Mul(&c, x)
	}
}

func (linCombOp) constTermIntoConst(res, term *field.Element) {
	res.Add(res, term)
}

func (linCombOp) vecTermIntoVec(res, term []field.Element) {
	vector.Add(res, res, term)
}

func (linCombOp) constTermIntoVec(res []field.Element, term *field.Element) {
	for i := range res {
		res[i].Add(&res[i], term)
	}
}

type productOp struct{}

// res *= x ^coeff where both res and x are constants
func (productOp) constIntoConst(res, x *field.Element, coeff int) {
	switch coeff {
	case 0:
		// Nothing to do
	case 1:
		res.Mul(res, x)
	case 2:
		res.Mul(res, x).Mul(res, x)
	case 3:
		var tmp field.Element
		tmp.Square(x)
		tmp.Mul(&tmp, x)
		res.Mul(res, &tmp)
	default:
		var tmp field.Element
		tmp.Exp(*x, big.NewInt(int64(coeff)))
		res.Mul(res, &tmp)
	}
}

// res *= x ^coeff where both res and x are vectors
func (productOp) vecIntoVec(res, x []field.Element, coeff int) {

	// Sanity-check
	assertHasLength(len(res), len(x))

	switch coeff {
	case 0:
		// Nothing to do
	case 1:
		vector.MulElementWise(res, res, x)
	case 2:
		for i := range res {
			res[i].Mul(&res[i], &x[i]).Mul(&res[i], &x[i])
		}
	case 3:
		for i := range res {
			var tmp field.Element
			tmp.Square(&x[i])
			tmp.Mul(&tmp, &x[i])
			res[i].Mul(&res[i], &tmp)
		}
	default:
		var tmp field.Element
		for i := range res {
			field.ExpToInt(&tmp, x[i], coeff)
			res[i].Mul(&res[i], &tmp)
		}
	}
}

// res *= x ^coeff where res is a vector and x is a constant
func (productOp) constIntoVec(res []field.Element, x *field.Element, coeff int) {
	var term field.Element
	productOp.constIntoTerm(productOp{}, &term, x, coeff)
	productOp.constTermIntoVec(productOp{}, res, &term)
}

// res = x ^ coeff where x is a constant
func (productOp) constIntoTerm(res, x *field.Element, coeff int) {
	switch coeff {
	case 0:
		res.SetOne()
	case 1:
		res.Set(x)
	case 2:
		res.Square(x)
	case 3:
		var tmp field.Element
		tmp.Square(x)
		res.Mul(&tmp, x)
	default:
		res.Exp(*x, big.NewInt(int64(coeff)))
	}
}

// res = x * coeff or res = x ^ coeff where x is a vector
func (productOp) vecIntoTerm(res, x []field.Element, coeff int) {
	switch coeff {
	case 0:
		vector.Fill(res, field.One())
	case 1:
		copy(res, x)
	case 2:
		vector.MulElementWise(res, x, x)
	case 3:
		for i := range res {
			// Creating a new variable for the case where res and x are the same variable
			var tmp field.Element
			tmp.Square(&x[i])
			res[i].Mul(&tmp, &x[i])
		}
	default:
		c := big.NewInt(int64(coeff))
		for i := range res {
			res[i].Exp(x[i], c)
		}
	}
}

// res += term or res *= coeff for constants
func (productOp) constTermIntoConst(res, term *field.Element) {
	res.Mul(res, term)
}

// res += term for vectors
func (productOp) vecTermIntoVec(res, term []field.Element) {
	vector.MulElementWise(res, res, term)

}

// res += term where res is a vector and term is a constant
func (productOp) constTermIntoVec(res []field.Element, term *field.Element) {
	vector.ScalarMul(res, res, *term)
}
