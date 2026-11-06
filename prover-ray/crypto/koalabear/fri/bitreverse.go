package fri

import (
	"math/bits"
	"runtime"
	"unsafe"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/parallel"
	gutils "github.com/consensys/gnark-crypto/utils"
)

func bitReverse[T any](v []T) {
	gutils.BitReverse(v)
}

const (
	// bitReverseTileBytes bounds one row of a bit-reverse tile: the tile side
	// B is the largest power of two (at most 2^bitReverseMaxLogTile) with
	// B·sizeof(T) at most this, i.e. B=32 for base elements and B=8 for E6.
	// A row then spans two to three cache lines, and the B×B tile buffer
	// (4 KiB for base, 1.5 KiB for E6) stays in L1.
	bitReverseTileBytes  = 256
	bitReverseMaxLogTile = 5
	// bitReverseMinTiledBytes is the size below which [bitReverseCopy] uses
	// the plain scatter.
	bitReverseMinTiledBytes = 16 << 10
	// bitReverseBytesPerTask is the least data a goroutine is given: below
	// it, spawning and joining the goroutine costs more than the copy saves.
	bitReverseBytesPerTask = 256 << 10
)

// bitReverseLogTile returns log2 of the tile side used by [bitReverseCopy]
// for elements of size elemSize bytes.
func bitReverseLogTile(elemSize uintptr) int {
	q := bitReverseMaxLogTile
	for q > 1 && uintptr(1)<<q*elemSize > bitReverseTileBytes {
		q--
	}
	return q
}

// bitReverseCopy writes dst[bitReverse(i)] = src[i] on up to workers
// goroutines (workers ≤ 0 means GOMAXPROCS). The slices must have the same
// power-of-two length and must not overlap.
//
// A plain scatter misses the cache on (nearly) every store, since consecutive
// sources land 2^(log n − 1) apart. Instead the log n index bits are split as
// i = a|c|b, with a and b of q bits each, so that bitReverse(i) = rev(b) |
// rev(c) | rev(a): for a fixed middle c, the B×B sources (a, b) are B
// contiguous runs of B elements, and their destinations are again B
// contiguous runs of B elements. The tile is transposed through a small
// L1-resident buffer, so each source and destination cache line is touched in
// one burst. Going through the buffer (rather than storing straight from the
// tile) matters: the B destination runs are a power-of-two stride apart and
// would otherwise evict each other from the same cache sets. The tiles of
// distinct c are disjoint and are spread over the workers.
//
// Each RS encode runs one copy per column, inside a parallel loop over the
// columns that gives every encode its share of the CPUs; workers is that
// share, so the copies do not oversubscribe the machine.
func bitReverseCopy[T any](dst, src []T, workers int) {
	n := len(src)
	if len(dst) != n || bits.OnesCount(uint(n)) != 1 {
		panic("fri: bitReverseCopy: slices must have the same power-of-two length")
	}
	logN := bits.TrailingZeros(uint(n))
	var zero T
	q := bitReverseLogTile(unsafe.Sizeof(zero))
	if logN < 2*q || n*int(unsafe.Sizeof(zero)) < bitReverseMinTiledBytes {
		// Too small for one tile, or small enough to stay in L1, where the
		// plain scatter does not miss and the tiling is pure overhead.
		gutils.BitReverseCopyNaive(dst, src)
		return
	}

	if workers <= 0 {
		workers = runtime.GOMAXPROCS(0)
	}
	workers = min(workers, max(1, n*int(unsafe.Sizeof(zero))/bitReverseBytesPerTask))

	B := 1 << q
	mid := logN - 2*q
	var revQ [1 << bitReverseMaxLogTile]int
	for x := range B {
		revQ[x] = int(bits.Reverse64(uint64(x)) >> (64 - q))
	}

	// a selects the source run (stride 2^(mid+q)); rev(b) the destination run.
	runStride := mid + q
	parallel.ExecuteWithNbTasks(1<<mid, workers, func(start, stop int) {
		buf := make([]T, B*B)
		for c := start; c < stop; c++ {
			rc := 0
			if mid > 0 {
				rc = int(bits.Reverse64(uint64(c)) >> (64 - mid))
			}
			for a := range B {
				srcRun := src[a<<runStride|c<<q:][:B]
				ra := revQ[a]
				for b, v := range srcRun {
					buf[revQ[b]*B+ra] = v
				}
			}
			for rb := range B {
				copy(dst[rb<<runStride|rc<<q:][:B], buf[rb*B:][:B])
			}
		}
	})
}
