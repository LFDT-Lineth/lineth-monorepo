package smartvectors

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils"
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

// IsBase returns true if the vector type is a base vector
// will return false even if it is an extension that contains a base element wrapped
// inside an extension
func IsBase(vec SmartVector) bool {
	_, isBaseError := vec.GetBase(0)
	if isBaseError == nil {
		return true
	} else {
		return false
	}
}

// return true if all the vectors are base vectors
func AreAllBase(inp []SmartVector) bool {
	for _, v := range inp {
		if !IsBase(v) {
			return false
		}
	}
	return true
}

// IntoBase attempts to convert the input smartvector into a base smartvector,
// the function is not optimized and will return a new Regular vector even if
// the input is already on the base field.
func IntoBase(v SmartVector) (SmartVector, error) {

	// This is to ensure we listed all the types above
	new := make([]field.Element, v.Len())
	for i := 0; i < v.Len(); i++ {
		x, err := v.GetBase(i)
		new[i] = x
		if err != nil {
			return nil, fmt.Errorf("could not get base element at index %v: %v", i, err)
		}
	}

	return NewRegular(new), nil
}
