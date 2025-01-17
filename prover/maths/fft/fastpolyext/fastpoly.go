package fastpolyext

import (
	"fmt"
	"github.com/consensys/linea-monorepo/prover/maths/common/vectorext"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"

	"github.com/consensys/linea-monorepo/prover/maths/fft"
)

// Multiply twi polynomial modulo X^n - 1
// a and b must be in coefficient form, the result is in coefficient
// form
// a and b are destroyed during the operation
func MultModXMinus1(domain *fft.Domain, res, a, b []fext.Element) {
	// All the item must be of the right size
	if len(a) != len(b) || len(a) != len(res) || uint64(len(a)) != domain.Cardinality {
		panic(
			fmt.Sprintf("All items should have the right size %v %v %v %v",
				domain.Cardinality, len(res), len(a), len(b)),
		)
	}

	domain.FFTExt(a, fft.DIF)
	domain.FFTExt(b, fft.DIF)
	vectorext.MulElementWise(res, a, b)
	domain.FFTInverseExt(res, fft.DIT)

}

// Multiply two polynomials modulo X^n - 1
// `a` must be in coefficient form
// `precomp` must be in evaluation form over the domain (DIT)
// `res` can be either `a` or any pre-allocated array
// res must pre-allocated in all cases
// a is destroyed during the operation
func MultModXnMinus1Precomputed(domain *fft.Domain, res, a, precomp []fext.Element) {

	// All the item must be of the right size
	if len(a) != len(precomp) || len(a) != len(res) || uint64(len(a)) != domain.Cardinality {
		panic(
			fmt.Sprintf("All items should have the right size %v %v %v %v",
				domain.Cardinality, len(res), len(a), len(precomp)),
		)
	}

	domain.FFTExt(a, fft.DIF)
	vectorext.MulElementWise(res, a, precomp)
	domain.FFTInverseExt(res, fft.DIT)
}
