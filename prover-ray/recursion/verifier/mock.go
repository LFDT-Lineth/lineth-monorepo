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

// Mock is the dev and test [Verifier]. It runs the child's wiop verifier in
// Go, on the host, instead of in the machine's instructions:
//
//   - Allocate allocates the public inputs, the round commitments and a
//     verdict, instead of the whole proof.
//   - Assign verifies the proof in Go with its system, then sets the verdict
//     to the digest of the system's key, the public inputs and the
//     commitments.
//   - Define only recomputes that digest and checks it equals the verdict.
//
// Programs built on a Mock have the inputs and the flow of programs built on
// the real verifier, but the Mock is not sound: a host can skip the Go
// verification and compute the verdict anyway. Never on a production path.
type Mock struct {
	shape shape
}

// NewMock returns the mock verifier of the proofs of the systems of sys's
// shape.
func NewMock(sys *wiop.System) (Verifier, error) {
	if sys == nil {
		return nil, errors.New("verifier: no system")
	}
	return &Mock{shape: shapeOf(sys)}, nil
}

// Allocate implements [Verifier]: the public inputs, the commitments, the
// verdict.
func (v *Mock) Allocate(m *recursion.Machine) ProofCells {
	cells := make(ProofCells, 0, len(v.shape.pubs)+v.shape.commitments+1)
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

// Define implements [Verifier]: the verdict is the digest of the key, the
// public inputs and the commitments, laid out as Pack does, which
// [Mock.Assign] computes with poseidon2.MDHasher.
func (v *Mock) Define(m *recursion.Machine, active, key int, proof ProofCells) []int {
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
	return slices.Clone([]int(pubs))
}

// Assign implements [Verifier]: it checks that sys has the mock's shape and
// the proof its shape, verifies the proof in Go with sys and lays it out with
// its verdict.
func (v *Mock) Assign(sys *wiop.System, proof wiop.Proof, pub wiop.PublicInput) ([]recursion.Cell, error) {
	if sys == nil {
		return nil, errors.New("no system")
	}
	if !shapeOf(sys).equal(v.shape) {
		return nil, errors.New("the system has not the verifier's shape")
	}
	if len(pub) != len(v.shape.pubs) {
		return nil, fmt.Errorf("%d public inputs, want %d", len(pub), len(v.shape.pubs))
	}
	if len(proof.Commitments) != v.shape.commitments {
		return nil, fmt.Errorf("%d round commitments, want %d", len(proof.Commitments), v.shape.commitments)
	}
	cells := make([]recursion.Cell, 0, len(pub)+v.shape.commitments+1)
	elements := append([]field.Element(nil), sys.PrecomputedCommitment[:]...)
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
