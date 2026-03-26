package collection

import (
	"iter"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

// A set is an unordered collection addressed by keys, which supports
type Set[T comparable] struct {
	Inner map[T]struct{}
}

// Constructor for KVStore
func NewSet[K comparable]() Set[K] {
	return Set[K]{
		Inner: make(map[K]struct{}),
	}
}

// Returns `true` if the entry exists
func (kv Set[K]) Exists(ks ...K) bool {
	for _, k := range ks {
		_, found := kv.Inner[k]
		if !found {
			return false
		}
	}
	return true
}

/*
Inserts regarless of whether the entry was already
present of not. Returns whether the entry was present
already.
*/
func (kv *Set[K]) Insert(k K) bool {
	if _, ok := kv.Inner[k]; !ok {
		kv.Inner[k] = struct{}{}
		return false
	}
	return true
}

// Iter iterates over all elements in the Set in non-deterministic order
func (kv Set[T]) Iter() iter.Seq[T] {
	return func(yield func(T) bool) {
		for k := range kv.Inner {
			if !yield(k) {
				return
			}
		}
	}
}

// Merge merge the content of the given set into the receiver.
func (kv *Set[T]) Merge(other *Set[T]) {
	for k := range other.Inner {
		kv.Inner[k] = struct{}{}
	}
}

// Clear removes all the keys from the set
func (kv *Set[T]) Clear() {
	kv.Inner = make(map[T]struct{})
}

// SortKeysBy returns sorted keys of the set using the given less function.
func (kv Set[T]) SortKeysBy(less func(T, T) bool) []T {
	return utils.SortedKeysOf(kv.Inner, less)
}

// Size returns the numbers of keys stored in the set
func (kv Set[T]) Size() int {
	return len(kv.Inner)
}
