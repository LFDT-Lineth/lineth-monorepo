package wiop

import "fmt"

// NewLocalConstraint registers a [Vanishing] enforced at a single row of the
// module — a "local constraint" in the sense of
// prover/protocol/query.LocalConstraint.
//
// The position argument selects which row of the module the predicate is
// pinned to. Only three values are accepted:
//
//   - 0: the first row (row 0).
//   - 1: the second row (row 1).
//   - -1: the last row (row n−1, where n is the module size).
//
// Every column reference in expr is interpreted as the column's value at the
// chosen row. A reference of the form col.View().Shift(k) (a [*ColumnView]
// with non-zero ShiftingOffset k) is composed with position: the column is
// read at row (position + k) mod n. Coins, cells, and scalar constants do
// not depend on position and are evaluated as-is.
//
// Internally, every [*ColumnView] in expr is rewritten to a [*ColumnPosition]
// at the resolved row, and every vector [*Constant] is collapsed to its
// scalar form. The lowered expression is necessarily scalar, so the returned
// [*Vanishing] is checked through the scalar branch of [Vanishing.Check].
// This keeps a single Query type for both "global" (multi-valued) and "local"
// (scalar) vanishing predicates.
//
// Reading row −1 (or any negative resolved row) requires m to be statically
// sized so the row can be normalised; an unsized or dynamic module cannot
// resolve negative rows at construction time.
//
// Column ownership is not validated: in line with [Module.NewVanishing], the
// caller is responsible for ensuring that the expression references columns
// belonging to m. Mixing columns from different modules will still evaluate
// correctly (each [*ColumnPosition] resolves through its own column's module)
// but is outside the intended use.
//
// Panics if ctx or expr is nil, if position is not in {−1, 0, 1}, or if a
// resolved row is negative on an unsized/dynamic module.
func (m *Module) NewLocalConstraint(ctx *ContextFrame, expr Expression, position int) *Vanishing {
	if ctx == nil {
		panic("wiop: Module.NewLocalConstraint requires a non-nil ContextFrame")
	}
	if expr == nil {
		panic("wiop: Module.NewLocalConstraint requires a non-nil Expression")
	}
	if position != -1 && position != 0 && position != 1 {
		panic(fmt.Sprintf(
			"wiop: Module.NewLocalConstraint: position must be -1 (last row), 0 (first row), or 1 (second row), got %d",
			position,
		))
	}
	scalar := m.lowerToRow(expr, position)
	if scalar.IsMultiValued() {
		panic(fmt.Sprintf(
			"wiop: Module.NewLocalConstraint(%s): lowered expression is unexpectedly multi-valued",
			ctx.Path(),
		))
	}
	return m.newVanishing(ctx, scalar, nil)
}

// lowerToRow traverses expr bottom-up via [EditExpression] and produces the
// scalar expression that the local constraint evaluates at the given row.
// The rewrite rules are:
//
//   - [*ColumnView]{Column: c, ShiftingOffset: k}
//     → [*ColumnPosition]{Column: c, Position: (position + k) mod n}.
//
//   - vector [*Constant]{Value: v, module: !=nil}
//     → scalar [*Constant]{Value: v, module: nil} (the same value at any row).
//
//   - every other leaf ([*Cell], [*CoinField], [*ColumnPosition], scalar
//     [*Constant]) is passed through unchanged.
//
//   - [*ArithmeticOperation] is rebuilt with the rewritten operands.
func (m *Module) lowerToRow(expr Expression, position int) Expression {
	return EditExpression(expr, func(curr Expression, newChildren []Expression) Expression {
		switch e := curr.(type) {
		case *ColumnView:
			return columnViewAtRow(e, position)
		case *ColumnPosition:
			return e
		case *Constant:
			if e.module != nil {
				return NewConstantField(e.Value)
			}
			return e
		default:
			return DefaultConstruct(curr, newChildren)
		}
	})
}

// columnViewAtRow converts a [*ColumnView] into the [*ColumnPosition] it
// evaluates to at logical row `position`. The resolved row is
// (position + cv.ShiftingOffset) and is normalised modulo the module size
// when the module is sized. For an unsized or dynamic module the resolved
// row is passed through directly; a negative resolved row is rejected because
// it cannot be normalised without knowing n.
func columnViewAtRow(cv *ColumnView, position int) *ColumnPosition {
	target := position + cv.ShiftingOffset
	m := cv.Column.Module
	if m.IsSized() {
		n := m.Size()
		target %= n
		if target < 0 {
			target += n
		}
	} else if target < 0 {
		panic(fmt.Sprintf(
			"wiop: NewLocalConstraint: resolved row %d (position %d + shift %d) on column %q requires a sized module",
			target, position, cv.ShiftingOffset, cv.Column.Context.Path(),
		))
	}
	return &ColumnPosition{Column: cv.Column, Position: target}
}
