package circuit

import (
	"math/big"

	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/math/emulated"
)

// KoalaBearAPI provides arithmetic operations for KoalaBear circuit variables.
// KoalaBear is always emulated in the circuit's field: the gnark circuits only
// wrap the final proof (KoalaBear-in-BN254), so there is no native mode.
type KoalaBearAPI struct {
	frontendAPI frontend.API
	emulatedAPI *emulated.Field[emulated.KoalaBear]
}

// NewAPI creates a [KoalaBearAPI] for the given gnark frontend. It panics if
// the circuit's field is KoalaBear itself, as native KoalaBear circuits are
// not supported.
func NewAPI(api frontend.API) *KoalaBearAPI {
	if api.Compiler().Field().Cmp(koalabearModulus) == 0 {
		panic("koalabear circuit: native KoalaBear circuits are not supported, compile over a different field (e.g. BN254)")
	}
	f, err := emulated.NewField[emulated.KoalaBear](api)
	if err != nil {
		panic(err)
	}
	return &KoalaBearAPI{frontendAPI: api, emulatedAPI: f}
}

// Frontend returns the underlying gnark frontend API.
func (a *KoalaBearAPI) Frontend() frontend.API {
	return a.frontendAPI
}

// EmulatedField returns the emulated field API.
func (a *KoalaBearAPI) EmulatedField() *emulated.Field[emulated.KoalaBear] {
	return a.emulatedAPI
}

// GetFrontendVariable extracts a frontend.Variable from a Var.
func (a *KoalaBearAPI) GetFrontendVariable(v Element) frontend.Variable {
	return v.EV.Limbs[0]
}

// --- Constants ---

// Const creates a circuit constant from an int64.
// Use this for compile-time known values. More efficient than using NewVar
// for constants as gnark can optimize constant operations.
func (a *KoalaBearAPI) Const(c int64) Element {
	return Element{EV: *a.emulatedAPI.NewElement(c)}
}

// ConstBig creates a circuit constant from a big.Int.
func (a *KoalaBearAPI) ConstBig(c *big.Int) Element {
	return Element{EV: *a.emulatedAPI.NewElement(c)}
}

// Zero returns the additive identity (0).
func (a *KoalaBearAPI) Zero() Element {
	return a.Const(0)
}

// One returns the multiplicative identity (1).
func (a *KoalaBearAPI) One() Element {
	return a.Const(1)
}

// FromFrontendVar wraps an existing frontend.Variable as a Var.
func (a *KoalaBearAPI) FromFrontendVar(v frontend.Variable) Element {
	return Element{EV: emulated.Element[emulated.KoalaBear]{Limbs: []frontend.Variable{v}}}
}

// --- Arithmetic Operations ---

// Add returns a + b.
func (a *KoalaBearAPI) Add(x, y Element) Element {
	return Element{EV: *a.emulatedAPI.Add(x.Emulated(), y.Emulated())}
}

// Sum returns a + b + c + d + ...
func (a *KoalaBearAPI) Sum(xs ...Element) Element {
	toSum := make([]*emulated.Element[emulated.KoalaBear], len(xs))
	for i := range xs {
		toSum[i] = &xs[i].EV
	}

	sum := a.emulatedAPI.Sum(toSum...)
	return Element{EV: *sum}
}

// Sub returns x - y.
func (a *KoalaBearAPI) Sub(x, y Element) Element {
	return Element{EV: *a.emulatedAPI.Sub(x.Emulated(), y.Emulated())}
}

// Neg returns -x.
func (a *KoalaBearAPI) Neg(x Element) Element {
	return Element{EV: *a.emulatedAPI.Neg(x.Emulated())}
}

// Mul returns x * y.
func (a *KoalaBearAPI) Mul(x, y Element) Element {
	return Element{EV: *a.emulatedAPI.Mul(x.Emulated(), y.Emulated())}
}

// ModReduce reduces x modulo the KoalaBear field modulus.
func (a *KoalaBearAPI) ModReduce(x Element) Element {
	reduced := a.emulatedAPI.Reduce(x.Emulated())
	return Element{EV: *reduced}
}

// MulConst returns x * c where c is a compile-time constant.
// More efficient than Mul(x, Const(c)) as it avoids range checks in emulated mode.
func (a *KoalaBearAPI) MulConst(x Element, c *big.Int) Element {
	return Element{EV: *a.emulatedAPI.MulConst(x.Emulated(), c)}
}

// MulConstInt returns x * c where c is an int64 constant.
func (a *KoalaBearAPI) MulConstInt(x Element, c int64) Element {
	return a.MulConst(x, big.NewInt(c))
}

// Inverse returns 1/x.
func (a *KoalaBearAPI) Inverse(x Element) Element {
	return Element{EV: *a.emulatedAPI.Inverse(x.Emulated())}
}

// Div returns x / y.
func (a *KoalaBearAPI) Div(x, y Element) Element {
	return Element{EV: *a.emulatedAPI.Div(x.Emulated(), y.Emulated())}
}

// --- Comparison and Selection ---

// IsZero returns 1 if x == 0, 0 otherwise.
func (a *KoalaBearAPI) IsZero(x Element) frontend.Variable {
	return a.emulatedAPI.IsZero(x.Emulated())
}

// Select returns x if sel=1, y otherwise.
func (a *KoalaBearAPI) Select(sel frontend.Variable, x, y Element) Element {
	return Element{EV: *a.emulatedAPI.Select(sel, x.Emulated(), y.Emulated())}
}

// Lookup2 returns i0 if (b0,b1)=(0,0), i1 if (0,1), i2 if (1,0), i3 if (1,1).
func (a *KoalaBearAPI) Lookup2(b0, b1 frontend.Variable, i0, i1, i2, i3 Element) Element {
	return Element{EV: *a.emulatedAPI.Lookup2(
		b0, b1,
		i0.Emulated(), i1.Emulated(), i2.Emulated(), i3.Emulated())}
}

// Mux returns inputs[sel].
func (a *KoalaBearAPI) Mux(sel frontend.Variable, inputs ...Element) Element {
	emulatedInputs := make([]*emulated.Element[emulated.KoalaBear], len(inputs))
	for i := range emulatedInputs {
		emulatedInputs[i] = inputs[i].Emulated()
	}
	res := a.emulatedAPI.Mux(sel, emulatedInputs...)
	return Element{EV: *res}
}

// --- Assertions ---

// AssertIsEqual constrains x == y.
func (a *KoalaBearAPI) AssertIsEqual(x, y Element) {
	a.emulatedAPI.AssertIsEqual(x.Emulated(), y.Emulated())
}

// AssertIsDifferent constrains x != y.
func (a *KoalaBearAPI) AssertIsDifferent(x, y Element) {
	a.emulatedAPI.AssertIsDifferent(x.Emulated(), y.Emulated())
}

// AssertIsLessOrEqual constrains x <= y.
func (a *KoalaBearAPI) AssertIsLessOrEqual(x, y Element) {
	a.emulatedAPI.AssertIsLessOrEqual(x.Emulated(), y.Emulated())
}

// AssertIsBoolean constrains x == 0 or x == 1.
func (a *KoalaBearAPI) AssertIsBoolean(x Element) {
	a.emulatedAPI.AssertIsEqual(a.emulatedAPI.Mul(&x.EV, &x.EV), &x.EV)
}

// --- Binary Operations ---

// ToBinary returns the binary decomposition of x.
func (a *KoalaBearAPI) ToBinary(x Element) []frontend.Variable {
	return a.emulatedAPI.ToBits(x.Emulated())
}

// ToBinaryCanonical returns the little-endian bits of the canonical (fully
// reduced, < modulus) value of x. Unlike [KoalaBearAPI.ToBinary], which in emulated mode
// decomposes the possibly unreduced internal representation, the result is
// unique for a given field element, so it is safe to derive indices or
// challenges from it.
func (a *KoalaBearAPI) ToBinaryCanonical(x Element) []frontend.Variable {
	return a.emulatedAPI.ToBitsCanonical(x.Emulated())
}

// FromBinary constructs a Var from binary bits.
func (a *KoalaBearAPI) FromBinary(bits ...frontend.Variable) Element {
	return Element{EV: *a.emulatedAPI.FromBits(bits...)}
}

// And returns a AND b (bitwise).
func (a *KoalaBearAPI) And(x, y frontend.Variable) frontend.Variable {
	return a.frontendAPI.And(x, y)
}

// Or returns a OR b (bitwise).
func (a *KoalaBearAPI) Or(x, y frontend.Variable) frontend.Variable {
	return a.frontendAPI.Or(x, y)
}

// Xor returns a XOR b (bitwise).
func (a *KoalaBearAPI) Xor(x, y frontend.Variable) frontend.Variable {
	return a.frontendAPI.Xor(x, y)
}

// --- Hints ---

// NewHint calls a hint function with Var inputs and outputs.
func (a *KoalaBearAPI) NewHint(f solver.Hint, nbOutputs int, inputs ...Element) ([]Element, error) {
	emulatedInputs := make([]*emulated.Element[emulated.KoalaBear], len(inputs))
	for i, r := range inputs {
		emulatedInputs[i] = r.Emulated()
	}
	emulatedRes, err := a.emulatedAPI.NewHint(f, nbOutputs, emulatedInputs...)
	if err != nil {
		return nil, err
	}
	res := make([]Element, nbOutputs)
	for i, r := range emulatedRes {
		res[i] = Element{EV: *r}
	}
	return res, nil
}

// --- Debug ---

// Println prints variables for debugging.
func (a *KoalaBearAPI) Println(vars ...Element) {
	for i := range vars {
		reduced := a.emulatedAPI.Reduce(&vars[i].EV)
		for j := range reduced.Limbs {
			a.frontendAPI.Println(reduced.Limbs[j])
		}
	}
}
