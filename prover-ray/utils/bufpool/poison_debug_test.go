//go:build bufpooldebug

package bufpool

import "testing"

// Built with bufpooldebug, every slice the pool hands out, fresh or reused,
// and every slice it takes back must hold the poison pattern; slices of a
// type holding pointers must be left alone.
func TestPoolPoisonsSlices(t *testing.T) {
	const want = 0xa5a5a5a5
	var p Pool[uint32]
	check := func(what string, v []uint32) {
		t.Helper()
		for i, x := range v[:cap(v)] {
			if x != want {
				t.Fatalf("%s: element %d is %#x, want %#x", what, i, x, uint32(want))
			}
		}
	}
	a := p.Get(100)
	check("fresh slice", a)
	for i := range a {
		a[i] = uint32(i)
	}
	p.Put(a)
	check("released slice", a) // a is retained by the test only to observe the poison
	b := p.Get(100)
	check("reused slice", b)

	var q Pool[*int]
	ptrs := q.Get(4)
	for i := range ptrs {
		if ptrs[i] != nil {
			t.Fatal("a slice of pointers must not be poisoned")
		}
	}
}
