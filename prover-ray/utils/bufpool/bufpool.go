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
// before reading it. Released slices are only handed out again within their
// capacity class. The pool keeps every released slice until [Pool.Trim] drops
// the classes that went unrequested, or [Pool.Drain] gives all the memory
// back.
type Pool[T any] struct {
	mu   sync.Mutex
	free [bits.UintSize][][]T
	idle [bits.UintSize]uint8 // per class, Trims since its last Get, up to trimAfter
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
		p.mu.Unlock()
		return v[:n]
	}
	p.mu.Unlock()
	v := make([]T, 1<<class)
	hugepage.Advise(v)
	return v[:n]
}

// Put releases v for reuse. v must have been returned by [Pool.Get] and no
// longer be used. A slice whose capacity is not a power of two is ignored;
// any other one is kept and handed out again.
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
}
