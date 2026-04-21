// vector offers a set of utility function relating to slices of field element
// and that are commonly used as part of the repo.
package vectorext

import (
	"fmt"
	"math/rand/v2"
	"slices"

	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/parallel"
)

// DeepCopy deep-copies the input vector
func DeepCopy(pol []fext.Element) []fext.Element {
	return slices.Clone(pol)
}

// Rand creates a random vector of size n
func Rand(n int) Vector {
	vec := make(Vector, n)
	for i := range vec {
		_, err := vec[i].SetRandom()
		// Just to enfore never having to deal with zeroes
		if err != nil {
			panic(err)
		}
	}
	return vec
}

// Prettify returns a string representing `a` in a human-readable fashion
func Prettify(a []fext.Element) string {
	res := "["

	for i := range a {
		// Discards the case first element when adding a comma
		if i > 0 {
			res += ", "
		}

		res += fmt.Sprintf("%v", a[i].String())
	}
	res += "]"

	return res
}

// Reverse the elements of a vector inplace
func Reverse(v Vector) {
	slices.Reverse(v)
}

// Repeat returns a vector of size n whose values are all equal to x.
func Repeat(x fext.Element, n int) Vector {
	res := make(Vector, n)
	for i := range res {
		res[i].Set(&x)
	}
	return res
}

// ForTest returns a vector instantiated from a list of integers.
func ForTest(xs ...int) Vector {
	res := make(Vector, len(xs))
	for i, x := range xs {
		res[i].B0.A0.SetInt64(int64(x))
	}
	return res
}

// ForTestFromVect computes a vector of field extensions,
// where each field extension is populated using one vector of size [fext.ExtensionDegree]
func ForRandTestFromLen(len int) Vector {
	res := make(Vector, len)
	for i := range res {
		res[i].SetRandom()
	}
	return res
}

// ForTestFromQuads groups the input into Quaternarys. Each Quaternary populates the first four
// coordinates of a field extension, and the function then
// returns a vector instantiated these field extension elements.
func ForTestFromQuads(xs ...int) Vector {
	if len(xs)%4 != 0 {
		panic("ForTestFromQuads must receive a 4n-length input vector")
	}
	res := make(Vector, len(xs)/4)
	for i := 0; i < len(res); i++ {
		res[i] = fext.NewFromInt(int64(xs[4*i]), int64(xs[4*i+1]), int64(xs[4*i+2]), int64(xs[4*i+3]))
	}
	return res
}

// ZeroPad pads a vector to a given length.
// If the newLen is smaller than len(v), the function panic. It pads to the
// right (appending, not prepending)
// The resulting slice is allocated by the function, so it can be safely
// modified by the caller after the function returns.
func ZeroPad(v Vector, newLen int) Vector {
	if newLen < len(v) {
		utils.Panic("newLen (%v) < len(v) (%v)", newLen, len(v))
	}
	res := make(Vector, newLen)
	copy(res, v)
	return res
}

// Interleave interleave two vectors:
//
//	(a, a, a, a), (b, b, b, b) -> (a, b, a, b, a, b, a, b)
//
// The vecs[i] vectors must all have the same length
func Interleave(vecs ...Vector) Vector {
	numVecs := len(vecs)
	vecSize := len(vecs[0])

	// all vectors must have the same length
	for i := range vecs {
		if len(vecs[i]) != vecSize {
			utils.Panic("length mismatch, %v != %v", len(vecs[i]), vecSize)
		}
	}

	res := make(Vector, numVecs*vecSize)
	for i := 0; i < vecSize; i++ {
		for j := 0; j < numVecs; j++ {
			res[i*numVecs+j] = vecs[j][i]
		}
	}

	return res
}

// PowerVec allocates and returns a vector of size n consisting of consecutive
// powers of x, starting from x^0 = 1 and ending on x^{n-1}. The function panics
// if given x=0 and returns an empty vector if n=0.
func PowerVec(x fext.Element, n int) Vector {
	if x.IsZero() {
		utils.Panic("cannot build a power vec for x=0")
	}

	if n == 0 {
		return Vector{}
	}

	res := make(Vector, n)
	parallel.Execute(n, func(start, stop int) {
		res[start].ExpInt64(x, int64(start))
		for i := start + 1; i < stop; i++ {
			res[i].Mul(&res[i-1], &x)
		}
	})

	return res
}

// IntoGnarkAssignment converts an array of field.Element into an array of
// koalagnark.Var that can be used to assign a vector of koalagnark.Var
// in a circuit or to generate a vector of constant in the circuit definition.
func IntoGnarkAssignment(msgData Vector) []koalagnark.Ext {
	assignedMsg := []koalagnark.Ext{}
	for _, x := range msgData {
		assignedMsg = append(assignedMsg, koalagnark.NewExt(x))
	}
	return assignedMsg
}

// PseudoRand generates a vector of field element with a given size using the
// provided random number generator
func PseudoRand(rng *rand.Rand, size int) Vector {
	slice := make(Vector, size)
	for i := range slice {
		slice[i] = fext.PseudoRand(rng)
	}
	return slice
}
