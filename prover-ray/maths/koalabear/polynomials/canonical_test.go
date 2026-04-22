package polynomials

import (
	"math/rand/v2"
	"testing"

	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
)

const (
	testN    = 100
	testSeed = 42
)

func newRng() *rand.Rand {
	// #nosec G404 -- we are fine with this RNG for testcases
	return rand.New(rand.NewPCG(testSeed, 0))
}

func randBase(_ *rand.Rand) field.Element {
	var e field.Element
	if _, err := e.SetRandom(); err != nil {
		panic(err)
	}
	return e
}

func randExt(_ *rand.Rand) field.Ext {
	var e field.Ext
	if _, err := e.B0.A0.SetRandom(); err != nil {
		panic(err)
	}
	if _, err := e.B0.A1.SetRandom(); err != nil {
		panic(err)
	}
	if _, err := e.B1.A0.SetRandom(); err != nil {
		panic(err)
	}
	if _, err := e.B1.A1.SetRandom(); err != nil {
		panic(err)
	}
	return e
}

func extEq(a, b field.Ext) bool {
	return a.B0.A0.Equal(&b.B0.A0) &&
		a.B0.A1.Equal(&b.B0.A1) &&
		a.B1.A0.Equal(&b.B1.A0) &&
		a.B1.A1.Equal(&b.B1.A1)
}

// hornerExt evaluates p(X) = Σᵢ p[i]·Xⁱ at x using Horner's method directly
// on []Ext — used as a ground-truth reference in tests.
func hornerExt(poly []field.Ext, x field.Ext) field.Ext {
	var res field.Ext
	for i := len(poly) - 1; i >= 0; i-- {
		res.Mul(&res, &x)
		res.Add(&res, &poly[i])
	}
	return res
}

func TestEvalCanonical(t *testing.T) {
	rng := newRng()
	const polyLen = 10

	t.Run("base/base", func(t *testing.T) {
		for range testN {
			base := make([]field.Element, polyLen)
			for i := range base {
				base[i] = randBase(rng)
			}
			z := field.ElemFromBase(randBase(rng))
			poly := field.VecFromBase(base)

			got := EvalCanonical(poly, z)

			// Reference: lift to Ext and use hornerExt
			ext := make([]field.Ext, polyLen)
			for i, b := range base {
				ext[i] = field.Lift(b)
			}
			want := hornerExt(ext, z.AsExt())

			if !extEq(got.AsExt(), want) {
				t.Fatalf("base/base: got %v, want %v", got.AsExt(), want)
			}
			if !got.IsBase() {
				t.Fatal("base/base: result should be tagged base")
			}
		}
	})

	t.Run("ext/ext", func(t *testing.T) {
		for range testN {
			ext := make([]field.Ext, polyLen)
			for i := range ext {
				ext[i] = randExt(rng)
			}
			z := field.ElemFromExt(randExt(rng))
			poly := field.VecFromExt(ext)

			got := EvalCanonical(poly, z)
			want := hornerExt(ext, z.AsExt())

			if !extEq(got.AsExt(), want) {
				t.Fatalf("ext/ext: got %v, want %v", got.AsExt(), want)
			}
			if got.IsBase() {
				t.Fatal("ext/ext: result should not be tagged base")
			}
		}
	})

	t.Run("base/ext", func(t *testing.T) {
		for range testN {
			base := make([]field.Element, polyLen)
			for i := range base {
				base[i] = randBase(rng)
			}
			z := field.ElemFromExt(randExt(rng))
			poly := field.VecFromBase(base)

			got := EvalCanonical(poly, z)

			ext := make([]field.Ext, polyLen)
			for i, b := range base {
				ext[i] = field.Lift(b)
			}
			want := hornerExt(ext, z.AsExt())

			if !extEq(got.AsExt(), want) {
				t.Fatalf("base/ext: got %v, want %v", got.AsExt(), want)
			}
			if got.IsBase() {
				t.Fatal("base/ext: result should not be tagged base")
			}
		}
	})
}

func TestEvalCanonicalBatch(t *testing.T) {
	rng := newRng()
	lengths := []int{3, 7, 10, 1, 5}

	for range testN {
		polys := make([]field.Vec, len(lengths))
		for i, l := range lengths {
			base := make([]field.Element, l)
			for j := range base {
				base[j] = randBase(rng)
			}
			polys[i] = field.VecFromBase(base)
		}
		z := field.ElemFromBase(randBase(rng))

		batch := EvalCanonicalBatch(polys, z)
		if len(batch) != len(polys) {
			t.Fatalf("batch length mismatch: got %d, want %d", len(batch), len(polys))
		}
		for i, poly := range polys {
			want := EvalCanonical(poly, z)
			if !extEq(batch[i].AsExt(), want.AsExt()) {
				t.Fatalf("poly %d: batch=%v single=%v", i, batch[i].AsExt(), want.AsExt())
			}
		}
	}

	t.Run("empty", func(t *testing.T) {
		if EvalCanonicalBatch(nil, field.ElemOne()) != nil {
			t.Fatal("expected nil for empty input")
		}
	})

	t.Run("single_delegates", func(t *testing.T) {
		base := make([]field.Element, 5)
		for i := range base {
			base[i] = randBase(rng)
		}
		poly := field.VecFromBase(base)
		z := field.ElemFromBase(randBase(rng))
		batch := EvalCanonicalBatch([]field.Vec{poly}, z)
		want := EvalCanonical(poly, z)
		if !extEq(batch[0].AsExt(), want.AsExt()) {
			t.Fatal("single-poly batch should match EvalCanonical")
		}
	})
}
