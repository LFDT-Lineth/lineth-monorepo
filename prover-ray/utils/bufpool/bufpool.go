// Package bufpool recycles large slices from one proof to the next.
//
// A proof allocates many large short-lived vectors whose sizes repeat from
// proof to proof. Allocating them afresh costs the zeroing of every element
// and, when the memory comes back from the operating system, a page fault per
// page. A Pool keeps released slices instead, so the next proof reuses them
// as they are, at the price of keeping that memory resident between proofs.
package bufpool

import (
	"math/bits"
	"sync"
	"unsafe"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/hugepage"
)

// Pool recycles slices of T by capacity, rounded up to a power of two. Its
// zero value is ready to use and safe for concurrent use.
//
// Slices from [Pool.Get] hold stale values: a caller must write every element
// before reading it. Built with the bufpooldebug tag, the pool fills every
// slice it hands out or takes back with a non-zero pattern, so that reading
// an element before writing it, or using a slice after releasing it, gives
// wrong values deterministically instead of only when the stale values
// happen to differ. Released slices are only handed out again within their
// capacity class. The pool keeps released slices until [Pool.Trim] drops
// the classes that went unrequested, or [Pool.Drain] gives all the memory
// back, up to a cap on the bytes it retains (see [Pool.SetMaxRetainedBytes]):
// a slice released past the cap is dropped for the garbage collector.
type Pool[T any] struct {
	mu   sync.Mutex
	free [bits.UintSize][][]T
	idle [bits.UintSize]uint8 // per class, Trims since its last Get, up to trimAfter

	retained    int64 // bytes of the slices in free
	maxRetained int64 // cap on retained; 0 means DefaultMaxRetainedBytes
}

// DefaultMaxRetainedBytes is the cap on the bytes a [Pool] retains unless
// [Pool.SetMaxRetainedBytes] sets another one. It is far above what a proof
// releases into any one pool (the field pools together keep a few GiB on a
// full proof), so it does not change proving; it only bounds a pool that
// would otherwise grow without limit, e.g. under many concurrent proofs.
const DefaultMaxRetainedBytes int64 = 32 << 30

// SetMaxRetainedBytes caps the bytes of the released slices p keeps; a slice
// released while it would bring p past the cap is dropped instead. A cap of
// zero or less restores [DefaultMaxRetainedBytes]. Lowering the cap does not
// drop slices already kept; [Pool.Drain] or [Pool.Trim] do.
func (p *Pool[T]) SetMaxRetainedBytes(n int64) {
	if n < 0 {
		n = 0
	}
	p.mu.Lock()
	p.maxRetained = n
	p.mu.Unlock()
}

// RetainedBytes returns the bytes of the released slices p currently keeps.
func (p *Pool[T]) RetainedBytes() int64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.retained
}

// classBytes returns the size in bytes of a slice of the given capacity
// class.
func classBytes[T any](class int) int64 {
	var zero T
	return int64(unsafe.Sizeof(zero)) << class
}

// trimAfter is the number of Trims in a row a capacity class must go
// unrequested before [Pool.Trim] drops it. A class used every few Trims --
// by proofs of several systems run in turn, or by proofs run concurrently,
// each trimming the shared pool -- is kept.
const trimAfter = 4

// Get returns a slice of length n, from the released slices of the same
// capacity class when there is one; its elements are unspecified.
func (p *Pool[T]) Get(n int) []T {
	if n <= 0 {
		return nil
	}
	class := bits.Len(uint(n - 1))
	p.mu.Lock()
	p.idle[class] = 0
	if k := len(p.free[class]); k > 0 {
		v := p.free[class][k-1]
		p.free[class][k-1] = nil
		p.free[class] = p.free[class][:k-1]
		p.retained -= classBytes[T](class)
		p.mu.Unlock()
		if poisonEnabled {
			poison(v)
		}
		return v[:n]
	}
	p.mu.Unlock()
	v := make([]T, 1<<class)
	hugepage.Advise(v)
	if poisonEnabled {
		poison(v)
	}
	return v[:n]
}

// Put releases v for reuse. v must have been returned by [Pool.Get] and no
// longer be used. A slice whose capacity is not a power of two, or that would
// bring the pool past its retained-bytes cap, is ignored; any other one is
// kept and handed out again.
func (p *Pool[T]) Put(v []T) {
	c := cap(v)
	if c == 0 || c&(c-1) != 0 {
		return
	}
	class := bits.Len(uint(c - 1))
	size := classBytes[T](class)
	if poisonEnabled {
		poison(v[:c])
	}
	p.mu.Lock()
	limit := p.maxRetained
	if limit == 0 {
		limit = DefaultMaxRetainedBytes
	}
	if p.retained+size > limit {
		p.mu.Unlock()
		return
	}
	p.retained += size
	p.free[class] = append(p.free[class], v[:c])
	p.mu.Unlock()
}

// Trim drops the released slices of every capacity class that no [Pool.Get]
// requested during the last trimAfter Trims, so the garbage collector can
// reclaim them. Slices in use are never touched.
func (p *Pool[T]) Trim() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range p.free {
		if p.idle[i] < trimAfter {
			p.idle[i]++
		}
		if p.idle[i] >= trimAfter {
			p.retained -= int64(len(p.free[i])) * classBytes[T](i)
			p.free[i] = nil
		}
	}
}

// Drain drops every released slice, so the garbage collector can reclaim
// them.
func (p *Pool[T]) Drain() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range p.free {
		p.free[i] = nil
	}
	p.retained = 0
}
