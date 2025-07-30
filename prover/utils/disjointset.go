package utils

import "iter"

// DisjointSet represents a union-find data structure, which efficiently groups elements (columns)
// into disjoint sets (modules). It supports fast union and find operations with path compression.
type DisjointSet[T comparable] struct {
	Parent map[T]T   // Maps a column to its representative parent.
	Rank   map[T]int // Stores the rank (tree depth) for optimization.
}

// NewDisjointSet initializes a new DisjointSet with empty mappings.
func NewDisjointSet[T comparable]() *DisjointSet[T] {
	return &DisjointSet[T]{
		Parent: make(map[T]T),
		Rank:   make(map[T]int),
	}
}

// NewDisjointSetFromList initializes a new DisjointSet with the given elements.
func NewDisjointSetFromList[T comparable](elements []T) *DisjointSet[T] {
	ds := NewDisjointSet[T]()
	for _, element := range elements {
		ds.Parent[element] = element
		ds.Rank[element] = 0
	}
	return ds
}

// AddList adds a list of elements to the DisjointSet.
func (ds *DisjointSet[T]) AddList(elements []T) {
	for _, element := range elements {
		ds.Parent[element] = element
		ds.Rank[element] = 0
		ds.Union(ds.Find(elements[0]), element)
	}
}

// Reset clears the DisjointSet, removing all elements.
func (ds *DisjointSet[T]) Reset() {
	ds.Parent = make(map[T]T)
	ds.Rank = make(map[T]int)
}

// Find returns the representative (root) of a column using path compression for optimization.
// Path compression ensures that the structure remains nearly flat, reducing the time complexity to O(α(n)),
// where α(n) is the inverse Ackermann function, which is nearly constant in practice.
//
// Example:
// Suppose we have the following sets:
//
//	A -> B -> C (C is the root)
//	D -> E -> F (F is the root)
//
// Calling Find(A) will compress the path so that:
//
//	A -> C
//	B -> C
//	C remains the root
//
// Similarly, calling Find(D) will compress the path so that:
//
//	D -> F
//	E -> F
//	F remains the root
func (ds *DisjointSet[T]) Find(col T) T {
	if _, exists := ds.Parent[col]; !exists {
		ds.Parent[col] = col
		ds.Rank[col] = 0
	}
	if ds.Parent[col] != col {
		ds.Parent[col] = ds.Find(ds.Parent[col])
	}
	return ds.Parent[col]
}

// Union merges two sets by linking the root of one to the root of another, optimizing with rank.
// The smaller tree is always attached to the larger tree to keep the depth minimal.
//
// Time Complexity: O(α(n)) (nearly constant due to path compression and union by rank).
//
// Example:
// Suppose we have:
//
//	Set 1: A -> B (B is the root)
//	Set 2: C -> D (D is the root)
//
// Calling Union(A, C) will merge the sets:
//
//	If B has a higher rank than D:
//	    D -> B
//	    C -> D -> B
//	If D has a higher rank than B:
//	    B -> D
//	    A -> B -> D
//	If B and D have equal rank:
//	    D -> B (or B -> D)
//	    Rank of the new root increases by 1
func (ds *DisjointSet[T]) Union(col1, col2 T) {
	root1 := ds.Find(col1)
	root2 := ds.Find(col2)

	if root1 != root2 {
		if ds.Rank[root1] > ds.Rank[root2] {
			ds.Parent[root2] = root1
		} else if ds.Rank[root1] < ds.Rank[root2] {
			ds.Parent[root1] = root2
		} else {
			ds.Parent[root2] = root1
			ds.Rank[root1]++
		}
	}
}

// Has returns a boolean indicating if the provided value exists in the
// DisjointSet.
func (ds *DisjointSet[T]) Has(col T) bool {
	_, exists := ds.Parent[col]
	return exists
}

// Size returns the number of elements in the DisjointSet.
func (ds *DisjointSet[T]) Size() int {
	return len(ds.Parent)
}

// Iter iterates over all elements in the DisjointSet in non-deterministic order
func (ds *DisjointSet[T]) Iter() iter.Seq[T] {
	return func(yield func(T) bool) {
		for k := range ds.Parent {
			if !yield(k) {
				return
			}
		}
	}
}
