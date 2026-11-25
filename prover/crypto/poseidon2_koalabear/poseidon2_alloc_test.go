package poseidon2_koalabear

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/field"
)

// TestResetClearsBuffer checks that Reset discards elements written before it.
func TestResetClearsBuffer(t *testing.T) {
	a := make([]field.Element, 5)
	for i := range a {
		a[i].SetUint64(uint64(i + 1))
	}
	b := make([]field.Element, 3)
	for i := range b {
		b[i].SetUint64(uint64(100 + i))
	}

	h := NewMDHasher()
	h.WriteElements(a...)
	h.Reset()
	h.WriteElements(b...)
	if got, want := h.SumElement(), HashVec(b...); got != want {
		t.Fatalf("Reset did not clear the buffer: got %v, want %v", got, want)
	}
}

// TestGetStateOctupletIsPure checks that peeking at the state does not alter
// the hasher.
func TestGetStateOctupletIsPure(t *testing.T) {
	v := make([]field.Element, 13)
	for i := range v {
		v[i].SetUint64(uint64(i))
	}
	h := NewMDHasher()
	h.WriteElements(v...)
	peek := h.GetStateOctuplet()
	if got := h.SumElement(); got != peek {
		t.Fatalf("GetStateOctuplet disagrees with SumElement: %v vs %v", peek, got)
	}
}

// TestNoAllocs guards the hot paths against heap allocation regressions.
func TestNoAllocs(t *testing.T) {
	v := make([]field.Element, 16)
	p := make([]byte, 64)
	h := NewMDHasher()
	h.WriteElements(v...) // warm up the buffer capacity
	h.SumElement()

	cases := map[string]func(){
		"HashVec": func() { _ = HashVec(v...) },
		"ReuseWriteElements": func() {
			h.Reset()
			h.WriteElements(v...)
			_ = h.SumElement()
		},
		"ReuseWriteBytes": func() {
			h.Reset()
			_, _ = h.Write(p)
			_ = h.SumElement()
		},
		"GetStateOctuplet": func() {
			h.Reset()
			h.WriteElements(v[:12]...)
			_ = h.GetStateOctuplet()
		},
	}
	for name, f := range cases {
		if n := testing.AllocsPerRun(100, f); n != 0 {
			t.Errorf("%s: %v allocs/op, want 0", name, n)
		}
	}
}
