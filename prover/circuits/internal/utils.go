package internal

import (
	"encoding/binary"
	"errors"
	"math/big"
	"slices"

	fr377 "github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	fr381 "github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark-crypto/hash"
	hint "github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/compress"
	snarkHash "github.com/consensys/gnark/std/hash"
	"github.com/consensys/gnark/std/hash/mimc"
	"github.com/consensys/gnark/std/lookup/logderivlookup"
	"github.com/consensys/gnark/std/math/emulated"
	"github.com/consensys/linea-monorepo/prover/circuits/internal/plonk"
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

func SliceToTable(api frontend.API, slice []frontend.Variable) *logderivlookup.Table {
	table := logderivlookup.New(api)
	for i := range slice {
		table.Insert(slice[i])
	}
	return table
}

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

func NoCheck(b *bool) {
	*b = false
}

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

func RegisterHints() {
	hint.RegisterHint(toCrumbsHint, concatHint, checksumSubSlicesHint, partitionSliceHint, divEuclideanHint)
}

func toCrumbsHint(_ *big.Int, ins, outs []*big.Int) error {
	if len(ins) != 1 {
		return errors.New("expected 1 input")
	}

	if ins[0].Cmp(big.NewInt(0)) < 0 {
		return errors.New("input is negative")
	}
	if ins[0].Cmp(new(big.Int).Lsh(big.NewInt(1), 2*uint(len(outs)))) >= 0 {
		return errors.New("input exceeds the expected number of crumbs")
	}

	inLen := (len(outs) + 3) / 4
	in := ins[0].Bytes()
	in = append(make([]byte, inLen-len(in), inLen), in...) // zero pad to the left
	slices.Reverse(in)                                     // to little-endian

	for i := range outs {
		outs[i].SetUint64(uint64(in[0] & 3))
		in[0] >>= 2

		if i%4 == 3 {
			in = in[1:]
		}
	}

	return nil
}

// TODO add to gnark: bits.ToBase
// ToCrumbs decomposes scalar v into nbCrumbs 2-bit digits.
// It uses Little Endian order for compatibility with gnark, even though we use Big Endian order in the circuit
func ToCrumbs(api frontend.API, v frontend.Variable, nbCrumbs int) []frontend.Variable {
	res, err := api.Compiler().NewHint(toCrumbsHint, nbCrumbs, v)
	if err != nil {
		panic(err)
	}
	for _, c := range res {
		api.AssertIsCrumb(c)
	}
	return res
}

// PackedBytesToCrumbs converts a slice of bytes, padded with zeros on the left to make bitsPerElem bits field elements, into a slice of two-bit crumbs
// panics if bitsPerElem is not a multiple of 2
func PackedBytesToCrumbs(api frontend.API, bytes []frontend.Variable, bitsPerElem int) []frontend.Variable {
	crumbsPerElem := bitsPerElem / 2
	if bitsPerElem != 2*crumbsPerElem {
		panic("packing size must be a multiple of 2")
	}
	bytesPerElem := (bitsPerElem + 7) / 8
	firstByteNbCrumbs := crumbsPerElem % 4
	if firstByteNbCrumbs == 0 {
		firstByteNbCrumbs = 4
	}
	nbElems := (len(bytes) + bytesPerElem - 1) / bytesPerElem

	if nbElems*bytesPerElem != len(bytes) { // pad with zeros if necessary
		tmp := bytes
		bytes = make([]frontend.Variable, nbElems*bytesPerElem)
		copy(bytes, tmp)
		for i := len(tmp); i < len(bytes); i++ {
			bytes[i] = 0
		}
	}

	res := make([]frontend.Variable, 0, nbElems*crumbsPerElem)

	for i := 0; i < len(bytes); i += bytesPerElem {
		// first byte
		b := ToCrumbs(api, bytes[i], firstByteNbCrumbs)
		slices.Reverse(b)
		res = append(res, b...)
		// remaining bytes
		for j := 1; j < bytesPerElem; j++ {
			b = ToCrumbs(api, bytes[i+j], 4)
			slices.Reverse(b)
			res = append(res, b...)
		}
	}

	return res
}

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

// Concat does not range check the slice lengths individually, but it does their sum
// all slices have to be nonempty. This is not checked either
// Runtime in the order of maxLinLength + len(slices) * max_i(len(slices[i])
// it does not perform well when one of the slices is statically much longer than the others
func Concat(api frontend.API, maxLinearizedLength int, slices ...VarSlice) Slice[frontend.Variable] {

	res := Slice[frontend.Variable]{make([]frontend.Variable, maxLinearizedLength), 0}
	var outT *logderivlookup.Table
	{ // hint
		inLen := 2 * len(slices)
		for i := range slices {
			inLen += len(slices[i].Values)
		}
		in := make([]frontend.Variable, inLen)
		i := 0
		for _, s := range slices {
			in[i], in[i+1] = len(s.Values), s.Length
			copy(in[i+2:], s.Values)
			i += 2 + len(s.Values)
		}
		out, err := api.Compiler().NewHint(concatHint, maxLinearizedLength, in...)
		if err != nil {
			panic(err)
		}
		copy(res.Values, out)
		outT = SliceToTable(api, out)
		outT.Insert(0)
	}

	for _, s := range slices {
		r := NewRange(api, s.Length, len(s.Values))
		for j := range s.Values {
			AssertEqualIf(api, r.InRange[j], outT.Lookup(res.Length)[0], s.Values[j])
			res.Length = api.Add(res.Length, r.InRange[j])
		}
	}

	api.AssertIsDifferent(res.Length, maxLinearizedLength+1) // the only possible overflow without a range error on the lookup side

	return res
}

// ins = [maxLen(s[0]), len(s[0]), s[0][0], s[0][1], ..., maxLen(s[1]), len(s[1]), s[1][0], ...]
func concatHint(_ *big.Int, ins, outs []*big.Int) error {
	for len(ins) != 0 {
		m := ins[0].Uint64()
		l := ins[1].Uint64()
		if !ins[0].IsUint64() || !ins[1].IsUint64() || l > m {
			return errors.New("unacceptable lengths")
		}
		for i := uint64(0); i < l; i++ {
			outs[i].Set(ins[2+i])
		}
		ins = ins[2+m:]
		outs = outs[l:]
	}
	for i := range outs {
		outs[i].SetUint64(0)
	}
	return nil
}

// SubSlice DOES NOT range check start and length, but it does end := start + length
func SubSlice(api frontend.API, slice Slice[frontend.Variable], start, length frontend.Variable, maxLength int) Slice[frontend.Variable] {
	if maxLength < 0 || maxLength > len(slice.Values) {
		panic("invalid maxLength")
	}
	res := make([]frontend.Variable, maxLength)
	t := SliceToTable(api, slice.Values)
	/*for i := 0; i < maxLength; i++ {	TODO make sure padding is unnecessary
		t.Insert(0)
	}*/

	end := api.Add(start, length)
	api.AssertIsLessOrEqual(end, slice.Length)

	r := NewRange(api, length, maxLength)
	for i := range res {
		res[i] = api.Mul(r.InRange[i], t.Lookup(api.Add(start, i))[0])
	}

	return Slice[frontend.Variable]{res, length}
}

// ChecksumSlice returns H(len(slice), partial) where partial is slice[0] if length is 1, and hsh(slice[0], slice[1], ..., slice[len(slice)-1]) otherwise
// Proof of collision resistance: By the collision resistance of H, we may assume that f(a_1, ..., a_n) = f(b_1, ..., b_m) implies
// m=1 and f(a_1,...,a_n) = f(b) = b. If n = 1, then f(a) = a and thus a = b.
// Otherwise, n ≠ 1 implies H(n, f(a_1,...,a_n)) ≠ H(1, b).
func ChecksumSlice(slice [][]byte) []byte {
	if len(slice) == 0 {
		panic("empty slice not supported")
	}

	partial := slice[0]
	hsh := hash.MIMC_BLS12_377.New()
	for i := 1; i < len(slice); i++ {
		hsh.Reset()
		hsh.Write(partial)
		hsh.Write(slice[i])
		partial = hsh.Sum(nil)
	}

	hsh.Reset()
	n := Uint64To32Bytes(uint64(len(slice)))
	hsh.Write(n[:])
	hsh.Write(partial)

	return hsh.Sum(nil)
}

// TODO test that ChecksumSliceSnark matches ChecksumSubSlices

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

// ChecksumSubSlices returns the hash of consecutive sub-slices, compatible with ChecksumSliceSnark
// the endpoints are not range checked and are assumed to be strictly increasing, and that endpoint[0] ≠ 0 and endpoint[endpoints.Length-1] \leq len(slice)
// due to its hint, it only works with MiMC in BLS12-377
func ChecksumSubSlices(api frontend.API, hsh snarkHash.FieldHasher, slice []frontend.Variable, subEndPoints VarSlice) []frontend.Variable {
	lastElems := make([]frontend.Variable, len(subEndPoints.Values))
	endpointsR := subEndPoints.Range(api)
	for i, e := range subEndPoints.Values {
		// inRange ? e - 1 : l  = l + inRange * (e- 1 -l) = inRange * e - (1+l) * inRange + l
		lastElems[i] =
			plonk.EvaluateExpression(api, e, endpointsR.InRange[i], 0, -1-len(slice), 1, len(slice))
	}
	var lastElemsT *logderivlookup.Table
	if len(slice) > 1 {
		lastElemsT = SliceToTable(api, lastElems)
		lastElemsT.Insert(len(slice)) // in case subEndPoints is tight
	}

	hintIn := make([]frontend.Variable, len(lastElems)+len(slice))
	copy(hintIn, lastElems)
	copy(hintIn[len(lastElems):], slice)
	res, err := api.Compiler().NewHint(checksumSubSlicesHint, len(subEndPoints.Values), hintIn...)
	if err != nil {
		panic(err)
	}
	resT := SliceToTable(api, res)
	resT.Insert(0) // in case subEndPoints is tight

	var subI, workingHash, incrementI frontend.Variable
	subI, workingHash, incrementI = 0, slice[0], api.IsZero(api.Sub(subEndPoints.Values[0], 1))

	for i := 1; i < len(slice); i++ {

		// if we are done with this hash, check it
		AssertEqualIf(api, incrementI, workingHash, resT.Lookup(subI)[0])
		subI = api.Add(subI, incrementI)

		hsh.Reset()
		hsh.Write(workingHash, slice[i])
		workingHash = api.Select(incrementI, slice[i], hsh.Sum()) // if this is a new checksum, we will just take the raw value

		incrementI = api.IsZero(api.Sub(i, lastElemsT.Lookup(subI)[0]))
	}

	// if we are done with this hash, check it (needed in case subEndPoints is tight)
	AssertEqualIf(api, incrementI, workingHash, resT.Lookup(subI)[0])
	subI = api.Add(subI, incrementI)

	api.AssertIsEqual(subI, subEndPoints.Length)

	lengths := Differences(api, subEndPoints.Values)

	for i := range res {
		hsh.Reset()
		hsh.Write(lengths[i], res[i])
		res[i] = hsh.Sum()
	}

	return res
}

// the in has the format [subEndPoints..., slice...]
// TODO figure out the expected behavior for outs past the end of subEndPoints
func checksumSubSlicesHint(_ *big.Int, ins, outs []*big.Int) error {
	subLastPoints := ins[:len(outs)]
	slice := ins[len(outs):]

	sliceAt := func(i int64) []byte {
		res := slice[i].Bytes()
		if len(res) == 0 {
			return []byte{0} // the mimc hash impl ignores empty input
		}
		return res
	}

	hsh := hash.MIMC_BLS12_377.New()
	var (
		first int64
		i     int
	)
	for ; i < len(outs); i++ {
		last := subLastPoints[i].Int64()
		if last >= int64(len(slice)) {
			break
		}

		out := sliceAt(first)

		for j := first + 1; j <= last; j++ { // TODO just do a loop of "writes" as this is how mimc computes long hashes anyway
			hsh.Reset()
			hsh.Write(out)
			hsh.Write(sliceAt(j))
			out = hsh.Sum(nil)
		}

		outs[i].SetBytes(out)

		first = last + 1
	}

	for ; i < len(outs); i++ {
		outs[i].SetUint64(0)
	}

	return nil
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

func CombineBytesIntoElements(api frontend.API, b [32]frontend.Variable) [2]frontend.Variable {
	r := big.NewInt(256)
	return [2]frontend.Variable{
		compress.ReadNum(api, b[:16], r),
		compress.ReadNum(api, b[16:], r),
	}
}

// Bls12381ScalarToBls12377Scalars interprets its input as a BLS12-381 scalar, with a modular reduction if necessary, returning two BLS12-377 scalars
// r[1] is the lower 252 bits. r[0] is the higher 3 bits.
// useful in circuit "assign" functions
func Bls12381ScalarToBls12377Scalars(v interface{}) (r [2][]byte, err error) {
	var x fr381.Element
	if _, err = x.SetInterface(v); err != nil {
		return
	}

	b := x.Bytes()

	r[0] = make([]byte, fr377.Bytes)
	r[0][fr381.Bytes-1] = b[0] >> 4

	b[0] &= 0x0f
	r[1] = b[:]
	return
}

// PartialSums returns s[0], s[0]+s[1], ..., s[0]+s[1]+...+s[len(s)-1]
func PartialSums(api frontend.API, s []frontend.Variable) []frontend.Variable {
	res := make([]frontend.Variable, len(s))
	res[0] = s[0]
	for i := 1; i < len(s); i++ {
		res[i] = api.Add(res[i-1], s[i])
	}
	return res
}

func Differences(api frontend.API, s []frontend.Variable) []frontend.Variable {
	res := make([]frontend.Variable, len(s))
	prev := frontend.Variable(0)
	for i := range s {
		res[i] = api.Sub(s[i], prev)
		prev = s[i]
	}
	return res
}

func Sum[T constraints.Integer](x ...T) T {
	var res T

	for _, xI := range x {
		res += xI
	}

	return res
}

func Uint64To32Bytes(i uint64) [32]byte {
	var b [32]byte
	binary.BigEndian.PutUint64(b[24:], i)
	return b
}

func PartialSumsInt[T constraints.Integer](s []T) []T {
	res := make([]T, len(s))
	prev := T(0)
	for i := range res {
		prev += s[i]
		res[i] = prev
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

// Truncate ensures that the slice is 0 starting from the n-th element
func Truncate(api frontend.API, slice []frontend.Variable, n frontend.Variable) []frontend.Variable {
	nYet := frontend.Variable(0)
	res := make([]frontend.Variable, len(slice))
	for i := range slice {
		nYet = api.Add(nYet, api.IsZero(api.Sub(i, n)))
		res[i] = api.MulAcc(api.Mul(1, slice[i]), slice[i], api.Neg(nYet))
	}
	return res
}

// RotateLeft rotates the slice v by n positions to the left, so that res[i] becomes v[(i+n)%len(v)]
func RotateLeft(api frontend.API, v []frontend.Variable, n frontend.Variable) (res []frontend.Variable) {
	res = make([]frontend.Variable, len(v))
	t := SliceToTable(api, v)
	for _, x := range v {
		t.Insert(x)
	}
	for i := range res {
		res[i] = t.Lookup(api.Add(i, n))[0]
	}
	return
}

func CloneSlice[T any](s []T, cap ...int) []T {
	res := make([]T, len(s), max(len(s), Sum(cap...)))
	copy(res, s)
	return res
}

// PartitionSlice populates sub-slices subs[0], ... where subs[i] contains the elements s[j] with selectors[j] = i
// There are no guarantee on the values in the subs past their actual lengths. The hint sets them to zero but PartitionSlice does not check that fact.
// It may produce an incorrect result if selectors are out of range
func PartitionSlice(api frontend.API, s []frontend.Variable, selectors []frontend.Variable, subs ...[]frontend.Variable) {
	if len(s) != len(selectors) {
		panic("s and selectors must have the same length")
	}
	hintIn := make([]frontend.Variable, 1+len(subs)+len(s)+len(selectors))
	hintIn[0] = len(subs)
	hintOutLen := 0
	for i := range subs {
		hintIn[1+i] = len(subs[i])
		hintOutLen += len(subs[i])
	}
	for i := range s {
		hintIn[1+len(subs)+i] = s[i]
		hintIn[1+len(subs)+len(s)+i] = selectors[i]
	}
	subsGlued, err := api.Compiler().NewHint(partitionSliceHint, hintOutLen, hintIn...)
	if err != nil {
		panic(err)
	}

	subsT := make([]*logderivlookup.Table, len(subs))
	for i := range subs {
		copy(subs[i], subsGlued[:len(subs[i])])
		subsGlued = subsGlued[len(subs[i]):]
		subsT[i] = SliceToTable(api, subs[i])
		subsT[i].Insert(0)
	}

	subI := make([]frontend.Variable, len(subs))
	for i := range subI {
		subI[i] = 0
	}

	indicators := make([]frontend.Variable, len(subs))
	subHeads := make([]frontend.Variable, len(subs))
	for i := range s {
		for j := range subs[:len(subs)-1] {
			indicators[j] = api.IsZero(api.Sub(selectors[i], j))
		}
		indicators[len(subs)-1] = api.Sub(1, SumSnark(api, indicators[:len(subs)-1]...))

		for j := range subs {
			subHeads[j] = subsT[j].Lookup(subI[j])[0]
			subI[j] = api.Add(subI[j], indicators[j])
		}

		api.AssertIsEqual(s[i], InnerProd(api, subHeads, indicators))
	}

	// Check that the dummy trailing values weren't actually picked
	for i := range subI {
		api.AssertIsDifferent(subI[i], len(subs[i])+1)
	}
}

func SumSnark(api frontend.API, x ...frontend.Variable) frontend.Variable {
	res := frontend.Variable(0)
	for i := range x {
		res = api.Add(res, x[i])
	}
	return res
}

// ins: [nbSubs, maxLen_0, ..., maxLen_{nbSubs-1}, s..., indicators...]
func partitionSliceHint(_ *big.Int, ins, outs []*big.Int) error {

	subs := make([][]*big.Int, ins[0].Uint64())
	for i := range subs {
		subs[i] = outs[:ins[1+i].Uint64()]
		outs = outs[len(subs[i]):]
	}
	if len(outs) != 0 {
		return errors.New("the sum of subslice max lengths does not equal output length")
	}

	ins = ins[1+len(subs):]

	s := ins[:len(ins)/2]
	indicators := ins[len(s):]
	if len(s) != len(indicators) {
		return errors.New("s and indicators must be of the same length")
	}

	for i := range s {
		b := indicators[i].Int64()
		if b < 0 || b >= int64(len(subs)) || !indicators[i].IsUint64() {
			return errors.New("indicator out of range")
		}
		subs[b][0] = s[i]
		subs[b] = subs[b][1:]
	}

	for i := range subs {
		for j := range subs[i] {
			subs[i][j].SetInt64(0)
		}
	}

	return nil
}

// PartitionSliceEmulated populates sub-slices subs[0], ... where subs[i] contains the elements s[j] with selectors[j] = i
// There are no guarantee on the values in the subs past their actual lengths. The hint sets them to zero but PartitionSlice does not check that fact.
// It may produce an incorrect result if selectors are out of range
func PartitionSliceEmulated[T emulated.FieldParams](api frontend.API, s []emulated.Element[T], selectors []frontend.Variable, subSliceMaxLens ...int) [][]emulated.Element[T] {
	field, err := emulated.NewField[T](api)
	if err != nil {
		panic(err)
	}

	// transpose limbs for selection
	limbs := make([][]frontend.Variable, len(s[0].Limbs)) // limbs are indexed limb first, element second
	for i := range limbs {
		limbs[i] = make([]frontend.Variable, len(s))
	}
	for i := range s {
		if len(limbs) != len(s[i].Limbs) {
			panic("expected uniform number of limbs")
		}
		for j := range limbs {
			limbs[j][i] = s[i].Limbs[j]
		}
	}

	subLimbs := make([][][]frontend.Variable, len(limbs)) // subLimbs is indexed limb first, sub-slice second, element third

	for i := range limbs { // construct the sub-slices limb by limb
		subLimbs[i] = make([][]frontend.Variable, len(subSliceMaxLens))
		for j := range subSliceMaxLens {
			subLimbs[i][j] = make([]frontend.Variable, subSliceMaxLens[j])
		}

		PartitionSlice(api, limbs[i], selectors, subLimbs[i]...)
	}

	// put the limbs back together
	subSlices := make([][]emulated.Element[T], len(subSliceMaxLens))
	for i := range subSlices {
		subSlices[i] = make([]emulated.Element[T], subSliceMaxLens[i])
		for j := range subSlices[i] {
			currLimbs := make([]frontend.Variable, len(limbs))
			for k := range currLimbs {
				currLimbs[k] = subLimbs[k][i][j]
			}
			subSlices[i][j] = *field.NewElement(currLimbs) // TODO make sure dereferencing is not problematic
		}
	}

	return subSlices
}

func InnerProd(api frontend.API, x, y []frontend.Variable) frontend.Variable {
	if len(x) != len(y) {
		panic("mismatched lengths")
	}
	res := frontend.Variable(0)
	for i := range x {
		res = api.Add(res, api.Mul(x[i], y[i]))
	}
	return res
}

func SelectMany(api frontend.API, c frontend.Variable, ifSo, ifNot []frontend.Variable) []frontend.Variable {
	if len(ifSo) != len(ifNot) {
		panic("incompatible lengths")
	}
	res := make([]frontend.Variable, len(ifSo))
	for i := range res {
		res[i] = api.Select(c, ifSo[i], ifNot[i])
	}
	return res
}

// DivEuclidean conventional integer division with a remainder
// TODO @Tabaie replace all/most special-case divisions with this, barring performance issues
func DivEuclidean(api frontend.API, a, b frontend.Variable) (quotient, remainder frontend.Variable) {
	api.AssertIsDifferent(b, 0)
	outs, err := api.Compiler().NewHint(divEuclideanHint, 2, a, b)
	if err != nil {
		panic(err)
	}
	quotient, remainder = outs[0], outs[1]
	api.AssertIsLessOrEqual(remainder, api.Sub(b, 1))
	api.AssertIsLessOrEqual(quotient, a)

	return
}

func divEuclideanHint(_ *big.Int, ins, outs []*big.Int) error {
	if len(ins) != 2 || len(outs) != 2 {
		return errors.New("expected two inputs and two outputs")
	}

	a, b := ins[0], ins[1]
	quotient, remainder := outs[0], outs[1]

	quotient.Div(a, b)
	remainder.Mod(a, b)

	return nil
}
