package utils

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/math/emulated"
)

func ToVariableSlice[X any](s []X) []frontend.Variable {
	res := make([]frontend.Variable, len(s))
	Copy(res, s)
	return res
}

func Copy[T any](dst []frontend.Variable, src []T) (n int) {
	n = min(len(dst), len(src))

	for i := 0; i < n; i++ {
		dst[i] = src[i]
	}

	return
}

func RegisterHints() {
}

// ReduceBytes reduces given bytes modulo a given field. As a side effect, the "bytes" are range checked
func ReduceBytes[T emulated.FieldParams](api frontend.API, bytes []frontend.Variable) []frontend.Variable {
	f, err := emulated.NewField[T](api)
	if err != nil {
		panic(err)
	}

	bits := f.ToBits(NewElementFromBytes[T](api, bytes))
	res := make([]frontend.Variable, (len(bits)+7)/8)
	copy(bits[:], bits)
	for i := len(bits); i < len(bits); i++ {
		bits[i] = 0
	}
	for i := range res {
		bitsStart := 8 * (len(res) - i - 1)
		bitsEnd := bitsStart + 8
		if i == 0 {
			bitsEnd = len(bits)
		}
		res[i] = api.FromBinary(bits[bitsStart:bitsEnd]...)
	}

	return res
}

// NewElementFromBytes range checks the bytes and gives a reduced field element
func NewElementFromBytes[T emulated.FieldParams](api frontend.API, bytes []frontend.Variable) *emulated.Element[T] {
	bits := make([]frontend.Variable, 8*len(bytes))
	for i := range bytes {
		copy(bits[8*i:], api.ToBinary(bytes[len(bytes)-i-1], 8))
	}

	f, err := emulated.NewField[T](api)
	if err != nil {
		panic(err)
	}

	return f.Reduce(f.Add(f.FromBits(bits...), f.Zero()))
}
