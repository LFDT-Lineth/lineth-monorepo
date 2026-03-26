package fastpolyext

import (
	"math/big"
	"slices"

	"github.com/consensys/gnark-crypto/field/koalabear/fft"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/gnarkutil"
)

// EvaluateLagrangeGnark evaluates a polynomial in lagrange basis on a gnark circuit
func EvaluateLagrangeGnark(api frontend.API, poly []koalagnark.Ext, x koalagnark.Ext) koalagnark.Ext {

	if !utils.IsPowerOfTwo(len(poly)) {
		utils.Panic("only support powers of two but poly has length %v", len(poly))
	}

	koalaAPI := koalagnark.NewAPI(api)

	size := len(poly)
	omega, err := fft.Generator(uint64(size))
	if err != nil {
		// TODO handle that properly
		panic(err)
	}

	dens := make([]koalagnark.Ext, size)

	omega.Inverse(&omega)
	wInvOmega := koalagnark.NewFromBaseExt(omega.String())
	dens[0] = x
	for i := 1; i < size; i++ {
		dens[i] = koalaAPI.MulExt(dens[i-1], wInvOmega)
	}

	wOne := koalaAPI.OneExt()
	for i := 0; i < size; i++ {
		dens[i] = koalaAPI.SubExt(dens[i], wOne)
		dens[i] = koalaAPI.InverseExt(dens[i])
	}

	res := koalaAPI.ZeroExt()
	for i := 0; i < size; i++ {
		tmp := koalaAPI.MulExt(dens[i], poly[i])
		res = koalaAPI.AddExt(res, tmp)
	}

	var tmp koalagnark.Ext
	tmp = gnarkutil.ExpExt(api, x, size)
	tmp = koalaAPI.SubExt(tmp, wOne) // xⁿ-1
	var invSize field.Element
	invSize.SetUint64(uint64(size)).Inverse(&invSize)
	bInvSize := big.NewInt(0).SetUint64(invSize.Uint64())
	tmp = koalaAPI.MulConstExt(tmp, bInvSize)
	res = koalaAPI.MulExt(res, tmp)

	return res

}

// BatchEvaluateLagrangeGnark evaluates a batch of polynomials in Lagrange basis
// on a gnark circuit. The evaluation point is common to all polynomials.
// The implementation relies on the barycentric interpolation formula and
// leverages
func BatchEvaluateLagrangeGnark(api frontend.API, polys [][]*koalagnark.Ext, x koalagnark.Ext) []koalagnark.Ext {

	if len(polys) == 0 {
		return []koalagnark.Ext{}
	}

	var (
		koalaAPI         = koalagnark.NewAPI(api)
		sizes            = listOfDifferentSizes(polys)
		maxSize          = sizes[len(sizes)-1]
		xNs              = raiseToPowersOfTwosExt(api, x, sizes)
		powersOfOmegaInv = powerVectorOfOmegaInv(maxSize)

		// innerProductTerms lists a sequence of term computed as
		// \frac{1}{x \omega^{-i} - 1}. These will be used to compute
		// an inner-product between the polys to obtain an unscaled
		// result. The unscaled result will then be scaled by (x^n - 1)/n
		// to obtain the result of the Lagrange interpolation.
		//
		// These are lazily evaluated. innerProductTermAssigned indicates whether
		// a term has been evaluated.
		innerProductTerms = make([]koalagnark.Ext, maxSize)
		// innerProductTermAssigned indicates whether a term has been evaluated.
		innerProductTermAssigned = make([]bool, maxSize)

		// scalingTerms lists a sequence of term computed as (x^n - 1)/n
		// where n stands for all the different sizes of the polys in
		// increasing order. (as found in "sizes"). The used to derive
		// the final result of the evaluations.
		scalingTerms = make([]koalagnark.Ext, len(sizes))

		// res stores the final result of the interpolation
		res   = make([]koalagnark.Ext, len(polys))
		e4one = koalaAPI.OneExt()
	)

	// getProductMultiplierLazy is a helper function to lazily evaluate the scaling
	// terms "si = 1 / (x - w^i)" and store them in [scalingTerms] when they are
	// requested by the evaluation loop. As this is an expensive operation, we
	// can save computes when the polys are sparsely assigned (e.g) they contain
	// a lot of zeroes.
	getProductMultiplierLazy := func(i int) koalagnark.Ext {

		if innerProductTermAssigned[i] {
			return innerProductTerms[i]
		}

		omegaInv := big.NewInt(0)
		omegaInv.SetUint64(powersOfOmegaInv[i])
		innerProductTerms[i] = koalaAPI.MulConstExt(x, omegaInv)
		innerProductTerms[i] = koalaAPI.SubExt(innerProductTerms[i], e4one)
		innerProductTerms[i] = koalaAPI.InverseExt(innerProductTerms[i])
		innerProductTermAssigned[i] = true
		return innerProductTerms[i]
	}

	for i, n := range sizes {
		wn := koalagnark.NewElement(n)
		scalingTerms[i] = koalaAPI.SubExt(xNs[i], e4one)
		scalingTerms[i] = koalaAPI.DivByBaseExt(scalingTerms[i], wn)
	}

	for i := range polys {

		var (
			poly          = polys[i]
			n             = len(poly)
			scalingFactor koalagnark.Ext
			// A fundamental optimization that we can do is to compute the inner
			// product in a native field. Since, this is a bilinear operation it
			// incure a an overflow of log(n) + 32 bits. But the field has
			// plenty of space to hold the result of the inner product and let
			// us reduce at the end only. The way we do it is that we accumulate
			// the pairwise product for all the non-trivial terms (e.g.
			// excluding the positions where poly[k] is a constant equal to zero
			// ) and then we sum at once.
			productTerms = []koalagnark.Ext{}
		)

		for j := range sizes {
			if sizes[j] == n {
				scalingFactor = scalingTerms[j]
			}
		}

		if scalingFactor.B0.A0.IsEmpty() {
			utils.Panic("could not find scaling factor for poly of size %v", n)
		}

		var (
			// This is a caching mechanism to avoid calling [ConstantValueOfExt]
			// as much as possible as this is a massive slow down during the
			// compilation of the circuit. Although, this is without impact on
			// the total number of constraints of the circuit.
			lastSeenZeroPtr *koalagnark.Ext
		)

		for k := 0; k < n; k++ {

			if poly[k] == lastSeenZeroPtr {
				continue
			}

			// This saves constraints when the constant term is zero.
			if koalaAPI.IsConstantZeroExt(*poly[k]) {
				lastSeenZeroPtr = poly[k]
				continue
			}

			productMultiplier := getProductMultiplierLazy(k * maxSize / n)

			// this saves constraints when the provided elements represent a
			// base field column (when statically known).
			if polyK, isBase := koalaAPI.BaseValueOfElement(*poly[k]); isBase {
				productTerm := koalaAPI.MulByFpExt(productMultiplier, *polyK)
				productTerms = append(productTerms, productTerm)
				continue
			}

			productTerm := koalaAPI.MulExt(productMultiplier, *poly[k])
			productTerms = append(productTerms, productTerm)
		}

		yUnscaled := koalaAPI.SumExt(productTerms...)
		res[i] = koalaAPI.MulExt(yUnscaled, scalingFactor)
	}

	return res
}

// listOfDifferentSizes returns the list of sizes of the different polys
// vectors. The returned slices is deduplicated and sorted in ascending order.
func listOfDifferentSizes(polys [][]*koalagnark.Ext) []int {

	sizes := make([]int, 0, len(polys))
	for _, poly := range polys {
		sizes = append(sizes, len(poly))
	}

	slices.Sort(sizes)
	sizes = slices.Compact(sizes)
	sizes = slices.Clip(sizes)
	return sizes
}

// raiseToPowersOfTwosExt returns x raised to the powers contained in ns. For
// instance, if ns = [2, 4, 8], then it returns [x, x^2, x^4, x^8]. It assumes
// that ns is sorted in ascending order and deduplicated and non-zero and are
// powers of two.
func raiseToPowersOfTwosExt(api frontend.API, x koalagnark.Ext, ns []int) []koalagnark.Ext {

	res := make([]koalagnark.Ext, len(ns))
	curr := x
	currN := 1

	koalaAPI := koalagnark.NewAPI(api)

	for i, n := range ns {

		if n <= 0 || !utils.IsPowerOfTwo(n) {
			utils.Panic("ns should be sorted in ascending order and deduplicated and non-zero and be powers of two, was %v", n)
		}

		for currN < n {
			curr = koalaAPI.SquareExt(curr)
			currN *= 2
		}

		res[i] = curr
	}

	return res
}

// powerVector returns w raised to the powers contained up to n starting from
// 0. For instance, if n = 4, then it returns [1, w^-1, w^-2, w^-3]. Where w
// is the inverse of the generator of the subgroup of root of unity of order 4.
func powerVectorOfOmegaInv(n int) []uint64 {

	var (
		resField = field.One()
		res      = make([]uint64, n)
		w, _     = fft.Generator(uint64(n))
	)

	w.Inverse(&w)

	for i := 0; i < n; i++ {
		res[i] = uint64(resField.Uint64())
		resField.Mul(&resField, &w)
	}

	return res
}
