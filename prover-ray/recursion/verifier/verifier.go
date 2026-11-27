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
// verify proofs of several systems of the same shape, itself included. The
// key is only the Merkle root of the precomputed batch: the machine's
// programs share the chips' constraints and differ in their setup columns and
// heights, so a size-oblivious verifier, which takes the heights of the
// child's modules as data (pcs.WithSizeOblivious), verifies every program
// with the same chips and columns under its key: that is the plan, with the
// verifier shard proofs need anyway. A static verifier, of one shape (module
// heights, rounds, PCS geometry), would be cheaper, as a later optimisation:
// a program verifying proofs of itself must then be padded to the shape its
// verifier was built for, which is only known after compiling, so the
// constructor would take a shape, not a system, and the machine a target
// shape to pad to.
//
// Define verifies unconditionally. A program with a slot it does not use
// fills it with a proof that verifies (the node program copies its first
// child) and leaves the slot's values out of its own computation.
//
// PROTOTYPE (#4405): [New] wraps the in-machine wiop verifier of package
// recursion, for systems compiled size-oblivious; verifiertest.NewMock, which
// is not sound, remains for dev and test code. Like
// wiop.AllocateVerifierCircuit, [New] fails on a system registering a
// verifier action it cannot run in the machine (the node programs register
// recursion.E6Check for every extension public input), rather than drop the
// check. Not done: the transcript does not absorb the key (#4510).
package verifier

import (
	"errors"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/recursion"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// ErrNotImplemented was returned by [New] before the wiop verifier was written
// in the machine's instructions. Kept for callers that test for it.
var ErrNotImplemented = errors.New("verifier: the in-machine wiop verifier is not implemented")

// Constructor returns the verifier of the proofs of the systems of sys's
// shape: [New] in production, verifiertest.NewMock in tests. Code that builds
// programs takes a Constructor, so that only its caller chooses the mock.
type Constructor func(sys *wiop.System) (Verifier, error)

// Key is what a proof is verified under, as cells of the program: Root, the
// digest cell of the child system's setup commitment, and Heights, the scalar
// cells of the log sizes of the child system's sized modules, in module
// order. With nil Heights, the sizes are those of the system the verifier was
// built for. A size is part of what a key verifies: a program taking the root
// from its prover takes the heights with it, and binds both together (the
// node programs' key box).
type Key struct {
	Root    int
	Heights []int
}

// New returns the verifier of the proofs of the systems of sys's shape: the
// in-machine wiop verifier of package recursion (PROTOTYPE, #4405). sys must
// be compiled size-oblivious (pcs.WithSizeOblivious).
func New(sys *wiop.System) (Verifier, error) {
	if sys == nil {
		return nil, errors.New("verifier: no system")
	}
	return &real{sys: sys}, nil
}

// real is the in-machine wiop verifier behind the Verifier interface. Every
// Define builds a recursion.Verifier, which allocates its inputs as it goes:
// Allocate allocates nothing, and the proof's cells are the inputs Define
// allocates. Every verifier of one system has the same inputs, so Assign uses
// the first.
type real struct {
	sys *wiop.System
	v   *recursion.Verifier
}

func (r *real) Allocate(*recursion.Machine) ProofCells { return nil }

func (r *real) Define(m *recursion.Machine, key Key, _ ProofCells) ([]int, error) {
	v, err := recursion.NewVerifierWith(m, r.sys, recursion.VerifierOptions{
		Key: key.Root, HasKey: true, Heights: key.Heights,
	})
	if err != nil {
		return nil, err
	}
	if r.v == nil {
		r.v = v
	}
	pubs := make([]int, len(r.sys.PublicInputs))
	for i := range pubs {
		pubs[i] = v.PublicInput(i)
	}
	return pubs, nil
}

func (r *real) Assign(sys *wiop.System, proof wiop.Proof, pub wiop.PublicInput) ([]recursion.Cell, error) {
	if r.v == nil {
		return nil, errors.New("verifier: Assign before Define")
	}
	return r.v.InputsFor(sys, proof, pub)
}

// ProofCells are the addresses of the input cells that hold one child proof,
// in the order the verifier allocates them.
type ProofCells []int

// Verifier verifies, inside a program, the proofs of the systems of one
// shape.
type Verifier interface {
	// Allocate allocates on m the input cells of one whole child proof: its
	// public inputs, round commitments, opened values and Merkle paths. A
	// verifier may leave some or all of them to Define: the cells of a proof
	// are the inputs both allocate.
	Allocate(m *recursion.Machine) ProofCells

	// Define adds to m the child's wiop verifier, in machine instructions,
	// on proof, under key: it replays the transcript to derive the coins,
	// runs the verifier actions and checks the PCS openings. It returns the
	// proof's public inputs as cells, in the child system's order.
	Define(m *recursion.Machine, key Key, proof ProofCells) ([]int, error)

	// Assign, for the host, turns a proof of sys and its public inputs into
	// the values of the cells of Allocate and Define. It only lays the proof
	// out; the proof is verified by Define, inside the program, under sys's
	// key.
	Assign(sys *wiop.System, proof wiop.Proof, pub wiop.PublicInput) ([]recursion.Cell, error)
}

// AssertEqDigest asserts that the cells a and b hold the same digest (all 8
// lanes). Machine.AssertEqE6 compares 6 lanes, so the digests are compared as
// two overlapping E6 values, lanes 0..5 and 2..7.
func AssertEqDigest(m *recursion.Machine, a, b int) {
	for _, off := range []int{0, recursion.KindDigest - recursion.KindE6} {
		m.AssertEqE6(m.Lanes(a, off, recursion.KindE6), m.Lanes(b, off, recursion.KindE6))
	}
}
