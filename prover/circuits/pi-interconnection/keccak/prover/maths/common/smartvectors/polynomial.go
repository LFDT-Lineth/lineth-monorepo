package smartvectors

import (
	"math/big"
	"sync/atomic"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/poly"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/fft"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/fft/fastpoly"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/parallel"
)

// Add two vectors representing polynomials in coefficient form.
// a and b may have different sizes
func PolyAdd(a, b SmartVector) SmartVector {

	small, large := a, b
	if a.Len() > b.Len() {
		small, large = large, small
	}

	res := make([]field.Element, large.Len())
	large.WriteInSlice(res)
	for i := 0; i < small.Len(); i++ {
		x := small.Get(i)
		res[i].Add(&res[i], &x)
	}

	return NewRegular(res)
}

func PolySub(a, b SmartVector) SmartVector {

	maxLen := utils.Max(a.Len(), b.Len())
	res := make([]field.Element, maxLen)
	a.WriteInSlice(res[:a.Len()])

	for i := 0; i < b.Len(); i++ {
		bi := b.Get(i)
		res[i].Sub(&res[i], &bi)
	}

	return NewRegular(res)
}

/*
Ruffini division
  - p polynomial in coefficient form
  - q field.Element, caracterizing the divisor X - q
  - quo quotient polynomial in coefficient form, result will be passed
    here. quo is truncated of its first entry in the process
  - expected to be at least as large as `p`

- rem, remainder also equals to P(r)

Supports &p == quo
*/
func RuffiniQuoRem(p SmartVector, q field.Element) (quo SmartVector, rem field.Element) {

	// The case where "p" is zero is assumed to be impossible as every type of
	// smart-vector strongly forbid dealing with zero length smart-vectors.
	if p.Len() == 0 {
		panic("Zero-length smart-vectors are forbidden")
	}

	// If p has length 1, then the general case algorithm does not work
	if p.Len() == 1 {
		quo = NewConstant(field.Zero(), 1)
		rem = p.Get(0)
		return quo, rem
	}

	quo_ := make([]field.Element, p.Len())

	// Pass the last coefficient
	quo_[p.Len()-1] = p.Get(p.Len() - 1)

	for i := p.Len() - 2; i >= 0; i-- {
		var c field.Element
		c.Mul(&quo_[i+1], &q)
		pi := p.Get(i)
		quo_[i].Add(&c, &pi)
	}

	// As we employ custom allocation, we should not pass x[1:]
	rem = quo_[0]
	quo = NewRegular(quo_[1:])

	return quo, rem
}

// Evaluate a polynomial in Lagrange basis
func Interpolate(v SmartVector, x field.Element, oncoset ...bool) field.Element {
	switch con := v.(type) {
	case *Constant:
		return con.Value
	}

	// Maybe there is an optim for windowed here
	res := make([]field.Element, v.Len())
	v.WriteInSlice(res)
	return fastpoly.Interpolate(res, x, oncoset...)
}

// Batch-evaluate polynomials in Lagrange basis
func BatchInterpolate(vs []SmartVector, x field.Element) []field.Element {

	var (
		polys         = make([][]field.Element, len(vs))
		results       = make([]field.Element, len(vs))
		computed      = make([]bool, len(vs))
		totalConstant = uint64(0)
	)

	// smartvector to []fr.element
	parallel.Execute(len(vs), func(start, stop int) {
		for i := start; i < stop; i++ {
			switch con := vs[i].(type) {
			case *Constant:
				// constant vectors
				results[i] = con.Value
				computed[i] = true
				atomic.AddUint64(&totalConstant, 1)
				continue
			}

			// non-constant vectors
			polys[i], _ = vs[i].IntoRegVecSaveAllocBase()
		}
	})

	if int(totalConstant) == len(vs) {
		return results
	}

	return batchInterpolateSV(results, computed, polys, x)
}

// Optimized batch interpolate for smart vectors.
// This reduces the number of computation by pre-processing
// constant vectors in advance in BatchInterpolate()
func batchInterpolateSV(results []field.Element, computed []bool, polys [][]field.Element, x field.Element) []field.Element {

	n := 0
	for i := range polys {
		if len(polys[i]) > 0 {
			n = len(polys[i])
		}
	}

	if n == 0 {
		// that's a possible edge-case and it can happen if all the input polys
		// are constant smart-vectors. This is should be prevented by the the
		// caller.
		return results
	}

	if !utils.IsPowerOfTwo(n) {
		utils.Panic("only support powers of two but poly has length %v", len(polys))
	}

	domain := fft.NewDomain(n)
	denominator := make([]field.Element, n)

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

	parallel.Execute(n, func(start, stop int) {
		one := field.One()
		for i := start; i < stop; i++ {
			denominator[i].Sub(&denominator[i], &one)
		}
	})
	for i := 0; i < n; i++ {
		if denominator[i].IsZero() {
			// edge-case : x is a root of unity of the domain. In this case, we can just return
			// the associated value for poly
			for k := range polys {
				if computed[k] {
					continue
				}
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
	denominator = field.ParBatchInvert(denominator, 0)

	// Precompute the value of x^n once outside the loop
	xN := new(field.Element).Exp(x, big.NewInt(int64(n)))

	// Precompute the value of domain.CardinalityInv outside the loop
	cardinalityInv := &domain.CardinalityInv

	// Compute factor as (x^n - 1) * (1 / domain.Cardinality).
	one := field.One()
	factor := new(field.Element).Sub(xN, &one)
	factor.Mul(factor, cardinalityInv)

	parallel.Execute(len(polys), func(start, stop int) {
		for k := start; k < stop; k++ {

			if computed[k] {
				continue
			}
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

// Evaluate a polynomial in coefficient basis
func EvalCoeff(v SmartVector, x field.Element) field.Element {
	// Maybe there is an optim for windowed here
	res := make([]field.Element, v.Len())
	v.WriteInSlice(res)
	return poly.EvalUnivariate(res, x)
}

func EvalCoeffBivariate(v SmartVector, x field.Element, numCoeffX int, y field.Element) field.Element {

	if v.Len()%numCoeffX != 0 {
		panic("size of v and nb coeff x are inconsistent")
	}

	// naive evaluation : we think it is not performance critical
	slice := make([]field.Element, v.Len())
	v.WriteInSlice(slice)

	foldOnX := make([]field.Element, len(slice)/numCoeffX)
	for i := 0; i < len(slice); i += numCoeffX {
		foldOnX[i/numCoeffX] = poly.EvalUnivariate(slice[i:i+numCoeffX], x)
	}

	return poly.EvalUnivariate(foldOnX, y)
}
