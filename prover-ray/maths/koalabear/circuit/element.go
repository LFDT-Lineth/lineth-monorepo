package circuit

import (
	"math/big"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/gnark-crypto/field/koalabear"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/math/emulated"
)

// koalabearModulus caches koalabear.Modulus() to avoid repeated allocations.
var koalabearModulus = koalabear.Modulus()

// Element represents a circuit variable over the KoalaBear base field,
// emulated in the circuit's field.
type Element struct {
	// EV is the emulated representation.
	// Exported for gnark witness serialization.
	EV emulated.Element[emulated.KoalaBear]
}

// Octuplet is an array of 8 Var elements.
type Octuplet [8]Element

// NewElement creates a Var for witness assignment from any value.
// Use this when initializing circuit struct fields with witness values.
//
// For in-circuit constants, use [KoalaBearAPI.Const] instead.
func NewElement(v any) Element {
	switch v := v.(type) {
	case Element:
		return v
	case *Element:
		return *v
	}

	var res Element
	res.EV = emulated.ValueOf[emulated.KoalaBear](v)
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
	res.EV = emulated.Element[emulated.KoalaBear]{Limbs: []frontend.Variable{v}}
	return res
}

// Emulated returns the emulated element representation.
func (v *Element) Emulated() *emulated.Element[emulated.KoalaBear] {
	return &v.EV
}

// IsEmpty returns true if the Var has not been initialized.
func (v *Element) IsEmpty() bool {
	return len(v.EV.Limbs) == 0
}

// Initialize prepares the Var for the given field modulus.
func (v *Element) Initialize(modulus *big.Int) {
	v.EV.Initialize(modulus)
}

// IsConstantZero returns true if the variable represent a constant value equal
// to zero.
func (a *KoalaBearAPI) IsConstantZero(v Element) bool {

	for i := range v.EV.Limbs {
		g, ok := a.frontendAPI.Compiler().ConstantValue(v.EV.Limbs[i])
		if !ok {
			return false
		}

		if g.Sign() != 0 {
			return false
		}
	}

	return true
}
