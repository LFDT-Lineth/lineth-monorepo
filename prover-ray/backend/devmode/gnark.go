package devmode

import (
	"fmt"
	"math/big"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver/risc5"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
)

// GnarkCircuit is the gnark (BN254 PLONK) statement of dev mode. It exposes the
// same two public inputs as the wiop wrapper circuit: the packed guest output
// and the packed program VK, in that order. It does not verify a wiop proof: it
// only binds the 40 devmode public-input cells to their packed values, so its
// proof is as unsound as a devmode wiop proof (test/dev builds only).
type GnarkCircuit struct {
	// GuestOutput is the 32 guest-output bytes read as one big-endian number,
	// reduced mod r.
	GuestOutput frontend.Variable `gnark:",public"`
	// ProgramVK is the 8 program-VK limbs written big-endian, 4 bytes each,
	// read as one big-endian number, reduced mod r.
	ProgramVK frontend.Variable `gnark:",public"`

	// GuestOutputCells are the guest-output cells, one byte each.
	GuestOutputCells [NumGuestOutputCells]frontend.Variable
	// ProgramVKCells are the program-VK limbs, one per cell.
	ProgramVKCells [NumProgramVKCells]frontend.Variable
}

// Define implements [frontend.Circuit].
func (c *GnarkCircuit) Define(api frontend.API) error {
	api.AssertIsEqual(c.GuestOutput, PackGuestOutputCircuit(api, c.GuestOutputCells[:]))
	api.AssertIsEqual(c.ProgramVK, PackProgramVKCircuit(api, c.ProgramVKCells[:]))
	return nil
}

// NewGnarkAssignment returns the full assignment of [GnarkCircuit] for the
// devmode public inputs pub of a proof over sys. The cells are located by
// their wiop tags.
func NewGnarkAssignment(sys *wiop.System, pub wiop.PublicInput) *GnarkCircuit {
	out := risc5.GetGuestPublicOutputs(sys, pub)
	vk := risc5.GetProgramVK(sys, pub)

	a := &GnarkCircuit{
		GuestOutput: PackGuestOutput(out),
		ProgramVK:   PackProgramVK(vk[:]),
	}
	for i := range a.GuestOutputCells {
		a.GuestOutputCells[i] = out[i].Uint64()
	}
	for i := range a.ProgramVKCells {
		a.ProgramVKCells[i] = vk[i].Uint64()
	}
	return a
}

// PackGuestOutput reads the guest-output cells, one byte each, as one
// big-endian number (cell 0 is the most significant byte) and reduces it mod
// the BN254 scalar field order r, as the contract does with
// mod(keccak256(...), MODULO_R).
func PackGuestOutput(cells []field.Element) *big.Int {
	if len(cells) != NumGuestOutputCells {
		panic(fmt.Sprintf("devmode: PackGuestOutput: got %d cells, want %d", len(cells), NumGuestOutputCells))
	}
	return packNative(cells, 8)
}

// PackProgramVK writes the program-VK limbs big-endian, 4 bytes each (limb i
// in bytes 4i to 4i+4), reads the 32 bytes as one big-endian number and
// reduces it mod the BN254 scalar field order r.
func PackProgramVK(limbs []field.Element) *big.Int {
	if len(limbs) != NumProgramVKCells {
		panic(fmt.Sprintf("devmode: PackProgramVK: got %d limbs, want %d", len(limbs), NumProgramVKCells))
	}
	return packNative(limbs, 32)
}

// PackGuestOutputCircuit is the in-circuit counterpart of [PackGuestOutput].
// The cells are not range-checked.
func PackGuestOutputCircuit(api frontend.API, cells []frontend.Variable) frontend.Variable {
	return packCircuit(api, cells, 8)
}

// PackProgramVKCircuit is the in-circuit counterpart of [PackProgramVK].
// The limbs are not range-checked.
func PackProgramVKCircuit(api frontend.API, limbs []frontend.Variable) frontend.Variable {
	return packCircuit(api, limbs, 32)
}

// packNative returns sum_i cells[i] * 2^(bits*(n-1-i)) mod r.
func packNative(cells []field.Element, bits uint) *big.Int {
	acc := new(big.Int)
	for _, c := range cells {
		acc.Lsh(acc, bits)
		acc.Add(acc, new(big.Int).SetUint64(c.Uint64()))
	}
	return acc.Mod(acc, ecc.BN254.ScalarField())
}

// packCircuit is the in-circuit Horner evaluation of [packNative]; the
// reduction mod r is the native field arithmetic of the circuit.
func packCircuit(api frontend.API, cells []frontend.Variable, bits uint) frontend.Variable {
	shift := new(big.Int).Lsh(big.NewInt(1), bits)
	var acc frontend.Variable = 0
	for _, c := range cells {
		acc = api.Add(api.Mul(acc, shift), c)
	}
	return acc
}
