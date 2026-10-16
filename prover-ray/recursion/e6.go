package recursion

import (
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// e6MulExprs returns z = x·y as expressions (schoolbook over E2, with
// u² = 3 and v³ = u + 1), in the layout (B0.A0, B0.A1, B1.A0, B1.A1, B2.A0,
// B2.A1).
func e6MulExprs(x, y []wiop.Expression) []wiop.Expression {
	three := constant(3)
	// E2 product (a0 + a1·u)(b0 + b1·u) = (a0·b0 + 3·a1·b1) + (a0·b1 + a1·b0)·u
	mul2 := func(a0, a1, b0, b1 wiop.Expression) (wiop.Expression, wiop.Expression) {
		return wiop.Add(wiop.Mul(a0, b0), prod(three, a1, b1)), wiop.Add(wiop.Mul(a0, b1), wiop.Mul(a1, b0))
	}
	// ξ·(p + q·u) = (p + 3·q) + (p + q)·u
	xi := func(p, q wiop.Expression) (wiop.Expression, wiop.Expression) {
		return wiop.Add(p, wiop.Mul(three, q)), wiop.Add(p, q)
	}
	p := func(i, j int) (wiop.Expression, wiop.Expression) {
		return mul2(x[2*i], x[2*i+1], y[2*j], y[2*j+1])
	}
	p00r, p00u := p(0, 0)
	p01r, p01u := p(0, 1)
	p02r, p02u := p(0, 2)
	p10r, p10u := p(1, 0)
	p11r, p11u := p(1, 1)
	p12r, p12u := p(1, 2)
	p20r, p20u := p(2, 0)
	p21r, p21u := p(2, 1)
	p22r, p22u := p(2, 2)
	t0r, t0u := xi(wiop.Add(p12r, p21r), wiop.Add(p12u, p21u))
	t1r, t1u := xi(p22r, p22u)
	return []wiop.Expression{
		wiop.Add(p00r, t0r), wiop.Add(p00u, t0u),
		sum(p01r, p10r, t1r), sum(p01u, p10u, t1u),
		sum(p02r, p11r, p20r), sum(p02u, p11u, p20u),
	}
}
