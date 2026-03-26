package symbolic

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sync"

	"github.com/consensys/gnark/frontend"
	sv "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/collection"
)

type esHash = field.Element

// Expression represents a symbolic arithmetic expression. Expression can be
// built using the [Add], [Mul], [Sub] etc... package. However, they can only
// represent polynomial expressions: inversion is not supported.
//
// Expressions is structured like a tree where each node performs an elementary
// operation.
//
// Expression cannot be directly evaluated with an assignment. To do that, the
// expression must be first converted into a [BoardedExpression] using the
// [Expression.Board] method.
type Expression struct {
	// ESHash stands for Expression Sensitive Hash and is an identifier for
	// the expression. Specifically, it represents what the expression computes
	// rather than how the expression is structured.
	//
	// For instance, the expressions "a * 2" and "a + a" have the same ESHash,
	// the user should not modify this value directly.
	//
	// It is computed with the following rules:
	//
	//		- ESHash(a + b) = ESHash(a) + ESHash(b)
	// 		- ESHash(a * b) = ESHash(a) * ESHash(b)
	// 		- ESHash(c: Constant) = c
	// 		- ESHash(a: variable) = H(a.String())
	ESHash esHash
	// Children stores the list of all the sub-expressions the current
	// Expression uses as operands.
	Children []*Expression
	// Operator stores information relative to operation that the current
	// Expression performs on its inputs.
	Operator Operator
}

// Operator specifies an elementary operation a node of an [Expression] performs
// (Add, Mul, Sub ...)
type Operator interface {
	// Evaluate returns an evaluation of the operator from a list of assignments:
	// one for each operand (children) of the expression.
	Evaluate([]sv.SmartVector) sv.SmartVector
	// Validate performs a sanity-check of the expression the Operator belongs
	// to.
	Validate(e *Expression) error
	// Returns the polynomial degree of the expression.
	Degree([]int) int
	// GnarkEval returns an evaluation of the operator in a gnark circuit.
	GnarkEval(frontend.API, []frontend.Variable) frontend.Variable
}

// Board pins down the expression into an ExpressionBoard. This converts the
// Expression into a DAG and runs a topological sorting algorithm over the
// nodes of the expression. This has the effect of removing the duplicates
// nodes and making the expression more efficient to evaluate.
func (e *Expression) Board() ExpressionBoard {
	board := emptyBoard()
	e.anchor(&board)
	board.EsHashesToPos = nil // free the map, we don't need it anymore
	return board
}

func (e *Expression) BoardListVariableMetadata() []Metadata {
	// no need to build the whole board here, we simply walk the graph
	// and collect variables as they appear
	uniqueVars := map[string]struct{}{}
	metadatas := []Metadata{}
	var collectVars func(expr *Expression)
	collectVars = func(expr *Expression) {
		switch op := expr.Operator.(type) {
		case Variable:
			if _, seen := uniqueVars[op.Metadata.String()]; seen {
				return
			}
			uniqueVars[op.Metadata.String()] = struct{}{}
			metadatas = append(metadatas, op.Metadata)
		default:
			for _, c := range expr.Children {
				collectVars(c)
			}
		}
	}
	collectVars(e)
	return metadatas
}

// anchor pins down the Expression onto an ExpressionBoard.
func (e *Expression) anchor(b *ExpressionBoard) nodeID {
	// recursion base case: the node was already anchored
	if id, ok := b.EsHashesToPos[e.ESHash]; ok {
		return id
	}

	// anchor recursively all the children
	maxChildrenLevel := 0
	childrenIDs := []nodeID{}
	for _, child := range e.Children {
		childID := child.anchor(b)
		maxChildrenLevel = utils.Max(maxChildrenLevel, childID.level())
		childrenIDs = append(childrenIDs, childID)
	}

	newLevel := maxChildrenLevel + 1
	if len(e.Children) == 0 {
		// Then it is a leaf node and the level should be 0
		newLevel = 0
	}

	/*
		Computes the new ID of the node by adding a new one into the list.
		We extend the outer-list of Nodes if necessary.
	*/
	if len(b.Nodes) <= newLevel {
		b.Nodes = append(b.Nodes, []Node{})
	}
	id := newNodeID(newLevel, len(b.Nodes[newLevel]))
	newNode := Node{
		ESHash:   e.ESHash,
		Children: childrenIDs,
		Operator: e.Operator,
	}

	/*
		Registers this NewNodeID in the children as a parents and in the
		`ESHashToPosition` registry and in the children as a new `Parent`.
		Also, add it to the list of nodes at the position corresponding to
		its new NodeID
	*/
	b.EsHashesToPos[e.ESHash] = id
	b.Nodes[id.level()] = append(b.Nodes[id.level()], newNode)

	return id
}

// Validate operates a list of sanity-checks over the current expression to
// assess its well-formedness. It returns an error if the check fails.
func (e *Expression) Validate() error {

	eshashes := make([]sv.SmartVector, len(e.Children))
	for i := range e.Children {
		eshashes[i] = sv.NewConstant(e.Children[i].ESHash, 1)
	}

	if len(e.Children) > 0 {
		// The cast back to sv.Constant is not functionally important but is an
		// easy sanity check.
		expectedESH := e.Operator.Evaluate(eshashes).(*sv.Constant).Get(0)
		if expectedESH != e.ESHash {
			return fmt.Errorf("esh mismatch %v %v", expectedESH.String(), e.ESHash.String())
		}
	}

	// Operator specific validation
	if err := e.Operator.Validate(e); err != nil {
		return err
	}

	// Validate the children recursively
	for i := range e.Children {
		if err := e.Children[i].Validate(); err != nil {
			return err
		}
	}

	return nil
}

// Same as [Expression.Validate] but panics on error.
func (e *Expression) AssertValid() {
	if err := e.Validate(); err != nil {
		panic(err)
	}
}

// Replay constructs an analogous expression. Replacing all the [Variable] by
// those given in the translation map.
func (e *Expression) Replay(translationMap collection.Mapping[string, *Expression]) (res *Expression) {

	switch op := e.Operator.(type) {
	case Constant:
		return e
	case Variable:
		res, ok := translationMap.TryGet(op.Metadata.String())
		if !ok {
			return e // return itself if not found
		}
		return res
	case LinComb:
		children := make([]*Expression, len(e.Children))
		allSame := true
		for i, c := range e.Children {
			children[i] = c.Replay(translationMap)
			if children[i] != c {
				allSame = false
			}
		}
		if allSame {
			return e // return itself if all children are the same
		}
		res := NewLinComb(children, op.Coeffs)
		return res
	case Product:
		children := make([]*Expression, len(e.Children))
		allSame := true
		for i, c := range e.Children {
			children[i] = c.Replay(translationMap)
			if children[i] != c {
				allSame = false
			}
		}
		if allSame {
			return e // return itself if all children are the same
		}
		res := NewProduct(children, op.Exponents)
		return res
	// PolyEval
	case PolyEval:
		children := make([]*Expression, len(e.Children))
		allSame := true
		for i, c := range e.Children {
			children[i] = c.Replay(translationMap)
			if children[i] != c {
				allSame = false
			}
		}
		if allSame {
			return e // return itself if all children are the same
		}
		res := NewPolyEval(children[0], children[1:])
		return res
	}
	panic("unreachable")
}

// ReconstructBottomUp applies the constructor function from the bottom-up of
// the current expression. It can be used to construct a new expression from
// `expr`. This is useful for symbolic expression simplication routines etc...
// The constructor function must not modify the value of the input expression
// nor it children.
func (e *Expression) ReconstructBottomUp(
	constructor func(e *Expression, children []*Expression) (new *Expression),
) *Expression {

	switch e.Operator.(type) {
	// Constant, indicating we reached the bottom of the expression. Thus it
	// applies the mutator and returns.
	case Constant, Variable:
		return constructor(e, []*Expression{})
	// LinComb or Product or PolyEval. This is an intermediate expression.
	case LinComb, Product, PolyEval:
		children := make([]*Expression, len(e.Children))
		var wg sync.WaitGroup
		wg.Add(len(e.Children))

		for i, c := range e.Children {
			go func(i int, c *Expression) {
				defer wg.Done()
				children[i] = c.ReconstructBottomUp(constructor)
			}(i, c)
		}

		wg.Wait()
		return constructor(e, children)
	}

	panic("unreachable")
}

// ReconstructBottomUpSingleThreaded is the same as [Expression.ReconstructBottomUp]
// but it is single threaded.
func (e *Expression) ReconstructBottomUpSingleThreaded(
	constructor func(e *Expression, children []*Expression) (new *Expression),
) *Expression {

	switch e.Operator.(type) {
	// Constant, indicating we reached the bottom of the expression. Thus it
	// applies the mutator and returns.
	case Constant, Variable:
		x := constructor(e, []*Expression{})
		if x == nil {
			panic(x)
		}
		return x
	// LinComb or Product or PolyEval. This is an intermediate expression.
	case LinComb, Product, PolyEval:
		children := make([]*Expression, len(e.Children))
		for i, c := range e.Children {
			children[i] = c.ReconstructBottomUp(constructor)
			if children[i] == nil {
				panic(children[i])
			}
		}
		x := constructor(e, children)
		if x == nil {
			panic(x)
		}
		return x
	}

	panic("unreachable")
}

// SameWithNewChildren constructs a new expression that is a copy-cat of the
// receiver expression but swapping the children with the new ones instead. It
// is common for rebuilding expressions. If the expression is a variable or a
// constant, it returns itself.
func (e *Expression) SameWithNewChildren(newChildren []*Expression) *Expression {

	switch op := e.Operator.(type) {
	// Constant
	case Constant, Variable:
		return e
	// LinComb
	case LinComb:
		return NewLinComb(newChildren, op.Coeffs)
	// Product
	case Product:
		return NewProduct(newChildren, op.Exponents)
	// PolyEval
	case PolyEval:
		return NewPolyEval(newChildren[0], newChildren[1:])
	default:
		panic("unexpected type: " + reflect.TypeOf(op).String())
	}
}

// MarshalJSONString returns a JSON string returns a JSON string representation
// of the expression.
func (e *Expression) MarshalJSONString() string {
	js, jsErr := json.MarshalIndent(e, "", "  ")
	if jsErr != nil {
		utils.Panic("failed to marshal expression: %v", jsErr)
	}
	return string(js)
}
