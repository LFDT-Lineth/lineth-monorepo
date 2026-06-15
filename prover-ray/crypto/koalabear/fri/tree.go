package fri

import (
	"errors"

	"github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/poseidon2"
	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/linea-monorepo/prover-ray/utils"
)

// Tree is a Merkle tree for multi-size FRI. The tree is 3-ary, each node may
// have:
//
//   - 0 children: leaf
//   - 2 children: internal node and there is no batch of polynomial
//     evaluations corresponding to this layer.
//   - 3 children: internal node and there is a batch of polynomial
//     evaluations corresponding to this layer.
type Tree struct {
	// Nodes stores the nodes of the tree. The first node is the root. The 
	// children of node k are at indices 2k+1 and 2k+2.
	Nodes []field.Octuplet
	// Aux stores the auxiliary leaves of the tree. Aux[i] is the auxiliary leaf 
	// of Nodes[i]. Thus Nodes[i] = H(Nodes[2*i+1], Nodes[2*i+2], Aux[i])
	Aux []*field.Octuplet
}

// Branch is a Merkle opening proof for a single leaf. The branch does  not open 
// a particular position but all the leaves targetted by a FRI IOP query. Unlike
// usual Merkle proof construction, the Branch contains the leaf. The reason is
// that it is used to open all the leaves in the same branch in practice.
//
// The root may be reconstructed from the branch thanks to the following 
// pseudo-code
//
// ```
// curPos := idx
// curr := Leaf
// for i := len(Siblings) - 1; i >= 0; i-- {
//		left, right = curr, leaf[i]
//		if currPos & 1 > 0:
//			left, right = right, left
//		curr = Hash(left, right, aux[i])
//		currPos = currPos >> 1
// }
// return curr // now equal to the root
// ```
type Branch struct {
	// Leaf is the deepest leaf that could be queried in the tree.
	Leaf        field.Octuplet
	// Siblings stores the siblings of the opened branch. The first sibling
	// corresponds to the greatest uncle of the opened leaf, just below of the
	// root. The last entry corresponds to the
	Siblings    []field.Octuplet
	// AuxSiblings are the auxiliary siblings. We have 
	// `len(Siblings) == len(AuxSiblings)``
	AuxSiblings []*field.Octuplet
}

// NewTree builds a new Tree from the given leaves. The leaves must be  provided 
// in the following order.
//
//	for all 0 <= i < len(leaves): leaves[i] = 2**(N-i-1) or 0
func NewTree(leaves [][]field.Octuplet) *Tree {

	if len(leaves) == 0 {
		panic("at least one level must be provided")
	}

	if len(leaves[0]) == 0 {
		panic("the first level must be non-empty")
	}

	for i, _ := range leaves {
		n := len(leaves[i])
		if n != 0 && len(leaves[i]) != 1<<(len(leaves)-i-1) {
			panic("leaves must be provided in the following order: " +
				"for all 0 <= i < len(leaves): leaves[i] = 2**(N-i-1)")
		}
	}

	var (
		nodes = make([]field.Octuplet, 2*len(leaves[0])-1)
		aux   = make([]*field.Octuplet, len(leaves[0])-1)
	)

	copy(nodes[len(leaves[0])-1:], leaves[0])

	for i := 1; i < len(leaves); i++ {

		var (
			n             = 1 << (len(leaves) - i - 1)
			levelStartPos = 1<<i - 1
		)

		for j := 0; j < n; j++ {

			k := levelStartPos + j

			if aux[k] != nil {
				panic("indices on aux are wrong and we are overlapping values")
			}

			if len(leaves[i]) > 0 {
				// we already asserted that len(leaves[i]) == n. So this 
				// will not go OOB.
				aux[k] = &leaves[i][j]
			}

			left, right := nodes[2*k+1], nodes[2*k+2]
			if (nodes[k] != field.Octuplet{}) {
				panic("already computed node; the indexing must be wrong")
			}

			nodes[k] = hashNode(left, right, aux[k])
		}
	}

	// as the tree cannot be empty (as per our sanity-checks), the root cannot 
	// be zero.
	if nodes[0] == (field.Octuplet{}) {
		panic("sanity-check failed : the root is zero.")
	}

	return &Tree{
		Nodes: nodes,
		Aux:   aux,
	}
}

// Root returns the Merkle root digest. Build must be called first.
func (t *Tree) Root() field.Octuplet {
	return t.Nodes[0]
}

// NumLevel returns the number of levels in the tree. It verifies 
// that numNode = 2^numLevel - 1
func (t *Tree) NumLevel() int {
	return utils.Log2Ceil(len(t.Nodes))
}

// NumLeaves returns the number of leaves in the tree
func (t *Tree) NumLeaves() int {
	return (len(t.Nodes) + 1) / 2
}

// OpenProof returns the Merkle opening proof for the leaf at 0-based index idx. 
// The function panics if the requested position is not openable.
func (t *Tree) OpenBranch(idx int) Branch {
	
	if idx < 0 || idx >= t.NumLeaves() {
		panic("out of bound opening")
	}

	// The branch is computed from the bottom-up. current initially points to
	// the position of the leaf in Node and is updated to its parent position
	// iteratively until we reach the top of the tree. idxRemBit helps tracking
	// the current node is a left or a right child to its parent throughout the
	// iteration.

	var (
		current = len(t.Aux) + idx
		idxRemBits = idx
		numSiblings = t.NumLevel() - 1
		branch = Branch{
			Siblings: make([]field.Octuplet, numSiblings),
			AuxSiblings: make([]*field.Octuplet, numSiblings),
			Leaf: t.Nodes[current],
		}
	)

	for level := numSiblings - 1; level >= 0; level-- {

		var (
			parent = current / 2
			currBit = idxRemBits & 1
			sibling = 2 * parent + 2 - currBit 
		)

		branch.AuxSiblings[level] = t.Aux[parent]
		branch.Siblings[level] = t.Nodes[sibling]
		idxRemBits <<= 1
		current = parent
	}


	return branch
}

// RecoverRoot recovers the root of the tree from a branch and a position. The
// function errors if 
func (branch *Branch) RecoverRoot(idx int) (field.Octuplet, error) {

	if len(branch.AuxSiblings) != len(branch.Siblings) {
		return field.Octuplet{}, errors.New("malformed proof")
	}

	if len(branch.Siblings) == 0 {
		panic("empty proof")
	}

	var (
		ancestor = branch.Leaf
		currPos = idx
	)

	for i := len(branch.Siblings) - 1; i >= 0; i-- {

		left, right := ancestor, branch.Siblings[i]
		if currPos & 1 > 0 {
			left, right = right, left
		}

		ancestor = hashNode(left, right, branch.AuxSiblings[i])
		currPos <<= 1
	}

	if currPos > 0 {
		return field.Octuplet{}, errors.New("all bits of currPos should have been bitshifted beyond LSb")
	}

	return ancestor, nil
}

// hashNode hashes two field.Octuplets and an optional field.Octuplet.
func hashNode(left, right field.Octuplet, aux *field.Octuplet) field.Octuplet {
	hasher := poseidon2.NewMDHasher()
	hasher.WriteElements(left[:]...)
	hasher.WriteElements(right[:]...)
	if aux != nil {
		hasher.WriteElements(aux[:]...)
	}
	return hasher.SumDigest()
}
