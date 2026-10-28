// Package generic provides a field-agnostic description of a prime-field
// element, so that algorithms (e.g. Poseidon2) can be written once and
// instantiated over several fields (KoalaBear, MamaBear, ...).
//
// Unlike zkc's value-based field.Element, the constraint follows gnark-crypto's
// pointer-receiver convention: z.Op(x, y) writes the result into z and returns
// z. Any gnark-crypto generated field element (koalabear.Element,
// mamabear.Element, ...) satisfies it as-is.
//
// Generic code is parameterized by the value type T and its pointer type PT:
//
//	func Foo[T any, PT generic.Element[T]](x *T) { PT(x).Square(x) }
package generic

// Element is the constraint satisfied by *T, where T is a prime-field element.
type Element[T any] interface {
	*T

	Set(x *T) *T
	SetZero() *T
	SetOne() *T
	SetUint64(v uint64) *T
	SetInt64(v int64) *T
	// SetBytes interprets e as a big-endian integer and reduces it modulo p.
	SetBytes(e []byte) *T
	// SetBytesCanonical interprets e as a big-endian canonical encoding and
	// errors if it is not of the expected size or not reduced.
	SetBytesCanonical(e []byte) error

	Add(x, y *T) *T
	Sub(x, y *T) *T
	Mul(x, y *T) *T
	Square(x *T) *T
	Double(x *T) *T
	Neg(x *T) *T
	// Inverse sets z = x⁻¹, or 0 if x = 0.
	Inverse(x *T) *T

	Equal(x *T) bool
	IsZero() bool
	IsOne() bool

	// Marshal returns the canonical big-endian encoding of z; its length is
	// the same for every element of the field (see [ByteSize]).
	Marshal() []byte
	Uint64() uint64
	String() string
}

// ByteSize returns the size in bytes of the canonical encoding of an element.
func ByteSize[T any, PT Element[T]]() int {
	var x T
	return len(PT(&x).Marshal())
}

// FromInt64 returns the field element representing v (mod p).
func FromInt64[T any, PT Element[T]](v int64) T {
	var x T
	PT(&x).SetInt64(v)
	return x
}

// Pow2Inv returns 2⁻ⁿ.
func Pow2Inv[T any, PT Element[T]](n uint) T {
	var x T
	PT(&x).SetUint64(1 << n)
	PT(&x).Inverse(&x)
	return x
}
