package poly

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
)

// EvaluateUnivariateGnarkExt evaluate a univariate polynomial in a gnark circuit.
// It mirrors [EvalUnivariate].
func EvaluateUnivariateGnarkExt(api frontend.API, pol []koalagnark.Ext, x koalagnark.Ext) koalagnark.Ext {

	koalaAPI := koalagnark.NewAPI(api)

	res := koalaAPI.ZeroExt()
	for i := len(pol) - 1; i >= 0; i-- {
		res = koalaAPI.MulExt(res, x)
		res = koalaAPI.AddExt(res, pol[i])
	}
	return res
}
