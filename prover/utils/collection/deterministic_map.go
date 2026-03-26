package collection

import (
	"iter"

	"github.com/consensys/linea-monorepo/prover/utils"
)

// DeterministicMap wraps a map and allows iterating in a deterministic order
type DeterministicMap[K comparable, V any] struct {
	InnerMap map[K]int
	Values   []V
	Keys     []K
}

// MakeDeterministicMap returns a DeterministicMap with the requested capacity
func MakeDeterministicMap[K comparable, V any](cap int) *DeterministicMap[K, V] {
	return &DeterministicMap[K, V]{
		InnerMap: make(map[K]int, cap),
		Values:   make([]V, 0, cap),
		Keys:     make([]K, 0, cap),
	}
}

// InsertNew inserts a new value and panics if it was contained already
func (kv *DeterministicMap[K, V]) InsertNew(key K, value V) {
	if _, found := kv.InnerMap[key]; found {
		utils.Panic("Entry %v already found", key)
	}
	kv.InnerMap[key] = len(kv.Values)
	kv.Values = append(kv.Values, value)
	kv.Keys = append(kv.Keys, key)
}

// Set a new value
func (kv *DeterministicMap[K, V]) Set(key K, value V) {

	if _, found := kv.InnerMap[key]; !found {
		kv.InsertNew(key, value)
		return
	}

	idx := kv.InnerMap[key]
	kv.Values[idx] = value
}

// GetPtr returns a pointer to the value
func (kv *DeterministicMap[K, V]) GetPtr(key K) (*V, bool) {
	idx, ok := kv.InnerMap[key]
	if !ok {
		return nil, false
	}
	if len(kv.Values) <= idx {
		utils.Panic("%++v", kv.InnerMap)
	}
	return &kv.Values[idx], true
}

// Get returns the value
func (kv *DeterministicMap[K, V]) Get(key K) (V, bool) {
	idx, ok := kv.InnerMap[key]
	if !ok {
		var v V
		return v, false
	}
	return kv.Values[idx], true
}

// MustGet returns the value or panics
func (kv *DeterministicMap[K, V]) MustGet(key K) V {
	idx, ok := kv.InnerMap[key]
	if !ok {
		utils.Panic("Entry %v not found", key)
	}
	return kv.Values[idx]
}

// HasKey returns whether the key is included in the map
func (kv *DeterministicMap[K, V]) HasKey(key K) bool {
	_, ok := kv.InnerMap[key]
	return ok
}

// Iter returns an iterator over the map's key
func (kv *DeterministicMap[K, V]) IterKey() iter.Seq[K] {
	return func(yield func(K) bool) {
		for k := range kv.Keys {
			if !yield(kv.Keys[k]) {
				return
			}
		}
	}
}

// IterValues returns an iterator over the map's values
func (kv *DeterministicMap[K, V]) IterValues() iter.Seq[V] {
	return func(yield func(V) bool) {
		for k := range kv.Values {
			if !yield(kv.Values[k]) {
				return
			}
		}
	}
}

// ValueSlice returns the slice of values stored in the map
func (kv *DeterministicMap[K, V]) ValueSlice() []V {
	return kv.Values
}

// Len returns the number of values stored in the map
func (kv *DeterministicMap[K, V]) Len() int {
	return len(kv.Values)
}
