// Package field is a library for working with the koalabear field.
package field

import "math/rand/v2"

// Gen is a union type that holds either a base field element ([Element])
// or a degree-4 extension field element ([Ext]). The embedded [Ext] is the
// canonical storage in both cases; [Gen.IsBase] tracks whether the value
// was constructed from a base element and has remained in the base field
// through subsequent operations.
//
// isBase is an advisory tag propagated conservatively: a binary operation
// produces a base result only when both operands are base; unary operations
// preserve it. The tag does NOT scan extension coordinates, so a value
// constructed via [ElemFromExt] will have IsBase() == false even if all its
// extension coordinates happen to be zero.
//
// Gen is intended for off-hot-path code where ergonomics matter more
// than raw throughput. For performance-critical inner loops, work with
// [Element] and [Ext] directly.
//
// All arithmetic methods use value semantics: they return a new Gen and
// leave the receiver unchanged.
type Gen struct {
	Ext         // canonical storage; always valid
	isBase bool // true iff the value lives in the base field
}

// ElemFromBase constructs a [Gen] that wraps a base field element.
// [Gen.IsBase] returns true on the result.
func ElemFromBase(e Element) Gen {
	return Gen{Ext: Lift(e), isBase: true}
}

// ElemFromExt constructs a [Gen] that wraps an extension field element.
// [Gen.IsBase] returns false on the result, even if the extension
// element has zero extension coordinates.
func ElemFromExt(e Ext) Gen {
	return Gen{Ext: e}
}

// ElemZero returns the zero [Gen], tagged as base.
func ElemZero() Gen { return ElemFromBase(Zero()) }

// ElemOne returns the multiplicative identity [Gen], tagged as base.
func ElemOne() Gen { return ElemFromBase(One()) }

// IsBase reports whether the element was constructed from a base field element
// and has remained in the base field through all subsequent operations.
func (e Gen) IsBase() bool { return e.isBase }

// AsBase returns the base field component. Panics if [Gen.IsBase] is
// false, as the extension coordinates would be silently discarded.
func (e Gen) AsBase() Element {
	if !e.isBase {
		panic("field: AsBase called on a non-base FieldElem; check IsBase() first")
	}
	return e.B0.A0
}

// AsExt returns the underlying [Ext] value. Always valid regardless of IsBase.
func (e Gen) AsExt() Ext { return e.Ext }

// Add returns e + b. The result is tagged base iff both operands are base.
func (e Gen) Add(b Gen) Gen {
	var res Ext
	res.Add(&e.Ext, &b.Ext)
	return Gen{Ext: res, isBase: e.isBase && b.isBase}
}

// Sub returns e - b. The result is tagged base iff both operands are base.
func (e Gen) Sub(b Gen) Gen {
	var res Ext
	res.Sub(&e.Ext, &b.Ext)
	return Gen{Ext: res, isBase: e.isBase && b.isBase}
}

// Mul returns e * b. When both operands are base, it uses the cheaper base
// field multiplication (1 mul vs ~9). When exactly one is base, it uses
// [Ext.MulByElement] (4 muls vs ~9).
// The result is tagged base iff both operands are base.
func (e Gen) Mul(b Gen) Gen {
	if e.isBase && b.isBase {
		var res Element
		res.Mul(&e.B0.A0, &b.B0.A0)
		return ElemFromBase(res)
	}
	var res Ext
	switch {
	case e.isBase:
		res.MulByElement(&b.Ext, &e.B0.A0)
	case b.isBase:
		res.MulByElement(&e.Ext, &b.B0.A0)
	default:
		res.Mul(&e.Ext, &b.Ext)
	}
	return Gen{Ext: res}
}

// Neg returns -e. The result preserves the base tag.
func (e Gen) Neg() Gen {
	var res Ext
	res.Neg(&e.Ext)
	return Gen{Ext: res, isBase: e.isBase}
}

// Square returns e * e. When e is base it uses the cheaper base-field squaring.
// The result preserves the base tag.
func (e Gen) Square() Gen {
	if e.isBase {
		var res Element
		res.Square(&e.B0.A0)
		return ElemFromBase(res)
	}
	var res Ext
	res.Square(&e.Ext)
	return Gen{Ext: res}
}

// Inverse returns 1/e. When e is base it uses the cheaper base-field inverse.
// The result preserves the base tag. Panics if e is zero.
func (e Gen) Inverse() Gen {
	if e.isBase {
		var res Element
		res.Inverse(&e.B0.A0)
		return ElemFromBase(res)
	}
	var res Ext
	res.Inverse(&e.Ext)
	return Gen{Ext: res}
}

// Div returns e / b. Implemented as e * b.Inverse().
// The result is tagged base iff both operands are base. Panics if b is zero.
func (e Gen) Div(b Gen) Gen {
	return e.Mul(b.Inverse())
}

// RandomElemBase returns a cryptographically random base-field [Gen].
func RandomElemBase() Gen { return ElemFromBase(RandomElement()) }

// RandomElemExt returns a cryptographically random extension-field [Gen].
func RandomElemExt() Gen { return ElemFromExt(RandomElementExt()) }

// PseudoRandElemBase returns a pseudo-random base-field [Gen] drawn from rng.
func PseudoRandElemBase(rng *rand.Rand) Gen { return ElemFromBase(PseudoRand(rng)) }

// PseudoRandElemExt returns a pseudo-random extension-field [Gen] drawn from rng.
func PseudoRandElemExt(rng *rand.Rand) Gen { return ElemFromExt(PseudoRandExt(rng)) }
