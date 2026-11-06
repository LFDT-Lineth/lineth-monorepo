package fri

import (
	"fmt"
	"math/bits"
	"math/rand/v2"
	"testing"
	"unsafe"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
)

// naiveBitReverseCopy is the definition: dst[rev_{log n}(i)] = src[i].
func naiveBitReverseCopy[T any](dst, src []T) {
	logN := bits.TrailingZeros(uint(len(src)))
	for i := range src {
		r := 0
		if logN > 0 {
			r = int(bits.Reverse64(uint64(i)) >> (64 - logN))
		}
		dst[r] = src[i]
	}
}

// checkBitReverseCopy compares bitReverseCopy against the definition for
// every size up to 2^maxLog and several worker counts: serial, GOMAXPROCS,
// odd counts, and more workers than there are tiles (or elements). dst is
// pre-filled with garbage, so a position the copy fails to write is caught.
func checkBitReverseCopy[T comparable](t *testing.T, maxLog int, rnd func(*rand.Rand, int) []T) {
	t.Helper()
	rng := rand.New(rand.NewChaCha8([32]byte{1}))
	for logN := 0; logN <= maxLog; logN++ {
		n := 1 << logN
		src := rnd(rng, n)
		want := make([]T, n)
		naiveBitReverseCopy(want, src)
		garbage := rnd(rng, n)
		got := make([]T, n)
		for _, workers := range []int{-1, 0, 1, 2, 3, 7, 64, 1 << 20} {
			copy(got, garbage)
			bitReverseCopy(got, src, workers)
			for i := range want {
				if got[i] != want[i] {
					t.Fatalf("log n=%d workers=%d: mismatch at %d", logN, workers, i)
				}
			}
		}
	}
}

func TestBitReverseCopyBase(t *testing.T) {
	checkBitReverseCopy(t, 22, field.VecPseudoRandBase)
}

func TestBitReverseCopyExt(t *testing.T) {
	checkBitReverseCopy(t, 20, field.VecPseudoRandExt)
}

func TestBitReverseLogTile(t *testing.T) {
	var (
		base field.Element
		ext  field.Ext
	)
	if q := bitReverseLogTile(unsafe.Sizeof(base)); q != 5 {
		t.Fatalf("base tile: got 2^%d, want 2^5", q)
	}
	if q := bitReverseLogTile(unsafe.Sizeof(ext)); q != 3 {
		t.Fatalf("ext tile: got 2^%d, want 2^3", q)
	}
}

func TestBitReverseCopyRejectsBadLengths(t *testing.T) {
	for _, c := range []struct{ dst, src int }{{3, 3}, {4, 8}, {0, 0}} {
		t.Run(fmt.Sprintf("%d-%d", c.dst, c.src), func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("expected a panic")
				}
			}()
			bitReverseCopy(make([]field.Element, c.dst), make([]field.Element, c.src), 1)
		})
	}
}

// BenchmarkBitReverseCopy times the copy as the RS encoder runs it, on all the
// CPUs (workers=0) and serially (workers=1, the share of a small encode).
func BenchmarkBitReverseCopy(b *testing.B) {
	rng := rand.New(rand.NewChaCha8([32]byte{2}))
	for _, logN := range []int{12, 16, 20, 22} {
		src := field.VecPseudoRandBase(rng, 1<<logN)
		dst := make([]field.Element, 1<<logN)
		for _, workers := range []int{0, 1} {
			b.Run(fmt.Sprintf("base/2^%d/workers=%d", logN, workers), func(b *testing.B) {
				b.SetBytes(int64(len(src)) * 4)
				for b.Loop() {
					bitReverseCopy(dst, src, workers)
				}
			})
		}
	}
	for _, logN := range []int{12, 16, 20} {
		src := field.VecPseudoRandExt(rng, 1<<logN)
		dst := make([]field.Ext, 1<<logN)
		for _, workers := range []int{0, 1} {
			b.Run(fmt.Sprintf("ext/2^%d/workers=%d", logN, workers), func(b *testing.B) {
				b.SetBytes(int64(len(src)) * 24)
				for b.Loop() {
					bitReverseCopy(dst, src, workers)
				}
			})
		}
	}
}
