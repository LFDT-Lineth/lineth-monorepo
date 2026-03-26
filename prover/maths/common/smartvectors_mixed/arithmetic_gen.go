package smartvectors_mixed

import (
	sv "github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// LinCombMixed computes a linear combination of the given vectors with integer coefficients.
//   - The function panics if provided sv.SmartVector of different lengths
//   - The function panics if svecs is empty
//   - The function panics if the length of coeffs does not match the length of
//     svecs
func LinCombMixed(coeffs []int, svecs []sv.SmartVector) sv.SmartVector {
	vecsBase, vecsExt, coeffsBase, coeffsExt := SeparateBaseAndExtVectorsWithCoeffs(coeffs, svecs)
	// compute the base result
	var resBase sv.SmartVector = sv.NewConstant(field.Zero(), svecs[0].Len())
	if len(vecsBase) > 0 {
		resBase = sv.LinComb(coeffsBase, vecsBase)
	}

	if len(vecsExt) == 0 {
		// no extension vectors, return the base result
		return resBase
	} else {
		// sanity check that the base and extension vectors have the same length
		// but only if the vecBase are present at all
		if len(vecsBase) > 0 && vecsBase[0].Len() != vecsExt[0].Len() {
			utils.Panic("base and extension vectors should have the same length")
		}
		// there are some extension vectors present
		// apply the extension operation to the extension vectors
		resExt := sv.LinCombExt(coeffsExt, vecsExt)
		// lift the base result to extension representation and then apply the extension operation
		liftedBase := LiftToExt(resBase)
		return AddMixed(liftedBase, resExt)
	}
}

// ProductMixed computes a product of smart-vectors with integer exponents
//   - The function panics if provided sv.SmartVector of different lengths
//   - The function panics if svecs is empty
//   - The function panics if the length of exponents does not match the length of
//     svecs
func ProductMixed(exponents []int, svecs []sv.SmartVector) sv.SmartVector {
	vecsBase, vecsExt, expBase, expExt := SeparateBaseAndExtVectorsWithCoeffs(exponents, svecs)
	// compute the base result
	var resBase sv.SmartVector = sv.NewConstant(field.One(), svecs[0].Len())
	if len(vecsBase) > 0 {
		resBase = sv.Product(expBase, vecsBase)
	}

	if len(vecsExt) == 0 {
		// no extension vectors, return the base result
		return resBase
	} else {
		// sanity check that the base and extension vectors have the same length
		// but only if the vecBase are present at all
		if len(vecsBase) > 0 && vecsBase[0].Len() != vecsExt[0].Len() {
			utils.Panic("base and extension vectors should have the same length")
		}
		// there are some extension vectors present
		// apply the extension operation to the extension vectors
		resExt := sv.ProductExt(expExt, vecsExt)
		// lift the base result to extension representation and then apply the extension operation
		liftedBase := LiftToExt(resBase)
		return MulMixed(liftedBase, resExt)
	}
}
