package polyext

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext/gnarkfext"
)

// EvaluateLagrangeAnyDomainGnark mirrors [EvaluateLagrangesAnyDomain] but in
// a gnark circuit. The same usage precautions applies for it.
func EvaluateLagrangeAnyDomainGnark(api frontend.API, domain []gnarkfext.Variable, x gnarkfext.Variable) []gnarkfext.Variable {

	outerAPI := gnarkfext.API{Inner: api}
	lagrange := make([]gnarkfext.Variable, len(domain))

	for i, hi := range domain {
		lhix := gnarkfext.Variable{field.One(), field.Zero()}
		for j, hj := range domain {
			if i == j {
				// Skip it
				continue
			}
			// more convenient to store -h instead of h
			factor := outerAPI.Sub(x, hj)
			den := outerAPI.Sub(hi, hj) // so x - h
			den = outerAPI.Inverse(den)

			// accumulate the product
			lhix = outerAPI.Mul(lhix, factor, den)
		}
		lagrange[i] = lhix
	}

	return lagrange

}

// EvaluateUnivariateGnark evaluate a univariate polynomial in a gnark circuit.
// It mirrors [EvalUnivariate].
func EvaluateUnivariateGnark(api frontend.API, pol []gnarkfext.Variable, x gnarkfext.Variable) gnarkfext.Variable {
	res := gnarkfext.Variable{frontend.Variable(0), frontend.Variable(0)}
	outerAPI := gnarkfext.API{Inner: api}
	for i := len(pol) - 1; i >= 0; i-- {
		res = outerAPI.Mul(res, x)
		res = outerAPI.Add(res, pol[i])
	}
	return res
}
