package field

import (
	"math/big"
	"math/rand/v2"
	"unsafe"

	"math/bits"

	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

// Element aliases [fr.Element] and represents a field element in the scalar
// field of the BLS12-377 curve. The zero value of this struct corresponds to
// the zero value of the field. However, for the rest the Elements are
// represented in Montgommery form. So neither `field.Element([0, 0, 0, 1])“ or
// `field.Element(1, 0, 0, 0)` represent valid field elements.
type Element = fr.Element
type Vector = fr.Vector

const (
	// RootOfUnityOrder is the smallest integer such that
	// 		[RootOfUnity] ** (2 ** RootOfUnityOrder) == 1
	RootOfUnityOrder uint64 = 47
	// MultiplicativeGen represents a (small) field element which does not
	// divide q - 1. It has the property that every element x of the field can
	// be generated as [MultiplicativeGen] ** n == x. Here q denotes the modulus
	// of the field.
	MultiplicativeGen uint64 = 22
	// Bits is the number of bits needed to represent a field element.
	Bits = fr.Bits
	// Bytes is the number of bytes needed to represent a field element.
	Bytes = fr.Bytes
)

var (
	// RootOfUnity is a 47-th root of unity in the field. It is the same as
	// gnark's for evaluating FFTs.
	RootOfUnity = NewFromString("8065159656716812877374967518403273466521432693661810619979959746626482506078")
	// rInv is the inverse of the Montgommery constant [R]
	rInv = NewFromString("3482466379256973933331601287759811764685972354380176549708408303012390300674")
	// r is the Montgommery constant that is used to convert between the regular
	// representation of a field element (the one that is parsable by a human)
	// and the Montgommery representation that gnark uses internally to
	// speed-up modular reductions.
	r = NewFromString("6014086494747379908336260804527802945383293308637734276299549080986809532403")

	// Modulus is the modulus of the field.
	Modulus = fr.Modulus
	// Butterfly sets
	// 		a = a + b
	// 		b = a - b
	Butterfly = fr.Butterfly
	// NewElement constructs a new field element corresponding to an integer.
	NewElement = fr.NewElement
	// BatchInvert returns a new slice of inverted field elements. It uses the
	// Montgommery batch inversion trick and should be the go to method to
	// invert a slice of field.Element. The function returns an incorrect result
	// if the input slice contains a zero.
	BatchInvert = fr.BatchInvert
	// One constructs a new field element representing 1
	One = fr.One
)

// Zero returns the zero field element
func Zero() Element {
	var res Element
	return res
}

// NewFromString constructs a new field element from a string. The rules to
// determine how the string is casted into a field elements are the one of
// [fr.Element.SetString]
func NewFromString(s string) (res Element) {
	_, err := res.SetString(s)
	if err != nil {
		utils.Panic("Invalid string encoding %v", s)
	}
	return res
}

// MulRInv multiplies the field element by R^-1, where R is the Montgommery constant
func MulRInv(x Element) Element {
	var res Element
	res.Mul(&x, &rInv)
	return res
}

// MulR multiplies by R, where R is the Montgommery constant
func MulR(x Element) Element {
	var res Element
	res.Mul(&x, &r)
	return res
}

func ExpToInt(z *Element, x Element, k int) *Element {
	if k == 0 {
		return z.SetOne()
	}

	if k < 0 {
		x.Inverse(&x)
		k = -k
	}

	z.Set(&x)

	for i := bits.Len(uint(k)) - 2; i >= 0; i-- {
		z.Square(z)
		if (k>>i)&1 == 1 {
			z.Mul(z, &x)
		}
	}

	return z
}

// PseudoRand generates a field using a pseudo-random number generator
func PseudoRand(rng *rand.Rand) Element {

	var (
		bigInt    = &big.Int{}
		res       = Element{}
		bareU64   = [4]uint64{rng.Uint64(), rng.Uint64(), rng.Uint64(), rng.Uint64()}
		bareBytes = *(*[32]byte)(unsafe.Pointer(&bareU64))
	)

	bigInt.SetBytes(bareBytes[:]).Mod(bigInt, Modulus())
	res.SetBigInt(bigInt)
	return res
}

// PseudoRandTruncated generates a field using a pseudo-random number generator
func PseudoRandTruncated(rng *rand.Rand, sizeByte int) Element {

	if sizeByte > 32 {
		utils.Panic("supplied a byteSize larger than 32 (%v), this must be a mistake. Please check that the supplied value is not instead a BIT-size.", sizeByte)
	}

	var (
		bigInt    = &big.Int{}
		res       = Element{}
		bareU64   = [4]uint64{rng.Uint64(), rng.Uint64(), rng.Uint64(), rng.Uint64()}
		bareBytes = *(*[32]byte)(unsafe.Pointer(&bareU64))
	)

	bigInt.SetBytes(bareBytes[:sizeByte]).Mod(bigInt, Modulus())
	res.SetBigInt(bigInt)
	return res
}

// FromBool returns 1 if true and zero if false
func FromBool(b bool) Element {
	if b {
		return One()
	}
	return Zero()
}

// ExpVec sets vector[i] = a[i]ᵏ for all i
func ExpVec(vector, a Vector, k int64) {
	N := len(a)
	if N != len(vector) {
		panic("vector.Exp: vectors don't have the same length")
	}
	if k == 0 {
		for i := range vector {
			vector[i].SetOne()
		}
		return
	}
	base := a
	exp := k
	if k < 0 {
		// call batch inverse: note that this allocates a new slice, so even if vector and a are
		// the same, it's fine.
		base = BatchInvert(a)
		exp = -k // if k == math.MinInt64, -k overflows, but uint64(-k) is correct
	} else if N > 0 {
		// ensure that vector and a are not the same slice; else we need to copy a into base
		v0 := &vector[0] // #nosec G602 we check that N > 0 above
		a0 := &a[0]      // #nosec G602 we check that N > 0 above
		if v0 == a0 {
			base = make(Vector, N)
			copy(base, a)
		}
	}

	copy(vector, base)

	// Use bits.Len64 to iterate only over significant bits
	for i := bits.Len64(uint64(exp)) - 2; i >= 0; i-- {
		vector.Mul(vector, vector)
		if (uint64(exp)>>uint(i))&1 != 0 {
			vector.Mul(vector, base)
		}
	}
}

// ExpInt64 z = xᵏ (mod q)
func ExpInt64(z *Element, x Element, k int64) {
	if k == 0 {
		z.SetOne()
		return
	}

	if k < 0 {
		// negative k, we invert
		// if k < 0: xᵏ (mod q) == (x⁻¹)⁻ᵏ (mod q)
		x.Inverse(&x)
		k = -k // if k == math.MinInt64, -k overflows, but uint64(-k) is correct
	}
	e := uint64(k)

	z.Set(&x)

	for i := int(bits.Len64(e)) - 2; i >= 0; i-- {
		z.Square(z)
		if (e>>i)&1 == 1 {
			z.Mul(z, &x)
		}
	}
}
