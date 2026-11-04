package zkcdriver

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/zkc/pkg/schema"
)

// exprKey identifies an [wiop.Expression] node by the value it denotes, so two
// nodes carrying the same key are interchangeable. It is a comparable struct
// (no slices, no pointers to mutable state), so it can be a map key directly.
//
// Operands are named by their *interned* pointers rather than by their own
// subtrees. That is enough to make the key canonical because interning runs
// bottom-up: an operand is interned before the node referencing it, so two
// structurally equal operand subtrees have already collapsed onto one pointer
// by the time their parents are keyed.
//
// Only the fields relevant to `kind` are set; the rest stay at their zero
// value. `b` is nil for unary operations, which keeps a unary node distinct
// from a binary one sharing the same first operand.
type exprKey struct {
	kind exprKind
	// column is the [wiop.ObjectID] of the parent column, for the leaf kinds
	// that reference one. Column IDs are unique across the whole system, so
	// they alone discriminate leaves.
	column wiop.ObjectID
	// offset is the shift of a ColumnView or the row of a ColumnPosition.
	offset int
	// constant is the value of a Constant. field.Element is [1]uint32, a fixed
	// array, hence directly comparable.
	constant field.Element
	// operator and operands describe an ArithmeticOperation. a and b are
	// interface values, so Go compares them by dynamic type and value; every
	// [wiop.Expression] reachable here is a pointer type, which makes that
	// comparison pointer identity and keeps the key hashable. A non-pointer,
	// non-comparable implementation would panic on map access instead of
	// silently mis-keying, and [makeExprKey] rejects unknown types anyway.
	operator wiop.ArithmeticOperator
	a, b     wiop.Expression
}

// exprKind tags which variant of [exprKey] is populated, so that e.g. a
// Constant of value 0 and a ColumnView of the unregistered column 0 never
// collide.
type exprKind uint8

const (
	kindColumnView exprKind = iota
	kindColumnPosition
	kindConstant
	kindArithmeticOp
)

// makeExprKey derives the [exprKey] of expr. Its operands, when it has any,
// must already be interned.
//
// Only [wiop.ColumnView], [wiop.ColumnPosition], [wiop.Constant] and
// [wiop.ArithmeticOperation] are reachable from [schemaScanner.castExpression]
// and from the local-constraint rewrite that follows it. Anything else is a
// caller error rather than a node to pass through un-interned, so it panics:
// silently skipping would leave the cache quietly ineffective.
func makeExprKey(expr wiop.Expression) exprKey {
	switch e := expr.(type) {
	case *wiop.ColumnView:
		return exprKey{
			kind:   kindColumnView,
			column: e.Column.Context.ID,
			offset: e.ShiftingOffset,
		}
	case *wiop.ColumnPosition:
		return exprKey{
			kind:   kindColumnPosition,
			column: e.Column.Context.ID,
			offset: e.Position,
		}
	case *wiop.Constant:
		// Only scalar constants are built here ([wiop.NewConstantField]), so
		// the bound module needs no part in the key.
		if e.Module() != nil {
			panic("zkcdriver: makeExprKey: unexpected vector Constant")
		}
		return exprKey{kind: kindConstant, constant: e.Value}
	case *wiop.ArithmeticOperation:
		k := exprKey{kind: kindArithmeticOp, operator: e.Operator, a: e.Operands[0]}
		if len(e.Operands) > 1 {
			k.b = e.Operands[1]
		}
		return k
	default:
		panic(fmt.Sprintf("zkcdriver: makeExprKey: unsupported expression type %T", expr))
	}
}

// newExprCache allocates an empty interning cache. It exists so that [Define]
// can initialise [schemaScanner.ExprCache] without naming [schema.ModuleId],
// which its own `schema` parameter shadows.
func newExprCache() map[schema.ModuleId]map[exprKey]wiop.Expression {
	return map[schema.ModuleId]map[exprKey]wiop.Expression{}
}

// intern returns the canonical node for expr within the given module: the
// first structurally identical node seen there, or expr itself when it is the
// first. Operands of expr must already be interned.
//
// Interning is per-module because the cache is keyed by module. Column
// [wiop.ObjectID]s are already system-unique, so the module scope only matters
// for constant-only subtrees — which are cheap either way; the scope is kept
// to match the issue's contract and to keep the maps small.
//
// Sharing is sound because a node denotes a pure function of the columns it
// reads: two structurally identical nodes always evaluate to the same value.
// Nothing downstream mutates an [wiop.Expression] in place — the caches on
// [wiop.ArithmeticOperation] (isMultiValued, the compiled program) are
// memoised pure functions of the subtree, so a shared node computes them once
// instead of once per occurrence.
func (s *schemaScanner) intern(context schema.ModuleId, expr wiop.Expression) wiop.Expression {

	cache, ok := s.ExprCache[context]
	if !ok {
		cache = map[exprKey]wiop.Expression{}
		s.ExprCache[context] = cache
	}

	key := makeExprKey(expr)
	if canonical, ok := cache[key]; ok {
		return canonical
	}

	cache[key] = expr
	return expr
}

// internedSum folds terms left-to-right into a binary Add tree, interning
// every intermediate node. It mirrors [wiop.Sum], which cannot intern on its
// own: the intermediate nodes of the fold are never exposed to the caller, so
// interning only the root would leave every partial sum distinct and defeat
// the sharing of any prefix.
//
// Panics if terms is empty, like [wiop.Sum].
func (s *schemaScanner) internedSum(context schema.ModuleId, terms ...wiop.Expression) wiop.Expression {
	if len(terms) == 0 {
		panic("zkcdriver: internedSum requires at least one term")
	}
	result := terms[0]
	for _, t := range terms[1:] {
		result = s.intern(context, wiop.Add(result, t))
	}
	return result
}

// internedProduct is the [wiop.Product] counterpart of [internedSum].
//
// Panics if factors is empty, like [wiop.Product].
func (s *schemaScanner) internedProduct(context schema.ModuleId, factors ...wiop.Expression) wiop.Expression {
	if len(factors) == 0 {
		panic("zkcdriver: internedProduct requires at least one factor")
	}
	result := factors[0]
	for _, f := range factors[1:] {
		result = s.intern(context, wiop.Mul(result, f))
	}
	return result
}
