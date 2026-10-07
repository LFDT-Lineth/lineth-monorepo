package global

import (
	"fmt"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/stretchr/testify/require"
)

// serialCancellationCoset is the reference evaluation of the cancellation
// polynomial on the coset: a single pass stepping x by ω_N.
func serialCancellationCoset(cancelled []int, n, N int) []field.Element {
	omega := field.RootOfUnityBy(n)
	roots := make([]field.Element, len(cancelled))
	for i, pos := range cancelled {
		k := pos
		if k < 0 {
			k = n + pos
		}
		field.ExpToInt(&roots[i], omega, k)
	}
	omegaN := field.RootOfUnityBy(N)
	var x field.Element
	x.SetUint64(field.MultiplicativeGen)
	out := make([]field.Element, N)
	for j := range N {
		out[j].SetOne()
		for _, root := range roots {
			var diff field.Element
			diff.Sub(&x, &root)
			out[j].Mul(&out[j], &diff)
		}
		x.Mul(&x, &omegaN)
	}
	return out
}

// serialLagrangeSelectorCoset is the reference evaluation of L_p on the coset,
// one point at a time with a field inversion per point.
func serialLagrangeSelectorCoset(position, n, N int) []field.Element {
	p := ((position % n) + n) % n
	var omegaP, nInv, numCoef, x, one field.Element
	field.ExpToInt(&omegaP, field.RootOfUnityBy(n), p)
	nInv.SetUint64(uint64(n))
	nInv.Inverse(&nInv)
	numCoef.Mul(&omegaP, &nInv)
	one.SetOne()
	omegaN := field.RootOfUnityBy(N)
	x.SetUint64(field.MultiplicativeGen)
	out := make([]field.Element, N)
	for j := range N {
		var xn, num, den field.Element
		field.ExpToInt(&xn, x, n)
		num.Sub(&xn, &one)
		den.Sub(&x, &omegaP)
		den.Inverse(&den)
		out[j].Mul(&numCoef, &num)
		out[j].Mul(&out[j], &den)
		x.Mul(&x, &omegaN)
	}
	return out
}

// The parallel coset evaluations split the points into chunks that each start
// from their own power of ω_N; they must agree with a single serial pass, for
// domains small enough to give one point per worker and large enough to give
// many points per worker.
func TestCosetsMatchSerialReference(t *testing.T) {
	for _, sz := range []struct{ n, ratio int }{{1, 1}, {8, 4}, {64, 2}, {1024, 4}, {1 << 14, 2}} {
		n, N := sz.n, sz.n*sz.ratio
		t.Run(fmt.Sprintf("n=%d/N=%d", n, N), func(t *testing.T) {
			for _, cancelled := range [][]int{{0}, {-1}, {0, -1}, {1, 2, -2}} {
				require.Equal(t, serialCancellationCoset(cancelled, n, N), computeCancellationCoset(cancelled, n, N),
					"cancelled %v", cancelled)
			}
			for _, pos := range []int{0, 1, -1, n - 1, 3*n + 2} {
				require.Equal(t, serialLagrangeSelectorCoset(pos, n, N), computeLagrangeSelectorCoset(pos, n, N),
					"position %d", pos)
			}
		})
	}
}

// The per-bucket cache computes each distinct set of cancelled positions once
// and hands every vanishing that cancels the same rows the same slice.
func TestCancellationCosetsCache(t *testing.T) {
	const n, N = 64, 256
	c := newCancellationCosets(n, N)
	require.Nil(t, c.get(nil))

	a, b := c.get([]int{0, -1}), c.get([]int{0, -1})
	require.Equal(t, computeCancellationCoset([]int{0, -1}, n, N), a)
	require.Same(t, &a[0], &b[0], "same positions must share one coset")

	other := c.get([]int{-1})
	require.Equal(t, computeCancellationCoset([]int{-1}, n, N), other)
	require.NotSame(t, &a[0], &other[0])
	require.Len(t, c.byKey, 2)
}
