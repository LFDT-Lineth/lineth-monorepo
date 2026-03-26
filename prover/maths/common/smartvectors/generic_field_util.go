package smartvectors

import (
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
)

func ToConstantSmartvector(e *fext.GenericFieldElem, length int) SmartVector {
	if e.GetIsBase() {
		baseElem, _ := e.GetBase()
		return NewConstant(baseElem, length)
	} else {
		return NewConstantExt(e.GetExt(), length)
	}
}

func GetGenericElemOfSmartvector(vector SmartVector, index int) fext.GenericFieldElem {
	if IsBase(vector) {
		elem, _ := vector.GetBase(index)
		return fext.NewGenFieldFromBase(elem)
	}
	// If the vector is not over base elements, we assume it is over extensions
	elem := vector.GetExt(index)
	return fext.NewGenFieldFromExt(elem)
}
