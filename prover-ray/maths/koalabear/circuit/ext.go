package circuit

import (
	"math/big"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/gnark-crypto/field/koalabear/extensions"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/math/emulated"
)

// Register hints at package initialization
func init() {
	solver.RegisterHint(
		inverseE2Hint,
		inverseExtHintNative, inverseExtHintEmulated,
		divExtHintNative, divExtHintEmulated,
		mulExtHintNative, mulExtHintEmulated)
}

// E2 is a quadratic extension element.
// It represents an element of F_p^2 = F_p[u] / (u^2 - 3).
type E2 struct {
	A0, A1 Element
}

// Ext is a circuit variable over the degree-6 extension field.
// It represents an element of F_p^6 = F_p^2[v] / (v^3 - (u+1)), i.e. each
// element is stored as (B0, B1, B2) with each Bi in E2.
type Ext struct {
	B0, B1, B2 E2
}

// --- Ext Constructors (for witness assignment) ---

// NewExt creates an Ext from field.Ext for witness assignment.
func NewExt(v field.Ext) Ext {
	return Ext{
		B0: newE2(v.B0),
		B1: newE2(v.B1),
		B2: newE2(v.B2),
	}
}

func newE2(v extensions.E2) E2 {
	return E2{
		A0: NewElementFromKoala(v.A0),
		A1: NewElementFromKoala(v.A1),
	}
}

// NewFromBaseExt creates an Ext with a base field witness value in the constant term.
func NewFromBaseExt(v any) Ext {
	z := NewElement(0)
	return Ext{
		B0: E2{A0: NewElement(v), A1: z},
		B1: E2{A0: z, A1: z},
		B2: E2{A0: z, A1: z},
	}
}

// NewExtFromFrontendVar creates an Ext from a frontend.Variable for the base component.
func NewExtFromFrontendVar(v frontend.Variable) Ext {
	z := NewElement(0)
	return Ext{
		B0: E2{A0: WrapFrontendVariable(v), A1: z},
		B1: E2{A0: z, A1: z},
		B2: E2{A0: z, A1: z},
	}
}

// NewExtFrom6FrontendVars creates an Ext from 6 frontend.Variable values, one
// per coordinate (in the order B0.A0, B0.A1, B1.A0, B1.A1, B2.A0, B2.A1).
func NewExtFrom6FrontendVars(b0a0, b0a1, b1a0, b1a1, b2a0, b2a1 frontend.Variable) Ext {
	return Ext{
		B0: E2{A0: WrapFrontendVariable(b0a0), A1: WrapFrontendVariable(b0a1)},
		B1: E2{A0: WrapFrontendVariable(b1a0), A1: WrapFrontendVariable(b1a1)},
		B2: E2{A0: WrapFrontendVariable(b2a0), A1: WrapFrontendVariable(b2a1)},
	}
}

// Coordinates returns all 6 base field coordinates.
func (x Ext) Coordinates() (b0a0, b0a1, b1a0, b1a1, b2a0, b2a1 Element) {
	return x.B0.A0, x.B0.A1, x.B1.A0, x.B1.A1, x.B2.A0, x.B2.A1
}

// FromBaseVar creates an Ext from a Var (for in-circuit conversion).
// Use this when you have an existing circuit variable to embed in the extension field.
func FromBaseVar(v Element) Ext {
	z := NewElement(0)
	return Ext{
		B0: E2{A0: v, A1: z},
		B1: E2{A0: z, A1: z},
		B2: E2{A0: z, A1: z},
	}
}

// --- Ext Constants (in-circuit) ---

// ZeroExt returns the additive identity in the extension field.
func (a *API) ZeroExt() Ext {
	z := a.Zero()
	return Ext{B0: E2{A0: z, A1: z}, B1: E2{A0: z, A1: z}, B2: E2{A0: z, A1: z}}
}

// OneExt returns the multiplicative identity in the extension field.
func (a *API) OneExt() Ext {
	z, o := a.Zero(), a.One()
	return Ext{B0: E2{A0: o, A1: z}, B1: E2{A0: z, A1: z}, B2: E2{A0: z, A1: z}}
}

// FromBaseExt creates an Ext element with a base field value in the constant term.
func (a *API) FromBaseExt(x Element) Ext {
	z := a.Zero()
	return Ext{B0: E2{A0: x, A1: z}, B1: E2{A0: z, A1: z}, B2: E2{A0: z, A1: z}}
}

// ConstExt creates a constant Ext element from a field.Ext.
// This should be used during circuit definition to create constant extension field values.
// For witness assignment, use NewExt instead.
func (a *API) ConstExt(v field.Ext) Ext {
	return Ext{
		B0: E2{
			A0: a.Const(int64(v.B0.A0.Uint64())),
			A1: a.Const(int64(v.B0.A1.Uint64())),
		},
		B1: E2{
			A0: a.Const(int64(v.B1.A0.Uint64())),
			A1: a.Const(int64(v.B1.A1.Uint64())),
		},
		B2: E2{
			A0: a.Const(int64(v.B2.A0.Uint64())),
			A1: a.Const(int64(v.B2.A1.Uint64())),
		},
	}
}

// --- Ext Arithmetic Operations ---

// AddExt returns x + y in the extension field.
func (a *API) AddExt(x, y Ext) Ext {
	return Ext{
		B0: a.e2Add(x.B0, y.B0),
		B1: a.e2Add(x.B1, y.B1),
		B2: a.e2Add(x.B2, y.B2),
	}
}

// SubExt returns x - y in the extension field.
func (a *API) SubExt(x, y Ext) Ext {
	return Ext{
		B0: a.e2Sub(x.B0, y.B0),
		B1: a.e2Sub(x.B1, y.B1),
		B2: a.e2Sub(x.B2, y.B2),
	}
}

// NegExt returns -x in the extension field.
func (a *API) NegExt(x Ext) Ext {
	z := a.Zero()
	zero := E2{A0: z, A1: z}
	return Ext{
		B0: a.e2Sub(zero, x.B0),
		B1: a.e2Sub(zero, x.B1),
		B2: a.e2Sub(zero, x.B2),
	}
}

// DoubleExt returns 2*x in the extension field.
func (a *API) DoubleExt(x Ext) Ext {
	two := big.NewInt(2)
	return Ext{
		B0: a.e2MulConst(x.B0, two),
		B1: a.e2MulConst(x.B1, two),
		B2: a.e2MulConst(x.B2, two),
	}
}

// qnrE2 is the quadratic non-residue constant for E2: u^2 = 3.
var qnrE2 = big.NewInt(3)

// e2MulByCubicNonResidue multiplies an E2 by the cubic non-residue (u+1).
// Given x = a0 + a1*u, (a0 + a1*u)*(1+u) = (a0+3*a1) + (a0+a1)*u (because u^2=3).
func (a *API) e2MulByCubicNonResidue(x E2) E2 {
	z1 := a.Add(x.A0, x.A1)
	z0 := a.MulConst(x.A1, qnrE2) // 3*a1
	z0 = a.Add(z0, x.A0)          // a0 + 3*a1
	return E2{A0: z0, A1: z1}
}

// e2Add returns x + y in E2.
func (a *API) e2Add(x, y E2) E2 {
	return E2{A0: a.Add(x.A0, y.A0), A1: a.Add(x.A1, y.A1)}
}

// e2Sub returns x - y in E2.
func (a *API) e2Sub(x, y E2) E2 {
	return E2{A0: a.Sub(x.A0, y.A0), A1: a.Sub(x.A1, y.A1)}
}

// e2Mul returns x * y in E2 using Karatsuba.
// (a0 + a1*u) * (b0 + b1*u) where u^2 = 3
func (a *API) e2Mul(x, y E2) E2 {
	l1 := a.Add(x.A0, x.A1)
	l2 := a.Add(y.A0, y.A1)
	u := a.Mul(l1, l2)      // (a0+a1)(b0+b1)
	ac := a.Mul(x.A0, y.A0) // a0*b0
	bd := a.Mul(x.A1, y.A1) // a1*b1

	sum := a.Add(ac, bd)
	a1 := a.Sub(u, sum) // (a0+a1)(b0+b1) - a0*b0 - a1*b1

	bd3 := a.MulConst(bd, qnrE2) // 3*a1*b1 (since u^2 = 3)
	a0 := a.Add(ac, bd3)

	return E2{A0: a0, A1: a1}
}

// e2Square returns x^2 in E2.
func (a *API) e2Square(x E2) E2 {
	a0sq := a.Mul(x.A0, x.A0)
	a1sq := a.Mul(x.A1, x.A1)
	a1sq3 := a.MulConst(a1sq, qnrE2)
	cross := a.Mul(x.A0, x.A1)
	return E2{
		A0: a.Add(a0sq, a1sq3),
		A1: a.MulConst(cross, big.NewInt(2)),
	}
}

// e2MulByFp multiplies an E2 by a base field element.
func (a *API) e2MulByFp(x E2, c Element) E2 {
	return E2{
		A0: a.Mul(x.A0, c),
		A1: a.Mul(x.A1, c),
	}
}

// e2MulConst multiplies an E2 by a constant.
func (a *API) e2MulConst(x E2, c *big.Int) E2 {
	return E2{
		A0: a.MulConst(x.A0, c),
		A1: a.MulConst(x.A1, c),
	}
}

// MulExt returns x * y in the extension field using Karatsuba over E2.
// Implements Algorithm 13 from https://eprint.iacr.org/2010/354.pdf, specialized
// for E6 = E2[v]/(v^3 - (u+1)). Costs 6 E2 multiplications (≈18 variable base muls).
func (a *API) MulExt(x, y Ext, more ...*Ext) Ext {
	t0 := a.e2Mul(x.B0, y.B0)
	t1 := a.e2Mul(x.B1, y.B1)
	t2 := a.e2Mul(x.B2, y.B2)

	// z0 = ((B1+B2)*(C1+C2) - t1 - t2) * (u+1) + t0
	c0 := a.e2Add(x.B1, x.B2)
	tmp := a.e2Add(y.B1, y.B2)
	c0 = a.e2Mul(c0, tmp)
	c0 = a.e2Sub(c0, t1)
	c0 = a.e2Sub(c0, t2)
	c0 = a.e2MulByCubicNonResidue(c0)
	c0 = a.e2Add(c0, t0)

	// z1 = (B0+B1)*(C0+C1) - t0 - t1 + t2*(u+1)
	c1 := a.e2Add(x.B0, x.B1)
	tmp = a.e2Add(y.B0, y.B1)
	c1 = a.e2Mul(c1, tmp)
	c1 = a.e2Sub(c1, t0)
	c1 = a.e2Sub(c1, t1)
	t2NR := a.e2MulByCubicNonResidue(t2)
	c1 = a.e2Add(c1, t2NR)

	// z2 = (B0+B2)*(C0+C2) - t0 - t2 + t1
	c2 := a.e2Add(x.B0, x.B2)
	tmp = a.e2Add(y.B0, y.B2)
	c2 = a.e2Mul(c2, tmp)
	c2 = a.e2Sub(c2, t0)
	c2 = a.e2Sub(c2, t2)
	c2 = a.e2Add(c2, t1)

	result := Ext{B0: c0, B1: c1, B2: c2}

	if len(more) > 0 {
		return a.MulExt(result, *more[0], more[1:]...)
	}
	return result
}

// SquareExt returns x^2 in the extension field, following Algorithm 16 from
// https://eprint.iacr.org/2010/354.pdf, specialized for E6 = E2[v]/(v^3-(u+1)).
func (a *API) SquareExt(x Ext) Ext {
	// c4 = 2*B0*B1
	c4 := a.e2Mul(x.B0, x.B1)
	c4 = a.e2MulConst(c4, big.NewInt(2))

	// c5 = B2^2
	c5 := a.e2Square(x.B2)

	// c1 = c5*(u+1) + c4
	c1 := a.e2MulByCubicNonResidue(c5)
	c1 = a.e2Add(c1, c4)

	// c2 = c4 - c5
	c2 := a.e2Sub(c4, c5)

	// c3 = B0^2
	c3 := a.e2Square(x.B0)

	// c4 = B0 - B1 + B2
	c4 = a.e2Sub(x.B0, x.B1)
	c4 = a.e2Add(c4, x.B2)

	// c5 = 2*B1*B2
	c5 = a.e2Mul(x.B1, x.B2)
	c5 = a.e2MulConst(c5, big.NewInt(2))

	// c4 = c4^2
	c4 = a.e2Square(c4)

	// c0 = c5*(u+1) + c3
	c0 := a.e2MulByCubicNonResidue(c5)
	c0 = a.e2Add(c0, c3)

	// z.B2 = c2 + c4 + c5 - c3
	b2 := a.e2Add(c2, c4)
	b2 = a.e2Add(b2, c5)
	b2 = a.e2Sub(b2, c3)

	return Ext{B0: c0, B1: c1, B2: b2}
}

// MulByE2Ext multiplies an Ext by an E2 element.
func (a *API) MulByE2Ext(x Ext, c E2) Ext {
	return Ext{
		B0: a.e2Mul(x.B0, c),
		B1: a.e2Mul(x.B1, c),
		B2: a.e2Mul(x.B2, c),
	}
}

// MulByFpExt multiplies an Ext by a base field element.
func (a *API) MulByFpExt(x Ext, c Element) Ext {
	return Ext{
		B0: a.e2MulByFp(x.B0, c),
		B1: a.e2MulByFp(x.B1, c),
		B2: a.e2MulByFp(x.B2, c),
	}
}

// MulConstExt multiplies an Ext by a constant.
func (a *API) MulConstExt(x Ext, c *big.Int) Ext {
	return Ext{
		B0: a.e2MulConst(x.B0, c),
		B1: a.e2MulConst(x.B1, c),
		B2: a.e2MulConst(x.B2, c),
	}
}

// ModReduceExt reduces an Ext element (no-op in native mode).
func (a *API) ModReduceExt(x Ext) Ext {
	if a.IsNative() {
		// in native mode, no reduction is necessary
		return x
	}
	return Ext{
		B0: E2{A0: a.ModReduce(x.B0.A0), A1: a.ModReduce(x.B0.A1)},
		B1: E2{A0: a.ModReduce(x.B1.A0), A1: a.ModReduce(x.B1.A1)},
		B2: E2{A0: a.ModReduce(x.B2.A0), A1: a.ModReduce(x.B2.A1)},
	}
}

// AddByBaseExt adds a base field element to the constant term.
func (a *API) AddByBaseExt(x Ext, y Element) Ext {
	return Ext{
		B0: E2{A0: a.Add(x.B0.A0, y), A1: x.B0.A1},
		B1: x.B1,
		B2: x.B2,
	}
}

// SumExt returns x + y + z...
func (a *API) SumExt(xs ...Ext) Ext {
	// One scratch slice reused across the six coordinate-wise reductions to
	// avoid allocating 6× len(xs) Elements per call on the hot witness path.
	coords := make([]Element, len(xs))

	sumCoord := func(get func(x Ext) Element) Element {
		for i := range xs {
			coords[i] = get(xs[i])
		}
		return a.Sum(coords...)
	}

	return Ext{
		B0: E2{
			A0: sumCoord(func(x Ext) Element { return x.B0.A0 }),
			A1: sumCoord(func(x Ext) Element { return x.B0.A1 }),
		},
		B1: E2{
			A0: sumCoord(func(x Ext) Element { return x.B1.A0 }),
			A1: sumCoord(func(x Ext) Element { return x.B1.A1 }),
		},
		B2: E2{
			A0: sumCoord(func(x Ext) Element { return x.B2.A0 }),
			A1: sumCoord(func(x Ext) Element { return x.B2.A1 }),
		},
	}
}

// MulByNonResidueExt multiplies x by v, where v is the irreducible cubic root
// generator (v^3 = u+1). Equivalent to a single-coordinate cyclic shift with
// (u+1) wrap on the highest slot.
func (a *API) MulByNonResidueExt(x Ext) Ext {
	return Ext{
		B0: a.e2MulByCubicNonResidue(x.B2),
		B1: x.B0,
		B2: x.B1,
	}
}

// --- Ext Comparison and Selection ---

// IsZeroExt returns 1 if x == 0, 0 otherwise.
func (a *API) IsZeroExt(x Ext) frontend.Variable {
	b0Zero := a.And(a.IsZero(x.B0.A0), a.IsZero(x.B0.A1))
	b1Zero := a.And(a.IsZero(x.B1.A0), a.IsZero(x.B1.A1))
	b2Zero := a.And(a.IsZero(x.B2.A0), a.IsZero(x.B2.A1))
	return a.And(a.And(b0Zero, b1Zero), b2Zero)
}

// SelectExt returns x if sel=1, y otherwise.
func (a *API) SelectExt(sel frontend.Variable, x, y Ext) Ext {
	return Ext{
		B0: E2{
			A0: a.Select(sel, x.B0.A0, y.B0.A0),
			A1: a.Select(sel, x.B0.A1, y.B0.A1),
		},
		B1: E2{
			A0: a.Select(sel, x.B1.A0, y.B1.A0),
			A1: a.Select(sel, x.B1.A1, y.B1.A1),
		},
		B2: E2{
			A0: a.Select(sel, x.B2.A0, y.B2.A0),
			A1: a.Select(sel, x.B2.A1, y.B2.A1),
		},
	}
}

// AssertIsEqualExt constrains x == y.
func (a *API) AssertIsEqualExt(x, y Ext) {
	a.AssertIsEqual(x.B0.A0, y.B0.A0)
	a.AssertIsEqual(x.B0.A1, y.B0.A1)
	a.AssertIsEqual(x.B1.A0, y.B1.A0)
	a.AssertIsEqual(x.B1.A1, y.B1.A1)
	a.AssertIsEqual(x.B2.A0, y.B2.A0)
	a.AssertIsEqual(x.B2.A1, y.B2.A1)
}

// --- Ext Division and Inverse ---

// InverseExt returns 1/x in the extension field.
func (a *API) InverseExt(x Ext) Ext {
	hint := a.inverseExtHint()
	res, err := a.NewHint(hint, extDegree,
		x.B0.A0, x.B0.A1, x.B1.A0, x.B1.A1, x.B2.A0, x.B2.A1)
	if err != nil {
		panic(err)
	}
	inv := a.extFromVars(res)

	// Verify: x * inv == 1
	product := a.MulExt(x, inv)
	a.AssertIsEqualExt(product, a.OneExt())
	return inv
}

// DivExt returns x / y in the extension field.
func (a *API) DivExt(x, y Ext) Ext {
	hint := a.divExtHint()
	res, err := a.NewHint(hint, extDegree,
		x.B0.A0, x.B0.A1, x.B1.A0, x.B1.A1, x.B2.A0, x.B2.A1,
		y.B0.A0, y.B0.A1, y.B1.A0, y.B1.A1, y.B2.A0, y.B2.A1)
	if err != nil {
		panic(err)
	}
	quot := a.extFromVars(res)

	// Verify: y * quot == x
	product := a.MulExt(y, quot)
	a.AssertIsEqualExt(product, x)
	return quot
}

// DivByBaseExt divides an Ext by a base field element.
func (a *API) DivByBaseExt(x Ext, y Element) Ext {
	return Ext{
		B0: E2{A0: a.Div(x.B0.A0, y), A1: a.Div(x.B0.A1, y)},
		B1: E2{A0: a.Div(x.B1.A0, y), A1: a.Div(x.B1.A1, y)},
		B2: E2{A0: a.Div(x.B2.A0, y), A1: a.Div(x.B2.A1, y)},
	}
}

const extDegree = field.ExtensionDegree

// extFromVars creates an Ext from 6 Vars.
func (a *API) extFromVars(v []Element) Ext {
	return Ext{
		B0: E2{A0: v[0], A1: v[1]},
		B1: E2{A0: v[2], A1: v[3]},
		B2: E2{A0: v[4], A1: v[5]},
	}
}

// --- Ext Exponentiation ---

// ExpExt computes x^n using square-and-multiply.
// Optimized for power-of-two exponents.
func (a *API) ExpExt(x Ext, n *big.Int) Ext {
	if n.Sign() == 0 {
		return a.OneExt()
	}
	if n.Cmp(big.NewInt(1)) == 0 {
		return x
	}

	// Fast path: power of two (only squaring needed)
	if n.BitLen() > 0 && new(big.Int).And(n, new(big.Int).Sub(n, big.NewInt(1))).Sign() == 0 {
		res := x
		for i := 0; i < n.BitLen()-1; i++ {
			res = a.SquareExt(res)
		}
		return res
	}

	// General case: square-and-multiply
	res := a.OneExt()
	nBytes := n.Bytes()
	for _, b := range nBytes {
		for j := 0; j < 8; j++ {
			c := (b >> (7 - j)) & 1
			res = a.SquareExt(res)
			if c == 1 {
				res = a.MulExt(res, x)
			}
		}
	}
	return res
}

// ExpVariableExponentExt computes x^exp where exp is a circuit variable.
func (a *API) ExpVariableExponentExt(x Ext, exp frontend.Variable, expNumBits int) Ext {
	expBits := a.nativeAPI.ToBinary(exp, expNumBits)
	res := a.OneExt()

	for i := len(expBits) - 1; i >= 0; i-- {
		if i != len(expBits)-1 {
			res = a.SquareExt(res)
		}
		tmp := a.MulExt(res, x)
		res = a.SelectExt(expBits[i], tmp, res)
	}
	return res
}

// --- Ext Debug ---

// PrintlnExt prints Ext variables for debugging.
func (a *API) PrintlnExt(vars ...Ext) {
	for i := range vars {
		a.Println(vars[i].B0.A0, vars[i].B0.A1, vars[i].B1.A0, vars[i].B1.A1, vars[i].B2.A0, vars[i].B2.A1)
	}
}

// --- Ext Hints ---

// NewHintExt calls a hint function with Ext inputs and outputs.
func (a *API) NewHintExt(f solver.Hint, nbOutputs int, inputs ...Ext) ([]Ext, error) {
	if a.IsNative() {
		flatInputs := make([]frontend.Variable, extDegree*len(inputs))
		for i, r := range inputs {
			flatInputs[extDegree*i+0] = r.B0.A0.Native()
			flatInputs[extDegree*i+1] = r.B0.A1.Native()
			flatInputs[extDegree*i+2] = r.B1.A0.Native()
			flatInputs[extDegree*i+3] = r.B1.A1.Native()
			flatInputs[extDegree*i+4] = r.B2.A0.Native()
			flatInputs[extDegree*i+5] = r.B2.A1.Native()
		}
		flatRes, err := a.nativeAPI.NewHint(f, extDegree*nbOutputs, flatInputs...)
		if err != nil {
			return nil, err
		}
		res := make([]Ext, nbOutputs)
		for i := range res {
			res[i] = Ext{
				B0: E2{A0: Element{V: flatRes[extDegree*i+0]}, A1: Element{V: flatRes[extDegree*i+1]}},
				B1: E2{A0: Element{V: flatRes[extDegree*i+2]}, A1: Element{V: flatRes[extDegree*i+3]}},
				B2: E2{A0: Element{V: flatRes[extDegree*i+4]}, A1: Element{V: flatRes[extDegree*i+5]}},
			}
		}
		return res, nil
	}

	flatInputs := make([]*emulated.Element[emulated.KoalaBear], extDegree*len(inputs))
	for i, r := range inputs {
		flatInputs[extDegree*i+0] = r.B0.A0.Emulated()
		flatInputs[extDegree*i+1] = r.B0.A1.Emulated()
		flatInputs[extDegree*i+2] = r.B1.A0.Emulated()
		flatInputs[extDegree*i+3] = r.B1.A1.Emulated()
		flatInputs[extDegree*i+4] = r.B2.A0.Emulated()
		flatInputs[extDegree*i+5] = r.B2.A1.Emulated()
	}
	flatRes, err := a.emulatedAPI.NewHint(f, extDegree*nbOutputs, flatInputs...)
	if err != nil {
		return nil, err
	}
	res := make([]Ext, nbOutputs)
	for i := range res {
		res[i] = Ext{
			B0: E2{A0: Element{EV: *flatRes[extDegree*i+0]}, A1: Element{EV: *flatRes[extDegree*i+1]}},
			B1: E2{A0: Element{EV: *flatRes[extDegree*i+2]}, A1: Element{EV: *flatRes[extDegree*i+3]}},
			B2: E2{A0: Element{EV: *flatRes[extDegree*i+4]}, A1: Element{EV: *flatRes[extDegree*i+5]}},
		}
	}
	return res, nil
}

// --- Hint implementations ---

func inverseE2Hint(_ *big.Int, inputs []*big.Int, res []*big.Int) error {
	var a, c extensions.E2
	a.A0.SetBigInt(inputs[0])
	a.A1.SetBigInt(inputs[1])
	c.Inverse(&a)
	c.A0.BigInt(res[0])
	c.A1.BigInt(res[1])
	return nil
}

func extFromInputs(inputs []*big.Int) (e field.Ext) {
	e.B0.A0.SetBigInt(inputs[0])
	e.B0.A1.SetBigInt(inputs[1])
	e.B1.A0.SetBigInt(inputs[2])
	e.B1.A1.SetBigInt(inputs[3])
	e.B2.A0.SetBigInt(inputs[4])
	e.B2.A1.SetBigInt(inputs[5])
	return e
}

func extToOutputs(e field.Ext, out []*big.Int) {
	e.B0.A0.BigInt(out[0])
	e.B0.A1.BigInt(out[1])
	e.B1.A0.BigInt(out[2])
	e.B1.A1.BigInt(out[3])
	e.B2.A0.BigInt(out[4])
	e.B2.A1.BigInt(out[5])
}

func inverseExtHintNative(_ *big.Int, inputs []*big.Int, res []*big.Int) error {
	a := extFromInputs(inputs)
	var c field.Ext
	c.Inverse(&a)
	extToOutputs(c, res)
	return nil
}

func inverseExtHintEmulated(_ *big.Int, inputs []*big.Int, output []*big.Int) error {
	return emulated.UnwrapHint(inputs, output, inverseExtHintNative)
}

func (a *API) inverseExtHint() solver.Hint {
	if a.IsNative() {
		return inverseExtHintNative
	}
	return inverseExtHintEmulated
}

func divExtHintNative(_ *big.Int, inputs []*big.Int, res []*big.Int) error {
	x := extFromInputs(inputs[:extDegree])
	y := extFromInputs(inputs[extDegree : 2*extDegree])
	var c field.Ext
	c.Div(&x, &y)
	extToOutputs(c, res)
	return nil
}

func divExtHintEmulated(_ *big.Int, inputs []*big.Int, output []*big.Int) error {
	return emulated.UnwrapHint(inputs, output, divExtHintNative)
}

func (a *API) divExtHint() solver.Hint {
	if a.IsNative() {
		return divExtHintNative
	}
	return divExtHintEmulated
}

func mulExtHintNative(_ *big.Int, inputs []*big.Int, res []*big.Int) error {
	x := extFromInputs(inputs[:extDegree])
	y := extFromInputs(inputs[extDegree : 2*extDegree])
	var c field.Ext
	c.Mul(&x, &y)
	extToOutputs(c, res)
	return nil
}

func mulExtHintEmulated(_ *big.Int, inputs []*big.Int, output []*big.Int) error {
	return emulated.UnwrapHint(inputs, output, mulExtHintNative)
}

// IsConstantZeroExt returns true if e is a constant zero element.
func (a *API) IsConstantZeroExt(e Ext) bool {
	return a.IsConstantZero(e.B0.A0) &&
		a.IsConstantZero(e.B0.A1) &&
		a.IsConstantZero(e.B1.A0) &&
		a.IsConstantZero(e.B1.A1) &&
		a.IsConstantZero(e.B2.A0) &&
		a.IsConstantZero(e.B2.A1)
}

// BaseValueOfElement returns true if the Ext element actually represents a
// base field element and returns it as an [Element]. The function checks that
// every non-constant coordinate is a zero constant.
func (a *API) BaseValueOfElement(e Ext) (*Element, bool) {
	if !a.IsConstantZero(e.B0.A1) ||
		!a.IsConstantZero(e.B1.A0) ||
		!a.IsConstantZero(e.B1.A1) ||
		!a.IsConstantZero(e.B2.A0) ||
		!a.IsConstantZero(e.B2.A1) {
		return nil, false
	}
	return &e.B0.A0, true
}
