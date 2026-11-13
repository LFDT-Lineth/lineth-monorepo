package conglomeration

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/recursionguests/node"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/recursionguests/verifier"
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
	risc5.SharedRandomnessPI:             node.OpSeed,
	risc5.SharedRandomnessContributionPI: node.OpAdd,
}

// RecursionMerger is the dev and test [Merger] proving the merges with the
// recursion machine's node programs (package recursionguests/node): the merger is
// the host side of a merge, picking the program by the children's kind and
// checking their coverage and shape, and the program its in-circuit layer.
// Shard proofs are merged by the leaf program, merged nodes by the internal program
// whatever their depth, and the root by the root program: one key per role,
// so that the tree has any shape. A merged node's public inputs are the shard
// layout followed by the node's key box: empty for a leaf node, the internal
// program's key for the others, which the root program pins (see the node
// package).
//
// The programs verify the children with the in-machine verifier its caller
// chooses (see package recursionguests/verifier): verifier.NewMock in tests, as
// verifier.New fails until the wiop verifier is written in the machine's
// instructions. The programs then check the children agree on the program VK, the
// guest output and the seed γ, multiply the bus products, add the
// contributions and apply the key rule; the root program also checks that
// every product is one and that γ is the digest of the contributions added
// over every shard. A violation is caught by verifying the node's own proof.
type RecursionMerger struct {
	sys                  *wiop.System // the shard system, for its public-input layout
	shard                node.Layout  // the layout of a shard proof
	leaf, internal, root *node.Program
}

// NewRecursionMerger compiles the leaf, internal and root programs for the
// public-input layout of sys, a compiled RISCV-ZKC system (see
// risc5.CheckPublicInputs), verifying their children with the verifiers
// newVerifier returns.
func NewRecursionMerger(sys *wiop.System, newVerifier verifier.Constructor, opts ...compilers.Option) (*RecursionMerger, error) {
	shard, err := node.LayoutOf(sys, roles)
	if err != nil {
		return nil, err
	}
	m := &RecursionMerger{sys: sys, shard: shard}
	if m.leaf, err = node.NewLeaf(shard, sys, newVerifier, opts...); err != nil {
		return nil, fmt.Errorf("leaf program: %w", err)
	}
	if m.internal, err = node.NewInternal(shard, m.leaf, newVerifier, opts...); err != nil {
		return nil, fmt.Errorf("internal program: %w", err)
	}
	if m.root, err = node.NewRoot(shard, m.leaf, m.internal, newVerifier, opts...); err != nil {
		return nil, fmt.Errorf("root program: %w", err)
	}
	return m, nil
}

// Merge implements [Merger].
func (m *RecursionMerger) Merge(children []Node) (Node, error) {
	if err := checkChildren(children); err != nil {
		return Node{}, err
	}
	program := m.leaf
	if children[0].Kind == Merged {
		program = m.internal
	}
	inputs, coverage, err := m.children(children)
	if err != nil {
		return Node{}, err
	}
	proof, pub, err := program.ProveChildren(m.internal.Key(), inputs...)
	if err != nil {
		return Node{}, fmt.Errorf("merging nodes %v: %w", coverage, err)
	}
	return Node{Kind: Merged, Coverage: coverage, Proof: proof, Pub: pub}, nil
}

// children lays the children of a merge out for a node program, checking
// their shapes, that their coverages are disjoint and that they agree on the
// propagated values, and returns the union of their coverage.
func (m *RecursionMerger) children(children []Node) ([]node.Child, []uint, error) {
	first := children[0]
	coverage := first.Coverage
	inputs := make([]node.Child, len(children))
	for c, child := range children {
		var err error
		if inputs[c], err = m.child(child); err != nil {
			return nil, nil, err
		}
		if c > 0 {
			if coverage, err = mergeCoverage(coverage, child.Coverage); err != nil {
				return nil, nil, err
			}
			if err := m.checkAgree(first, child); err != nil {
				return nil, nil, err
			}
		}
	}
	return inputs, coverage, nil
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
		return node.Child{Proof: n.Proof, Pub: n.Pub}, nil
	}
	key, err := node.KeyOf(n.Pub)
	if err != nil {
		return node.Child{}, fmt.Errorf("node %v: %w", n.Coverage, err)
	}
	switch key {
	case field.Octuplet{}:
		return node.Child{Proof: n.Proof, Pub: n.Pub, Leaf: true}, nil
	case m.internal.Key():
		return node.Child{Proof: n.Proof, Pub: n.Pub}, nil
	}
	return node.Child{}, fmt.Errorf("node %v carries a foreign key box %v", n.Coverage, key)
}

// checkAgree names the first propagated public input a and b differ on, so
// that a disagreement is reported before a proof is attempted.
func (m *RecursionMerger) checkAgree(a, b Node) error {
	for pos, role := range m.shard {
		if role.Op != node.OpEqual && role.Op != node.OpSeed {
			continue
		}
		if diff := a.Pub[pos].Sub(b.Pub[pos]); !diff.IsZero() {
			return fmt.Errorf("nodes %v and %v have different %s_%d", a.Coverage, b.Coverage, role.Tag, role.Index)
		}
	}
	return nil
}

// Finalize implements [Merger].
func (m *RecursionMerger) Finalize(children []Node, numShards uint) (Node, error) {
	if err := checkRoot(children); err != nil {
		return Node{}, err
	}
	inputs, coverage, err := m.children(children)
	if err != nil {
		return Node{}, err
	}
	if err := checkCoverageComplete(coverage, numShards); err != nil {
		return Node{}, err
	}
	// Named before the root program rejects it at verification, as checkAgree
	// does for a disagreement.
	pubs := make([]wiop.PublicInput, len(children))
	for c, child := range children {
		pubs[c] = child.Pub
	}
	if err := checkSeedDerived(m.sys, pubs...); err != nil {
		return Node{}, err
	}
	proof, pub, err := m.root.ProveChildren(m.internal.Key(), inputs...)
	if err != nil {
		return Node{}, fmt.Errorf("finalizing the root: %w", err)
	}
	return Node{Kind: Merged, Coverage: coverage, Proof: proof, Pub: pub}, nil
}

// VerifyRoot verifies the proof of a root returned by [RecursionMerger.Finalize].
func (m *RecursionMerger) VerifyRoot(root Node) error {
	return m.root.Verify(root.Proof, root.Pub)
}
