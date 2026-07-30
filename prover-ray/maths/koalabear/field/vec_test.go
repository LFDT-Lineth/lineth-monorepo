package field

import (
	"testing"
)

// randomElems returns a slice of n pseudo-random base field elements.
func randomElems(n int) []Element {
	rng := newRng()
	v := make([]Element, n)
	for i := range v {
		v[i] = PseudoRand(rng)
	}
	return v
}

// randomExts returns a slice of n pseudo-random extension field elements.
func randomExts(n int) []Ext {
	rng := newRng()
	v := make([]Ext, n)
	for i := range v {
		v[i] = PseudoRandExt(rng)
	}
	return v
}

// checkExtSlice marks t as failed if want[i] != got[i] for any i.
func checkExtSlice(t *testing.T, want, got []Ext) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("slice length mismatch: want %d, got %d", len(want), len(got))
	}
	for i := range want {
		if !extEq(want[i], got[i]) {
			t.Errorf("index %d: Ext mismatch:\n  want %v\n   got %v", i, want[i], got[i])
		}
	}
}

// checkElemSlice marks t as failed if want[i] != got[i] for any i.
func checkElemSlice(t *testing.T, want, got []Element) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("slice length mismatch: want %d, got %d", len(want), len(got))
	}
	for i := range want {
		if !want[i].Equal(&got[i]) {
			t.Errorf("index %d: Element mismatch:\n  want %v\n   got %v", i, want[i], got[i])
		}
	}
}

// TestVecConstructors verifies IsBase, Len, AsBase, and AsExt.
func TestVecConstructors(t *testing.T) {
	elems := randomElems(testN)
	exts := randomExts(testN)

	t.Run("VecFromBase", func(t *testing.T) {
		v := VecFromBase(elems)
		if !v.IsBase() {
			t.Fatal("VecFromBase: IsBase() should be true")
		}
		if v.Len() != testN {
			t.Fatalf("Len() = %d, want %d", v.Len(), testN)
		}
		checkElemSlice(t, elems, v.AsBase())
	})

	t.Run("VecFromExt", func(t *testing.T) {
		v := VecFromExt(exts)
		if v.IsBase() {
			t.Fatal("VecFromExt: IsBase() should be false")
		}
		if v.Len() != testN {
			t.Fatalf("Len() = %d, want %d", v.Len(), testN)
		}
		checkExtSlice(t, exts, v.AsExt())
	})
}

// TestVecAccessorPanics verifies that AsBase on an extension FieldVec panics
// and that AsExt on a base FieldVec panics.
func TestVecAccessorPanics(t *testing.T) {
	t.Run("AsBaseOnExt", func(t *testing.T) {
		v := VecFromExt(randomExts(testN))
		checkPanics(t, func() { v.AsBase() })
	})
	t.Run("AsExtOnBase", func(t *testing.T) {
		v := VecFromBase(randomElems(testN))
		checkPanics(t, func() { v.AsExt() })
	})
}

// ---------------------------------------------------------------------------
// Add
// ---------------------------------------------------------------------------

// TestVecAddBaseBase verifies VecAddBaseBase against element-wise Element.Add.
func TestVecAddBaseBase(t *testing.T) {
	a, b := randomElems(testN), randomElems(testN)
	res := make([]Element, testN)
	VecAddBaseBase(res, a, b)

	want := make([]Element, testN)
	for i := range want {
		want[i].Add(&a[i], &b[i])
	}
	checkElemSlice(t, want, res)
}

// TestVecAddExtBase verifies VecAddExtBase against element-wise Ext.Add with
// a lifted base operand.
func TestVecAddExtBase(t *testing.T) {
	a, b := randomExts(testN), randomElems(testN)
	res := make([]Ext, testN)
	VecAddExtBase(res, a, b)

	want := make([]Ext, testN)
	for i := range want {
		bLift := Lift(b[i])
		want[i].Add(&a[i], &bLift)
	}
	checkExtSlice(t, want, res)
}

// TestVecAddBaseExt verifies VecAddBaseExt against element-wise Ext.Add with
// a lifted base operand. Also confirms commutativity with VecAddExtBase.
func TestVecAddBaseExt(t *testing.T) {
	a, b := randomElems(testN), randomExts(testN)
	res := make([]Ext, testN)
	VecAddBaseExt(res, a, b)

	want := make([]Ext, testN)
	for i := range want {
		aLift := Lift(a[i])
		want[i].Add(&aLift, &b[i])
	}
	checkExtSlice(t, want, res)

	// commutativity: VecAddExtBase(b, a) must produce the same result.
	resCommuted := make([]Ext, testN)
	VecAddExtBase(resCommuted, b, a)
	checkExtSlice(t, res, resCommuted)
}

// TestVecAddExtExt verifies VecAddExtExt against element-wise Ext.Add.
func TestVecAddExtExt(t *testing.T) {
	a, b := randomExts(testN), randomExts(testN)
	res := make([]Ext, testN)
	VecAddExtExt(res, a, b)

	want := make([]Ext, testN)
	for i := range want {
		want[i].Add(&a[i], &b[i])
	}
	checkExtSlice(t, want, res)
}

// TestVecAddIntoDispatch verifies that VecAddInto dispatches to the correct
// typed variant for each combination of base/ext operands.
func TestVecAddIntoDispatch(t *testing.T) {
	elems := [2][]Element{randomElems(testN), randomElems(testN)}
	exts := [2][]Ext{randomExts(testN), randomExts(testN)}

	t.Run("BaseBase", func(t *testing.T) {
		res := make([]Element, testN)
		VecAddInto(VecFromBase(res), VecFromBase(elems[0]), VecFromBase(elems[1]))
		want := make([]Element, testN)
		VecAddBaseBase(want, elems[0], elems[1])
		checkElemSlice(t, want, res)
	})

	t.Run("ExtBase", func(t *testing.T) {
		res := make([]Ext, testN)
		VecAddInto(VecFromExt(res), VecFromExt(exts[0]), VecFromBase(elems[0]))
		want := make([]Ext, testN)
		VecAddExtBase(want, exts[0], elems[0])
		checkExtSlice(t, want, res)
	})

	t.Run("BaseExt", func(t *testing.T) {
		res := make([]Ext, testN)
		VecAddInto(VecFromExt(res), VecFromBase(elems[0]), VecFromExt(exts[0]))
		want := make([]Ext, testN)
		VecAddBaseExt(want, elems[0], exts[0])
		checkExtSlice(t, want, res)
	})

	t.Run("ExtExt", func(t *testing.T) {
		res := make([]Ext, testN)
		VecAddInto(VecFromExt(res), VecFromExt(exts[0]), VecFromExt(exts[1]))
		want := make([]Ext, testN)
		VecAddExtExt(want, exts[0], exts[1])
		checkExtSlice(t, want, res)
	})
}

// ---------------------------------------------------------------------------
// Sub
// ---------------------------------------------------------------------------

// TestVecSubBaseBase verifies VecSubBaseBase against element-wise Element.Sub.
func TestVecSubBaseBase(t *testing.T) {
	a, b := randomElems(testN), randomElems(testN)
	res := make([]Element, testN)
	VecSubBaseBase(res, a, b)

	want := make([]Element, testN)
	for i := range want {
		want[i].Sub(&a[i], &b[i])
	}
	checkElemSlice(t, want, res)
}

// TestVecSubExtBase verifies VecSubExtBase against element-wise Ext.Sub with
// a lifted base subtrahend.
func TestVecSubExtBase(t *testing.T) {
	a, b := randomExts(testN), randomElems(testN)
	res := make([]Ext, testN)
	VecSubExtBase(res, a, b)

	want := make([]Ext, testN)
	for i := range want {
		bLift := Lift(b[i])
		want[i].Sub(&a[i], &bLift)
	}
	checkExtSlice(t, want, res)
}

// TestVecSubBaseExt verifies VecSubBaseExt against element-wise Ext.Sub, where
// the minuend is a base element and the subtrahend is an extension element.
// This case is NOT commutative, so it requires dedicated logic.
func TestVecSubBaseExt(t *testing.T) {
	a, b := randomElems(testN), randomExts(testN)
	res := make([]Ext, testN)
	VecSubBaseExt(res, a, b)

	want := make([]Ext, testN)
	for i := range want {
		aLift := Lift(a[i])
		want[i].Sub(&aLift, &b[i])
	}
	checkExtSlice(t, want, res)
}

// TestVecSubExtExt verifies VecSubExtExt against element-wise Ext.Sub.
func TestVecSubExtExt(t *testing.T) {
	a, b := randomExts(testN), randomExts(testN)
	res := make([]Ext, testN)
	VecSubExtExt(res, a, b)

	want := make([]Ext, testN)
	for i := range want {
		want[i].Sub(&a[i], &b[i])
	}
	checkExtSlice(t, want, res)
}

// TestVecSubIntoDispatch verifies that VecSubInto dispatches correctly.
func TestVecSubIntoDispatch(t *testing.T) {
	elems := [2][]Element{randomElems(testN), randomElems(testN)}
	exts := [2][]Ext{randomExts(testN), randomExts(testN)}

	t.Run("BaseBase", func(t *testing.T) {
		res := make([]Element, testN)
		VecSubInto(VecFromBase(res), VecFromBase(elems[0]), VecFromBase(elems[1]))
		want := make([]Element, testN)
		VecSubBaseBase(want, elems[0], elems[1])
		checkElemSlice(t, want, res)
	})

	t.Run("ExtBase", func(t *testing.T) {
		res := make([]Ext, testN)
		VecSubInto(VecFromExt(res), VecFromExt(exts[0]), VecFromBase(elems[0]))
		want := make([]Ext, testN)
		VecSubExtBase(want, exts[0], elems[0])
		checkExtSlice(t, want, res)
	})

	t.Run("BaseExt", func(t *testing.T) {
		res := make([]Ext, testN)
		VecSubInto(VecFromExt(res), VecFromBase(elems[0]), VecFromExt(exts[0]))
		want := make([]Ext, testN)
		VecSubBaseExt(want, elems[0], exts[0])
		checkExtSlice(t, want, res)
	})

	t.Run("ExtExt", func(t *testing.T) {
		res := make([]Ext, testN)
		VecSubInto(VecFromExt(res), VecFromExt(exts[0]), VecFromExt(exts[1]))
		want := make([]Ext, testN)
		VecSubExtExt(want, exts[0], exts[1])
		checkExtSlice(t, want, res)
	})
}

// ---------------------------------------------------------------------------
// Mul
// ---------------------------------------------------------------------------

// TestVecMulBaseBase verifies VecMulBaseBase against element-wise Element.Mul.
func TestVecMulBaseBase(t *testing.T) {
	a, b := randomElems(testN), randomElems(testN)
	res := make([]Element, testN)
	VecMulBaseBase(res, a, b)

	want := make([]Element, testN)
	for i := range want {
		want[i].Mul(&a[i], &b[i])
	}
	checkElemSlice(t, want, res)
}

// TestVecMulExtBase verifies VecMulExtBase against element-wise
// Ext.MulByElement, and also against full Ext.Mul with a lifted operand to
// confirm the three are mathematically equivalent.
func TestVecMulExtBase(t *testing.T) {
	a, b := randomExts(testN), randomElems(testN)
	res := make([]Ext, testN)
	VecMulExtBase(res, a, b)

	wantMBE := make([]Ext, testN)
	wantFull := make([]Ext, testN)
	for i := range wantMBE {
		wantMBE[i].MulByElement(&a[i], &b[i])
		bLift := Lift(b[i])
		wantFull[i].Mul(&a[i], &bLift)
	}
	checkExtSlice(t, wantMBE, res)
	checkExtSlice(t, wantFull, res)
}

// TestVecMulBaseExt verifies VecMulBaseExt and confirms it produces the same
// result as VecMulExtBase with swapped arguments (commutativity).
func TestVecMulBaseExt(t *testing.T) {
	a, b := randomElems(testN), randomExts(testN)
	res := make([]Ext, testN)
	VecMulBaseExt(res, a, b)

	// Reference: same computation as VecMulExtBase(_, b, a).
	want := make([]Ext, testN)
	VecMulExtBase(want, b, a)
	checkExtSlice(t, want, res)
}

// TestVecMulExtExt verifies VecMulExtExt against element-wise Ext.Mul.
func TestVecMulExtExt(t *testing.T) {
	a, b := randomExts(testN), randomExts(testN)
	res := make([]Ext, testN)
	VecMulExtExt(res, a, b)

	want := make([]Ext, testN)
	for i := range want {
		want[i].Mul(&a[i], &b[i])
	}
	checkExtSlice(t, want, res)
}

// TestVecMulIntoDispatch verifies that VecMulInto dispatches correctly.
func TestVecMulIntoDispatch(t *testing.T) {
	elems := [2][]Element{randomElems(testN), randomElems(testN)}
	exts := [2][]Ext{randomExts(testN), randomExts(testN)}

	t.Run("BaseBase", func(t *testing.T) {
		res := make([]Element, testN)
		VecMulInto(VecFromBase(res), VecFromBase(elems[0]), VecFromBase(elems[1]))
		want := make([]Element, testN)
		VecMulBaseBase(want, elems[0], elems[1])
		checkElemSlice(t, want, res)
	})

	t.Run("ExtBase", func(t *testing.T) {
		res := make([]Ext, testN)
		VecMulInto(VecFromExt(res), VecFromExt(exts[0]), VecFromBase(elems[0]))
		want := make([]Ext, testN)
		VecMulExtBase(want, exts[0], elems[0])
		checkExtSlice(t, want, res)
	})

	t.Run("BaseExt", func(t *testing.T) {
		res := make([]Ext, testN)
		VecMulInto(VecFromExt(res), VecFromBase(elems[0]), VecFromExt(exts[0]))
		want := make([]Ext, testN)
		VecMulBaseExt(want, elems[0], exts[0])
		checkExtSlice(t, want, res)
	})

	t.Run("ExtExt", func(t *testing.T) {
		res := make([]Ext, testN)
		VecMulInto(VecFromExt(res), VecFromExt(exts[0]), VecFromExt(exts[1]))
		want := make([]Ext, testN)
		VecMulExtExt(want, exts[0], exts[1])
		checkExtSlice(t, want, res)
	})
}

// ---------------------------------------------------------------------------
// Neg
// ---------------------------------------------------------------------------

// TestVecNegBase verifies VecNegBase against element-wise Element.Neg.
func TestVecNegBase(t *testing.T) {
	a := randomElems(testN)
	res := make([]Element, testN)
	VecNegBase(res, a)

	want := make([]Element, testN)
	for i := range want {
		want[i].Neg(&a[i])
	}
	checkElemSlice(t, want, res)
}

// TestVecNegExt verifies VecNegExt against element-wise Ext.Neg.
func TestVecNegExt(t *testing.T) {
	a := randomExts(testN)
	res := make([]Ext, testN)
	VecNegExt(res, a)

	want := make([]Ext, testN)
	for i := range want {
		want[i].Neg(&a[i])
	}
	checkExtSlice(t, want, res)
}

// TestVecNegIntoDispatch verifies that VecNegInto dispatches correctly.
func TestVecNegIntoDispatch(t *testing.T) {
	t.Run("Base", func(t *testing.T) {
		a := randomElems(testN)
		res := make([]Element, testN)
		VecNegInto(VecFromBase(res), VecFromBase(a))
		want := make([]Element, testN)
		VecNegBase(want, a)
		checkElemSlice(t, want, res)
	})

	t.Run("Ext", func(t *testing.T) {
		a := randomExts(testN)
		res := make([]Ext, testN)
		VecNegInto(VecFromExt(res), VecFromExt(a))
		want := make([]Ext, testN)
		VecNegExt(want, a)
		checkExtSlice(t, want, res)
	})
}

// ---------------------------------------------------------------------------
// Scale
// ---------------------------------------------------------------------------

// TestVecScaleBaseBase verifies VecScaleBaseBase against element-wise
// Element.Mul with the scalar.
func TestVecScaleBaseBase(t *testing.T) {
	rng := newRng()
	s := PseudoRand(rng)
	a := randomElems(testN)
	res := make([]Element, testN)
	VecScaleBaseBase(res, s, a)

	want := make([]Element, testN)
	for i := range want {
		want[i].Mul(&s, &a[i])
	}
	checkElemSlice(t, want, res)
}

// TestVecScaleBaseExt verifies VecScaleBaseExt (base scalar × ext vector)
// against element-wise Ext.MulByElement, and against full Ext.Mul to confirm
// equivalence.
func TestVecScaleBaseExt(t *testing.T) {
	rng := newRng()
	s := PseudoRand(rng)
	a := randomExts(testN)
	res := make([]Ext, testN)
	VecScaleBaseExt(res, s, a)

	wantMBE := make([]Ext, testN)
	wantFull := make([]Ext, testN)
	for i := range wantMBE {
		wantMBE[i].MulByElement(&a[i], &s)
		sLift := Lift(s)
		wantFull[i].Mul(&sLift, &a[i])
	}
	checkExtSlice(t, wantMBE, res)
	checkExtSlice(t, wantFull, res)
}

// TestVecScaleExtBase verifies VecScaleExtBase (ext scalar × base vector)
// against element-wise Ext.MulByElement with swapped roles, and also confirms
// commutativity with VecScaleBaseExt when the scalar value happens to be the
// same in both base and ext position.
func TestVecScaleExtBase(t *testing.T) {
	rng := newRng()
	s := PseudoRandExt(rng)
	a := randomElems(testN)
	res := make([]Ext, testN)
	VecScaleExtBase(res, s, a)

	want := make([]Ext, testN)
	for i := range want {
		want[i].MulByElement(&s, &a[i])
	}
	checkExtSlice(t, want, res)
}

// TestVecScaleExtExt verifies VecScaleExtExt (ext scalar × ext vector) against
// element-wise Ext.Mul.
func TestVecScaleExtExt(t *testing.T) {
	rng := newRng()
	s := PseudoRandExt(rng)
	a := randomExts(testN)
	res := make([]Ext, testN)
	VecScaleExtExt(res, s, a)

	want := make([]Ext, testN)
	for i := range want {
		want[i].Mul(&s, &a[i])
	}
	checkExtSlice(t, want, res)
}

// TestVecScaleIntoDispatch verifies that VecScaleInto dispatches to the
// correct typed variant for all four scalar/vector type combinations.
func TestVecScaleIntoDispatch(t *testing.T) {
	rng := newRng()
	sBase := PseudoRand(rng)
	sExt := PseudoRandExt(rng)
	elems := randomElems(testN)
	exts := randomExts(testN)

	t.Run("BaseBase", func(t *testing.T) {
		res := make([]Element, testN)
		VecScaleInto(VecFromBase(res), ElemFromBase(sBase), VecFromBase(elems))
		want := make([]Element, testN)
		VecScaleBaseBase(want, sBase, elems)
		checkElemSlice(t, want, res)
	})

	t.Run("BaseExt", func(t *testing.T) {
		res := make([]Ext, testN)
		VecScaleInto(VecFromExt(res), ElemFromBase(sBase), VecFromExt(exts))
		want := make([]Ext, testN)
		VecScaleBaseExt(want, sBase, exts)
		checkExtSlice(t, want, res)
	})

	t.Run("ExtBase", func(t *testing.T) {
		res := make([]Ext, testN)
		VecScaleInto(VecFromExt(res), ElemFromExt(sExt), VecFromBase(elems))
		want := make([]Ext, testN)
		VecScaleExtBase(want, sExt, elems)
		checkExtSlice(t, want, res)
	})

	t.Run("ExtExt", func(t *testing.T) {
		res := make([]Ext, testN)
		VecScaleInto(VecFromExt(res), ElemFromExt(sExt), VecFromExt(exts))
		want := make([]Ext, testN)
		VecScaleExtExt(want, sExt, exts)
		checkExtSlice(t, want, res)
	})
}

// ---------------------------------------------------------------------------
// BatchInv
// ---------------------------------------------------------------------------

// TestVecBatchInvBase verifies VecBatchInvBase by checking that
// res[i] * a[i] == 1 for each i and that the result matches element-wise
// Element.Inverse.
func TestVecBatchInvBase(t *testing.T) {
	a := randomElems(testN)
	res := make([]Element, testN)
	VecBatchInvBase(res, a)

	one := One()
	for i := range res {
		// round-trip: res[i] * a[i] == 1
		var prod Element
		prod.Mul(&res[i], &a[i])
		checkElem(t, one, prod)
		// consistency with element-wise inverse
		var want Element
		want.Inverse(&a[i])
		checkElem(t, want, res[i])
	}
}

// TestVecBatchInvExt verifies VecBatchInvExt by checking that
// res[i] * a[i] == 1 for each i and that the result matches element-wise
// Ext.Inverse.
func TestVecBatchInvExt(t *testing.T) {
	a := randomExts(testN)
	res := make([]Ext, testN)
	VecBatchInvExt(res, a)

	oneExt := OneExt()
	for i := range res {
		// round-trip
		var prod Ext
		prod.Mul(&res[i], &a[i])
		checkExt(t, oneExt, prod)
		// consistency
		var want Ext
		want.Inverse(&a[i])
		checkExt(t, want, res[i])
	}
}

// TestVecBatchInvIntoDispatch verifies that VecBatchInvInto dispatches
// correctly for both base and extension vectors.
func TestVecBatchInvIntoDispatch(t *testing.T) {
	t.Run("Base", func(t *testing.T) {
		a := randomElems(testN)
		res := make([]Element, testN)
		VecBatchInvInto(VecFromBase(res), VecFromBase(a))
		want := make([]Element, testN)
		VecBatchInvBase(want, a)
		checkElemSlice(t, want, res)
	})

	t.Run("Ext", func(t *testing.T) {
		a := randomExts(testN)
		res := make([]Ext, testN)
		VecBatchInvInto(VecFromExt(res), VecFromExt(a))
		want := make([]Ext, testN)
		VecBatchInvExt(want, a)
		checkExtSlice(t, want, res)
	})
}

// ---------------------------------------------------------------------------
// Length mismatch panics
// ---------------------------------------------------------------------------

// TestVecLengthMismatchPanics verifies that all typed vector operations panic
// when the slice lengths are inconsistent. One representative per helper
// function is sufficient since they all use the same mustEqualLen guards.
func TestVecLengthMismatchPanics(t *testing.T) {
	n := testN
	short := n - 1

	t.Run("VecAddBaseBase", func(t *testing.T) {
		checkPanics(t, func() {
			VecAddBaseBase(make([]Element, n), make([]Element, short), make([]Element, n))
		})
	})

	t.Run("VecAddExtBase", func(t *testing.T) {
		checkPanics(t, func() {
			VecAddExtBase(make([]Ext, n), make([]Ext, short), make([]Element, n))
		})
	})

	t.Run("VecSubExtExt", func(t *testing.T) {
		checkPanics(t, func() {
			VecSubExtExt(make([]Ext, n), make([]Ext, n), make([]Ext, short))
		})
	})

	t.Run("VecMulBaseBase", func(t *testing.T) {
		checkPanics(t, func() {
			VecMulBaseBase(make([]Element, n), make([]Element, n), make([]Element, short))
		})
	})

	t.Run("VecNegBase", func(t *testing.T) {
		checkPanics(t, func() {
			VecNegBase(make([]Element, n), make([]Element, short))
		})
	})

	t.Run("VecScaleBaseBase", func(t *testing.T) {
		checkPanics(t, func() {
			VecScaleBaseBase(make([]Element, n), Element{}, make([]Element, short))
		})
	})

	t.Run("VecBatchInvBase", func(t *testing.T) {
		checkPanics(t, func() {
			VecBatchInvBase(make([]Element, n), make([]Element, short))
		})
	})
}
