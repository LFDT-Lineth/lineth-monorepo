package field

import (
	"fmt"
	"io"
	"math/big"
	"math/bits"
	"math/rand/v2"
	"runtime"
	"unsafe"

	"github.com/consensys/gnark-crypto/field/koalabear"
	"github.com/consensys/linea-monorepo/prover-ray/utils"
	"github.com/consensys/linea-monorepo/prover-ray/utils/parallel"
)

// Element is the base field element type alias for the KoalaBear field.
type Element = koalabear.Element

// Vector is a slice of base field elements.
type Vector = koalabear.Vector

// Octuplet is an array of 8 base field elements.
type Octuplet = [8]Element

const (
	// MaxOrderRoot is the 2-adicity of the field
	MaxOrderRoot uint64 = 24
	// MultiplicativeGen generator of 𝔽ᵣ*
	MultiplicativeGen uint64 = 3
	// Limbs is the number of 32-bit words needed to represent an Element.
	Limbs = 1
	// Bits is the number of bits needed to represent a field element.
	Bits = koalabear.Bits
	// Bytes is the number of bytes needed to represent a field element.
	Bytes = koalabear.Bytes
)

var (
	// RootOfUnity is a primitive 2^24-th root of unity in the KoalaBear field.
	RootOfUnity = NewFromString("1791270792")
	// MontConstant is the Montgomery constant R = 2^32 mod p used for Montgomery multiplication.
	MontConstant    = NewFromString("33554430")
	MontConstantInv = NewFromString("1057030144")
	Modulus         = koalabear.Modulus
	MaxVal          = new(Element).SetUint64(Modulus().Uint64() - 1)

	Butterfly   = koalabear.Butterfly
	NewElement  = koalabear.NewElement
	BatchInvert = koalabear.BatchInvert
	One         = koalabear.One
)

// MulR multiplies by montConstant, where montConstant is the Montgommery constant
func MulR(x Element) Element {
	var res Element
	res.Mul(&x, &MontConstant)
	return res
}

// MulRInv multiplies the field element by R^-1, where R is the Montgommery constant
func MulRInv(x Element) Element {
	var res Element
	res.Mul(&x, &MontConstantInv)
	return res
}

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
		panic(err)
	}
	return res
}

// ExpToInt sets z to x**k
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

// RandomElement returns a random element
func RandomElement() Element {
	var res Element
	if _, err := res.SetRandom(); err != nil {
		panic(err)
	}
	return res
}

// PseudoRand generates a field using a pseudo-random number generator
func PseudoRand(rng *rand.Rand) Element {
	const q = 2130706433 // koalabear modulus
	var res Element
	res[0] = rng.Uint32() % q
	return res
}

// PseudoRandTruncated generates a field using a pseudo-random number generator
func PseudoRandTruncated(rng *rand.Rand, sizeByte int) Element {

	if sizeByte > 4 {
		utils.Panic("supplied a byteSize larger than 4 (%v), this must be a mistake. Please check that the supplied value is not instead a BIT-size.", sizeByte)
	}

	var (
		bigInt    = &big.Int{}
		res       = Element{}
		bareU32   = [1]uint32{rng.Uint32()}
		bareBytes = *(*[4]byte)(unsafe.Pointer(&bareU32)) // #nosec G103 -- audited
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

// ParseOctuplet parses a [32]byte into an octuplet of field element
func ParseOctuplet(data [32]byte) [8]Element {
	var res [8]Element
	for i := range res {
		if err := res[i].SetBytesCanonical(data[i*4 : i*4+4]); err != nil {
			utils.Panic("could not parse octuplet: %v, data: %v", err, data)
		}
	}
	return res
}

// NewOctupletFromStrings constructs an octuplet from 8 string representations.
// Each string is parsed according to [Element.SetString] rules.
func NewOctupletFromStrings(s [8]string) (res Octuplet) {
	for i := range res {
		_, err := res[i].SetString(s[i])
		if err != nil {
			panic(fmt.Sprintf("failed to parse element %d: %v", i, err))
		}
	}
	return res
}

// OctupletToBytes converts an octuplet of field elements to a [32]byte
func OctupletToBytes(octuplet [8]Element) [32]byte {
	var res [32]byte
	for i := range octuplet {
		x := octuplet[i].Bytes()
		copy(res[i*4:i*4+4], x[:])
	}
	return res
}

// RandomOctuplet returns a cryptographically random Octuplet.
func RandomOctuplet() Octuplet {
	var oct Octuplet
	for i := range oct {
		oct[i] = RandomElement()
	}
	return oct
}

// PseudoRandOctuplet returns a pseudo-random Octuplet drawn from rng.
func PseudoRandOctuplet(rng *rand.Rand) Octuplet {
	var oct Octuplet
	for i := range oct {
		oct[i] = PseudoRand(rng)
	}
	return oct
}

// WriteOctupletTo writes the octuplet to w as a sequence of 4-byte big-endian values.
func WriteOctupletTo(w io.Writer, octuplet Octuplet) error {
	for i := range octuplet {
		f := octuplet[i].Bytes()
		if _, err := w.Write(f[:]); err != nil {
			return fmt.Errorf("error writing field octuplet, could not write position %v: %w", i, err)
		}
	}
	return nil
}

// ParBatchInvert is as a parallel implementation of [BatchInvert]. The caller
// can supply the target number of cores to use to perform the paralellization.
// If `numCPU=0` is provided, the function defaults to using all the available
// cores exposed by the OS.
func ParBatchInvert(a []Element, numCPU int) []Element {

	if numCPU == 0 {
		numCPU = runtime.NumCPU()
	}

	res := make([]Element, len(a))

	parallel.Execute(len(a), func(start, stop int) {
		subRes := BatchInvert(a[start:stop])
		copy(res[start:stop], subRes)
	}, numCPU)

	return res
}

// ToInt returns the field element as a native int.
func ToInt(e *Element) int {
	n := e.Uint64()
	return int(n)
}

// RootOfUnityBy returns the canonical n-th root of unity. n must be a power of
// two. The returned element is such that e^N = 1. By canonical, we mean that
// the function will return consistent values for different "n":
//
// for all n: RootOfUnityBy(2*n)^2 == RootOfUnityBy(n)
func RootOfUnityBy(n int) Element {

	if n <= 0 || !utils.IsPowerOfTwo(n) {
		utils.Panic("n must be positive and be a power of 2, got %v", n)
	}

	logN := utils.Log2Ceil(n)
	if logN > int(MaxOrderRoot) {
		utils.Panic("supplied a root of unity larger than %v, this must be a mistake. The provided value was %v", MaxOrderRoot, n)
	}

	res := RootOfUnity
	for i := logN; i < int(MaxOrderRoot); i++ {
		res.Square(&res)
	}

	return res
}
