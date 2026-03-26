package polyext

import (
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/vectorext"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

// EvalUnivariate evaluates a univariate polynomial `pol` given as a vector of
// coefficients. Coefficients are for increasing degree monomials: meaning that
// pol[0] is the constant term and pol[len(pol) - 1] is the highest degree term.
// The evaluation is done using the Horner method.
//
// If the empty slice is provided, it is understood as the zero polynomial and
// the function returns zero.
func EvalUnivariate(pol []fext.Element, x fext.Element) fext.Element {
	var res fext.Element
	for i := len(pol) - 1; i >= 0; i-- {
		res.Mul(&res, &x)
		res.Add(&res, &pol[i])
	}
	return res
}

func EvalUnivariateBase(pol []fext.Element, x field.Element) fext.Element {
	wrappedX := fext.Element{x, field.Zero()}
	return EvalUnivariate(pol, wrappedX)
}

// Mul multiplies two polynomials expressed by their coefficients using the
// naive method and writes the result in res. `a` and `b` may have distinct
// degrees and the result is returned in a slice of size len(a) + len(b) - 1.
//
// The algorithm is the schoolbook algorithm and runs in O(n^2). The usage of
// this usage should be reserved to reasonnably small polynomial. Otherwise,
// FFT methods should be preferred to this end.
//
// The empty slice is understood as the zero polynomial. If provided on either
// side the function returns []fext.Element{}
func Mul(a, b []fext.Element) (res []fext.Element) {

	if len(a) == 0 || len(b) == 0 {
		return []fext.Element{}
	}

	res = make([]fext.Element, len(a)+len(b)-1)

	for i := 0; i < len(a); i++ {
		for j := 0; j < len(b); j++ {
			var tmp fext.Element
			tmp.Mul(&a[i], &b[j])
			res[i+j].Add(&res[i+j], &tmp)
		}
	}

	return res
}

// Add adds two polynomials in coefficient form of possibly distinct degree.
// The returned slice has length = max(len(a), len(b)).
// The empty slice is understood as the zero polynomial and if both a and b are
// empty, the function returns the empty slice.
func Add(a, b []fext.Element) (res []fext.Element) {

	res = make([]fext.Element, utils.Max(len(a), len(b)))
	copy(res, a)
	for i := range b {
		res[i].Add(&res[i], &b[i])
	}

	return res
}

// ScalarMul multiplies a polynomials in coefficient form by a scalar.
func ScalarMul(p []fext.Element, x fext.Element) (res []fext.Element) {
	res = make([]fext.Element, len(p))
	vectorext.ScalarMul(res, p, x)
	return res
}

// EvaluateLagrangesAnyDomain evaluates all the Lagrange polynomials for a
// custom domain defined as the point point x. The function implements the naive
// schoolbook algorithm and is only relevant for small domains.
//
// The function panics if provided an empty domain.
func EvaluateLagrangesAnyDomain(domain []fext.Element, x fext.Element) []fext.Element {

	if len(domain) == 0 {
		utils.Panic("got provided an empty domain")
	}

	lagrange := make([]fext.Element, len(domain))

	for i := range domain {
		// allocate outside of the loop to avoid memory aliasing in for loop
		// (gosec G601)
		hi := domain[i]

		lhix := fext.One()
		for j := range domain {
			// allocate outside of the loop to avoid memory aliasing in for loop
			// (gosec G601)
			hj := domain[j]

			if i == j {
				// Skip it
				continue
			}

			// Otherwise, it would divide by zeri
			if hi == hj {
				utils.Panic("the domain contained a duplicate %v (at %v and %v)", hi.String(), i, j)
			}

			// more convenient to store -h instead of h
			hj.Neg(&hj)
			factor := x
			factor.Add(&factor, &hj)
			hj.Add(&hi, &hj) // so x - h
			hj.Inverse(&hj)
			factor.Mul(&factor, &hj)

			lhix.Mul(&lhix, &factor)
		}
		lagrange[i] = lhix
	}

	return lagrange
}
