package utils

import (
	"math/big"
	"sort"
	"strconv"
)

/*
	All * standard * functions that we manually implement
*/

// IsPowerOfTwo returns true if n is a power of two.
func IsPowerOfTwo[T ~int](n T) bool {
	return n&(n-1) == 0 && n > 0
}

// Abs returns the absolute value of a.
func Abs(a int) int {
	mask := a >> (strconv.IntSize - 1) // made up of the sign bit
	// if mask is 0, then a ^ 0 - 0 = a. if mask is -1, then a ^ -1 - (-1) = -a - 1 - (-1) = -a
	return (a ^ mask) - mask
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
		Panic("inexact division: %v / %v", a, b)
	}
	return res
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

// GCD calculates GCD of a and b by Euclidean algorithm.
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

// BigsToBytes converts a slice of big.Int values to a slice of bytes.
func BigsToBytes(ins []*big.Int) []byte {
	res := make([]byte, len(ins))
	for i := range ins {
		res[i] = byte(ins[i].Uint64())
	}
	return res
}

// BigsToInts converts a slice of big.Int values to a slice of ints.
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

// StringKeysOfMap returns a sorted list of the keys of the map
func StringKeysOfMap[K ~string, V any](m map[K]V) []K {
	return SortedKeysOf(m, func(a, b K) bool {
		return a < b
	})
}

// NextMultipleOf returns the next multiple of "multiple" for "n".
// For instance n=8 and multiple=5 returns 10.
func NextMultipleOf(n, multiple int) int {
	return multiple * ((n + multiple - 1) / multiple)
}
