package fext

import (
	"encoding/binary"
	"math/bits"

	"github.com/consensys/gnark-crypto/field/koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/field"
)

// One returns 1
func One() Element {
	var res Element
	res.B0.A0.SetOne()
	return res
}

// SetZero sets an E4 elmt to zero
func Zero() Element {
	var res Element
	return res
}

func Uint64(z *Element) (uint64, uint64, uint64, uint64) {
	return uint64(z.B0.A0.Bits()[0]), uint64(z.B0.A1.Bits()[0]), uint64(z.B1.A0.Bits()[0]), uint64(z.B1.A1.Bits()[0])
}

// SetInt64 sets z to v and returns z
// ./common/smartvectorsext/fuzzing.go:195:		coeffField.SetInt64(int64(tcase.coeffs[i]))
// ./common/smartvectorsext/arithmetic_op.go:95:		c.SetInt64(int64(coeff))
// ./common/smartvectorsext/arithmetic_op.go:119:		c.SetInt64(int64(coeff))
// ./common/smartvectorsext/arithmetic_op.go:149:		c.SetInt64(int64(coeff))
// ./common/smartvectorsext/arithmetic_op.go:168:		c.SetInt64(int64(coeff))
// ./common/smartvectors/fft_test.go:254:				xCoeff.SetInt64(2)
// ./common/smartvectors/fuzzing.go:193:		coeffField.SetInt64(int64(tcase.coeffs[i]))
// ./common/smartvectors/arithmetic_op.go:96:		c.SetInt64(int64(coeff))
// ./common/smartvectors/arithmetic_op.go:120:		c.SetInt64(int64(coeff))
// ./common/smartvectors/arithmetic_op.go:150:		c.SetInt64(int64(coeff))
// ./common/smartvectors/arithmetic_op.go:169:		c.SetInt64(int64(coeff))
// ./common/vector/vector_wizard.go:102:		res[i].SetInt64(int64(x))
// SetFromUIntBase sets z to v and returns z
func SetFromUIntBase(z *Element, v uint64) *Element {
	//  sets z LSB to v (non-Montgomery form) and convert z to Montgomery form
	z.B0.A0.SetUint64(v)
	return z // z.toMont()
}

func SetFromIntBase(z *Element, v int64) *Element {
	z.B0.A0.SetInt64(v)
	return z // z.toMont()
}

func SetFromBase(z *Element, x *field.Element) *Element {
	z.B0.A0[0] = x[0]
	z.B0.A1[0] = 0
	z.B1.A0[0] = 0
	z.B1.A1[0] = 0
	return z
}

func NewFromUintBase(b uint64) Element {
	var res Element
	res.B0.A0.SetUint64(b)
	return res
}

func Lift(v field.Element) Element {
	var res Element
	res.B0.A0.Set(&v)
	return res
}

// PseudoRand generates a field using a pseudo-random number generator
func RandomElement() Element {
	var res Element
	res.SetRandom()
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

// Bytes returns the value of z as a big-endian byte array
// TODO: check if this way is correct
// the output is 16 bytes, not Bytes32
func Bytes(z *Element) (res [field.Bytes * 4]byte) {
	var result [field.Bytes * 4]byte

	valBytes := z.B0.A0.Bytes()
	copy(result[0:field.Bytes], valBytes[:])

	valBytes = z.B0.A1.Bytes()
	copy(result[field.Bytes:2*field.Bytes], valBytes[:])

	valBytes = z.B1.A0.Bytes()
	copy(result[2*field.Bytes:3*field.Bytes], valBytes[:])

	valBytes = z.B1.A1.Bytes()
	copy(result[3*field.Bytes:4*field.Bytes], valBytes[:])

	return result
}

func SetBytes(data []byte) Element {
	var res Element
	res.B0.A0 = koalabear.Element{binary.BigEndian.Uint32(data[0:4])}
	res.B0.A1 = koalabear.Element{binary.BigEndian.Uint32(data[4:8])}
	res.B1.A0 = koalabear.Element{binary.BigEndian.Uint32(data[8:12])}
	res.B1.A1 = koalabear.Element{binary.BigEndian.Uint32(data[12:16])}
	return res
}
