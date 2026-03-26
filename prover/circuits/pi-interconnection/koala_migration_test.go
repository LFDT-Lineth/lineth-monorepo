package pi_interconnection

import (
	"math/big"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	fr377 "github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	"github.com/consensys/gnark-crypto/field/koalabear"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/test"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// isActuallyKoalaHashCircuit wraps the unexported isActuallyKoalaHash so it
// can be exercised through test.IsSolved.
type isActuallyKoalaHashCircuit struct {
	Hash     [2]frontend.Variable
	Expected frontend.Variable // 1 if koala, 0 if not
}

func (c *isActuallyKoalaHashCircuit) Define(api frontend.API) error {
	got := isActuallyKoalaHash(api, c.Hash)
	api.AssertIsEqual(got, c.Expected)
	return nil
}

// koalaOctupletToHalves converts 8 koalabear field elements (each < 0x7f000001)
// into two 128-bit big.Ints representing the hi and lo halves of the 256-bit hash.
func koalaOctupletToHalves(elems [8]uint32) (hi, lo *big.Int) {
	var buf [32]byte
	for i, v := range elems {
		buf[4*i] = byte(v >> 24)
		buf[4*i+1] = byte(v >> 16)
		buf[4*i+2] = byte(v >> 8)
		buf[4*i+3] = byte(v)
	}
	hi = new(big.Int).SetBytes(buf[:16])
	lo = new(big.Int).SetBytes(buf[16:])
	return
}

func TestIsActuallyKoalaHash_ValidKoalaBear(t *testing.T) {
	var circuit isActuallyKoalaHashCircuit
	mod := koalabear.Modulus().Uint64() // 0x7f000001 = 2130706433

	cases := []struct {
		name  string
		elems [8]uint32
	}{
		{
			name:  "all zeros",
			elems: [8]uint32{0, 0, 0, 0, 0, 0, 0, 0},
		},
		{
			name:  "all ones",
			elems: [8]uint32{1, 1, 1, 1, 1, 1, 1, 1},
		},
		{
			name:  "max valid (modulus - 1)",
			elems: [8]uint32{uint32(mod - 1), uint32(mod - 1), uint32(mod - 1), uint32(mod - 1), uint32(mod - 1), uint32(mod - 1), uint32(mod - 1), uint32(mod - 1)},
		},
		{
			name:  "mixed valid values",
			elems: [8]uint32{0, uint32(mod - 1), 42, 1000000, uint32(mod / 2), 7, 12345678, uint32(mod - 2)},
		},
		{
			name: "realistic koalabear octuplet",
			elems: func() [8]uint32 {
				var res [8]uint32
				for i := range res {
					var e field.Element
					e.SetUint64(uint64(i*1000 + 1))
					res[i] = uint32(e.Uint64())
				}
				return res
			}(),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hi, lo := koalaOctupletToHalves(tc.elems)
			assignment := isActuallyKoalaHashCircuit{
				Hash:     [2]frontend.Variable{hi, lo},
				Expected: 1,
			}
			assert.NoError(t, test.IsSolved(&circuit, &assignment, ecc.BLS12_377.ScalarField()))
		})
	}
}

func TestIsActuallyKoalaHash_InvalidBLS(t *testing.T) {
	var circuit isActuallyKoalaHashCircuit
	mod := koalabear.Modulus().Uint64() // 0x7f000001

	cases := []struct {
		name  string
		elems [8]uint32
	}{
		{
			name:  "first limb equals modulus",
			elems: [8]uint32{uint32(mod), 0, 0, 0, 0, 0, 0, 0},
		},
		{
			name:  "last limb equals modulus",
			elems: [8]uint32{0, 0, 0, 0, 0, 0, 0, uint32(mod)},
		},
		{
			name:  "one limb exceeds modulus",
			elems: [8]uint32{0, 0, uint32(mod + 1), 0, 0, 0, 0, 0},
		},
		{
			name:  "all limbs at max uint32",
			elems: [8]uint32{0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF},
		},
		{
			name:  "middle limb just over modulus",
			elems: [8]uint32{1, 2, 3, uint32(mod), 5, 6, 7, 8},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hi, lo := koalaOctupletToHalves(tc.elems)
			assignment := isActuallyKoalaHashCircuit{
				Hash:     [2]frontend.Variable{hi, lo},
				Expected: 0,
			}
			assert.NoError(t, test.IsSolved(&circuit, &assignment, ecc.BLS12_377.ScalarField()))
		})
	}
}

func TestIsActuallyKoalaHash_RealBLSElement(t *testing.T) {
	// Generate a random BLS12-377 scalar field element packed into 32 bytes.
	// The top limb of a BLS element is ~29 bits, so it fits in 32 bits and
	// will be < modulus. But the lower limbs are essentially random 32-bit
	// values and overwhelmingly likely to exceed the koalabear modulus.
	var circuit isActuallyKoalaHashCircuit

	for i := 0; i < 10; i++ {
		var x fr377.Element
		_, err := x.SetRandom()
		require.NoError(t, err)

		b := x.Bytes() // 32 bytes big-endian

		// Check if any of the 8 limbs >= koalabear modulus
		mod := koalabear.Modulus().Uint64()
		hasInvalidLimb := false
		for j := 0; j < 8; j++ {
			limb := uint64(b[4*j])<<24 | uint64(b[4*j+1])<<16 | uint64(b[4*j+2])<<8 | uint64(b[4*j+3])
			if limb >= mod {
				hasInvalidLimb = true
				break
			}
		}

		expected := 1
		if hasInvalidLimb {
			expected = 0
		}

		hi := new(big.Int).SetBytes(b[:16])
		lo := new(big.Int).SetBytes(b[16:])

		assignment := isActuallyKoalaHashCircuit{
			Hash:     [2]frontend.Variable{hi, lo},
			Expected: expected,
		}
		assert.NoError(t, test.IsSolved(&circuit, &assignment, ecc.BLS12_377.ScalarField()),
			"failed for BLS element %x (expected koala=%v)", b, expected == 1)
	}
}

// stateRootSelectionCircuit tests the opt-out mechanism: when
// InitialStateRootHash is a valid koalabear octuplet use it, otherwise
// fall back to FirstExecutionInitialStateRootHash.
type stateRootSelectionCircuit struct {
	InitialStateRootHash               [2]frontend.Variable
	FirstExecutionInitialStateRootHash [2]frontend.Variable
	ExpectedSelected                   [2]frontend.Variable
}

func (c *stateRootSelectionCircuit) Define(api frontend.API) error {
	isKoala := isActuallyKoalaHash(api, c.InitialStateRootHash)
	selected := [2]frontend.Variable{
		api.Select(isKoala, c.InitialStateRootHash[0], c.FirstExecutionInitialStateRootHash[0]),
		api.Select(isKoala, c.InitialStateRootHash[1], c.FirstExecutionInitialStateRootHash[1]),
	}
	api.AssertIsEqual(selected[0], c.ExpectedSelected[0])
	api.AssertIsEqual(selected[1], c.ExpectedSelected[1])
	return nil
}

func TestStateRootSelection_KoalaBearUsesContractValue(t *testing.T) {
	// When InitialStateRootHash is a valid koalabear octuplet, the circuit
	// should select it (not the fallback).
	var circuit stateRootSelectionCircuit

	contractHi, contractLo := koalaOctupletToHalves([8]uint32{100, 200, 300, 400, 500, 600, 700, 800})
	executionHi, executionLo := koalaOctupletToHalves([8]uint32{1, 2, 3, 4, 5, 6, 7, 8})

	assignment := stateRootSelectionCircuit{
		InitialStateRootHash:               [2]frontend.Variable{contractHi, contractLo},
		FirstExecutionInitialStateRootHash: [2]frontend.Variable{executionHi, executionLo},
		ExpectedSelected:                   [2]frontend.Variable{contractHi, contractLo},
	}

	assert.NoError(t, test.IsSolved(&circuit, &assignment, ecc.BLS12_377.ScalarField()))
}

func TestStateRootSelection_BLSFallsBackToExecution(t *testing.T) {
	// When InitialStateRootHash is NOT a valid koalabear octuplet (i.e. it
	// came from BLS12-377), the circuit should fall back to
	// FirstExecutionInitialStateRootHash.
	var circuit stateRootSelectionCircuit

	// Use a value with limbs >= koalabear modulus to simulate a BLS hash
	blsHi, blsLo := koalaOctupletToHalves([8]uint32{0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF})
	executionHi, executionLo := koalaOctupletToHalves([8]uint32{10, 20, 30, 40, 50, 60, 70, 80})

	assignment := stateRootSelectionCircuit{
		InitialStateRootHash:               [2]frontend.Variable{blsHi, blsLo},
		FirstExecutionInitialStateRootHash: [2]frontend.Variable{executionHi, executionLo},
		ExpectedSelected:                   [2]frontend.Variable{executionHi, executionLo},
	}

	assert.NoError(t, test.IsSolved(&circuit, &assignment, ecc.BLS12_377.ScalarField()))
}

func TestStateRootSelection_RealBLSElementFallsBack(t *testing.T) {
	// Ensure a real random BLS12-377 element (which will almost certainly
	// have at least one limb >= koalabear modulus) triggers the fallback.
	var circuit stateRootSelectionCircuit

	var x fr377.Element
	_, err := x.SetRandom()
	require.NoError(t, err)
	b := x.Bytes()

	// Verify this element is actually invalid for koalabear (skip test if
	// we hit the ~0.5% false positive)
	mod := koalabear.Modulus().Uint64()
	hasInvalidLimb := false
	for j := 0; j < 8; j++ {
		limb := uint64(b[4*j])<<24 | uint64(b[4*j+1])<<16 | uint64(b[4*j+2])<<8 | uint64(b[4*j+3])
		if limb >= mod {
			hasInvalidLimb = true
			break
		}
	}
	if !hasInvalidLimb {
		t.Skip("randomly generated BLS element happened to look like valid koalabear (rare)")
	}

	blsHi := new(big.Int).SetBytes(b[:16])
	blsLo := new(big.Int).SetBytes(b[16:])

	executionHi, executionLo := koalaOctupletToHalves([8]uint32{99, 88, 77, 66, 55, 44, 33, 22})

	assignment := stateRootSelectionCircuit{
		InitialStateRootHash:               [2]frontend.Variable{blsHi, blsLo},
		FirstExecutionInitialStateRootHash: [2]frontend.Variable{executionHi, executionLo},
		ExpectedSelected:                   [2]frontend.Variable{executionHi, executionLo},
	}

	assert.NoError(t, test.IsSolved(&circuit, &assignment, ecc.BLS12_377.ScalarField()))
}

func TestStateRootSelection_WrongExpectationFails(t *testing.T) {
	// Negative test: assert the circuit rejects when we claim the wrong
	// selection result.
	var circuit stateRootSelectionCircuit

	// Valid koala hash — circuit should pick this, not the execution fallback
	contractHi, contractLo := koalaOctupletToHalves([8]uint32{1, 2, 3, 4, 5, 6, 7, 8})
	executionHi, executionLo := koalaOctupletToHalves([8]uint32{10, 20, 30, 40, 50, 60, 70, 80})

	assignment := stateRootSelectionCircuit{
		InitialStateRootHash:               [2]frontend.Variable{contractHi, contractLo},
		FirstExecutionInitialStateRootHash: [2]frontend.Variable{executionHi, executionLo},
		// Wrong: we claim the execution value was selected, but it should be the contract value
		ExpectedSelected: [2]frontend.Variable{executionHi, executionLo},
	}

	assert.Error(t, test.IsSolved(&circuit, &assignment, ecc.BLS12_377.ScalarField()))
}
