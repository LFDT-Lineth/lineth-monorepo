package koalagnark

import (
	"math/big"

	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/math/emulated"
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

// Select returns x if sel=1, y otherwise.
func (a *API) Select(sel frontend.Variable, x, y Element) Element {
	if a.IsNative() {
		return Element{V: a.nativeAPI.Select(sel, x.Native(), y.Native())}
	}
	return Element{EV: *a.emulatedAPI.Select(sel, x.Emulated(), y.Emulated())}
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
