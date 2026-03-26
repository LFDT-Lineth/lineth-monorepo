package smartvectors

import (
	"github.com/consensys/gnark-crypto/field/koalabear/extensions"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// LinearCombinationMixed returns a [SmartVector] computed as:
//
//	result = vecs[0] + vecs[1] * x + vecs[2] * x^2 + vecs[3] * x^3 + ...
//
// where `x` is a scalar in fext and `vecs[i]` are [SmartVector] holding field elements
func LinearCombinationMixed(vecs []SmartVector, x fext.Element) (result SmartVector) {

	if len(vecs) == 0 {
		panic("no input vectors")
	}

	length := vecs[0].Len()

	// Preallocate the intermediate values
	resReg := make([]fext.Element, length)
	tmpVec := make([]field.Element, length)

	var tmpF, resCon fext.Element
	var anyReg, anyCon bool
	xPow := fext.One()

	// accumulateRegMixed := func(acc []fext.Element, v []field.Element, x fext.Element) {
	// 	for i := 0; i < length; i++ {
	// 		tmpF.MulByElement(&x, &v[i])
	// 		acc[i].Add(&acc[i], &tmpF)
	// 	}
	// }

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
		if asRotated, ok := v.(*Rotated); ok {
			v = rotatedAsRegular(asRotated)
		}

		switch casted := v.(type) {
		case *Constant:
			anyCon = true
			tmpF.MulByElement(&xPow, &casted.Value)
			resCon.Add(&resCon, &tmpF)
		case *Regular:
			anyReg = true
			v := field.Vector(*casted)
			accumulateRegMixed(resReg, v, xPow)
		case *PaddedCircularWindow:
			// treat it as a regular, reusing the buffer
			anyReg = true
			casted.WriteInSlice(tmpVec)
			accumulateRegMixed(resReg, tmpVec, xPow)
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
