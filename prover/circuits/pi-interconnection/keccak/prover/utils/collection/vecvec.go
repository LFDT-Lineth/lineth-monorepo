package collection

import "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"

// VecVec is a wrapper around double vecs
// The inner slice is append only
type VecVec[T any] struct {
	Inner [][]T
}

/*
Constructor for double vecs. Can optionally give it an initial length
(by default set to zero)
*/
func NewVecVec[T any](length ...int) VecVec[T] {
	// Set the default length if necessary
	if len(length) == 0 {
		length = []int{0}
	}
	return VecVec[T]{Inner: make([][]T, length[0])}
}

// Extends the collections by appending empty inner slices
// Do nothing is the slice is already large enough
func (v *VecVec[T]) reserveOuter(newLen int) {
	for len((*v).Inner) < newLen {
		(*v).Inner = append((*v).Inner, make([]T, 0))
	}
}

// Returns the inner double-slice
func (v *VecVec[T]) GetInner() [][]T {
	return v.Inner
}

// Returns the inner double-slice
func (v *VecVec[T]) Len() int {
	return len(v.Inner)
}

/*
Returns a subslice. Panic if the subslice was not allocated
*/
func (v *VecVec[T]) MustGet(pos int) []T {
	/*
		This will panic if pos is larger than the slice. So no need
		to additionally check here albeit not very clean.
	*/
	return v.Inner[pos]
}

// GetOrEmpty attempts to return the required subslice or returns an
// empty slice if it goes out of bound.
func (v *VecVec[T]) GetOrEmpty(pos int) []T {
	if pos >= len(v.Inner) {
		return []T{}
	}
	return v.Inner[pos]
}

/*
Append one or more values to the given subslice. Will extend the larger size
to match the requested position if necessary
*/
func (v *VecVec[T]) AppendToInner(pos int, t ...T) {
	/*
		Sanity check : Not a mandatory one. But it's somewhat unexpected
		if that happens
	*/
	if len(t) == 0 {
		utils.Panic("Passed an empty list of values. Probably a bug")
	}

	// Make sure the subslice to append to exists or create it
	v.reserveOuter(pos + 1)

	// Then do the appending
	v.Inner[pos] = append(v.Inner[pos], t...)
}

// Allocates up to a given rounds number
func (v *VecVec[T]) Reserve(newLen int) {
	// We may not have to append the sequence
	// If we need to, we append to it as many time as we need
	for len(v.Inner) < newLen {
		v.Inner = append(v.Inner, []T{})
	}
}

// Returns the length of an inner slice, also allocate the slice
// if it was not allocated, it will reserve it.
func (v *VecVec[T]) LenOf(pos int) int {
	if v.Len() <= pos {
		v.Reserve(pos + 1)
	}
	return len(v.Inner[pos])
}
