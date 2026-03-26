package koalagnark

import (
	"math/big"

	"github.com/consensys/gnark-crypto/field/koalabear"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/math/emulated"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// koalabearModulus caches koalabear.Modulus() to avoid repeated allocations.
var koalabearModulus = koalabear.Modulus()

// Element represents a circuit variable over the KoalaBear base field.
// It abstracts over native and emulated representations, allowing the same
// circuit code to work in both native KoalaBear circuits and emulated circuits.
type Element struct {
	// V is used when the circuit's native field is KoalaBear.
	// Exported for gnark witness serialization.
	V frontend.Variable

	// EV is used when KoalaBear is emulated in another field.
	// Exported for gnark witness serialization.
	EV emulated.Element[emulated.KoalaBear]
}

// Octuplet is an array of 8 Var elements.
type Octuplet [8]Element

// NewElement creates a Var for witness assignment from any value.
// Use this when initializing circuit struct fields with witness values.
//
// For in-circuit constants, use [API.Const] instead.
func NewElement(v any) Element {
	switch v := v.(type) {
	case Element:
		return v
	case *Element:
		return *v
	}

	var res Element
	res.EV = emulated.ValueOf[emulated.KoalaBear](v)
	res.V = v
	return res
}

// NewElementFromKoala creates a Var from a field.Element for witness assignment.
func NewElementFromKoala(v field.Element) Element {
	return NewElement(v.Uint64())
}

// WrapFrontendVariable wraps an existing frontend.Variable as a Var.
// Use this for variables that come from other gnark APIs.
func WrapFrontendVariable(v frontend.Variable) Element {
	switch v.(type) {
	case Element, *Element:
		panic("attempted to wrap a koalagnark.Var into a koalagnark.Var")
	}

	var res Element
	res.V = v
	res.EV = emulated.Element[emulated.KoalaBear]{Limbs: []frontend.Variable{v}}
	return res
}

// Native returns the native frontend.Variable representation.
// Panics if the variable cannot be represented as a single native variable.
func (v *Element) Native() frontend.Variable {
	if v.V != nil {
		return v.V
	}
	if len(v.EV.Limbs) == 1 {
		return v.EV.Limbs[0]
	}
	utils.Panic("unexpected shape for Var: %++v", v)
	return nil // unreachable
}

// Emulated returns the emulated element representation.
func (v *Element) Emulated() *emulated.Element[emulated.KoalaBear] {
	return &v.EV
}

// IsEmpty returns true if the Var has not been initialized.
func (v *Element) IsEmpty() bool {
	return v.V == nil && len(v.EV.Limbs) == 0
}

// Initialize prepares the Var for the given field modulus.
func (v *Element) Initialize(modulus *big.Int) {
	if modulus.Cmp(koalabearModulus) == 0 {
		return
	}
	v.EV.Initialize(modulus)
}

// NativeArray converts an Octuplet to an array of native frontend.Variables.
func (o Octuplet) NativeArray() [8]frontend.Variable {
	res := [8]frontend.Variable{}
	for i := range res {
		res[i] = o[i].Native()
		if res[i] == nil {
			panic("Var is nil")
		}
	}
	return res
}

// In case the
var (
	zeroKoalaFr = field.Zero()
	oneKoalaFr  = field.One()
	oneBigInt   = big.NewInt(1)
)

// IsConstantZero returns true if the variable represent a constant value equal
// to zero.
func (api *API) IsConstantZero(v Element) bool {

	if api.IsNative() {

		if v.V == nil {
			panic("unexpected, api is native but not the field element")
		}

		f, ok := api.nativeAPI.Compiler().ConstantValue(v.V)
		if !ok {
			return false
		}

		return f.Sign() == 0
	}

	for i := range v.EV.Limbs {
		g, ok := api.nativeAPI.Compiler().ConstantValue(v.EV.Limbs[i])
		if !ok {
			return false
		}

		if g.Sign() != 0 {
			return false
		}
	}

	return true
}
