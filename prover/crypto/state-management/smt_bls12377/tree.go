package smt_bls12377

import (
	"fmt"

	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	"github.com/consensys/linea-monorepo/prover/crypto/poseidon2_bls12377"

	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/parallel"
)

// Tree represents a binary sparse Merkle-tree (SMT).
type Tree struct {
	Depth int

	// Config of the hash function
	// Config *Config
	// Root stores the root of the tree
	Root fr.Element
	// OccupiedLeaves continuously list of the occupied leaves. For the toy
	// implementation we track all the leaves.
	OccupiedLeaves []fr.Element
	// Occupied not stores all the node with a non-trivial value in the tree.
	//
	// Does not include the root. (So there are 39 levels and not 40).
	// Returns a node at a given level:
	// 	- 0 is the level just above the leaves
	// 	- 38 is the level just below the root
	OccupiedNodes [][]fr.Element
	// EmptyNodes stores the value of the trivial nodes of the SMT (i.e the one
	// corresponding to empty sub-trees).
	//
	// It does not include the "empty root" nor the empty leaf
	// so the first position contains the empty node for the level one.
	// So there are 39, and not 40 levels. That way, the indexing stays
	// consistent with "OccupiedNode"
	EmptyNodes []fr.Element
}

// EmptyLeaf returns an empty leaf (e.g. the zero value).
func EmptyLeaf() fr.Element {
	return fr.Element{}
}

// hashLR is used for hashing the leaf-right children. It returns H(nodeL, nodeR)
// taking H as the HashFunc of the config.
func hashLR(nodeL, nodeR fr.Element) fr.Element {
	var d fr.Element
	hasher := poseidon2_bls12377.NewMDHasher()
	hasher.WriteElements(nodeL)
	hasher.WriteElements(nodeR)
	d = hasher.SumElement()
	return d
}

// NewEmptyTree creates and returns an empty tree
func NewEmptyTree(depth int) *Tree {
	// Computes the empty nodes
	emptyNodes := make([]fr.Element, depth-1)
	prevNode := EmptyLeaf()

	for i := range emptyNodes {
		newNode := hashLR(prevNode, prevNode)
		emptyNodes[i] = newNode
		prevNode = newNode
	}

	// Stores the initial root separately
	root := hashLR(prevNode, prevNode)

	return &Tree{
		Depth:          depth,
		Root:           root,
		OccupiedLeaves: make([]fr.Element, 0),
		OccupiedNodes:  make([][]fr.Element, depth-1),
		EmptyNodes:     emptyNodes,
	}
}

func (t *Tree) GetRoot() fr.Element {
	return t.Root
}

// GetLeaf returns a leaf by position or an error if the leaf is out of bounds.
func (t *Tree) GetLeaf(pos int) (fr.Element, error) {
	// Check that the accessed node is within the bounds of the SMT
	maxPos := 1 << t.Depth
	if pos >= maxPos {
		return fr.Element{}, fmt.Errorf("out of bound: %v", pos)
	}

	if pos < 0 {
		return fr.Element{}, fmt.Errorf("negative position: %v", pos)
	}
	// Check if this is an empty leaf
	if pos >= len(t.OccupiedLeaves) {
		return EmptyLeaf(), nil
	}
	// Return the leaf if occupied
	return t.OccupiedLeaves[pos], nil
}

// MustGetLeaf is as [Tree.GetLeaf] but panics on errors.
func (t *Tree) MustGetLeaf(pos int) fr.Element {
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
func (t *Tree) getNode(level, posInLevel int) fr.Element {
	switch {
	case level == t.Depth:
		// The only logical posInLevels value is zero in this case
		if posInLevel > 0 {
			utils.Panic("there is only one root but posInLevel was %v", posInLevel)
		}
		// Opportunistic sanity-checks. Parenthesis are necessary because
		// of a hole in my linter.
		if t.Root == (fr.Element{}) {
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
		if res == (fr.Element{}) {
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
func (t *Tree) updateNode(level, posInLevel int, newVal fr.Element) {
	switch {
	case level == t.Depth:
		// The only logical posInLevels value is zero in this case
		if posInLevel > 0 {
			utils.Panic("there is only one root but posInLevel was %v", posInLevel)
		}
		// Opportunistic sanity-checks. Parenthesis are necessary because
		// of a hole in my linter.
		if t.Root == (fr.Element{}) {
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
		padding := make([]fr.Element, newSize-len(t.OccupiedLeaves))
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
	padding := make([]fr.Element, newSize-len(t.OccupiedNodes[level-1]))
	for i := range padding {
		padding[i] = t.EmptyNodes[level-1]
	}
	t.OccupiedNodes[level-1] = append(t.OccupiedNodes[level-1], padding...)
}

// BuildComplete builds a Merkle-tree. Requires that the
// input leaves are powers of 2.
func BuildComplete(leaves []fr.Element) *Tree {

	numLeaves := len(leaves)

	if !utils.IsPowerOfTwo(numLeaves) || numLeaves == 0 {
		utils.Panic("expected power of two number of leaves, got %v", numLeaves)
	}

	depth := utils.Log2Ceil(numLeaves)
	tree := NewEmptyTree(depth)
	tree.OccupiedLeaves = leaves
	currLevels := leaves

	for i := 0; i < depth-1; i++ {
		nextLevel := make([]fr.Element, len(currLevels)/2)
		// TODO @gbotrel revisit parallelization here
		if len(nextLevel) >= 64 {
			parallel.Execute(len(nextLevel), func(start, end int) {
				for k := start; k < end; k++ {
					nextLevel[k] = hashLR(currLevels[2*k], currLevels[2*k+1])
				}
			})
		} else {
			for k := range nextLevel {
				nextLevel[k] = hashLR(currLevels[2*k], currLevels[2*k+1])
			}
		}

		tree.OccupiedNodes[i] = nextLevel
		currLevels = nextLevel
	}

	// sanity-check : len(currLevels) == 2 is an invariant
	if len(currLevels) != 2 {
		utils.Panic("broken invariant : len(currLevels) != 2, =%v", len(currLevels))
	}

	// And overwrite the root
	tree.Root = hashLR(currLevels[0], currLevels[1])
	return tree
}
