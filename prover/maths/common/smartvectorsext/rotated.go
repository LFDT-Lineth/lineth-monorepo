package smartvectorsext

import (
	"fmt"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
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
	v      *PooledExt
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
		v: &PooledExt{RegularExt: reg}, offset: offset,
	}
}

// Returns the lenght of the vector
func (r *RotatedExt) Len() int {
	return r.v.Len()
}

// Returns a particular element of the vector
func (r *RotatedExt) GetBase(n int) (field.Element, error) {
	return field.Zero(), fmt.Errorf(conversionError)
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

// Returns a particular element. The subvector is taken at indices
// [start, stop). (stop being excluded from the span)
func (r *RotatedExt) SubVector(start, stop int) smartvectors.SmartVector {

	if stop+r.offset < len(r.v.RegularExt) && start+r.offset > 0 {
		res := RegularExt(r.v.RegularExt[start+r.offset : stop+r.offset])
		return &res
	}

	res := make([]fext.Element, stop-start)
	size := r.Len()
	spanSize := stop - start

	// checking
	if stop <= start {
		utils.Panic("the start %v >= stop %v", start, stop)
	}

	// boundary checks
	if start < 0 {
		utils.Panic("the start value was negative %v", start)
	}

	if stop > size {
		utils.Panic("the stop is OOO : %v (the length is %v)", stop, size)
	}

	// normalize the offset to something positive [0: size)
	startWithOffsetClean := utils.PositiveMod(start+r.offset, size)

	// NB: we may need to construct the res in several steps
	// in case
	copy(res, r.v.RegularExt[startWithOffsetClean:utils.Min(size, startWithOffsetClean+spanSize)])

	// If this is negative of zero, it means the first copy already copied
	// everything we needed to copy
	howManyElementLeftToCopy := startWithOffsetClean + spanSize - size
	howManyAlreadyCopied := spanSize - howManyElementLeftToCopy
	if howManyElementLeftToCopy <= 0 {
		ret := RegularExt(res)
		return &ret
	}

	// if necessary perform a second
	copy(res[howManyAlreadyCopied:], r.v.RegularExt[:howManyElementLeftToCopy])
	ret := RegularExt(res)
	return &ret
}

// Rotates the vector into a new one, a positive offset means a left cyclic shift
func (r *RotatedExt) RotateRight(offset int) smartvectors.SmartVector {
	// We limit the offset value to prevent integer overflow
	if offset > 1<<40 {
		utils.Panic("offset is too large")
	}
	return &RotatedExt{
		v: &PooledExt{
			RegularExt: vectorext.DeepCopy(r.v.RegularExt),
		},
		offset: r.offset + offset,
	}
}

func (r *RotatedExt) DeepCopy() smartvectors.SmartVector {
	return NewRotatedExt(vectorext.DeepCopy(r.v.RegularExt), r.offset)
}

func (r *RotatedExt) WriteInSlice(s []field.Element) {
	panic(conversionError)
}

func (r *RotatedExt) WriteInSliceExt(s []fext.Element) {
	temp := rotatedAsRegular(r)
	assertHasLength(len(s), len(*temp))
	copy(s, *temp)
}

func (r *RotatedExt) Pretty() string {
	return fmt.Sprintf("Rotated[%v, %v]", r.v.Pretty(), r.offset)
}

// rotatedAsRegular converts a [Rotated] into a [Regular] by effecting the
// symbolic shifting operation. The function allocates the result.
func rotatedAsRegular(r *RotatedExt) *RegularExt {
	return r.SubVector(0, r.Len()).(*RegularExt)
}

func (r *RotatedExt) IntoRegVecSaveAlloc() []field.Element {
	panic(conversionError)
}

func (r *RotatedExt) IntoRegVecSaveAllocBase() ([]field.Element, error) {
	return nil, fmt.Errorf(conversionError)
}

func (r *RotatedExt) IntoRegVecSaveAllocExt() []fext.Element {
	temp := *rotatedAsRegular(r)
	res := make([]fext.Element, temp.Len())
	for i := 0; i < temp.Len(); i++ {
		res[i].Set(&temp[i])
	}
	return res
}

// SoftRotate converts v into a [SmartVector] representing the same
// [SmartVector]. The function tries to not reallocate the result. This means
// that changing the v can subsequently affects the result of this function.
func SoftRotate(v smartvectors.SmartVector, offset int) smartvectors.SmartVector {

	switch casted := v.(type) {
	case *RegularExt:
		return NewRotatedExt(*casted, offset)
	case *RotatedExt:
		return NewRotatedExt(casted.v.RegularExt, utils.PositiveMod(offset+casted.offset, v.Len()))
	case *PaddedCircularWindowExt:
		return NewPaddedCircularWindowExt(
			casted.window,
			casted.paddingVal,
			utils.PositiveMod(casted.offset+offset, casted.Len()),
			casted.Len(),
		)
	case *ConstantExt:
		// It's a constant so it does not need to be rotated
		return v
	case *PooledExt:
		return &RotatedExt{
			v:      casted,
			offset: offset,
		}
	default:
		utils.Panic("unknown type %T", v)
	}

	panic("unreachable")

}
