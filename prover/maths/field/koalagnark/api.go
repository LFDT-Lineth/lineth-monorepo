package koalagnark

import (
	"math/big"

	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/math/emulated"
	"github.com/consensys/gnark/std/selector"
)

// VType indicates whether the API is operating in native or emulated mode.
type VType uint

const (
	// Native indicates the circuit's native field is KoalaBear.
	Native VType = iota
	// Emulated indicates KoalaBear is being emulated in another field.
	Emulated
)

// API provides arithmetic operations for KoalaBear circuit variables.
// It automatically detects whether to use native or emulated arithmetic
// based on the circuit's field.
type API struct {
	nativeAPI   frontend.API
	emulatedAPI *emulated.Field[emulated.KoalaBear]
}

// NewAPI creates an API for the given gnark frontend.
// It automatically detects whether to use native or emulated arithmetic.
func NewAPI(api frontend.API) *API {
	ff := api.Compiler().Field()
	if ff.Cmp(koalabearModulus) == 0 {
		return &API{nativeAPI: api}
	}
	f, err := emulated.NewField[emulated.KoalaBear](api)
	if err != nil {
		panic(err)
	}
	return &API{nativeAPI: api, emulatedAPI: f}
}

// Type returns whether the API is operating in native or emulated mode.
func (a *API) Type() VType {
	if a.emulatedAPI == nil {
		return Native
	}
	return Emulated
}

// IsNative returns true if the API is operating in native mode.
func (a *API) IsNative() bool {
	return a.emulatedAPI == nil
}

// Frontend returns the underlying gnark frontend API.
func (a *API) Frontend() frontend.API {
	return a.nativeAPI
}

// EmulatedField returns the emulated field API, or nil if in native mode.
func (a *API) EmulatedField() *emulated.Field[emulated.KoalaBear] {
	return a.emulatedAPI
}

// GetFrontendVariable extracts a frontend.Variable from a Var.
func (a *API) GetFrontendVariable(v Element) frontend.Variable {
	if a.emulatedAPI == nil {
		return v.V
	}
	return v.EV.Limbs[0]
}

// --- Constants ---

// Const creates a circuit constant from an int64.
// Use this for compile-time known values. More efficient than using NewVar
// for constants as gnark can optimize constant operations.
func (a *API) Const(c int64) Element {
	if a.IsNative() {
		return Element{V: c}
	}
	return Element{EV: *a.emulatedAPI.NewElement(c)}
}

// ConstBig creates a circuit constant from a big.Int.
func (a *API) ConstBig(c *big.Int) Element {
	if a.IsNative() {
		return Element{V: c}
	}
	return Element{EV: *a.emulatedAPI.NewElement(c)}
}

// Zero returns the additive identity (0).
func (a *API) Zero() Element {
	return a.Const(0)
}

// One returns the multiplicative identity (1).
func (a *API) One() Element {
	return a.Const(1)
}

// FromFrontendVar wraps an existing frontend.Variable as a Var.
func (a *API) FromFrontendVar(v frontend.Variable) Element {
	if a.IsNative() {
		return Element{V: v}
	}
	return Element{EV: emulated.Element[emulated.KoalaBear]{Limbs: []frontend.Variable{v}}}
}

// --- Arithmetic Operations ---

// Add returns a + b.
func (a *API) Add(x, y Element) Element {
	if a.IsNative() {
		return Element{V: a.nativeAPI.Add(x.Native(), y.Native())}
	}
	return Element{EV: *a.emulatedAPI.Add(x.Emulated(), y.Emulated())}
}

// Sum returns a + b + c + d + ...
func (a *API) Sum(xs ...Element) Element {
	if a.IsNative() {
		res := frontend.Variable(0)
		for _, x := range xs {
			res = a.nativeAPI.Add(res, x.Native())
		}
		return Element{V: res}
	}

	toSum := make([]*emulated.Element[emulated.KoalaBear], len(xs))
	for i := range xs {
		toSum[i] = &xs[i].EV
	}

	sum := a.emulatedAPI.Sum(toSum...)
	return Element{EV: *sum}
}

// Sub returns x - y.
func (a *API) Sub(x, y Element) Element {
	if a.IsNative() {
		return Element{V: a.nativeAPI.Sub(x.Native(), y.Native())}
	}
	return Element{EV: *a.emulatedAPI.Sub(x.Emulated(), y.Emulated())}
}

// Neg returns -x.
func (a *API) Neg(x Element) Element {
	if a.IsNative() {
		return Element{V: a.nativeAPI.Neg(x.Native())}
	}
	return Element{EV: *a.emulatedAPI.Neg(x.Emulated())}
}

// Mul returns x * y.
func (a *API) Mul(x, y Element) Element {
	if a.IsNative() {
		return Element{V: a.nativeAPI.Mul(x.Native(), y.Native())}
	}
	return Element{EV: *a.emulatedAPI.Mul(x.Emulated(), y.Emulated())}
}

// ModReduce reduces x modulo the KoalaBear field modulus.
func (a *API) ModReduce(x Element) Element {
	if a.IsNative() {
		// in native mode, no reduction is necessary
		return x
	}
	reduced := a.emulatedAPI.Reduce(x.Emulated())
	return Element{EV: *reduced}
}

// MulConst returns x * c where c is a compile-time constant.
// More efficient than Mul(x, Const(c)) as it avoids range checks in emulated mode.
func (a *API) MulConst(x Element, c *big.Int) Element {
	if a.IsNative() {
		return Element{V: a.nativeAPI.Mul(x.Native(), c)}
	}
	return Element{EV: *a.emulatedAPI.MulConst(x.Emulated(), c)}
}

// MulConstInt returns x * c where c is an int64 constant.
func (a *API) MulConstInt(x Element, c int64) Element {
	return a.MulConst(x, big.NewInt(c))
}

// Inverse returns 1/x.
func (a *API) Inverse(x Element) Element {
	if a.IsNative() {
		return Element{V: a.nativeAPI.Inverse(x.Native())}
	}
	return Element{EV: *a.emulatedAPI.Inverse(x.Emulated())}
}

// Div returns x / y.
func (a *API) Div(x, y Element) Element {
	if a.IsNative() {
		return Element{V: a.nativeAPI.Div(x.Native(), y.Native())}
	}
	return Element{EV: *a.emulatedAPI.Div(x.Emulated(), y.Emulated())}
}

// --- Comparison and Selection ---

// IsZero returns 1 if x == 0, 0 otherwise.
func (a *API) IsZero(x Element) frontend.Variable {
	if a.IsNative() {
		return a.nativeAPI.IsZero(x.Native())
	}
	return a.emulatedAPI.IsZero(x.Emulated())
}

// Select returns x if sel=1, y otherwise.
func (a *API) Select(sel frontend.Variable, x, y Element) Element {
	if a.IsNative() {
		return Element{V: a.nativeAPI.Select(sel, x.Native(), y.Native())}
	}
	return Element{EV: *a.emulatedAPI.Select(sel, x.Emulated(), y.Emulated())}
}

// Lookup2 returns i0 if (b0,b1)=(0,0), i1 if (0,1), i2 if (1,0), i3 if (1,1).
func (a *API) Lookup2(b0, b1 frontend.Variable, i0, i1, i2, i3 Element) Element {
	if a.IsNative() {
		return Element{V: a.nativeAPI.Lookup2(
			b0, b1,
			i0.Native(), i1.Native(), i2.Native(), i3.Native())}
	}
	return Element{EV: *a.emulatedAPI.Lookup2(
		b0, b1,
		i0.Emulated(), i1.Emulated(), i2.Emulated(), i3.Emulated())}
}

// Mux returns inputs[sel].
func (a *API) Mux(sel frontend.Variable, inputs ...Element) Element {
	if a.IsNative() {
		nativeInputs := make([]frontend.Variable, len(inputs))
		for i := range nativeInputs {
			nativeInputs[i] = inputs[i].Native()
		}
		res := selector.Mux(a.nativeAPI, sel, nativeInputs...)
		return Element{V: res}
	}
	emulatedInputs := make([]*emulated.Element[emulated.KoalaBear], len(inputs))
	for i := range emulatedInputs {
		emulatedInputs[i] = inputs[i].Emulated()
	}
	res := a.emulatedAPI.Mux(sel, emulatedInputs...)
	return Element{EV: *res}
}

// --- Assertions ---

// AssertIsEqual constrains x == y.
func (a *API) AssertIsEqual(x, y Element) {
	if a.IsNative() {
		a.nativeAPI.AssertIsEqual(x.Native(), y.Native())
	} else {
		a.emulatedAPI.AssertIsEqual(x.Emulated(), y.Emulated())
	}
}

// AssertIsDifferent constrains x != y.
func (a *API) AssertIsDifferent(x, y Element) {
	if a.IsNative() {
		a.nativeAPI.AssertIsDifferent(x.Native(), y.Native())
	} else {
		a.emulatedAPI.AssertIsDifferent(x.Emulated(), y.Emulated())
	}
}

// AssertIsLessOrEqual constrains x <= y.
func (a *API) AssertIsLessOrEqual(x, y Element) {
	if a.IsNative() {
		a.nativeAPI.AssertIsLessOrEqual(x.Native(), y.Native())
	} else {
		a.emulatedAPI.AssertIsLessOrEqual(x.Emulated(), y.Emulated())
	}
}

// AssertIsBoolean constrains x == 0 or x == 1.
func (a *API) AssertIsBoolean(x Element) {
	if a.IsNative() {
		a.nativeAPI.AssertIsBoolean(x.Native())
	} else {
		a.emulatedAPI.AssertIsEqual(a.emulatedAPI.Mul(&x.EV, &x.EV), &x.EV)
	}
}

// --- Binary Operations ---

// ToBinary returns the binary decomposition of x.
func (a *API) ToBinary(x Element, n ...int) []frontend.Variable {
	if a.IsNative() {
		return a.nativeAPI.ToBinary(x.Native(), n...)
	}
	return a.emulatedAPI.ToBits(x.Emulated())
}

// FromBinary constructs a Var from binary bits.
func (a *API) FromBinary(bits ...frontend.Variable) Element {
	if a.IsNative() {
		return Element{V: a.nativeAPI.FromBinary(bits...)}
	}
	return Element{EV: *a.emulatedAPI.FromBits(bits...)}
}

// And returns a AND b (bitwise).
func (a *API) And(x, y frontend.Variable) frontend.Variable {
	return a.nativeAPI.And(x, y)
}

// Or returns a OR b (bitwise).
func (a *API) Or(x, y frontend.Variable) frontend.Variable {
	return a.nativeAPI.Or(x, y)
}

// Xor returns a XOR b (bitwise).
func (a *API) Xor(x, y frontend.Variable) frontend.Variable {
	return a.nativeAPI.Xor(x, y)
}

// --- Hints ---

// NewHint calls a hint function with Var inputs and outputs.
func (a *API) NewHint(f solver.Hint, nbOutputs int, inputs ...Element) ([]Element, error) {
	if a.IsNative() {
		nativeInputs := make([]frontend.Variable, len(inputs))
		for i, r := range inputs {
			nativeInputs[i] = r.Native()
		}
		nativeRes, err := a.nativeAPI.NewHint(f, nbOutputs, nativeInputs...)
		if err != nil {
			return nil, err
		}
		res := make([]Element, nbOutputs)
		for i, r := range nativeRes {
			res[i] = Element{V: r}
		}
		return res, nil
	}

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
func (a *API) Println(vars ...Element) {
	if a.IsNative() {
		for i := range vars {
			a.nativeAPI.Println(vars[i].Native())
		}
	} else {
		for i := range vars {
			v := vars[i]
			reduced := a.emulatedAPI.Reduce(&v.EV) // Use the return value!
			for j := 0; j < len(reduced.Limbs); j++ {
				a.nativeAPI.Println(reduced.Limbs[j])
			}
		}
	}
}
