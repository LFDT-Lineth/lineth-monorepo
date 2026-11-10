// Package verifier holds the in-machine verifiers: the layer of a program of
// the recursion machine (package recursion) that verifies a child proof and
// yields its public inputs as values the program can build on. The real one,
// the wiop verifier in the machine's language, will replay the child's
// transcript, run its verifier actions and check its PCS opening; until it is
// written, [Mock] stands in behind the same interface, so that the programs
// around a verifier, their hosts and their tests take their final shape now
// and swap the verifier later.
//
// A verifier is built for one child system and verifies its proofs under a
// key: the system's setup commitment (wiop.System.PrecomputedCommitment), the
// program key of a program of the machine. The key is a digest cell of the
// program, a constant or a value it computes, so that one program verifies
// proofs of several systems of the same shape under a key it selects.
package verifier

import (
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/recursion"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// New returns the verifier of the proofs of the systems, which share a
// shape, each known by its key: the [Mock], until the wiop verifier is
// written in the machine's language. It is the one place to swap them.
func New(systems ...*wiop.System) (Verifier, error) {
	return NewMock(systems...)
}

// Verifier verifies the proofs of child systems of one shape in the machine.
// A program declares the proof's input cells with Declare, in the place of
// its inputs it chooses, verifies them with Verify, and its host lays a proof
// out with Cells.
type Verifier interface {
	// Accept adds a system of the verifier's shape whose proofs it verifies
	// under the system's key: one that exists only once a program is
	// compiled with the verifier, the program verifying proofs of itself.
	Accept(sys *wiop.System) error
	// Declare allocates the input cells of one proof on m, in a fixed order,
	// and returns them: the handle Verify takes.
	Declare(m *recursion.Machine) []int
	// Verify adds to m the verification of the proof held by the cells of
	// Declare under the key in the digest cell key, when the scalar cell
	// active is 1; when it is 0 it checks nothing, and the cells may be zero.
	// It returns the proof's public inputs as cells, in the child system's
	// order: values the proof verifies.
	Verify(m *recursion.Machine, active, key int, proof []int) []int
	// Cells lays a proof and its public inputs out as the cells of Declare,
	// for the host, under the key the proof is claimed under.
	Cells(key field.Octuplet, proof wiop.Proof, pub wiop.PublicInput) ([]recursion.Cell, error)
}

// AssertEqDigest asserts that the cells a and b hold the same digest, all
// [recursion.KindDigest] lanes, when the scalar cell active is 1, and nothing
// when it is 0. Machine.AssertEqE6 compares six lanes, so the digests are
// compared as two overlapping E6 values, lanes 0..5 and 2..7.
func AssertEqDigest(m *recursion.Machine, active, a, b int) {
	for _, off := range []int{0, recursion.KindDigest - recursion.KindE6} {
		x := m.Lanes(a, off, recursion.KindE6)
		y := m.Lanes(b, off, recursion.KindE6)
		m.AssertEqE6(m.Select(active, x, y), y)
	}
}
