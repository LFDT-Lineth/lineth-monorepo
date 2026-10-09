package field

import (
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/require"
)

// The vectorised product must equal Ext.Mul element by element, for lengths
// around the 16-lane width, in place, and on limbs at the ends of the range.
func TestVecMulExtExtMatchesMul(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	var qMinusOne Element
	qMinusOne.SetOne()
	qMinusOne.Neg(&qMinusOne)
	for _, n := range []int{0, 1, 15, 16, 17, 31, 32, 100, 1024 + 7} {
		a, b := VecPseudoRandExt(rng, n), VecPseudoRandExt(rng, n)
		for i := 0; i < n; i += 5 {
			// Extreme limbs: zero and q-1 in every coordinate.
			for _, v := range []*Ext{&a[i], &b[(i+3)%n]} {
				v.B0.A0, v.B1.A1, v.B2.A0 = qMinusOne, qMinusOne, qMinusOne
				v.B0.A1, v.B2.A1 = Element{}, qMinusOne
			}
		}
		want := make([]Ext, n)
		for i := range want {
			want[i].Mul(&a[i], &b[i])
		}
		got := make([]Ext, n)
		VecMulExtExt(got, a, b)
		require.Equal(t, want, got, "n=%d", n)

		inPlace := make([]Ext, n)
		copy(inPlace, a)
		VecMulExtExt(inPlace, inPlace, b)
		require.Equal(t, want, inPlace, "n=%d in place", n)

		squares := make([]Ext, n)
		VecMulExtExt(squares, a, a)
		for i := range squares {
			var sq Ext
			sq.Mul(&a[i], &a[i])
			require.Equal(t, sq, squares[i], "n=%d square %d", n, i)
		}
	}
}

func BenchmarkVecMulExtExtVsScalar(b *testing.B) {
	rng := rand.New(rand.NewPCG(3, 4))
	x, y := VecPseudoRandExt(rng, 256), VecPseudoRandExt(rng, 256)
	z := make([]Ext, 256)
	b.Run("vector", func(b *testing.B) {
		for b.Loop() {
			VecMulExtExt(z, x, y)
		}
	})
	b.Run("scalar", func(b *testing.B) {
		for b.Loop() {
			for i := range z {
				z[i].Mul(&x[i], &y[i])
			}
		}
	})
}
