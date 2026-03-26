package internal

import (
	"math/big"

	"github.com/consensys/gnark/frontend"
	snarkHash "github.com/consensys/gnark/std/hash"
	"github.com/consensys/gnark/std/hash/mimc"
	"golang.org/x/exp/constraints"
)

// @reviewer: let's discuss which of these may be good candidates for gnark/std
// and if so, the ideal API for them

// AssertEqualIf asserts cond ≠ 0 ⇒ (a == b)
func AssertEqualIf(api frontend.API, cond, a, b frontend.Variable) {
	// in r1cs it's more efficient to do api.AssertIsEqual(0, api.Mul(cond, api.Sub(a, b))) but we don't care about that
	// and the following is better for debugging
	api.AssertIsEqual(api.Mul(cond, a), api.Mul(cond, b))
}

// AssertIsLessIf asserts cond ≠ 0 ⇒ (a < b)

func AssertSliceEquals(api frontend.API, a, b []frontend.Variable) {
	api.AssertIsEqual(len(a), len(b))
	for i := range a {
		api.AssertIsEqual(a[i], b[i])
	}
}

type Range struct {
	InRange, IsLast, IsFirstBeyond []frontend.Variable
	api                            frontend.API
}

type NewRangeOption func(*bool)

func NewRange(api frontend.API, n frontend.Variable, max int, opts ...NewRangeOption) *Range {

	if max < 0 {
		panic("negative maximum not allowed")
	}

	check := true
	for _, o := range opts {
		o(&check)
	}

	if max == 0 {
		if check {
			api.AssertIsEqual(n, 0)
		}
		return &Range{api: api}
	}

	inRange := make([]frontend.Variable, max)
	isLast := make([]frontend.Variable, max)
	isFirstBeyond := make([]frontend.Variable, max)

	prevInRange := frontend.Variable(1)
	for i := range isFirstBeyond {
		isFirstBeyond[i] = api.IsZero(api.Sub(i, n))
		prevInRange = api.Sub(prevInRange, isFirstBeyond[i])
		inRange[i] = prevInRange
		if i != 0 {
			isLast[i-1] = isFirstBeyond[i]
		}
	}
	isLast[max-1] = api.IsZero(api.Sub(max, n))

	if check {
		// if the last element is still in range, it must be the last, meaning isLast = 1 = inRange, otherwise n > max
		// if the last element is not in range, it already means n is in range and we don't need to check anything, but isLast = 0 = inRange will be the case anyway
		api.AssertIsEqual(isLast[max-1], inRange[max-1])
	}

	return &Range{inRange, isLast, isFirstBeyond, api}
}

// AssertEqualI i ∈ [0, n) ⇒ a == b with n as given to the constructor
func (r *Range) AssertEqualI(i int, a, b frontend.Variable) {
	AssertEqualIf(r.api, r.InRange[i], a, b)
}

// AssertEqualLastI i == n-1 ⇒ a == b with n as given to the constructor
func (r *Range) AssertEqualLastI(i int, a, b frontend.Variable) {
	AssertEqualIf(r.api, r.IsLast[i], a, b)
}

// AddIfLastI returns accumulator + IsLast[i] * atI
func (r *Range) AddIfLastI(i int, accumulator, atI frontend.Variable) frontend.Variable {
	toAdd := r.api.Mul(r.IsLast[i], atI)
	if accumulator == nil {
		return toAdd
	}
	return r.api.Add(accumulator, toAdd)
}

func (r *Range) AssertArrays32Equal(a, b [][32]frontend.Variable) {
	// TODO generify when array length parameters become available
	r.api.AssertIsEqual(len(a), len(b))
	for i := range a {
		for j := range a[i] {
			r.AssertEqualI(i, a[i][j], b[i][j])
		}
	}
}

func (r *Range) LastArray32(slice [][32]frontend.Variable) [32]frontend.Variable {
	return r.LastArray32F(func(i int) [32]frontend.Variable { return slice[i] })
}

func (r *Range) LastArray32F(provider func(int) [32]frontend.Variable) [32]frontend.Variable {
	// TODO generify when array length parameters become available
	var res [32]frontend.Variable
	for i := 0; i < len(r.InRange); i++ {
		for j := 0; j < 32; j++ {
			res[j] = r.AddIfLastI(i, res[j], provider(i)[j])
		}
	}
	return res
}

// TODO add to gnark: bits.ToBase
// ToCrumbs decomposes scalar v into nbCrumbs 2-bit digits.
// It uses Little Endian order for compatibility with gnark, even though we use Big Endian order in the circuit

// PackedBytesToCrumbs converts a slice of bytes, padded with zeros on the left to make bitsPerElem bits field elements, into a slice of two-bit crumbs
// panics if bitsPerElem is not a multiple of 2

func (r *Range) StaticLength() int {
	return len(r.InRange)
}

func MimcHash(api frontend.API, e ...frontend.Variable) frontend.Variable {
	hsh, err := mimc.NewMiMC(api)
	if err != nil {
		panic(err)
	}
	hsh.Write(e...)
	return hsh.Sum()
}

type Slice[T any] struct {

	// @reviewer: better for slice to be non-generic with frontend.Variable type and just duplicate the funcs for the few [32]frontend.Variable applications?
	Values []T // Values[:Length] contains the data
	Length frontend.Variable
}

func (s VarSlice) Range(api frontend.API) *Range {
	return NewRange(api, s.Length, len(s.Values))
}

type VarSlice Slice[frontend.Variable]
type Var32Slice Slice[[32]frontend.Variable]

// Checksum is the SNARK equivalent of ChecksumSlice
// TODO consider doing (r *Range) f (slice []frontend.Variable)
func (s VarSlice) Checksum(api frontend.API, hsh snarkHash.FieldHasher) frontend.Variable {
	if len(s.Values) == 0 {
		panic("zero-length input")
	}
	api.AssertIsDifferent(s.Length, 0)

	r := NewRange(api, s.Length, len(s.Values))

	hsh.Reset()
	sum := s.Values[0]
	for i := 1; i < len(s.Values); i++ {
		hsh.Reset()
		hsh.Write(sum, s.Values[i])
		sum = api.Select(r.InRange[i], hsh.Sum(), sum)
	}

	hsh.Reset()
	hsh.Write(s.Length, sum)

	return hsh.Sum()
}

// PartialSums returns a slice of the same length as slice, where res[i] = slice[0] + ... + slice[i]. Out of range values are excluded.
func (r *Range) PartialSums(slice []frontend.Variable) []frontend.Variable {
	return r.PartialSumsF(func(i int) frontend.Variable { return slice[i] })
}

func (r *Range) PartialSumsF(provider func(int) frontend.Variable) []frontend.Variable {
	if len(r.InRange) == 0 {
		return nil
	}

	res := make([]frontend.Variable, len(r.InRange))

	res[0] = r.api.Mul(provider(0), r.InRange[0])

	for i := 1; i < len(r.InRange); i++ {
		res[i] = r.api.Add(r.api.Mul(provider(i), r.InRange[i]), res[i-1])
	}

	return res
}

func (r *Range) LastF(provider func(i int) frontend.Variable) frontend.Variable {
	if len(r.IsLast) == 0 {
		return 0
	}
	res := r.api.Mul(r.IsLast[0], provider(0))
	for i := 1; i < len(r.IsLast); i++ {
		res = r.api.Add(res, r.api.Mul(r.IsLast[i], provider(i)))
	}
	return res
}

// PackFull packs as many words as possible into a single field element
// The words are construed in big-endian, and 0 padding is added as needed on the left for every element and on the right for the last element
func PackFull(api frontend.API, words []frontend.Variable, bitsPerWord int) []frontend.Variable {
	return Pack(api, words, api.Compiler().FieldBitLen()-1, bitsPerWord)
}

func Pack(api frontend.API, words []frontend.Variable, bitsPerElem, bitsPerWord int) []frontend.Variable {
	if bitsPerWord > bitsPerElem {
		panic("words don't fit in elements")
	}
	wordsPerElem := bitsPerElem / bitsPerWord
	res := make([]frontend.Variable, (len(words)+wordsPerElem-1)/wordsPerElem)
	if len(words) != len(res)*wordsPerElem {
		tmp := words
		words = make([]frontend.Variable, len(res)*wordsPerElem)
		copy(words, tmp)
		for i := len(tmp); i < len(words); i++ {
			words[i] = 0
		}
	}

	// TODO add this to gnark: bits.FromBase
	coeffs := make([]*big.Int, wordsPerElem)
	for i := range coeffs {
		coeffs[len(coeffs)-1-i] = new(big.Int).Lsh(big.NewInt(1), uint(i*bitsPerWord))
	}

	// TODO use compress.ReadNum?
	for i := range res {
		currWords := words[i*wordsPerElem : (i+1)*wordsPerElem]
		res[i] = api.Mul(coeffs[0], currWords[0]) // TODO once the "add 0" optimization is implemented in gnark, remove this line
		for j := 1; j < len(currWords); j++ {
			res[i] = api.MulAcc(res[i], coeffs[j], currWords[j])
		}
	}
	return res
}

func flatten(s [][32]frontend.Variable) []frontend.Variable {
	res := make([]frontend.Variable, len(s)*32)
	for i := range s {
		for j := range s[i] {
			res[i*32+j] = s[i][j]
		}
	}
	return res
}

func (s Var32Slice) Checksum(api frontend.API) frontend.Variable {
	values := PackFull(api, flatten(s.Values), 8)
	valsAndLen := make([]frontend.Variable, 1, len(values)+1)
	valsAndLen[0] = s.Length
	return MimcHash(api, append(valsAndLen, values...)...)
}

func Sum[T constraints.Integer](x ...T) T {
	var res T

	for _, xI := range x {
		res += xI
	}

	return res
}

func MapSlice[X, Y any](f func(X) Y, x ...X) []Y {
	y := make([]Y, len(x))
	for i := range x {
		y[i] = f(x[i])
	}
	return y
}
