package poseidon2

import (
	"fmt"
	"testing"

	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/stretchr/testify/require"
)

func randomElems(t *testing.T, n int) []field.Element {
	t.Helper()
	vals := make([]field.Element, n)
	for i := range vals {
		_, err := vals[i].SetRandom()
		require.NoError(t, err)
	}
	return vals
}

// TestDeterminism checks that hashing the same inputs twice yields the same digest.
func TestDeterminism(t *testing.T) {
	vals := randomElems(t, 13)

	h1 := NewMDHasher()
	h1.WriteElements(vals...)
	r1 := h1.SumElement()

	h2 := NewMDHasher()
	h2.WriteElements(vals...)
	r2 := h2.SumElement()

	require.Equal(t, r1, r2)
}

// TestIncrementalWrites checks that splitting WriteElements calls produces the same hash as a single call.
func TestIncrementalWrites(t *testing.T) {
	vals := randomElems(t, 13)

	h1 := NewMDHasher()
	h1.WriteElements(vals...)
	r1 := h1.SumElement()

	h2 := NewMDHasher()
	h2.WriteElements(vals[:5]...)
	h2.WriteElements(vals[5:9]...)
	h2.WriteElements(vals[9:]...)
	r2 := h2.SumElement()

	require.Equal(t, r1, r2)
}

// TestResetAfterSum checks that Reset() after SumElement() allows re-hashing cleanly.
func TestResetAfterSum(t *testing.T) {
	vals := randomElems(t, 7)

	h := NewMDHasher()
	h.WriteElements(vals...)
	_ = h.SumElement()
	h.Reset()
	h.WriteElements(vals...)
	r1 := h.SumElement()

	h2 := NewMDHasher()
	h2.WriteElements(vals...)
	r2 := h2.SumElement()

	require.Equal(t, r1, r2)
}

// TestResetWithoutPriorSum checks that Reset() clears the buffer even when SumElement was not called first.
// This exercises a latent bug: Reset() resets state but not buffer.
func TestResetWithoutPriorSum(t *testing.T) {
	vals := randomElems(t, 7)
	extra := randomElems(t, 3)

	h := NewMDHasher()
	h.WriteElements(vals...)
	h.Reset() // must discard vals from the buffer
	h.WriteElements(extra...)
	r1 := h.SumElement()

	h2 := NewMDHasher()
	h2.WriteElements(extra...)
	r2 := h2.SumElement()

	require.Equal(t, r1, r2, "Reset() must clear the buffer even when SumElement was not called first")
}

// TestSumElementIdempotent checks that calling SumElement again on an already-flushed hasher is a no-op.
func TestSumElementIdempotent(t *testing.T) {
	vals := randomElems(t, 9)

	h := NewMDHasher()
	h.WriteElements(vals...)
	r1 := h.SumElement()
	r2 := h.SumElement()

	require.Equal(t, r1, r2)
}

// TestGetStateOctupletNonDestructive checks that GetStateOctuplet does not alter the ongoing hash.
func TestGetStateOctupletNonDestructive(t *testing.T) {
	vals := randomElems(t, 11)

	h1 := NewMDHasher()
	h1.WriteElements(vals[:3]...)
	_ = h1.GetStateOctuplet() // must not consume the buffer
	h1.WriteElements(vals[3:]...)
	r1 := h1.SumElement()

	h2 := NewMDHasher()
	h2.WriteElements(vals...)
	r2 := h2.SumElement()

	require.Equal(t, r1, r2, "GetStateOctuplet must not consume the buffer")
}

// TestLeftPadding checks that a partial block is left-padded: the element lands at the rightmost position.
func TestLeftPadding(t *testing.T) {
	a := field.NewFromString("42")
	zero := field.Zero()

	// [a] must hash identically to [0, 0, 0, 0, 0, 0, 0, a]
	h1 := NewMDHasher()
	h1.WriteElements(a)
	r1 := h1.SumElement()

	h2 := NewMDHasher()
	h2.WriteElements(zero, zero, zero, zero, zero, zero, zero, a)
	r2 := h2.SumElement()

	require.Equal(t, r1, r2, "single element must be placed at the end of the block (left-padding)")

	// Must differ from right-padding: [a, 0, 0, 0, 0, 0, 0, 0]
	h3 := NewMDHasher()
	h3.WriteElements(a, zero, zero, zero, zero, zero, zero, zero)
	r3 := h3.SumElement()

	require.NotEqual(t, r1, r3, "left-padded hash must differ from right-padded hash")
}

// TestBlockBoundaries checks determinism at key block-size boundaries.
func TestBlockBoundaries(t *testing.T) {
	for _, n := range []int{0, 1, BlockSize - 1, BlockSize, BlockSize + 1, 2 * BlockSize, 2*BlockSize + 1} {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			vals := randomElems(t, n)

			h1 := NewMDHasher()
			h1.WriteElements(vals...)
			r1 := h1.SumElement()

			h2 := NewMDHasher()
			h2.WriteElements(vals...)
			r2 := h2.SumElement()

			require.Equal(t, r1, r2)
		})
	}
}

// TestWriteWriteElementsConsistency checks that Write(canonical bytes) and WriteElements agree.
func TestWriteWriteElementsConsistency(t *testing.T) {
	vals := randomElems(t, 10)

	buf := make([]byte, 0, len(vals)*field.Bytes)
	for _, v := range vals {
		b := v.Bytes()
		buf = append(buf, b[:]...)
	}

	h1 := NewMDHasher()
	_, err := h1.Write(buf)
	require.NoError(t, err)
	r1 := h1.SumElement()

	h2 := NewMDHasher()
	h2.WriteElements(vals...)
	r2 := h2.SumElement()

	require.Equal(t, r1, r2, "Write(canonical bytes) and WriteElements must produce the same hash")
}

// TestHashVecMatchesMDHasher checks that HashVec produces the same result as a manual MDHasher.
func TestHashVecMatchesMDHasher(t *testing.T) {
	vals := randomElems(t, 17)

	r1 := HashVec(vals...)

	h := NewMDHasher()
	h.WriteElements(vals...)
	r2 := h.SumElement()

	require.Equal(t, r1, r2)
}

// TestSetGetStateRoundtrip checks that SetStateOctuplet followed by GetStateOctuplet (with empty buffer) is identity.
func TestSetGetStateRoundtrip(t *testing.T) {
	s := field.RandomOctuplet()

	h := NewMDHasher()
	h.SetStateOctuplet(s)
	got := h.GetStateOctuplet()

	require.Equal(t, s, got)
}

// TestOrderSensitivity checks that permuting inputs produces a different hash.
func TestOrderSensitivity(t *testing.T) {
	a := field.NewFromString("1")
	b := field.NewFromString("2")

	h1 := NewMDHasher()
	h1.WriteElements(a, b)
	r1 := h1.SumElement()

	h2 := NewMDHasher()
	h2.WriteElements(b, a)
	r2 := h2.SumElement()

	require.NotEqual(t, r1, r2, "permuting inputs must change the hash")
}
