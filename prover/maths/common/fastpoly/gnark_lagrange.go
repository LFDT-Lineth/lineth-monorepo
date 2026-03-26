package fastpoly

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

// EvaluateLagrangeGnarkMixed a polynomial in lagrange basis on a gnark circuit
func EvaluateLagrangeGnarkMixed(api frontend.API, poly []koalagnark.Element, x koalagnark.Ext) koalagnark.Ext {

	koalaAPI := koalagnark.NewAPI(api)

	if !utils.IsPowerOfTwo(len(poly)) {
		utils.Panic("only support powers of two but poly has length %v", len(poly))
	}

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
		tmp := koalaAPI.MulByFpExt(dens[i], poly[i])
		res = koalaAPI.AddExt(res, tmp)
	}

	var tmp koalagnark.Ext
	tmp = gnarkutil.ExpExt(api, x, size)
	tmp = koalaAPI.SubExt(tmp, wOne) // xⁿ-1
	var invSize field.Element
	invSize.SetUint64(uint64(size)).Inverse(&invSize)
	wInvSize := koalagnark.NewElementFromKoala(invSize)
	tmp = koalaAPI.MulByFpExt(tmp, wInvSize)
	res = koalaAPI.MulExt(res, tmp)

	return res

}

// BatchEvaluateLagrangeGnarkMixed evaluates a batch of polynomials in Lagrange basis
// on a gnark circuit. The evaluation point is common to all polynomials.
// The implementation relies on the barycentric interpolation formula and
// leverages
func BatchEvaluateLagrangeGnarkMixed(api frontend.API, polys [][]koalagnark.Element, x koalagnark.Ext) []koalagnark.Ext {
	if len(polys) == 0 {
		return []koalagnark.Ext{}
	}

	koalaAPI := koalagnark.NewAPI(api)

	var (
		sizes            = listOfDifferentSizes(polys)
		maxSize          = sizes[len(sizes)-1]
		xNs              = raiseToPowersOfTwosExt(api, x, sizes)
		powersOfOmegaInv = powerVectorOfOmegaInv(maxSize)

		// innerProductTerms lists a sequence of term computed as
		// \frac{1}{x \omega^{-i} - 1}. These will be used to compute
		// an inner-product between the polys to obtain an unscaled
		// result. The unscaled result will then be scaled by (x^n - 1)/n
		// to obtain the result of the Lagrange interpolation.
		innerProductTerms = make([]koalagnark.Ext, maxSize)

		// scalingTerms lists a sequence of term computed as (x^n - 1)/n
		// where n stands for all the different sizes of the polys in
		// increasing order. (as found in "sizes"). The used to derive
		// the final result of the evaluations.
		scalingTerms = make([]koalagnark.Ext, len(sizes))

		// res stores the final result of the interpolation
		res = make([]koalagnark.Ext, len(polys))
	)
	e4one := koalaAPI.OneExt()
	omegaInv := big.NewInt(0)
	for i := range innerProductTerms {
		omegaInv.SetUint64(powersOfOmegaInv[i])
		innerProductTerms[i] = koalaAPI.MulConstExt(x, omegaInv)
		innerProductTerms[i] = koalaAPI.SubExt(innerProductTerms[i], e4one)
		innerProductTerms[i] = koalaAPI.InverseExt(innerProductTerms[i])
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
			yUnscaled     koalagnark.Ext
			tmp           koalagnark.Ext
		)

		for j := range sizes {
			if sizes[j] == n {
				scalingFactor = scalingTerms[j]
			}
		}

		// That would be completely unexpectedly wrong if that happened since
		// the sizes are deduplicated and sorted from the sizes found in poly
		// in the first place.
		if scalingFactor.B0.A0.IsEmpty() {
			utils.Panic("could not find scaling factor for poly of size %v", n)
		}

		yUnscaled = koalaAPI.ZeroExt()

		for k := 0; k < n; k++ {

			// This optimization saves constraints when dealing with a polynomial
			// that is precomputed and raised as a Proof object in the wizard. When
			// that happens, it will contain constant values (very often actually)
			// and when they contain zeroes we can just skip the related
			// computation. The saving was not negligible in practice.
			if isConstantZeroGnarkVariable(api, poly[k]) {
				continue
			}

			tmp = koalaAPI.MulByFpExt(innerProductTerms[k*maxSize/n], poly[k])
			yUnscaled = koalaAPI.AddExt(tmp, yUnscaled)

		}

		res[i] = koalaAPI.MulExt(yUnscaled, scalingFactor)
	}

	return res
}

// listOfDifferentSizes returns the list of sizes of the different polys
// vectors. The returned slices is deduplicated and sorted in ascending order.
func listOfDifferentSizes(polys [][]koalagnark.Element) []int {

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

	var (
		res   = make([]koalagnark.Ext, len(ns))
		curr  = x
		currN = 1
	)

	koalaAPI := koalagnark.NewAPI(api)

	for i, n := range ns {

		if n <= 0 || !utils.IsPowerOfTwo(n) {
			utils.Panic("ns should be sorted in ascending order and deduplicated and non-zero and be powers of two, was %v", n)
		}

		for currN < n {
			curr = koalaAPI.SquareExt(curr) // Use Square instead of Mul for self-multiplication
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

// isConstantZeroGnarkVariable returns true if the variable is a constant equal to zero
func isConstantZeroGnarkVariable(api frontend.API, p koalagnark.Element) bool {
	c, isC := api.Compiler().ConstantValue(p.Native())
	return isC && c.IsInt64() && c.Int64() == 0
}
