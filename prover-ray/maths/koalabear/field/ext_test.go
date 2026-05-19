package field

import (
	"math/big"
	"testing"
)

// TestNewExtFromUints verifies that UintsToExt correctly sets all six
// extension coordinates to the given canonical values.
func TestNewExtFromUints(t *testing.T) {
	e := UintsToExt(10, 20, 30, 40, 50, 60)
	checks := []struct {
		name string
		got  uint64
		want uint64
	}{
		{"B0.A0", e.B0.A0.Uint64(), 10},
		{"B0.A1", e.B0.A1.Uint64(), 20},
		{"B1.A0", e.B1.A0.Uint64(), 30},
		{"B1.A1", e.B1.A1.Uint64(), 40},
		{"B2.A0", e.B2.A0.Uint64(), 50},
		{"B2.A1", e.B2.A1.Uint64(), 60},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}
}

// TestNewExtFromInt verifies positive and negative int64 inputs.
// Negative values are reduced mod p, consistent with Element.SetInt64.
func TestNewExtFromInt(t *testing.T) {
	e := IntsToExt(1, 2, 3, 4, 5, 6)
	got := []uint64{
		e.B0.A0.Uint64(), e.B0.A1.Uint64(),
		e.B1.A0.Uint64(), e.B1.A1.Uint64(),
		e.B2.A0.Uint64(), e.B2.A1.Uint64(),
	}
	for i, v := range got {
		if v != uint64(i+1) {
			t.Errorf("coordinate %d = %d, want %d", i, v, i+1)
		}
	}
	// Negative: SetInt64(-1) gives p-1.
	e2 := IntsToExt(-1, 0, 0, 0, 0, 0)
	var want Element
	want.SetInt64(-1)
	checkElem(t, want, e2.B0.A0)
}

// TestNewExtFromString verifies that only the first coordinate is set and the
// rest remain zero.
func TestNewExtFromString(t *testing.T) {
	e := NewExtFromString("42")
	want := NewFromString("42")
	checkElem(t, want, e.B0.A0)
	if !extUpperIsZero(e) {
		t.Error("NewExtFromString: extension coordinates should be zero")
	}
}

// TestBatchInvertExtRoundTrip verifies the round-trip property: a[i] * inv[i] == 1.
func TestBatchInvertExtRoundTrip(t *testing.T) {
	a := randomExts(testN)
	inv := BatchInvertExt(a)
	oneExt := OneExt()
	for i := range a {
		var prod Ext
		prod.Mul(&a[i], &inv[i])
		checkExt(t, oneExt, prod)
	}
}

// TestBatchInvertExtInto exercises the edge cases of BatchInvertExtInto:
// length mismatch (panic), empty slice (no-op), and zero elements (skipped).
func TestBatchInvertExtInto(t *testing.T) {
	t.Run("LengthMismatch", func(t *testing.T) {
		checkPanics(t, func() {
			BatchInvertExtInto(randomExts(testN), make([]Ext, testN+1))
		})
	})

	t.Run("EmptySlice", func(_ *testing.T) {
		// Must return immediately without panicking.
		BatchInvertExtInto([]Ext{}, []Ext{})
	})

	t.Run("ZeroElement", func(t *testing.T) {
		// A zero element at index 0 is silently skipped; non-zero elements are
		// correctly inverted. The result for the zero position is left as the
		// zero value of a freshly allocated slice.
		//
		// Note: the documentation of VecBatchInvExt says "panics if any a[i]
		// is zero", but BatchInvertExtInto (which it delegates to) does NOT
		// panic — it skips zero entries instead.
		a := randomExts(testN)
		a[0] = ZeroExt()
		res := make([]Ext, testN)
		BatchInvertExtInto(a, res) // must not panic

		oneExt := OneExt()
		for i := 1; i < testN; i++ {
			var prod Ext
			prod.Mul(&a[i], &res[i])
			checkExt(t, oneExt, prod)
		}
		// The zero element's slot is left unmodified (zero by allocation).
		if !res[0].IsZero() {
			t.Error("BatchInvertExtInto: result slot for zero input should remain zero")
		}
	})
}

// TestIsBaseFunc verifies the package-level IsBase(*Ext) function (distinct
// from FieldElem.IsBase). A lifted base element must return true; a generic
// extension element with non-zero upper coordinates must return false.
func TestIsBaseFunc(t *testing.T) {
	rng := newRng()
	for range testN {
		e := PseudoRand(rng)
		lifted := Lift(e)
		if !IsBase(&lifted) {
			t.Error("IsBase: lifted base element should be true")
		}
	}
	for range testN {
		e := PseudoRandExt(rng)
		// Skip the rare case where all extension coordinates happen to be zero.
		if extUpperIsZero(e) {
			continue
		}
		if IsBase(&e) {
			t.Error("IsBase: extension element with non-zero upper coords should be false")
		}
	}
}

// TestGetBase verifies that GetBase returns the base component for lifted
// elements and an error for genuine extension elements.
func TestGetBase(t *testing.T) {
	rng := newRng()
	for range testN {
		e := PseudoRand(rng)
		lifted := Lift(e)
		got, isBase := GetBase(&lifted)
		if !isBase {
			t.Fatalf("GetBase on lifted element failed: f: %v", ExtToText(&lifted, 10))
		}
		checkElem(t, e, got)
	}
	for range testN {
		e := PseudoRandExt(rng)
		if extUpperIsZero(e) {
			continue
		}
		if _, isBase := GetBase(&e); isBase {
			t.Error("GetBase: expected error for non-base extension element, got nil")
		}
	}
}

// TestAddByBase verifies that AddByBase(z, x, y) computes z = x + Lift(y).
func TestAddByBase(t *testing.T) {
	rng := newRng()
	for range testN {
		x := PseudoRandExt(rng)
		y := PseudoRand(rng)
		var z Ext
		AddByBase(&z, &x, &y)
		yLift := Lift(y)
		var want Ext
		want.Add(&x, &yLift)
		checkExt(t, want, z)
	}
}

// TestDivByBase verifies that DivByBase(z, x, y) computes z = x / Lift(y) via
// the round-trip property (x/y)*y == x.
func TestDivByBase(t *testing.T) {
	rng := newRng()
	for range testN {
		x := PseudoRandExt(rng)
		y := PseudoRand(rng)
		var z Ext
		DivByBase(&z, &x, &y)
		yLift := Lift(y)
		var product Ext
		product.Mul(&z, &yLift)
		checkExt(t, x, product)
	}
}

// TestSetInterface verifies SetInterface for all supported input types,
// including nil and unknown-type error paths.
func TestSetInterface(t *testing.T) {
	rng := newRng()

	t.Run("nil", func(t *testing.T) {
		var z Ext
		if _, err := SetInterface(&z, nil); err == nil {
			t.Error("expected error for nil input")
		}
	})

	t.Run("Ext", func(t *testing.T) {
		e := PseudoRandExt(rng)
		var z Ext
		if _, err := SetInterface(&z, e); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		checkExt(t, e, z)
	})

	t.Run("*Ext", func(t *testing.T) {
		e := PseudoRandExt(rng)
		var z Ext
		if _, err := SetInterface(&z, &e); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		checkExt(t, e, z)
	})

	t.Run("*ExtNil", func(t *testing.T) {
		var z Ext
		var ePtr *Ext
		if _, err := SetInterface(&z, ePtr); err == nil {
			t.Error("expected error for nil *Ext")
		}
	})

	t.Run("Element", func(t *testing.T) {
		e := PseudoRand(rng)
		var z Ext
		if _, err := SetInterface(&z, e); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		checkExt(t, Lift(e), z)
	})

	t.Run("*Element", func(t *testing.T) {
		e := PseudoRand(rng)
		var z Ext
		if _, err := SetInterface(&z, &e); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		checkExt(t, Lift(e), z)
	})

	t.Run("*ElementNil", func(t *testing.T) {
		var z Ext
		var ePtr *Element
		if _, err := SetInterface(&z, ePtr); err == nil {
			t.Error("expected error for nil *Element")
		}
	})

	t.Run("uint8", func(t *testing.T) {
		var z Ext
		if _, err := SetInterface(&z, uint8(5)); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if z.B0.A0.Uint64() != 5 {
			t.Errorf("B0.A0 = %d, want 5", z.B0.A0.Uint64())
		}
	})

	t.Run("uint64", func(t *testing.T) {
		var z Ext
		if _, err := SetInterface(&z, uint64(99)); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if z.B0.A0.Uint64() != 99 {
			t.Errorf("B0.A0 = %d, want 99", z.B0.A0.Uint64())
		}
	})

	t.Run("int8", func(t *testing.T) {
		var z Ext
		if _, err := SetInterface(&z, int8(7)); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if z.B0.A0.Uint64() != 7 {
			t.Errorf("B0.A0 = %d, want 7", z.B0.A0.Uint64())
		}
	})

	t.Run("int64", func(t *testing.T) {
		var z Ext
		if _, err := SetInterface(&z, int64(42)); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if z.B0.A0.Uint64() != 42 {
			t.Errorf("B0.A0 = %d, want 42", z.B0.A0.Uint64())
		}
	})

	t.Run("int", func(t *testing.T) {
		var z Ext
		if _, err := SetInterface(&z, int(13)); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if z.B0.A0.Uint64() != 13 {
			t.Errorf("B0.A0 = %d, want 13", z.B0.A0.Uint64())
		}
	})

	t.Run("string", func(t *testing.T) {
		var z Ext
		if _, err := SetInterface(&z, "123"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if z.B0.A0.Uint64() != 123 {
			t.Errorf("B0.A0 = %d, want 123", z.B0.A0.Uint64())
		}
	})

	t.Run("*big.Int", func(t *testing.T) {
		bi := big.NewInt(999)
		var z Ext
		if _, err := SetInterface(&z, bi); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if z.B0.A0.Uint64() != 999 {
			t.Errorf("B0.A0 = %d, want 999", z.B0.A0.Uint64())
		}
	})

	t.Run("*big.IntNil", func(t *testing.T) {
		var z Ext
		var bi *big.Int
		if _, err := SetInterface(&z, bi); err == nil {
			t.Error("expected error for nil *big.Int")
		}
	})

	t.Run("big.Int", func(t *testing.T) {
		bi := *big.NewInt(777)
		var z Ext
		if _, err := SetInterface(&z, bi); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if z.B0.A0.Uint64() != 777 {
			t.Errorf("B0.A0 = %d, want 777", z.B0.A0.Uint64())
		}
	})

	t.Run("baseInputsClearUpperCoordinates", func(t *testing.T) {
		cases := []struct {
			name  string
			input any
			want  uint64
		}{
			{name: "string", input: "123", want: 123},
			{name: "*big.Int", input: big.NewInt(999), want: 999},
			{name: "big.Int", input: *big.NewInt(777), want: 777},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				z := PseudoRandExt(rng)
				if _, err := SetInterface(&z, tc.input); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if z.B0.A0.Uint64() != tc.want {
					t.Errorf("B0.A0 = %d, want %d", z.B0.A0.Uint64(), tc.want)
				}
				if !extUpperIsZero(z) {
					t.Error("SetInterface base input should clear all extension coordinates")
				}
			})
		}
	})

	t.Run("[]byte", func(t *testing.T) {
		// The []byte case in SetInterface returns a new *Ext built via
		// BytesToExt rather than modifying the receiver. This is an
		// inconsistency with every other type case (all of which modify the
		// receiver z), but it is the current behaviour, so the test checks
		// the returned pointer.
		data := make([]byte, ExtensionDegree*Bytes)
		for i := range data {
			data[i] = byte((i*17 + 3) & 0xff)
		}
		var z Ext
		result, err := SetInterface(&z, data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := BytesToExt(data)
		checkExt(t, want, *result)
	})

	t.Run("unknownType", func(t *testing.T) {
		var z Ext
		if _, err := SetInterface(&z, struct{}{}); err == nil {
			t.Error("expected error for unknown type")
		}
	})
}

// TestText verifies that Text panics for out-of-range bases, returns "<nil>"
// for a nil pointer, and produces non-empty output for a valid element.
func TestText(t *testing.T) {
	t.Run("baseTooSmall", func(t *testing.T) {
		z := ZeroExt()
		checkPanics(t, func() { ExtToText(&z, 1) })
	})

	t.Run("baseTooLarge", func(t *testing.T) {
		z := ZeroExt()
		checkPanics(t, func() { ExtToText(&z, 37) })
	})

	t.Run("nilPointer", func(t *testing.T) {
		if got := ExtToText(nil, 10); got != "<nil>" {
			t.Errorf("Text(nil, 10) = %q, want \"<nil>\"", got)
		}
	})

	t.Run("validDecimal", func(t *testing.T) {
		z := ZeroExt()
		if got := ExtToText(&z, 10); got == "" {
			t.Error("Text: returned empty string for valid input")
		}
	})
}

// TestParBatchInvertExt verifies that ParBatchInvertExt returns the same
// result as the sequential BatchInvertExt.
func TestParBatchInvertExt(t *testing.T) {
	a := randomExts(testN)
	want := BatchInvertExt(a)
	got := ParBatchInvertExt(a, 4)
	checkExtSlice(t, want, got)
}

// TestMulRInvExt verifies that MulRInvExt is the inverse of multiplication by
// MontConstant: MulByElement(MulRInvExt(x), MontConstant) == x.
func TestMulRInvExt(t *testing.T) {
	rng := newRng()
	for range testN {
		x := PseudoRandExt(rng)
		r := MulRInvExt(x)
		var got Ext
		got.MulByElement(&r, &MontConstant)
		checkExt(t, x, got)
	}
}

// TestZeroExtFunc verifies that ZeroExt() returns an all-zero extension element.
func TestZeroExtFunc(t *testing.T) {
	z := ZeroExt()
	if !z.IsZero() {
		t.Error("ZeroExt: should be zero")
	}
}

// TestToUint64s verifies consistency of ToUint64s: zero produces all-zeros and
// two copies of the same Ext produce identical tuples.
func TestToUint64s(t *testing.T) {
	z := ZeroExt()
	u0, u1, u2, u3, u4, u5 := ExtToUint64s(&z)
	if u0 != 0 || u1 != 0 || u2 != 0 || u3 != 0 || u4 != 0 || u5 != 0 {
		t.Errorf("ToUint64s(ZeroExt) = (%d,%d,%d,%d,%d,%d), want all zeros", u0, u1, u2, u3, u4, u5)
	}

	rng := newRng()
	for range testN {
		a := PseudoRandExt(rng)
		b := a // value copy
		if extToUint64sTuple(&a) != extToUint64sTuple(&b) {
			t.Error("ToUint64s: equal Ext values should produce equal tuples")
		}
	}
}

// TestSetExtFromUInt verifies that SetExtFromUInt sets B0.A0 to the given
// canonical value and zeroes all remaining coordinates.
func TestSetExtFromUInt(t *testing.T) {
	var z Ext
	SetExtFromUInt(&z, 42)
	if z.B0.A0.Uint64() != 42 {
		t.Errorf("B0.A0 = %d, want 42", z.B0.A0.Uint64())
	}
	if !extUpperIsZero(z) {
		t.Error("SetExtFromUInt: extension coordinates should be zero")
	}
}

// TestSetExtFromInt verifies SetExtFromInt for a positive and a negative value.
func TestSetExtFromInt(t *testing.T) {
	var z Ext
	SetExtFromInt(&z, 100)
	if z.B0.A0.Uint64() != 100 {
		t.Errorf("B0.A0 = %d, want 100", z.B0.A0.Uint64())
	}

	var neg Ext
	SetExtFromInt(&neg, -1)
	var want Element
	want.SetInt64(-1)
	checkElem(t, want, neg.B0.A0)
}

// TestSetExtFromBase verifies that SetExtFromBase sets only B0.A0 and zeroes
// all other coordinates.
func TestSetExtFromBase(t *testing.T) {
	rng := newRng()
	for range testN {
		e := PseudoRand(rng)
		var z Ext
		SetExtFromBase(&z, &e)
		checkElem(t, e, z.B0.A0)
		if !extUpperIsZero(z) {
			t.Error("SetExtFromBase: extension coordinates should be zero")
		}
	}
}

// TestNewExtFromUint verifies that Uint64ToExt sets B0.A0 to the given
// value and leaves the remaining coordinates at zero.
func TestNewExtFromUint(t *testing.T) {
	e := Uint64ToExt(55)
	if e.B0.A0.Uint64() != 55 {
		t.Errorf("B0.A0 = %d, want 55", e.B0.A0.Uint64())
	}
	if !extUpperIsZero(e) {
		t.Error("Uint64ToExt: extension coordinates should be zero")
	}
}

// TestExpByIntExt verifies ExpByIntExt for k=0, k=1, a positive exponent,
// and negative exponents using round-trip and repeated-multiplication checks.
func TestExpByIntExt(t *testing.T) {
	rng := newRng()
	oneExt := OneExt()

	t.Run("k=0", func(t *testing.T) {
		for range testN {
			x := PseudoRandExt(rng)
			var z Ext
			ExpByIntExt(&z, x, 0)
			checkExt(t, oneExt, z)
		}
	})

	t.Run("k=1", func(t *testing.T) {
		for range testN {
			x := PseudoRandExt(rng)
			var z Ext
			ExpByIntExt(&z, x, 1)
			checkExt(t, x, z)
		}
	})

	// Verify x^3 against x * x * x.
	t.Run("k=3", func(t *testing.T) {
		for range testN {
			x := PseudoRandExt(rng)
			var want Ext
			want.Mul(&x, &x)
			want.Mul(&want, &x)
			var z Ext
			ExpByIntExt(&z, x, 3)
			checkExt(t, want, z)
		}
	})

	// x^(-1) * x == 1.
	t.Run("k=-1", func(t *testing.T) {
		for range testN {
			x := PseudoRandExt(rng)
			var z Ext
			ExpByIntExt(&z, x, -1)
			var product Ext
			product.Mul(&z, &x)
			checkExt(t, oneExt, product)
		}
	})
}

// TestBytesToExt verifies BytesToExt for two key properties:
//  1. All-zero bytes produce the zero extension element.
//  2. BytesToExt and SetInterface([]byte) agree on their output.
//
// Note: BytesToExt expects the canonical coordinate encoding produced by
// Element.Bytes.
func TestBytesToExt(t *testing.T) {
	// All-zero bytes produce the zero element (zero is zero in any representation).
	zeroData := make([]byte, ExtensionDegree*Bytes)
	if e := BytesToExt(zeroData); !e.IsZero() {
		t.Error("BytesToExt(zeros) should be the zero extension element")
	}

	// BytesToExt and SetInterface([]byte) must agree.
	rng := newRng()
	for range testN {
		src := PseudoRandExt(rng)
		b := ExtToBytes(&src)
		fromFunc := BytesToExt(b[:])
		if !extEq(src, fromFunc) {
			t.Error("BytesToExt(ExtToBytes(src)) should round-trip to src")
		}
		var dummy Ext
		result, err := SetInterface(&dummy, b[:])
		if err != nil {
			t.Fatalf("SetInterface([]byte): %v", err)
		}
		if !extEq(fromFunc, *result) {
			t.Error("BytesToExt and SetInterface([]byte) should produce equal results")
		}
	}
}

// TestExtFromBytes verifies that ExtToBytes extracts the canonical byte
// representation of each coordinate, consistent with Element.Bytes().
func TestExtFromBytes(t *testing.T) {
	rng := newRng()
	for range testN {
		e := PseudoRandExt(rng)
		b := ExtToBytes(&e)

		coords := []Element{e.B0.A0, e.B0.A1, e.B1.A0, e.B1.A1, e.B2.A0, e.B2.A1}
		for k, c := range coords {
			cb := c.Bytes()
			for i := 0; i < Bytes; i++ {
				if b[k*Bytes+i] != cb[i] {
					t.Errorf("coord %d byte %d: got %d, want %d", k, i, b[k*Bytes+i], cb[i])
				}
			}
		}
	}
}

// TestRandomElementExt is a smoke test verifying that RandomElementExt does
// not panic and returns some value.
func TestRandomElementExt(_ *testing.T) {
	_ = RandomElementExt()
}

// extUpperIsZero reports whether every coordinate except B0.A0 is zero.
func extUpperIsZero(e Ext) bool {
	return e.B0.A1.IsZero() &&
		e.B1.A0.IsZero() && e.B1.A1.IsZero() &&
		e.B2.A0.IsZero() && e.B2.A1.IsZero()
}

// extToUint64sTuple wraps ExtToUint64s into a comparable struct for tests.
func extToUint64sTuple(e *Ext) [6]uint64 {
	a, b, c, d, f, g := ExtToUint64s(e)
	return [6]uint64{a, b, c, d, f, g}
}
