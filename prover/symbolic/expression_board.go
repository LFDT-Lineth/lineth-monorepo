package symbolic

import "github.com/consensys/linea-monorepo/prover/maths/field/fext"

/*
ExpressionBoard is a shared space for defining expressions. Several expressions
can use the same board. Ideally, all expressions uses the same common board.

Contrary to [Expression] the board maintains a topological ordering for the
anchored expressions where the sub-expressions are de-duplicated.
*/
type ExpressionBoard struct {
	// Topologically sorted list of (deduplicated) nodes
	// The nodes are sorted by dependency: a node at index i only depends on nodes at indices j < i.
	Nodes []Node
	// Maps nodes to their index in the Nodes slice.
	ESHashesToPos map[esHash]nodeID

	// Compiled program
	Bytecode          []int
	Constants         []fext.Element
	NumSlots          int
	ResultSlot        int
	ProgramNodesCount int
}

// BytecodeStats holds counts of each opcode kind in a compiled board.
type BytecodeStats struct {
	Const, Input, Mul, LinComb, PolyEval int
}

// BytecodeStats walks the compiled bytecode and returns per-opcode counts.
// Panics if the board has not been compiled yet.
func (b *ExpressionBoard) BytecodeStats() BytecodeStats {
	var s BytecodeStats
	pc := 0
	for pc < len(b.Bytecode) {
		switch opCode(b.Bytecode[pc]) {
		case opLoadConst:
			s.Const++
			pc += 3
		case opLoadInput:
			s.Input++
			pc += 3
		case opMul:
			n := b.Bytecode[pc+2]
			s.Mul++
			pc += 3 + n*2
		case opLinComb:
			n := b.Bytecode[pc+2]
			s.LinComb++
			pc += 3 + n*2
		case opPolyEval:
			n := b.Bytecode[pc+2]
			s.PolyEval++
			pc += 3 + n
		default:
			panic("unknown opcode in bytecode")
		}
	}
	return s
}

// emptyBoard initializes a board with no Node in it.
func emptyBoard() ExpressionBoard {
	return ExpressionBoard{
		Nodes:         []Node{},
		ESHashesToPos: map[esHash]nodeID{},
	}
}

/*
nodeID represents the position of a Node in the ExpressionBoard.
*/
type nodeID uint32

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
