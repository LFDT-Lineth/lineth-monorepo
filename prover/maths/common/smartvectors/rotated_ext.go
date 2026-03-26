package smartvectors

import (
	"fmt"
	"iter"

	"github.com/consensys/linea-monorepo/prover/maths/common/vectorext"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// Rotated represents a rotated version of a regular smartvector and also
// implements the [SmartVector] interface. Rotated have a very niche use-case
// in the repository as they are used to help saving FFT operations in the
// [github.com/consensys/linea-monorepo/prover/protocol/compiler/arithmetic.CompileGlobal]
// compiler when the coset evaluation is done over a cyclic rotation of a
// smart-vector.
//
// Rotated works by abstractly storing the offset and only applying the rotation
// when the vector is written or sub-vectored. This makes rotations essentially
// free.
type RotatedExt struct {
	v      RegularExt
	offset int
}

// NewRotated constructs a new Rotated, positive offset means a cyclic left shift.
func NewRotatedExt(reg RegularExt, offset int) *RotatedExt {

	// empty vector
	if len(reg) == 0 {
		utils.Panic("got an empty vector")
	}

	// negative offset is not allowed
	if offset < 0 {
		if -offset > len(reg) {
			utils.Panic("len %v is less than, offset %v", len(reg), offset)
		}
	}

	// offset larger than the vector itself
	if offset > len(reg) {
		utils.Panic("len %v, is less than, offset %v", len(reg), offset)
	}

	return &RotatedExt{
		v: reg, offset: offset,
	}
}

// Returns the lenght of the vector
func (r *RotatedExt) Len() int {
	return r.v.Len()
}

// Returns a particular element of the vector
func (r *RotatedExt) GetBase(n int) (field.Element, error) {
	return field.Zero(), errConversion
}

// Returns a particular element of the vector
func (r *RotatedExt) GetExt(n int) fext.Element {
	return r.v.GetExt(utils.PositiveMod(n+r.offset, r.Len()))
}

func (r *RotatedExt) Get(n int) field.Element {
	res, err := r.GetBase(n)
	if err != nil {
		panic(err)
	}
	return res
}

// TODO @gbotrel WriteSubVectorInSliceExt should be in interface and available for regular too.
// review smart vector naming scheme and make it more consistant with canonical golang slices.

func (r *RotatedExt) SubVector(start, stop int) SmartVector {
	res := make([]fext.Element, stop-start)
	copySubVector(r.v, r.offset, start, stop, res)
	ret := RegularExt(res)
	return &ret
}

func (r *RotatedExt) WriteSubVectorInSliceExt(start, stop int, s []fext.Element) {
	copySubVector(r.v, r.offset, start, stop, s)
}

// Rotates the vector into a new one, a positive offset means a left cyclic shift
func (r *RotatedExt) RotateRight(offset int) SmartVector {
	// We limit the offset value to prevent integer overflow
	if offset > 1<<40 {
		utils.Panic("offset is too large")
	}
	return &RotatedExt{
		v:      vectorext.DeepCopy(r.v),
		offset: r.offset + offset,
	}
}

func (r *RotatedExt) DeepCopy() SmartVector {
	return NewRotatedExt(vectorext.DeepCopy(r.v), r.offset)
}

func (r *RotatedExt) WriteInSlice(s []field.Element) {
	panic(errConversion)
}

func (r *RotatedExt) WriteInSliceExt(s []fext.Element) {
	r.WriteSubVectorInSliceExt(0, r.Len(), s)
}

func (r *RotatedExt) Pretty() string {
	return fmt.Sprintf("Rotated[%v, %v]", r.v.Pretty(), r.offset)
}

// rotatedAsRegular converts a [Rotated] into a [Regular] by effecting the
// symbolic shifting operation. The function allocates the result.
func rotatedAsRegularExt(r *RotatedExt) *RegularExt {
	return r.SubVector(0, r.Len()).(*RegularExt)
}

func (r *RotatedExt) IntoRegVecSaveAlloc() []field.Element {
	panic(errConversion)
}

func (r *RotatedExt) IntoRegVecSaveAllocBase() ([]field.Element, error) {
	return nil, errConversion
}

func (r *RotatedExt) IntoRegVecSaveAllocExt() []fext.Element {
	temp := *rotatedAsRegularExt(r)
	res := make([]fext.Element, temp.Len())
	for i := 0; i < temp.Len(); i++ {
		elem := r.GetExt(i)
		temp[i].Set(&elem)
	}
	return res
}

func (r *RotatedExt) IterateCompact() iter.Seq[field.Element] {
	panic("not available for extensions")
}

func (c *RotatedExt) IterateSkipPadding() iter.Seq[field.Element] {
	panic("not available for extensions")
}

func (c *RotatedExt) GetPtr(n int) *field.Element {
	panic("not available for extensions")
}

// SoftRotate converts v into a [SmartVector] representing the same
// [SmartVector]. The function tries to not reallocate the result. This means
// that changing the v can subsequently affects the result of this function.
func SoftRotateExt(v SmartVector, offset int) SmartVector {

	switch casted := v.(type) {
	case *RegularExt:
		return NewRotatedExt(*casted, offset)
	case *RotatedExt:
		return NewRotatedExt(casted.v, utils.PositiveMod(offset+casted.offset, v.Len()))
	case *PaddedCircularWindowExt:
		return NewPaddedCircularWindowExt(
			casted.Window_,
			casted.PaddingVal_,
			utils.PositiveMod(casted.Offset+offset, casted.Len()),
			casted.Len(),
		)
	case *ConstantExt:
		// It's a constant so it does not need to be rotated
		return v
	default:
		utils.Panic("unknown type %T", v)
	}

	panic("unreachable")

}
