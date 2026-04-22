package polynomials

import (
	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
)

// evalNative evaluates p(X) = Σᵢ p[i]·Xⁱ at z using Horner's method.
// It is the inner kernel shared by [EvalCanonical] and [EvalCanonicalBatch].
func evalNative(poly field.Vec, z field.Gen) field.Gen {
	res := field.ElemZero()
	n := poly.Len()
	if poly.IsBase() {
		base := poly.AsBase()
		for i := n - 1; i >= 0; i-- {
			res = res.Mul(z).Add(field.ElemFromBase(base[i]))
		}
	} else {
		ext := poly.AsExt()
		for i := n - 1; i >= 0; i-- {
			res = res.Mul(z).Add(field.ElemFromExt(ext[i]))
		}
	}
	return res
}

// EvalCanonical evaluates p(X) = Σᵢ p[i]·Xⁱ at z using Horner's method.
// The result is tagged base iff both poly and z are base-field values.
func EvalCanonical(poly field.Vec, z field.Gen) field.Gen {
	return evalNative(poly, z)
}

// EvalCanonicalBatch evaluates multiple polynomials at the same point z,
// sharing a precomputed power table z⁰, z¹, ..., z^(maxDeg-1).
//
// For k polynomials of maximum degree d, this saves approximately (k-1)·d
// multiplications compared to k separate Horner evaluations.
//
// Falls back to [EvalCanonical] when len(polys) == 1 since Horner is cheaper
// for a single polynomial.
func EvalCanonicalBatch(polys []field.Vec, z field.Gen) []field.Gen {
	if len(polys) == 0 {
		return nil
	}
	if len(polys) == 1 {
		return []field.Gen{EvalCanonical(polys[0], z)}
	}

	maxDeg := 0
	for _, p := range polys {
		if p.Len() > maxDeg {
			maxDeg = p.Len()
		}
	}

	if maxDeg == 0 {
		res := make([]field.Gen, len(polys))
		for i := range res {
			res[i] = field.ElemZero()
		}
		return res
	}

	// Precompute powers[i] = z^i
	powers := make([]field.Gen, maxDeg)
	powers[0] = field.ElemOne()
	for i := 1; i < maxDeg; i++ {
		powers[i] = powers[i-1].Mul(z)
	}

	results := make([]field.Gen, len(polys))
	for j, poly := range polys {
		n := poly.Len()
		if n == 0 {
			results[j] = field.ElemZero()
			continue
		}
		acc := field.ElemZero()
		if poly.IsBase() {
			base := poly.AsBase()
			for i := 0; i < n; i++ {
				acc = acc.Add(field.ElemFromBase(base[i]).Mul(powers[i]))
			}
		} else {
			ext := poly.AsExt()
			for i := 0; i < n; i++ {
				acc = acc.Add(field.ElemFromExt(ext[i]).Mul(powers[i]))
			}
		}
		results[j] = acc
	}
	return results
}
