package fext

import (
	"errors"
	"fmt"
	"math/big"
	"math/rand/v2"
	"reflect"
	"runtime"

	"github.com/consensys/gnark-crypto/field/koalabear/extensions"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils/parallel"
)

const ExtensionDegree int = 4

var montConstantInv = field.NewFromString("1057030144")

// Embedding
type Element = extensions.E4

func NewFromUint(v1, v2, v3, v4 uint64) Element {
	var z Element
	z.B0.A0.SetUint64(v1)
	z.B0.A1.SetUint64(v2)
	z.B1.A0.SetUint64(v3)
	z.B1.A1.SetUint64(v4)
	return z
}

func NewFromInt(v1, v2, v3, v4 int64) Element {
	var z Element
	z.B0.A0.SetInt64(v1)
	z.B0.A1.SetInt64(v2)
	z.B1.A0.SetInt64(v3)
	z.B1.A1.SetInt64(v4)
	return z
}

// NewFromString only sets the first coordinate of the field extension
func NewFromString(s string) (res Element) {
	res.B0.A0 = field.NewFromString(s)
	return res
}

// var RootPowers = []int{1, 3}, v^2=u and u^2=3.
var RootPowers = []int{1, 3}

func BatchInvert(a []Element) []Element {
	return extensions.BatchInvertE4(a)
}

// BatchInvertInto computes the inverses of all elements in a and writes the result into res.
// use with caution, avoid copies and allocation in parallel contexts.
// TODO @gbotrel move in gnark-crypto
func BatchInvertInto(a, res []Element) {
	if len(a) != len(res) {
		// TODO @gbotrel add check that a != res
		panic("input and output slices must have the same length")
	}
	if len(a) == 0 {
		return
	}

	zeroes := make([]bool, len(a))
	var accumulator Element
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

func PseudoRand(rng *rand.Rand) Element {

	result := new(Element).SetZero()
	result.B0.A0 = field.PseudoRand(rng)
	result.B0.A1 = field.PseudoRand(rng)
	result.B1.A0 = field.PseudoRand(rng)
	result.B1.A1 = field.PseudoRand(rng)

	return *result
}

/*
IsBase checks if the field extensionElement is a base element.
An Element is considered a base element if all coordinates are 0 except for the first one.
*/
func IsBase(z *Element) bool {
	if z.B0.A1[0] == 0 && z.B1.A0[0] == 0 && z.B1.A1[0] == 0 {
		return true
	} else {
		return false
	}

}

func GetBase(z *Element) (field.Element, error) {
	if IsBase(z) {
		return z.B0.A0, nil
	} else {
		return field.Zero(), fmt.Errorf("requested a base element but the field extension is not a wrapped field element")
	}
}
func AddByBase(z *Element, first *Element, second *field.Element) *Element {
	z.Set(first)
	z.B0.A0.Add(&z.B0.A0, second)
	return z
}
func DivByBase(z *Element, first *Element, second *field.Element) *Element {
	z.B0.A0.Div(&first.B0.A0, second)
	z.B0.A1.Div(&first.B0.A1, second)
	z.B1.A0.Div(&first.B1.A0, second)
	z.B1.A1.Div(&first.B1.A1, second)
	return z
}

func SetInterface(z *Element, i1 interface{}) (*Element, error) {
	if i1 == nil {
		return nil, errors.New("can't set fr.Element with <nil>")
	}

	switch c1 := i1.(type) {
	case Element:
		return z.Set(&c1), nil
	case *Element:
		if c1 == nil {
			return nil, errors.New("can't set fext.Element with <nil>")
		}
		return z.Set(c1), nil
	case GenericFieldElem:
		return z.Set(&c1.Ext), nil
	case *GenericFieldElem:
		if c1 == nil {
			return nil, errors.New("can't set fext.Element with <nil>")
		}
		return z.Set(&c1.Ext), nil
	case field.Element:
		return SetFromBase(z, &c1), nil
	case *field.Element:
		if c1 == nil {
			return nil, errors.New("can't set fext.Element with <nil>")
		}
		return SetFromBase(z, c1), nil
	case uint8:
		return SetFromUIntBase(z, uint64(c1)), nil
	case uint16:
		return SetFromUIntBase(z, uint64(c1)), nil
	case uint32:
		return SetFromUIntBase(z, uint64(c1)), nil
	case uint:
		return SetFromUIntBase(z, uint64(c1)), nil
	case uint64:
		return SetFromUIntBase(z, c1), nil
	case int8:
		return SetFromIntBase(z, int64(c1)), nil
	case int16:
		return SetFromIntBase(z, int64(c1)), nil
	case int32:
		return SetFromIntBase(z, int64(c1)), nil
	case int64:
		return SetFromIntBase(z, c1), nil
	case int:
		return SetFromIntBase(z, int64(c1)), nil
	case string:
		z.B0.A0.SetString(c1)
		z.B0.A1.SetZero()
		z.B1.SetZero()
		return z, nil
	case *big.Int:
		if c1 == nil {
			return nil, errors.New("can't set fr.Element with <nil>")
		}
		z.B0.A0.SetBigInt(c1)
		return z, nil
	case big.Int:
		z.B0.A0.SetBigInt(&c1)
		return z, nil
	case []byte:
		z := SetBytes(c1)
		return &z, nil
	default:
		return nil, errors.New("can't set fext.Element from type " + reflect.TypeOf(i1).String())
	}
}

func Text(z *Element, base int) string {
	if base < 2 || base > 36 {
		panic("invalid base")
	}
	if z == nil {
		return "<nil>"
	}

	res := fmt.Sprintf("%s + %s*u + (%s + %s*u)*v", z.B0.A0.Text(base), z.B0.A1.Text(base), z.B1.A0.Text(base), z.B1.A1.Text(base))
	return res
}

func ParBatchInvert(a []Element, numCPU int) []Element {

	if numCPU == 0 {
		numCPU = runtime.GOMAXPROCS(0)
	}

	res := make([]Element, len(a))

	parallel.Execute(len(a), func(start, stop int) {
		BatchInvertInto(a[start:stop], res[start:stop])
	}, numCPU)

	return res
}

// MulRInv multiplies the field element by R^-1, where R is the Montgommery constant
func MulRInv(x Element) Element {
	var res Element
	res.MulByElement(&x, &field.MontConstantInv)
	return res
}
