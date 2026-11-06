package field

import "testing"

// DrainPools must drop the released buffers of every field pool, so the next
// Get of each allocates afresh.
func TestDrainPoolsDropsEveryPool(t *testing.T) {
	const n = 1 << 10
	b, e, o := BasePool.Get(n), ExtPool.Get(n), OctupletPool.Get(n)
	BasePool.Put(b)
	ExtPool.Put(e)
	OctupletPool.Put(o)
	DrainPools()
	if &BasePool.Get(n)[0] == &b[0] {
		t.Fatal("DrainPools kept a BasePool buffer")
	}
	if &ExtPool.Get(n)[0] == &e[0] {
		t.Fatal("DrainPools kept an ExtPool buffer")
	}
	if &OctupletPool.Get(n)[0] == &o[0] {
		t.Fatal("DrainPools kept an OctupletPool buffer")
	}
}
