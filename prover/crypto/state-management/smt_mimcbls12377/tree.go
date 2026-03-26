package smt

import (
	"fmt"
	"runtime"
	"sync"

	hashtypes "github.com/consensys/linea-monorepo/prover/crypto/state-management/hashtypes_legacy"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/types"
)

// Config specifies the parameters of the tree (choice of hash function, depth).
type Config struct {
	// HashFunc is a function returning initialized hashers
	HashFunc func() hashtypes.Hasher
	// Depth is the depth of the tree
	Depth int
}

// Tree represents a binary sparse Merkle-tree (SMT).
type Tree struct {
	// Config of the hash function
	Config *Config
	// Root stores the root of the tree
	Root types.Bls12377Fr
	// OccupiedLeaves continuously list of the occupied leaves. For the toy
	// implementation we track all the leaves.
	OccupiedLeaves []types.Bls12377Fr
	// Occupied not stores all the node with a non-trivial value in the tree.
	//
	// Does not include the root. (So there are 39 levels and not 40).
	// Returns a node at a given level:
	// 	- 0 is the level just above the leaves
	// 	- 38 is the level just below the root
	OccupiedNodes [][]types.Bls12377Fr
	// EmptyNodes stores the value of the trivial nodes of the SMT (i.e the one
	// corresponding to empty sub-trees).
	//
	// It does not include the "empty root" nor the empty leaf
	// so the first position contains the empty node for the level one.
	// So there are 39, and not 40 levels. That way, the indexing stays
	// consistent with "OccupiedNode"
	EmptyNodes []types.Bls12377Fr
}

// EmptyLeaf returns an empty leaf (e.g. the zero bytes value).
func EmptyLeaf() types.Bls12377Fr {
	return types.Bls12377Fr{}
}

// hashLR is used for hashing the leaf-right children. It returns H(nodeL, nodeR)
// taking H as the HashFunc of the config.
func hashLR(hasher hashtypes.Hasher, nodeL, nodeR types.Bls12377Fr) types.Bls12377Fr {
	hasher.Reset()
	nodeL.WriteTo(hasher)
	nodeR.WriteTo(hasher)
	digest := hasher.Sum(nil)
	d := types.AsBls12377Fr(digest)
	return d
}

// NewEmptyTree creates and returns an empty tree with the provided config.
func NewEmptyTree(conf *Config) *Tree {
	// Computes the empty nodes
	emptyNodes := make([]types.Bls12377Fr, conf.Depth-1)
	prevNode := EmptyLeaf()

	hasher := conf.HashFunc()

	for i := range emptyNodes {
		newNode := hashLR(hasher, prevNode, prevNode)
		emptyNodes[i] = newNode
		prevNode = newNode
	}

	// Stores the initial root separately
	root := hashLR(hasher, prevNode, prevNode)

	return &Tree{
		Config:         conf,
		Root:           root,
		OccupiedLeaves: make([]types.Bls12377Fr, 0),
		OccupiedNodes:  make([][]types.Bls12377Fr, conf.Depth-1),
		EmptyNodes:     emptyNodes,
	}
}

// GetLeaf returns a leaf by position or an error if the leaf is out of bounds.
func (t *Tree) GetLeaf(pos int) (types.Bls12377Fr, error) {
	// Check that the accessed node is within the bounds of the SMT
	maxPos := 1 << t.Config.Depth
	if pos >= maxPos {
		return types.Bls12377Fr{}, fmt.Errorf("out of bound: %v", pos)
	}

	if pos < 0 {
		return types.Bls12377Fr{}, fmt.Errorf("negative position: %v", pos)
	}
	// Check if this is an empty leaf
	if pos >= len(t.OccupiedLeaves) {
		return EmptyLeaf(), nil
	}
	// Return the leaf if occupied
	return t.OccupiedLeaves[pos], nil
}

// MustGetLeaf is as [Tree.GetLeaf] but panics on errors.
func (t *Tree) MustGetLeaf(pos int) types.Bls12377Fr {
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
func (t *Tree) getNode(level, posInLevel int) types.Bls12377Fr {
	switch {
	case level == t.Config.Depth:
		// The only logical posInLevels value is zero in this case
		if posInLevel > 0 {
			utils.Panic("there is only one root but posInLevel was %v", posInLevel)
		}
		// Opportunistic sanity-checks. Parenthesis are necessary because
		// of a hole in my linter.
		if t.Root == (types.Bls12377Fr{}) {
			utils.Panic("sanity-check failed : the root is zero.")
		}
		return t.Root
	case level >= 1 && level <= t.Config.Depth-1:
		// Check that the accessed node is within the bounds of the SMT
		maxPos := 1 << (t.Config.Depth - level)
		if posInLevel >= maxPos {
			utils.Panic("nodeID is out of bound")
		}
		// Check if this is an empty node
		if posInLevel >= len(t.OccupiedNodes[level-1]) {
			return t.EmptyNodes[level-1]
		}
		// Or return an non-empty one
		res := t.OccupiedNodes[level-1][posInLevel]
		if res == (types.Bls12377Fr{}) {
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
func (t *Tree) updateNode(level, posInLevel int, newVal types.Bls12377Fr) {
	switch {
	case level == t.Config.Depth:
		// The only logical posInLevels value is zero in this case
		if posInLevel > 0 {
			utils.Panic("there is only one root but posInLevel was %v", posInLevel)
		}
		// Opportunistic sanity-checks. Parenthesis are necessary because
		// of a hole in my linter.
		if t.Root == (types.Bls12377Fr{}) {
			utils.Panic("sanity-check failed : the root is zero.")
		}
		t.Root = newVal
	case level >= 1 && level < t.Config.Depth:
		// Check that the accessed node is within the bounds of the SMT
		maxPos := 1 << (t.Config.Depth - level)
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
		maxPos := 1 << t.Config.Depth
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
	if level > t.Config.Depth {
		utils.Panic("level out of bound %v", level)
	}

	// Edge : case root level
	if level == t.Config.Depth {
		utils.Panic("tried extending the root level %v", newSize)
	}

	// Will work for both the leaves and the intermediate nodes
	if newSize > 1<<(t.Config.Depth-level) {
		utils.Panic("overextending the tree %v, %v", level, newSize)
	}

	if level == 0 {
		if newSize <= len(t.OccupiedLeaves) {
			// already big enough
			return
		}
		// else, we add extra "empty leaves" at the end of the slice
		padding := make([]types.Bls12377Fr, newSize-len(t.OccupiedLeaves))
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
	padding := make([]types.Bls12377Fr, newSize-len(t.OccupiedNodes[level-1]))
	for i := range padding {
		padding[i] = t.EmptyNodes[level-1]
	}
	t.OccupiedNodes[level-1] = append(t.OccupiedNodes[level-1], padding...)
}

// BuildComplete builds from scratch a complete Merkle-tree. Requires that the
// input leaves are powers of 2. The depth of the tree is deduced from the list.
//
// It panics if the number of leaves is a non-power of 2.
func BuildComplete(leaves []types.Bls12377Fr, hashFunc func() hashtypes.Hasher) *Tree {
	// TODO @gbotrel this accumulates to 50sec of runtime for proof creation
	// plenty of innefficenies to optimize for.
	// it also stores a lot of data in memory, investigate why.

	numLeaves := len(leaves)

	// Sanity check : there should be a power of two number of leaves
	if !utils.IsPowerOfTwo(numLeaves) || numLeaves == 0 {
		utils.Panic("expected power of two number of leaves, got %v", numLeaves)
	}

	depth := utils.Log2Ceil(numLeaves)
	config := &Config{HashFunc: hashFunc, Depth: depth}

	// Builds an empty tree and passes the leaves
	tree := NewEmptyTree(config)
	tree.OccupiedLeaves = leaves

	// pre-allocate the memory for all levels
	nbTotalNodes := 0
	for d := 1; d < depth; d++ {
		nbTotalNodes += 1 << (depth - d)
	}
	arena := make([]types.Bls12377Fr, nbTotalNodes)
	offset := 0

	// start a worker pool that will parallelize the hashing at each level
	type workerTask struct {
		curr  []types.Bls12377Fr
		next  []types.Bls12377Fr
		start int
		stop  int
		wg    *sync.WaitGroup
	}

	workerCount := runtime.GOMAXPROCS(0)
	if workerCount < 1 {
		workerCount = 1
	}

	tasks := make(chan workerTask)
	var workersWG sync.WaitGroup
	workersWG.Add(workerCount)
	for i := 0; i < workerCount; i++ {
		go func() {
			hasher := config.HashFunc()
			for task := range tasks {
				for k := task.start; k < task.stop; k++ {
					task.next[k] = hashLR(hasher, task.curr[2*k], task.curr[2*k+1])
				}
				task.wg.Done()
			}
			workersWG.Done()
		}()
	}

	// build the tree bottom up
	currLevels := leaves
	for d := 1; d < depth; d++ {
		levelSize := 1 << (depth - d)
		nextLevel := arena[offset : offset+levelSize]

		activeWorkers := workerCount
		if levelSize < activeWorkers {
			activeWorkers = levelSize
		}

		var levelWG sync.WaitGroup
		levelWG.Add(activeWorkers)
		for i := 0; i < activeWorkers; i++ {
			start := i * levelSize / activeWorkers
			stop := (i + 1) * levelSize / activeWorkers
			tasks <- workerTask{
				curr:  currLevels,
				next:  nextLevel,
				start: start,
				stop:  stop,
				wg:    &levelWG,
			}
		}
		levelWG.Wait()

		tree.OccupiedNodes[d-1] = nextLevel
		currLevels = nextLevel
		offset += levelSize
	}

	close(tasks)
	workersWG.Wait()

	// sanity-check : len(currLevels) == 2 is an invariant
	if len(currLevels) != 2 {
		utils.Panic("broken invariant : len(currLevels) != 2, =%v", len(currLevels))
	}

	// And overwrite the root
	hasher := config.HashFunc()
	tree.Root = hashLR(hasher, currLevels[0], currLevels[1])
	return tree
}
