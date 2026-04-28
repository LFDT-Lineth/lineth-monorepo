package wiop

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
)

// Module is a group of columns sharing the same domain size and padding
// semantics. All columns within a module are eligible to participate in the
// same set of global constraints.
//
// A module may be declared without a size (unsized) and have its size fixed
// later via [Module.SetSize]. Once set, the size is immutable.
//
// Modules are created via [System.NewModule] or [System.NewSizedModule], which
// register them with their owning System and set the system back-reference.
type Module struct {
	// Context identifies the module in the protocol hierarchy.
	Context *ContextFrame
	// Padding specifies how column assignments are padded to the module size.
	Padding PaddingDirection
	// Annotations holds arbitrary metadata attached to this module.
	Annotations Annotations
	// Columns is the ordered list of columns declared in this module.
	Columns []*Column
	// Vanishings is the ordered list of vanishing constraints registered on
	// this module via [Module.NewVanishing] and [Module.NewVanishingManual].
	Vanishings []*Vanishing
	// LocalOpenings holds all [LocalOpening] queries registered with this
	// system via [System.NewLocalOpening], in declaration order.
	LocalOpenings []*LocalOpening
	// size is zero when the module has not yet been sized. Use [Module.Size]
	// and [Module.SetSize] rather than accessing this field directly.
	size int
	// isDynamic indicates that this module's domain size is supplied per-Runtime
	// via [WithModuleSize] instead of being fixed once via [Module.SetSize].
	// Dynamic modules always report IsSized() == false from the static API.
	isDynamic bool
	// index is the position of this module in [System.Modules]. Set once at
	// registration time by [System.NewModule] and used to construct column IDs.
	index int
	// system is the owning System. Set once at registration time by
	// [System.NewModule], never nil for a well-formed Module.
	system *System
}

// System returns the owning System. It is always non-nil for a well-formed
// Module.
func (m *Module) System() *System { return m.system }

// IsDynamic reports whether this module's domain size is supplied per-Runtime
// via [WithModuleSize] rather than fixed statically via [Module.SetSize].
func (m *Module) IsDynamic() bool { return m.isDynamic }

// Size returns the declared domain size of the module. Returns 0 if the module
// has not yet been sized. For dynamic modules this always returns 0; use
// [Module.RuntimeSize] to obtain the size for a specific Runtime.
func (m *Module) Size() int { return m.size }

// IsSized reports whether a domain size has been fixed for this module.
// Dynamic modules always return false here; they are sized per-Runtime.
func (m *Module) IsSized() bool { return m.size > 0 }

// RuntimeSize returns the effective domain size for the given Runtime.
// For static modules it delegates to [Module.Size] (panics if not yet sized).
// For dynamic modules it reads the size registered via [WithModuleSize]
// (panics if no size was provided for this Runtime).
func (m *Module) RuntimeSize(rt Runtime) int {
	if !m.isDynamic {
		return m.Size()
	}
	return rt.dynamicModuleSize(m)
}

// SetSize fixes the domain size of the module. It may be called at any point
// after construction but only once; subsequent calls panic.
//
// Panics if size is not positive, if the module is already sized, or if the
// module is dynamic (dynamic modules are sized via [WithModuleSize] on each
// [Runtime]).
func (m *Module) SetSize(size int) {
	if m.isDynamic {
		panic(fmt.Sprintf("wiop: module %q is dynamic; set its size via WithModuleSize on each Runtime", m.Context.Path()))
	}
	if size <= 0 {
		panic(fmt.Sprintf("wiop: Module.SetSize requires a positive size, got %d", size))
	}
	if m.IsSized() {
		panic(fmt.Sprintf("wiop: module %q is already sized to %d; cannot resize to %d",
			m.Context.Path(), m.size, size))
	}
	m.size = size
}

// newColumn is the shared constructor used by [Module.NewColumn] and
// [Module.NewExtensionColumn]. It creates the column, appends it to both the
// module's and the round's column lists, and returns it.
func (m *Module) newColumn(ctx *ContextFrame, vis Visibility, isExt bool, r *Round) *Column {
	if ctx == nil {
		panic("wiop: NewColumn requires a non-nil ContextFrame")
	}
	if ctx.ID != 0 {
		panic(fmt.Sprintf("wiop: ContextFrame %q is already registered (id=%d)", ctx.Path(), ctx.ID))
	}
	ctx.ID = newColumnID(m.index, len(m.Columns))
	col := &Column{
		Context:     ctx,
		Visibility:  vis,
		IsExtension: isExt,
		Annotations: make(Annotations),
		Module:      m,
		round:       r,
	}
	m.Columns = append(m.Columns, col)
	r.Columns = append(r.Columns, col)
	return col
}

// NewColumn declares a new base-field column in this module for the given
// round, registers it with both the module and the round, and returns it.
//
// Panics if ctx or r is nil.
func (m *Module) NewColumn(ctx *ContextFrame, vis Visibility, r *Round) *Column {
	if r == nil {
		panic("wiop: Module.NewColumn requires a non-nil Round")
	}
	return m.newColumn(ctx, vis, false, r)
}

// NewExtensionColumn declares a new extension-field column in this module for
// the given round, registers it with both the module and the round, and
// returns it. Extension columns are evaluated over an extended domain (e.g. a
// coset) rather than the standard domain.
//
// Panics if ctx or r is nil.
func (m *Module) NewExtensionColumn(ctx *ContextFrame, vis Visibility, r *Round) *Column {
	if r == nil {
		panic("wiop: Module.NewExtensionColumn requires a non-nil Round")
	}
	return m.newColumn(ctx, vis, true, r)
}

// NewPrecomputedColumn declares a new precomputed column in this module,
// registers it with the system's PrecomputedRound together with its static
// field assignment, and returns it.
//
// The round is implicit: it is always System.PrecomputedRound, accessed via
// the module's system back-reference. Precomputed columns are always
// base-field columns.
//
// Panics if ctx or assignment is nil, or if the module has no owning System.
func (m *Module) NewPrecomputedColumn(ctx *ContextFrame, vis Visibility, assignment *ConcreteVector) *Column {
	if ctx == nil {
		panic("wiop: Module.NewPrecomputedColumn requires a non-nil ContextFrame")
	}
	if m.isDynamic {
		panic(fmt.Sprintf(
			"wiop: module %q is dynamic; precomputed columns require a statically-sized module",
			m.Context.Path(),
		))
	}
	if m.system == nil {
		panic(fmt.Sprintf(
			"wiop: module %q has no owning System; cannot declare a precomputed column",
			m.Context.Path(),
		))
	}
	if ctx.ID != 0 {
		panic(fmt.Sprintf("wiop: ContextFrame %q is already registered (id=%d)", ctx.Path(), ctx.ID))
	}
	ctx.ID = newColumnID(m.index, len(m.Columns))
	pr := m.system.PrecomputedRound
	col := &Column{
		Context:     ctx,
		Visibility:  vis,
		IsExtension: false,
		Annotations: make(Annotations),
		Module:      m,
		round:       &pr.Round,
	}
	m.Columns = append(m.Columns, col)
	pr.addPrecomputedColumn(col, assignment)
	return col
}

// Column is a symbolic vector-valued object representing a sequence of field
// elements that the prover assigns at runtime. Columns are the primary
// building blocks of constraints: they appear in expressions and are
// referenced by queries.
//
// A Column always belongs to exactly one [Module], from which it inherits its
// domain size and padding semantics, and to exactly one [Round], which
// determines when in the protocol the prover must commit to it.
type Column struct {
	// Context identifies this column in the protocol hierarchy.
	Context *ContextFrame
	// Visibility controls how this column participates in queries and whether
	// the verifier can observe it.
	Visibility Visibility
	// IsExtension indicates that this column is evaluated over an extended
	// domain rather than the standard domain.
	IsExtension bool
	// Annotations holds arbitrary metadata attached to this column.
	Annotations Annotations
	// Module is the owning module. It is always non-nil for a well-formed
	// column.
	Module *Module
	// round is the owning Round. Set once at construction, never nil for a
	// well-formed Column.
	round *Round
}

// Round returns the round in which this column is committed. It is always
// non-nil for a well-formed Column.
func (c *Column) Round() *Round { return c.round }

// Degree returns the polynomial degree of the column over its domain, which
// is Size() - 1. Panics if the owning module has not been sized yet.
func (c *Column) Degree() int {
	if !c.Module.IsSized() {
		panic(fmt.Sprintf("wiop: Degree() called on unsized column %q", c.Context.Path()))
	}
	return c.Module.Size() - 1
}

// ColumnView is a column derived from a parent [Column] by applying a
// cyclic shift of ShiftingOffset positions. For a positive offset, the i-th
// element of the shifted column equals the (i+ShiftingOffset)-th element of
// the parent, wrapping around cyclically. A zero ShiftingOffset is the
// unshifted identity view, as returned by [Column.View].
//
// ColumnView has no identity (no ContextFrame) because it is a purely
// derived, structural value: the same parent and offset always describe the
// same object.
type ColumnView struct {
	// Column is the column being shifted.
	Column *Column
	// ShiftingOffset is the non-zero cyclic shift amount. Negative values shift left.
	ShiftingOffset int
}

// View returns an unshifted ColumnView wrapping this column. Use [ColumnView.Shift]
// to apply a non-zero offset.
//
// Panics if the receiver is nil.
func (c *Column) View() *ColumnView {
	if c == nil {
		panic("wiop: Column.View requires a non-nil Column")
	}
	return &ColumnView{Column: c}
}

// Shift plain-copies the current view and increments the shifting by offset.
// The shift is cyclic. It does not mutate the receiver.
func (cv *ColumnView) Shift(offset int) *ColumnView {
	if cv == nil || cv.Column == nil {
		panic("wiop: Shift requires a non-nil parent Column or ColumnView")
	}
	// Doing the update with a plain struct copy makes the code supports better
	// new field additions.
	new := *cv
	new.ShiftingOffset += offset
	return &new
}

// Round returns the round of the parent column.
func (cv *ColumnView) Round() *Round { return cv.Column.round }

// Module returns the module of the parent column.
func (cv *ColumnView) Module() *Module { return cv.Column.Module }

// IsExtension implements [Expression]. Delegates to the parent column.
func (cv *ColumnView) IsExtension() bool { return cv.Column.IsExtension }

// IsMultiValued implements [Expression]. Always returns true: a column view
// is always vector-valued.
func (cv *ColumnView) IsMultiValued() bool { return true }

// IsSized implements [Expression]. Reports whether the owning module has a
// fixed size.
func (cv *ColumnView) IsSized() bool { return cv.Column.Module.IsSized() }

// Size implements [Expression]. Returns the domain size of the owning module.
// Returns 0 if the module has not been sized yet.
func (cv *ColumnView) Size() int { return cv.Column.Module.Size() }

// Degree implements [Expression]. Returns Size() - 1. Panics if the owning
// module has not been sized yet.
func (cv *ColumnView) Degree() int {
	if !cv.Column.Module.IsSized() {
		panic(fmt.Sprintf("wiop: Degree() called on unsized column view of %q", cv.Column.Context.Path()))
	}
	return cv.Column.Module.Size() - 1
}

// EvaluateVector implements [Expression]. Returns a full-sized concrete vector
// (length == module size) where logical row i holds the column value at
// physical row (i + ShiftingOffset) mod n, accounting for the module's padding.
func (cv *ColumnView) EvaluateVector(rt Runtime) ConcreteVector {
	concrete := rt.GetColumnAssignment(cv.Column)
	m := cv.Column.Module
	n := m.RuntimeSize(rt)

	var result field.Vec
	if cv.Column.IsExtension {
		dst := make([]field.Ext, n)
		for i := range n {
			phys := ((i+cv.ShiftingOffset)%n + n) % n
			dst[i] = concrete.ElementAtN(m.Padding, n, phys).Ext
		}
		result = field.VecFromExt(dst)
	} else {
		dst := make([]field.Element, n)
		for i := range n {
			phys := ((i+cv.ShiftingOffset)%n + n) % n
			dst[i] = concrete.ElementAtN(m.Padding, n, phys).AsBase()
		}
		result = field.VecFromBase(dst)
	}

	return ConcreteVector{
		Plain:   result,
		Padding: concrete.Padding,
		promise: cv,
	}
}

// EvaluateSingle implements [Expression]. Panics unconditionally: a column
// view is vector-valued and produces no scalar. Check IsMultiValued() before
// calling EvaluateSingle.
func (cv *ColumnView) EvaluateSingle(_ Runtime) ConcreteField {
	panic("wiop: EvaluateSingle() cannot be called on a VectorPromise")
}

// Visibility implements [Expression]. Delegates to the parent column's
// declared visibility.
func (cv *ColumnView) Visibility() Visibility { return cv.Column.Visibility }

// ColumnPosition represents the evaluation of a [Column] at a single fixed
// row index. It implements [FieldPromise] (and thereby [Expression]),
// producing the scalar value Column[Position] when assigned.
//
// ColumnPosition is a pure value type with no identity: the same Column and
// Position always describe the same object. Construct via [Column.At].
type ColumnPosition struct {
	// Column is the parent column.
	Column *Column
	// Position is the zero-based row index into the column.
	Position int
}

// At constructs a [ColumnPosition] for this column at the given zero-based
// row index. The result is a scalar [FieldPromise] evaluating to Column[pos].
//
// Panics if the receiver is nil.
func (c *Column) At(pos int) *ColumnPosition {
	if c == nil {
		panic("wiop: Column.At requires a non-nil Column")
	}
	return &ColumnPosition{Column: c, Position: pos}
}

// IsMultiValued implements [Expression]. Always returns false: a column
// position evaluates to a single field element.
func (cp *ColumnPosition) IsMultiValued() bool { return false }

// IsExtension implements [Expression]. Delegates to the parent column.
func (cp *ColumnPosition) IsExtension() bool { return cp.Column.IsExtension }

// Degree implements [Expression]. Always returns 0: a scalar evaluation is a
// degree-0 constant.
func (cp *ColumnPosition) Degree() int { return 0 }

// Round returns the round of the parent column.
func (cp *ColumnPosition) Round() *Round { return cp.Column.round }

// Module implements [Expression]. Always returns nil: a column position is
// scalar, not vector-valued.
func (cp *ColumnPosition) Module() *Module { return nil }

// IsSized implements [Expression]. Panics unconditionally: IsSized has no
// meaning for a scalar. Check IsMultiValued() before calling.
func (cp *ColumnPosition) IsSized() bool {
	panic("wiop: IsSized() cannot be called on a FieldPromise")
}

// Size implements [Expression]. Panics unconditionally: Size has no meaning
// for a scalar. Check IsMultiValued() before calling.
func (cp *ColumnPosition) Size() int {
	panic("wiop: Size() cannot be called on a FieldPromise")
}

// Visibility implements [Expression]. Delegates to the parent column's
// declared visibility.
func (cp *ColumnPosition) Visibility() Visibility { return cp.Column.Visibility }

// EvaluateVector implements [Expression]. Panics unconditionally: a column
// position is scalar and produces no vector. Check IsMultiValued() before
// calling.
func (cp *ColumnPosition) EvaluateVector(_ Runtime) ConcreteVector {
	panic("wiop: EvaluateVector() cannot be called on a FieldPromise")
}

// EvaluateSingle implements [Expression]. Returns the value of the parent
// column at Position in the given runtime.
func (cp *ColumnPosition) EvaluateSingle(rt Runtime) ConcreteField {
	m := cp.Column.Module
	elem := rt.GetColumnAssignment(cp.Column).ElementAtN(m.Padding, m.RuntimeSize(rt), cp.Position)
	return ConcreteField{Value: elem, promise: cp}
}
