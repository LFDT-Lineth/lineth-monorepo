package smartvectorsext

import (
	"fmt"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/vectorext"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field/fext/gnarkfext"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

// ForTestExt returns a regular smartvector with field extensions of the vector xs
// The field extensions are simple wrappers of the base field, padded with 0s
func ForTestExt(xs ...int) smartvectors.SmartVector {
	return NewRegularExt(vectorext.ForTest(xs...))
}

// ForTestFromVect computes a regular smartvector of field extensions,
// where each field extension is populated using one vector of size [fext.ExtensionDegree]
func ForTestFromVect(xs ...[fext.ExtensionDegree]int) smartvectors.SmartVector {
	return NewRegularExt(vectorext.ForTestFromVect(xs...))
}

// ForTestFromPairs groups the inputs into pairs and computes a regular smartvector of
// field extensions, where each field extension has only the first two coordinates populated.
func ForTestFromPairs(xs ...int) smartvectors.SmartVector {
	return NewRegularExt(vectorext.ForTestFromPairs(xs...))
}

// IntoRegVec converts a smart-vector into a normal vec. The resulting vector
// is always reallocated and can be safely mutated without side-effects
// on s.
func IntoRegVec(s smartvectors.SmartVector) []field.Element {
	panic(conversionError)
}

// IntoRegVecExt converts a smart-vector into a normal vector of field extensions.
// The resulting vector is always reallocated and can be safely mutated without side-effects
// on s.
func IntoRegVecExt(s smartvectors.SmartVector) []fext.Element {
	res := make([]fext.Element, s.Len())
	s.WriteInSliceExt(res)
	return res
}

// IntoGnarkAssignment converts a smart-vector into a gnark assignment
func IntoGnarkAssignment(sv smartvectors.SmartVector) []gnarkfext.Variable {
	res := make([]gnarkfext.Variable, sv.Len())
	for i := range res {
		elem := sv.GetExt(i)
		res[i] = gnarkfext.Variable{
			A0: frontend.Variable(elem.A0),
			A1: frontend.Variable(elem.A1),
		}
	}
	return res
}

// LeftPadded creates a new padded vector (padded on the left)
func LeftPadded(v []fext.Element, padding fext.Element, targetLen int) smartvectors.SmartVector {

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
func RightPadded(v []fext.Element, padding fext.Element, targetLen int) smartvectors.SmartVector {

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
func RightZeroPadded(v []fext.Element, targetLen int) smartvectors.SmartVector {
	return RightPadded(v, fext.Zero(), targetLen)
}

// LeftZeroPadded creates a new vector (padded on the left)
func LeftZeroPadded(v []fext.Element, targetLen int) smartvectors.SmartVector {
	return LeftPadded(v, fext.Zero(), targetLen)
}

// Density returns the density of a smart-vector. By density we mean the size
// of the concrete underlying vectors. This can be used as a proxi for the
// memory required to store the smart-vector.
func Density(v smartvectors.SmartVector) int {
	switch w := v.(type) {
	case *ConstantExt:
		return 0
	case *PaddedCircularWindowExt:
		return len(w.window)
	case *RegularExt:
		return len(*w)
	case *RotatedExt:
		return len(w.v.RegularExt)
	case *PooledExt:
		return len(w.RegularExt)
	default:
		panic(fmt.Sprintf("unexpected type %T", v))
	}
}

// Window returns the effective window of the vector,
// if the vector is Padded with zeroes it return the window.
// Namely, the part without zero pads.
func Window(v smartvectors.SmartVector) []fext.Element {
	switch w := v.(type) {
	case *ConstantExt:
		return w.IntoRegVecSaveAllocExt()
	case *PaddedCircularWindowExt:
		return w.window
	case *RegularExt:
		return *w
	case *RotatedExt:
		return w.IntoRegVecSaveAllocExt()
	default:
		panic(fmt.Sprintf("unexpected type %T", v))
	}
}

func WindowExt(v smartvectors.SmartVector) []fext.Element {
	switch w := v.(type) {
	case *ConstantExt:
		return w.IntoRegVecSaveAllocExt()
	case *PaddedCircularWindowExt:
		temp := make([]fext.Element, len(w.window))
		for i := 0; i < len(w.window); i++ {
			elem := w.window[i]
			temp[i].Set(&elem)
		}
		return temp
	case *RegularExt:
		temp := make([]fext.Element, len(*w))
		for i := 0; i < len(*w); i++ {
			elem := w.GetExt(i)
			temp[i].Set(&elem)
		}
		return temp
	case *RotatedExt:
		return w.IntoRegVecSaveAllocExt()
	default:
		panic(fmt.Sprintf("unexpected type %T", v))
	}
}
