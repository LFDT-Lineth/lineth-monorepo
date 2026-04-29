package smt_koalabear

import (
	"fmt"
	"runtime"

	"github.com/consensys/linea-monorepo/prover/crypto/poseidon2_koalabear"

	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/parallel"
)

// DefaultDepth is the default depth of the Linea state Merkle tree.
// This value should not be changed as it would modify the state structure.
const DefaultDepth = 40

// Tree represents a binary sparse Merkle-tree (SMT).
type Tree struct {
	Depth int
	// Root stores the root of the tree
	Root field.Octuplet
	// OccupiedLeaves continuously list of the occupied leaves. For the toy
	// implementation we track all the leaves.
	OccupiedLeaves []field.Octuplet
	// Occupied not stores all the node with a non-trivial value in the tree.
	//
	// Does not include the root. (So there are 39 levels and not 40).
	// Returns a node at a given level:
	// 	- 0 is the level just above the leaves
	// 	- 38 is the level just below the root
	OccupiedNodes [][]field.Octuplet
	// EmptyNodes stores the value of the trivial nodes of the SMT (i.e the one
	// corresponding to empty sub-trees).
	//
	// It does not include the "empty root" nor the empty leaf
	// so the first position contains the empty node for the level one.
	// So there are 39, and not 40 levels. That way, the indexing stays
	// consistent with "OccupiedNode"
	EmptyNodes []field.Octuplet
}

// NewEmptyTree creates and returns an empty tree with the provided config.
func NewEmptyTree(depths ...int) *Tree {
	// Default depth is DefaultDepth if no input is provided
	depth := DefaultDepth
	if len(depths) > 0 && depths[0] > 0 {
		depth = depths[0]
	} // Computes the empty nodes
	emptyNodes := make([]field.Octuplet, depth-1)
	prevNode := EmptyLeaf()

	hasher := poseidon2_koalabear.NewMDHasher()

	for i := range emptyNodes {
		newNode := hashLR(hasher, prevNode, prevNode)
		emptyNodes[i] = newNode
		prevNode = newNode
	}

	// Stores the initial root separately
	root := hashLR(hasher, prevNode, prevNode)

	return &Tree{
		Depth:          depth,
		Root:           root,
		OccupiedLeaves: make([]field.Octuplet, 0),
		OccupiedNodes:  make([][]field.Octuplet, depth-1),
		EmptyNodes:     emptyNodes,
	}
}

// NewTree builds from scratch a complete Merkle-tree. Requires that the
// input leaves are powers of 2. The depth of the tree is deduced from the list.
//
// It panics if the number of leaves is a non-power of 2.
// The hash function is by default poseidon2 over koalabear.
func NewTree(leaves []field.Octuplet) *Tree {
	numLeaves := len(leaves)

	if !utils.IsPowerOfTwo(numLeaves) || numLeaves == 0 {
		utils.Panic("expected power of two number of leaves, got %v", numLeaves)
	}

	depth := utils.Log2Ceil(numLeaves)

	// Pre-allocate all intermediate nodes in a single slice to improve memory locality
	// and reduce allocation overhead.
	// Total intermediate nodes = numLeaves - 2 (excluding root and leaves)
	var allNodes []field.Octuplet
	if depth > 1 {
		allNodes = make([]field.Octuplet, numLeaves-2)
	}

	tree := &Tree{
		Depth:          depth,
		OccupiedLeaves: leaves,
		OccupiedNodes:  make([][]field.Octuplet, depth-1),
		EmptyNodes:     make([]field.Octuplet, depth-1),
	}

	// Initialize EmptyNodes
	{
		prevNode := EmptyLeaf()
		hasher := poseidon2_koalabear.NewMDHasher()
		for i := range tree.EmptyNodes {
			newNode := hashLR(hasher, prevNode, prevNode)
			tree.EmptyNodes[i] = newNode
			prevNode = newNode
		}
	}

	// Slice allNodes into OccupiedNodes
	if depth > 1 {
		offset := 0
		currentLevelSize := numLeaves / 2
		for i := 0; i < depth-1; i++ {
			tree.OccupiedNodes[i] = allNodes[offset : offset+currentLevelSize]
			offset += currentLevelSize
			currentLevelSize /= 2
		}
	}

	// Parallelization Strategy:
	// 1. Process the bottom of the tree in small, fixed-size subtrees.
	//    This creates many tasks (much more than numCPU), ensuring better load balancing
	//    via the scheduler, preventing the "tail latency" problem where some cores finish early.
	// 2. Process the upper levels level-by-level, parallelizing each level if it's large enough.

	// 1. Bottom-up subtrees
	// We choose a subtree height that fits comfortably in L1/L2 cache (e.g., height 10 => 1024 leaves).
	// This ensures that a single task stays hot in cache while computing its subtree.
	const targetSubtreeHeight = 10
	subtreeHeight := targetSubtreeHeight
	// We can only compute up to depth-1 in OccupiedNodes.
	if subtreeHeight > depth-1 {
		subtreeHeight = depth - 1
	}
	if subtreeHeight < 0 {
		subtreeHeight = 0
	}

	chunkSize := 1 << subtreeHeight
	numTasks := numLeaves / chunkSize

	// Execute parallel tasks. Each task computes a complete subtree of height `subtreeHeight`.
	parallel.Execute(numTasks, func(startTask, endTask int) {
		hasher := poseidon2_koalabear.NewMDHasher()

		for t := startTask; t < endTask; t++ {
			leafStart := t * chunkSize

			// Process Level 0 (Leaves -> OccupiedNodes[0])
			if subtreeHeight > 0 {
				outStart := t * (chunkSize / 2)
				count := chunkSize / 2

				leaves := tree.OccupiedLeaves
				outLevel := tree.OccupiedNodes[0]

				for k := 0; k < count; k++ {
					outLevel[outStart+k] = hashLR(hasher, leaves[leafStart+2*k], leaves[leafStart+2*k+1])
				}
			}

			// Process Levels 1 to subtreeHeight-1
			// Level l input is OccupiedNodes[l-1], output is OccupiedNodes[l]
			for l := 1; l < subtreeHeight; l++ {
				inputStart := t * (chunkSize / (1 << l))
				outputStart := t * (chunkSize / (1 << (l + 1)))
				count := chunkSize / (1 << (l + 1))

				inLevel := tree.OccupiedNodes[l-1]
				outLevel := tree.OccupiedNodes[l]

				for k := 0; k < count; k++ {
					outLevel[outputStart+k] = hashLR(hasher, inLevel[inputStart+2*k], inLevel[inputStart+2*k+1])
				}
			}
		}
	})

	// 2. Upper levels
	// Continue from where the subtrees left off.
	for i := subtreeHeight; i < depth-1; i++ {
		var prevLevel []field.Octuplet
		if i == 0 {
			prevLevel = tree.OccupiedLeaves
		} else {
			prevLevel = tree.OccupiedNodes[i-1]
		}
		currLevel := tree.OccupiedNodes[i]

		n := len(currLevel)
		// Threshold to justify parallel overhead.
		const parallelThreshold = 32

		if n >= parallelThreshold && runtime.GOMAXPROCS(0) > 1 {
			parallel.Execute(n, func(start, end int) {
				hasher := poseidon2_koalabear.NewMDHasher()
				for k := start; k < end; k++ {
					currLevel[k] = hashLR(hasher, prevLevel[2*k], prevLevel[2*k+1])
				}
			})
		} else {
			hasher := poseidon2_koalabear.NewMDHasher()
			for k := 0; k < n; k++ {
				currLevel[k] = hashLR(hasher, prevLevel[2*k], prevLevel[2*k+1])
			}
		}
	}

	// Root calculation
	if depth == 1 {
		tree.Root = hashLR(poseidon2_koalabear.NewMDHasher(), leaves[0], leaves[1])
	} else {
		lastLevel := tree.OccupiedNodes[depth-2]
		if len(lastLevel) != 2 {
			utils.Panic("broken invariant : len(lastLevel) != 2, =%v", len(lastLevel))
		}
		tree.Root = hashLR(poseidon2_koalabear.NewMDHasher(), lastLevel[0], lastLevel[1])
	}

	return tree
}

func (t *Tree) GetRoot() field.Octuplet {
	return t.Root
}

// GetLeaf returns a leaf by position or an error if the leaf is out of bounds.
func (t *Tree) GetLeaf(pos int) (field.Octuplet, error) {
	// Check that the accessed node is within the bounds of the SMT
	maxPos := 1 << t.Depth
	if pos >= maxPos {
		return field.Octuplet{}, fmt.Errorf("out of bound: %v", pos)
	}

	if pos < 0 {
		return field.Octuplet{}, fmt.Errorf("negative position: %v", pos)
	}
	// Check if this is an empty leaf
	if pos >= len(t.OccupiedLeaves) {
		return EmptyLeaf(), nil
	}
	// Return the leaf if occupied
	return t.OccupiedLeaves[pos], nil
}

// MustGetLeaf is as [Tree.GetLeaf] but panics on errors.
func (t *Tree) MustGetLeaf(pos int) field.Octuplet {
	l, err := t.GetLeaf(pos)
	if err != nil {
		utils.Panic("could not get leaf: %v", err.Error())
	}
	return l
}

// getNode returns a node at a given level:
//   - 0 is a leaf
//   - 1 - 39 : is an intermediate node
//   - 40 is the root
//
// (for config.Depth == 40)
func (t *Tree) getNode(level, posInLevel int) field.Octuplet {
	switch {
	case level == t.Depth:
		// The only logical posInLevels value is zero in this case
		if posInLevel > 0 {
			utils.Panic("there is only one root but posInLevel was %v", posInLevel)
		}
		// Opportunistic sanity-checks. Parenthesis are necessary because
		// of a hole in my linter.
		if t.Root == (field.Octuplet{}) {
			utils.Panic("sanity-check failed : the root is zero.")
		}
		return t.Root
	case level >= 1 && level <= t.Depth-1:
		// Check that the accessed node is within the bounds of the SMT
		maxPos := 1 << (t.Depth - level)
		if posInLevel >= maxPos {
			utils.Panic("nodeID is out of bound")
		}
		// Check if this is an empty node
		if posInLevel >= len(t.OccupiedNodes[level-1]) {
			return t.EmptyNodes[level-1]
		}
		// Or return an non-empty one
		res := t.OccupiedNodes[level-1][posInLevel]
		if res == (field.Octuplet{}) {
			utils.Panic("sanity-check : intermediary node is 0")
		}
		return res
	case level == 0:
		leaf, err := t.GetLeaf(posInLevel)
		if err != nil {
			utils.Panic("node corresponds to an OOB leaf: %v", err)
		}
		return leaf
	default:
		utils.Panic("Got level %v", level)
	}
	panic("unreachable")
}

// updateNode updates a node at a given level:
//   - 0 is a leaf
//   - 1 - 39 : is an intermediate node
//   - 40 is the root
//
// (for config.Depth == 40)
func (t *Tree) updateNode(level, posInLevel int, newVal field.Octuplet) {
	switch {
	case level == t.Depth:
		// The only logical posInLevels value is zero in this case
		if posInLevel > 0 {
			utils.Panic("there is only one root but posInLevel was %v", posInLevel)
		}
		// Opportunistic sanity-checks. Parenthesis are necessary because
		// of a hole in my linter.
		if t.Root == (field.Octuplet{}) {
			utils.Panic("sanity-check failed : the root is zero.")
		}
		t.Root = newVal
	case level >= 1 && level < t.Depth:
		// Check that the accessed node is within the bounds of the SMT
		maxPos := 1 << (t.Depth - level)
		if posInLevel >= maxPos {
			utils.Panic("node is out of bound level %v (maxPos %v), pos=%v", level, maxPos, posInLevel)
		}
		// Check if this is an empty node : and reserve if necessary
		if posInLevel >= len(t.OccupiedNodes[level-1]) {
			t.reserveLevel(level, posInLevel+1)
		}
		// Or return an non-empty one
		t.OccupiedNodes[level-1][posInLevel] = newVal
	case level == 0:
		// Check that the accessed node is within the bounds of the SMT
		maxPos := 1 << t.Depth
		if posInLevel >= maxPos {
			utils.Panic("nodeID is out of bound")
		}
		// Check if this is an empty leaf
		if posInLevel >= len(t.OccupiedLeaves) {
			t.reserveLevel(0, posInLevel+1)
		}
		// Return the leaf if occupied
		t.OccupiedLeaves[posInLevel] = newVal
	default:
		utils.Panic("Got level %v", level)
	}
}

// reserveLevel extends the `OccupiedLeaves` and `OccupiedNodes` fields of the
// tree by appending `trivial nodes` to the specified level/
//
// Level == 0 : reserve in the leaves
// Level == 1..39 : reserve in the nodes
// Level == 40 : panic, can't reserve at the root level
//
// (for config.Depth == 40)
func (t *Tree) reserveLevel(level, newSize int) {

	// Edge-case, level out of bound
	if level > t.Depth {
		utils.Panic("level out of bound %v", level)
	}

	// Edge : case root level
	if level == t.Depth {
		utils.Panic("tried extending the root level %v", newSize)
	}

	// Will work for both the leaves and the intermediate nodes
	if newSize > 1<<(t.Depth-level) {
		utils.Panic("overextending the tree %v, %v", level, newSize)
	}

	if level == 0 {
		if newSize <= len(t.OccupiedLeaves) {
			// already big enough
			return
		}
		// else, we add extra "empty leaves" at the end of the slice
		padding := make([]field.Octuplet, newSize-len(t.OccupiedLeaves))
		for i := range padding {
			padding[i] = EmptyLeaf()
		}
		t.OccupiedLeaves = append(t.OccupiedLeaves, padding...)
		return
	}

	/*
		Else we want to reserve within the occupiedNodes
	*/
	if newSize <= len(t.OccupiedNodes[level-1]) {
		// already big enough
		return
	}
	// else, we add extra "empty nodes" at the end of the slice
	padding := make([]field.Octuplet, newSize-len(t.OccupiedNodes[level-1]))
	for i := range padding {
		padding[i] = t.EmptyNodes[level-1]
	}
	t.OccupiedNodes[level-1] = append(t.OccupiedNodes[level-1], padding...)
}

// EmptyLeaf returns an empty leaf (e.g. the zero value).
func EmptyLeaf() field.Octuplet {
	return field.Octuplet{}
}

// hashLR is used for hashing the leaf-right children. It returns H(nodeL, nodeR)
// taking H as the HashFunc of the config.
func hashLR(hasher *poseidon2_koalabear.MDHasher, nodeL, nodeR field.Octuplet) field.Octuplet {
	hasher.Reset()
	var d field.Octuplet
	hasher.WriteElements(nodeL[:]...)
	hasher.WriteElements(nodeR[:]...)
	d = hasher.SumElement()
	return d
}
