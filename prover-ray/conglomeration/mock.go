package conglomeration

import (
	"fmt"
	"slices"

	multisethashing "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/multiset_hashing"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/messagebus"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver/risc5"
)

// MockMerger is the dev and test [Merger] over the public inputs of a
// RISCV-ZKC system. It produces no proof: a merged node's Proof is the
// zero wiop.Proof, so a mock node can never pass for a verifiable proof, and
// its Pub is the first child's with the message-bus products multiplied and the
// shared randomness contributions added. A shard proof that reaches Finalize
// unmerged keeps its proof.
type MockMerger struct {
	sys *wiop.System
}

// NewMockMerger returns the mock merger of the shard proofs over sys,
// a compiled RISCV-ZKC system (see risc5.CheckPublicInputs).
// [Node.Proof] is empty.
func NewMockMerger(sys *wiop.System) MockMerger {
	return MockMerger{sys: sys}
}

// Merge implements [Merger].
func (m MockMerger) Merge(children []Node) (Node, error) {
	if len(children) < 2 || len(children) > RecursionTreeArity {
		return Node{}, fmt.Errorf("merging %d nodes, want 2 to %d", len(children), RecursionTreeArity)
	}
	first := children[0]
	coverage := first.Coverage
	for _, child := range children {
		if len(child.Pub) != len(m.sys.PublicInputs) {
			return Node{}, fmt.Errorf("node %v has %d public inputs, want %d",
				child.Coverage, len(child.Pub), len(m.sys.PublicInputs))
		}
	}
	seeded := messagebus.HasSharedRandomness(m.sys)
	pub := slices.Clone(first.Pub)
	products := messagebus.GetProducts(m.sys, first.Pub)
	var contribution multisethashing.MSetHash
	if seeded {
		contribution = messagebus.GetSharedRandomnessContribution(m.sys, first.Pub)
	}
	for _, child := range children[1:] {
		var err error
		if coverage, err = mergeCoverage(coverage, child.Coverage); err != nil {
			return Node{}, err
		}
		if risc5.GetProgramVK(m.sys, first.Pub) != risc5.GetProgramVK(m.sys, child.Pub) {
			return Node{}, fmt.Errorf("nodes %v and %v have different program VK", first.Coverage, child.Coverage)
		}
		if !slices.Equal(risc5.GetGuestPublicOutputs(m.sys, first.Pub), risc5.GetGuestPublicOutputs(m.sys, child.Pub)) {
			return Node{}, fmt.Errorf("nodes %v and %v have different guest output", first.Coverage, child.Coverage)
		}
		for h, p := range messagebus.GetProducts(m.sys, child.Pub) {
			products[h] = products[h].Mul(p)
		}
		if seeded {
			if messagebus.GetSharedRandomnessSeed(m.sys, first.Pub) != messagebus.GetSharedRandomnessSeed(m.sys, child.Pub) {
				return Node{}, fmt.Errorf("nodes %v and %v have different shared randomness seed", first.Coverage, child.Coverage)
			}
			// The contributions live in the multiset-hash group, whose
			// operation is the componentwise addition.
			contribution = multisethashing.Combine(contribution,
				messagebus.GetSharedRandomnessContribution(m.sys, child.Pub))
		}
	}

	for h, p := range products {
		_, pos := m.sys.LookupPublicInputByTag(messagebus.PublicInputTag, h)
		pub[pos] = p
	}
	if seeded {
		for i := range contribution {
			_, pos := m.sys.LookupPublicInputByTag(messagebus.SharedRandomnessSeedContributionPI, i)
			pub[pos] = field.ElemFromBase(contribution[i])
		}
	}
	return Node{Coverage: coverage, Pub: pub}, nil
}

// Finalize implements [Merger].
func (m MockMerger) Finalize(root Node, numShards uint) (Node, error) {
	if err := checkCoverageComplete(root.Coverage, numShards); err != nil {
		return Node{}, err
	}
	for h, p := range messagebus.GetProducts(m.sys, root.Pub) {
		if diff := p.Sub(field.ElemOne()); !diff.IsZero() {
			return Node{}, fmt.Errorf("message-bus handle %d does not balance across the shards: product is %v", h, p)
		}
	}
	if messagebus.HasSharedRandomness(m.sys) {
		seed := multisethashing.ToSeed(messagebus.GetSharedRandomnessContribution(m.sys, root.Pub))
		if messagebus.GetSharedRandomnessSeed(m.sys, root.Pub) != seed {
			return Node{}, fmt.Errorf("the shared randomness seed is not derived from the shards' contributions")
		}
	}
	return root, nil
}
