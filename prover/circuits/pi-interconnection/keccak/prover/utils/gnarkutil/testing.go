package gnarkutil

import (
	"testing"
	"unsafe"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/test"
	"github.com/stretchr/testify/assert"
)

// This is a compile time assertion that the struct GnarkTestCircuit implements
// the interface frontend.Circuit (or its pointer).
var _ frontend.Circuit = &gnarkTestCircuit{}

// TestDefineFunc represents a gnark's Define function that should be
// self-standing and does not uses a witness (all variables are either constants
// or internal).
type TestDefineFunc func(frontend.API) error

// AssertCircuitSolved asserts that the testing circuit defined by def is
// satisfied. This is used to check that a specified "gnark" circuit functions
// works as expected without having to implement a custom circuit for every test.
func AssertCircuitSolved(t *testing.T, def TestDefineFunc) {
	var (
		circ = &gnarkTestCircuit{DefineFunc: unsafe.Pointer(&def)}
		assi = &gnarkTestCircuit{DefineFunc: unsafe.Pointer(&def)}
	)

	err := test.IsSolved(circ, assi, ecc.BLS12_377.ScalarField())
	assert.NoError(t, err)
}

// gnarkTestCircuit is a generic circuit that allows easy test implementation
// without having to worry about writing a unique struct for every gnark
// function.
//
// The implementation is unsafe and should be restricted to unit-tests as this
// using unsafe function pointers under the hood.
type gnarkTestCircuit struct {
	// DefineFunc is a pointer to a function of type [TestDefineFunc]. The reason
	// we need an unsafe pointer there is that gnark will attempt to clone
	// the structure during [test.IsSolved] and doing so is not possible if the
	// struct contains function directly.
	//
	// The struct is additionally purposefully tagged to be ignored by the gnark
	// struct parser does not try to walk into the field. Removing the tag
	// will result in an error because the circuit parser throws errors upon
	// walking through an unsafe.Pointer..
	DefineFunc unsafe.Pointer `gnark:"-"`
}

// Define implements the [frontend.Circuit] interface. The general
// implementation is deferred to the user's provided define function.
func (c *gnarkTestCircuit) Define(api frontend.API) error {
	define := *(*TestDefineFunc)(c.DefineFunc)
	return define(api)
}
