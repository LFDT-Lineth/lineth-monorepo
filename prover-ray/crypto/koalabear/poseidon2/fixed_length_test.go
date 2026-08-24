package poseidon2

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/stretchr/testify/require"
)

func fixedLengthElements(n int) []field.Element {
	values := make([]field.Element, n)
	for i := range values {
		values[i].SetUint64(uint64(i + 1))
	}
	return values
}

func fixedLengthReference(values []field.Element) field.Octuplet {
	var state field.Octuplet
	initialized := false
	for len(values) != 0 {
		n := min(len(values), BlockSize)
		var block field.Octuplet
		copy(block[BlockSize-n:], values[:n])
		if initialized {
			state = Compress(state, block)
		} else {
			state = block
			initialized = true
		}
		values = values[n:]
	}
	return state
}

func TestFixedLengthHasherBlockBoundaries(t *testing.T) {
	values := fixedLengthElements(17)
	for _, n := range []int{0, 1, 7, 8, 9, 15, 16, 17} {
		h := NewFixedLengthHasher()
		h.WriteElements(values[:n]...)
		require.Equal(t, fixedLengthReference(values[:n]), h.SumDigest(), "length=%d", n)
	}
}

func TestFixedLengthHasherSplitWrites(t *testing.T) {
	values := fixedLengthElements(17)
	split := NewFixedLengthHasher()
	split.WriteElements(values[:3]...)
	split.WriteElements(values[3:8]...)
	split.WriteElements(values[8:11]...)
	split.WriteElements(values[11:17]...)

	oneShot := NewFixedLengthHasher()
	oneShot.WriteElements(values...)
	require.Equal(t, oneShot.SumDigest(), split.SumDigest())
}

func TestFixedLengthHasherZeroFirstBlockIsInitialized(t *testing.T) {
	h := NewFixedLengthHasher()
	zeros := make([]field.Element, BlockSize)
	h.WriteElements(zeros...)
	var nine field.Element
	nine.SetUint64(9)
	h.WriteElements(nine)

	var first, last field.Octuplet
	last[BlockSize-1] = nine
	require.Equal(t, Compress(first, last), h.SumDigest())
}

func TestFixedLengthHasherReset(t *testing.T) {
	values := fixedLengthElements(17)
	h := NewFixedLengthHasher()

	h.WriteElements(values[:7]...)
	h.Reset()
	require.Equal(t, field.Octuplet{}, h.SumDigest())

	h.WriteElements(values[:16]...)
	h.Reset()
	h.WriteElements(values[:9]...)
	require.Equal(t, fixedLengthReference(values[:9]), h.SumDigest())
}
