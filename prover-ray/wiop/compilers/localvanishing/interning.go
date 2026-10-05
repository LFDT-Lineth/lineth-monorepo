package localvanishing

import (
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// nodeKey identifies a lifted [wiop.Expression] node by the value it denotes,
// so two nodes carrying the same key are interchangeable. It is comparable, so
// it is a map key directly.
//
// Operands are named by their *interned* pointers: interning runs bottom-up,
// so two structurally equal operand subtrees have already collapsed onto one
// pointer by the time their parent is keyed.
//
// Only the fields relevant to `kind` are set. `b` is nil for unary operations,
// which keeps a unary node distinct from a binary one on the same operand.
type nodeKey struct {
	kind nodeKind
	// column and offset describe a ColumnView (offset = shift) or a
	// ColumnPosition (offset = row). Column IDs are system-unique.
	column wiop.ObjectID
	offset int
	// constant and module describe a Constant; module is nil for a scalar one.
	constant field.Element
	module   *wiop.Module
	// operator, a and b describe an ArithmeticOperation. For kindOpaque, a
	// holds the node itself, so it only ever matches by pointer identity.
	operator wiop.ArithmeticOperator
	a, b     wiop.Expression
}

// nodeKind tags which variant of [nodeKey] is populated.
type nodeKind uint8

const (
	kindColumnView nodeKind = iota
	kindColumnPosition
	kindConstant
	kindArithmeticOp
	// kindOpaque covers leaves with identity of their own — [wiop.Cell],
	// [wiop.CoinField], [wiop.LagrangeSelector] — and any other
	// implementation: they are keyed by pointer, which never merges two nodes
	// that could differ.
	kindOpaque
)

// keyOf derives the [nodeKey] of expr. Its operands, when it has any, must
// already be interned.
func keyOf(expr wiop.Expression) nodeKey {
	switch e := expr.(type) {
	case *wiop.ColumnView:
		return nodeKey{kind: kindColumnView, column: e.Column.Context.ID, offset: e.ShiftingOffset}
	case *wiop.ColumnPosition:
		return nodeKey{kind: kindColumnPosition, column: e.Column.Context.ID, offset: e.Position}
	case *wiop.Constant:
		return nodeKey{kind: kindConstant, constant: e.Value, module: e.Module()}
	case *wiop.ArithmeticOperation:
		k := nodeKey{kind: kindArithmeticOp, operator: e.Operator, a: e.Operands[0]}
		if len(e.Operands) > 1 {
			k.b = e.Operands[1]
		}
		return k
	default:
		return nodeKey{kind: kindOpaque, a: expr}
	}
}

// interner is a per-module structural interning cache over expression nodes.
//
// It is seeded with every node of the module's already-registered multi-valued
// vanishings, so a lifted subtree that denotes a value some global constraint
// already carries reuses that constraint's pointer. Lifted trees are
// anchor-relative, so constraints of the same shape pinned at different rows
// collapse too.
//
// Sharing is sound for the same reason as in zkcdriver: a node denotes a pure
// function of what it reads, and nothing downstream mutates an expression in
// place. The cache is used for lookup only and never iterated, so it does not
// affect output ordering.
type interner struct {
	nodes map[nodeKey]wiop.Expression
}

func newInterner() *interner {
	return &interner{nodes: map[nodeKey]wiop.Expression{}}
}

// intern returns the canonical node for expr: the first structurally identical
// node seen, or expr itself when it is the first. Operands of expr must
// already be interned.
func (in *interner) intern(expr wiop.Expression) wiop.Expression {
	key := keyOf(expr)
	if canonical, ok := in.nodes[key]; ok {
		return canonical
	}
	in.nodes[key] = expr
	return expr
}

// seed registers every node of expr as canonical, bottom-up. A node whose key
// is already taken by a different pointer is left as is — expr is not
// rewritten — but its parents are keyed on the canonical operand, so they can
// only match nodes built from canonical parts. Seeding an interned expression
// (as zkcdriver produces) registers each of its nodes.
func (in *interner) seed(expr wiop.Expression, visited map[wiop.Expression]struct{}) {
	if _, ok := visited[expr]; ok {
		return
	}
	visited[expr] = struct{}{}
	if ao, ok := expr.(*wiop.ArithmeticOperation); ok {
		for _, operand := range ao.Operands {
			in.seed(operand, visited)
		}
	}
	key := keyOf(expr)
	if _, ok := in.nodes[key]; !ok {
		in.nodes[key] = expr
	}
}
