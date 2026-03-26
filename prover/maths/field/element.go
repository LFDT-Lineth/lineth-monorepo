package field

import (
	"fmt"
	"io"
	"math/big"
	"math/bits"
	"math/rand/v2"
	"unsafe"

	fr377 "github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	"github.com/consensys/gnark-crypto/field/koalabear"
	"github.com/consensys/linea-monorepo/prover/utils"
)

type Element = koalabear.Element
type Vector = koalabear.Vector
type Octuplet = [8]Element

const (
	// RmaxOrderRoot
	MaxOrderRoot uint64 = 24

	// MultiplicativeGen generator of 𝔽ᵣ*
	MultiplicativeGen uint64 = 3
	// number of 32 bits words needed to represent a Element
	Limbs = 1
	// Bits is the number of bits needed to represent a field element.
	Bits = koalabear.Bits
	// Bytes is the number of bytes needed to represent a field element.
	Bytes = koalabear.Bytes
)

var (
	RootOfUnity     = NewFromString("1791270792")
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
	res.SetRandom()
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
		bareBytes = *(*[4]byte)(unsafe.Pointer(&bareU32))
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

func RandomOctuplet() Octuplet {
	var oct Octuplet
	for i := range oct {
		oct[i] = RandomElement()
	}
	return oct
}

func PseudoRandOctuplet(rng *rand.Rand) Octuplet {
	var oct Octuplet
	for i := range oct {
		oct[i] = PseudoRand(rng)
	}
	return oct
}

func WriteOctupletTo(w io.Writer, octuplet Octuplet) error {
	for i := range octuplet {
		f := octuplet[i].Bytes()
		if _, err := w.Write(f[:]); err != nil {
			return fmt.Errorf("error writing field octuplet, could not write position %v: %w", i, err)
		}
	}
	return nil
}

func EquivalentBLS12377Fr(f Element) fr377.Element {

	var (
		x big.Int
		y fr377.Element
	)

	f.BigInt(&x)
	y.SetBigInt(&x)
	return y
}
