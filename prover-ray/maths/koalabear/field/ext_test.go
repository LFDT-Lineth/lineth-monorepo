package field

import (
	"math/big"
	"testing"
)

// TestNewExtFromUints verifies that NewExtFromUints correctly sets all four
// extension coordinates to the given canonical values.
func TestNewExtFromUints(t *testing.T) {
	e := UintsToExt(10, 20, 30, 40)
	if got := e.B0.A0.Uint64(); got != 10 {
		t.Errorf("B0.A0 = %d, want 10", got)
	}
	if got := e.B0.A1.Uint64(); got != 20 {
		t.Errorf("B0.A1 = %d, want 20", got)
	}
	if got := e.B1.A0.Uint64(); got != 30 {
		t.Errorf("B1.A0 = %d, want 30", got)
	}
	if got := e.B1.A1.Uint64(); got != 40 {
		t.Errorf("B1.A1 = %d, want 40", got)
	}
}

// TestNewExtFromInt verifies positive and negative int64 inputs.
// Negative values are reduced mod p, consistent with Element.SetInt64.
func TestNewExtFromInt(t *testing.T) {
	e := IntsToExt(1, 2, 3, 4)
	for i, got := range []uint64{e.B0.A0.Uint64(), e.B0.A1.Uint64(), e.B1.A0.Uint64(), e.B1.A1.Uint64()} {
		if got != uint64(i+1) {
			t.Errorf("coordinate %d = %d, want %d", i, got, i+1)
		}
	}
	// Negative: SetInt64(-1) gives p-1.
	e2 := IntsToExt(-1, 0, 0, 0)
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
	if !e.B0.A1.IsZero() || !e.B1.A0.IsZero() || !e.B1.A1.IsZero() {
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
		if e.B0.A1.IsZero() && e.B1.A0.IsZero() && e.B1.A1.IsZero() {
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
		if e.B0.A1.IsZero() && e.B1.A0.IsZero() && e.B1.A1.IsZero() {
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

	t.Run("[]byte", func(t *testing.T) {
		// The []byte case in SetInterface returns a new *Ext built via
		// BytesToExt rather than modifying the receiver. This is an
		// inconsistency with every other type case (all of which modify the
		// receiver z), but it is the current behaviour, so the test checks
		// the returned pointer.
		data := []byte{0, 0, 0, 5, 0, 0, 0, 6, 0, 0, 0, 7, 0, 0, 0, 8}
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

// TestToUint64s verifies consistency of ToUint64s: zero produces (0,0,0,0) and
// two copies of the same Ext produce identical tuples.
func TestToUint64s(t *testing.T) {
	z := ZeroExt()
	u1, u2, u3, u4 := ExtToUint64s(&z)
	if u1 != 0 || u2 != 0 || u3 != 0 || u4 != 0 {
		t.Errorf("ToUint64s(ZeroExt) = (%d,%d,%d,%d), want (0,0,0,0)", u1, u2, u3, u4)
	}

	rng := newRng()
	for range testN {
		a := PseudoRandExt(rng)
		b := a // value copy
		a1, a2, a3, a4 := ExtToUint64s(&a)
		b1, b2, b3, b4 := ExtToUint64s(&b)
		if a1 != b1 || a2 != b2 || a3 != b3 || a4 != b4 {
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
	if !z.B0.A1.IsZero() || !z.B1.A0.IsZero() || !z.B1.A1.IsZero() {
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
		if !z.B0.A1.IsZero() || !z.B1.A0.IsZero() || !z.B1.A1.IsZero() {
			t.Error("SetExtFromBase: extension coordinates should be zero")
		}
	}
}

// TestNewExtFromUint verifies that NewExtFromUint sets B0.A0 to the given
// value and leaves the remaining coordinates at zero.
func TestNewExtFromUint(t *testing.T) {
	e := Uint64ToExt(55)
	if e.B0.A0.Uint64() != 55 {
		t.Errorf("B0.A0 = %d, want 55", e.B0.A0.Uint64())
	}
	if !e.B0.A1.IsZero() || !e.B1.A0.IsZero() || !e.B1.A1.IsZero() {
		t.Error("NewExtFromUint: extension coordinates should be zero")
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
// Note: BytesToExt stores the input uint32 values as raw Montgomery-form
// internal representations. ToUint64s uses Bits()[0] which converts *from*
// Montgomery form (multiplying by R⁻¹ = MontConstantInv), so
// ToUint64s(BytesToExt(data)) ≠ the raw numeric bytes — this is expected.
func TestBytesToExt(t *testing.T) {
	// All-zero bytes produce the zero element (zero is zero in any representation).
	zeroData := make([]byte, 16)
	if e := BytesToExt(zeroData); !e.IsZero() {
		t.Error("BytesToExt(zeros) should be the zero extension element")
	}

	// BytesToExt and SetInterface([]byte) must agree.
	rng := newRng()
	for range testN {
		src := PseudoRandExt(rng)
		b := ExtToBytes(&src)
		fromFunc := BytesToExt(b[:])
		var dummy Ext
		result, err := SetInterface(&dummy, b[:])
		if err != nil {
			t.Fatalf("SetInterface([]byte): %v", err)
		}
		// SetInterface for []byte returns a new *Ext (see SetInterface/[]byte test).
		if !extEq(fromFunc, *result) {
			t.Error("BytesToExt and SetInterface([]byte) should produce equal results")
		}
	}
}

// TestExtFromBytes verifies that ExtFromBytes extracts the canonical byte
// representation of each coordinate, consistent with Element.Bytes().
func TestExtFromBytes(t *testing.T) {
	rng := newRng()
	for range testN {
		e := PseudoRandExt(rng)
		b := ExtToBytes(&e)

		b00 := e.B0.A0.Bytes()
		b01 := e.B0.A1.Bytes()
		b10 := e.B1.A0.Bytes()
		b11 := e.B1.A1.Bytes()

		for i := 0; i < Bytes; i++ {
			if b[i] != b00[i] {
				t.Errorf("B0.A0 byte %d: got %d, want %d", i, b[i], b00[i])
			}
			if b[Bytes+i] != b01[i] {
				t.Errorf("B0.A1 byte %d: got %d, want %d", i, b[Bytes+i], b01[i])
			}
			if b[2*Bytes+i] != b10[i] {
				t.Errorf("B1.A0 byte %d: got %d, want %d", i, b[2*Bytes+i], b10[i])
			}
			if b[3*Bytes+i] != b11[i] {
				t.Errorf("B1.A1 byte %d: got %d, want %d", i, b[3*Bytes+i], b11[i])
			}
		}
	}
}

// TestRandomElementExt is a smoke test verifying that RandomElementExt does
// not panic and returns some value.
func TestRandomElementExt(_ *testing.T) {
	_ = RandomElementExt()
}
