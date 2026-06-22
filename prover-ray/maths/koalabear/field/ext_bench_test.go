package field

import (
	"math/rand/v2"
	"testing"
)

// Benchmarks intended to be diffed across the E4→E6 migration.

const benchN = 1 << 16

func benchSeededRng() *rand.Rand {
	// #nosec G404 -- deterministic seed for reproducible benchmarks.
	return rand.New(rand.NewPCG(1, 2))
}

func benchExts(n int) []Ext {
	rng := benchSeededRng()
	v := make([]Ext, n)
	for i := range v {
		v[i] = PseudoRandExt(rng)
	}
	return v
}

func benchElems(n int) []Element {
	rng := benchSeededRng()
	v := make([]Element, n)
	for i := range v {
		v[i] = PseudoRand(rng)
	}
	return v
}

// BenchmarkExtMul measures cost of a single full extension multiplication.
func BenchmarkExtMul(b *testing.B) {
	xs := benchExts(2)
	x, y := xs[0], xs[1]
	var z Ext
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		z.Mul(&x, &y)
	}
	_ = z
}

// BenchmarkExtSquare measures cost of a single extension squaring.
func BenchmarkExtSquare(b *testing.B) {
	xs := benchExts(1)
	x := xs[0]
	var z Ext
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		z.Square(&x)
	}
	_ = z
}

// BenchmarkExtMulByBase measures cost of multiplying an extension element by a
// base field scalar (the common "scale" path).
func BenchmarkExtMulByBase(b *testing.B) {
	x := benchExts(1)[0]
	s := benchElems(1)[0]
	var z Ext
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		z.MulByElement(&x, &s)
	}
	_ = z
}

// BenchmarkExtInverse measures cost of a single extension inversion.
func BenchmarkExtInverse(b *testing.B) {
	x := benchExts(1)[0]
	var z Ext
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		z.Inverse(&x)
	}
	_ = z
}

// BenchmarkBatchInvertExt measures the cost of the Montgomery batch inversion
// over a fixed-size vector.
func BenchmarkBatchInvertExt(b *testing.B) {
	a := benchExts(benchN)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = BatchInvertExt(a)
	}
}

// BenchmarkParBatchInvertExt measures parallel batch inversion across all CPU
// cores; the only difference from the sequential one is the goroutine fan-out.
func BenchmarkParBatchInvertExt(b *testing.B) {
	a := benchExts(benchN)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ParBatchInvertExt(a, 0)
	}
}

// BenchmarkVecAddExtExt measures element-wise extension addition over a vector.
func BenchmarkVecAddExtExt(b *testing.B) {
	a := benchExts(benchN)
	c := benchExts(benchN)
	out := make([]Ext, benchN)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		VecAddExtExt(out, a, c)
	}
}

// BenchmarkVecMulExtExt measures element-wise extension multiplication.
func BenchmarkVecMulExtExt(b *testing.B) {
	a := benchExts(benchN)
	c := benchExts(benchN)
	out := make([]Ext, benchN)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		VecMulExtExt(out, a, c)
	}
}

// BenchmarkVecScaleBaseExt measures the cost of scaling an extension vector
// by a single base scalar (uses MulByElement under the hood).
func BenchmarkVecScaleBaseExt(b *testing.B) {
	a := benchExts(benchN)
	s := benchElems(1)[0]
	out := make([]Ext, benchN)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		VecScaleBaseExt(out, s, a)
	}
}
