package field

import (
	"errors"
	"fmt"
	"math/big"
	"math/bits"
	"math/rand/v2"
	"reflect"
	"runtime"

	"github.com/consensys/gnark-crypto/field/koalabear/extensions"
	"github.com/consensys/linea-monorepo/prover-ray/utils/parallel"
)

// ExtensionDegree is the degree of the field extension over the base field.
const ExtensionDegree int = 6

// Ext is the degree-6 extension field element type alias.
//
// Layout: 𝔽_{p^6} = 𝔽_{p^2}[v] / (v^3 − (u+1)) where 𝔽_{p^2} = 𝔽_p[u] / (u^2 − 3).
// An element is stored as (B0, B1, B2) of three E2 = (A0, A1) pairs, packing
// six base-field coordinates contiguously in memory.
type Ext = extensions.E6

// RootPowers stores the irreducible polynomial coefficients used to define the
// tower. The first triple encodes v^3 = u + 1, the trailing 3 encodes u^2 = 3.
var RootPowers = []int{1, 1, 0, 3}

// BatchInvertExt compute the inverses of all elements in the provided slice
// using the Montgommery trick. Zeroes are ignored.
func BatchInvertExt(a []Ext) []Ext {
	return extensions.BatchInvertE6(a)
}

// BatchInvertExtInto computes the inverses of all elements in a and writes the result into res.
// Use with caution; avoid copies and allocations in parallel contexts.
func BatchInvertExtInto(a, res []Ext) {
	if len(a) != len(res) {
		panic("input and output slices must have the same length")
	}
	if len(a) == 0 {
		return
	}

	zeroes := make([]bool, len(a))
	var accumulator Ext
	accumulator.SetOne()

	for i := 0; i < len(a); i++ {
		if a[i].IsZero() {
			zeroes[i] = true
			continue
		}
		res[i].Set(&accumulator)
		accumulator.Mul(&accumulator, &a[i])
	}

	accumulator.Inverse(&accumulator)

	for i := len(a) - 1; i >= 0; i-- {
		if zeroes[i] {
			continue
		}
		res[i].Mul(&res[i], &accumulator)
		accumulator.Mul(&accumulator, &a[i])
	}
}

// NewExtFromString only sets the first coordinate of the field extension
func NewExtFromString(s string) (res Ext) {
	res.B0.A0 = NewFromString(s)
	return res
}

// PseudoRandExt returns a random field extension element with all six
// coordinates drawn from the same RNG.
func PseudoRandExt(rng *rand.Rand) Ext {
	var res Ext
	res.B0.A0 = PseudoRand(rng)
	res.B0.A1 = PseudoRand(rng)
	res.B1.A0 = PseudoRand(rng)
	res.B1.A1 = PseudoRand(rng)
	res.B2.A0 = PseudoRand(rng)
	res.B2.A1 = PseudoRand(rng)
	return res
}

// IsBase checks if the field extensionElement is a base element. An element
// lies in the base field iff every non-constant coordinate is zero.
func IsBase(z *Ext) bool {
	return z.B0.A1[0] == 0 &&
		z.B1.A0[0] == 0 && z.B1.A1[0] == 0 &&
		z.B2.A0[0] == 0 && z.B2.A1[0] == 0
}

// GetBase attempts to unlift a field extension element into a base field
// element and returns a boolean indicating success.
func GetBase(z *Ext) (Element, bool) {
	if IsBase(z) {
		return z.B0.A0, true
	}
	return Element{}, false
}

// AddByBase implements mixed addition of a base field element.
func AddByBase(z *Ext, first *Ext, second *Element) *Ext {
	z.Set(first)
	z.B0.A0.Add(&z.B0.A0, second)
	return z
}

// MulByBase implements element-wise multiplication by a base field scalar.
func MulByBase(z *Ext, first *Ext, second *Element) *Ext {
	z.B0.A0.Mul(&first.B0.A0, second)
	z.B0.A1.Mul(&first.B0.A1, second)
	z.B1.A0.Mul(&first.B1.A0, second)
	z.B1.A1.Mul(&first.B1.A1, second)
	z.B2.A0.Mul(&first.B2.A0, second)
	z.B2.A1.Mul(&first.B2.A1, second)
	return z
}

// DivByBase implements a division by a base field element.
func DivByBase(z *Ext, first *Ext, second *Element) *Ext {
	z.B0.A0.Div(&first.B0.A0, second)
	z.B0.A1.Div(&first.B0.A1, second)
	z.B1.A0.Div(&first.B1.A0, second)
	z.B1.A1.Div(&first.B1.A1, second)
	z.B2.A0.Div(&first.B2.A0, second)
	z.B2.A1.Div(&first.B2.A1, second)
	return z
}

// SetInterface is a generic purpose function to set a field element from an
// interface{}
func SetInterface(z *Ext, i1 interface{}) (*Ext, error) {
	if i1 == nil {
		return nil, errors.New("can't set fr.Element with <nil>")
	}

	switch c1 := i1.(type) {
	case Ext:
		return z.Set(&c1), nil
	case *Ext:
		if c1 == nil {
			return nil, errors.New("can't set fext.Element with <nil>")
		}
		return z.Set(c1), nil
	case Element:
		return SetExtFromBase(z, &c1), nil
	case *Element:
		if c1 == nil {
			return nil, errors.New("can't set fext.Element with <nil>")
		}
		return SetExtFromBase(z, c1), nil
	case uint8:
		return SetExtFromUInt(z, uint64(c1)), nil
	case uint16:
		return SetExtFromUInt(z, uint64(c1)), nil
	case uint32:
		return SetExtFromUInt(z, uint64(c1)), nil
	case uint:
		return SetExtFromUInt(z, uint64(c1)), nil
	case uint64:
		return SetExtFromUInt(z, c1), nil
	case int8:
		return SetExtFromInt(z, int64(c1)), nil
	case int16:
		return SetExtFromInt(z, int64(c1)), nil
	case int32:
		return SetExtFromInt(z, int64(c1)), nil
	case int64:
		return SetExtFromInt(z, c1), nil
	case int:
		return SetExtFromInt(z, int64(c1)), nil
	case string:
		if _, err := z.B0.A0.SetString(c1); err != nil {
			return nil, err
		}
		z.B0.A1.SetZero()
		z.B1.SetZero()
		z.B2.SetZero()
		return z, nil
	case *big.Int:
		if c1 == nil {
			return nil, errors.New("can't set fr.Element with <nil>")
		}
		z.B0.A0.SetBigInt(c1)
		z.B0.A1.SetZero()
		z.B1.SetZero()
		z.B2.SetZero()
		return z, nil
	case big.Int:
		z.B0.A0.SetBigInt(&c1)
		z.B0.A1.SetZero()
		z.B1.SetZero()
		z.B2.SetZero()
		return z, nil
	case []byte:
		z := BytesToExt(c1)
		return &z, nil
	default:
		return nil, errors.New("can't set fext.Element from type " + reflect.TypeOf(i1).String())
	}
}

// ExtToText returns a string representation of z in the given base.
func ExtToText(z *Ext, base int) string {
	if base < 2 || base > 36 {
		panic("invalid base")
	}
	if z == nil {
		return "<nil>"
	}

	return fmt.Sprintf("%s + %s*u + (%s + %s*u)*v + (%s + %s*u)*v^2",
		z.B0.A0.Text(base), z.B0.A1.Text(base),
		z.B1.A0.Text(base), z.B1.A1.Text(base),
		z.B2.A0.Text(base), z.B2.A1.Text(base))
}

// ParBatchInvertExt computes inverses of all elements in a in parallel using numCPU goroutines.
func ParBatchInvertExt(a []Ext, numCPU int) []Ext {

	if numCPU == 0 {
		numCPU = runtime.GOMAXPROCS(0)
	}

	res := make([]Ext, len(a))
	parallel.Execute(len(a), func(start, stop int) {
		BatchInvertExtInto(a[start:stop], res[start:stop])
	}, numCPU)

	return res
}

// MulRInvExt multiplies the extension field element by R^-1, where R is the Montgomery constant.
func MulRInvExt(x Ext) Ext {
	var res Ext
	res.MulByElement(&x, &MontConstantInv)
	return res
}

// OneExt returns the multiplicative identity element of the extension field.
func OneExt() Ext {
	var res Ext
	res.B0.A0.SetOne()
	return res
}

// ZeroExt returns the additive identity element of the extension field.
func ZeroExt() Ext {
	var res Ext
	return res
}

// ExtToUint64s returns the value of z as a tuple of 6 uint64.
func ExtToUint64s(z *Ext) (uint64, uint64, uint64, uint64, uint64, uint64) {
	return uint64(z.B0.A0.Bits()[0]),
		uint64(z.B0.A1.Bits()[0]),
		uint64(z.B1.A0.Bits()[0]),
		uint64(z.B1.A1.Bits()[0]),
		uint64(z.B2.A0.Bits()[0]),
		uint64(z.B2.A1.Bits()[0])
}

// SetExtFromUInt sets z to v and returns z. After conversion, z is in the base
// field and can safely be converted into an [Element].
func SetExtFromUInt(z *Ext, v uint64) *Ext {
	//  sets z LSB to v (non-Montgomery form) and convert z to Montgomery form
	z.B0.A0.SetUint64(v)
	z.B0.A1.SetZero()
	z.B1.SetZero()
	z.B2.SetZero()
	return z
}

// SetExtFromInt sets z to v and returns z. After conversion, z is in the base
// field and can safely be converted into an [Element].
func SetExtFromInt(z *Ext, v int64) *Ext {
	z.B0.A0.SetInt64(v)
	z.B0.A1.SetZero()
	z.B1.SetZero()
	z.B2.SetZero()
	return z
}

// SetExtFromBase sets z to v and returns z. After conversion, z is in the base
// field and can safely be converted into an [Element].
func SetExtFromBase(z *Ext, x *Element) *Ext {
	z.B0.A0[0] = x[0]
	z.B0.A1[0] = 0
	z.B1.A0[0] = 0
	z.B1.A1[0] = 0
	z.B2.A0[0] = 0
	z.B2.A1[0] = 0
	return z
}

// Uint64ToExt constructs a field extension from a uint64. The returned element
// is convertible into a base field element.
func Uint64ToExt(b uint64) Ext {
	var res Ext
	res.B0.A0.SetUint64(b)
	return res
}

// UintsToExt constructs a field extension from 6 uint64.
func UintsToExt(v1, v2, v3, v4, v5, v6 uint64) Ext {
	var z Ext
	z.B0.A0.SetUint64(v1)
	z.B0.A1.SetUint64(v2)
	z.B1.A0.SetUint64(v3)
	z.B1.A1.SetUint64(v4)
	z.B2.A0.SetUint64(v5)
	z.B2.A1.SetUint64(v6)
	return z
}

// IntsToExt constructs a field extension from 6 int64.
func IntsToExt(v1, v2, v3, v4, v5, v6 int64) Ext {
	var z Ext
	z.B0.A0.SetInt64(v1)
	z.B0.A1.SetInt64(v2)
	z.B1.A0.SetInt64(v3)
	z.B1.A1.SetInt64(v4)
	z.B2.A0.SetInt64(v5)
	z.B2.A1.SetInt64(v6)
	return z
}

// Lift embeds a base field element into the extension field.
func Lift(v Element) Ext {
	var res Ext
	res.B0.A0.Set(&v)
	return res
}

// RandomElementExt returns a cryptographically random extension field element.
func RandomElementExt() Ext {
	var res Ext
	if _, err := res.SetRandom(); err != nil {
		panic(err)
	}
	return res
}

// ExpByIntExt sets z = x^k and returns z.
func ExpByIntExt(z *Ext, x Ext, k int) *Ext {
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

// ExtToBytes returns the value of z as a big-endian byte array (one block per
// coordinate, totalling 6 * Bytes bytes).
func ExtToBytes(z *Ext) (res [Bytes * ExtensionDegree]byte) {
	var result [Bytes * ExtensionDegree]byte

	valBytes := z.B0.A0.Bytes()
	copy(result[0*Bytes:1*Bytes], valBytes[:])

	valBytes = z.B0.A1.Bytes()
	copy(result[1*Bytes:2*Bytes], valBytes[:])

	valBytes = z.B1.A0.Bytes()
	copy(result[2*Bytes:3*Bytes], valBytes[:])

	valBytes = z.B1.A1.Bytes()
	copy(result[3*Bytes:4*Bytes], valBytes[:])

	valBytes = z.B2.A0.Bytes()
	copy(result[4*Bytes:5*Bytes], valBytes[:])

	valBytes = z.B2.A1.Bytes()
	copy(result[5*Bytes:6*Bytes], valBytes[:])

	return result
}

// BytesToExt constructs an extension field element from a (6 * Bytes)-byte
// big-endian encoding produced by [ExtToBytes].
func BytesToExt(data []byte) Ext {
	var res Ext
	res.B0.A0.SetBytes(data[0*Bytes : 1*Bytes])
	res.B0.A1.SetBytes(data[1*Bytes : 2*Bytes])
	res.B1.A0.SetBytes(data[2*Bytes : 3*Bytes])
	res.B1.A1.SetBytes(data[3*Bytes : 4*Bytes])
	res.B2.A0.SetBytes(data[4*Bytes : 5*Bytes])
	res.B2.A1.SetBytes(data[5*Bytes : 6*Bytes])
	return res
}
