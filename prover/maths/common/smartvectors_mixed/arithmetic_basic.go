package smartvectors_mixed

import (
	sv "github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/utils"
)

func IsBase(vec sv.SmartVector) bool {
	_, isBaseError := vec.GetBase(0)
	if isBaseError == nil {
		return true
	} else {
		return false
	}
}

func LiftToExt(vec sv.SmartVector) sv.SmartVector {
	if !IsBase(vec) {
		panic("cannot lift to extension, vector is not base")
	}
	switch v := vec.(type) {
	case *sv.Constant:
		res := sv.NewConstantExt(
			fext.Lift(v.Val()),
			v.Len(),
		)
		return res
	case *sv.PaddedCircularWindow:
		windowExt := make([]fext.Element, len(v.Window()))
		for i := range v.Window() {
			fext.SetFromBase(&windowExt[i], &v.Window()[i])
		}
		res := sv.NewPaddedCircularWindowExt(
			windowExt,
			fext.Lift(v.PaddingVal()),
			v.Offset(),
			v.Len(),
		)
		return res
	case *sv.Rotated:
		vecExt := make([]fext.Element, v.Len())
		v.WriteInSliceExt(vecExt)
		return sv.NewRegularExt(vecExt)
	case *sv.Regular:
		vecExt := make([]fext.Element, v.Len())
		v.WriteInSliceExt(vecExt)
		return sv.NewRegularExt(vecExt)
	default:
		utils.Panic("unknown type %T", v)
	}
	panic("unsupported type")
}

func Mul(vecs ...sv.SmartVector) sv.SmartVector {
	coeffs := make([]int, len(vecs))
	for i := range coeffs {
		coeffs[i] = 1
	}

	return ProductMixed(coeffs, vecs)
}

// ScalarMul returns a smart-vector obtained by  multiplying a scalar with a [SmartVector].
//   - the output smart-vector has the same size as the input vector
func ScalarMul(vec sv.SmartVector, x field.Element) sv.SmartVector {
	xVec := sv.NewConstant(x, vec.Len())
	return Mul(vec, xVec)
}
func executeFuncOnBaseExt(
	vecs []sv.SmartVector,
	baseOperation func(vec ...sv.SmartVector) sv.SmartVector,
	extOperation func(vec ...sv.SmartVector) sv.SmartVector,
	finalOperation func(vec ...sv.SmartVector) sv.SmartVector,
	neutralElement field.Element,
) sv.SmartVector {
	vecsBase, vecsExt := SeparateBaseAndExtVectors(vecs)

	var res sv.SmartVector = sv.NewConstant(neutralElement, vecs[0].Len())

	if len(vecsBase) > 0 {
		res = baseOperation(vecsBase...)
	}

	if len(vecsExt) == 0 {
		// no extension vectors, return the base result
		return res
	} else {
		// there are some extension vectors present
		// apply the extension operation to the extension vectors
		addExt := extOperation(vecsExt...)
		// lift the base result to extension representation and then apply the extension operation
		liftedBase := LiftToExt(res)
		return finalOperation(liftedBase, addExt)
	}
}

func SeparateBaseAndExtVectors(vecs []sv.SmartVector) ([]sv.SmartVector, []sv.SmartVector) {
	vecsBase := make([]sv.SmartVector, 0, len(vecs))
	vecsExt := make([]sv.SmartVector, 0, len(vecs))
	for _, vec := range vecs {
		if IsBase(vec) {
			vecsBase = append(vecsBase, vec)
		} else {
			vecsExt = append(vecsExt, vec)
		}
	}
	return vecsBase, vecsExt
}

func SeparateBaseAndExtVectorsWithCoeffs(coeffs []int, vecs []sv.SmartVector) ([]sv.SmartVector, []sv.SmartVector, []int, []int) {
	vecsBase := make([]sv.SmartVector, 0, len(vecs))
	vecsExt := make([]sv.SmartVector, 0, len(vecs))
	coeffsBase := make([]int, 0, len(vecs))
	coeffsExt := make([]int, 0, len(vecs))

	for index, vec := range vecs {
		if IsBase(vec) {
			vecsBase = append(vecsBase, vec)
			coeffsBase = append(coeffsBase, coeffs[index])
		} else {
			vecsExt = append(vecsExt, vec)
			coeffsExt = append(coeffsExt, coeffs[index])
		}
	}
	return vecsBase, vecsExt, coeffsBase, coeffsExt
}

func ExecuteFuncOnBaseExtWithMempool(
	vecs []sv.SmartVector,
	baseOperation func(vec []sv.SmartVector) sv.SmartVector,
	extOperation func(vec []sv.SmartVector) sv.SmartVector,
	finalOperation func(vec ...sv.SmartVector) sv.SmartVector,
) sv.SmartVector {
	vecsBase, vecsExt := SeparateBaseAndExtVectors(vecs)

	var res sv.SmartVector = sv.NewConstant(field.Zero(), vecs[0].Len())
	if len(vecsBase) > 0 {
		res = baseOperation(vecsBase)
	}

	if len(vecsExt) == 0 {
		// no extension vectors, return the base result
		return res
	} else {
		// there are some extension vectors present
		// apply the extension operation to the extension vectors
		addExt := extOperation(vecsExt)
		// lift the base result to extension representation and then apply the extension operation
		liftedBase := LiftToExt(res)
		return finalOperation(liftedBase, addExt)
	}
}

// AddMixed returns a smart-vector obtained by position-wise adding [SmartVector].
//   - all inputs `vecs` must have the same size, or the function panics
//   - the output smart-vector has the same size as the input vectors
//   - if no input vectors are provided, the function panics
func AddMixed(vecs ...sv.SmartVector) sv.SmartVector {
	return executeFuncOnBaseExt(
		vecs,
		sv.Add,
		sv.AddExt,
		sv.AddExt,
		field.Zero(),
	)
}

// MulMixed returns a smart-vector obtained by position-wise multiplying [SmartVector].
//   - all inputs `vecs` must have the same size, or the function panics
//   - the output smart-vector has the same size as the input vectors
//   - if no input vectors are provided, the function panics
func MulMixed(vecs ...sv.SmartVector) sv.SmartVector {
	return executeFuncOnBaseExt(
		vecs,
		sv.Mul,
		sv.MulExt,
		sv.MulExt,
		field.One(),
	)
}
