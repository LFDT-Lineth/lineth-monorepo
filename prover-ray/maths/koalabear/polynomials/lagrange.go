package polynomials

import (
	"math/bits"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/gnark-crypto/field/koalabear"
	"github.com/consensys/gnark-crypto/field/koalabear/extensions"
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
