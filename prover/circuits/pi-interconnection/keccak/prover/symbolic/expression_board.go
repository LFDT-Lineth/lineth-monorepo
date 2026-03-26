package symbolic

import (
	"sort"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

/*
ExpressionBoard is a shared space for defining expressions. Several expressions
can use the same board. Ideally, all expressions uses the same common board.

Contrary to [Expression] the board maintains a topological ordering for the
anchored expressions where the sub-expressions are de-duplicated.
*/
type ExpressionBoard struct {
	// Topologically sorted list of (deduplicated) nodes
	// The nodes are sorted by level : e.g the first entry corresponds to
	// the leaves and the last one (which should be unique). To the root.
	Nodes [][]Node
	// Maps nodes to their level in the DAG structure. The 32 MSB bits
	// of the ID indicates the level and the LSB bits indicates the position
	// in the level.
	EsHashesToPos map[field.Element]nodeID
}

// emptyBoard initializes a board with no Node in it.
func emptyBoard() ExpressionBoard {
	return ExpressionBoard{
		Nodes:         [][]Node{},
		EsHashesToPos: map[field.Element]nodeID{},
	}
}

/*
nodeID represents the position of a Node in the ExpressionBoard. It consists of
two subIDs:
  - the 32 MSB represents the level of the node in the DAG
  - the 32 LSB represents the index of the node in its level

The two together gives the position of the node = expression.Nodes[level][pos]
*/
type nodeID uint64

/*
Node consists of an operator and a list of operands and is meant to be stored
in an [ExpressionBoard]. Additionally; it gather "DAG-wise" data such as the
parents of the Node etc...
*/
type Node struct {
	/*
		ESHash (Expression Sensitive Hash) is a field.Element representing an expression.
		Two expression with the same ESHash are considered as equals. This helps expression
		pruning.
	*/
	ESHash esHash

	// Indicates the IDs of the parents of the current node; corresponding to
	// the nodes representing admitting this node as an input.
	// Parents  []nodeID
	Children []nodeID

	// Operator contains the logic to evaluate an expression
	Operator Operator
}

// posInLevel returns the position in the level from a NodeID
func (i nodeID) posInLevel() int {
	res := i & ((1 << 32) - 1)
	return utils.ToInt(res)
}

// level returns the level from a NodeID
func (i nodeID) level() int {
	return utils.ToInt(i >> 32)
}

// newNodeID returns the node id given its level and its position in a level
func newNodeID(level, posInLevel int) nodeID {
	if level >= 1<<32 {
		utils.Panic("Level is too large %v", level)
	}
	if posInLevel >= 1<<32 {
		utils.Panic("Pos in level is too large %v", posInLevel)
	}
	return nodeID(uint64(level)<<32 | uint64(posInLevel))
}

// SortExpression the children list of every node by increasing ESHash and goes
// recursively.
func SortChildren(e *Expression) {

	// Recursively call the function for the children if applicable
	for i := range e.Children {
		SortChildren(e.Children[i])
	}

	if len(e.Children) < 2 {
		return
	}

	switch op := e.Operator.(type) {
	case LinComb:

		sorter := utils.GenSorter{
			LenFn: func() int { return len(e.Children) },
			SwapFn: func(i, j int) {
				e.Children[i], e.Children[j] = e.Children[j], e.Children[i]
				op.Coeffs[i], op.Coeffs[j] = op.Coeffs[j], op.Coeffs[i]
			},
			LessFn: func(i, j int) bool {
				return e.Children[i].ESHash.Cmp(&e.Children[j].ESHash) < 0
			},
		}

		sort.Sort(sorter)
		return

	case Product:

		sorter := utils.GenSorter{
			LenFn: func() int { return len(e.Children) },
			SwapFn: func(i, j int) {
				e.Children[i], e.Children[j] = e.Children[j], e.Children[i]
				op.Exponents[i], op.Exponents[j] = op.Exponents[j], op.Exponents[i]
			},
			LessFn: func(i, j int) bool {
				return e.Children[i].ESHash.Cmp(&e.Children[j].ESHash) < 0
			},
		}

		sort.Sort(sorter)
		return
	}
}
