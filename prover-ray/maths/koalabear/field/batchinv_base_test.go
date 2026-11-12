package field

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/consensys/gnark-crypto/field/koalabear"
	"github.com/stretchr/testify/require"
)

// batchInvBaseInput returns n random base elements with zeros sprinkled in.
func batchInvBaseInput(rng *rand.Rand, n int) []Element {
	a := VecPseudoRandBase(rng, n)
	for i := range a {
		if rng.IntN(7) == 0 {
			a[i] = Element{}
		}
	}
	return a
}

// requireBaseInverses checks got[i] = 1/a[i], and 0 where a[i] is 0.
func requireBaseInverses(t *testing.T, a, got []Element, msg string) {
	t.Helper()
	require.Len(t, got, len(a), msg)
	for i := range a {
		var want Element
		want.Inverse(&a[i]) // the inverse of zero is zero
		require.Equal(t, want, got[i], "%s, element %d", msg, i)
	}
}

// Every lane width must match one inversion per element, out of place and in
// place, around and across the row widths.
func TestBatchInvertScratchMatchesInverse(t *testing.T) {
	rng := rand.New(rand.NewPCG(11, 12))
	for _, n := range batchInvLengths {
		for _, lanes := range []int{-1, 0, 16, 32, 48, 64, 128} {
			a := batchInvBaseInput(rng, n)
			msg := fmt.Sprintf("n=%d lanes=%d", n, lanes)
			invert := func(res, a, scratch []Element) {
				if lanes < 0 {
					BatchInvertScratch(res, a, scratch)
				} else {
					batchInvertBaseLanes(res, a, scratch, lanes)
				}
			}

			res := make([]Element, n)
			invert(res, a, make([]Element, n+3))
			requireBaseInverses(t, a, res, msg+" out of place")

			inPlace := append([]Element(nil), a...)
			invert(inPlace, inPlace, make([]Element, n))
			requireBaseInverses(t, a, inPlace, msg+" in place")
		}
	}
}

// BatchInvertInto keeps its contract: zeros map to zero, res may alias a,
// on both sides of the stack-scratch length.
func TestBatchInvertIntoMatchesInverse(t *testing.T) {
	rng := rand.New(rand.NewPCG(13, 14))
	for _, n := range append(batchInvLengths, batchInvBaseStackLen, batchInvBaseStackLen+1) {
		a := batchInvBaseInput(rng, n)
		res := make([]Element, n)
		BatchInvertInto(res, a)
		requireBaseInverses(t, a, res, fmt.Sprintf("n=%d out of place", n))
		inPlace := append([]Element(nil), a...)
		BatchInvertInto(inPlace, inPlace)
		requireBaseInverses(t, a, inPlace, fmt.Sprintf("n=%d in place", n))
	}
}

func TestBatchInvertScratchRejectsBadLengths(t *testing.T) {
	require.Panics(t, func() { BatchInvertScratch(make([]Element, 3), make([]Element, 4), make([]Element, 4)) })
	require.Panics(t, func() { BatchInvertScratch(make([]Element, 4), make([]Element, 4), make([]Element, 3)) })
	require.Panics(t, func() { BatchInvertInto(make([]Element, 3), make([]Element, 4)) })
}

func TestBatchInvertScratchAllocatesNothing(t *testing.T) {
	a := batchInvBaseInput(rand.New(rand.NewPCG(1, 1)), 4000)
	res, scratch := make([]Element, len(a)), make([]Element, len(a))
	allocs := testing.AllocsPerRun(10, func() { BatchInvertScratch(res, a, scratch) })
	require.Zero(t, allocs)
	small := a[:batchInvBaseStackLen]
	allocs = testing.AllocsPerRun(10, func() { BatchInvertInto(res[:len(small)], small) })
	require.Zero(t, allocs, "BatchInvertInto keeps a short scratch on the stack")
}

// BenchmarkBatchInvertBaseLanes compares the lane widths, BatchInvertInto
// (stack or pooled scratch) and gnark-crypto's allocating batch inversion.
func BenchmarkBatchInvertBaseLanes(b *testing.B) {
	rng := rand.New(rand.NewPCG(5, 6))
	for _, n := range []int{16, 128, 256, 384, 512, 768, 1024, 2048, 4096, 1 << 16} {
		a := VecPseudoRandBase(rng, n)
		res, scratch := make([]Element, n), make([]Element, n)
		for _, lanes := range []int{0, 64, 128} {
			b.Run(fmt.Sprintf("n=%d/lanes=%d", n, lanes), func(b *testing.B) {
				for b.Loop() {
					batchInvertBaseLanes(res, a, scratch, lanes)
				}
			})
		}
		b.Run(fmt.Sprintf("n=%d/into", n), func(b *testing.B) {
			for b.Loop() {
				BatchInvertInto(res, a)
			}
		})
		b.Run(fmt.Sprintf("n=%d/gnark", n), func(b *testing.B) {
			for b.Loop() {
				copy(res, koalabear.BatchInvert(a))
			}
		})
	}
}
