package gnarkutil

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/compress"
	"github.com/consensys/gnark/std/math/emulated"
	"github.com/consensys/gnark/std/rangecheck"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
)

func Copy[T any](dst []frontend.Variable, src []T) (n int) {
	n = min(len(dst), len(src))

	for i := 0; i < n; i++ {
		dst[i] = koalagnark.NewElement(src[i])
	}

	return
}

// ToBytes32 decomposes x into 32 bytes.
func ToBytes32(api frontend.API, x frontend.Variable) [32]frontend.Variable {
	var res [32]frontend.Variable
	d, err := ToBytes(api, []frontend.Variable{x}, 256)
	if err != nil {
		panic(err)
	}
	slack := 32 - len(d) // should be zero
	copy(res[slack:], d)
	for i := range slack {
		res[i] = 0
	}
	return res
}

// ToBytes16 decomposes x into 32 bytes.
func ToBytes16(api frontend.API, x frontend.Variable) [16]frontend.Variable {
	var res [16]frontend.Variable
	d, err := ToBytes(api, []frontend.Variable{x}, 128)
	if err != nil {
		panic(err)
	}
	slack := 16 - len(d) // should be zero
	copy(res[slack:], d)
	for i := range slack {
		res[i] = 0
	}
	return res
}

// ReduceBytes reduces given bytes modulo a given field. As a side effect, the "bytes" are range checked
func ReduceBytes[T emulated.FieldParams](api frontend.API, bytes []*frontend.Variable) []frontend.Variable {

	bits := api.ToBinary(NewElementFromBytes[T](api, bytes))
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
func NewElementFromBytes[T emulated.FieldParams](api frontend.API, bytes []*frontend.Variable) frontend.Variable {

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

func ToVariableSlice[X any](s []X) []frontend.Variable {
	res := make([]frontend.Variable, len(s))
	Copy(res, s)
	return res
}

// ToBytes takes data words each containing wordNbBits many bits, and repacks them as 8-bit bytes.
// The data will be range checked only if the words are larger than a byte.
func ToBytes(api frontend.API, data []frontend.Variable, wordNbBits int) ([]frontend.Variable, error) {
	if wordNbBits == 8 {
		return data, nil
	}

	if wordNbBits < 8 {
		wordsPerByte := 8 / wordNbBits
		if wordsPerByte*wordNbBits != 8 {
			return nil, fmt.Errorf("currently only multiples or quotients of bytes supported, not the case for the given %d-bit words", wordNbBits)
		}
		radix := big.NewInt(1 << uint(wordNbBits))
		bytes := make([]frontend.Variable, len(data)*wordNbBits/8)
		for i := range bytes {
			bytes[i] = compress.ReadNum(api, data[i*wordsPerByte:i*wordsPerByte+wordsPerByte], radix)
		}
		return bytes, nil
	}

	bytesPerWord := wordNbBits / 8
	if bytesPerWord*8 != wordNbBits {
		return nil, fmt.Errorf("currently only multiples or quotients of bytes supported, not the case for the given %d-bit words", wordNbBits)
	}
	bytes, err := api.Compiler().NewHint(breakIntoBytesHint, len(data)*wordNbBits/8, data...)
	if err != nil {
		return nil, err
	}
	rc := rangecheck.New(api)
	for _, b := range bytes {
		rc.Check(b, 8)
	}

	radix := big.NewInt(256)
	for i := range data {
		api.AssertIsEqual(data[i], compress.ReadNum(api, bytes[i*bytesPerWord:i*bytesPerWord+bytesPerWord], radix))
	}

	return bytes, nil
}

func breakIntoBytesHint(_ *big.Int, words []*big.Int, bytes []*big.Int) error {
	bytesPerWord := len(bytes) / len(words)
	if bytesPerWord*len(words) != len(bytes) {
		return errors.New("words are not byte aligned")
	}

	for i := range words {
		b := words[i].Bytes()
		if len(b) > bytesPerWord {
			return fmt.Errorf("word #%d doesn't fit in %d bytes: 0x%s", i, bytesPerWord, words[i].Text(16))
		}
		for j := range b {
			bytes[i*bytesPerWord+j+bytesPerWord-len(b)].SetUint64(uint64(b[j]))
		}
	}
	return nil
}
