// Package dummy is a very light proof system for tests: a fixed dummy circuit
// whose proofs are accepted by the standalone dummy verifier
// (zig/src/dummy.zig) if and only if they were made for exactly the claimed
// public inputs.
//
// The public inputs X are those of the root of the conglomeration tree: the
// guest-output hash, then the program verification key. The proof carries one
// witness value Y_i per public input, with, over KoalaBear and limb by limb,
//
//	Y_i = X_i^5
//
// x ↦ x^5 is a permutation of KoalaBear (gcd(5, p-1) = 1), so a proof made for
// X is rejected for any other public-input vector.
//
// The system is not compiled with the PCS: no columns, no commitments. A dummy
// proof can be produced by anyone who knows X, so it must only be used in test
// builds.
package dummy

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/proofserialization"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver/risc5"
)

const (
	// NumGuestOutputCells is the number of guest-output public inputs: one per
	// bound output byte.
	NumGuestOutputCells = risc5.NumGuestPublicOutputs
	// NumProgramVKCells is the number of program-VK public inputs. The
	// encoding of the program VK is not fixed yet; 8 (a Poseidon2 digest) is a
	// placeholder.
	NumProgramVKCells = 8
	// NumPublicInputs is the number of public inputs of the dummy circuit.
	// The Zig verifier (zig/src/dummy.zig) hard-codes the same number.
	NumPublicInputs = NumGuestOutputCells + NumProgramVKCells

	// GuestBase is the default base address images are encoded for: the start
	// of the guest's input region.
	GuestBase = proofserialization.GuestBase
	// SizePairHeader is the two-pointer header of a pair image.
	SizePairHeader = 16
)

// NewSystem builds the dummy circuit. Round 0 holds, for each public input, its
// cell X_i (registered as a public input) followed by its witness cell Y_i.
// Round 1 is empty and carries the relation check: a proof's round messages
// cover every round but the last, so the witness must not sit in the last one.
func NewSystem() *wiop.System {
	sys := wiop.NewSystemf("dummy")
	r0 := sys.NewRound()
	r1 := r0.EnsureNext()
	ctx := sys.Context.Childf("dummy")

	for i := range NumPublicInputs {
		tag, n := risc5.GuestPublicOutputsPI, i
		if i >= NumGuestOutputCells {
			tag, n = risc5.ProgramVKPI, i-NumGuestOutputCells
		}
		x := r0.NewCell(ctx.Childf("x-%d", i), false)
		sys.RegisterPublicInputs(tag, x, n)
		r0.NewCell(ctx.Childf("y-%d", i), false)
	}

	r1.RegisterVerifierAction(relationCheck{})
	return sys
}

// Prove returns the dummy proof for the public inputs pub. It panics if pub
// does not have [NumPublicInputs] entries.
func Prove(sys *wiop.System, pub wiop.PublicInput) (wiop.Proof, wiop.PublicInput) {
	if len(pub) != NumPublicInputs {
		panic(fmt.Sprintf("dummy: Prove: got %d public inputs, want %d", len(pub), NumPublicInputs))
	}
	y := witnessCells(sys)
	return sys.Prove(func(rt *wiop.Runtime) {
		for i, x := range sys.PublicInputs {
			rt.AssignCell(x, pub[i])
			rt.AssignCell(y[i], Witness(pub[i]))
		}
	})
}

// Verify is the Go reference verifier of a dummy proof.
func Verify(sys *wiop.System, proof wiop.Proof, pub wiop.PublicInput) error {
	return sys.Verify(proof, pub)
}

// Witness returns the witness Y for a public input X: X^5, limb by limb.
func Witness(x field.Gen) field.Gen {
	if x.IsBase() {
		return field.ElemFromBase(pow5(x.AsBase()))
	}
	e := x.AsExt()
	for _, limb := range []*field.Element{&e.B0.A0, &e.B0.A1, &e.B1.A0, &e.B1.A1, &e.B2.A0, &e.B2.A1} {
		*limb = pow5(*limb)
	}
	return field.ElemFromExt(e)
}

// Encode writes the image of a dummy proof and its public inputs, encoded for
// base, with proofserialization like any ray proof: the witness lands in round
// 0's cells, the PCS opening is empty.
func Encode(sys *wiop.System, proof wiop.Proof, pub wiop.PublicInput, base uint64) ([]byte, error) {
	input, err := proofserialization.Project(sys, proof, pub)
	if err != nil {
		return nil, err
	}
	return proofserialization.Encode(input, base)
}

// EncodePair writes the pair image of two dummy proofs, encoded for base: the
// two-pointer header, then each proof's image encoded at its own absolute
// start, as verifier-ray's pair images are laid out.
func EncodePair(
	sys *wiop.System,
	proofA wiop.Proof, pubA wiop.PublicInput,
	proofB wiop.Proof, pubB wiop.PublicInput,
	base uint64,
) ([]byte, error) {
	offA := uint64(SizePairHeader)
	imgA, err := Encode(sys, proofA, pubA, base+offA)
	if err != nil {
		return nil, err
	}
	offB := (offA + uint64(len(imgA)) + 7) &^ 7
	imgB, err := Encode(sys, proofB, pubB, base+offB)
	if err != nil {
		return nil, err
	}

	img := make([]byte, offB+uint64(len(imgB)))
	binary.LittleEndian.PutUint64(img[0:], base+offA)
	binary.LittleEndian.PutUint64(img[8:], base+offB)
	copy(img[offA:], imgA)
	copy(img[offB:], imgB)
	return img, nil
}

// witnessCells returns the witness cells Y, aligned with sys.PublicInputs: the
// round-0 cells that are not public inputs, in declaration order.
func witnessCells(sys *wiop.System) []*wiop.Cell {
	y := make([]*wiop.Cell, 0, len(sys.PublicInputs))
	for _, cell := range sys.Rounds[0].Cells {
		if _, pos := sys.LookupPublicInput(cell.Context.ID); pos < 0 {
			y = append(y, cell)
		}
	}
	return y
}

// relationCheck is the verifier action of the dummy circuit: every witness cell
// must be the image of its public input.
type relationCheck struct{}

var errRelationViolated = errors.New("dummy: relation violated")

// Check implements [wiop.VerifierAction].
func (relationCheck) Check(rt *wiop.Runtime) error {
	y := witnessCells(rt.System)
	for i, x := range rt.System.PublicInputs {
		want := Witness(rt.GetCellValue(x))
		got := rt.GetCellValue(y[i])
		if !want.Ext.Equal(&got.Ext) {
			return fmt.Errorf("%w: public input %d", errRelationViolated, i)
		}
	}
	return nil
}

func pow5(x field.Element) field.Element {
	var r field.Element
	r.Square(&x)
	r.Square(&r)
	r.Mul(&r, &x)
	return r
}
