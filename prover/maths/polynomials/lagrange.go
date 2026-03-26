package polynomials

import (
	"math/big"
	"math/bits"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
)

// GnarkEvaluateLagrangeExt evaluates a polynomial in Lagrange basis at point z.
// Uses the barycentric formula: P(z) = (zⁿ - 1) * Σᵢ [ (ωⁱ/n * pᵢ) / (z - ωⁱ) ]
// This requires only one MulExt for the final (zⁿ - 1) * sum multiplication.
func GnarkEvaluateLagrangeExt(api frontend.API, p []koalagnark.Ext, z koalagnark.Ext, gen field.Element, cardinality uint64) koalagnark.Ext {
	koalaAPI := koalagnark.NewAPI(api)
	if cardinality == 0 {
		return koalaAPI.ZeroExt()
	}

	// Precompute barycentric weights: wᵢ = ωⁱ/n
	// and weighted coefficients: wᵢ * pᵢ
	var accOmega field.Element
	accOmega.SetOne()

	// Precompute 1/n as a constant
	var invN field.Element
	invN.SetUint64(cardinality)
	invN.Inverse(&invN)
	bwi := big.NewInt(0)

	// Compute zⁿ - 1
	tb := bits.TrailingZeros(uint(cardinality))
	zPowN := z
	for i := 0; i < tb; i++ {
		zPowN = koalaAPI.SquareExt(zPowN)
	}
	zPowNMinusOne := koalaAPI.SubExt(zPowN, koalaAPI.OneExt())

	// Compute Σᵢ [ (ωⁱ/n * pᵢ) / (z - ωⁱ) ]
	addends := make([]koalagnark.Ext, cardinality)
	for i := uint64(0); i < cardinality; i++ {
		// wᵢ = ωⁱ/n
		var wi field.Element
		wi.Mul(&accOmega, &invN)
		bwi.SetUint64(wi.Uint64())

		// weightedP = wᵢ * pᵢ
		weightedP := koalaAPI.MulConstExt(p[i], bwi)

		// z - ωⁱ
		wOmegai := koalagnark.NewElementFromKoala(accOmega)
		omegaiExt := koalaAPI.FromBaseExt(wOmegai)
		zMinusOmegai := koalaAPI.SubExt(z, omegaiExt)

		// weightedP / (z - ωⁱ)
		term := koalaAPI.DivExt(weightedP, zMinusOmegai)
		addends[i] = term

		accOmega.Mul(&accOmega, &gen)
	}
	if len(addends) == 1 { // case cardinality == 0 handled above
		return koalaAPI.MulExt(zPowNMinusOne, addends[0])
	}
	sum := koalaAPI.SumExt(addends...)

	// P(z) = (zⁿ - 1) * sum
	return koalaAPI.MulExt(zPowNMinusOne, sum)
}

// GnarkEvaluateLagrangeExtBatch evaluates the same polynomial p at multiple points zs
// more efficiently by sharing the barycentric weight computation.
//
// Uses the barycentric formula: P(z) = (zⁿ - 1) * Σᵢ [ (ωⁱ/n * pᵢ) / (z - ωⁱ) ]
//
// For k evaluation points, this saves approximately (k-1)*n multiplications
// compared to k separate calls to GnarkEvaluateLagrangeExt.
func GnarkEvaluateLagrangeExtBatch(api frontend.API, p []koalagnark.Ext, zs []koalagnark.Ext, gen field.Element, cardinality uint64) []koalagnark.Ext {
	if len(zs) == 0 {
		return nil
	}
	if len(zs) == 1 {
		return []koalagnark.Ext{GnarkEvaluateLagrangeExt(api, p, zs[0], gen, cardinality)}
	}

	koalaAPI := koalagnark.NewAPI(api)

	// Precompute barycentric weights: wᵢ = ωⁱ/n
	// and weighted coefficients: wᵢ * pᵢ
	// This is shared across all evaluation points
	weightedP := make([]koalagnark.Ext, cardinality)
	var accOmega field.Element
	accOmega.SetOne()

	// Precompute 1/n as a constant
	var invN field.Element
	invN.SetUint64(cardinality)
	invN.Inverse(&invN)
	bwi := big.NewInt(0)

	for i := uint64(0); i < cardinality; i++ {
		// wᵢ = ωⁱ/n
		var wi field.Element
		wi.Mul(&accOmega, &invN)
		bwi.SetUint64(wi.Uint64())

		// weightedP[i] = wᵢ * pᵢ
		weightedP[i] = koalaAPI.MulConstExt(p[i], bwi)

		accOmega.Mul(&accOmega, &gen)
	}

	// Precompute all ωⁱ values as constants
	omegaPowers := make([]field.Element, cardinality)
	omegaPowers[0].SetOne()
	for i := uint64(1); i < cardinality; i++ {
		omegaPowers[i].Mul(&omegaPowers[i-1], &gen)
	}

	results := make([]koalagnark.Ext, len(zs))
	tb := bits.TrailingZeros(uint(cardinality))

	// Evaluate at each point z
	for j, z := range zs {
		// Compute zⁿ - 1
		zPowN := z
		for i := 0; i < tb; i++ {
			zPowN = koalaAPI.SquareExt(zPowN)
		}
		zPowNMinusOne := koalaAPI.SubExt(zPowN, koalaAPI.OneExt())

		// Compute Σᵢ [ weightedP[i] / (z - ωⁱ) ]
		sum := koalaAPI.ZeroExt()
		for i := uint64(0); i < cardinality; i++ {
			// z - ωⁱ
			wOmegai := koalagnark.NewElementFromKoala(omegaPowers[i])
			omegaiExt := koalaAPI.FromBaseExt(wOmegai)
			zMinusOmegai := koalaAPI.SubExt(z, omegaiExt)

			// weightedP[i] / (z - ωⁱ)
			term := koalaAPI.DivExt(weightedP[i], zMinusOmegai)
			sum = koalaAPI.AddExt(sum, term)
		}

		// P(z) = (zⁿ - 1) * sum
		results[j] = koalaAPI.MulExt(zPowNMinusOne, sum)
	}

	return results
}

// computeLagrange returns Lᵢ(ζ) for i=1..n
// with lᵢ(ζ) = ωⁱ/n*(ζⁿ-1)/(ζ - ωⁱ)
func gnarkComputeLagrangeAtZ(api frontend.API, z koalagnark.Ext, gen field.Element, cardinality uint64) []koalagnark.Ext {

	res := make([]koalagnark.Ext, cardinality)
	tb := bits.TrailingZeros(uint(cardinality))

	koalaAPI := koalagnark.NewAPI(api)

	// ζⁿ-1
	res[0] = z
	for i := 0; i < tb; i++ {
		res[0] = koalaAPI.SquareExt(res[0])
	}
	wOne := koalaAPI.OneExt()
	res[0] = koalaAPI.SubExt(res[0], wOne)

	// ζ-1
	accZetaMinusOmegai := koalaAPI.SubExt(z, wOne)

	// (ζⁿ-1)/(ζ-1)
	res[0] = koalaAPI.DivExt(res[0], accZetaMinusOmegai)

	// 1/n*(ζⁿ-1)/(ζ-1)
	wCardinality := koalagnark.NewElement(cardinality)
	res[0] = koalaAPI.DivByBaseExt(res[0], wCardinality)

	// res[i] <- res[i-1] * (ζ-ωⁱ⁻¹)/(ζ-ωⁱ) * ω
	var accOmega field.Element
	accOmega.SetOne()
	wGen := big.NewInt(0).SetUint64(gen.Uint64())
	var wAccOmega koalagnark.Element
	for i := uint64(1); i < cardinality; i++ {
		res[i] = koalaAPI.MulConstExt(res[i-1], wGen)        // res[i] <- ω * res[i-1]
		res[i] = koalaAPI.MulExt(res[i], accZetaMinusOmegai) // res[i] <- res[i]*(ζ-ωⁱ⁻¹)
		accOmega.Mul(&accOmega, &gen)                        // accOmega <- accOmega * ω
		wAccOmega = koalagnark.NewElementFromKoala(accOmega)
		wAccOmegaExt := koalaAPI.FromBaseExt(wAccOmega)
		accZetaMinusOmegai = koalaAPI.SubExt(z, wAccOmegaExt) // accZetaMinusOmegai <- ζ-ωⁱ
		res[i] = koalaAPI.DivExt(res[i], accZetaMinusOmegai)  // res[i]  <- res[i]/(ζ-ωⁱ)
	}

	return res
}
