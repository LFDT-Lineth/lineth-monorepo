package smartvectorsext

import (
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/sirupsen/logrus"
)

// assertCorrectBound panics if pos >= length
func assertCorrectBound(pos, length int) {
	if pos >= length {
		logrus.Panicf("Bound assertion failed, cannot access pos %v for vector of length %v", pos, length)
	}
}

// assertHasLength panics if a and b are not equal
func assertHasLength(a, b int) {
	if a != b {
		utils.Panic("the two slices should have the same length (found %v and %v)", a, b)
	}
}

// assertPowerOfTwoLen panics if l is not a power of two
func assertPowerOfTwoLen(l int) {
	if !utils.IsPowerOfTwo(l) {
		logrus.Panicf("Slice should have a power of two length but has %v", l)
	}
}

// assertStrictPositiveLen panics if l is 0 or negative
func assertStrictPositiveLen(l int) {

	if l == 0 {
		logrus.Panicf("FORBIDDEN : Got a null length vector")
	}

	if l <= 0 {
		logrus.Panicf("FORBIDDEN : Got a negative length %v", l)
	}
}
