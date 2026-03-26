package gnarkutilext

import (
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field/fext/gnarkfext"
)

/*
exponentiation in gnark circuit, using the fast exponentiation
*/
func Exp(api gnarkfext.API, x gnarkfext.Variable, n int) gnarkfext.Variable {

	if n < 0 {
		x = api.Inverse(x)
		return Exp(api, x, -n)
	}

	if n == 0 {
		return gnarkfext.One()
	}

	if n == 1 {
		return x
	}

	if n%2 == 0 {
		x2 := api.Mul(x, x)
		return Exp(api, x2, n/2)
	}

	if n%2 == 1 {
		x2 := api.Mul(x, x)
		res := Exp(api, x2, (n-1)/2)
		return api.Mul(res, x)
	}

	panic("unreachable")
}
