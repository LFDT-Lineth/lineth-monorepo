package hugepage

import "testing"

// Advise is a hint: it must accept empty, small, unaligned and large buffers
// of any element type, and leave their contents untouched.
func TestAdviseLeavesBuffersIntact(t *testing.T) {
	Advise([]uint32(nil))
	small := []uint64{1, 2, 3}
	Advise(small)
	large := make([]uint32, 3*hugePageSize/4+17)
	for i := range large {
		large[i] = uint32(i)
	}
	Advise(large[1:])
	for i := range large {
		if large[i] != uint32(i) {
			t.Fatalf("Advise changed element %d", i)
		}
	}
	if small[2] != 3 {
		t.Fatal("Advise changed a small buffer")
	}
}
