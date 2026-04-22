package polynomials

import (
	"math/bits"

	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
)

// EvalLagrange evaluates a polynomial given in Lagrange (evaluation) basis at
// point z using the barycentric formula:
//
//	P(z) = (zⁿ - 1) · Σᵢ [ (ωⁱ/n · p[i]) / (z - ωⁱ) ]
//
// The subgroup size n = len(p) must be a power of two not exceeding 2^MaxOrderRoot.
// z must not be a root of unity.
func EvalLagrange(p field.Vec, z field.Gen) field.Gen {
	if p.Len() == 0 {
		return field.ElemZero()
	}
	gen := field.RootOfUnityBy(p.Len())
	switch {
	case p.IsBase() && z.IsBase():
		return field.ElemFromBase(evalLagrangeBaseBase(p.AsBase(), z.AsBase(), gen))
	case p.IsBase():
		return field.ElemFromExt(evalLagrangeBaseExt(p.AsBase(), z.AsExt(), gen))
	case z.IsBase():
		return field.ElemFromExt(evalLagrangeExtBase(p.AsExt(), z.AsBase(), gen))
	default:
		return field.ElemFromExt(evalLagrangeExtExt(p.AsExt(), z.AsExt(), gen))
	}
}

// EvalLagrangeBatch evaluates the same Lagrange polynomial p at multiple points
// zs, sharing the precomputed barycentric weight vector wᵢ·p[i] and ω^i table.
//
// For k evaluation points this saves approximately (k-1)·n multiplications
// compared to k individual calls to [EvalLagrange].
func EvalLagrangeBatch(p field.Vec, zs []field.Gen) []field.Gen {
	if len(zs) == 0 {
		return nil
	}
	if len(zs) == 1 {
		return []field.Gen{EvalLagrange(p, zs[0])}
	}
	n := p.Len()
	gen := field.RootOfUnityBy(n)
	tb := bits.TrailingZeros(uint(n))

	var invN field.Element
	invN.SetUint64(uint64(n))
	invN.Inverse(&invN)

	omegaPowers := make([]field.Element, n)
	omegaPowers[0].SetOne()
	for i := 1; i < n; i++ {
		omegaPowers[i].Mul(&omegaPowers[i-1], &gen)
	}

	results := make([]field.Gen, len(zs))
	if p.IsBase() {
		weightedP := make([]field.Element, n)
		pBase := p.AsBase()
		for i := 0; i < n; i++ {
			var wi field.Element
			wi.Mul(&omegaPowers[i], &invN)
			weightedP[i].Mul(&wi, &pBase[i])
		}
		for j, z := range zs {
			if z.IsBase() {
				results[j] = field.ElemFromBase(evalBatchBaseBase(weightedP, z.AsBase(), omegaPowers, tb))
			} else {
				results[j] = field.ElemFromExt(evalBatchBaseExt(weightedP, z.AsExt(), omegaPowers, tb))
			}
		}
	} else {
		weightedP := make([]field.Ext, n)
		pExt := p.AsExt()
		for i := 0; i < n; i++ {
			var wi field.Element
			wi.Mul(&omegaPowers[i], &invN)
			weightedP[i].MulByElement(&pExt[i], &wi)
		}
		for j, z := range zs {
			if z.IsBase() {
				results[j] = field.ElemFromExt(evalBatchExtBase(weightedP, z.AsBase(), omegaPowers, tb))
			} else {
				results[j] = field.ElemFromExt(evalBatchExtExt(weightedP, z.AsExt(), omegaPowers, tb))
			}
		}
	}
	return results
}

// ComputeLagrangeAtZ returns the vector [L₀(z), L₁(z), …, L_{n-1}(z)] where
//
//	Lᵢ(z) = ωⁱ/n · (zⁿ - 1) / (z - ωⁱ)
//
// Uses the recurrence relation to avoid n-1 extra inversions:
//
//	L₀(z) = (1/n) · (zⁿ - 1) / (z - 1)
//	Lᵢ(z) = Lᵢ₋₁(z) · ω · (z - ωⁱ⁻¹) / (z - ωⁱ)
//
// cardinality must be a power of two not exceeding 2^MaxOrderRoot; z must not
// be a root of unity.
func ComputeLagrangeAtZ(z field.Gen, cardinality uint64) []field.Gen {
	gen := field.RootOfUnityBy(int(cardinality))
	n := int(cardinality)
	if z.IsBase() {
		res := computeLagrangeAtZBase(z.AsBase(), gen, n)
		out := make([]field.Gen, n)
		for i, e := range res {
			out[i] = field.ElemFromBase(e)
		}
		return out
	}
	res := computeLagrangeAtZExt(z.AsExt(), gen, n)
	out := make([]field.Gen, n)
	for i, e := range res {
		out[i] = field.ElemFromExt(e)
	}
	return out
}

// ---------------------------------------------------------------------------
// Single-evaluation specializations
// ---------------------------------------------------------------------------

// evalLagrangeBaseBase: p in 𝔽_p, z in 𝔽_p → result in 𝔽_p.
func evalLagrangeBaseBase(p []field.Element, z, gen field.Element) field.Element {
	n := len(p)
	tb := bits.TrailingZeros(uint(n))

	var invN field.Element
	invN.SetUint64(uint64(n))
	invN.Inverse(&invN)

	zPowN := z
	for i := 0; i < tb; i++ {
		zPowN.Square(&zPowN)
	}
	var one, zPowNMinusOne field.Element
	one.SetOne()
	zPowNMinusOne.Sub(&zPowN, &one)

	weightedP := make([]field.Element, n)
	denom := make([]field.Element, n)
	var accOmega field.Element
	accOmega.SetOne()
	for i := 0; i < n; i++ {
		var wi field.Element
		wi.Mul(&accOmega, &invN)
		weightedP[i].Mul(&wi, &p[i])
		denom[i].Sub(&z, &accOmega)
		accOmega.Mul(&accOmega, &gen)
	}

	invDenom := make([]field.Element, n)
	field.VecBatchInvBase(invDenom, denom)

	var sum field.Element
	for i := 0; i < n; i++ {
		var term field.Element
		term.Mul(&weightedP[i], &invDenom[i])
		sum.Add(&sum, &term)
	}

	var result field.Element
	result.Mul(&zPowNMinusOne, &sum)
	return result
}

// evalLagrangeBaseExt: p in 𝔽_p, z in 𝔽_{p^4} → result in 𝔽_{p^4}.
func evalLagrangeBaseExt(p []field.Element, z field.Ext, gen field.Element) field.Ext {
	n := len(p)
	tb := bits.TrailingZeros(uint(n))

	var invN field.Element
	invN.SetUint64(uint64(n))
	invN.Inverse(&invN)

	var zPowN field.Ext
	zPowN.Set(&z)
	for i := 0; i < tb; i++ {
		zPowN.Square(&zPowN)
	}
	var one, zPowNMinusOne field.Ext
	one.SetOne()
	zPowNMinusOne.Sub(&zPowN, &one)

	// weightedP[i] = (ω^i / n) * p[i] stays in 𝔽_p since both factors are base.
	weightedP := make([]field.Element, n)
	denom := make([]field.Ext, n)
	var accOmega field.Element
	accOmega.SetOne()
	for i := 0; i < n; i++ {
		var wi field.Element
		wi.Mul(&accOmega, &invN)
		weightedP[i].Mul(&wi, &p[i])
		denom[i].Set(&z)
		denom[i].B0.A0.Sub(&denom[i].B0.A0, &accOmega) // z - ω^i
		accOmega.Mul(&accOmega, &gen)
	}

	invDenom := make([]field.Ext, n)
	field.VecBatchInvExt(invDenom, denom)

	var sum field.Ext
	for i := 0; i < n; i++ {
		var term field.Ext
		term.MulByElement(&invDenom[i], &weightedP[i])
		sum.Add(&sum, &term)
	}

	var result field.Ext
	result.Mul(&zPowNMinusOne, &sum)
	return result
}

// evalLagrangeExtBase: p in 𝔽_{p^4}, z in 𝔽_p → result in 𝔽_{p^4}.
func evalLagrangeExtBase(p []field.Ext, z, gen field.Element) field.Ext {
	n := len(p)
	tb := bits.TrailingZeros(uint(n))

	var invN field.Element
	invN.SetUint64(uint64(n))
	invN.Inverse(&invN)

	zPowN := z
	for i := 0; i < tb; i++ {
		zPowN.Square(&zPowN)
	}
	var one, zPowNMinusOne field.Element
	one.SetOne()
	zPowNMinusOne.Sub(&zPowN, &one)

	weightedP := make([]field.Ext, n)
	denom := make([]field.Element, n)
	var accOmega field.Element
	accOmega.SetOne()
	for i := 0; i < n; i++ {
		var wi field.Element
		wi.Mul(&accOmega, &invN)
		weightedP[i].MulByElement(&p[i], &wi)
		denom[i].Sub(&z, &accOmega)
		accOmega.Mul(&accOmega, &gen)
	}

	invDenom := make([]field.Element, n)
	field.VecBatchInvBase(invDenom, denom)

	var sum field.Ext
	for i := 0; i < n; i++ {
		var term field.Ext
		term.MulByElement(&weightedP[i], &invDenom[i])
		sum.Add(&sum, &term)
	}

	var result field.Ext
	result.MulByElement(&sum, &zPowNMinusOne)
	return result
}

// evalLagrangeExtExt: p in 𝔽_{p^4}, z in 𝔽_{p^4} → result in 𝔽_{p^4}.
func evalLagrangeExtExt(p []field.Ext, z field.Ext, gen field.Element) field.Ext {
	n := len(p)
	tb := bits.TrailingZeros(uint(n))

	var invN field.Element
	invN.SetUint64(uint64(n))
	invN.Inverse(&invN)

	var zPowN field.Ext
	zPowN.Set(&z)
	for i := 0; i < tb; i++ {
		zPowN.Square(&zPowN)
	}
	var one, zPowNMinusOne field.Ext
	one.SetOne()
	zPowNMinusOne.Sub(&zPowN, &one)

	weightedP := make([]field.Ext, n)
	denom := make([]field.Ext, n)
	var accOmega field.Element
	accOmega.SetOne()
	for i := 0; i < n; i++ {
		var wi field.Element
		wi.Mul(&accOmega, &invN)
		weightedP[i].MulByElement(&p[i], &wi)
		denom[i].Set(&z)
		denom[i].B0.A0.Sub(&denom[i].B0.A0, &accOmega) // z - ω^i
		accOmega.Mul(&accOmega, &gen)
	}

	invDenom := make([]field.Ext, n)
	field.VecBatchInvExt(invDenom, denom)

	var sum field.Ext
	for i := 0; i < n; i++ {
		var term field.Ext
		term.Mul(&weightedP[i], &invDenom[i])
		sum.Add(&sum, &term)
	}

	var result field.Ext
	result.Mul(&zPowNMinusOne, &sum)
	return result
}

// ---------------------------------------------------------------------------
// Batch inner helpers — accept pre-computed weightedP and omegaPowers
// ---------------------------------------------------------------------------

func evalBatchBaseBase(weightedP []field.Element, z field.Element, omegaPowers []field.Element, tb int) field.Element {
	n := len(weightedP)

	zPowN := z
	for i := 0; i < tb; i++ {
		zPowN.Square(&zPowN)
	}
	var one, zPowNMinusOne field.Element
	one.SetOne()
	zPowNMinusOne.Sub(&zPowN, &one)

	denom := make([]field.Element, n)
	for i := 0; i < n; i++ {
		denom[i].Sub(&z, &omegaPowers[i])
	}
	invDenom := make([]field.Element, n)
	field.VecBatchInvBase(invDenom, denom)

	var sum field.Element
	for i := 0; i < n; i++ {
		var term field.Element
		term.Mul(&weightedP[i], &invDenom[i])
		sum.Add(&sum, &term)
	}

	var result field.Element
	result.Mul(&zPowNMinusOne, &sum)
	return result
}

func evalBatchBaseExt(weightedP []field.Element, z field.Ext, omegaPowers []field.Element, tb int) field.Ext {
	n := len(weightedP)

	var zPowN field.Ext
	zPowN.Set(&z)
	for i := 0; i < tb; i++ {
		zPowN.Square(&zPowN)
	}
	var one, zPowNMinusOne field.Ext
	one.SetOne()
	zPowNMinusOne.Sub(&zPowN, &one)

	denom := make([]field.Ext, n)
	for i := 0; i < n; i++ {
		denom[i].Set(&z)
		denom[i].B0.A0.Sub(&denom[i].B0.A0, &omegaPowers[i])
	}
	invDenom := make([]field.Ext, n)
	field.VecBatchInvExt(invDenom, denom)

	var sum field.Ext
	for i := 0; i < n; i++ {
		var term field.Ext
		term.MulByElement(&invDenom[i], &weightedP[i])
		sum.Add(&sum, &term)
	}

	var result field.Ext
	result.Mul(&zPowNMinusOne, &sum)
	return result
}

func evalBatchExtBase(weightedP []field.Ext, z field.Element, omegaPowers []field.Element, tb int) field.Ext {
	n := len(weightedP)

	zPowN := z
	for i := 0; i < tb; i++ {
		zPowN.Square(&zPowN)
	}
	var one, zPowNMinusOne field.Element
	one.SetOne()
	zPowNMinusOne.Sub(&zPowN, &one)

	denom := make([]field.Element, n)
	for i := 0; i < n; i++ {
		denom[i].Sub(&z, &omegaPowers[i])
	}
	invDenom := make([]field.Element, n)
	field.VecBatchInvBase(invDenom, denom)

	var sum field.Ext
	for i := 0; i < n; i++ {
		var term field.Ext
		term.MulByElement(&weightedP[i], &invDenom[i])
		sum.Add(&sum, &term)
	}

	var result field.Ext
	result.MulByElement(&sum, &zPowNMinusOne)
	return result
}

func evalBatchExtExt(weightedP []field.Ext, z field.Ext, omegaPowers []field.Element, tb int) field.Ext {
	n := len(weightedP)

	var zPowN field.Ext
	zPowN.Set(&z)
	for i := 0; i < tb; i++ {
		zPowN.Square(&zPowN)
	}
	var one, zPowNMinusOne field.Ext
	one.SetOne()
	zPowNMinusOne.Sub(&zPowN, &one)

	denom := make([]field.Ext, n)
	for i := 0; i < n; i++ {
		denom[i].Set(&z)
		denom[i].B0.A0.Sub(&denom[i].B0.A0, &omegaPowers[i])
	}
	invDenom := make([]field.Ext, n)
	field.VecBatchInvExt(invDenom, denom)

	var sum field.Ext
	for i := 0; i < n; i++ {
		var term field.Ext
		term.Mul(&weightedP[i], &invDenom[i])
		sum.Add(&sum, &term)
	}

	var result field.Ext
	result.Mul(&zPowNMinusOne, &sum)
	return result
}

// ---------------------------------------------------------------------------
// Lagrange-basis vector specializations
// ---------------------------------------------------------------------------

// computeLagrangeAtZBase: z in 𝔽_p → each Lᵢ(z) in 𝔽_p.
func computeLagrangeAtZBase(z, gen field.Element, n int) []field.Element {
	res := make([]field.Element, n)
	tb := bits.TrailingZeros(uint(n))

	zPowN := z
	for i := 0; i < tb; i++ {
		zPowN.Square(&zPowN)
	}
	var one, zPowNMinusOne field.Element
	one.SetOne()
	zPowNMinusOne.Sub(&zPowN, &one)

	var invN field.Element
	invN.SetUint64(uint64(n))
	invN.Inverse(&invN)

	// L₀ = (zⁿ - 1) / (n · (z - 1))
	var zMinusOne field.Element
	zMinusOne.Sub(&z, &one)
	res[0].Div(&zPowNMinusOne, &zMinusOne)
	res[0].Mul(&res[0], &invN)

	prevZMinusOmega := zMinusOne
	var accOmega field.Element
	accOmega.SetOne()

	for i := 1; i < n; i++ {
		// Lᵢ = ω · Lᵢ₋₁ · (z - ω^{i-1}) / (z - ωⁱ)
		accOmega.Mul(&accOmega, &gen)
		var curZMinusOmega field.Element
		curZMinusOmega.Sub(&z, &accOmega)
		res[i].Mul(&res[i-1], &gen)
		res[i].Mul(&res[i], &prevZMinusOmega)
		res[i].Div(&res[i], &curZMinusOmega)
		prevZMinusOmega = curZMinusOmega
	}
	return res
}

// computeLagrangeAtZExt: z in 𝔽_{p^4} → each Lᵢ(z) in 𝔽_{p^4}.
func computeLagrangeAtZExt(z field.Ext, gen field.Element, n int) []field.Ext {
	res := make([]field.Ext, n)
	tb := bits.TrailingZeros(uint(n))

	var zPowN field.Ext
	zPowN.Set(&z)
	for i := 0; i < tb; i++ {
		zPowN.Square(&zPowN)
	}
	var one, zPowNMinusOne field.Ext
	one.SetOne()
	zPowNMinusOne.Sub(&zPowN, &one)

	var invN field.Element
	invN.SetUint64(uint64(n))
	invN.Inverse(&invN)

	// L₀ = (zⁿ - 1) / (n · (z - 1))
	var zMinusOne field.Ext
	zMinusOne.Sub(&z, &one)
	res[0].Div(&zPowNMinusOne, &zMinusOne)
	res[0].MulByElement(&res[0], &invN)

	prevZMinusOmega := zMinusOne
	var accOmega field.Element
	accOmega.SetOne()

	for i := 1; i < n; i++ {
		// Lᵢ = ω · Lᵢ₋₁ · (z - ω^{i-1}) / (z - ωⁱ)
		accOmega.Mul(&accOmega, &gen)
		var curZMinusOmega field.Ext
		curZMinusOmega.Set(&z)
		curZMinusOmega.B0.A0.Sub(&curZMinusOmega.B0.A0, &accOmega) // z - ω^i
		res[i].MulByElement(&res[i-1], &gen)
		res[i].Mul(&res[i], &prevZMinusOmega)
		res[i].Div(&res[i], &curZMinusOmega)
		prevZMinusOmega = curZMinusOmega
	}
	return res
}
