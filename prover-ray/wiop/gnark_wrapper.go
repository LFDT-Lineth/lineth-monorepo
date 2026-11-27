package wiop

import (
	"fmt"
	"math/big"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/math/emulated"
	"github.com/consensys/gnark/std/rangecheck"
)

const (
	// GuestOutputBitsPerCell is the width of a guest-output cell: one byte.
	GuestOutputBitsPerCell = 8
	// ProgramVKBitsPerCell is the width of a program-VK cell: one 4-byte limb.
	ProgramVKBitsPerCell = 32
	// packedBits is the width of a packed group: 32 bytes.
	packedBits = 256
)

// WrapperCircuit is the BN254 gnark circuit wrapping a wiop proof: it verifies
// the wiop proof in-circuit and exposes the wiop public inputs, packed into two
// BN254 elements, as its only public inputs: the guest output first, then the
// program VK, in the order of the wiop [PublicInput] vector.
//
// Each group is 32 bytes read as one big-endian number and reduced mod r, the
// way the contract turns a keccak hash into a public input. The guest output
// has one byte per cell; the program VK has one 4-byte limb per cell, limb i in
// bytes 4i to 4i+4. See [PackBigEndian].
//
// The wiop verifier only sees each cell mod p, so a prover could assign x+p
// for x and change the packed value. Each group rules this out with the
// cheapest check that does it: guest-output cells are range-checked to 8 bits
// (x+p >= p > 2^8 fails it, and a cell wider than a byte, which would overlap
// its neighbour in the packing, is rejected too); program-VK limbs can be any
// value below p, so they are strictly reduced mod p instead.
//
// The packed values are only defined mod r: 32 bytes can exceed r, so two
// groups whose 256-bit values differ by r pack to the same public input. This
// matches the contract, which reduces the same way; exploiting it would need a
// keccak or Poseidon2 output equal to a chosen value mod r.
type WrapperCircuit struct {
	// GuestOutput is the packed guest output.
	GuestOutput frontend.Variable `gnark:",public"`
	// ProgramVK is the packed program VK.
	ProgramVK frontend.Variable `gnark:",public"`

	// Verifier is the wrapped wiop verifier circuit. // private
	Verifier VerifierCircuit

	// Compile-time metadata: the positions in the wiop [PublicInput] vector of
	// each group's cells, in packing order.
	guestOutputPos []int `gnark:"-"`
	programVKPos   []int `gnark:"-"`
}

// AllocateWrapperCircuit returns the unassigned wrapper circuit of sys. The two
// groups are the public inputs registered under guestOutputTag and
// programVKTag with numeric suffixes 0, 1, … (as [System.RegisterPublicInputs]
// registers a multi-cell role). The tags are passed in because they belong to
// the caller (zkcdriver/risc5), which wiop cannot import.
//
// It panics if a group is missing, does not span exactly 32 bytes, or carries
// an extension-field cell, and on any [AllocateVerifierCircuit] restriction.
func AllocateWrapperCircuit(sys *System, guestOutputTag, programVKTag PublicInputTag) *WrapperCircuit {
	return &WrapperCircuit{
		Verifier:       *AllocateVerifierCircuit(sys),
		guestOutputPos: groupPositions(sys, guestOutputTag, GuestOutputBitsPerCell),
		programVKPos:   groupPositions(sys, programVKTag, ProgramVKBitsPerCell),
	}
}

// AssignWrapperCircuit returns the witness assignment of the circuit for
// (proof, pub). It panics like [VerifierCircuit.AssignVerifierCircuit].
func (c *WrapperCircuit) AssignWrapperCircuit(proof Proof, pub PublicInput) *WrapperCircuit {
	return &WrapperCircuit{
		GuestOutput:    PackBigEndian(gather(pub, c.guestOutputPos), GuestOutputBitsPerCell),
		ProgramVK:      PackBigEndian(gather(pub, c.programVKPos), ProgramVKBitsPerCell),
		Verifier:       *c.Verifier.AssignVerifierCircuit(proof, pub),
		guestOutputPos: c.guestOutputPos,
		programVKPos:   c.programVKPos,
	}
}

// Define implements [frontend.Circuit]: it verifies the wiop proof, then binds
// the two public inputs to the packing of the wiop public inputs the verifier
// used.
func (c *WrapperCircuit) Define(api frontend.API) error {
	if err := c.Verifier.Define(api); err != nil {
		return err
	}
	kb, err := emulated.NewField[emulated.KoalaBear](api)
	if err != nil {
		return err
	}
	// Bind each public input to its group: take the cells the verifier used,
	// make sure each is the canonical value (rejecting x+p), then pack them
	// big-endian mod r.
	// Guest output: 32 cells, one byte each. An 8-bit range check both rejects
	// x+p and enforces the byte width, more cheaply than a strict reduction.
	guestOutputCells := c.byteCells(api, c.guestOutputPos)
	api.AssertIsEqual(c.GuestOutput, PackBigEndianCircuit(api, guestOutputCells, GuestOutputBitsPerCell))
	// Program VK: 8 cells, one 4-byte limb each. A limb can be any value below
	// p, so a range check cannot replace the strict reduction.
	programVKCells := c.canonicalCells(kb, c.programVKPos)
	api.AssertIsEqual(c.ProgramVK, PackBigEndianCircuit(api, programVKCells, ProgramVKBitsPerCell))
	return nil
}

// byteCells returns the native values of the public-input cells at the given
// positions, each range-checked to 8 bits. A byte below 2^8 is canonical, so
// this also rejects x+p, which is at least p.
func (c *WrapperCircuit) byteCells(api frontend.API, positions []int) []frontend.Variable {
	rc := rangecheck.New(api)
	out := make([]frontend.Variable, len(positions))
	for i, pos := range positions {
		// KoalaBear fits one 31-bit limb, so Limbs[0] is the whole value.
		out[i] = c.publicCell(pos).Limbs[0]
		rc.Check(out[i], GuestOutputBitsPerCell)
	}
	return out
}

// canonicalCells returns the native values of the public-input cells at the
// given positions. Each cell is strictly reduced first: an emulated element is
// only defined mod p, so without it a prover could pack x+p for x, which the
// wiop verifier accepts but which changes the packed value.
func (c *WrapperCircuit) canonicalCells(kb *emulated.Field[emulated.KoalaBear], positions []int) []frontend.Variable {
	out := make([]frontend.Variable, len(positions))
	for i, pos := range positions {
		// Reduce to [0, p) and assert it, so x+p cannot stand in for x.
		e := kb.ReduceStrict(c.publicCell(pos))
		// KoalaBear fits one 31-bit limb, so Limbs[0] is the canonical value.
		out[i] = e.Limbs[0]
	}
	return out
}

// publicCell returns the emulated element the verifier holds for the wiop
// public input at position pos, mapped to its witness slot.
func (c *WrapperCircuit) publicCell(pos int) *emulated.Element[emulated.KoalaBear] {
	slot := c.Verifier.layout.cells[c.Verifier.spec.PublicInputs[pos].Context.ID]
	return &c.Verifier.PublicInputsBase[slot.index].EV
}

// groupPositions returns the positions of the cells registered under tag_0,
// tag_1, …, checking they are base-field cells spanning exactly 32 bytes.
func groupPositions(sys *System, tag PublicInputTag, bits uint) []int {
	var positions []int
	for k := 0; ; k++ {
		cell, pos := sys.LookupPublicInputByTag(tag, k)
		if pos < 0 {
			break
		}
		if cell.IsExtension() {
			panic(fmt.Sprintf("wiop: WrapperCircuit: public input %s_%d must be a base-field cell", tag, k))
		}
		positions = append(positions, pos)
	}
	if uint(len(positions))*bits != packedBits {
		panic(fmt.Sprintf("wiop: WrapperCircuit: %d public inputs tagged %s of %d bits each, want %d bits",
			len(positions), tag, bits, packedBits))
	}
	return positions
}

// gather returns the base-field values of pub at the given positions.
func gather(pub PublicInput, positions []int) []field.Element {
	out := make([]field.Element, len(positions))
	for i, pos := range positions {
		out[i] = pub[pos].AsBase()
	}
	return out
}

// PackBigEndian reads cells as one big-endian number, bits per cell, cell 0
// the most significant, and reduces it mod the BN254 scalar field order r:
// sum_i cells[i] * 2^(bits*(n-1-i)) mod r. Cells are taken in canonical form
// and are not range-checked against bits.
func PackBigEndian(cells []field.Element, bits uint) *big.Int {
	acc := new(big.Int)
	for _, c := range cells {
		acc.Lsh(acc, bits)
		acc.Add(acc, new(big.Int).SetUint64(c.Uint64()))
	}
	return acc.Mod(acc, ecc.BN254.ScalarField())
}

// PackBigEndianCircuit is the in-circuit counterpart of [PackBigEndian], over
// BN254: the reduction mod r is the circuit's native arithmetic. The cells are
// not range-checked; the caller must pass canonical values.
func PackBigEndianCircuit(api frontend.API, cells []frontend.Variable, bits uint) frontend.Variable {
	shift := new(big.Int).Lsh(big.NewInt(1), bits)
	var acc frontend.Variable = 0
	for _, c := range cells {
		acc = api.Add(api.Mul(acc, shift), c)
	}
	return acc
}
