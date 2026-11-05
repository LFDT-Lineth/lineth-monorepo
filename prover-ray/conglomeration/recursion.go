package conglomeration

import (
	"fmt"

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
// recursion machine's node program (package recursion/node). It verifies
// nothing about the children: their public inputs are plain inputs of the
// node program, which checks that they agree on the program VK, the guest
// output and the seed, multiplies their bus products, adds their
// contributions and exposes the result in the shard layout; the root program
// also checks that every product is one. A disagreement is caught by
// verifying the node's own proof. Shard proofs and merged nodes go through
// the same program, as nothing is verified yet; the kinds are still kept
// apart, as the contract requires. The derivation of γ from the
// contributions stays a Go check at the root until the root program hashes
// them.
type RecursionMerger struct {
	sys        *wiop.System // the shard system, for its public-input layout
	layout     node.Layout
	node, root *node.Program
}

// NewRecursionMerger compiles the node and root programs for the public-input
// layout of sys, a compiled RISCV-ZKC system (see risc5.CheckPublicInputs).
func NewRecursionMerger(sys *wiop.System, opts ...compilers.Option) (*RecursionMerger, error) {
	layout, err := node.LayoutOf(sys, roles)
	if err != nil {
		return nil, err
	}
	nodeProgram, err := node.New(layout, false, opts...)
	if err != nil {
		return nil, fmt.Errorf("node program: %w", err)
	}
	rootProgram, err := node.New(layout, true, opts...)
	if err != nil {
		return nil, fmt.Errorf("root program: %w", err)
	}
	return &RecursionMerger{sys: sys, layout: layout, node: nodeProgram, root: rootProgram}, nil
}

// Merge implements [Merger].
func (m *RecursionMerger) Merge(children []Node) (Node, error) {
	if err := checkChildren(children); err != nil {
		return Node{}, err
	}
	first := children[0]
	coverage := first.Coverage
	pubs := make([]wiop.PublicInput, len(children))
	for c, child := range children {
		if err := m.checkShape(child); err != nil {
			return Node{}, err
		}
		if c > 0 {
			var err error
			if coverage, err = mergeCoverage(coverage, child.Coverage); err != nil {
				return Node{}, err
			}
			if err := m.checkAgree(first, child); err != nil {
				return Node{}, err
			}
		}
		pubs[c] = child.Pub
	}
	proof, pub, err := m.node.ProveChildren(pubs...)
	if err != nil {
		return Node{}, fmt.Errorf("merging nodes %v: %w", coverage, err)
	}
	return Node{Kind: Merged, Coverage: coverage, Proof: proof, Pub: pub}, nil
}

// checkShape checks that a node's public inputs have the shard layout's
// length.
func (m *RecursionMerger) checkShape(n Node) error {
	if len(n.Pub) != len(m.layout) {
		return fmt.Errorf("node %v has %d public inputs, want %d", n.Coverage, len(n.Pub), len(m.layout))
	}
	return nil
}

// checkAgree names the first propagated public input a and b differ on, so
// that a disagreement is reported before a proof is attempted.
func (m *RecursionMerger) checkAgree(a, b Node) error {
	for pos, role := range m.layout {
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
	if err := m.checkShape(root); err != nil {
		return Node{}, err
	}
	if err := checkSeedDerived(m.sys, root.Pub); err != nil {
		return Node{}, err
	}
	proof, pub, err := m.root.ProveChildren(root.Pub)
	if err != nil {
		return Node{}, fmt.Errorf("finalizing the root: %w", err)
	}
	return Node{Kind: Merged, Coverage: root.Coverage, Proof: proof, Pub: pub}, nil
}

// VerifyRoot verifies the proof of a root returned by [RecursionMerger.Finalize].
func (m *RecursionMerger) VerifyRoot(root Node) error {
	return m.root.Verify(root.Proof, root.Pub)
}
