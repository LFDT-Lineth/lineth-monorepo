package fri

import (
	"fmt"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	gutils "github.com/consensys/gnark-crypto/utils"
)

func TestBitReverseMatchesNaive(t *testing.T) {
	for logN := 1; logN <= 22; logN++ {
		n := 1 << logN
		want := make([]field.Element, n)
		got := make([]field.Element, n)
		for i := range want {
			want[i].SetUint64(uint64(i) * 2654435761)
			got[i] = want[i]
		}
		gutils.BitReverse(want)
		bitReverse(got)
		for i := range want {
			if want[i] != got[i] {
				t.Fatalf("logN=%d: mismatch at %d", logN, i)
			}
		}
	}
}

// TestBitReverseCopyMatchesInPlace covers both the naive and tiled regimes of
// the out-of-place bit-reversed copy.
func TestBitReverseCopyMatchesInPlace(t *testing.T) {
	for _, logN := range []int{0, 3, 10, 21} {
		n := 1 << logN
		src := make([]field.Element, n)
		for i := range src {
			src[i].SetUint64(uint64(i) * 40503)
		}
		want := make([]field.Element, n)
		copy(want, src)
		gutils.BitReverse(want)

		dst := make([]field.Element, n)
		bitReverseCopy(dst, src)
		for i := range want {
			if dst[i] != want[i] {
				t.Fatalf("logN=%d: mismatch at %d", logN, i)
			}
		}
	}
}

// BenchmarkBitReverse re-verifies the naive/COBRA crossover recorded in
// bitRevCobraMinLog2. The production regime is all cores bit-reversing
// concurrently; run with -cpu=N to reproduce contention.
func BenchmarkBitReverse(b *testing.B) {
	for _, logN := range []int{20, 21, 22} {
		v := make([]field.Element, 1<<logN)
		for i := range v {
			v[i].SetUint64(uint64(i))
		}
		b.Run(fmt.Sprintf("cobra/2^%d", logN), func(b *testing.B) {
			b.SetBytes(int64(len(v)) * field.Bytes)
			for b.Loop() {
				bitReverseCobraInPlace(v)
			}
		})
		b.Run(fmt.Sprintf("naive/2^%d", logN), func(b *testing.B) {
			b.SetBytes(int64(len(v)) * field.Bytes)
			for b.Loop() {
				gutils.BitReverse(v)
			}
		})
	}
}
