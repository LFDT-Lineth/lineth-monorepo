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
