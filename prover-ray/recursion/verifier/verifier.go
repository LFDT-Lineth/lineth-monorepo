// Package verifier verifies a child proof inside a program of the recursion
// machine (package recursion): the child system's wiop verifier, rewritten in
// the machine's instructions, so that the outer proof of the program proves
// the child proof verified. It plays the role wiop.VerifierCircuit plays in
// gnark.
//
// A Verifier is used in two phases:
//
//   - Building the program (once): Allocate allocates the input cells that
//     hold a child proof, and Define adds the instructions that verify it and
//     returns its public inputs as cells the program builds on.
//   - Proving (per child proof): the host calls Assign to turn the child
//     proof into the values of those input cells; Program.Execute computes
//     the full witness from them, and Program.Prove proves it.
//
// A proof is verified under a key, its system's setup commitment
// (wiop.System.PrecomputedCommitment), held in a digest cell of the program:
// a constant, or a value the program computes, so that one program can
// verify proofs of several systems of the same shape, itself included.
//
// The wiop verifier is not written in the machine's instructions yet. [New],
// the production constructor, returns [ErrNotImplemented]; dev and test code
// uses [NewMock], which is not sound.
package verifier

import (
	"errors"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/recursion"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// ErrNotImplemented is returned by [New] until the wiop verifier is written
// in the machine's instructions.
var ErrNotImplemented = errors.New("verifier: the in-machine wiop verifier is not implemented")

// Constructor returns the verifier of the proofs of the systems of sys's
// shape: [New] in production, [NewMock] in tests. Code that builds programs
// takes a Constructor, so that only its caller chooses the mock.
type Constructor func(sys *wiop.System) (Verifier, error)

// New returns the verifier of the proofs of the systems of sys's shape. It is
// the production constructor and returns [ErrNotImplemented] for now; use
// [NewMock] in dev and test code.
func New(sys *wiop.System) (Verifier, error) {
	return nil, ErrNotImplemented
}

// ProofCells are the addresses of the input cells that hold one child proof,
// in the order the verifier allocates them.
type ProofCells []int

// Verifier verifies, inside a program, the proofs of the systems of one
// shape.
type Verifier interface {
	// Allocate allocates on m the input cells of one whole child proof: its
	// public inputs, round commitments, opened values and Merkle paths.
	Allocate(m *recursion.Machine) ProofCells

	// Define adds to m the child's wiop verifier, in machine instructions,
	// on proof, under the key in the digest cell key: it replays the
	// transcript to derive the coins, runs the verifier actions and checks
	// the PCS openings. It checks this when the scalar cell active is 1; when
	// it is 0 it checks nothing and the cells may be zero. It returns the
	// proof's public inputs as cells, in the child system's order.
	Define(m *recursion.Machine, active, key int, proof ProofCells) []int

	// Assign, for the host, turns a proof of sys and its public inputs into
	// the values of the cells of Allocate. It only lays the proof out; the
	// proof is verified by Define, inside the program, under sys's key.
	Assign(sys *wiop.System, proof wiop.Proof, pub wiop.PublicInput) ([]recursion.Cell, error)
}

// AssertEqDigest asserts that the cells a and b hold the same digest (all 8
// lanes) when the scalar cell active is 1, and asserts nothing when it is 0.
// Machine.AssertEqE6 compares 6 lanes, so the digests are compared as two
// overlapping E6 values, lanes 0..5 and 2..7.
func AssertEqDigest(m *recursion.Machine, active, a, b int) {
	for _, off := range []int{0, recursion.KindDigest - recursion.KindE6} {
		x := m.Lanes(a, off, recursion.KindE6)
		y := m.Lanes(b, off, recursion.KindE6)
		m.AssertEqE6(m.Select(active, x, y), y)
	}
}
