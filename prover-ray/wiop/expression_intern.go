package wiop

import "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"

// internKey identifies an [Expression] node by the value it denotes within a
// module. Operands are named by their canonical pointers, which interning
// bottom-up makes sufficient. Only the fields relevant to kind are set; b is
// nil for unary operations.
type internKey struct {
	kind     internKind
	column   ObjectID // ColumnView and ColumnPosition
	offset   int      // shift of a ColumnView, row of a ColumnPosition
	constant field.Element
	module   *Module // nil for a scalar Constant
	operator ArithmeticOperator
	a, b     Expression // operands; for internOpaque, a is the node itself
}

type internKind uint8

const (
	internColumnView internKind = iota
	internColumnPosition
	internConstant
	internArithmeticOp
	// internOpaque keys a node by pointer: cells, coins and Lagrange
	// selectors have identity of their own, and any other type is never
	// merged.
	internOpaque
)

// exprInterner is the structural interning cache of one module's vanishings
// (see [Module.newVanishing]): each value is carried by a single pointer
// across all of them, whichever compiler built them. Sharing is sound because
// a node is a pure function of what it reads and nothing mutates an
// expression in place. The maps are never iterated, so they do not affect
// output order.
type exprInterner struct {
	nodes map[internKey]Expression // key -> canonical node
	// canonical maps every node seen, canonical or not, to its canonical
	// node, so each distinct pointer is resolved once.
	canonical map[Expression]Expression
}

func newExprInterner() *exprInterner {
	return &exprInterner{
		nodes:     map[internKey]Expression{},
		canonical: map[Expression]Expression{},
	}
}

// intern returns the canonical node for expr without modifying expr: an
// [ArithmeticOperation] whose operands are not canonical is rebuilt on the
// canonical ones, unless an equal node already exists. An already canonical
// expression is returned as is.
func (in *exprInterner) intern(expr Expression) Expression {
	if c, ok := in.canonical[expr]; ok {
		return c
	}
	node, key := expr, internKey{kind: internOpaque, a: expr}
	switch e := expr.(type) {
	case *ColumnView:
		key = internKey{kind: internColumnView, column: e.Column.Context.ID, offset: e.ShiftingOffset}
	case *ColumnPosition:
		key = internKey{kind: internColumnPosition, column: e.Column.Context.ID, offset: e.Position}
	case *Constant:
		// A vector and a scalar constant stay apart: they differ in
		// multi-valuedness.
		key = internKey{kind: internConstant, constant: e.Value, module: e.module}
	case *ArithmeticOperation:
		operands := make([]Expression, len(e.Operands))
		changed := false
		for i, o := range e.Operands {
			operands[i] = in.intern(o)
			changed = changed || operands[i] != o
		}
		key = internKey{kind: internArithmeticOp, operator: e.Operator, a: operands[0]}
		if len(operands) > 1 {
			key.b = operands[1]
		}
		if _, ok := in.nodes[key]; !ok && changed {
			node = NewArithmeticOperation(e.Operator, operands...)
		}
	}
	c, ok := in.nodes[key]
	if !ok {
		c = node
		in.nodes[key] = c
		in.canonical[c] = c
	}
	in.canonical[expr] = c
	return c
}
