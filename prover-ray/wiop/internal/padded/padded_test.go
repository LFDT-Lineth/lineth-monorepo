package padded

import (
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/stretchr/testify/require"
)

// naive returns row r of the padded column, row by row.
func naive[T any](data []T, pad T, start, n, r int) T {
	r %= n
	if r >= start && r < start+len(data) {
		return data[r-start]
	}
	return pad
}

// TestReadMatchesNaive checks Read and ReadLift against a row-by-row
// reference on every start (including overflowing data on either side),
// every first row and every read length up to two wraps.
func TestReadMatchesNaive(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	for _, n := range []int{1, 2, 3, 8} {
		for dataLen := 0; dataLen <= n+2; dataLen++ {
			data := make([]field.Element, dataLen)
			for i := range data {
				data[i].SetUint64(rng.Uint64())
			}
			var pad field.Element
			pad.SetUint64(rng.Uint64())
			for start := -dataLen - 1; start <= n+1; start++ {
				for idx := range n {
					for L := 0; L <= 2*n+1; L++ {
						out := make([]field.Element, L)
						outExt := make([]field.Ext, L)
						Read(out, data, pad, start, n, idx)
						ReadLift(outExt, data, pad, start, n, idx)
						for i := range L {
							want := naive(data, pad, start, n, idx+i)
							require.Equal(t, want, out[i], "n=%d len=%d start=%d idx=%d L=%d row %d", n, dataLen, start, idx, L, i)
							require.Equal(t, field.Lift(want), outExt[i], "lift: n=%d len=%d start=%d idx=%d L=%d row %d", n, dataLen, start, idx, L, i)
						}
					}
				}
			}
		}
	}
}
