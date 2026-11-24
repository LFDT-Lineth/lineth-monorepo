package polynomials

import (
	"math/bits"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/gnark-crypto/field/koalabear"
	"github.com/consensys/gnark-crypto/field/koalabear/extensions"
	"github.com/consensys/gnark-crypto/field/koalabear/fft"
)

// EvalLagrange evaluates a polynomial given in Lagrange (evaluation) basis at
// point z using the barycentric formula:
//
//	P(z) = (zⁿ - 1) · Σᵢ [ (ωⁱ/n · p[i]) / (z - ωⁱ) ]
//
// The subgroup size n = len(p) must be a power of two not exceeding 2^MaxOrderRoot.
// z must not be a root of unity.
func EvalLagrange(p field.Vec, z field.Gen) field.Gen {
	return EvalLagrangeBatch(p, []field.Gen{z})[0]
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
	results := make([]field.Gen, len(zs))
	n := p.Len()
	if n == 0 {
		for j := range results {
			results[j] = field.ElemZero()
		}
		return results
	}
	tb := bits.TrailingZeros(uint(n))

	var invN field.Element
	invN.SetUint64(uint64(n))
	invN.Inverse(&invN)

	// omegaPowers[i] = ωⁱ is a running product, so it stays a loop; every
	// other pass below is a gnark-crypto vector kernel.
	gen := field.RootOfUnityBy(n)
	omegaPowers := make([]field.Element, n)
	omegaPowers[0].SetOne()
	for i := 1; i < n; i++ {
		omegaPowers[i].Mul(&omegaPowers[i-1], &gen)
	}

	// weightedP[i] = p[i]·ωⁱ/n.
	if p.IsBase() {
		weightedP := koalabear.Vector(make([]field.Element, n))
		weightedP.Mul(omegaPowers, p.AsBase())
		weightedP.ScalarMul(weightedP, &invN)
		for j, z := range zs {
			if z.IsBase() {
				results[j] = field.ElemFromBase(evalBatchBaseBase(weightedP, z.AsBase(), omegaPowers, tb))
			} else {
				results[j] = field.ElemFromExt(evalBatchBaseExt(weightedP, z.AsExt(), omegaPowers, tb))
			}
		}
	} else {
		weightedP := extensions.VectorE6(make([]field.Ext, n))
		weightedP.MulByElement(p.AsExt(), omegaPowers)
		weightedP.ScalarMulByElement(weightedP, &invN)
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

// EvalLagrangeBasisAtZ returns the vector [L₀(z), L₁(z), …, L_{n-1}(z)] where
//
//	Lᵢ(z) = ωⁱ/n · (zⁿ - 1) / (z - ωⁱ)
//
// are the Lagrange basis vectors on the root of unity subgroup.
//
// Uses the recurrence relation to avoid n-1 extra inversions:
//
//	L₀(z) = (1/n) · (zⁿ - 1) / (z - 1)
//	Lᵢ(z) = Lᵢ₋₁(z) · ω · (z - ωⁱ⁻¹) / (z - ωⁱ)
//
// cardinality must be a power of two not exceeding 2^MaxOrderRoot; z must not
// be a root of unity.
func EvalLagrangeBasisAtZ(z field.Gen, cardinality uint64) []field.Gen {

	var (
		gen = field.RootOfUnityBy(int(cardinality))
		n   = int(cardinality)
	)

	if z.IsBase() {
		var (
			res = computeLagrangeAtZBase(z.AsBase(), gen, n)
			out = make([]field.Gen, n)
		)

		for i, e := range res {
			out[i] = field.ElemFromBase(e)
		}
		return out
	}

	var (
		res = computeLagrangeAtZExt(z.AsExt(), gen, n)
		out = make([]field.Gen, n)
	)

	for i, e := range res {
		out[i] = field.ElemFromExt(e)
	}
	return out
}

// QuotientLagrange computes q(X) = (v - p(X)) / (z - X) where p is in Lagrange
// form over domain d and v = p(z) is the claimed evaluation at z outside the
// domain. Returns q in Lagrange Normal form: q[j] = (v - p(ω^j)) / (z - ω^j).
// Panics (division by zero) if z happens to be a domain point.
func QuotientLagrange(p []field.Element, v, z field.Element) []field.Element {

	var (
		n      = len(p)
		q      = make([]field.Element, n)
		omegaJ = field.One()
	)

	omega, err := fft.Generator(uint64(n))
	if err != nil {
		panic(err) // there is no actual error case, this is a void error
	}

	for j := 0; j < n; j++ {
		var num, den koalabear.Element
		num.Sub(&v, &p[j])
		den.Sub(&z, &omegaJ)
		den.Inverse(&den)
		q[j].Mul(&num, &den)
		omegaJ.Mul(&omegaJ, &omega)
	}

	return q
}

// QuotientLagrangeExt is as QuotientLagrange but for extension fields.
func QuotientLagrangeExt(p []field.Ext, v, z field.Ext) []field.Ext {

	var (
		n      = len(p)
		q      = make([]field.Ext, n)
		omegaJ = field.One()
	)

	omega, err := fft.Generator(uint64(n))
	if err != nil {
		panic(err) // there is no actual error case, this is a void error
	}

	for j := 0; j < n; j++ {
		var num, den field.Ext
		num.Sub(&v, &p[j])
		den.SubElement(&z, &omegaJ)
		// @alex: batch inverse here? My guts tell me that the efficient thing
		// would be to implement a function computing all the quotients provided
		// a list of poly and a list of claim. That way, we can batch the
		// computation as efficiently as possible.
		den.Inverse(&den)
		q[j].Mul(&num, &den)
		omegaJ.Mul(&omegaJ, &omega)
	}

	return q
}

// ---------------------------------------------------------------------------
// Evaluation at one point, given weightedP[i] = p[i]·ωⁱ/n and the ωⁱ table
// ---------------------------------------------------------------------------

// zPowNMinusOneBase returns zⁿ - 1 for n = 2^tb.
func zPowNMinusOneBase(z field.Element, tb int) field.Element {
	for range tb {
		z.Square(&z)
	}
	var one field.Element
	one.SetOne()
	z.Sub(&z, &one)
	return z
}

// zPowNMinusOneExt returns zⁿ - 1 for n = 2^tb.
func zPowNMinusOneExt(z field.Ext, tb int) field.Ext {
	for range tb {
		z.Square(&z)
	}
	var one field.Ext
	one.SetOne()
	z.Sub(&z, &one)
	return z
}

// invDenomBase returns 1/(z - ωⁱ) for every i.
func invDenomBase(z field.Element, omegaPowers []field.Element) []field.Element {
	d := koalabear.Vector(make([]field.Element, len(omegaPowers)))
	d.ScalarSubFrom(&z, omegaPowers)
	field.VecBatchInvBase(d, d)
	return d
}

// invDenomExt returns 1/(z - ωⁱ) for every i.
func invDenomExt(z field.Ext, omegaPowers []field.Element) []field.Ext {
	d := extensions.VectorE6(make([]field.Ext, len(omegaPowers)))
	d.ScalarSubFromBase(&z, omegaPowers)
	field.VecBatchInvExt(d, d)
	return d
}

func evalBatchBaseBase(weightedP []field.Element, z field.Element, omegaPowers []field.Element, tb int) field.Element {
	sum := koalabear.Vector(weightedP).InnerProduct(invDenomBase(z, omegaPowers))
	scale := zPowNMinusOneBase(z, tb)
	sum.Mul(&sum, &scale)
	return sum
}

func evalBatchBaseExt(weightedP []field.Element, z field.Ext, omegaPowers []field.Element, tb int) field.Ext {
	sum := extensions.VectorE6(invDenomExt(z, omegaPowers)).InnerProductByElement(weightedP)
	scale := zPowNMinusOneExt(z, tb)
	sum.Mul(&sum, &scale)
	return sum
}

func evalBatchExtBase(weightedP []field.Ext, z field.Element, omegaPowers []field.Element, tb int) field.Ext {
	sum := extensions.VectorE6(weightedP).InnerProductByElement(invDenomBase(z, omegaPowers))
	scale := zPowNMinusOneBase(z, tb)
	sum.MulByElement(&sum, &scale)
	return sum
}

func evalBatchExtExt(weightedP []field.Ext, z field.Ext, omegaPowers []field.Element, tb int) field.Ext {
	sum := extensions.VectorE6(weightedP).InnerProduct(invDenomExt(z, omegaPowers))
	scale := zPowNMinusOneExt(z, tb)
	sum.Mul(&sum, &scale)
	return sum
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
