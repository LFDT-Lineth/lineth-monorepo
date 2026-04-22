package field

import (
	"math/rand/v2"
	"testing"
)

// Test constants shared across elem_test.go and vec_test.go.
const (
	testN    = 100 // number of random inputs per subtest
	testSeed = 42  // fixed seed for reproducibility
)

// newRng returns a deterministic RNG seeded with testSeed.
func newRng() *rand.Rand {
	// #nosec G404 -- we are fine with this RNG for testcases
	return rand.New(rand.NewPCG(testSeed, 0))
}

// extEq reports whether two Ext values are equal by comparing each coordinate.
func extEq(a, b Ext) bool {
	return a.B0.A0.Equal(&b.B0.A0) &&
		a.B0.A1.Equal(&b.B0.A1) &&
		a.B1.A0.Equal(&b.B1.A0) &&
		a.B1.A1.Equal(&b.B1.A1)
}

// checkExt marks t as failed if want != got.
func checkExt(t *testing.T, want, got Ext) {
	t.Helper()
	if !extEq(want, got) {
		t.Errorf("Ext mismatch:\n  want %v\n   got %v", want, got)
	}
}

// checkElem marks t as failed if want != got.
func checkElem(t *testing.T, want, got Element) {
	t.Helper()
	if !want.Equal(&got) {
		t.Errorf("Element mismatch:\n  want %v\n   got %v", want, got)
	}
}

// checkPanics marks t as failed if f does not panic.
func checkPanics(t *testing.T, f func()) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected a panic, got none")
		}
	}()
	f()
}

// TestElemConstructors verifies that each constructor sets IsBase and the
// stored value correctly, and that AsExt always returns the embedded Ext.
func TestElemConstructors(t *testing.T) {
	rng := newRng()

	t.Run("ElemFromBase", func(t *testing.T) {
		for range testN {
			e := PseudoRand(rng)
			fe := ElemFromBase(e)

			if !fe.IsBase() {
				t.Fatal("ElemFromBase: IsBase() should be true")
			}
			checkElem(t, e, fe.AsBase())
			// AsExt must return the lift: B0.A0 == e, extension coords == 0.
			want := Lift(e)
			checkExt(t, want, fe.AsExt())
		}
	})

	t.Run("ElemFromExt", func(t *testing.T) {
		for range testN {
			e := PseudoRandExt(rng)
			fe := ElemFromExt(e)

			if fe.IsBase() {
				t.Fatal("ElemFromExt: IsBase() should be false")
			}
			checkExt(t, e, fe.AsExt())
		}
	})

	t.Run("ElemZero", func(t *testing.T) {
		fe := ElemZero()
		if !fe.IsBase() {
			t.Fatal("ElemZero: IsBase() should be true")
		}
		want := Zero()
		checkElem(t, want, fe.AsBase())
	})

	t.Run("ElemOne", func(t *testing.T) {
		fe := ElemOne()
		if !fe.IsBase() {
			t.Fatal("ElemOne: IsBase() should be true")
		}
		want := One()
		checkElem(t, want, fe.AsBase())
	})
}

// TestElemAsBasePanic verifies that AsBase panics on a non-base FieldElem.
func TestElemAsBasePanic(t *testing.T) {
	rng := newRng()
	fe := ElemFromExt(PseudoRandExt(rng))
	checkPanics(t, func() { fe.AsBase() })
}

// TestElemAdd verifies Add for all four base/ext combinations and checks that
// the result is consistent with direct Element / Ext arithmetic.
func TestElemAdd(t *testing.T) {
	rng := newRng()

	t.Run("BaseBase", func(t *testing.T) {
		for range testN {
			a, b := PseudoRand(rng), PseudoRand(rng)
			got := ElemFromBase(a).Add(ElemFromBase(b))
			if !got.IsBase() {
				t.Error("base+base: IsBase() should be true")
			}
			var want Element
			want.Add(&a, &b)
			checkElem(t, want, got.AsBase())
		}
	})

	t.Run("ExtBase", func(t *testing.T) {
		for range testN {
			a, b := PseudoRandExt(rng), PseudoRand(rng)
			got := ElemFromExt(a).Add(ElemFromBase(b))
			if got.IsBase() {
				t.Error("ext+base: IsBase() should be false")
			}
			bLift := Lift(b)
			var want Ext
			want.Add(&a, &bLift)
			checkExt(t, want, got.AsExt())
		}
	})

	t.Run("BaseExt", func(t *testing.T) {
		for range testN {
			a, b := PseudoRand(rng), PseudoRandExt(rng)
			got := ElemFromBase(a).Add(ElemFromExt(b))
			if got.IsBase() {
				t.Error("base+ext: IsBase() should be false")
			}
			aLift := Lift(a)
			var want Ext
			want.Add(&aLift, &b)
			checkExt(t, want, got.AsExt())
		}
	})

	t.Run("ExtExt", func(t *testing.T) {
		for range testN {
			a, b := PseudoRandExt(rng), PseudoRandExt(rng)
			got := ElemFromExt(a).Add(ElemFromExt(b))
			if got.IsBase() {
				t.Error("ext+ext: IsBase() should be false")
			}
			var want Ext
			want.Add(&a, &b)
			checkExt(t, want, got.AsExt())
		}
	})
}

// TestElemSub verifies Sub for all four base/ext combinations.
func TestElemSub(t *testing.T) {
	rng := newRng()

	t.Run("BaseBase", func(t *testing.T) {
		for range testN {
			a, b := PseudoRand(rng), PseudoRand(rng)
			got := ElemFromBase(a).Sub(ElemFromBase(b))
			if !got.IsBase() {
				t.Error("base-base: IsBase() should be true")
			}
			var want Element
			want.Sub(&a, &b)
			checkElem(t, want, got.AsBase())
		}
	})

	t.Run("ExtBase", func(t *testing.T) {
		for range testN {
			a, b := PseudoRandExt(rng), PseudoRand(rng)
			got := ElemFromExt(a).Sub(ElemFromBase(b))
			if got.IsBase() {
				t.Error("ext-base: IsBase() should be false")
			}
			bLift := Lift(b)
			var want Ext
			want.Sub(&a, &bLift)
			checkExt(t, want, got.AsExt())
		}
	})

	t.Run("BaseExt", func(t *testing.T) {
		for range testN {
			a, b := PseudoRand(rng), PseudoRandExt(rng)
			got := ElemFromBase(a).Sub(ElemFromExt(b))
			if got.IsBase() {
				t.Error("base-ext: IsBase() should be false")
			}
			aLift := Lift(a)
			var want Ext
			want.Sub(&aLift, &b)
			checkExt(t, want, got.AsExt())
		}
	})

	t.Run("ExtExt", func(t *testing.T) {
		for range testN {
			a, b := PseudoRandExt(rng), PseudoRandExt(rng)
			got := ElemFromExt(a).Sub(ElemFromExt(b))
			if got.IsBase() {
				t.Error("ext-ext: IsBase() should be false")
			}
			var want Ext
			want.Sub(&a, &b)
			checkExt(t, want, got.AsExt())
		}
	})
}

// TestElemMul verifies Mul for all four combinations. For mixed cases it also
// checks consistency with Ext.MulByElement and full Ext.Mul to confirm the
// three paths are mathematically equivalent.
func TestElemMul(t *testing.T) {
	rng := newRng()

	t.Run("BaseBase", func(t *testing.T) {
		for range testN {
			a, b := PseudoRand(rng), PseudoRand(rng)
			got := ElemFromBase(a).Mul(ElemFromBase(b))
			if !got.IsBase() {
				t.Error("base*base: IsBase() should be true")
			}
			var want Element
			want.Mul(&a, &b)
			checkElem(t, want, got.AsBase())
		}
	})

	t.Run("ExtBase", func(t *testing.T) {
		// The implementation uses MulByElement. Verify against both
		// MulByElement and full Ext.Mul (they must give the same result).
		for range testN {
			a, b := PseudoRandExt(rng), PseudoRand(rng)
			got := ElemFromExt(a).Mul(ElemFromBase(b))
			if got.IsBase() {
				t.Error("ext*base: IsBase() should be false")
			}
			var wantMBE Ext
			wantMBE.MulByElement(&a, &b)
			checkExt(t, wantMBE, got.AsExt())

			bLift := Lift(b)
			var wantFull Ext
			wantFull.Mul(&a, &bLift)
			checkExt(t, wantFull, got.AsExt())
		}
	})

	t.Run("BaseExt", func(t *testing.T) {
		for range testN {
			a, b := PseudoRand(rng), PseudoRandExt(rng)
			got := ElemFromBase(a).Mul(ElemFromExt(b))
			if got.IsBase() {
				t.Error("base*ext: IsBase() should be false")
			}
			var wantMBE Ext
			wantMBE.MulByElement(&b, &a)
			checkExt(t, wantMBE, got.AsExt())
		}
	})

	t.Run("ExtExt", func(t *testing.T) {
		for range testN {
			a, b := PseudoRandExt(rng), PseudoRandExt(rng)
			got := ElemFromExt(a).Mul(ElemFromExt(b))
			if got.IsBase() {
				t.Error("ext*ext: IsBase() should be false")
			}
			var want Ext
			want.Mul(&a, &b)
			checkExt(t, want, got.AsExt())
		}
	})
}

// TestElemNeg verifies Neg preserves IsBase and produces the correct negation.
func TestElemNeg(t *testing.T) {
	rng := newRng()

	t.Run("Base", func(t *testing.T) {
		for range testN {
			a := PseudoRand(rng)
			got := ElemFromBase(a).Neg()
			if !got.IsBase() {
				t.Error("neg(base): IsBase() should be true")
			}
			var want Element
			want.Neg(&a)
			checkElem(t, want, got.AsBase())
		}
	})

	t.Run("Ext", func(t *testing.T) {
		for range testN {
			a := PseudoRandExt(rng)
			got := ElemFromExt(a).Neg()
			if got.IsBase() {
				t.Error("neg(ext): IsBase() should be false")
			}
			var want Ext
			want.Neg(&a)
			checkExt(t, want, got.AsExt())
		}
	})
}

// TestElemSquare verifies Square preserves IsBase and gives the correct result.
func TestElemSquare(t *testing.T) {
	rng := newRng()

	t.Run("Base", func(t *testing.T) {
		for range testN {
			a := PseudoRand(rng)
			got := ElemFromBase(a).Square()
			if !got.IsBase() {
				t.Error("sq(base): IsBase() should be true")
			}
			var want Element
			want.Square(&a)
			checkElem(t, want, got.AsBase())
		}
	})

	t.Run("Ext", func(t *testing.T) {
		for range testN {
			a := PseudoRandExt(rng)
			got := ElemFromExt(a).Square()
			if got.IsBase() {
				t.Error("sq(ext): IsBase() should be false")
			}
			var want Ext
			want.Square(&a)
			checkExt(t, want, got.AsExt())
		}
	})
}

// TestElemInverse verifies Inverse preserves IsBase and satisfies a * a⁻¹ = 1.
func TestElemInverse(t *testing.T) {
	rng := newRng()

	oneExt := OneExt()

	t.Run("Base", func(t *testing.T) {
		for range testN {
			a := PseudoRand(rng)
			inv := ElemFromBase(a).Inverse()
			if !inv.IsBase() {
				t.Error("inv(base): IsBase() should be true")
			}
			var want Element
			want.Inverse(&a)
			checkElem(t, want, inv.AsBase())

			// a * a⁻¹ == 1
			product := ElemFromBase(a).Mul(inv)
			checkExt(t, oneExt, product.AsExt())
		}
	})

	t.Run("Ext", func(t *testing.T) {
		for range testN {
			a := PseudoRandExt(rng)
			inv := ElemFromExt(a).Inverse()
			if inv.IsBase() {
				t.Error("inv(ext): IsBase() should be false")
			}
			var want Ext
			want.Inverse(&a)
			checkExt(t, want, inv.AsExt())

			// a * a⁻¹ == 1
			product := ElemFromExt(a).Mul(inv)
			checkExt(t, oneExt, product.AsExt())
		}
	})
}

// TestElemDiv verifies Div for all four combinations by checking that
// (a / b) * b == a (round-trip), and that IsBase propagates correctly.
func TestElemDiv(t *testing.T) {
	rng := newRng()

	cases := []struct {
		name     string
		mkA      func() Gen
		mkB      func() Gen
		wantBase bool
	}{
		{
			"BaseBase",
			func() Gen { return ElemFromBase(PseudoRand(rng)) },
			func() Gen { return ElemFromBase(PseudoRand(rng)) },
			true,
		},
		{
			"ExtBase",
			func() Gen { return ElemFromExt(PseudoRandExt(rng)) },
			func() Gen { return ElemFromBase(PseudoRand(rng)) },
			false,
		},
		{
			"BaseExt",
			func() Gen { return ElemFromBase(PseudoRand(rng)) },
			func() Gen { return ElemFromExt(PseudoRandExt(rng)) },
			false,
		},
		{
			"ExtExt",
			func() Gen { return ElemFromExt(PseudoRandExt(rng)) },
			func() Gen { return ElemFromExt(PseudoRandExt(rng)) },
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for range testN {
				a, b := tc.mkA(), tc.mkB()
				q := a.Div(b)
				if q.IsBase() != tc.wantBase {
					t.Errorf("IsBase() = %v, want %v", q.IsBase(), tc.wantBase)
				}
				// Round-trip: (a / b) * b should equal a.
				roundTrip := q.Mul(b)
				checkExt(t, a.AsExt(), roundTrip.AsExt())
			}
		})
	}
}
