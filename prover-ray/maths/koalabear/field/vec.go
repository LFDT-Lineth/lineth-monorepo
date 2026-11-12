package field

import (
	"fmt"
	"math/rand/v2"
	"unsafe"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/parallel"
	"github.com/consensys/gnark-crypto/field/koalabear"
	"github.com/consensys/gnark-crypto/field/koalabear/extensions"
)

// minusOne and oneExt are the scalars of the vectorised negations and
// base-by-extension accumulations below.
var (
	minusOne = func() (m Element) { m.SetOne(); m.Neg(&m); return m }()
	oneExt   = func() (o Ext) { o.SetOne(); return o }()
)

// Vec holds a vector of field elements in either the base field 𝔽_p or
// the degree-6 extension 𝔽_{p^6}. Exactly one of base or ext is non-nil;
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

// ExtLimbs views extension elements as their 6·len(v) contiguous base-field
// coordinates (an Ext is six Elements laid out back to back). Coordinate-wise
// operations — Add, Sub, scaling by a base scalar — on the view act on the
// extension elements, so they can run through the koalabear.Vector kernels.
// The view aliases v.
func ExtLimbs(v []Ext) Vector {
	if len(v) == 0 {
		return nil
	}
	// #nosec G103 -- an Ext is six contiguous Elements
	return unsafe.Slice((*Element)(unsafe.Pointer(unsafe.SliceData(v))), ExtensionDegree*len(v))
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
	(*koalabear.Vector)(&res).Add(a, b)
}

// VecAddExtBase sets res[i] = a[i] + b[i] where a is an extension vector and
// b is a base vector. Only the first coordinate of each a[i] is updated; the
// remaining five are copied unchanged. Cost: 1 base addition per element.
// All slices must have equal length.
//
// It stays a loop: no gnark-crypto kernel adds a base vector into the first
// coordinates only, and copying a then accumulating b with
// VectorE6.ScalarMulAccByElement measured ~4x slower at 2^16.
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
// Cost: 6 base additions per element, run as one koalabear.Vector.Add over
// the limb views (see [ExtLimbs]). All slices must have equal length; res
// may alias a or b.
func VecAddExtExt(res, a, b []Ext) {
	mustEqualLen(len(res), len(a), len(b))
	limbs := ExtLimbs(res)
	limbs.Add(ExtLimbs(a), ExtLimbs(b))
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
	(*koalabear.Vector)(&res).Sub(a, b)
}

// VecSubExtBase sets res[i] = a[i] - b[i] where a is an extension vector and
// b is a base vector. Only the first coordinate of each a[i] is updated.
// Cost: 1 base subtraction per element. All slices must have equal length.
// It stays a loop for the reason given on [VecAddExtBase].
func VecSubExtBase(res []Ext, a []Ext, b []Element) {
	mustEqualLen(len(res), len(a), len(b))
	for i := range res {
		res[i] = a[i]
		res[i].B0.A0.Sub(&res[i].B0.A0, &b[i])
	}
}

// VecSubBaseExt sets res[i] = a[i] - b[i] where a is a base vector and b is
// an extension vector. Subtraction is not commutative, so this cannot
// delegate to VecSubExtBase: it negates b through its limb view and adds a
// into the first coordinates with the vectorised base-by-extension
// accumulation. All slices must have equal length.
func VecSubBaseExt(res []Ext, a []Element, b []Ext) {
	mustEqualLen(len(res), len(a), len(b))
	limbs := ExtLimbs(res)
	limbs.ScalarMul(ExtLimbs(b), &minusOne)
	extensions.VectorE6(res).ScalarMulAccByElement(a, &oneExt)
}

// VecSubExtExt sets res[i] = a[i] - b[i] over the extension field.
// Cost: 6 base subtractions per element, run as one koalabear.Vector.Sub over
// the limb views (see [ExtLimbs]). All slices must have equal length; res
// may alias a or b.
func VecSubExtExt(res, a, b []Ext) {
	mustEqualLen(len(res), len(a), len(b))
	limbs := ExtLimbs(res)
	limbs.Sub(ExtLimbs(a), ExtLimbs(b))
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
	(*koalabear.Vector)(&res).Mul(a, b)
}

// VecMulExtBase sets res[i] = a[i] * b[i] where a is an extension vector and
// b is a base vector. Uses [Ext.MulByElement] which exploits the base-field
// structure of b[i].
// Cost: 6 base multiplications per element (vs ~24 for full extension mul).
// All slices must have equal length.
func VecMulExtBase(res []Ext, a []Ext, b []Element) {
	mustEqualLen(len(res), len(a), len(b))
	extensions.VectorE6(res).MulByElement(a, b)
}

// VecMulBaseExt sets res[i] = a[i] * b[i] where a is a base vector and b is
// an extension vector. Multiplication is commutative, so this delegates to
// [VecMulExtBase].
// Cost: 6 base multiplications per element.
// All slices must have equal length.
func VecMulBaseExt(res []Ext, a []Element, b []Ext) {
	VecMulExtBase(res, b, a)
}

// VecMulExtExt sets res[i] = a[i] * b[i] over the extension field, using
// gnark-crypto's vectorised E6 product. res may alias a or b. All slices must
// have equal length.
func VecMulExtExt(res, a, b []Ext) {
	mustEqualLen(len(res), len(a), len(b))
	extensions.VectorE6(res).Mul(a, b)
}

// VecInnerProdExtExt returns Σᵢ a[i]·b[i] over the extension field, using
// gnark-crypto's vectorised E6 inner product. a and b must have equal length.
func VecInnerProdExtExt(a, b []Ext) Ext {
	mustEqualLen2(len(a), len(b))
	return extensions.VectorE6(a).InnerProduct(b)
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
	(*koalabear.Vector)(&res).ScalarMul(a, &minusOne)
}

// VecNegExt sets res[i] = -a[i] over the extension field.
// All slices must have equal length.
func VecNegExt(res, a []Ext) {
	mustEqualLen2(len(res), len(a))
	limbs := ExtLimbs(res)
	limbs.ScalarMul(ExtLimbs(a), &minusOne)
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

// VecScaleExtExt sets res[i] = s * a[i] where s and a are both extension-field.
// Cost: ~24 base multiplications per element; AVX-512 vectorized (gnark-crypto
// VectorE6.ScalarMul).
// res and a must have equal length.
func VecScaleExtExt(res []Ext, s Ext, a []Ext) {
	mustEqualLen2(len(res), len(a))
	extensions.VectorE6(res).ScalarMul(extensions.VectorE6(a), &s)
}

// ---------------------------------------------------------------------------
// ScaleAdd (Horner step: acc = acc·s + b)
// ---------------------------------------------------------------------------

// VecScaleAddExtBase sets acc[i] = acc[i]·s + b[i] where b is a base vector:
// a vectorised scaling of acc, then a vectorised base-by-extension
// accumulation of b into the first coordinates (about 4x faster than one
// fused scalar pass). acc and b must have equal length.
func VecScaleAddExtBase(acc []Ext, s Ext, b []Element) {
	mustEqualLen2(len(acc), len(b))
	extensions.VectorE6(acc).ScalarMul(acc, &s)
	extensions.VectorE6(acc).ScalarMulAccByElement(b, &oneExt)
}

// VecScaleAddExtExt sets acc[i] = acc[i]·s + b[i] over the extension field: a
// vectorised scaling of acc, then a limb-view addition of b (about 5x faster
// than one fused scalar pass). acc and b must have equal length.
func VecScaleAddExtExt(acc []Ext, s Ext, b []Ext) {
	mustEqualLen2(len(acc), len(b))
	extensions.VectorE6(acc).ScalarMul(acc, &s)
	limbs := ExtLimbs(acc)
	limbs.Add(limbs, ExtLimbs(b))
}

// ---------------------------------------------------------------------------
// BatchInv
// ---------------------------------------------------------------------------

// VecBatchInvBase sets res[i] = 1/a[i] for all i over the base field, using
// the Montgomery batch-inversion trick (one inversion + n multiplications)
// without allocating; see [BatchInvertInto]. Zeroes map to zero. res and a
// must have equal length; res may alias a.
func VecBatchInvBase(res, a []Element) {
	BatchInvertInto(res, a)
}

// VecBatchInvExt sets res[i] = 1/a[i] for all i over the extension field,
// using the batch-inversion trick. Zeroes map to zero. res and a must have
// equal length; res may alias a.
func VecBatchInvExt(res, a []Ext) {
	BatchInvertExtInto(res, a)
}

// VecBatchInvInto sets res[i] = 1/a[i] for all i, dispatching on the field
// type of res. res and a must have the same field type and equal length.
// Zeroes map to zero. Panics if lengths/types are inconsistent.
func VecBatchInvInto(res, a Vec) {
	if res.IsBase() {
		VecBatchInvBase(res.base, a.base)
	} else {
		VecBatchInvExt(res.ext, a.ext)
	}
}

// ---------------------------------------------------------------------------
// Construction
// ---------------------------------------------------------------------------

// VecRepeatBase allocates a []Element of length n where every entry equals x.
func VecRepeatBase(x Element, n int) []Element {
	res := make([]Element, n)
	for i := range res {
		res[i] = x
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

// ---------------------------------------------------------------------------
// Prefix scans
// ---------------------------------------------------------------------------

// prefixScanMinChunk is the smallest number of elements a worker of
// [VecPrefixSumExt] or [VecPrefixProductExt] scans: below it, splitting costs
// more than it saves.
const prefixScanMinChunk = 4096

// VecPrefixSumExt replaces z by its running sum, z[i] = Σ_{k≤i} z[k], on at
// most workers goroutines. Field addition is exact, so the result does not
// depend on workers.
func VecPrefixSumExt(z []Ext, workers int) { prefixScanExt(z, workers, false) }

// VecPrefixProductExt replaces z by its running product, z[i] = ∏_{k≤i} z[k],
// on at most workers goroutines. Field multiplication is exact, so the result
// does not depend on workers.
func VecPrefixProductExt(z []Ext, workers int) { prefixScanExt(z, workers, true) }

// prefixScanExt is a two-pass chunked scan: each chunk is scanned from the
// identity, then every chunk but the first is combined with the totals of the
// chunks before it.
func prefixScanExt(z []Ext, workers int, mul bool) {
	// scan replaces v[i] by acc ∘ v[0] ∘ … ∘ v[i] and returns the last one;
	// shift replaces v[i] by v[i] ∘ offset.
	scan := func(v []Ext, acc Ext) Ext {
		if mul {
			for i := range v {
				acc.Mul(&acc, &v[i])
				v[i] = acc
			}
		} else {
			for i := range v {
				acc.Add(&acc, &v[i])
				v[i] = acc
			}
		}
		return acc
	}
	shift := func(v []Ext, offset *Ext) {
		if mul {
			for i := range v {
				v[i].Mul(&v[i], offset)
			}
		} else {
			for i := range v {
				v[i].Add(&v[i], offset)
			}
		}
	}
	var identity Ext
	if mul {
		identity.SetOne()
	}

	n := len(z)
	size := max(prefixScanMinChunk, (n+workers-1)/max(1, workers))
	nbChunks := (n + size - 1) / size
	chunk := func(c int) []Ext { return z[c*size : min((c+1)*size, n)] }
	offsets := make([]Ext, nbChunks)
	parallel.Execute(nbChunks, func(start, end int) {
		for c := start; c < end; c++ {
			offsets[c] = scan(chunk(c), identity)
		}
	}, workers)
	if nbChunks < 2 {
		return
	}
	// offsets[c] becomes the combination of the totals of chunks 0..c-1.
	scan(offsets[:nbChunks-1], identity)
	copy(offsets[1:], offsets[:nbChunks-1])
	parallel.Execute(nbChunks-1, func(start, end int) {
		for c := start + 1; c < end+1; c++ {
			shift(chunk(c), &offsets[c])
		}
	}, workers)
}
