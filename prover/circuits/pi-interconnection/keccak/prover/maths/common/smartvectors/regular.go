package smartvectors

import (
	"fmt"
	"iter"
	"slices"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field/fext"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/mempool"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

// It's normal vector in a nutshell
type Regular []field.Element

// Instanstiate a new regular from a slice. Returns a pointer so that the result
// can be reused without referencing as a SmartVector.
func NewRegular(v []field.Element) *Regular {
	assertStrictPositiveLen(len(v))
	res := Regular(v)
	return &res
}

// Returns the length of the regular vector
func (r *Regular) Len() int { return len(*r) }

// Returns a particular element of the vector
func (r *Regular) GetBase(n int) (field.Element, error) { return (*r)[n], nil }

func (r *Regular) GetExt(n int) fext.Element {
	return *new(fext.Element).SetFromBase(&(*r)[n])
}

func (r *Regular) Get(n int) field.Element {
	res, err := r.GetBase(n)
	if err != nil {
		panic(err)
	}
	return res
}

// Returns a subvector of the regular
func (r *Regular) SubVector(start, stop int) SmartVector {
	if start > stop {
		utils.Panic("Negative length are not allowed")
	}
	if start == stop {
		utils.Panic("Subvector of zero lengths are not allowed")
	}
	res := Regular((*r)[start:stop])
	return &res
}

// Rotates the vector into a new one
func (r *Regular) RotateRight(offset int) SmartVector {
	resSlice := make(Regular, r.Len())

	if offset == 0 {
		copy(resSlice, *r)
		return &resSlice
	}

	if offset < 0 || offset > len(*r) {
		offset = utils.PositiveMod(offset, len(*r))
	}

	// v and w may be the same vector thus we should use a
	// separate leftover buffer for temporary memory buffers.
	cutAt := len(*r) - offset
	leftovers := vector.DeepCopy((*r)[cutAt:])
	copy(resSlice[offset:], (*r)[:cutAt])
	copy(resSlice[:offset], leftovers)
	return &resSlice
}

func (r *Regular) WriteInSlice(s []field.Element) {
	assertHasLength(len(s), len(*r))
	copy(s, *r)
}

func (r *Regular) WriteInSliceExt(s []fext.Element) {
	assertHasLength(len(s), len(*r))
	for i := 0; i < len(s); i++ {
		elem, _ := r.GetBase(i)
		s[i].SetFromBase(&elem)
	}
}

func (r *Regular) Pretty() string {
	return fmt.Sprintf("Regular[%v]", vector.Prettify(*r))
}

// IterateCompact returns an iterator over the elements of the Regular.
func (r *Regular) IterateCompact() iter.Seq[field.Element] {
	return slices.Values(*r)
}

// IterateSkipPadding returns an interator over all the elements of the
// smart-vector.
func (r *Regular) IterateSkipPadding() iter.Seq[field.Element] {
	return r.IterateCompact()
}

func processRegularOnly(op operator, svecs []SmartVector, coeffs []int, p ...mempool.MemPool) (result *Pooled, numMatches int) {

	length := svecs[0].Len()

	pool, hasPool := mempool.ExtractCheckOptionalStrict(length, p...)

	var resvec *Pooled

	isFirst := true
	numMatches = 0

	for i := range svecs {

		svec := svecs[i]
		// In case the current vec is Rotated, we reduce it to a regular form
		// NB : this could use the pool.
		if rot, ok := svec.(*Rotated); ok {
			svec = rotatedAsRegular(rot)
		}

		if pooled, ok := svec.(*Pooled); ok {
			svec = &pooled.Regular
		}

		if reg, ok := svec.(*Regular); ok {
			numMatches++
			// For the first one, we can save by just copying the result
			// Importantly, we do not need to assume that regRes is originally
			// zero.
			if isFirst {
				if hasPool {
					resvec = AllocFromPool(pool)
				} else {
					resvec = &Pooled{Regular: make([]field.Element, length)}
				}

				isFirst = false
				op.vecIntoTerm(resvec.Regular, *reg, coeffs[i])
				continue
			}

			op.vecIntoVec(resvec.Regular, *reg, coeffs[i])
		}
	}

	if numMatches == 0 {
		return nil, 0
	}

	return resvec, numMatches
}

func (r *Regular) DeepCopy() SmartVector {
	return NewRegular(vector.DeepCopy(*r))
}

// Converts a smart-vector into a normal vec. The implementation minimizes
// then number of copies.
func (r *Regular) IntoRegVecSaveAlloc() []field.Element {
	res, err := r.IntoRegVecSaveAllocBase()
	if err != nil {
		panic(conversionError)
	}
	return res
}

func (r *Regular) IntoRegVecSaveAllocBase() ([]field.Element, error) {
	return (*r)[:], nil
}

func (r *Regular) IntoRegVecSaveAllocExt() []fext.Element {
	temp := make([]fext.Element, r.Len())
	for i := 0; i < r.Len(); i++ {
		elem, _ := r.GetBase(i)
		temp[i].SetFromBase(&elem)
	}
	return temp
}

func (r *Regular) GetPtr(n int) *field.Element {
	return &(*r)[n]
}

type Pooled struct {
	Regular
	poolPtr *[]field.Element `serde:"omit"`
}

func AllocFromPool(pool mempool.MemPool) *Pooled {
	poolPtr := pool.Alloc()
	return &Pooled{
		Regular: *poolPtr,
		poolPtr: poolPtr,
	}
}

func (p *Pooled) Free(pool mempool.MemPool) {
	if p.poolPtr != nil {
		if err := pool.Free(p.poolPtr); err != nil {
			utils.Panic("failed to free slice in pool: %v", err)
		}
	}
	p.poolPtr = nil
	p.Regular = nil
}
