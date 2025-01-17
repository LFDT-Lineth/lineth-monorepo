package smartvectorsext

import (
	"fmt"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/vectorext"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"

	"github.com/consensys/linea-monorepo/prover/maths/common/mempoolext"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils"
)

const conversionError = "smartvector holds field extensions, but a base element was requested"

// RegularExt is s normal vector in a nutshell
type RegularExt []fext.Element

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
	return field.Zero(), fmt.Errorf(conversionError)
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
func (r *RegularExt) SubVector(start, stop int) smartvectors.SmartVector {
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
func (r *RegularExt) RotateRight(offset int) smartvectors.SmartVector {
	resSlice := make(RegularExt, r.Len())

	if offset == 0 {
		copy(resSlice, *r)
		return &resSlice
	}

	if offset > 0 {
		// v and w may be the same vector thus we should use a
		// separate leftover buffer for temporary memory buffers.
		cutAt := len(*r) - offset
		leftovers := vectorext.DeepCopy((*r)[cutAt:])
		copy(resSlice[offset:], (*r)[:cutAt])
		copy(resSlice[:offset], leftovers)
		return &resSlice
	}

	if offset < 0 {
		glueAt := len(*r) + offset
		leftovers := vectorext.DeepCopy((*r)[:-offset])
		copy(resSlice[:glueAt], (*r)[-offset:])
		copy(resSlice[glueAt:], leftovers)
		return &resSlice
	}

	panic("unreachable")
}

func (r *RegularExt) WriteInSlice(s []field.Element) {
	panic(conversionError)
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

func processRegularOnlyExt(op operator, svecs []smartvectors.SmartVector, coeffs []int, p ...mempoolext.MemPool) (result *PooledExt, numMatches int) {

	length := svecs[0].Len()

	pool, hasPool := mempoolext.ExtractCheckOptionalStrict(length, p...)

	var resvec *PooledExt

	isFirst := true
	numMatches = 0

	for i := range svecs {

		svec := svecs[i]
		// In case the current vec is Rotated, we reduce it to a regular form
		// NB : this could use the pool.
		if rot, ok := svec.(*RotatedExt); ok {
			svec = rotatedAsRegular(rot)
		}

		if pooled, ok := svec.(*smartvectors.Pooled); ok {
			svec = &pooled.Regular
		}

		if reg, ok := svec.(*RegularExt); ok {
			numMatches++
			// For the first one, we can save by just copying the result
			// Importantly, we do not need to assume that regRes is originally
			// zero.
			if isFirst {
				if hasPool {
					resvec = AllocFromPoolExt(pool)
				} else {
					resvec = &PooledExt{RegularExt: make([]fext.Element, length)}
				}

				isFirst = false
				op.vecIntoTerm(resvec.RegularExt, *reg, coeffs[i])
				continue
			}

			op.vecIntoVec(resvec.RegularExt, *reg, coeffs[i])
		}
	}

	if numMatches == 0 {
		return nil, 0
	}

	return resvec, numMatches
}

func (r *RegularExt) DeepCopy() smartvectors.SmartVector {
	return NewRegularExt(vectorext.DeepCopy(*r))
}

// Converts a smart-vector into a normal vec. The implementation minimizes
// then number of copies.
func (r *RegularExt) IntoRegVecSaveAlloc() []field.Element {
	panic(conversionError)
}

func (r *RegularExt) IntoRegVecSaveAllocBase() ([]field.Element, error) {
	return nil, fmt.Errorf(conversionError)
}

func (r *RegularExt) IntoRegVecSaveAllocExt() []fext.Element {
	temp := make([]fext.Element, r.Len())
	for i := 0; i < r.Len(); i++ {
		elem, _ := r.GetBase(i)
		temp[i].SetFromBase(&elem)
	}
	return temp
}

type PooledExt struct {
	RegularExt
	poolPtr *[]fext.Element
}

func AllocFromPoolExt(pool mempoolext.MemPool) *PooledExt {
	poolPtr := pool.Alloc()
	return &PooledExt{
		RegularExt: *poolPtr,
		poolPtr:    poolPtr,
	}
}

func (p *PooledExt) Free(pool mempoolext.MemPool) {
	if p.poolPtr != nil {
		pool.Free(p.poolPtr)
	}
	p.poolPtr = nil
	p.RegularExt = nil
}
