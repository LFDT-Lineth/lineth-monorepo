package fastpoly

import (
	"math/big"

	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/fft"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/parallel"
)

/*
Given the evaluations of a polynomial on a domain (whose
size must be a power of two, or panic), return an evaluation
at a chosen x.

As an input the user can specify that the inputs are given
on a coset.
*/
func Interpolate(poly []field.Element, x fr.Element, oncoset ...bool) field.Element {

	if !utils.IsPowerOfTwo(len(poly)) {
		utils.Panic("only support powers of two but poly has length %v", len(poly))
	}

	n := len(poly)

	domain := fft.NewDomain(n)
	denominator := make([]field.Element, n)

	one := field.One()

	if len(oncoset) > 0 && oncoset[0] {
		x.Mul(&x, &domain.FrMultiplicativeGenInv)
	}

	/*
		First, we compute the denominator,

		D_x = \frac{X}{x} - g for x \in H
			where H is the subgroup of the roots of unity (not the coset)
			and g a field element such that gH is the coset
	*/
	denominator[0] = x
	for i := 1; i < n; i++ {
		denominator[i].Mul(&denominator[i-1], &domain.GeneratorInv)
	}

	for i := 0; i < n; i++ {
		denominator[i].Sub(&denominator[i], &one)
		if denominator[i].IsZero() {
			// edge-case : x is a root of unity of the domain. In this case, we can just return
			// the associated value for poly
			return poly[i]
		}
	}

	/*
		Then, we compute the sum between the inverse of the denominator
		and the poly

		\sum_{x \in H}\frac{P(gx)}{D_x}
	*/
	denominator = field.ParBatchInvert(denominator, 8)
	res := vector.ScalarProd(poly, denominator)

	/*
		Then multiply the res by a factor \frac{g^{1 - n}X^n -g}{n}
	*/
	var factor field.Element
	factor.Exp(x, big.NewInt(int64(n)))
	factor.Sub(&factor, &one)
	factor.Mul(&factor, &domain.CardinalityInv)
	res.Mul(&res, &factor)

	return res

}

// Batch version of Interpolate
func BatchInterpolate(polys [][]field.Element, x fr.Element, oncoset ...bool) []field.Element {

	results := make([]field.Element, len(polys))
	poly := polys[0]

	if !utils.IsPowerOfTwo(len(poly)) {
		utils.Panic("only support powers of two but poly has length %v", len(poly))
	}

	n := len(poly)

	domain := fft.NewDomain(n)
	denominator := make([]field.Element, n)

	one := field.One()

	if len(oncoset) > 0 && oncoset[0] {
		x.Mul(&x, &domain.FrMultiplicativeGenInv)
	}

	/*
		First, we compute the denominator,

		D_x = \frac{X}{x} - g for x \in H
			where H is the subgroup of the roots of unity (not the coset)
			and g a field element such that gH is the coset
	*/
	denominator[0] = x
	for i := 1; i < n; i++ {
		denominator[i].Mul(&denominator[i-1], &domain.GeneratorInv)
	}

	for i := 0; i < n; i++ {
		denominator[i].Sub(&denominator[i], &one)

		if denominator[i].IsZero() {
			// edge-case : x is a root of unity of the domain. In this case, we can just return
			// the associated value for poly

			for k := range polys {
				results[k] = polys[k][i]
			}

			return results
		}
	}

	/*
		Then, we compute the sum between the inverse of the denominator
		and the poly

		\sum_{x \in H}\frac{P(gx)}{D_x}
	*/
	denominator = field.ParBatchInvert(denominator, 8)

	// Precompute the value of x^n once outside the loop
	xN := new(field.Element).Exp(x, big.NewInt(int64(n)))

	// Precompute the value of domain.CardinalityInv outside the loop
	cardinalityInv := &domain.CardinalityInv

	// Compute factor as (x^n - 1) * (1 / domain.Cardinality).
	factor := new(field.Element).Sub(xN, &one)
	factor.Mul(factor, cardinalityInv)

	parallel.Execute(len(polys), func(start, stop int) {
		for k := start; k < stop; k++ {

			// Compute the scalar product.
			res := vector.ScalarProd(polys[k], denominator)

			// Multiply res with factor.
			res.Mul(&res, factor)

			// Store the result.
			results[k] = res
		}
	})

	return results
}
