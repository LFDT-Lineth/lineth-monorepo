package conglomeration

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/recursion/node"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/messagebus"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver/risc5"
)

// A node program merges RecursionTreeArity children: the two constants must
// agree, which these conversions only compile if they do.
const _ = uint(RecursionTreeArity-node.Arity) + uint(node.Arity-RecursionTreeArity)

// roles is how a node combines each public input of a RISCV-ZKC shard proof.
var roles = map[wiop.PublicInputTag]node.Op{
	messagebus.PublicInputTag:            node.OpMul,
	risc5.ProgramVKPI:                    node.OpEqual,
	risc5.GuestPublicOutputsPI:           node.OpEqual,
	risc5.SharedRandomnessPI:             node.OpEqual,
	risc5.SharedRandomnessContributionPI: node.OpAdd,
}

// RecursionMerger is the dev and test [Merger] proving the merges with the
// recursion machine's node programs (package recursion/node). Shard proofs
// are merged by the leaf program, merged nodes by the internal program
// whatever their depth, and the root by the root program: one key per role,
// so that the tree has any shape. A merged node's public inputs are the shard
// layout followed by the node's key box: empty for a leaf node, the internal
// program's key for the others, which the root program pins (see the node
// package).
//
// The programs verify nothing about the children: their public inputs are
// plain inputs, which the programs check agree on the program VK, the guest
// output and the seed, multiply the bus products of, add the contributions
// of, and apply the key rule to; the root program also checks that every
// product is one. A disagreement is caught by verifying the node's own
// proof. The derivation of γ from the contributions stays a Go check at the
// root until the root program hashes them.
type RecursionMerger struct {
	sys                  *wiop.System // the shard system, for its public-input layout
	shard                node.Layout  // the layout of a shard proof
	leaf, internal, root *node.Program
}

// NewRecursionMerger compiles the leaf, internal and root programs for the
// public-input layout of sys, a compiled RISCV-ZKC system (see
// risc5.CheckPublicInputs).
func NewRecursionMerger(sys *wiop.System, opts ...compilers.Option) (*RecursionMerger, error) {
	shard, err := node.LayoutOf(sys, roles)
	if err != nil {
		return nil, err
	}
	leaf, err := node.NewLeaf(shard, opts...)
	if err != nil {
		return nil, fmt.Errorf("leaf program: %w", err)
	}
	internal, err := node.NewInternal(shard, opts...)
	if err != nil {
		return nil, fmt.Errorf("internal program: %w", err)
	}
	root, err := node.NewRoot(shard, internal.Key(), opts...)
	if err != nil {
		return nil, fmt.Errorf("root program: %w", err)
	}
	return &RecursionMerger{sys: sys, shard: shard, leaf: leaf, internal: internal, root: root}, nil
}

// Merge implements [Merger].
func (m *RecursionMerger) Merge(children []Node) (Node, error) {
	if err := checkChildren(children); err != nil {
		return Node{}, err
	}
	first := children[0]
	program := m.leaf
	if first.Kind == Merged {
		program = m.internal
	}
	coverage := first.Coverage
	inputs := make([]node.Child, len(children))
	for c, child := range children {
		var err error
		if inputs[c], err = m.child(child); err != nil {
			return Node{}, err
		}
		if c > 0 {
			if coverage, err = mergeCoverage(coverage, child.Coverage); err != nil {
				return Node{}, err
			}
			if err := m.checkAgree(first, child); err != nil {
				return Node{}, err
			}
		}
	}
	proof, pub, err := program.ProveChildren(m.internal.Key(), inputs...)
	if err != nil {
		return Node{}, fmt.Errorf("merging nodes %v: %w", coverage, err)
	}
	return Node{Kind: Merged, Coverage: coverage, Proof: proof, Pub: pub}, nil
}

// child checks the shape of a node's public inputs, the shard layout for a
// shard proof and the node layout for a merged node, and lays it out as a
// child: a merged node whose key box is empty is a leaf node, one carrying
// the internal program's key an internal node; any other box is foreign.
func (m *RecursionMerger) child(n Node) (node.Child, error) {
	want := len(m.shard)
	if n.Kind == Merged {
		want = len(m.internal.Layout)
	}
	if len(n.Pub) != want {
		return node.Child{}, fmt.Errorf("node %v has %d public inputs, want %d", n.Coverage, len(n.Pub), want)
	}
	if n.Kind != Merged {
		return node.Child{Pub: n.Pub}, nil
	}
	key, err := node.KeyOf(n.Pub)
	if err != nil {
		return node.Child{}, fmt.Errorf("node %v: %w", n.Coverage, err)
	}
	switch key {
	case field.Octuplet{}:
		return node.Child{Pub: n.Pub, Leaf: true}, nil
	case m.internal.Key():
		return node.Child{Pub: n.Pub}, nil
	}
	return node.Child{}, fmt.Errorf("node %v carries a foreign key box %v", n.Coverage, key)
}

// checkAgree names the first propagated public input a and b differ on, so
// that a disagreement is reported before a proof is attempted.
func (m *RecursionMerger) checkAgree(a, b Node) error {
	for pos, role := range m.shard {
		if role.Op != node.OpEqual {
			continue
		}
		if diff := a.Pub[pos].Sub(b.Pub[pos]); !diff.IsZero() {
			return fmt.Errorf("nodes %v and %v have different %s_%d", a.Coverage, b.Coverage, role.Tag, role.Index)
		}
	}
	return nil
}

// Finalize implements [Merger].
func (m *RecursionMerger) Finalize(root Node, numShards uint) (Node, error) {
	if err := checkRoot(root); err != nil {
		return Node{}, err
	}
	if err := checkCoverageComplete(root.Coverage, numShards); err != nil {
		return Node{}, err
	}
	child, err := m.child(root)
	if err != nil {
		return Node{}, err
	}
	if err := checkSeedDerived(m.sys, root.Pub); err != nil {
		return Node{}, err
	}
	proof, pub, err := m.root.ProveChildren(m.internal.Key(), child)
	if err != nil {
		return Node{}, fmt.Errorf("finalizing the root: %w", err)
	}
	return Node{Kind: Merged, Coverage: root.Coverage, Proof: proof, Pub: pub}, nil
}

// VerifyRoot verifies the proof of a root returned by [RecursionMerger.Finalize].
func (m *RecursionMerger) VerifyRoot(root Node) error {
	return m.root.Verify(root.Proof, root.Pub)
}
