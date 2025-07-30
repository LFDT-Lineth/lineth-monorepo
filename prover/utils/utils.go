package utils

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"math"
	"math/big"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/consensys/gnark/frontend"
	"golang.org/x/exp/constraints"
)

/*
	All * standard * functions that we manually implement
*/

// Return true if n is a power of two
func IsPowerOfTwo[T ~int](n T) bool {
	return n&(n-1) == 0 && n > 0
}

func Abs(a int) int {
	mask := a >> (strconv.IntSize - 1) // made up of the sign bit
	return (a ^ mask) - mask           // if mask is 0, then a ^ 0 - 0 = a. if mask is -1, then a ^ -1 - (-1) = -a - 1 - (-1) = -a
}

// DivCeil for int a, b
func DivCeil(a, b int) int {
	res := a / b
	if b*res < a {
		return res + 1
	}
	return res
}

// DivExact for int a, b. Panics if b does not divide a exactly.
func DivExact(a, b int) int {
	res := a / b
	if res*b != a {
		panic("inexact division")
	}
	return res
}

// Iterates the function on all the given arguments and return an error
// if one is not equal to the first one. Panics if given an empty array.
func AllReturnEqual[T, U any](fs func(T) U, args []T) (U, error) {

	if len(args) < 1 {
		Panic("Empty list of slice")
	}

	first := fs(args[0])

	for _, arg := range args[1:] {
		curr := fs(arg)
		if !reflect.DeepEqual(first, curr) {
			return first, fmt.Errorf("mismatch between %v %v, got %v != %v",
				args[0], arg, first, curr,
			)
		}
	}

	return first, nil
}

/*
NextPowerOfTwo returns the next power of two for the given number.
It returns the number itself if it's a power of two. As an edge case,
zero returns zero.

Taken from :
https://github.com/protolambda/zrnt/blob/v0.13.2/eth2/util/math/math_util.go#L58
The function panics if the input is more than  2**62 as this causes overflow
*/
func NextPowerOfTwo[T ~int64 | ~uint64 | ~uintptr | ~int | ~uint](in T) T {
	if in < 0 || uint64(in) > 1<<62 {
		panic("input out of range")
	}
	v := in
	v--
	v |= v >> (1 << 0)
	v |= v >> (1 << 1)
	v |= v >> (1 << 2)
	v |= v >> (1 << 3)
	v |= v >> (1 << 4)
	v |= v >> (1 << 5)
	v++
	return v
}

// PositiveMod returns the positive modulus of [a] modulo [n]
func PositiveMod[T ~int](a, n T) T {
	res := a % n
	if res < 0 {
		return res + n
	}
	return res
}

// Join joins a set of slices by appending them into a new array. It can also
// be used to flatten a double array.
func Join[T any](ts ...[]T) []T {
	res := []T{}
	for _, t := range ts {
		res = append(res, t...)
	}
	return res
}

// Log2Floor computes the floored value of Log2
func Log2Floor(a int) int {
	res := 0
	for i := a; i > 1; i = i >> 1 {
		res++
	}
	return res
}

// Log2Ceil computes the ceiled value of Log2
func Log2Ceil(a int) int {
	floor := Log2Floor(a)
	if a != 1<<floor {
		floor++
	}
	return floor
}

// GCD calculates GCD of a and b by Euclidian algorithm.
func GCD[T ~int](a, b T) T {
	for a != b {
		if a > b {
			a -= b
		} else {
			b -= a
		}
	}

	return a
}

// Returns a SHA256 checksum of the given asset.
// TODO @gbotrel merge with Digest
// Sha2SumHexOf returns a SHA256 checksum of the given asset.
func Sha2SumHexOf(w io.WriterTo) string {
	hasher := sha256.New()
	w.WriteTo(hasher)
	res := hasher.Sum(nil)
	return HexEncodeToString(res)
}

// Digest computes the SHA256 Digest of the contents of file and prepends a "0x"
// byte to it. Callers are responsible for closing the file. The reliance on
// SHA256 is motivated by the fact that we use the sum checksum for the verifier
// key to identify which verifier contract to use.
func Digest(src io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, src); err != nil {
		return "", fmt.Errorf("copy into hasher: %w", err)
	}

	return "0x" + hex.EncodeToString(h.Sum(nil)), nil
}

// RightPadWith copies `s` and returns a vector padded up to length `n` using
// `padWith` as a filling value. The function panics if len(s) > n and returns
// a copy of s if len(s) == n.
func RightPadWith[T any](s []T, n int, padWith T) []T {
	if len(s) > n {
		panic("input slice longer than desired padded length")
	}
	res := append(make([]T, 0, n), s...)
	for len(res) < n {
		res = append(res, padWith)
	}
	return res
}

// RightPad copies `s` and returns a vector padded up to length `n`.
// The padding value is T's default.
// The padding value. The function panics if len(s) > n and returns a copy of s if len(s) == n.
func RightPad[T any](s []T, n int) []T {
	var padWith T
	return RightPadWith(s, n, padWith)
}

// RepeatSlice returns the concatenation of `s` with itself `n` times
func RepeatSlice[T any](s []T, n int) []T {
	res := make([]T, 0, n*len(s))
	for i := 0; i < n; i++ {
		res = append(res, s...)
	}
	return res
}

func BigsToBytes(ins []*big.Int) []byte {
	res := make([]byte, len(ins))
	for i := range ins {
		res[i] = byte(ins[i].Uint64())
	}
	return res
}

func BigsToInts(ints []*big.Int) []int {
	res := make([]int, len(ints))
	for i := range ints {
		u := ints[i].Uint64()
		res[i] = int(u) // #nosec G115 - check below
		if !ints[i].IsUint64() || uint64(res[i]) != u {
			panic("overflow")
		}
	}
	return res
}

// ToInt converts a uint, uint64 or int64 to an int, panicking on overflow.
// Due to its use of generics, it is inefficient to use in loops than run a "cryptographic" number of iterations. Use type-specific functions in such cases.
func ToInt[T ~uint | ~uint64 | ~int64](i T) int {
	if i > math.MaxInt {
		panic("overflow")
	}
	return int(i) // #nosec G115 -- Checked for overflow
}

// ToUint64 converts a signed integer into a uint64, panicking on negative values.
// Due to its use of generics, it is inefficient to use in loops than run a "cryptographic" number of iterations. Use type-specific functions in such cases.
func ToUint64[T constraints.Signed](i T) uint64 {
	if i < 0 {
		panic("negative")
	}
	return uint64(i)
}

func ToUint16[T ~int | ~uint](i T) uint16 {
	if i < 0 || i > math.MaxUint16 {
		panic("out of range")
	}
	return uint16(i) // #nosec G115 -- Checked for overflow
}

func ToVariableSlice[X any](s []X) []frontend.Variable {
	res := make([]frontend.Variable, len(s))
	Copy(res, s)
	return res
}

func countInts[I constraints.Integer](s []I) []I {
	counts := make([]I, Max(s...)+1)
	for _, x := range s {
		counts[x]++
	}
	return counts
}

func Partition[T any, I constraints.Integer](s []T, index []I) [][]T {
	if len(s) != len(index) {
		panic("s and index must have the same length")
	}
	if len(s) == 0 {
		return nil
	}
	partitions := make([][]T, Max(index...)+1)
	counts := countInts(index)
	for i := range partitions {
		partitions[i] = make([]T, 0, counts[i])
	}
	for i := range s {
		partitions[index[i]] = append(partitions[index[i]], s[i])
	}
	return partitions
}

func Ite[T any](cond bool, ifSo, ifNot T) T {
	if cond {
		return ifSo
	}
	return ifNot
}

func RangeSlice[T constraints.Integer](length int, startingPoints ...T) []T {
	if len(startingPoints) == 0 {
		startingPoints = []T{0}
	}
	res := make([]T, length*len(startingPoints))
	for i := range startingPoints {
		FillRange(res[i*length:(i+1)*length], startingPoints[i])
	}
	return res
}

func FillRange[T constraints.Integer](dst []T, start T) {
	for l := range dst {
		dst[l] = T(l) + start
	}
}

func ReadFromJSON(path string, v interface{}) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewDecoder(f).Decode(v)
}

func WriteToJSON(path string, v interface{}) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(v)
}

func WriterstoEqual(expected, actual io.WriterTo) error {
	var bb bytes.Buffer
	if _, err := expected.WriteTo(&bb); err != nil {
		return err
	}
	ab := bb.Bytes()
	bb.Reset()
	if _, err := actual.WriteTo(&bb); err != nil {
		return err
	}
	return BytesEqual(ab, bb.Bytes())
}

// BytesEqual between byte slices a,b
// a readable error message would show in case of inequality
// TODO error options: block size, check forwards or backwards etc
func BytesEqual(expected, actual []byte) error {
	if bytes.Equal(expected, actual) {
		return nil // equality fast path
	}

	l := min(len(expected), len(actual))

	failure := 0
	for failure < l {
		if expected[failure] != actual[failure] {
			break
		}
		failure++
	}

	if len(expected) == len(actual) && failure == l {
		panic("bytes.Equal returned false, but could not find a mismatch")
	}

	// there is a mismatch
	var sb strings.Builder

	const (
		radius    = 40
		blockSize = 32
	)

	printCentered := func(b []byte) {

		for i := max(failure-radius, 0); i <= failure+radius; i++ {
			if i%blockSize == 0 && i != failure-radius {
				sb.WriteString("  ")
			}
			if i >= 0 && i < len(b) {
				sb.WriteString(hex.EncodeToString([]byte{b[i]})) // inefficient, but this whole error printing sub-procedure will not be run more than once
			} else {
				sb.WriteString("  ")
			}
		}
	}

	sb.WriteString(fmt.Sprintf("mismatch starting at byte %d\n", failure))

	sb.WriteString("expected: ")
	printCentered(expected)
	sb.WriteString("\n")

	sb.WriteString("actual:   ")
	printCentered(actual)
	sb.WriteString("\n")

	sb.WriteString("          ")
	for i := max(failure-radius, 0); i <= failure+radius; {
		if i%blockSize == 0 && i != failure-radius {
			s := strconv.Itoa(i)
			sb.WriteString("  ")
			sb.WriteString(s)
			i += len(s) / 2
			if len(s)%2 != 0 {
				sb.WriteString(" ")
				i++
			}
		} else {
			if i == failure {
				sb.WriteString("^^")
			} else {
				sb.WriteString("  ")
			}
			i++
		}
	}

	sb.WriteString("\n")

	return &BytesEqualError{
		Index: failure,
		error: sb.String(),
	}
}

type BytesEqualError struct {
	Index int
	error string
}

func (e *BytesEqualError) Error() string {
	return e.error
}

func ReadFromFile(path string, to io.ReaderFrom) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	_, err = to.ReadFrom(f)
	return errors.Join(err, f.Close())
}

func WriteToFile(path string, from io.WriterTo) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0600) // TODO @Tabaie option for permissions?
	if err != nil {
		return err
	}
	_, err = from.WriteTo(f)
	return errors.Join(err, f.Close())
}

// SortedKeysOf returns a sorted list of the keys of the map using less
// to determine the order. Less is as in [sort.Slice]
func SortedKeysOf[K comparable, V any](m map[K]V, less func(K, K) bool) []K {

	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	// Since the keys of a map are all unique, we don't have to worry
	// about the duplicates and thus, we don't need a stable sort.
	sort.Slice(keys, func(i, j int) bool {
		return less(keys[i], keys[j])
	})

	return keys
}

// MapFunc maps f to every entries of the slice and return an array with the
// result.
func MapFunc[T, U any](slice []T, f func(T) U) []U {
	res := make([]U, len(slice))
	for i, v := range slice {
		res[i] = f(v)
	}
	return res
}

// Ternary returns "a" if cond is true, else b
func Ternary[T any](cond bool, ifTrue, ifFalse T) T {
	if cond {
		return ifTrue
	}
	return ifFalse
}

// SumFloat64: Calculates the sum of all values inside the float64 slice
func SumFloat64(vals []float64) (sum float64) {
	for _, val := range vals {
		sum += val
	}
	return sum
}

// CalculateMinAvgMax computes min, avg, and max for a slice of float64 values
func CalculateMinAvgMax(values []float64) (min, avg, max float64) {
	if len(values) == 0 {
		return 0, 0, 0
	}

	min = math.Inf(1)
	max = math.Inf(-1)
	sum := 0.0

	for _, v := range values {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
		sum += v
	}

	avg = sum / float64(len(values))
	return min, avg, max
}

// BytesToGiB converts bytes to GiB (Gibibytes)
func BytesToGiB(bytes uint64) float64 {
	const bytesInGiB = 1024 * 1024 * 1024 // 1 GiB = 1024^3 bytes
	return float64(bytes) / bytesInGiB
}

// ChainIterators concatenates iterators into a single iterator
func ChainIterators[V any](iters ...iter.Seq[V]) iter.Seq[V] {
	return func(yield func(V) bool) {
		for _, iter := range iters {
			for v := range iter {
				if !yield(v) {
					return
				}
			}
		}
	}
}

// ConstantIterator returns an iterator that always returns the same value
// n times.
func ConstantIterator[T any](value T, n int) iter.Seq[T] {
	return func(yield func(T) bool) {
		for i := 0; i < n; i++ {
			if !yield(value) {
				return
			}
		}
	}
}

// SpliceExact splits a slice into a slice of slices of size n
func SpliceExact[T any](slice []T, n int) [][]T {

	if len(slice)%n != 0 {
		panic("slice length must be a multiple of n")
	}

	slices := make([][]T, 0, len(slice)/n)
	for i := 0; i < len(slice); i += n {
		slices = append(slices, slice[i:i+n])
	}
	return slices
}

// SetDiff returns the difference between two sets. The elements are returned
// in non-deterministic order. The function returns aExtra for the elements
// in a but not in b and bExtra for the elements in b but not in a.
func SetDiff[T comparable](a, b []T) (aExtra, bExtra []T) {

	mset := make(map[T]int, len(a))

	for _, av := range a {
		if _, ok := mset[av]; !ok {
			mset[av] = 0
		}
		mset[av]++
	}

	for _, bv := range b {
		if _, ok := mset[bv]; !ok {
			mset[bv] = 0
		}
		mset[bv]--
	}

	for v, cnt := range mset {
		if cnt > 0 {
			aExtra = append(aExtra, v)
			// Importantly, we want to ditch the "zeroes" so that they don't end up
			// in bExtra.
		} else if cnt < 0 {
			bExtra = append(bExtra, v)
		}
	}

	return aExtra, bExtra
}

// NextMultipleOf returns the next multiple of "multiple" for "n".
// For instance n=8 and multiple=5 returns 10.
func NextMultipleOf(n, multiple int) int {
	return multiple * ((n + multiple - 1) / multiple)
}

// FilterInSliceWithSet returns the entries of slice that are in the set.
// The returned parameter "in" contains the entries found in the set and
// the returned parameter "out" contains the entries not found in the set.
func FilterInSliceWithMap[T comparable](slice []T, set map[T]struct{}) (in []T, out []T) {
	for _, v := range slice {
		if _, ok := set[v]; ok {
			in = append(in, v)
		} else {
			out = append(out, v)
		}
	}
	return in, out
}

// GrowSliceSize grows the size of a slice to the provided size. The function
// does so by appending "zero" elements of the slice. If the slice is already
// large enough, the function does nothing.
func GrowSliceSize[T any](slice []T, size int) []T {

	// Note: this clause is not necessary as the loop will just be skipped if
	// the slice is already large enough.
	if len(slice) >= size {
		return slice
	}

	for i := len(slice); i < size; i++ {
		var t T
		slice = append(slice, t)
	}
	return slice
}
