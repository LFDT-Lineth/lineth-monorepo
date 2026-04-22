package field

import (
	"bytes"
	"errors"
	"testing"
)

// failWriter is an io.Writer that always returns an error. Used to exercise
// the error-return path of WriteOctupletTo.
type failWriter struct{}

func (failWriter) Write(_ []byte) (int, error) {
	return 0, errors.New("test write error")
}

// TestMulRMulRInv verifies that MulR and MulRInv are mutual inverses:
// MulRInv(MulR(x)) == x and MulR(MulRInv(x)) == x.
func TestMulRMulRInv(t *testing.T) {
	rng := newRng()
	for range testN {
		x := PseudoRand(rng)
		checkElem(t, x, MulRInv(MulR(x)))
		checkElem(t, x, MulR(MulRInv(x)))
	}
}

// TestNewFromStringPanic verifies that NewFromString panics when the input
// cannot be parsed as a field element.
func TestNewFromStringPanic(t *testing.T) {
	checkPanics(t, func() { NewFromString("not_a_valid_number") })
}

// TestExpToInt verifies ExpToInt for k=0, k=1, a positive exponent, and
// negative exponents using round-trip identity checks.
func TestExpToInt(t *testing.T) {
	rng := newRng()

	t.Run("k=0", func(t *testing.T) {
		for range testN {
			x := PseudoRand(rng)
			var z Element
			ExpToInt(&z, x, 0)
			checkElem(t, One(), z)
		}
	})

	t.Run("k=1", func(t *testing.T) {
		for range testN {
			x := PseudoRand(rng)
			var z Element
			ExpToInt(&z, x, 1)
			checkElem(t, x, z)
		}
	})

	// Verify x^5 against x*x*x*x*x computed by repeated multiplication.
	t.Run("k=5", func(t *testing.T) {
		for range testN {
			x := PseudoRand(rng)
			var want Element
			want.Mul(&x, &x)
			want.Mul(&want, &want)
			want.Mul(&want, &x)
			var z Element
			ExpToInt(&z, x, 5)
			checkElem(t, want, z)
		}
	})

	// x^(-1) must satisfy x * x^(-1) == 1.
	t.Run("k=-1", func(t *testing.T) {
		for range testN {
			x := PseudoRand(rng)
			var z Element
			ExpToInt(&z, x, -1)
			var product Element
			product.Mul(&x, &z)
			checkElem(t, One(), product)
		}
	})

	// x^(-2) must satisfy x^2 * x^(-2) == 1.
	t.Run("k=-2", func(t *testing.T) {
		for range testN {
			x := PseudoRand(rng)
			var z Element
			ExpToInt(&z, x, -2)
			var sq, product Element
			sq.Square(&x)
			product.Mul(&sq, &z)
			checkElem(t, One(), product)
		}
	})
}

// TestPseudoRandTruncated verifies that values generated with sizeByte in
// {1,2,3} fit within the expected range and that sizeByte=5 panics.
func TestPseudoRandTruncated(t *testing.T) {
	rng := newRng()
	for _, sizeByte := range []int{1, 2, 3} {
		maxVal := uint64(1) << (sizeByte * 8)
		for range testN {
			e := PseudoRandTruncated(rng, sizeByte)
			if n := e.Uint64(); n >= maxVal {
				t.Errorf("sizeByte=%d: value %d exceeds max %d", sizeByte, n, maxVal)
			}
		}
	}
	// sizeByte=4 must not panic; result is a valid field element in [0, p-1].
	for range testN {
		_ = PseudoRandTruncated(rng, 4)
	}
	// sizeByte > 4 must panic.
	checkPanics(t, func() { PseudoRandTruncated(rng, 5) })
}

// TestFromBool verifies that FromBool maps true to One and false to Zero.
func TestFromBool(t *testing.T) {
	checkElem(t, One(), FromBool(true))
	checkElem(t, Zero(), FromBool(false))
}

// TestOctupletRoundTrip verifies that ParseOctuplet(OctupletToBytes(o)) == o.
func TestOctupletRoundTrip(t *testing.T) {
	rng := newRng()
	for range testN {
		oct := PseudoRandOctuplet(rng)
		b := OctupletToBytes(oct)
		got := ParseOctuplet(b)
		for i := range oct {
			checkElem(t, oct[i], got[i])
		}
	}
}

// TestParseOctupletPanic verifies that ParseOctuplet panics when bytes represent
// values beyond the field modulus (0xFFFFFFFF = 4294967295 > p = 2130706433).
func TestParseOctupletPanic(t *testing.T) {
	var b [32]byte
	for i := range b {
		b[i] = 0xFF
	}
	checkPanics(t, func() { ParseOctuplet(b) })
}

// TestNewOctupletFromStrings verifies construction from decimal string
// representations and that an invalid string triggers a panic.
func TestNewOctupletFromStrings(t *testing.T) {
	rng := newRng()
	oct := PseudoRandOctuplet(rng)
	var strs [8]string
	for i := range oct {
		strs[i] = oct[i].String()
	}
	got := NewOctupletFromStrings(strs)
	for i := range oct {
		checkElem(t, oct[i], got[i])
	}
	checkPanics(t, func() {
		var bad [8]string
		bad[0] = "not_a_number"
		NewOctupletFromStrings(bad)
	})
}

// TestPseudoRandOctupletDeterminism verifies that two RNGs seeded identically
// produce the same octuplet sequence.
func TestPseudoRandOctupletDeterminism(t *testing.T) {
	rng1, rng2 := newRng(), newRng()
	for range testN {
		oct1 := PseudoRandOctuplet(rng1)
		oct2 := PseudoRandOctuplet(rng2)
		for i := range oct1 {
			checkElem(t, oct1[i], oct2[i])
		}
	}
}

// TestWriteOctupletTo verifies that the bytes written to an io.Writer match
// those returned by OctupletToBytes, and that write errors are propagated.
func TestWriteOctupletTo(t *testing.T) {
	rng := newRng()

	t.Run("success", func(t *testing.T) {
		for range testN {
			oct := PseudoRandOctuplet(rng)
			want := OctupletToBytes(oct)
			var buf bytes.Buffer
			if err := WriteOctupletTo(&buf, oct); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if buf.Len() != 32 {
				t.Fatalf("wrote %d bytes, want 32", buf.Len())
			}
			var got [32]byte
			copy(got[:], buf.Bytes())
			if got != want {
				t.Error("WriteOctupletTo: content mismatch vs OctupletToBytes")
			}
		}
	})

	t.Run("writeError", func(t *testing.T) {
		oct := PseudoRandOctuplet(rng)
		if err := WriteOctupletTo(failWriter{}, oct); err == nil {
			t.Error("WriteOctupletTo: expected error from failing writer, got nil")
		}
	})
}

// TestParBatchInvert verifies that ParBatchInvert returns the same result as
// the sequential BatchInvert for the same input.
func TestParBatchInvert(t *testing.T) {
	a := randomElems(testN)
	want := BatchInvert(a)
	got := ParBatchInvert(a, 4)
	checkElemSlice(t, want, got)
}

// TestToInt verifies that ToInt returns the canonical integer value of small
// field elements.
func TestToInt(t *testing.T) {
	check := func(e Element, want int) {
		t.Helper()
		if got := ToInt(&e); got != want {
			t.Errorf("ToInt(%v) = %d, want %d", e, got, want)
		}
	}
	check(Zero(), 0)
	check(One(), 1)
	var n Element
	n.SetUint64(12345)
	check(n, 12345)
}

// TestRandomElement is a smoke test verifying that RandomElement does not panic.
func TestRandomElement(_ *testing.T) {
	_ = RandomElement()
}

// TestRandomOctuplet is a smoke test verifying that RandomOctuplet does not panic.
func TestRandomOctuplet(_ *testing.T) {
	_ = RandomOctuplet()
}
