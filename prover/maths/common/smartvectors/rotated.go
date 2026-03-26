package smartvectors

import (
	"fmt"
	"iter"
	"slices"

	"github.com/consensys/linea-monorepo/prover/maths/field/fext"

	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/field"
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
type Rotated struct {
	v      Regular
	offset int
}

// NewRotated constructs a new Rotated, positive offset means a cyclic left shift.
func NewRotated(reg Regular, offset int) *Rotated {

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

	return &Rotated{
		v:      reg,
		offset: offset,
	}
}

// Returns the lenght of the vector
func (r *Rotated) Len() int {
	return r.v.Len()
}

// Returns a particular element of the vector
func (r *Rotated) GetBase(n int) (field.Element, error) {
	return r.v.GetBase(utils.PositiveMod(n+r.offset, r.Len()))
}

// Returns a particular element of the vector
func (r *Rotated) GetExt(n int) fext.Element {
	temp, _ := r.v.GetBase(utils.PositiveMod(n+r.offset, r.Len()))
	return fext.Lift(temp)
}

func (r *Rotated) Get(n int) field.Element {
	res, err := r.GetBase(n)
	if err != nil {
		panic(err)
	}
	return res
}

func (r *Rotated) GetPtr(n int) *field.Element {
	pos := utils.PositiveMod(n+r.offset, r.Len())
	return &r.v[pos]
}

// TODO @gbotrel this SubVector APIs needs some cleaning

// Returns a particular element. The subvector is taken at indices
// [start, stop). (stop being excluded from the span)
func (r *Rotated) SubVector(start, stop int) SmartVector {
	res := make([]field.Element, stop-start)
	copySubVector(r.v, r.offset, start, stop, res)
	ret := Regular(res)
	return &ret
}

func (r *Rotated) WriteSubVectorInSlice(start, stop int, s []field.Element) {
	copySubVector(r.v, r.offset, start, stop, s)
}

func (r *Rotated) WriteSubVectorInSliceExt(start, stop int, s []fext.Element) {
	// sanity checks on start / stop / len(s)
	if start < 0 || stop > r.Len() || stop <= start || len(s) != stop-start {
		utils.Panic("invalid start/stop/len(s): start=%v, stop=%v, len(s)=%v, vector len=%v", start, stop, len(s), r.Len())
	}

	// Fast path: contiguous slice
	if stop+r.offset < len(r.v) && start+r.offset >= 0 {
		for i := 0; i < stop-start; i++ {
			fext.SetFromBase(&s[i], &r.v[start+r.offset+i])
		}
		return
	}

	size := r.Len()
	spanSize := stop - start

	// normalize the offset to something positive [0: size)
	startWithOffsetClean := utils.PositiveMod(start+r.offset, size)

	// First part
	n1 := min(size-startWithOffsetClean, spanSize)
	for i := 0; i < n1; i++ {
		fext.SetFromBase(&s[i], &r.v[startWithOffsetClean+i])
	}

	// Second part (wrap around)
	howManyElementLeftToCopy := spanSize - n1
	if howManyElementLeftToCopy > 0 {
		for i := 0; i < howManyElementLeftToCopy; i++ {
			fext.SetFromBase(&s[n1+i], &r.v[i])
		}
	}
}

// copySubVector handles the rotation logic for copying a sub-segment of a slice
// into a destination slice, accounting for the cyclic shift defined by offset.
func copySubVector[T any](src []T, offset, start, stop int, dst []T) {
	size := len(src)
	spanSize := stop - start

	// sanity checks
	if start < 0 || stop > size || stop <= start || len(dst) != spanSize {
		utils.Panic("invalid subvector range or destination size: start=%v, stop=%v, size=%v, len(dst)=%v", start, stop, size, len(dst))
	}

	// Fast path: the requested range (shifted) falls within the physical slice bounds without wrapping.
	// Note: start+offset could be negative if offset is negative, but NewRotated ensures offset is somewhat sane relative to len?
	// Actually NewRotated allows negative offsets but checks bounds. However, here we usually normalize or check.
	// Let's check if the physical range [start+offset, stop+offset) is valid in src.
	// Since offset can be arbitrary (though usually normalized in constructor, but let's be safe or rely on logic below).
	// The logic below normalizes startWithOffsetClean.
	// However, if we want a fast path for non-wrapping:
	// We need to check if the logical range [start, stop) maps to a contiguous physical range.
	// The mapping is index -> (index + offset) % size.
	// It is contiguous if (start + offset) % size + (stop - start) <= size (and no wrap around 0).
	// Or simply:
	startWithOffsetClean := utils.PositiveMod(start+offset, size)
	if startWithOffsetClean+spanSize <= size {
		copy(dst, src[startWithOffsetClean:startWithOffsetClean+spanSize])
		return
	}

	// Slow path: wrapping around the end of the slice.
	// 1. Copy from startWithOffsetClean to end of src
	n1 := size - startWithOffsetClean
	copy(dst, src[startWithOffsetClean:])

	// 2. Copy the remainder from the beginning of src
	n2 := spanSize - n1
	copy(dst[n1:], src[:n2])
}

// Rotates the vector into a new one, a positive offset means a left cyclic shift
func (r *Rotated) RotateRight(offset int) SmartVector {
	// We limit the offset value to prevent integer overflow
	if offset > 1<<40 {
		utils.Panic("offset is too large")
	}
	res := r.DeepCopy()
	res.(*Rotated).offset += offset
	return res
}

func (r *Rotated) DeepCopy() SmartVector {
	return NewRotated(vector.DeepCopy(r.v), r.offset)
}

func (r *Rotated) WriteInSlice(s []field.Element) {
	r.WriteSubVectorInSlice(0, r.Len(), s)
	// res := rotatedAsRegular(r)
	// res.Write(s)
}

func (r *Rotated) WriteInSliceExt(s []fext.Element) {
	temp := rotatedAsRegular(r)
	for i := 0; i < temp.Len(); i++ {
		elem, _ := temp.GetBase(i)
		fext.SetFromBase(&s[i], &elem)
	}
}

func (r *Rotated) Pretty() string {
	return fmt.Sprintf("Rotated[%v, %v]", r.v.Pretty(), r.offset)
}

// rotatedAsRegular converts a [Rotated] into a [Regular] by effecting the
// symbolic shifting operation. The function allocates the result.
func rotatedAsRegular(r *Rotated) *Regular {
	return r.SubVector(0, r.Len()).(*Regular)
}

func (r *Rotated) IntoRegVecSaveAlloc() []field.Element {
	res, err := r.IntoRegVecSaveAllocBase()
	if err != nil {
		panic(errConversion)
	}
	return res
}

func (r *Rotated) IntoRegVecSaveAllocBase() ([]field.Element, error) {
	return *rotatedAsRegular(r), nil
}

func (r *Rotated) IntoRegVecSaveAllocExt() []fext.Element {
	temp := *rotatedAsRegular(r)
	res := make([]fext.Element, temp.Len())
	for i := 0; i < temp.Len(); i++ {
		fext.SetFromBase(&res[i], &temp[i])
	}
	return res
}

func (r *Rotated) IntoRegVec() []field.Element {
	return *rotatedAsRegular(r)
}

// IterateCompact returns an iterator over the elements of the Rotated.
// It is not very smart as it reallocate the slice but that should not
// matter as this is never called in practice.
func (r *Rotated) IterateCompact() iter.Seq[field.Element] {
	all := r.IntoRegVec()
	return slices.Values(all)
}

// IterateSkipPadding returns an interator over all the elements of the
// smart-vector. The function reallocates under the hood.
func (r *Rotated) IterateSkipPadding() iter.Seq[field.Element] {
	return r.IterateCompact()
}

// SoftRotate converts v into a [SmartVector] representing the same
// [SmartVector]. The function tries to not reallocate the result. This means
// that changing the v can subsequently affects the result of this function.
func SoftRotate(v SmartVector, offset int) SmartVector {

	switch casted := v.(type) {
	case *Regular:
		return NewRotated(*casted, offset)
	case *Rotated:
		return NewRotated(casted.v, utils.PositiveMod(offset+casted.offset, v.Len()))
	case *PaddedCircularWindow:
		return NewPaddedCircularWindow(
			casted.Window_,
			casted.PaddingVal_,
			utils.PositiveMod(casted.Offset_+offset, casted.Len()),
			casted.Len(),
		)
	case *Constant:
		// It's a constant so it does not need to be rotated
		return v
	default:
		utils.Panic("unknown type %T", v)
	}

	panic("unreachable")

}
