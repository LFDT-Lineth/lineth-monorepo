package bufpool

import "testing"

// Get must hand out slices of the requested length, reuse released slices of
// the same capacity class, and ignore slices it did not allocate.
func TestPoolReusesByCapacityClass(t *testing.T) {
	var p Pool[uint32]
	if p.Get(0) != nil {
		t.Fatal("Get(0) must be nil")
	}
	a := p.Get(100)
	if len(a) != 100 || cap(a) != 128 {
		t.Fatalf("Get(100): len %d cap %d", len(a), cap(a))
	}
	a[99] = 7
	p.Put(a)
	b := p.Get(65)
	if len(b) != 65 || &b[0] != &a[0] {
		t.Fatal("Get must reuse a released slice of the same class")
	}
	if c := p.Get(100); &c[0] == &a[0] {
		t.Fatal("a slice must not be handed out twice")
	}
	p.Put(make([]uint32, 3)) // capacity not a power of two: ignored
	if d := p.Get(3); cap(d) != 4 {
		t.Fatalf("Get(3): cap %d", cap(d))
	}
	p.Put(b)
	p.Drain()
	if e := p.Get(65); &e[0] == &b[0] {
		t.Fatal("Drain must drop released slices")
	}
}

// Trim must keep the classes requested during the last trimAfter Trims, and
// drop a class only once it went unrequested for trimAfter Trims in a row.
func TestPoolTrimDropsUnrequestedClasses(t *testing.T) {
	var p Pool[uint32]
	small, large := p.Get(4), p.Get(1024)
	p.Put(small)
	p.Put(large)
	for i := range trimAfter - 1 {
		p.Trim()
		s := p.Get(4) // the small class stays in use
		if &s[0] != &small[0] {
			t.Fatalf("Trim %d dropped a class requested since the previous Trim", i)
		}
		p.Put(s)
	}
	// The large class was last requested trimAfter-1 Trims ago: still kept.
	if l := p.Get(1024); &l[0] != &large[0] {
		t.Fatal("Trim dropped a class requested within the last trimAfter Trims")
	} else {
		p.Put(l)
	}
	for range trimAfter {
		p.Trim()
	}
	if l := p.Get(1024); &l[0] == &large[0] {
		t.Fatal("Trim must drop a class unrequested for trimAfter Trims")
	}
}

// The pool must count the bytes it retains, drop a released slice that would
// bring it past its cap, and count down on Get, Trim and Drain.
func TestPoolCapsRetainedBytes(t *testing.T) {
	const slice = 64 * 4 // bytes of a 64-element uint32 slice
	var p Pool[uint32]
	p.SetMaxRetainedBytes(3 * slice)
	a, b, c, d := p.Get(64), p.Get(64), p.Get(64), p.Get(64)
	for _, v := range [][]uint32{a, b, c, d} {
		p.Put(v)
	}
	if got := p.RetainedBytes(); got != 3*slice {
		t.Fatalf("retained %d bytes, want %d", got, 3*slice)
	}
	// d went past the cap: it was dropped, so the pool hands out c, b, a.
	for _, want := range [][]uint32{c, b, a} {
		if v := p.Get(64); &v[0] != &want[0] {
			t.Fatal("Get must hand out the retained slices")
		}
	}
	if got := p.RetainedBytes(); got != 0 {
		t.Fatalf("retained %d bytes after emptying the pool", got)
	}
	if v := p.Get(64); &v[0] == &d[0] {
		t.Fatal("a slice released past the cap must not be kept")
	}

	p.Put(a)
	p.Put(p.Get(1024)) // over the cap on its own: dropped
	if got := p.RetainedBytes(); got != slice {
		t.Fatalf("retained %d bytes, want %d", got, slice)
	}
	for range trimAfter {
		p.Trim()
	}
	if got := p.RetainedBytes(); got != 0 {
		t.Fatalf("retained %d bytes after Trim dropped every class", got)
	}

	p.SetMaxRetainedBytes(0) // back to the default
	p.Put(a)
	p.Put(make([]uint32, 1024))
	if got := p.RetainedBytes(); got != slice+1024*4 {
		t.Fatalf("retained %d bytes, want %d", got, slice+1024*4)
	}
	p.Drain()
	if got := p.RetainedBytes(); got != 0 {
		t.Fatalf("retained %d bytes after Drain", got)
	}
}
