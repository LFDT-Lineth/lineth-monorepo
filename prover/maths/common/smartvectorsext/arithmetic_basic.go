package smartvectorsext

import (
	"github.com/consensys/linea-monorepo/prover/maths/common/mempoolext"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/vectorext"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// Add returns a smart-vector obtained by position-wise adding [SmartVector].
//   - all inputs `vecs` must have the same size, or the function panics
//   - the output smart-vector has the same size as the input vectors
//   - if no input vectors are provided, the function panics
func Add(vecs ...smartvectors.SmartVector) smartvectors.SmartVector {

	coeffs := make([]int, len(vecs))
	for i := range coeffs {
		coeffs[i] = 1
	}

	return LinComb(coeffs, vecs)
}

// Mul returns a smart-vector obtained by position-wise multiplying [SmartVector].
//   - all inputs `vecs` must have the same size, or the function panics
//   - the output smart-vector has the same size as the input vectors
//   - if no input vectors are provided, the function panics
func Mul(vecs ...smartvectors.SmartVector) smartvectors.SmartVector {
	coeffs := make([]int, len(vecs))
	for i := range coeffs {
		coeffs[i] = 1
	}

	return Product(coeffs, vecs)
}

// ScalarMul returns a smart-vector obtained by  multiplying a scalar with a [SmartVector].
//   - the output smart-vector has the same size as the input vector
func ScalarMul(vec smartvectors.SmartVector, x fext.Element) smartvectors.SmartVector {
	xVec := NewConstantExt(x, vec.Len())
	return Mul(vec, xVec)
}

// InnerProduct returns a scalar obtained as the inner-product of `a` and `b`.
//   - a and b must have the same length, otherwise the function panics
func InnerProduct(a, b smartvectors.SmartVector) fext.Element {
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

// PolyEval returns a [SmartVector] computed as:
//
//	result = vecs[0] + vecs[1] * x + vecs[2] * x^2 + vecs[3] * x^3 + ...
//
// where `x` is a scalar and `vecs[i]` are [SmartVector]
func PolyEval(vecs []smartvectors.SmartVector, x fext.Element, p ...mempoolext.MemPool) (result smartvectors.SmartVector) {

	if len(vecs) == 0 {
		panic("no input vectors")
	}

	length := vecs[0].Len()
	pool, hasPool := mempoolext.ExtractCheckOptionalStrict(length, p...)

	// Preallocate the intermediate values
	var resReg, tmpVec []fext.Element
	if !hasPool {
		resReg = make([]fext.Element, length)
		tmpVec = make([]fext.Element, length)
	} else {
		a := AllocFromPoolExt(pool)
		b := AllocFromPoolExt(pool)
		resReg, tmpVec = a.RegularExt, b.RegularExt
		vectorext.Fill(resReg, fext.Zero())
		defer b.Free(pool)
	}

	var tmpF, resCon fext.Element
	var anyReg, anyCon bool
	xPow := fext.One()

	accumulateReg := func(acc, v []fext.Element, x fext.Element) {
		for i := 0; i < length; i++ {
			tmpF.Mul(&v[i], &x)
			acc[i].Add(&acc[i], &tmpF)
		}
	}

	// Computes the polynomial operation separately on the const,
	// windows and regular and the aggregate the results at the end.
	// The computation is done following horner's method.
	for i := range vecs {

		v := vecs[i]
		if asRotated, ok := v.(*RotatedExt); ok {
			v = rotatedAsRegular(asRotated)
		}

		switch casted := v.(type) {
		case *ConstantExt:
			anyCon = true
			tmpF.Mul(&casted.val, &xPow)
			resCon.Add(&resCon, &tmpF)
		case *RegularExt:
			anyReg = true
			v := *casted
			accumulateReg(resReg, v, xPow)
		case *PooledExt: // e.g. from product
			anyReg = true
			v := casted.RegularExt
			accumulateReg(resReg, v, xPow)
		case *PaddedCircularWindowExt:
			// treat it as a regular, reusing the buffer
			anyReg = true
			casted.WriteInSliceExt(tmpVec)
			accumulateReg(resReg, tmpVec, xPow)
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
		// and we can directly unpool resreg because it was not used
		if hasPool {
			pool.Free(&resReg)
		}
		return NewConstantExt(resCon, length)
	case !anyCon && anyReg:
		return NewRegularExt(resReg)
	}

	// can only happen if no vectors are found or if an unknow type is found
	panic("unreachable")
}

// BatchInvert performs the batch inverse operation over a [SmartVector] and
// returns a SmartVector of the same type. When an input element is zero, the
// function returns 0 at the corresponding position.
func BatchInvert(x smartvectors.SmartVector) smartvectors.SmartVector {

	switch v := x.(type) {
	case *ConstantExt:
		res := &ConstantExt{length: v.length}
		res.val.Inverse(&v.val)
		return res
	case *PaddedCircularWindowExt:
		res := &PaddedCircularWindowExt{
			totLen: v.totLen,
			offset: v.offset,
			window: fext.BatchInvert(v.window),
		}
		res.paddingVal.Inverse(&v.paddingVal)
		return res
	case *RotatedExt:
		return NewRotatedExt(
			fext.BatchInvert(v.v.RegularExt),
			v.offset,
		)
	case *PooledExt:
		return NewRegularExt(fext.BatchInvert(v.RegularExt))
	case *RegularExt:
		return NewRegularExt(fext.BatchInvert(*v))
	}

	panic("unsupported type")
}

// IsZero returns a [SmartVector] z with the same type of structure than x such
// that x[i] = 0 => z[i] = 1 AND x[i] != 0 => z[i] = 0.
func IsZero(x smartvectors.SmartVector) smartvectors.SmartVector {
	switch v := x.(type) {

	case *ConstantExt:
		res := &ConstantExt{length: v.length}
		if v.val == fext.Zero() {
			res.val = fext.One()
		}
		return res

	case *PaddedCircularWindowExt:
		res := &PaddedCircularWindowExt{
			totLen: v.totLen,
			offset: v.offset,
			window: make([]fext.Element, len(v.window)),
		}

		if v.paddingVal == fext.Zero() {
			res.paddingVal = fext.One()
		}

		for i := range res.window {
			if v.window[i] == fext.Zero() {
				res.window[i] = fext.One()
			}
		}
		return res

	case *RotatedExt:
		res := make([]fext.Element, len(v.v.RegularExt))
		for i := range res {
			if v.v.RegularExt[i] == fext.Zero() {
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

	case *PooledExt:
		res := make([]fext.Element, len(v.RegularExt))
		for i := range res {
			if v.RegularExt[i] == fext.Zero() {
				res[i] = fext.One()
			}
		}
		return NewRegularExt(res)
	}

	panic("unsupported type")
}

// Sum returns the field summation of all the elements contained in the vector
func Sum(a smartvectors.SmartVector) (res fext.Element) {

	switch v := a.(type) {
	case *RegularExt:
		res := fext.Zero()
		for i := range *v {
			res.Add(&res, &(*v)[i])
		}
		return res

	case *PaddedCircularWindowExt:
		res := fext.Zero()
		for i := range v.window {
			res.Add(&res, &v.window[i])
		}
		constTerm := fext.NewElement(uint64(v.totLen-len(v.window)), 0)
		constTerm.Mul(&constTerm, &v.paddingVal)
		res.Add(&res, &constTerm)
		return res

	case *ConstantExt:
		res := fext.NewElement(uint64(v.length), 0)
		res.Mul(&res, &v.val)
		return res

	case *RotatedExt:
		res := fext.Zero()
		for i := range v.v.RegularExt {
			res.Add(&res, &v.v.RegularExt[i])
		}
		return res

	case *PooledExt:
		res := fext.Zero()
		for i := range v.RegularExt {
			res.Add(&res, &v.RegularExt[i])
		}
		return res

	default:
		utils.Panic("unsupported type: %T", v)
	}

	return res
}
