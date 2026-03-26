package smartvectors

import (
	"fmt"
	"iter"
	"slices"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field/fext"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

// Rotated represents a rotated version of a regular smartvector and also
// implements the [SmartVector] interface. Rotated have a very niche use-case
// in the repository as they are used to help saving FFT operations in the
// [github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/arithmetic.CompileGlobal]
// compiler when the coset evaluation is done over a cyclic rotation of a
// smart-vector.
//
// Rotated works by abstractly storing the offset and only applying the rotation
// when the vector is written or sub-vectored. This makes rotations essentially
// free.
type Rotated struct {
	v      *Pooled
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
		v: &Pooled{Regular: reg}, offset: offset,
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
	return *new(fext.Element).SetFromBase(&temp)
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
	return &r.v.Regular[pos]
}

// Returns a particular element. The subvector is taken at indices
// [Start, Stop). (Stop being excluded from the span)
func (r *Rotated) SubVector(start, stop int) SmartVector {
	if stop+r.offset < len(r.v.Regular) && start+r.offset >= 0 {
		res := Regular(r.v.Regular[start+r.offset : stop+r.offset])
		return &res
	}
	res := make([]field.Element, stop-start)
	r.WriteSubVectorInSlice(start, stop, res)
	return NewRegular(res)
}

// Rotates the vector into a new one, a positive offset means a left cyclic shift
func (r *Rotated) RotateRight(offset int) SmartVector {
	// We limit the offset value to prevent integer overflow
	if offset > 1<<40 {
		utils.Panic("offset is too large")
	}
	return &Rotated{
		v: &Pooled{
			Regular: vector.DeepCopy(r.v.Regular),
		},
		offset: r.offset + offset,
	}
}

func (r *Rotated) DeepCopy() SmartVector {
	return NewRotated(vector.DeepCopy(r.v.Regular), r.offset)
}

func (r *Rotated) WriteInSlice(s []field.Element) {
	res := rotatedAsRegular(r)
	res.WriteInSlice(s)
}

func (r *Rotated) WriteInSliceExt(s []fext.Element) {
	temp := rotatedAsRegular(r)
	for i := 0; i < temp.Len(); i++ {
		elem, _ := temp.GetBase(i)
		s[i].SetFromBase(&elem)
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
		panic(conversionError)
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
		res[i].SetFromBase(&temp[i])
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
		return NewRotated(casted.v.Regular, utils.PositiveMod(offset+casted.offset, v.Len()))
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
	case *Pooled:
		return &Rotated{
			v:      casted,
			offset: offset,
		}
	default:
		utils.Panic("unknown type %T", v)
	}

	panic("unreachable")

}

// WriteSubVectorInSlice writes the subvector [start:stop) into the provided
// slice s. The slice s must be of length stop-start.
// This is equivalent to doing r.SubVector(start, stop).WriteInSlice(s) but
// avoids an allocation.
func (r *Rotated) WriteSubVectorInSlice(start, stop int, s []field.Element) {
	// sanity checks on start / stop / len(s)
	if start < 0 || stop > r.Len() || stop <= start || len(s) != stop-start {
		utils.Panic("invalid start/stop/len(s): start=%v, stop=%v, len(s)=%v, vector len=%v", start, stop, len(s), r.Len())
	}
	if stop+r.offset < len(r.v.Regular) && start+r.offset >= 0 {
		for i := 0; i < stop-start; i++ {
			s[i] = r.v.Regular[start+r.offset+i]
		}
		return
	}
	size := r.Len()
	spanSize := stop - start

	// normalize the offset to something positive [0: size)
	startWithOffsetClean := utils.PositiveMod(start+r.offset, size)

	// NB: we may need to construct the res in several steps
	for i := 0; i < min(size-startWithOffsetClean, spanSize); i++ {
		s[i] = r.v.Regular[startWithOffsetClean+i]
	}

	// If this is negative of zero, it means the first copy already copied
	// everything we needed to copy
	howManyElementLeftToCopy := startWithOffsetClean + spanSize - size
	howManyAlreadyCopied := spanSize - howManyElementLeftToCopy
	if howManyElementLeftToCopy <= 0 {
		return
	}

	// if necessary perform a second
	// copy(res[howManyAlreadyCopied:], r.v[:howManyElementLeftToCopy])
	for i := 0; i < howManyElementLeftToCopy; i++ {
		s[howManyAlreadyCopied+i] = r.v.Regular[i]
	}
	return
}
