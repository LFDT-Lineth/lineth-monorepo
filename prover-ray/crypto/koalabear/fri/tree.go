package fri

import (
	"errors"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/poseidon2"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/parallel"
)

// minParallelTreeLevel avoids paying goroutine scheduling costs for the small
// levels near the root. Larger levels contain independent nodes and benefit
// from using all available CPUs.
const minParallelTreeLevel = 512

// batchLanes is the width of the batched Poseidon2 compression used for
// internal tree levels.
const batchLanes = poseidon2.BatchLanes

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
// a particular position but all the leaves targeted by a FRI IOP query. Unlike
// usual Merkle proof construction, the Branch contains the leaf. The reason is
// that it is used to open all the leaves in the same branch in practice.
//
// The root may be reconstructed from the branch thanks to the following
// pseudo-code
//
// ```
// curPos := idx
// curr := Leaf
//
//	for i := len(Siblings) - 1; i >= 0; i-- {
//			left, right = curr, leaf[i]
//			if currPos & 1 > 0:
//				left, right = right, left
//			curr = Hash(left, right, aux[i])
//			currPos = currPos >> 1
//	}
//
// return curr // now equal to the root
// ```
type Branch struct {
	// Leaf is the deepest leaf that could be queried in the tree.
	Leaf field.Octuplet
	// Siblings stores the siblings of the opened branch. The first sibling
	// corresponds to the greatest uncle of the opened leaf, just below of the
	// root. The last entry corresponds to the
	Siblings []field.Octuplet
	// AuxSiblings are the auxiliary siblings. We have
	// `len(Siblings) == len(AuxSiblings)``
	AuxSiblings []*field.Octuplet
}

// NewTree builds a new Tree from the given leaves. The leaves must be  provided
// in increasing-size order, from the top of the tree (smallest) down to the
// bottom layer (largest):
//
//	for all 0 <= i < len(leaves): len(leaves[i]) = 2**i or 0
func NewTree(leaves [][]field.Octuplet) *Tree {

	if len(leaves) == 0 {
		panic("at least one level must be provided")
	}

	// The bottom layer (the largest, deepest leaves) must be non-empty.
	bottom := len(leaves) - 1
	if len(leaves[bottom]) == 0 {
		panic("the bottom level must be non-empty")
	}

	for i := range leaves {
		n := len(leaves[i])
		if n != 0 && n != 1<<i {
			panic("leaves must be provided in the following order: " +
				"for all 0 <= i < len(leaves): leaves[i] = 2**i")
		}
	}

	t := allocTree(len(leaves[bottom]))
	copy(t.Nodes[len(leaves[bottom])-1:], leaves[bottom])
	t.buildLevels(leaves[:bottom])
	return t
}

// allocTree allocates the node and aux storage for a tree with the given
// power-of-two number of bottom leaves. The caller must fill
// Nodes[numLeaves-1:] with the bottom leaves and then call buildLevels.
func allocTree(numLeaves int) *Tree {
	if numLeaves <= 0 || numLeaves&(numLeaves-1) != 0 {
		panic("fri: allocTree: number of leaves must be a positive power of two")
	}
	return &Tree{
		Nodes: make([]field.Octuplet, 2*numLeaves-1),
		Aux:   make([]*field.Octuplet, numLeaves-1),
	}
}

// buildLevels computes every internal level bottom-up; Nodes[NumLeaves()-1:]
// must already hold the bottom leaves. upperLeaves, if non-nil, lists the
// auxiliary leaves of the levels above the bottom, indexed as in [NewTree]:
// len(upperLeaves[i]) is 2^i or 0.
func (t *Tree) buildLevels(upperLeaves [][]field.Octuplet) {
	n := t.NumLeaves()
	for levelSize := n / 2; levelSize > 0; levelSize /= 2 {
		var auxLeaves []field.Octuplet
		if upperLeaves != nil {
			auxLeaves = upperLeaves[utils.Log2Ceil(levelSize)]
		}
		hashTreeLevel(t.Nodes, t.Aux, auxLeaves, levelSize)
	}

	// as the tree cannot be empty (as per our sanity-checks), the root cannot
	// be zero.
	if n > 1 && t.Nodes[0] == (field.Octuplet{}) {
		panic("sanity-check failed : the root is zero.")
	}
}

// hashTreeLevel computes the complete level of levelSize internal nodes at
// heap positions [levelSize-1, 2*levelSize-1). Children belong to
// already-computed lower levels, so the writes are independent. If leaves is
// non-empty, it contains exactly one auxiliary leaf per node at this level.
func hashTreeLevel(
	nodes []field.Octuplet,
	aux []*field.Octuplet,
	leaves []field.Octuplet,
	levelSize int,
) {
	levelStart := levelSize - 1
	if len(leaves) != 0 && len(leaves) != levelSize {
		panic("fri: hashTreeLevel: invalid auxiliary leaf count")
	}

	// Levels smaller than one batch (only the topmost few nodes) are hashed
	// with the scalar compression.
	if levelSize < batchLanes {
		for j := range levelSize {
			k := levelStart + j
			if len(leaves) != 0 {
				aux[k] = &leaves[j]
			}
			nodes[k] = hashNode(nodes[2*k+1], nodes[2*k+2], aux[k])
		}
		return
	}

	// Levels are powers of two, so levelSize is a multiple of batchLanes.
	// Each group of 16 sibling pairs is staged column-major and hashed by one
	// batched Poseidon2 compression, writing the 16 parents directly into
	// nodes[k0:k0+16]. Bit-identical to the scalar hashNode loop; with aux
	// leaves, C(C(left,right),aux) is the same chain with one more block.
	var (
		nbGroups = levelSize / batchLanes
		hasAux   = len(leaves) != 0
		nbSteps  = 1
	)
	if hasAux {
		nbSteps = 2
	}

	hashGroups := func(gStart, gEnd int) {
		state := make([]field.Element, 8*batchLanes)
		matrix := make([]field.Element, nbSteps*8*batchLanes)
		for g := gStart; g < gEnd; g++ {
			var (
				j        = g * batchLanes
				k0       = levelStart + j
				children = nodes[2*k0+1 : 2*k0+1+2*batchLanes]
			)
			// Fused poseidon2.StageOctuplet loops: one pass per lane over both
			// buffers measurably beats two helper calls on this hot path.
			for lane := range batchLanes {
				left, right := &children[2*lane], &children[2*lane+1]
				for pos := range 8 {
					state[pos*batchLanes+lane] = left[pos]
					matrix[pos*batchLanes+lane] = right[pos]
				}
			}
			if hasAux {
				for lane := range batchLanes {
					auxLeaf := &leaves[j+lane]
					aux[k0+lane] = auxLeaf
					for pos := range 8 {
						matrix[(8+pos)*batchLanes+lane] = auxLeaf[pos]
					}
				}
			}
			poseidon2.CompressChain16(state, matrix, nodes[k0:k0+batchLanes])
		}
	}

	if levelSize < minParallelTreeLevel {
		hashGroups(0, nbGroups)
		return
	}
	parallel.Execute(nbGroups, hashGroups)
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
		current     = len(t.Aux) + idx
		idxRemBits  = idx
		numSiblings = t.NumLevel() - 1
		branch      = Branch{
			Siblings:    make([]field.Octuplet, numSiblings),
			AuxSiblings: make([]*field.Octuplet, numSiblings),
			Leaf:        t.Nodes[current],
		}
	)

	for level := numSiblings - 1; level >= 0; level-- {

		var (
			parent  = (current - 1) / 2
			currBit = idxRemBits & 1
			sibling = 2*parent + 2 - currBit
		)

		branch.AuxSiblings[level] = t.Aux[parent]
		branch.Siblings[level] = t.Nodes[sibling]
		idxRemBits >>= 1
		current = parent
	}

	return branch
}

// RecoverRoot recovers the root of the tree from a branch and a position. The
// function errors if the branch is malformed its size is inconsistent with idx.
func (branch *Branch) RecoverRoot(idx int) (field.Octuplet, error) {

	if len(branch.AuxSiblings) != len(branch.Siblings) {
		return field.Octuplet{}, errors.New("malformed proof")
	}

	if len(branch.Siblings) == 0 {
		return field.Octuplet{}, errors.New("empty proof")
	}

	var (
		ancestor = branch.Leaf
		currPos  = idx
	)

	for i := len(branch.Siblings) - 1; i >= 0; i-- {
		left, right := ancestor, branch.Siblings[i]
		if currPos&1 > 0 {
			left, right = right, left
		}

		ancestor = hashNode(left, right, branch.AuxSiblings[i])
		currPos >>= 1
	}

	if currPos > 0 {
		return field.Octuplet{}, errors.New("all bits of currPos should have been bitshifted beyond LSb")
	}

	return ancestor, nil
}

// hashNode hashes two field.Octuplets and an optional field.Octuplet. It works
// by calling the compression function C directly (not MD hashing).
// res = C(left, right) or C(aux, C(left, right))
func hashNode(left, right field.Octuplet, aux *field.Octuplet) field.Octuplet {
	res := poseidon2.Compress(left, right)
	if aux != nil {
		res = poseidon2.Compress(res, *aux)
	}
	return res
}
