package symbolic

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/consensys/gnark/frontend"
	sv "github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/collection"
)

type esHash = fext.Element

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
	IsBase   bool
	// cachedDegree caches the value of the degree of the expression.
	cachedDegree *int `serde:"omit"`
}

// Operator specifies an elementary operation a node of an [Expression] performs
// (Add, Mul, Sub ...)
type Operator interface {
	// Evaluate returns an evaluation of the operator from a list of assignments:
	// one for each operand (children) of the expression.
	Evaluate([]sv.SmartVector) sv.SmartVector
	// EvaluateExt returns an evaluation of the operator from a list of assignments:
	// one for each operand (children) of the expression.
	EvaluateExt([]sv.SmartVector) sv.SmartVector
	EvaluateMixed([]sv.SmartVector) sv.SmartVector
	// Validate performs a sanity-check of the expression the Operator belongs
	// to.
	Validate(e *Expression) error
	// Returns the polynomial degree of the expression.
	Degree([]int) int
	// GnarkEval returns an evaluation of the operator in a gnark circuit.
	GnarkEval(frontend.API, []koalagnark.Element) koalagnark.Element
	GnarkEvalExt(frontend.API, []any) koalagnark.Ext
}

type OperatorWithResult interface {
	EvaluateExtResult(result sv.SmartVector, inputs []sv.SmartVector)
}

// Board pins down the expression into an ExpressionBoard. This converts the
// Expression into a DAG and runs a topological sorting algorithm over the
// nodes of the expression. This has the effect of removing the duplicates
// nodes and making the expression more efficient to evaluate.
func (e *Expression) Board() ExpressionBoard {
	board := emptyBoard()
	e.anchor(&board)
	return board
}

// ListBoardVariableMetadata returns the list of metadata of all the variables
// This is equivalent to calling Board() and ListVariableMetadata on the resulting
// ExpressionBoard, but avoid allocating the full board.
func (e *Expression) ListBoardVariableMetadata() []Metadata {
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
	// recursion base case: the expression is already anchored
	if nodeID, ok := b.ESHashesToPos[e.ESHash]; ok {
		return nodeID
	}

	/*
		Recurse the call in all the children to ensure all
		subexpressions are anchored.
	*/
	childrenIDs := make([]nodeID, 0, len(e.Children))
	for _, child := range e.Children {
		childID := child.anchor(b)
		childrenIDs = append(childrenIDs, childID)
	}

	/*
		Computes the new ID of the node by adding a new one into the list.
	*/
	id := nodeID(len(b.Nodes))
	newNode := Node{
		ESHash:   e.ESHash,
		Children: childrenIDs,
		Operator: e.Operator,
	}

	b.ESHashesToPos[e.ESHash] = id
	b.Nodes = append(b.Nodes, newNode)

	return id
}

// Validate operates a list of sanity-checks over the current expression to
// assess its well-formedness. It returns an error if the check fails.
func (e *Expression) Validate() error {
	eshashes := make([]sv.SmartVector, len(e.Children))
	for i := range e.Children {
		eshashes[i] = sv.NewConstantExt(e.Children[i].ESHash, 1)
	}

	if len(e.Children) > 0 {
		// The cast back to sv.Constant is not functionally important but is an
		// easy sanity check.
		expectedESH := e.Operator.EvaluateExt(eshashes).(*sv.ConstantExt).GetExt(0)

		if !e.ESHash.Equal(&expectedESH) {
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
	// LinCombExt or ProductExt or LinearCombinationExt. This is an intermediate expression.
	case LinComb, Product, PolyEval:
		children := make([]*Expression, len(e.Children))
		// var wg sync.WaitGroup
		// wg.Add(len(e.Children))

		for i, c := range e.Children {
			// TODO @gbotrel next fix that --> too many go routines.
			// go func(i int, c *Expression) {
			// 	defer wg.Done()
			children[i] = c.ReconstructBottomUp(constructor)
			// }(i, c)
		}

		// wg.Wait()
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
	case Constant, Variable:
		return e
	case LinComb:
		return NewLinComb(newChildren, op.Coeffs)
	case Product:
		return NewProduct(newChildren, op.Exponents)
	case PolyEval:
		return NewPolyEval(newChildren[0], newChildren[1:])
	default:
		panic("unexpected type: " + reflect.TypeOf(op).String())
	}
}

// SameWithNewChildrenNoSimplify is the same as [Expression.SameWithNewChildren]
// but it does not simplify the expression.
func (e *Expression) SameWithNewChildrenNoSimplify(newChildren []*Expression) *Expression {
	switch op := e.Operator.(type) {
	case Constant, Variable:
		return e
	case LinComb:
		return newLinCombNoSimplify(newChildren, op.Coeffs)
	case Product:
		return newProductNoSimplify(newChildren, op.Exponents)
	case PolyEval:
		return NewPolyEval(newChildren[0], newChildren[1:])
	default:
		panic("unexpected type: " + reflect.TypeOf(op).String())
	}
}

// Degree returns the degree of the expression
func (e *Expression) Degree(getDegree GetDegree) int {
	r := e.degreeKeepCache(getDegree)
	e.uncacheDegree()
	return r
}

// degreeKeepCache recursively performs the degree computation of the expression
// and caches its result. However, it does not uncache the degree of its children.
func (e *Expression) degreeKeepCache(getDegree GetDegree) int {

	if e.cachedDegree != nil {
		return *e.cachedDegree
	}

	switch v := e.Operator.(type) {
	case Constant:
		return 0
	case Variable:
		d := getDegree(v.Metadata)
		e.cachedDegree = &d
		return d
	default:
		childrenDeg := make([]int, len(e.Children))
		for k, child := range e.Children {
			d := child.degreeKeepCache(getDegree)
			childrenDeg[k] = d
		}
		d := e.Operator.Degree(childrenDeg)
		e.cachedDegree = &d
		return d
	}
}

// uncacheDegree un-caches the degree of the expression
func (e *Expression) uncacheDegree() {
	for _, child := range e.Children {
		child.uncacheDegree()
	}
	e.cachedDegree = nil
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

// computeIsBaseFromChildren determines if an expression is a base expression
// based on its children.
func computeIsBaseFromChildren(children []*Expression) bool {
	for _, child := range children {
		if !child.IsBase {
			// at least one child is not a base expression, therefore the expression is itself not a base expression
			return false
		}
	}
	// all children are base expressions, therefore the expression is itself a base expression
	return true
}
