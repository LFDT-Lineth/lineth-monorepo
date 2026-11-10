package verifier

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/poseidon2"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/recursion"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// shape is what a [Mock] knows of a child system: the kind of each public
// input, recursion.KindScalar or recursion.KindE6 in registration order, and
// the number of round commitments of its proofs.
type shape struct {
	pubs        []int
	commitments int
}

// shapeOf returns the shape of the proofs of sys.
func shapeOf(sys *wiop.System) shape {
	s := shape{pubs: make([]int, len(sys.PublicInputs))}
	for pos, cell := range sys.PublicInputs {
		s.pubs[pos] = recursion.KindScalar
		if cell.IsExtension() {
			s.pubs[pos] = recursion.KindE6
		}
	}
	for _, r := range sys.Rounds {
		if r.HasCommitment {
			s.commitments++
		}
	}
	return s
}

func (s shape) equal(o shape) bool {
	return slices.Equal(s.pubs, o.pubs) && s.commitments == o.commitments
}

// Mock is the dev and test [Verifier] of the proofs of one or more systems of
// one shape, each known by its key. It verifies nothing in the machine: a
// mock proof is the proof's round commitments and a verdict, the digest of
// the key, the public inputs and the commitments, which [Mock.Cells] computes
// once the proof is verified in Go by the system whose key it is claimed
// under, and the machine recomputes and checks. That binds the public inputs
// to the key and to the commitments of a proof that verified, so that a
// program built on a Mock has the inputs and the flow of one built on the
// real verifier, whose public inputs are likewise inputs it absorbs and
// constrains, but it is not sound: a host can claim any verdict. Never on a
// production path.
type Mock struct {
	shape   shape
	systems map[field.Octuplet]*wiop.System
}

// NewMock returns the mock verifier of the proofs of the systems, which share
// a shape, each verified in Go under its key (wiop.System.PrecomputedCommitment).
// A system that exists only once a program is compiled with the mock, the
// program verifying proofs of itself, is added with [Mock.Accept].
func NewMock(systems ...*wiop.System) (*Mock, error) {
	if len(systems) == 0 {
		return nil, errors.New("verifier: no system")
	}
	v := &Mock{shape: shapeOf(systems[0]), systems: make(map[field.Octuplet]*wiop.System)}
	for _, sys := range systems {
		if err := v.Accept(sys); err != nil {
			return nil, err
		}
	}
	return v, nil
}

// Accept adds sys to the systems the mock verifies the proofs of, under its
// key. sys must have the mock's shape and a key no other system has.
func (v *Mock) Accept(sys *wiop.System) error {
	if !shapeOf(sys).equal(v.shape) {
		return errors.New("verifier: the system has not the mock's shape")
	}
	if _, dup := v.systems[sys.PrecomputedCommitment]; dup {
		return fmt.Errorf("verifier: two systems have the key %v", sys.PrecomputedCommitment)
	}
	v.systems[sys.PrecomputedCommitment] = sys
	return nil
}

// Declare implements [Verifier]: the public inputs, the commitments, the
// verdict.
func (v *Mock) Declare(m *recursion.Machine) []int {
	cells := make([]int, 0, len(v.shape.pubs)+v.shape.commitments+1)
	for _, kind := range v.shape.pubs {
		if kind == recursion.KindE6 {
			cells = append(cells, m.InputE6())
		} else {
			cells = append(cells, m.InputScalar())
		}
	}
	for range v.shape.commitments + 1 {
		cells = append(cells, m.InputDigest())
	}
	return cells
}

// Verify implements [Verifier]: the verdict is the digest of the key, the
// public inputs and the commitments, laid out as Pack does, which [Mock.Cells]
// computes with poseidon2.MDHasher.
func (v *Mock) Verify(m *recursion.Machine, active, key int, proof []int) []int {
	n := len(v.shape.pubs)
	if len(proof) != n+v.shape.commitments+1 {
		panic(fmt.Sprintf("verifier: %d proof cells, want %d", len(proof), n+v.shape.commitments+1))
	}
	pubs, commitments, verdict := proof[:n], proof[n:n+v.shape.commitments], proof[n+v.shape.commitments]
	items := append([]int{key}, pubs...)
	kinds := append([]int{recursion.KindDigest}, v.shape.pubs...)
	for _, c := range commitments {
		items = append(items, c)
		kinds = append(kinds, recursion.KindDigest)
	}
	AssertEqDigest(m, active, m.MDHash(m.Pack(items, kinds)), verdict)
	return slices.Clone(pubs)
}

// Cells implements [Verifier]: it checks the proof's shape, verifies it in Go
// with the system of the key and lays it out with its verdict.
func (v *Mock) Cells(key field.Octuplet, proof wiop.Proof, pub wiop.PublicInput) ([]recursion.Cell, error) {
	if len(pub) != len(v.shape.pubs) {
		return nil, fmt.Errorf("%d public inputs, want %d", len(pub), len(v.shape.pubs))
	}
	if len(proof.Commitments) != v.shape.commitments {
		return nil, fmt.Errorf("%d round commitments, want %d", len(proof.Commitments), v.shape.commitments)
	}
	cells := make([]recursion.Cell, 0, len(pub)+v.shape.commitments+1)
	elements := append([]field.Element(nil), key[:]...)
	for pos, kind := range v.shape.pubs {
		var c recursion.Cell
		switch {
		case kind == recursion.KindE6:
			c = recursion.E6Cell(pub[pos].AsExt())
		case !pub[pos].IsBase():
			return nil, fmt.Errorf("public input %d is not a base field element", pos)
		default:
			c = recursion.ScalarCell(pub[pos].AsBase())
		}
		cells = append(cells, c)
		elements = append(elements, c[:kind]...)
	}
	sys, ok := v.systems[key]
	if !ok {
		return nil, fmt.Errorf("the proof is claimed under the key %v of no system", key)
	}
	if err := sys.Verify(proof, pub); err != nil {
		return nil, fmt.Errorf("verifying the proof in Go: %w", err)
	}
	for _, round := range slices.Sorted(maps.Keys(proof.Commitments)) {
		c := recursion.Cell(proof.Commitments[round])
		cells = append(cells, c)
		elements = append(elements, c[:]...)
	}
	h := poseidon2.NewMDHasher()
	h.WriteElements(elements...)
	return append(cells, recursion.Cell(h.SumDigest())), nil
}
