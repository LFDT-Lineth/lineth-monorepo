package poly

import (
	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// EvalUnivariate evaluates a univariate polynomial `pol` given as a vector of
// coefficients. Coefficients are for increasing degree monomials: meaning that
// pol[0] is the constant term and pol[len(pol) - 1] is the highest degree term.
// The evaluation is done using the Horner method.
//
// If the empty slice is provided, it is understood as the zero polynomial and
// the function returns zero.
func EvalUnivariate(pol []field.Element, x field.Element) field.Element {
	var res field.Element
	for i := len(pol) - 1; i >= 0; i-- {
		res.Mul(&res, &x)
		res.Add(&res, &pol[i])
	}
	return res
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
// side the function returns []field.Element{}
func Mul(a, b []field.Element) (res []field.Element) {

	if len(a) == 0 || len(b) == 0 {
		return []field.Element{}
	}

	res = make([]field.Element, len(a)+len(b)-1)

	for i := 0; i < len(a); i++ {
		for j := 0; j < len(b); j++ {
			var tmp field.Element
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
func Add(a, b []field.Element) (res []field.Element) {

	res = make([]field.Element, utils.Max(len(a), len(b)))
	copy(res, a)
	for i := range b {
		res[i].Add(&res[i], &b[i])
	}

	return res
}

// ScalarMul multiplies a polynomials in coefficient form by a scalar.
func ScalarMul(p []field.Element, x field.Element) (res []field.Element) {
	res = make([]field.Element, len(p))
	vector.ScalarMul(res, p, x)
	return res
}

// EvaluateLagrangesAnyDomain evaluates all the Lagrange polynomials for a
// custom domain defined as the point point x. The function implements the naive
// schoolbook algorithm and is only relevant for small domains.
//
// The function panics if provided an empty domain.
func EvaluateLagrangesAnyDomain(domain []field.Element, x field.Element) []field.Element {

	if len(domain) == 0 {
		utils.Panic("got provided an empty domain")
	}

	lagrange := make([]field.Element, len(domain))

	for i := range domain {
		// allocate outside of the loop to avoid memory aliasing in for loop
		// (gosec G601)
		hi := domain[i]

		lhix := field.One()
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

// GetHornerTrace computes a random Horner accumulation of the filtered elements
// starting from the last entry down to the first entry. The final value is
// stored in the last entry of the returned slice.
func GetHornerTrace(c, fC []field.Element, x field.Element) []field.Element {

	var (
		horner = make([]field.Element, len(c))
		prev   field.Element
	)

	for i := len(horner) - 1; i >= 0; i-- {

		if !fC[i].IsZero() && !fC[i].IsOne() {
			utils.Panic("we expected the filter to be binary")
		}

		if fC[i].IsOne() {
			prev.Mul(&prev, &x)
			prev.Add(&prev, &c[i])
		}

		horner[i] = prev
	}

	return horner
}
