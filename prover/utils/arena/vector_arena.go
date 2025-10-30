package arena

import (
	"sync/atomic"
	"unsafe"
)

// VectorArena is a non-generic, fixed-size allocator.
// It manages a raw block of memory
// It has no knowledge of the types that will be stored in it.
type VectorArena struct {
	data   []byte
	offset int64
}

// NewVectorArena creates a memory arena that can hold capacity elements of any type.
// The arena allocates a single contiguous block of memory upfront.
// If the arena is exhausted, allocations fall back to the heap.
func NewVectorArena[T any](capacity int) *VectorArena {
	var zero T
	totalBytes := int64(unsafe.Sizeof(zero)) * int64(capacity)
	return &VectorArena{
		data:   make([]byte, totalBytes),
		offset: 0,
	}
}

// get is an unexported method that returns the next raw byte slice from the arena.
// It returns nil if the arena is exhausted.
func (a *VectorArena) get(nbBytes int64) []byte {
	n := atomic.AddInt64(&a.offset, nbBytes)
	start := n - nbBytes
	end := n
	if end > int64(len(a.data)) {
		return nil
	}
	return a.data[start:end]
}

// Reset makes the arena available for reuse.
// This should only be called when previously allocated vectors are no longer in use.
// Offset should be 0 to reuse the entire arena, or set to a specific value (returned by Offset())
// There is no safety check, use at your own risk.
// Note that this is not safe for concurrent use with calls to Get.
func (a *VectorArena) Reset(offset int64) {
	atomic.StoreInt64(&a.offset, offset)
}

func (a *VectorArena) Offset() int64 {
	return atomic.LoadInt64(&a.offset)
}

// Remaining returns the number of elements of type T that can still be allocated
// from the arena before it is exhausted.
func Remaining[T any](a *VectorArena) int {
	var zero T
	totalBytes := int64(unsafe.Sizeof(zero)) * int64(1)
	used := atomic.LoadInt64(&a.offset)
	return int((int64(len(a.data)) - used) / totalBytes)
}

// Get is a generic function that retrieves a typed vector from the arena.
// It ensures that the requested type and length match the arena's chunk size.
func Get[T any](a *VectorArena, vectorLen int) []T {
	if vectorLen == 0 {
		return make([]T, 0)
	}
	var zero T

	// Runtime safety check: ensure the requested slice fits the arena's chunk size.
	requiredBytes := int64(unsafe.Sizeof(zero)) * int64(vectorLen)

	// Get the raw memory chunk.
	chunk := a.get(requiredBytes)
	if chunk == nil {
		// Arena is full, fall back to heap allocation.
		return make([]T, vectorLen)
	}

	// Create a typed slice header pointing to the raw memory.
	return unsafe.Slice((*T)(unsafe.Pointer(&chunk[0])), vectorLen)
}
