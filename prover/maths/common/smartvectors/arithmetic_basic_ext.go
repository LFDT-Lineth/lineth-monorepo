package smartvectors

import (
	"github.com/consensys/gnark-crypto/field/koalabear/extensions"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// AddExt returns a smart-vector obtained by position-wise adding [SmartVector].
//   - all inputs `vecs` must have the same size, or the function panics
//   - the output smart-vector has the same size as the input vectors
//   - if no input vectors are provided, the function panics
func AddExt(vecs ...SmartVector) SmartVector {

	coeffs := make([]int, len(vecs))
	for i := range coeffs {
		coeffs[i] = 1
	}

	return LinCombExt(coeffs, vecs)
}

// MulExt returns a smart-vector obtained by position-wise multiplying [SmartVector].
//   - all inputs `vecs` must have the same size, or the function panics
//   - the output smart-vector has the same size as the input vectors
//   - if no input vectors are provided, the function panics
func MulExt(vecs ...SmartVector) SmartVector {
	coeffs := make([]int, len(vecs))
	for i := range coeffs {
		coeffs[i] = 1
	}

	return ProductExt(coeffs, vecs)
}

// ScalarMulExt returns a smart-vector obtained by  multiplying a scalar with a [SmartVector].
//   - the output smart-vector has the same size as the input vector
func ScalarMulExt(vec SmartVector, x fext.Element) SmartVector {
	xVec := NewConstantExt(x, vec.Len())
	return MulExt(vec, xVec)
}

// InnerProductExt returns a scalar obtained as the inner-product of `a` and `b`.
//   - a and b must have the same length, otherwise the function panics
func InnerProductExt(a, b SmartVector) fext.Element {
	if a.Len() != b.Len() {
		panic("length mismatch")
	}

	var res fext.Element

	for i := 0; i < a.Len(); i++ {
		var tmp fext.Element
		a_, b_ := a.GetExt(i), b.GetExt(i)
		tmp.Mul(&a_, &b_)
		res.Add(&res, &tmp)
	}

	return res
}

// LinearCombinationExt returns a [SmartVector] computed as:
//
//	result = vecs[0] + vecs[1] * x + vecs[2] * x^2 + vecs[3] * x^3 + ...
//
// where `x` is a scalar and `vecs[i]` are [SmartVector]
func LinearCombinationExt(vecs []SmartVector, x fext.Element) (result SmartVector) {

	if len(vecs) == 0 {
		panic("no input vectors")
	}

	length := vecs[0].Len()
	// Preallocate the intermediate values

	resReg := make([]fext.Element, length)
	tmpVecExt := make([]fext.Element, length)
	tmpVec := make([]field.Element, length)

	var tmpF, resCon fext.Element
	var anyReg, anyCon bool
	xPow := fext.One()

	accumulateRegExt := func(acc, v extensions.Vector, x fext.Element) {
		for i := 0; i < length; i++ {
			tmpF.Mul(&v[i], &x)
			acc[i].Add(&acc[i], &tmpF)
		}
	}

	accumulateRegMixed := func(acc extensions.Vector, v field.Vector, x fext.Element) {
		acc.MulAccByElement(v, &x)
	}

	// Computes the polynomial operation separately on the const,
	// windows and regular and the aggregate the results at the end.
	// The computation is done following horner's method.
	for i := range vecs {

		v := vecs[i]
		if asRotated, ok := v.(*RotatedExt); ok {
			v = rotatedAsRegularExt(asRotated)
		}
		switch casted := v.(type) {
		case *Constant:
			anyCon = true
			tmpF.MulByElement(&xPow, &casted.Value)
			resCon.Add(&resCon, &tmpF)
		case *ConstantExt:
			anyCon = true
			tmpF.Mul(&casted.Value, &xPow)
			resCon.Add(&resCon, &tmpF)
		case *Regular:
			anyReg = true
			v := field.Vector(*casted)
			accumulateRegMixed(resReg, v, xPow)
		case *RegularExt:
			anyReg = true
			v := extensions.Vector(*casted)
			accumulateRegExt(resReg, v, xPow)
		case *PaddedCircularWindow:
			// treat it as a regular, reusing the buffer
			anyReg = true
			casted.WriteInSlice(tmpVec)
			accumulateRegMixed(resReg, tmpVec, xPow)
		case *PaddedCircularWindowExt:
			// treat it as a regular, reusing the buffer
			anyReg = true
			casted.WriteInSliceExt(tmpVecExt)
			accumulateRegExt(resReg, tmpVecExt, xPow)
		default:
			utils.Panic("unexpected type %T", v)
		}
		xPow.Mul(&x, &xPow)
	}

	switch {
	case anyCon && anyReg:
		for i := range resReg {
			resReg[i].Add(&resReg[i], &resCon)
		}
		return NewRegularExt(resReg)
	case anyCon && !anyReg:
		return NewConstantExt(resCon, length)
	case !anyCon && anyReg:
		return NewRegularExt(resReg)
	}

	// can only happen if no vectors are found or if an unknow type is found
	panic("unreachable")
}

// BatchInvertExt performs the batch inverse operation over a [SmartVector] and
// returns a SmartVector of the same type. When an input element is zero, the
// function returns 0 at the corresponding position.
func BatchInvertExt(x SmartVector) SmartVector {

	switch v := x.(type) {
	case *ConstantExt:
		res := &ConstantExt{Length: v.Length}
		res.Value.Inverse(&v.Value)
		return res
	case *PaddedCircularWindowExt:
		res := &PaddedCircularWindowExt{
			TotLen:  v.TotLen,
			Offset:  v.Offset,
			Window_: fext.BatchInvert(v.Window_),
		}
		res.PaddingVal_.Inverse(&v.PaddingVal_)
		return res
	case *RotatedExt:
		return NewRotatedExt(
			fext.BatchInvert(v.v),
			v.offset,
		)
	case *RegularExt:
		return NewRegularExt(fext.BatchInvert(*v))
	}

	panic("unsupported type")
}

// IsZeroExt returns a [SmartVector] z with the same type of structure than x such
// that x[i] = 0 => z[i] = 1 AND x[i] != 0 => z[i] = 0.
func IsZeroExt(x SmartVector) SmartVector {
	switch v := x.(type) {

	case *ConstantExt:
		res := &ConstantExt{Length: v.Length}
		if v.Value == fext.Zero() {
			res.Value = fext.One()
		}
		return res

	case *PaddedCircularWindowExt:
		res := &PaddedCircularWindowExt{
			TotLen:  v.TotLen,
			Offset:  v.Offset,
			Window_: make([]fext.Element, len(v.Window_)),
		}

		if v.PaddingVal_ == fext.Zero() {
			res.PaddingVal_ = fext.One()
		}

		for i := range res.Window_ {
			if v.Window_[i] == fext.Zero() {
				res.Window_[i] = fext.One()
			}
		}
		return res

	case *RotatedExt:
		res := make([]fext.Element, len(v.v))
		for i := range res {
			if v.v[i] == fext.Zero() {
				res[i] = fext.One()
			}
		}
		return NewRotatedExt(
			res,
			v.offset,
		)

	case *RegularExt:
		res := make([]fext.Element, len(*v))
		for i := range res {
			if (*v)[i] == fext.Zero() {
				res[i] = fext.One()
			}
		}
		return NewRegularExt(res)
	}

	panic("unsupported type")
}

// SumExt returns the field summation of all the elements contained in the vector
func SumExt(a SmartVector) (res fext.Element) {

	switch v := a.(type) {
	case *RegularExt:
		res := fext.Zero()
		for i := range *v {
			res.Add(&res, &(*v)[i])
		}
		return res

	case *PaddedCircularWindowExt:
		res := fext.Zero()
		for i := range v.Window_ {
			res.Add(&res, &v.Window_[i])
		}
		constTerm := fext.NewFromUint(uint64(v.TotLen-len(v.Window_)), 0, 0, 0)
		constTerm.Mul(&constTerm, &v.PaddingVal_)
		res.Add(&res, &constTerm)
		return res

	case *ConstantExt:
		res := fext.NewFromUint(uint64(v.Length), 0, 0, 0)
		res.Mul(&res, &v.Value)
		return res

	case *RotatedExt:
		res := fext.Zero()
		for i := range v.v {
			res.Add(&res, &v.v[i])
		}
		return res

	default:
		utils.Panic("unsupported type: %T", v)
		return res // unreachable, but satisfies the compiler
	}
}
