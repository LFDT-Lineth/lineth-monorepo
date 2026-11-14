// Package gkrverifier generates, from a gnark GKR circuit over KoalaBear's E₆ and its schedule, the
// modules and constraints of gnark's GKR verifier as part of a [wiop.System].
package gkrverifier

import (
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// E6 is an element of the degree-6 extension E₆, the six base expressions of its coordinates in the
// order of gnark-crypto's extensions.E6.AppendBase: B0.A0, B0.A1, B1.A0, B1.A1, B2.A0, B2.A1. With
//
//	E₂ = 𝔽[u] / (u² − 3),    E₆ = E₂[v] / (v³ − ξ),    ξ = 1 + u,
//
// an element is A₀ + A₁v + A₂v² with Aₖ = c₂ₖ + c₂ₖ₊₁u. Operations build expressions over the base
// field, which a [wiop.Module] constrains or reads.
type E6 [6]wiop.Expression

// e2 is an element of E₂, the two base expressions of its coordinates.
type e2 [2]wiop.Expression

// baseConstant returns the expression of the base field constant v.
func baseConstant(v uint64) wiop.Expression {
	var e field.Element
	e.SetUint64(v)
	return wiop.NewConstantField(e)
}

func (a e2) add(b e2) e2 { return e2{wiop.Add(a[0], b[0]), wiop.Add(a[1], b[1])} }

// mul returns (a₀ + a₁u)(b₀ + b₁u) = (a₀b₀ + 3a₁b₁) + (a₀b₁ + a₁b₀)u.
func (a e2) mul(b e2) e2 {
	return e2{
		wiop.Add(wiop.Mul(a[0], b[0]), wiop.Mul(baseConstant(3), wiop.Mul(a[1], b[1]))),
		wiop.Add(wiop.Mul(a[0], b[1]), wiop.Mul(a[1], b[0])),
	}
}

// mulXi returns (a₀ + a₁u)ξ = (a₀ + 3a₁) + (a₀ + a₁)u.
func (a e2) mulXi() e2 {
	return e2{wiop.Add(a[0], wiop.Mul(baseConstant(3), a[1])), wiop.Add(a[0], a[1])}
}

// coordinates returns the three E₂ components of x.
func (x E6) coordinates() [3]e2 {
	return [3]e2{{x[0], x[1]}, {x[2], x[3]}, {x[4], x[5]}}
}

func fromCoordinates(a [3]e2) E6 {
	return E6{a[0][0], a[0][1], a[1][0], a[1][1], a[2][0], a[2][1]}
}

// Add returns x + y.
func (x E6) Add(y E6) E6 {
	var z E6
	for i := range z {
		z[i] = wiop.Add(x[i], y[i])
	}
	return z
}

// MulByBase returns kx for an expression k of the base field, which may be a constant.
func (x E6) MulByBase(k wiop.Expression) E6 {
	var z E6
	for i := range z {
		z[i] = wiop.Mul(k, x[i])
	}
	return z
}

// Mul returns xy, with the product C = AB of the tower:
//
//	C₀ = A₀B₀ + ξ(A₁B₂ + A₂B₁)
//	C₁ = A₀B₁ + A₁B₀ + ξA₂B₂
//	C₂ = A₀B₂ + A₁B₁ + A₂B₀
//
// Each coordinate is a sum of products of base expressions, of degree 2 in them.
func (x E6) Mul(y E6) E6 {
	a, b := x.coordinates(), y.coordinates()
	return fromCoordinates([3]e2{
		a[0].mul(b[0]).add(a[1].mul(b[2]).add(a[2].mul(b[1])).mulXi()),
		a[0].mul(b[1]).add(a[1].mul(b[0])).add(a[2].mul(b[2]).mulXi()),
		a[0].mul(b[2]).add(a[1].mul(b[1])).add(a[2].mul(b[0])),
	})
}

// Equal returns the six expressions xᵢ − yᵢ, which vanish exactly when x = y.
func (x E6) Equal(y E6) [6]wiop.Expression {
	var diff [6]wiop.Expression
	for i := range diff {
		diff[i] = wiop.Sub(x[i], y[i])
	}
	return diff
}
