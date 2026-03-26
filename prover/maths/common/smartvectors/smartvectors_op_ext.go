package smartvectors

import (
	"github.com/consensys/linea-monorepo/prover/maths/common/vectorext"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// ForTestExt returns a regular smartvector with field extensions of the vector xs
// The field extensions are simple wrappers of the base field, padded with 0s
func ForTestExt(xs ...int) SmartVector {
	return NewRegularExt(vectorext.ForTest(xs...))
}

// ForTestFromVect computes a regular smartvector of field extensions,
// where each field extension is populated using one vector of size [fext.ExtensionDegree]
func ForTestFromVect(xs ...[fext.ExtensionDegree]int) SmartVector {
	return NewRegularExt(vectorext.ForTestFromVect(xs...))
}

// ForTestFromQuads groups the inputs into pairs and computes a regular smartvector of
// field extensions, where each field extension has only the first two coordinates populated.
func ForTestFromQuads(xs ...int) SmartVector {
	return NewRegularExt(vectorext.ForTestFromQuads(xs...))
}

// LeftPadded creates a new padded vector (padded on the left)
func LeftPaddedExt(v []fext.Element, padding fext.Element, targetLen int) SmartVector {

	if len(v) > targetLen {
		utils.Panic("target length %v must be less than %v", len(v), targetLen)
	}

	if len(v) == targetLen {
		return NewRegularExt(v)
	}

	if len(v) == 0 {
		return NewConstantExt(padding, targetLen)
	}

	return NewPaddedCircularWindowExt(v, padding, targetLen-len(v), targetLen)
}

// RightPadded creates a new vector (padded on the right)
func RightPaddedExt(v []fext.Element, padding fext.Element, targetLen int) SmartVector {

	if len(v) > targetLen {
		utils.Panic("target length %v must be less than %v", len(v), targetLen)
	}

	if len(v) == targetLen {
		return NewRegularExt(v)
	}

	if len(v) == 0 {
		return NewConstantExt(padding, targetLen)
	}

	return NewPaddedCircularWindowExt(v, padding, 0, targetLen)
}

// RightZeroPadded creates a new vector (padded on the right)
func RightZeroPaddedExt(v []fext.Element, targetLen int) SmartVector {
	return RightPaddedExt(v, fext.Zero(), targetLen)
}

// LeftZeroPadded creates a new vector (padded on the left)
func LeftZeroPaddedExt(v []fext.Element, targetLen int) SmartVector {
	return LeftPaddedExt(v, fext.Zero(), targetLen)
}
