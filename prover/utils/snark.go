package utils

import (
	"errors"
	"math/big"

	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/math/emulated"
	"github.com/consensys/gnark/std/rangecheck"
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

func ToBytes(api frontend.API, x frontend.Variable) [32]frontend.Variable {
	var res [32]frontend.Variable
	d := decomposeIntoBytes(api, x, fr.Bits)
	slack := 32 - len(d) // should be zero
	copy(res[slack:], d)
	for i := 0; i < slack; i++ {
		res[i] = 0
	}
	return res
}

func decomposeIntoBytes(api frontend.API, data frontend.Variable, nbBits int) []frontend.Variable {
	if nbBits == 0 {
		nbBits = api.Compiler().FieldBitLen()
	}

	nbBytes := (api.Compiler().FieldBitLen() + 7) / 8

	bytes, err := api.Compiler().NewHint(decomposeIntoBytesHint, nbBytes, data)
	if err != nil {
		panic(err)
	}
	lastNbBits := nbBits % 8
	if lastNbBits == 0 {
		lastNbBits = 8
	}
	rc := rangecheck.New(api)
	api.AssertIsLessOrEqual(bytes[0], 1<<lastNbBits-1) //TODO try range checking this as well
	for i := 1; i < nbBytes; i++ {
		rc.Check(bytes[i], 8)
	}

	return bytes
}

func decomposeIntoBytesHint(_ *big.Int, ins, outs []*big.Int) error {
	nbBytes := len(outs) / len(ins)
	if nbBytes*len(ins) != len(outs) {
		return errors.New("incongruent number of ins/outs")
	}
	var v, radix, zero big.Int
	radix.SetUint64(256)
	for i := range ins {
		v.Set(ins[i])
		for j := nbBytes - 1; j >= 0; j-- {
			outs[i*nbBytes+j].Mod(&v, &radix)
			v.Rsh(&v, 8)
		}
		if v.Cmp(&zero) != 0 {
			return errors.New("not fitting in len(outs)/len(ins) many bytes")
		}
	}
	return nil
}

func RegisterHints() {
	solver.RegisterHint(decomposeIntoBytesHint)
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
