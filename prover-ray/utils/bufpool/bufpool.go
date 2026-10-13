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

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/hugepage"
)

// Pool recycles slices of T by capacity, rounded up to a power of two. Its
// zero value is ready to use and safe for concurrent use.
//
// Slices from [Pool.Get] hold stale values: a caller must write every element
// before reading it. The pool keeps every released slice; its footprint is
// bounded by the largest set of slices in use at once, since released slices
// are handed out again before new ones are allocated. [Pool.Drain] gives the
// memory back.
type Pool[T any] struct {
	mu   sync.Mutex
	free [bits.UintSize][][]T
}

// Get returns a slice of length n, from the released slices of the same
// capacity class when there is one; its elements are unspecified.
func (p *Pool[T]) Get(n int) []T {
	if n <= 0 {
		return nil
	}
	class := bits.Len(uint(n - 1))
	p.mu.Lock()
	if k := len(p.free[class]); k > 0 {
		v := p.free[class][k-1]
		p.free[class][k-1] = nil
		p.free[class] = p.free[class][:k-1]
		p.mu.Unlock()
		return v[:n]
	}
	p.mu.Unlock()
	v := make([]T, 1<<class)
	hugepage.Advise(v)
	return v[:n]
}

// Put releases v for reuse. v must have been returned by [Pool.Get] and no
// longer be used; other slices are ignored.
func (p *Pool[T]) Put(v []T) {
	c := cap(v)
	if c == 0 || c&(c-1) != 0 {
		return
	}
	class := bits.Len(uint(c - 1))
	p.mu.Lock()
	p.free[class] = append(p.free[class], v[:c])
	p.mu.Unlock()
}

// Drain drops every released slice, so the garbage collector can reclaim
// them.
func (p *Pool[T]) Drain() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range p.free {
		p.free[i] = nil
	}
}
