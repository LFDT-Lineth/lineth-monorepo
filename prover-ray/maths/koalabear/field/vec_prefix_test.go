package field

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"
)

// The chunked prefix scans must equal a serial running sum and product, for
// lengths within one chunk, at chunk boundaries and over many chunks whose
// lengths are not multiples of the vector kernels' lanes, and for any number
// of workers.
func TestPrefixScanExtMatchesSerial(t *testing.T) {
	rng := rand.New(rand.NewPCG(51, 52))
	lengths := []int{0, 1, 17, prefixScanMinChunk - 1, prefixScanMinChunk + 1, 3*prefixScanMinChunk + 17, 20005}
	for _, n := range lengths {
		z := VecPseudoRandExt(rng, n) // no zeros: one would zero every later product
		wantSum, wantProd := slices.Clone(z), slices.Clone(z)
		for i := 1; i < n; i++ {
			wantSum[i].Add(&wantSum[i-1], &wantSum[i])
			wantProd[i].Mul(&wantProd[i-1], &wantProd[i])
		}
		for _, workers := range []int{1, 3, 8, 48} {
			t.Run(fmt.Sprintf("n=%d/workers=%d", n, workers), func(t *testing.T) {
				sum, prod := slices.Clone(z), slices.Clone(z)
				VecPrefixSumExt(sum, workers)
				VecPrefixProductExt(prod, workers)
				if !slices.Equal(wantSum, sum) {
					t.Fatal("prefix sum differs from the serial running sum")
				}
				if !slices.Equal(wantProd, prod) {
					t.Fatal("prefix product differs from the serial running product")
				}
			})
		}
	}
}

func BenchmarkPrefixScanExt(b *testing.B) {
	rng := rand.New(rand.NewPCG(1, 2))
	const n = 1 << 20
	z := VecPseudoRandExt(rng, n)
	work := make([]Ext, n)
	for _, workers := range []int{1, 48} {
		b.Run(fmt.Sprintf("product/workers=%d", workers), func(b *testing.B) {
			for b.Loop() {
				copy(work, z)
				VecPrefixProductExt(work, workers)
			}
		})
		b.Run(fmt.Sprintf("sum/workers=%d", workers), func(b *testing.B) {
			for b.Loop() {
				copy(work, z)
				VecPrefixSumExt(work, workers)
			}
		})
	}
}
