package field

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/consensys/gnark-crypto/field/koalabear/extensions"
	"github.com/stretchr/testify/require"
)

var batchInvLengths = []int{0, 1, 15, 16, 17, 31, 63, 64, 65, 127, 256, 257, 511, 512, 1000, 2047, 2048, 2049, 4096}

// batchInvInput returns n random extension elements with zeros sprinkled in.
func batchInvInput(rng *rand.Rand, n int) []Ext {
	a := VecPseudoRandExt(rng, n)
	for i := range a {
		if rng.IntN(7) == 0 {
			a[i] = Ext{}
		}
	}
	return a
}

// requireInverses checks got[i] = 1/a[i], and 0 where a[i] is 0.
func requireInverses(t *testing.T, a, got []Ext, msg string) {
	t.Helper()
	require.Len(t, got, len(a), msg)
	for i := range a {
		var want Ext
		want.Inverse(&a[i]) // the inverse of zero is zero
		require.Equal(t, want, got[i], "%s, element %d", msg, i)
	}
}

// Every lane width must match one inversion per element, out of place and in
// place, around and across the row widths.
func TestBatchInvertExtScratchMatchesInverse(t *testing.T) {
	rng := rand.New(rand.NewPCG(7, 8))
	for _, n := range batchInvLengths {
		for _, lanes := range []int{0, 16, 32, 48, 64, 128} {
			a := batchInvInput(rng, n)
			msg := fmt.Sprintf("n=%d lanes=%d", n, lanes)
			invert := func(res, a, scratch []Ext) {
				if lanes == 0 {
					BatchInvertExtScratch(res, a, scratch)
				} else {
					batchInvertExtLanes(res, a, scratch, lanes)
				}
			}

			res := make([]Ext, n)
			invert(res, a, make([]Ext, n+3))
			requireInverses(t, a, res, msg+" out of place")

			inPlace := append([]Ext(nil), a...)
			invert(inPlace, inPlace, make([]Ext, n))
			requireInverses(t, a, inPlace, msg+" in place")
		}
	}
}

// BatchInvertExtInto keeps its contract: zeros map to zero, res may alias a.
func TestBatchInvertExtIntoMatchesInverse(t *testing.T) {
	rng := rand.New(rand.NewPCG(9, 10))
	for _, n := range batchInvLengths {
		a := batchInvInput(rng, n)
		res := make([]Ext, n)
		BatchInvertExtInto(res, a)
		requireInverses(t, a, res, fmt.Sprintf("n=%d out of place", n))
		inPlace := append([]Ext(nil), a...)
		BatchInvertExtInto(inPlace, inPlace)
		requireInverses(t, a, inPlace, fmt.Sprintf("n=%d in place", n))
	}
}

func TestBatchInvertExtScratchAllocatesNothing(t *testing.T) {
	a := batchInvInput(rand.New(rand.NewPCG(1, 1)), 1000)
	res, scratch := make([]Ext, len(a)), make([]Ext, len(a))
	allocs := testing.AllocsPerRun(10, func() { BatchInvertExtScratch(res, a, scratch) })
	require.Zero(t, allocs)
	small := a[:batchInvStackLen]
	allocs = testing.AllocsPerRun(10, func() { BatchInvertExtInto(res[:len(small)], small) })
	require.Zero(t, allocs, "BatchInvertExtInto keeps a short scratch on the stack")
}

// BenchmarkBatchInvertExtLanes compares the lane widths, BatchInvertExtInto
// (pooled scratch) and gnark-crypto's allocating batch inversion.
func BenchmarkBatchInvertExtLanes(b *testing.B) {
	for _, n := range []int{16, 128, 256, 1024, 2048, 4096} {
		a := benchExts(n)
		res, scratch := make([]Ext, n), make([]Ext, n)
		for _, lanes := range []int{16, 32, 64, 128} {
			b.Run(fmt.Sprintf("n=%d/lanes=%d", n, lanes), func(b *testing.B) {
				for b.Loop() {
					batchInvertExtLanes(res, a, scratch, lanes)
				}
			})
		}
		b.Run(fmt.Sprintf("n=%d/into", n), func(b *testing.B) {
			for b.Loop() {
				BatchInvertExtInto(res, a)
			}
		})
		b.Run(fmt.Sprintf("n=%d/gnark", n), func(b *testing.B) {
			for b.Loop() {
				extensions.BatchInvertE6Into(res, a)
			}
		})
	}
}
