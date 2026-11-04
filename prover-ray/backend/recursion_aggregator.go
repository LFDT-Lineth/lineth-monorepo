package backend

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/recursion/node"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers"
)

// recursionAggregator implements [Aggregator] with the recursion machine's
// node program (see package recursion/node). It verifies nothing about the
// children: their propagated public inputs are plain inputs of the node
// program, which checks that they agree on the program VK and the guest
// output, multiplies their bus products and exposes the result; the root
// program also checks that every product is one. A disagreement is caught by
// verifying the node's own proof. Dev and test only, until the node program
// verifies the children's proofs.
//
// A child is a shard proof or a node proof, told apart by the length of its
// public inputs: a shard exposes one extension cell per bus handle, a node six
// base cells.
type recursionAggregator struct {
	shard      piLayout
	node, root *node.Program
}

// newRecursionAggregator compiles the node and root programs for the shard
// proofs' public-input layout.
func newRecursionAggregator(shard piLayout, opts ...compilers.Option) (*recursionAggregator, error) {
	l := node.Layout{NumBus: len(shard.bus), NumVK: len(shard.programVK), NumOut: len(shard.guestOutput)}
	nodeProgram, err := node.New(l, 2, false, opts...)
	if err != nil {
		return nil, fmt.Errorf("node program: %w", err)
	}
	rootProgram, err := node.New(l, 1, true, opts...)
	if err != nil {
		return nil, fmt.Errorf("root program: %w", err)
	}
	if nodeProgram.NumPublicInputs() == shard.numPublicInputs {
		return nil, fmt.Errorf("shard and node proofs both have %d public inputs, they cannot be told apart", shard.numPublicInputs)
	}
	return &recursionAggregator{shard: shard, node: nodeProgram, root: rootProgram}, nil
}

// child decodes the propagated values of a shard proof or a node proof.
func (a *recursionAggregator) child(n Node) (node.Child, error) {
	switch len(n.Pub) {
	case a.shard.numPublicInputs:
		return a.shardChild(n)
	case a.node.NumPublicInputs():
		c, err := a.node.Decode(n.Pub)
		if err != nil {
			return node.Child{}, fmt.Errorf("node %v: %w", n.Coverage, err)
		}
		return c, nil
	default:
		return node.Child{}, fmt.Errorf("node %v has %d public inputs, neither a shard proof (%d) nor a node proof (%d)",
			n.Coverage, len(n.Pub), a.shard.numPublicInputs, a.node.NumPublicInputs())
	}
}

// shardChild decodes the propagated values of a shard proof.
func (a *recursionAggregator) shardChild(n Node) (node.Child, error) {
	base := func(pos int) (field.Element, error) {
		if !n.Pub[pos].IsBase() {
			return field.Element{}, fmt.Errorf("node %v: public input %d is not a base field element", n.Coverage, pos)
		}
		return n.Pub[pos].AsBase(), nil
	}
	var (
		c = node.Child{
			Bus: make([]field.Ext, len(a.shard.bus)),
			VK:  make([]field.Element, len(a.shard.programVK)),
			Out: make([]field.Element, len(a.shard.guestOutput)),
		}
		err error
	)
	for h, pos := range a.shard.bus {
		c.Bus[h] = n.Pub[pos].AsExt()
	}
	for i, pos := range a.shard.programVK {
		if c.VK[i], err = base(pos); err != nil {
			return node.Child{}, err
		}
	}
	for k, pos := range a.shard.guestOutput {
		if c.Out[k], err = base(pos); err != nil {
			return node.Child{}, err
		}
	}
	return c, nil
}

// Aggregate implements [Aggregator].
func (a *recursionAggregator) Aggregate(left, right Node) (Node, error) {
	coverage, err := mergeCoverage(left.Coverage, right.Coverage)
	if err != nil {
		return Node{}, err
	}
	l, err := a.child(left)
	if err != nil {
		return Node{}, err
	}
	r, err := a.child(right)
	if err != nil {
		return Node{}, err
	}
	proof, pub, err := a.node.ProveChildren(l, r)
	if err != nil {
		return Node{}, fmt.Errorf("aggregating nodes %v and %v: %w", left.Coverage, right.Coverage, err)
	}
	return Node{Coverage: coverage, Proof: proof, Pub: pub}, nil
}

// Finalize implements [Aggregator].
func (a *recursionAggregator) Finalize(root Node, numShards uint) (Node, error) {
	if err := checkCoverageComplete(root.Coverage, numShards); err != nil {
		return Node{}, err
	}
	c, err := a.child(root)
	if err != nil {
		return Node{}, err
	}
	proof, pub, err := a.root.ProveChildren(c)
	if err != nil {
		return Node{}, fmt.Errorf("finalizing the root: %w", err)
	}
	return Node{Coverage: root.Coverage, Proof: proof, Pub: pub}, nil
}
