package smartvectors

import (
	"fmt"
	"iter"

	"github.com/consensys/linea-monorepo/prover/maths/common/vectorext"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"

	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// RegularExt is s normal vector in a nutshell
type RegularExt vectorext.Vector

// NewRegularExt instanstiate a new regular from a slice. Returns a pointer so that the result
// can be reused without referencing as a SmartVector.
func NewRegularExt(v []fext.Element) *RegularExt {
	assertStrictPositiveLen(len(v))
	res := RegularExt(v)
	return &res
}

// Len returns the length of the regular vector
func (r *RegularExt) Len() int { return len(*r) }

// GetBase returns a particular element of the vector
func (r *RegularExt) GetBase(n int) (field.Element, error) {
	return field.Zero(), errConversion
}

func (r *RegularExt) GetExt(n int) fext.Element {
	return (*r)[n]
}

func (r *RegularExt) Get(n int) field.Element {
	res, err := r.GetBase(n)
	if err != nil {
		panic(err)
	}
	return res
}

// SubVector returns a subvector of the regular type
func (r *RegularExt) SubVector(start, stop int) SmartVector {
	if start > stop {
		utils.Panic("Negative length are not allowed")
	}
	if start == stop {
		utils.Panic("Subvector of zero lengths are not allowed")
	}
	res := RegularExt((*r)[start:stop])
	return &res
}

// RotateRight rotates the vector into a new one
func (r *RegularExt) RotateRight(offset int) SmartVector {
	return NewRotatedExt(*r, -offset)
}

func (r *RegularExt) WriteInSlice(s []field.Element) {
	assertHasLength(len(s), len(*r))
	for i := 0; i < len(s); i++ {
		elem, _ := r.GetBase(i)
		s[i].Set(&elem)
	}

}

func (r *RegularExt) WriteInSliceExt(s []fext.Element) {
	assertHasLength(len(s), len(*r))
	for i := 0; i < len(s); i++ {
		elem := r.GetExt(i)
		s[i].Set(&elem)
	}
}

func (r *RegularExt) Pretty() string {
	return fmt.Sprintf("Regular[%v]", vectorext.Prettify(*r))
}

func processRegularOnlyExt(op operator, svecs []SmartVector, coeffs []int) (result RegularExt, numMatches int) {

	length := svecs[0].Len()

	var resvec RegularExt

	isFirst := true
	numMatches = 0

	for i := range svecs {

		svec := svecs[i]
		// In case the current vec is Rotated, we reduce it to a regular form
		// NB : this could use the pool.
		if rot, ok := svec.(*RotatedExt); ok {
			svec = rotatedAsRegularExt(rot)
		}

		if reg, ok := svec.(*RegularExt); ok {
			numMatches++
			// For the first one, we can save by just copying the result
			// Importantly, we do not need to assume that regRes is originally
			// zero.
			if isFirst {
				resvec = *NewRegularExt(make([]fext.Element, length))

				isFirst = false
				op.vecExtIntoTermExt(resvec, *reg, coeffs[i])
				continue
			}

			op.vecExtIntoVecExt(resvec, *reg, coeffs[i])
		}
	}

	if numMatches == 0 {
		return nil, 0
	}

	return resvec, numMatches
}

func (r *RegularExt) DeepCopy() SmartVector {
	return NewRegularExt(vectorext.DeepCopy(*r))
}

// Converts a smart-vector into a normal vec. The implementation minimizes
// then number of copies.
func (r *RegularExt) IntoRegVecSaveAlloc() []field.Element {
	panic(errConversion)
}

func (r *RegularExt) IntoRegVecSaveAllocBase() ([]field.Element, error) {
	return nil, errConversion
}

func (r *RegularExt) IntoRegVecSaveAllocExt() []fext.Element {
	return *r
}

func (r *RegularExt) IterateCompact() iter.Seq[field.Element] {
	panic("not available for extensions")
}

func (c *RegularExt) IterateSkipPadding() iter.Seq[field.Element] {
	panic("not available for extensions")
}

func (c *RegularExt) GetPtr(n int) *field.Element {
	panic("not available for extensions")
}
