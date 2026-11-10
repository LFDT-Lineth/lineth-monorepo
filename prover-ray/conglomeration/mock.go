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
// shared randomness contributions added; a lifted shard proof keeps its public
// inputs and loses its proof.
//
// It verifies no proof either: the shard proofs it merges must have been
// verified by whoever produced them, and a merged node has none to verify.
type MockMerger struct {
	sys *wiop.System
}

// NewMockMerger returns the mock merger of the shard proofs over sys, a
// compiled RISCV-ZKC system (see risc5.CheckPublicInputs).
func NewMockMerger(sys *wiop.System) MockMerger {
	return MockMerger{sys: sys}
}

// Merge implements [Merger].
func (m MockMerger) Merge(children []Node) (Node, error) {
	if err := checkChildren(children); err != nil {
		return Node{}, err
	}
	return m.combine(children)
}

// combine checks the shape and agreement of the children and returns their
// parent.
func (m MockMerger) combine(children []Node) (Node, error) {
	for _, child := range children {
		if len(child.Pub) != len(m.sys.PublicInputs) {
			return Node{}, fmt.Errorf("node %v has %d public inputs, want %d",
				child.Coverage, len(child.Pub), len(m.sys.PublicInputs))
		}
	}
	var (
		first        = children[0]
		coverage     = first.Coverage
		vk           = risc5.GetProgramVK(m.sys, first.Pub)
		output       = risc5.GetGuestPublicOutputs(m.sys, first.Pub)
		products     = messagebus.GetProducts(m.sys, first.Pub)
		seeded       = messagebus.HasSharedRandomness(m.sys)
		seed         field.Octuplet
		contribution multisethashing.MSetHash
	)
	if seeded {
		seed = messagebus.GetSharedRandomnessSeed(m.sys, first.Pub)
		contribution = messagebus.GetSharedRandomnessContribution(m.sys, first.Pub)
	}
	for _, child := range children[1:] {
		var err error
		if coverage, err = mergeCoverage(coverage, child.Coverage); err != nil {
			return Node{}, err
		}
		if vk != risc5.GetProgramVK(m.sys, child.Pub) {
			return Node{}, fmt.Errorf("nodes %v and %v have different program VK", first.Coverage, child.Coverage)
		}
		if !slices.Equal(output, risc5.GetGuestPublicOutputs(m.sys, child.Pub)) {
			return Node{}, fmt.Errorf("nodes %v and %v have different guest output", first.Coverage, child.Coverage)
		}
		for h, p := range messagebus.GetProducts(m.sys, child.Pub) {
			products[h] = products[h].Mul(p)
		}
		if seeded {
			if seed != messagebus.GetSharedRandomnessSeed(m.sys, child.Pub) {
				return Node{}, fmt.Errorf("nodes %v and %v have different shared randomness seed", first.Coverage, child.Coverage)
			}
			// The contributions live in the multiset-hash group, whose
			// operation is the componentwise addition.
			contribution = multisethashing.Combine(contribution,
				messagebus.GetSharedRandomnessContribution(m.sys, child.Pub))
		}
	}

	pub := slices.Clone(first.Pub)
	messagebus.SetProducts(m.sys, pub, products)
	if seeded {
		messagebus.SetSharedRandomnessContribution(m.sys, pub, contribution)
	}
	return Node{Kind: Merged, Coverage: coverage, Pub: pub}, nil
}

// Finalize implements [Merger].
func (m MockMerger) Finalize(children []Node, numShards uint) (Node, error) {
	if err := checkRoot(children); err != nil {
		return Node{}, err
	}
	root, err := m.combine(children)
	if err != nil {
		return Node{}, err
	}
	if err := checkCoverageComplete(root.Coverage, numShards); err != nil {
		return Node{}, err
	}
	for h, p := range messagebus.GetProducts(m.sys, root.Pub) {
		if diff := p.Sub(field.ElemOne()); !diff.IsZero() {
			return Node{}, fmt.Errorf("message-bus handle %d does not balance across the shards: product is %v", h, p)
		}
	}
	if err := checkSeedDerived(m.sys, root.Pub); err != nil {
		return Node{}, err
	}
	return root, nil
}

// checkSeedDerived checks, when sys has shared randomness, that the seed γ
// of the public inputs pubs, of the children of a root, is the one their
// contributions, added up, derive.
func checkSeedDerived(sys *wiop.System, pubs ...wiop.PublicInput) error {
	if !messagebus.HasSharedRandomness(sys) {
		return nil
	}
	sum := multisethashing.Identity()
	for _, pub := range pubs {
		sum = multisethashing.Combine(sum, messagebus.GetSharedRandomnessContribution(sys, pub))
	}
	if messagebus.GetSharedRandomnessSeed(sys, pubs[0]) != multisethashing.ToSeed(sum) {
		return fmt.Errorf("the shared randomness seed is not derived from the shards' contributions")
	}
	return nil
}
