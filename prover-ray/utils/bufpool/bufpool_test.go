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

// Trim must drop the released slices of the classes no Get requested since
// the previous Trim, and keep those of the classes requested.
func TestPoolTrimDropsUnrequestedClasses(t *testing.T) {
	var p Pool[uint32]
	small, large := p.Get(4), p.Get(1024)
	p.Put(small)
	p.Put(large)
	p.Trim() // both classes were requested: both kept
	if s := p.Get(4); &s[0] != &small[0] {
		t.Fatal("Trim must keep a class requested since the previous Trim")
	}
	p.Trim() // only the small class was requested since: the large one goes
	if l := p.Get(1024); &l[0] == &large[0] {
		t.Fatal("Trim must drop a class not requested since the previous Trim")
	}
}
