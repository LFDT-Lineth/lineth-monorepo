package fri

import (
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/poseidon2"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
)

// layerTree is the Merkle tree of a running FRI layer, as the prover opens it.
type layerTree interface {
	Root() field.Octuplet
	OpenCap(depth int) MerkleCap
	OpenBranchToDepth(idx, depth int) Branch
}

// zeroLayerTree is the Merkle tree [buildTreeExt] builds over the zero codeword
// of 2^height entries, represented by its per-level digests: every node of a
// level is the same digest, so committing to and opening a zero layer costs
// O(height) instead of O(2^height). A FixedDomain PCS folds the zero codeword
// in the rounds before its largest size opened joins.
type zeroLayerTree struct {
	height int
}

// digest returns the node of the zero tree whose subtree has height h: the
// zero leaf for h = 0, then C(z_{h-1}, z_{h-1}).
func (zeroLayerTree) digest(h int) field.Octuplet {
	var z field.Octuplet
	for range h {
		z = poseidon2.Compress(z, z)
	}
	return z
}

func (t zeroLayerTree) Root() field.Octuplet { return t.digest(t.height) }

// OpenCap mirrors [Tree.OpenCap]: the 2^depth frontier nodes, without
// injections.
func (t zeroLayerTree) OpenCap(depth int) MerkleCap {
	if depth < 0 || depth > t.height {
		panic("fri: treeCap depth out of bounds")
	}
	if depth == 0 {
		return MerkleCap{}
	}
	nodes := make([]field.Octuplet, 1<<depth)
	node := t.digest(t.height - depth)
	for i := range nodes {
		nodes[i] = node
	}
	return MerkleCap{Nodes: nodes, Aux: make([]*field.Octuplet, 1<<depth-1)}
}

// OpenBranchToDepth mirrors [Tree.OpenBranchToDepth]: the zero leaf, and the
// siblings from the depth-c frontier down to the leaves.
func (t zeroLayerTree) OpenBranchToDepth(idx, depth int) Branch {
	if idx < 0 || idx >= 1<<t.height {
		panic("out of bound opening")
	}
	if depth < 0 || depth > t.height {
		panic("fri: cap depth out of bounds")
	}
	branch := Branch{
		Siblings:    make([]field.Octuplet, t.height-depth),
		AuxSiblings: make([][]field.Octuplet, t.height-depth),
	}
	// Siblings[i] is at depth depth+i+1, its subtree of height height-depth-i-1.
	node := field.Octuplet{}
	for i := len(branch.Siblings) - 1; i >= 0; i-- {
		branch.Siblings[i] = node
		node = poseidon2.Compress(node, node)
	}
	return branch
}
