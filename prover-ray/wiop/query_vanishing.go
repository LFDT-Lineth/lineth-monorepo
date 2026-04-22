package wiop

import (
	"fmt"
	"slices"
	"sort"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
)

// Vanishing is a [Query] asserting that an expression evaluates to zero. It
// conflates GlobalConstraint (multi-valued expression) and LocalConstraint
// (scalar expression) into a single type: the nature of the expression
// determines which case applies at compile time.
//
// CancelledPositions lists the row indices that are exempt from the predicate.
// Positive indices count from the start of the domain; negative indices count
// from the end (−1 is the last row). The slice is always sorted and
// deduplicated.
//
// When the expression is fully public the verifier can check the predicate
// directly without needing a PCS reduction.
type Vanishing struct {
	baseQuery
	// Expression is the symbolic arithmetic expression that must vanish.
	// It is module-bound: all ColumnViews in the expression must reference
	// columns belonging to the same module.
	Expression Expression
	// CancelledPositions lists the rows exempt from the vanishing predicate,
	// sorted and deduplicated. Positive values index from the start; negative
	// values index from the end (−1 = last row).
	CancelledPositions []int
}

// Module returns the module associated with this vanishing predicate. It
// delegates to [Expression.Module], which returns the module of the first
// vector-valued subexpression, or nil if the expression is scalar.
func (v *Vanishing) Module() *Module {
	return v.Expression.Module()
}

// Round implements [Query]. Returns the [Round] with the highest ID among all
// round-carrying leaves in the expression (columns, cells, coins).
func (v *Vanishing) Round() *Round {
	return maxRoundInExpr(v.Expression)
}

// Check implements [Query]. For a scalar expression, verifies it evaluates to
// zero. For a vector expression, verifies every non-cancelled row is zero.
// CancelledPositions are normalised: negative indices count from the end of the
// domain (−1 = last row).
func (v *Vanishing) Check(rt Runtime) error {
	if !v.Expression.IsMultiValued() {
		val := v.Expression.EvaluateSingle(rt)
		if !val.Value.IsZero() {
			return fmt.Errorf("wiop: Vanishing(%s).Check: scalar expression is non-zero", v.context.Path())
		}
		return nil
	}

	vec := v.Expression.EvaluateVector(rt)
	fv := vec.Plain[0]
	n := fv.Len()

	// Build a set of cancelled row indices normalised to [0, n).
	cancelled := make(map[int]struct{}, len(v.CancelledPositions))
	for _, p := range v.CancelledPositions {
		if p < 0 {
			cancelled[n+p] = struct{}{}
		} else {
			cancelled[p] = struct{}{}
		}
	}

	for row := range n {
		if _, skip := cancelled[row]; skip {
			continue
		}
		var elem field.Gen
		if fv.IsBase() {
			elem = field.ElemFromBase(fv.AsBase()[row])
		} else {
			elem = field.ElemFromExt(fv.AsExt()[row])
		}
		if !elem.IsZero() {
			return fmt.Errorf(
				"wiop: Vanishing(%s).Check: expression is non-zero at row %d",
				v.context.Path(), row,
			)
		}
	}
	return nil
}

// CheckGnark implements [GnarkCheckableQuery].
func (v *Vanishing) CheckGnark(_ frontend.API, _ GnarkRuntime) {
	panic("wiop: Vanishing.CheckGnark not yet implemented")
}

// NewVanishing registers a new [Vanishing] constraint on the module. The
// cancelled positions are derived automatically from the shifts present in
// expr: for every [ColumnView] with a non-zero [ColumnView.ShiftingOffset] k,
//   - positive k: the last k rows (represented as −k, …, −1) are cancelled.
//   - negative k: the first |k| rows (represented as 0, …, |k|−1) are cancelled.
//
// The resulting position list is sorted and deduplicated before storage.
//
// Panics if ctx or expr is nil.
func (m *Module) NewVanishing(ctx *ContextFrame, expr Expression) *Vanishing {
	if ctx == nil {
		panic("wiop: Module.NewVanishing requires a non-nil ContextFrame")
	}
	if expr == nil {
		panic("wiop: Module.NewVanishing requires a non-nil Expression")
	}
	positions := cancelledPositionsFromExpr(expr)
	return m.newVanishing(ctx, expr, positions)
}

// NewVanishingManual registers a [Vanishing] constraint on the module with an
// explicit cancellation set. The caller owns the semantics:
//   - an empty positions list means the predicate is enforced on every row.
//   - a non-empty list gives the exact rows to skip.
//
// Positive positions index from the start; negative positions index from the
// end. The list is sorted and deduplicated before storage.
//
// Panics if ctx or expr is nil.
func (m *Module) NewVanishingManual(ctx *ContextFrame, expr Expression, positions ...int) *Vanishing {
	if ctx == nil {
		panic("wiop: Module.NewVanishingManual requires a non-nil ContextFrame")
	}
	if expr == nil {
		panic("wiop: Module.NewVanishingManual requires a non-nil Expression")
	}
	sorted := dedupSortedInts(positions)
	return m.newVanishing(ctx, expr, sorted)
}

// newVanishing is the shared implementation used by [Module.NewVanishing] and
// [Module.NewVanishingManual]. It constructs the [Vanishing], appends it to
// the module's Vanishings list, and returns it.
func (m *Module) newVanishing(ctx *ContextFrame, expr Expression, positions []int) *Vanishing {
	v := &Vanishing{
		baseQuery: baseQuery{
			context:     ctx,
			Annotations: make(Annotations),
		},
		Expression:         expr,
		CancelledPositions: positions,
	}
	m.Vanishings = append(m.Vanishings, v)
	return v
}

// cancelledPositionsFromExpr collects all non-zero [ColumnView.ShiftingOffset]
// values from the expression tree, converts each shift to a set of cancelled
// row positions, and returns the merged set in sorted order.
func cancelledPositionsFromExpr(expr Expression) []int {
	shifts := make(map[int]struct{})
	collectShifts(expr, shifts)
	return cancelledPositionsFromShifts(shifts)
}

// collectShifts recursively traverses expr and records all non-zero
// [ColumnView.ShiftingOffset] values into the shifts set.
func collectShifts(expr Expression, shifts map[int]struct{}) {
	switch e := expr.(type) {
	case *ColumnView:
		if e.ShiftingOffset != 0 {
			shifts[e.ShiftingOffset] = struct{}{}
		}
	case *ArithmeticOperation:
		for _, op := range e.Operands {
			collectShifts(op, shifts)
		}
		// *Cell, *CoinField, and other leaf types carry no shifts.
	}
}

// cancelledPositionsFromShifts converts a set of non-zero shift offsets into
// the corresponding set of cancelled row positions:
//   - shift k > 0: cancels the last k rows, represented as −k, …, −1.
//   - shift k < 0: cancels the first |k| rows, represented as 0, …, |k|−1.
//
// Returns the position list sorted in ascending order (negatives first, then
// non-negatives).
func cancelledPositionsFromShifts(shifts map[int]struct{}) []int {
	posSet := make(map[int]struct{})
	for off := range shifts {
		if off > 0 {
			for i := 1; i <= off; i++ {
				posSet[-i] = struct{}{}
			}
		} else if off < 0 {
			for i := 0; i < -off; i++ {
				posSet[i] = struct{}{}
			}
		}
	}
	positions := make([]int, 0, len(posSet))
	for p := range posSet {
		positions = append(positions, p)
	}
	sort.Ints(positions)
	return positions
}

// dedupSortedInts returns a new sorted, deduplicated copy of xs.
func dedupSortedInts(xs []int) []int {
	if len(xs) == 0 {
		return nil
	}
	cp := make([]int, len(xs))
	copy(cp, xs)
	sort.Ints(cp)
	return slices.Compact(cp)
}

// String returns a human-readable description of the Vanishing constraint for
// debugging and logging.
func (v *Vanishing) String() string {
	return fmt.Sprintf("Vanishing(%s, cancelled=%v)", v.context.Path(), v.CancelledPositions)
}
