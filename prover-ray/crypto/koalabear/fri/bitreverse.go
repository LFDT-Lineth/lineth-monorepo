package fri

import (
	"math/bits"
	"runtime"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/parallel"
	gutils "github.com/consensys/gnark-crypto/utils"
)

func bitReverse[T any](v []T) {
	gutils.BitReverse(v)
}

// bitReverseCopy writes dst[bitReverse(i)] = src[i]. Each RS encode runs one
// per column, so the serial gnark copy would leave a column's whole
// permutation on one core; the chunks of src are independent (a scatter with
// disjoint destinations), so they are spread over the CPUs.
func bitReverseCopy[T any](dst, src []T) {
	n := uint64(len(src))
	if n < 1<<16 || runtime.GOARCH == "arm64" {
		// The serial gnark copy (COBRA tiling on amd64) below 2^16 elements,
		// where the parallel dispatch would dominate, and at every size on
		// arm64.
		gutils.BitReverseCopy(dst, src)
		return
	}
	nn := uint64(64 - bits.TrailingZeros64(n))
	parallel.Execute(int(n), func(start, stop int) {
		for i := uint64(start); i < uint64(stop); i++ {
			dst[bits.Reverse64(i)>>nn] = src[i]
		}
	})
}
