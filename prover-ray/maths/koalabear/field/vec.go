package field

import (
	"fmt"
	"math/rand/v2"
	"strings"
)

// Vec holds a vector of field elements in either the base field 𝔽_p or
// the degree-4 extension 𝔽_{p^4}. Exactly one of base or ext is non-nil;
// this invariant is the caller's responsibility when constructing via
// [VecFromBase] or [VecFromExt].
//
// Vec is a pure view: it holds slice headers but does not own the backing
// memory. The caller controls allocation (including arena or pool strategies).
// All vector operation functions accept a pre-allocated result [Vec]
// whose field type must be consistent with the operand types.
type Vec struct {
	base []Element
	ext  []Ext
}

// VecFromBase wraps a []Element slice as a base-field [Vec].
func VecFromBase(v []Element) Vec { return Vec{base: v} }

// VecFromExt wraps a []Ext slice as an extension-field [Vec].
func VecFromExt(v []Ext) Vec { return Vec{ext: v} }

// IsBase reports whether the vector lives in the base field.
func (v Vec) IsBase() bool { return v.base != nil }

// Len returns the number of elements in the vector.
func (v Vec) Len() int {
	if v.base != nil {
		return len(v.base)
	}
	return len(v.ext)
}

// AsBase returns the underlying []Element. Panics if the vector is not base;
// check [Vec.IsBase] first.
func (v Vec) AsBase() []Element {
	if v.base == nil {
		panic("field: AsBase called on an extension FieldVec; check IsBase() first")
	}
	return v.base
}

// AsExt returns the underlying []Ext. Panics if the vector is not extension;
// check [Vec.IsBase] first.
func (v Vec) AsExt() []Ext {
	if v.ext == nil {
		panic("field: AsExt called on a base FieldVec; check IsBase() first")
	}
	return v.ext
}

// mustEqualLen panics if the three lengths are not all equal. Used to guard
// all typed vector operations against mismatched slice lengths.
func mustEqualLen(res, a, b int) {
	if res != a || a != b {
		panic(fmt.Sprintf("field: vector length mismatch: res=%d a=%d b=%d", res, a, b))
	}
}

// mustEqualLen2 is the unary variant of mustEqualLen.
func mustEqualLen2(res, a int) {
	if res != a {
		panic(fmt.Sprintf("field: vector length mismatch: res=%d a=%d", res, a))
	}
}

// ---------------------------------------------------------------------------
// Add
// ---------------------------------------------------------------------------

// VecAddBaseBase sets res[i] = a[i] + b[i] for all i, operating over the
// base field. All slices must have equal length.
func VecAddBaseBase(res, a, b []Element) {
	mustEqualLen(len(res), len(a), len(b))
	for i := range res {
		res[i].Add(&a[i], &b[i])
	}
}

// VecAddExtBase sets res[i] = a[i] + b[i] where a is an extension vector and
// b is a base vector. Only the first coordinate of each a[i] is updated; the
// remaining three are copied unchanged. Cost: 1 base addition per element.
// All slices must have equal length.
func VecAddExtBase(res []Ext, a []Ext, b []Element) {
	mustEqualLen(len(res), len(a), len(b))
	for i := range res {
		res[i] = a[i]
		res[i].B0.A0.Add(&res[i].B0.A0, &b[i])
	}
}

// VecAddBaseExt sets res[i] = a[i] + b[i] where a is a base vector and b is
// an extension vector. Delegates to [VecAddExtBase] by commutativity.
// Cost: 1 base addition per element. All slices must have equal length.
func VecAddBaseExt(res []Ext, a []Element, b []Ext) {
	VecAddExtBase(res, b, a)
}

// VecAddExtExt sets res[i] = a[i] + b[i] over the extension field.
// Cost: 4 base additions per element. All slices must have equal length.
func VecAddExtExt(res, a, b []Ext) {
	mustEqualLen(len(res), len(a), len(b))
	for i := range res {
		res[i].Add(&a[i], &b[i])
	}
}

// VecAddInto sets res[i] = a[i] + b[i] for all i, dispatching to the typed
// variant that matches the field types of the operands.
//
// res must be pre-allocated; its field type (IsBase) must be true iff both a
// and b are base. All vectors must have equal length. Panics otherwise.
func VecAddInto(res, a, b Vec) {
	switch {
	case res.IsBase():
		VecAddBaseBase(res.base, a.base, b.base)
	case !a.IsBase() && !b.IsBase():
		VecAddExtExt(res.ext, a.ext, b.ext)
	case !a.IsBase() && b.IsBase():
		VecAddExtBase(res.ext, a.ext, b.base)
	default: // a.IsBase() && !b.IsBase()
		VecAddBaseExt(res.ext, a.base, b.ext)
	}
}

// ---------------------------------------------------------------------------
// Sub
// ---------------------------------------------------------------------------

// VecSubBaseBase sets res[i] = a[i] - b[i] over the base field.
// All slices must have equal length.
func VecSubBaseBase(res, a, b []Element) {
	mustEqualLen(len(res), len(a), len(b))
	for i := range res {
		res[i].Sub(&a[i], &b[i])
	}
}

// VecSubExtBase sets res[i] = a[i] - b[i] where a is an extension vector and
// b is a base vector. Only the first coordinate of each a[i] is updated.
// Cost: 1 base subtraction per element. All slices must have equal length.
func VecSubExtBase(res []Ext, a []Ext, b []Element) {
	mustEqualLen(len(res), len(a), len(b))
	for i := range res {
		res[i] = a[i]
		res[i].B0.A0.Sub(&res[i].B0.A0, &b[i])
	}
}

// VecSubBaseExt sets res[i] = a[i] - b[i] where a is a base vector and b is
// an extension vector. Note that subtraction is not commutative, so this
// cannot simply delegate to VecSubExtBase.
// Cost: 4 base negations + 1 base addition per element.
// All slices must have equal length.
func VecSubBaseExt(res []Ext, a []Element, b []Ext) {
	mustEqualLen(len(res), len(a), len(b))
	for i := range res {
		// Compute Lift(a[i]) - b[i] without allocating:
		// negate all four components of b[i], then add a[i] to the first.
		res[i].Neg(&b[i])
		res[i].B0.A0.Add(&res[i].B0.A0, &a[i])
	}
}

// VecSubExtExt sets res[i] = a[i] - b[i] over the extension field.
// Cost: 4 base subtractions per element. All slices must have equal length.
func VecSubExtExt(res, a, b []Ext) {
	mustEqualLen(len(res), len(a), len(b))
	for i := range res {
		res[i].Sub(&a[i], &b[i])
	}
}

// VecSubInto sets res[i] = a[i] - b[i] for all i, dispatching to the typed
// variant that matches the field types of the operands.
//
// res must be pre-allocated; its field type (IsBase) must be true iff both a
// and b are base. All vectors must have equal length. Panics otherwise.
func VecSubInto(res, a, b Vec) {
	switch {
	case res.IsBase():
		VecSubBaseBase(res.base, a.base, b.base)
	case !a.IsBase() && !b.IsBase():
		VecSubExtExt(res.ext, a.ext, b.ext)
	case !a.IsBase() && b.IsBase():
		VecSubExtBase(res.ext, a.ext, b.base)
	default: // a.IsBase() && !b.IsBase()
		VecSubBaseExt(res.ext, a.base, b.ext)
	}
}

// ---------------------------------------------------------------------------
// Mul
// ---------------------------------------------------------------------------

// VecMulBaseBase sets res[i] = a[i] * b[i] over the base field.
// Cost: 1 base multiplication per element.
// All slices must have equal length.
func VecMulBaseBase(res, a, b []Element) {
	mustEqualLen(len(res), len(a), len(b))
	for i := range res {
		res[i].Mul(&a[i], &b[i])
	}
}

// VecMulExtBase sets res[i] = a[i] * b[i] where a is an extension vector and
// b is a base vector. Uses [Ext.MulByElement] which exploits the base-field
// structure of b[i].
// Cost: 4 base multiplications per element (vs ~9 for full extension mul).
// All slices must have equal length.
func VecMulExtBase(res []Ext, a []Ext, b []Element) {
	mustEqualLen(len(res), len(a), len(b))
	for i := range res {
		res[i].MulByElement(&a[i], &b[i])
	}
}

// VecMulBaseExt sets res[i] = a[i] * b[i] where a is a base vector and b is
// an extension vector. Multiplication is commutative, so this delegates to
// [VecMulExtBase].
// Cost: 4 base multiplications per element.
// All slices must have equal length.
func VecMulBaseExt(res []Ext, a []Element, b []Ext) {
	VecMulExtBase(res, b, a)
}

// VecMulExtExt sets res[i] = a[i] * b[i] over the extension field.
// Cost: ~9 base multiplications per element (Karatsuba over E2).
// All slices must have equal length.
func VecMulExtExt(res, a, b []Ext) {
	mustEqualLen(len(res), len(a), len(b))
	for i := range res {
		res[i].Mul(&a[i], &b[i])
	}
}

// VecMulInto sets res[i] = a[i] * b[i] for all i, dispatching to the typed
// variant that matches the field types of the operands.
//
// res must be pre-allocated; its field type (IsBase) must be true iff both a
// and b are base. All vectors must have equal length. Panics otherwise.
func VecMulInto(res, a, b Vec) {
	switch {
	case res.IsBase():
		VecMulBaseBase(res.base, a.base, b.base)
	case !a.IsBase() && !b.IsBase():
		VecMulExtExt(res.ext, a.ext, b.ext)
	case !a.IsBase() && b.IsBase():
		VecMulExtBase(res.ext, a.ext, b.base)
	default: // a.IsBase() && !b.IsBase()
		VecMulBaseExt(res.ext, a.base, b.ext)
	}
}

// ---------------------------------------------------------------------------
// Neg
// ---------------------------------------------------------------------------

// VecNegBase sets res[i] = -a[i] over the base field.
// All slices must have equal length.
func VecNegBase(res, a []Element) {
	mustEqualLen2(len(res), len(a))
	for i := range res {
		res[i].Neg(&a[i])
	}
}

// VecNegExt sets res[i] = -a[i] over the extension field.
// All slices must have equal length.
func VecNegExt(res, a []Ext) {
	mustEqualLen2(len(res), len(a))
	for i := range res {
		res[i].Neg(&a[i])
	}
}

// VecNegInto sets res[i] = -a[i] for all i, dispatching on the field type of
// res. res and a must have the same field type and equal length. Panics otherwise.
func VecNegInto(res, a Vec) {
	if res.IsBase() {
		VecNegBase(res.base, a.base)
	} else {
		VecNegExt(res.ext, a.ext)
	}
}

// ---------------------------------------------------------------------------
// Scale (scalar × vector)
// ---------------------------------------------------------------------------
//
// For each variant, the naming convention is VecScale{ScalarType}{VecType}
// where ScalarType and VecType are "Base" or "Ext".

// VecScaleBaseBase sets res[i] = s * a[i] where s and a are both base-field.
// Cost: 1 base multiplication per element.
// res and a must have equal length.
func VecScaleBaseBase(res []Element, s Element, a []Element) {
	mustEqualLen2(len(res), len(a))
	for i := range res {
		res[i].Mul(&s, &a[i])
	}
}

// VecScaleBaseExt sets res[i] = s * a[i] where s is a base scalar and a is an
// extension vector. Uses [Ext.MulByElement] to exploit the base structure of s.
// Cost: 4 base multiplications per element.
// res and a must have equal length.
func VecScaleBaseExt(res []Ext, s Element, a []Ext) {
	mustEqualLen2(len(res), len(a))
	for i := range res {
		res[i].MulByElement(&a[i], &s)
	}
}

// VecScaleExtBase sets res[i] = s * a[i] where s is an extension scalar and a
// is a base vector. Uses [Ext.MulByElement] to exploit the base structure of
// each a[i].
// Cost: 4 base multiplications per element.
// res and a must have equal length.
func VecScaleExtBase(res []Ext, s Ext, a []Element) {
	mustEqualLen2(len(res), len(a))
	for i := range res {
		res[i].MulByElement(&s, &a[i])
	}
}

// VecScaleExtExt sets res[i] = s * a[i] where s and a are both extension-field.
// Cost: ~9 base multiplications per element.
// res and a must have equal length.
func VecScaleExtExt(res []Ext, s Ext, a []Ext) {
	mustEqualLen2(len(res), len(a))
	for i := range res {
		res[i].Mul(&s, &a[i])
	}
}

// VecScaleInto sets res[i] = s * v[i] for all i, dispatching to the typed
// variant that matches the field types of s and v.
//
// res must be pre-allocated; its field type (IsBase) must be true iff both s
// and v are base. res and v must have equal length. Panics otherwise.
func VecScaleInto(res Vec, s Gen, v Vec) {
	switch {
	case res.IsBase():
		VecScaleBaseBase(res.base, s.B0.A0, v.base)
	case s.isBase && !v.IsBase():
		VecScaleBaseExt(res.ext, s.B0.A0, v.ext)
	case !s.isBase && v.IsBase():
		VecScaleExtBase(res.ext, s.Ext, v.base)
	default:
		VecScaleExtExt(res.ext, s.Ext, v.ext)
	}
}

// ---------------------------------------------------------------------------
// BatchInv
// ---------------------------------------------------------------------------

// VecBatchInvBase sets res[i] = 1/a[i] for all i over the base field, using
// the Montgomery batch-inversion trick (one inversion + n multiplications).
// Panics if any a[i] is zero. res and a must have equal length.
func VecBatchInvBase(res, a []Element) {
	mustEqualLen2(len(res), len(a))
	tmp := BatchInvert(a)
	copy(res, tmp)
}

// VecBatchInvExt sets res[i] = 1/a[i] for all i over the extension field,
// using the batch-inversion trick. Panics if any a[i] is zero.
// res and a must have equal length.
func VecBatchInvExt(res, a []Ext) {
	BatchInvertExtInto(a, res)
}

// VecBatchInvInto sets res[i] = 1/a[i] for all i, dispatching on the field
// type of res. res and a must have the same field type and equal length.
// Panics if any element is zero or if lengths/types are inconsistent.
func VecBatchInvInto(res, a Vec) {
	if res.IsBase() {
		VecBatchInvBase(res.base, a.base)
	} else {
		VecBatchInvExt(res.ext, a.ext)
	}
}

// ---------------------------------------------------------------------------
// Prettify
// ---------------------------------------------------------------------------

// VecPrettifyBase returns a human-readable string representation of a base-field
// vector, e.g. "[1, 2, 3]".
func VecPrettifyBase(a []Element) string {
	var b strings.Builder
	b.WriteByte('[')
	for i := range a {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(a[i].String())
	}
	b.WriteByte(']')
	return b.String()
}

// VecPrettifyExt returns a human-readable string representation of an
// extension-field vector.
func VecPrettifyExt(a []Ext) string {
	var b strings.Builder
	b.WriteByte('[')
	for i := range a {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(a[i].String())
	}
	b.WriteByte(']')
	return b.String()
}

// ---------------------------------------------------------------------------
// Construction
// ---------------------------------------------------------------------------

// VecFromInts allocates a []Element from a list of integer literals.
// Each integer is converted via SetInt64. Useful in tests.
func VecFromInts(xs ...int) []Element {
	res := make([]Element, len(xs))
	for i, x := range xs {
		res[i].SetInt64(int64(x))
	}
	return res
}

// VecRepeatBase allocates a []Element of length n where every entry equals x.
func VecRepeatBase(x Element, n int) []Element {
	res := make([]Element, n)
	for i := range res {
		res[i] = x
	}
	return res
}

// VecRepeatExt allocates a []Ext of length n where every entry equals x.
func VecRepeatExt(x Ext, n int) []Ext {
	res := make([]Ext, n)
	for i := range res {
		res[i] = x
	}
	return res
}

// VecRandomBase allocates a []Element of length size filled with
// cryptographically random base-field elements.
func VecRandomBase(size int) []Element {
	res := make([]Element, size)
	for i := range res {
		res[i] = RandomElement()
	}
	return res
}

// VecRandomExt allocates a []Ext of length size filled with
// cryptographically random extension-field elements.
func VecRandomExt(size int) []Ext {
	res := make([]Ext, size)
	for i := range res {
		res[i] = RandomElementExt()
	}
	return res
}

// VecPseudoRandBase allocates a []Element of length size filled with
// pseudo-random base-field elements drawn from rng.
func VecPseudoRandBase(rng *rand.Rand, size int) []Element {
	res := make([]Element, size)
	for i := range res {
		res[i] = PseudoRand(rng)
	}
	return res
}

// VecPseudoRandExt allocates a []Ext of length size filled with
// pseudo-random extension-field elements drawn from rng.
func VecPseudoRandExt(rng *rand.Rand, size int) []Ext {
	res := make([]Ext, size)
	for i := range res {
		res[i] = PseudoRandExt(rng)
	}
	return res
}

// VecPowerBase allocates and returns [x^0, x^1, ..., x^{n-1}].
// Panics if x is zero or n is negative.
func VecPowerBase(x Element, n int) []Element {
	var zero Element
	if x == zero {
		panic("field: VecPowerBase called with x=0")
	}
	if n == 0 {
		return []Element{}
	}
	res := make([]Element, n)
	res[0].SetOne()
	for i := 1; i < n; i++ {
		res[i].Mul(&res[i-1], &x)
	}
	return res
}

// ---------------------------------------------------------------------------
// In-place mutation
// ---------------------------------------------------------------------------

// VecFillBase sets every element of v to val in-place.
func VecFillBase(v []Element, val Element) {
	for i := range v {
		v[i] = val
	}
}

// VecFillExt sets every element of v to val in-place.
func VecFillExt(v []Ext, val Ext) {
	for i := range v {
		v[i] = val
	}
}

// VecReverseBase reverses v in-place.
func VecReverseBase(v []Element) {
	for i, j := 0, len(v)-1; i < j; i, j = i+1, j-1 {
		v[i], v[j] = v[j], v[i]
	}
}

// VecReverseExt reverses v in-place.
func VecReverseExt(v []Ext) {
	for i, j := 0, len(v)-1; i < j; i, j = i+1, j-1 {
		v[i], v[j] = v[j], v[i]
	}
}

// ---------------------------------------------------------------------------
// Comparison
// ---------------------------------------------------------------------------

// VecEqualBase reports whether a and b contain the same values.
// Panics if a and b have different lengths.
func VecEqualBase(a, b []Element) bool {
	mustEqualLen2(len(a), len(b))
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// VecEqualExt reports whether a and b contain the same values.
// Panics if a and b have different lengths.
func VecEqualExt(a, b []Ext) bool {
	mustEqualLen2(len(a), len(b))
	for i := range a {
		if !a[i].Equal(&b[i]) {
			return false
		}
	}
	return true
}
