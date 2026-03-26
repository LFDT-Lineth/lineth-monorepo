package smartvectors

import (
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/mempool"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/poly"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

// Add returns a smart-vector obtained by position-wise adding [SmartVector].
//   - all inputs `vecs` must have the same size, or the function panics
//   - the output smart-vector has the same size as the input vectors
//   - if no input vectors are provided, the function panics
func Add(vecs ...SmartVector) SmartVector {

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
func Mul(vecs ...SmartVector) SmartVector {
	coeffs := make([]int, len(vecs))
	for i := range coeffs {
		coeffs[i] = 1
	}

	return Product(coeffs, vecs)
}

// ScalarMul returns a smart-vector obtained by  multiplying a scalar with a [SmartVector].
//   - the output smart-vector has the same size as the input vector
func ScalarMul(vec SmartVector, x field.Element) SmartVector {
	xVec := NewConstant(x, vec.Len())
	return Mul(vec, xVec)
}

// InnerProduct returns a scalar obtained as the inner-product of `a` and `b`.
//   - a and b must have the same length, otherwise the function panics
func InnerProduct(a, b SmartVector) field.Element {
	if a.Len() != b.Len() {
		panic("length mismatch")
	}

	var res field.Element

	for i := 0; i < a.Len(); i++ {
		var tmp field.Element
		a_, b_ := a.Get(i), b.Get(i)
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
func PolyEval(vecs []SmartVector, x field.Element, p ...mempool.MemPool) (result SmartVector) {

	if len(vecs) == 0 {
		panic("no input vectors")
	}

	length := vecs[0].Len()

	// In case the provided inputs are all constants, we can take a shortcut
	// and skip the allocations.
	hasOnlyConst := true
	for i := 0; i < len(vecs); i++ {
		if _, ok := vecs[i].(*Constant); !ok {
			hasOnlyConst = false
			break
		}
	}

	if hasOnlyConst {
		v := make([]field.Element, len(vecs))
		for i := 0; i < len(vecs); i++ {
			v[i] = vecs[i].(*Constant).Value
		}

		y := poly.EvalUnivariate(v, x)
		return NewConstant(y, length)
	}

	pool, hasPool := mempool.ExtractCheckOptionalStrict(length, p...)

	// Preallocate the intermediate values
	var resReg, tmpVec []field.Element
	if !hasPool {
		resReg = make([]field.Element, length)
		tmpVec = make([]field.Element, length)
	} else {
		a := AllocFromPool(pool)
		b := AllocFromPool(pool)
		resReg, tmpVec = a.Regular, b.Regular
		vector.Fill(resReg, field.Zero())
		defer b.Free(pool)
	}

	var tmpF, resCon field.Element
	var anyReg, anyCon bool
	xPow := field.One()

	accumulateReg := func(acc, v []field.Element, x field.Element) {
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
		if asRotated, ok := v.(*Rotated); ok {
			v = rotatedAsRegular(asRotated)
		}

		switch casted := v.(type) {
		case *Constant:
			anyCon = true
			tmpF.Mul(&casted.Value, &xPow)
			resCon.Add(&resCon, &tmpF)
		case *Regular:
			anyReg = true
			v := *casted
			accumulateReg(resReg, v, xPow)
		case *Pooled: // e.g. from product
			anyReg = true
			v := casted.Regular
			accumulateReg(resReg, v, xPow)
		case *PaddedCircularWindow:
			// treat it as a regular, reusing the buffer
			anyReg = true
			casted.WriteInSlice(tmpVec)
			accumulateReg(resReg, tmpVec, xPow)
		}

		xPow.Mul(&x, &xPow)
	}

	switch {
	case anyCon && anyReg:
		for i := range resReg {
			resReg[i].Add(&resReg[i], &resCon)
		}
		return NewRegular(resReg)
	case anyCon && !anyReg:
		// and we can directly unpool resreg because it was not used
		if hasPool {
			pool.Free(&resReg)
		}
		return NewConstant(resCon, length)
	case !anyCon && anyReg:
		return NewRegular(resReg)
	}

	// can only happen if no vectors are found or if an unknow type is found
	panic("unreachable")
}

// BatchInvert performs the batch inverse operation over a [SmartVector] and
// returns a SmartVector of the same type. When an input element is zero, the
// function returns 0 at the corresponding position.
func BatchInvert(x SmartVector) SmartVector {

	switch v := x.(type) {
	case *Constant:
		res := &Constant{Length: v.Length}
		res.Value.Inverse(&v.Value)
		return res
	case *PaddedCircularWindow:
		res := &PaddedCircularWindow{
			TotLen_: v.TotLen_,
			Offset_: v.Offset_,
			Window_: field.BatchInvert(v.Window_),
		}
		res.PaddingVal_.Inverse(&v.PaddingVal_)
		return res
	case *Rotated:
		return NewRotated(
			field.BatchInvert(v.v.Regular),
			v.offset,
		)
	case *Pooled:
		return NewRegular(field.BatchInvert(v.Regular))
	case *Regular:
		return NewRegular(field.BatchInvert(*v))
	}

	panic("unsupported type")
}

// IsZero returns a [SmartVector] z with the same type of structure than x such
// that x[i] = 0 => z[i] = 1 AND x[i] != 0 => z[i] = 0.
func IsZero(x SmartVector) SmartVector {
	switch v := x.(type) {

	case *Constant:
		res := &Constant{Length: v.Length}
		if v.Value == field.Zero() {
			res.Value = field.One()
		}
		return res

	case *PaddedCircularWindow:
		res := &PaddedCircularWindow{
			TotLen_: v.TotLen_,
			Offset_: v.Offset_,
			Window_: make([]field.Element, len(v.Window_)),
		}

		if v.PaddingVal_ == field.Zero() {
			res.PaddingVal_ = field.One()
		}

		for i := range res.Window_ {
			if v.Window_[i] == field.Zero() {
				res.Window_[i] = field.One()
			}
		}
		return res

	case *Rotated:
		res := make([]field.Element, len(v.v.Regular))
		for i := range res {
			if v.v.Regular[i] == field.Zero() {
				res[i] = field.One()
			}
		}
		return NewRotated(
			res,
			v.offset,
		)

	case *Regular:
		res := make([]field.Element, len(*v))
		for i := range res {
			if (*v)[i] == field.Zero() {
				res[i] = field.One()
			}
		}
		return NewRegular(res)

	case *Pooled:
		res := make([]field.Element, len(v.Regular))
		for i := range res {
			if v.Regular[i] == field.Zero() {
				res[i] = field.One()
			}
		}
		return NewRegular(res)
	}

	panic("unsupported type")
}

// Sum returns the field summation of all the elements contained in the vector
func Sum(a SmartVector) (res field.Element) {

	switch v := a.(type) {
	case *Regular:
		res := field.Zero()
		for i := range *v {
			res.Add(&res, &(*v)[i])
		}
		return res

	case *PaddedCircularWindow:
		res := field.Zero()
		for i := range v.Window_ {
			res.Add(&res, &v.Window_[i])
		}
		constTerm := field.NewElement(uint64(v.TotLen_ - len(v.Window_)))
		constTerm.Mul(&constTerm, &v.PaddingVal_)
		res.Add(&res, &constTerm)
		return res

	case *Constant:
		res := field.NewElement(uint64(v.Length))
		res.Mul(&res, &v.Value)
		return res

	case *Rotated:
		res := field.Zero()
		for i := range v.v.Regular {
			res.Add(&res, &v.v.Regular[i])
		}
		return res

	case *Pooled:
		res := field.Zero()
		for i := range v.Regular {
			res.Add(&res, &v.Regular[i])
		}
		return res

	default:
		utils.Panic("unsupported type: %T", v)
	}

	return res
}
