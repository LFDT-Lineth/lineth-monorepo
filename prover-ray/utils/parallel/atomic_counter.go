package parallel

import "sync/atomic"

// AtomicCounter is a thread-safe counter that can be
// incremented atomically and is bounded by a maximum value.
type AtomicCounter struct {
	n       int
	current atomic.Uint32
}

// NewAtomicCounter creates a new AtomicCounter with an
// initial value of n bound.
func NewAtomicCounter(n int) *AtomicCounter {
	return &AtomicCounter{
		n:       n,
		current: atomic.Uint32{},
	}
}

// Next returns the next value of the counter and a
// boolean indicating if the counter is still within bounds.
func (c *AtomicCounter) Next() (int, bool) {
	current := int(c.current.Add(1)) - 1
	if current >= c.n {
		return 0, false
	}

	return current, true
}
