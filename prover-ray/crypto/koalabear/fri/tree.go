package fri

import (
	"errors"
	"fmt"
	"math/bits"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/poseidon2"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/parallel"
	gnarkposeidon2 "github.com/consensys/gnark-crypto/field/koalabear/poseidon2"
)

// minParallelTreeLevel avoids paying goroutine scheduling costs for the small
// levels near the root. Larger levels contain independent nodes and benefit
// from using all available CPUs.
const minParallelTreeLevel = 512

// batchLanes is the width of the batched Poseidon2 compression.
const batchLanes = 16

var batchPoseidon2 = gnarkposeidon2.NewPermutation(16, 6, 21)

// Tree is a Merkle tree for multi-size FRI. Every internal node hashes its two
// children, then each digest injected at its level, in order:
//
//	Nodes[k] = C(…C(C(Nodes[2k+1], Nodes[2k+2]), a_1)…, a_m)
//
// where a_1, …, a_m are the auxiliary digests injected into node k. A level
// with no injection has m = 0; a single injection is the classic 3-ary node.
type Tree struct {
	// Nodes stores the nodes of the tree. The first node is the root. The
	// children of node k are at indices 2k+1 and 2k+2.
	Nodes []field.Octuplet
	// Injections lists, per level of internal nodes, the auxiliary digests
	// injected into that level, in injection order. Level l holds the 2^l
	// nodes Nodes[2^l-1 : 2^(l+1)-1], and Injections[l][i][j] is the i-th
	// digest injected into the j-th node of that level.
	Injections [][][]field.Octuplet
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
	// AuxSiblings are the digests injected into each parent on the path, in
	// injection order (empty for a parent without injection). We have
	// `len(Siblings) == len(AuxSiblings)`.
	AuxSiblings [][]field.Octuplet
}

// MerkleCap authenticates the shared upper part of a Merkle tree. Nodes are
// the frontier, from left to right; Aux is the heap-order prefix above it.
// An empty cap represents depth zero and carries no copy of the known root.
//
// A cap carries at most one injected digest per node: it is the serialized
// form of the running FRI trees' caps, which have none. Trees with several
// injections per node (input trees) rebuild their caps from revealed rows
// instead, see [capRoot].
type MerkleCap struct {
	Nodes []field.Octuplet
	Aux   []*field.Octuplet
}

// OptimalMerkleCapHeight returns the cap height that minimises the proof's
// digest count for numQueries queries. The cap holds the 2^height frontier
// nodes; every branch meets that frontier in exactly one node, so each branch
// is opened only below the frontier and drops its top `height` siblings.
// Descending one more level doubles the cap and saves one sibling on each of
// the numQueries branches, so the cost balances at 2^height = numQueries.
func OptimalMerkleCapHeight(numQueries uint) uint8 {
	if numQueries <= 1 {
		return 0
	}
	return uint8(bits.Len(numQueries - 1))
}

// merkleCapDepth returns the depth of the frontier shared by all query
// branches of a tree of the given height: [Params.MerkleCapHeight], clamped to
// height-1 so that every query keeps at least one branch step and value
// extraction stays uniform.
func (p Params) merkleCapDepth(height int) int {
	if height <= 1 {
		return 0
	}
	return min(int(p.MerkleCapHeight), height-1)
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

	injections := make([][][]field.Octuplet, bottom)
	for i := range injections {
		if len(leaves[i]) != 0 {
			injections[i] = [][]field.Octuplet{leaves[i]}
		}
	}

	t := allocTree(len(leaves[bottom]))
	copy(t.Nodes[len(leaves[bottom])-1:], leaves[bottom])
	t.buildLevels(injections)
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
		Nodes:      make([]field.Octuplet, 2*numLeaves-1),
		Injections: make([][][]field.Octuplet, utils.Log2Ceil(numLeaves)),
	}
}

// buildLevels computes every internal level bottom-up; Nodes[NumLeaves()-1:]
// must already hold the bottom leaves. injections, if non-nil, lists the
// digests injected into each level of internal nodes, in injection order and
// indexed as [Tree.Injections]: every injected slice of level l has 2^l
// digests.
func (t *Tree) buildLevels(injections [][][]field.Octuplet) {
	n := t.NumLeaves()
	for levelSize := n / 2; levelSize > 0; levelSize /= 2 {
		level := utils.Log2Ceil(levelSize)
		if injections != nil {
			t.Injections[level] = injections[level]
		}
		hashTreeLevel(t.Nodes, t.Injections[level], levelSize)
	}

	// as the tree cannot be empty (as per our sanity-checks), the root cannot
	// be zero.
	if n > 1 && t.Nodes[0] == (field.Octuplet{}) {
		panic("sanity-check failed : the root is zero.")
	}
}

// hashTreeLevel computes the complete level of levelSize internal nodes at
// heap positions [levelSize-1, 2*levelSize-1). Children belong to
// already-computed lower levels, so the writes are independent. Every slice of
// injections holds exactly one digest per node of this level; they are
// chained into each node in order.
func hashTreeLevel(
	nodes []field.Octuplet,
	injections [][]field.Octuplet,
	levelSize int,
) {
	levelStart := levelSize - 1
	for _, injected := range injections {
		if len(injected) != levelSize {
			panic("fri: hashTreeLevel: invalid auxiliary leaf count")
		}
	}

	// Levels smaller than one batch (only the topmost few nodes) are hashed
	// with the scalar compression.
	if levelSize < batchLanes {
		for j := range levelSize {
			k := levelStart + j
			res := poseidon2.Compress(nodes[2*k+1], nodes[2*k+2])
			for _, injected := range injections {
				res = poseidon2.Compress(res, injected[j])
			}
			nodes[k] = res
		}
		return
	}

	// Levels are powers of two, so levelSize is a multiple of batchLanes.
	// Each group of 16 sibling pairs is staged column-major and hashed by one
	// batched Poseidon2 compression, writing the 16 parents directly into
	// nodes[k0:k0+16]. Bit-identical to the scalar loop: each injection is one
	// more block of the same C(…C(C(left,right),a_1)…,a_m) chain.
	var (
		nbGroups = levelSize / batchLanes
		nbSteps  = 1 + len(injections)
	)

	hashGroups := func(gStart, gEnd int) {
		state := make([]field.Element, 8*batchLanes)
		matrix := make([]field.Element, nbSteps*8*batchLanes)
		for g := gStart; g < gEnd; g++ {
			var (
				j        = g * batchLanes
				k0       = levelStart + j
				children = nodes[2*k0+1 : 2*k0+1+2*batchLanes]
			)
			// Stage both buffers in one pass per lane.
			for lane := range batchLanes {
				left, right := &children[2*lane], &children[2*lane+1]
				for pos := range 8 {
					state[pos*batchLanes+lane] = left[pos]
					matrix[pos*batchLanes+lane] = right[pos]
				}
			}
			for step, injected := range injections {
				row := 8 * (1 + step)
				for lane := range batchLanes {
					auxLeaf := &injected[j+lane]
					for pos := range 8 {
						matrix[(row+pos)*batchLanes+lane] = auxLeaf[pos]
					}
				}
			}
			batchPoseidon2.Compressx16ColumnsWithState(
				state,
				matrix,
				nbSteps*8,
				nodes[k0:k0+batchLanes],
			)
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
	// 2n−1 nodes for n leaves: log2(2n) levels, including n = 1.
	return utils.Log2Ceil(len(t.Nodes) + 1)
}

// NumLeaves returns the number of leaves in the tree
func (t *Tree) NumLeaves() int {
	return (len(t.Nodes) + 1) / 2
}

// injectedInto returns the digests injected into internal node k, in
// injection order, or nil if there is none.
func (t *Tree) injectedInto(k int) []field.Octuplet {
	level := bits.Len(uint(k+1)) - 1
	injections := t.Injections[level]
	if len(injections) == 0 {
		return nil
	}
	j := k - (1<<level - 1)
	res := make([]field.Octuplet, len(injections))
	for i, injected := range injections {
		res[i] = injected[j]
	}
	return res
}

// OpenBranch returns the Merkle opening proof for the leaf at 0-based index idx.
// The function panics if the requested position is not openable.
func (t *Tree) OpenBranch(idx int) Branch {
	return t.OpenBranchToDepth(idx, 0)
}

// OpenBranchToDepth opens idx only up to a depth-c cap frontier. The returned
// branch retains the lower path, in the same shallowest-to-deepest order as
// OpenBranch. The caller must authenticate that frontier separately.
func (t *Tree) OpenBranchToDepth(idx, depth int) Branch {

	if idx < 0 || idx >= t.NumLeaves() {
		panic("out of bound opening")
	}
	if depth < 0 || depth > t.NumLevel()-1 {
		panic("fri: cap depth out of bounds")
	}

	// The branch is computed from the bottom-up. current initially points to
	// the position of the leaf in Node and is updated to its parent position
	// iteratively until we reach the top of the tree. idxRemBit helps tracking
	// the current node is a left or a right child to its parent throughout the
	// iteration.

	var (
		current     = t.NumLeaves() - 1 + idx
		idxRemBits  = idx
		numSiblings = t.NumLevel() - 1
		branch      = Branch{
			Siblings:    make([]field.Octuplet, numSiblings-depth),
			AuxSiblings: make([][]field.Octuplet, numSiblings-depth),
			Leaf:        t.Nodes[current],
		}
	)

	for level := numSiblings - 1; level >= depth; level-- {

		var (
			parent  = (current - 1) / 2
			currBit = idxRemBits & 1
			sibling = 2*parent + 2 - currBit
		)

		branch.AuxSiblings[level-depth] = t.injectedInto(parent)
		branch.Siblings[level-depth] = t.Nodes[sibling]
		idxRemBits >>= 1
		current = parent
	}

	return branch
}

// OpenCap copies the shared prefix above a depth-c frontier. Auxiliary
// digests are deep-copied so later proof mutation cannot alter the tree. It
// panics if a node of the prefix has more than one injection, which a
// [MerkleCap] cannot carry.
func (t *Tree) OpenCap(depth int) MerkleCap {
	if depth < 0 || depth > t.NumLevel()-1 {
		panic("fri: treeCap depth out of bounds")
	}
	if depth == 0 {
		return MerkleCap{}
	}

	frontierStart := 1<<depth - 1
	frontierEnd := 2*frontierStart + 1
	treeCap := MerkleCap{
		Nodes: append([]field.Octuplet(nil), t.Nodes[frontierStart:frontierEnd]...),
		Aux:   make([]*field.Octuplet, frontierStart),
	}
	for i := range frontierStart {
		switch injected := t.injectedInto(i); len(injected) {
		case 0:
		case 1:
			treeCap.Aux[i] = &injected[0]
		default:
			panic(fmt.Sprintf("fri: OpenCap: node %d has %d injections, a MerkleCap carries at most one", i, len(injected)))
		}
	}
	return treeCap
}

// Validate checks that cap is the canonical representation for depth.
func (cap MerkleCap) Validate(depth int) error {
	if depth < 0 {
		return errors.New("negative cap depth")
	}
	if depth == 0 {
		if len(cap.Nodes) != 0 || len(cap.Aux) != 0 {
			return errors.New("depth-zero cap must be empty")
		}
		return nil
	}
	if depth >= bits.UintSize-1 {
		return errors.New("cap depth too large")
	}
	wantNodes := 1 << depth
	if len(cap.Nodes) != wantNodes {
		return fmt.Errorf("cap has %d nodes, want %d", len(cap.Nodes), wantNodes)
	}
	if len(cap.Aux) != wantNodes-1 {
		return fmt.Errorf("cap has %d auxiliary digests, want %d", len(cap.Aux), wantNodes-1)
	}
	return nil
}

// RecoverRoot rebuilds the root from a structurally valid nonempty cap.
func (cap MerkleCap) RecoverRoot(depth int) (field.Octuplet, error) {
	if err := cap.Validate(depth); err != nil {
		return field.Octuplet{}, err
	}
	if depth == 0 {
		return field.Octuplet{}, errors.New("empty cap has no root")
	}

	injections := make([][]field.Octuplet, len(cap.Aux))
	for i, aux := range cap.Aux {
		if aux != nil {
			injections[i] = []field.Octuplet{*aux}
		}
	}
	return capRoot(cap.Nodes, injections), nil
}

// capRoot rebuilds the root above a frontier of nodes, a nonempty power of two
// left to right. injections[i] lists the digests injected into the i-th node of
// the heap-order prefix above the frontier, in injection order, and may be
// empty.
func capRoot(nodes []field.Octuplet, injections [][]field.Octuplet) field.Octuplet {
	work := make([]field.Octuplet, 2*len(nodes)-1)
	copy(work[len(nodes)-1:], nodes)
	for i := len(nodes) - 2; i >= 0; i-- {
		work[i] = hashNode(work[2*i+1], work[2*i+2], injections[i]...)
	}
	return work[0]
}

// Authenticate checks a cap once against the caller's trusted root. A
// depth-zero cap is empty and represents the trusted root directly.
func (cap MerkleCap) Authenticate(depth int, root field.Octuplet) error {
	if depth == 0 {
		return cap.Validate(depth)
	}
	recovered, err := cap.RecoverRoot(depth)
	if err != nil {
		return err
	}
	if recovered != root {
		return errors.New("invalid Merkle cap")
	}
	return nil
}

// AuthenticateToCap checks this lower branch against a previously
// authenticated frontier.
func (branch *Branch) AuthenticateToCap(idx int, frontier []field.Octuplet) error {
	if idx < 0 {
		return errors.New("negative opening index")
	}
	if len(frontier) == 0 || len(frontier)&(len(frontier)-1) != 0 {
		return errors.New("frontier size is not a power of two")
	}
	if len(branch.AuxSiblings) != len(branch.Siblings) {
		return errors.New("malformed proof")
	}
	if len(branch.Siblings) >= bits.UintSize ||
		len(frontier) > int(^uint(0)>>1)>>len(branch.Siblings) ||
		idx >= len(frontier)<<len(branch.Siblings) {
		return errors.New("opening index outside capped tree")
	}

	ancestor := branch.Leaf
	currPos := idx
	for i := len(branch.Siblings) - 1; i >= 0; i-- {
		left, right := ancestor, branch.Siblings[i]
		if currPos&1 > 0 {
			left, right = right, left
		}
		ancestor = hashNode(left, right, branch.AuxSiblings[i]...)
		currPos >>= 1
	}
	if ancestor != frontier[currPos] {
		return errors.New("invalid Merkle proof")
	}
	return nil
}

// hashNode hashes two field.Octuplets, then chains the injected digests in
// order. It calls the compression function C directly (not MD hashing):
// res = C(…C(C(left, right), injected_1)…, injected_m).
func hashNode(left, right field.Octuplet, injected ...field.Octuplet) field.Octuplet {
	res := poseidon2.Compress(left, right)
	for i := range injected {
		res = poseidon2.Compress(res, injected[i])
	}
	return res
}
