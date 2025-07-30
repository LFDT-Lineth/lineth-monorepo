package testtools

import (
	"fmt"
	"math/rand/v2"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// #nosec G404 --we don't need a cryptographic RNG for testing purpose
var rng = rand.New(utils.NewRandSource(0))

// RandomVec returns a random vector of size "size".
func RandomVec(size int) smartvectors.SmartVector {
	return smartvectors.PseudoRand(rng, size)
}

// RandomVecPadded returns a random vector of size "size" such that the
// last "size-density" are zero.
func RandomVecPadded(density, size int) smartvectors.SmartVector {
	v := vector.PseudoRand(rng, density)
	return smartvectors.RightZeroPadded(v, size)
}

// RandomMatrix returns a random matrix of size "rows x cols" as a list
// of columns.
func RandomMatrix(rows, cols int) []smartvectors.SmartVector {
	var res []smartvectors.SmartVector
	for i := 0; i < cols; i++ {
		res = append(res, RandomVec(rows))
	}
	return res
}

// RandomBinary returns a random vector of binary values with 50% ones
// and 50% zeroes.
func RandBinary(size int) smartvectors.SmartVector {
	res := make([]field.Element, size)
	for i := range res {
		if rng.IntN(2) == 0 {
			res[i] = field.One()
		}
	}
	return smartvectors.NewRegular(res)
}

// RandomFromSeed returns a random vector generated from a user-supplied
// seed. This can be used to ensure that the same vector is generated
// through several calls.
func RandomFromSeed(size int, seed int64) smartvectors.SmartVector {
	// #nosec G404 --we don't need a cryptographic RNG for testing purpose
	rng := rand.New(utils.NewRandSource(seed))
	return smartvectors.PseudoRand(rng, size)
}

// OnesAt returns a vector of size "size" with ones at the positions
// given in "positions".
func OnesAt(size int, positions []int) smartvectors.SmartVector {
	res := make([]field.Element, size)
	for _, pos := range positions {
		res[pos] = field.One()
	}
	return smartvectors.NewRegular(res)
}

// Counting returns a vector of size "size" with values 0, 1, 2, ...
func Counting(size int) smartvectors.SmartVector {
	res := make([]field.Element, size)
	for i := range res {
		res[i] = field.NewElement(uint64(i))
	}
	return smartvectors.NewRegular(res)
}

// CountingAt returns a smartvector for size n, are starting from "init" and
// incrementing by 1 at the given indices. The indices must be sorted in
// ascending order.
func CountingAt(size int, init int, at []int) smartvectors.SmartVector {

	var (
		res      = make([]field.Element, size)
		cursorAt = 0
		one      = field.One()
	)

	for i := range res {

		if i == 0 {
			res[i] = field.NewElement(uint64(init))
		} else {
			res[i] = res[i-1]
		}

		if cursorAt < len(at) && i == at[cursorAt] {
			res[i].Add(&res[i], &one)
			cursorAt++
		}
	}

	return smartvectors.NewRegular(res)
}

// RandomAt returns a vector of size "size" with random values at the
// positions given in "positions".
func RandomAt(size int, positions ...int) smartvectors.SmartVector {
	res := make([]field.Element, size)
	for _, pos := range positions {
		res[pos] = field.PseudoRand(rng)
	}
	return smartvectors.NewRegular(res)
}

// XorTable returns a boolean XOR table consisting of 3 columns of size
// 2 ** 2nbits. The table lists all the possible triplets x, y, z such
// that x xor y = z for every (x, y) of 'nbits' bits.
func XorTable(nbits int) []smartvectors.SmartVector {

	var (
		numXs = 1 << nbits
		numYs = 1 << nbits

		xs = make([]field.Element, numXs*numYs)
		ys = make([]field.Element, numXs*numYs)
		zs = make([]field.Element, numXs*numYs)
	)

	for x := 0; x < numXs; x++ {
		for y := 0; y < numYs; y++ {
			pos := x*numYs + y
			z := x ^ y
			xs[pos] = field.NewElement(uint64(x))
			ys[pos] = field.NewElement(uint64(y))
			zs[pos] = field.NewElement(uint64(z))
		}
	}

	return []smartvectors.SmartVector{
		smartvectors.NewRegular(xs),
		smartvectors.NewRegular(ys),
		smartvectors.NewRegular(zs),
	}
}

// RandomSmallNumbers returns a vector of size "size" with random values
// between 0 and "max".
func RandomSmallNumbers(size, max int) smartvectors.SmartVector {
	res := make([]field.Element, size)
	for i := range res {
		res[i] = field.NewElement(uint64(rng.IntN(max)))
	}
	return smartvectors.NewRegular(res)
}

// formatName returns an underscore formatted string
func formatName[T ~string](args ...any) T {
	res := ""
	for i := range args {
		if i > 0 {
			res += "_"
		}
		res += fmt.Sprintf("%v", args[i])
	}
	return T(res)
}
