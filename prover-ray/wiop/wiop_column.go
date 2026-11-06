package wiop

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/parallel"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/internal/padded"
	"github.com/sirupsen/logrus"
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
	// RangeChecks holds all [RangeCheck] queries registered on this module
	// via [Module.NewRangeCheck], in declaration order.
	RangeChecks []*RangeCheck
	// NonNatives holds all [NonNative] queries registered on this module via
	// [Module.NewNonNative], in declaration order.
	NonNatives []*NonNative
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
// has not yet been sized.
//
// Panics if the module is dynamic: a dynamic module has no compile-time size,
// and silently returning 0 turns size-driven loops into no-ops (which is how
// [RangeCheck.Check] was made vacuous on dynamic modules). Use
// [Module.RuntimeSize] to obtain the size for a specific Runtime, and guard
// with [Module.IsDynamic] when a static size is genuinely optional.
func (m *Module) Size() int {
	if m.isDynamic {
		panic(fmt.Sprintf(
			"wiop: Size() called on dynamic module %q; its size is per-Runtime, use Module.RuntimeSize(rt)",
			m.Context.Path(),
		))
	}
	return m.size
}

// IsSized reports whether a domain size has been fixed for this module.
// Dynamic modules always return false here; they are sized per-Runtime.
func (m *Module) IsSized() bool { return m.size > 0 }

// RuntimeSize returns the effective domain size for the given Runtime.
// For static modules it delegates to [Module.Size] (panics if not yet sized).
// For dynamic modules it reads the size registered via [WithModuleSize]
// (panics if no size was provided for this Runtime).
func (m *Module) RuntimeSize(rt *Runtime) int {
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
	if !utils.IsPowerOfTwo(size) {
		origSize := size
		size = utils.NextPowerOfTwo(size)
		logrus.Warnf("wiop: Module.SetSize requires a power-of-two size, got %d; rounding up to %d for %v", origSize, size, m.Context.Path())
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
func (m *Module) newColumn(ctx *ContextFrame, isExt bool, r *Round) *Column {
	if ctx == nil {
		panic("wiop: NewColumn requires a non-nil ContextFrame")
	}
	if ctx.ID != 0 {
		panic(fmt.Sprintf("wiop: ContextFrame %q is already registered (id=%d)", ctx.Path(), ctx.ID))
	}
	ctx.ID = newColumnID(m.index, len(m.Columns))
	col := &Column{
		Context:     ctx,
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
func (m *Module) NewColumn(ctx *ContextFrame, r *Round) *Column {
	if r == nil {
		panic("wiop: Module.NewColumn requires a non-nil Round")
	}
	return m.newColumn(ctx, false, r)
}

// NewExtensionColumn declares a new extension-field column in this module for
// the given round, registers it with both the module and the round, and
// returns it. Extension columns are evaluated over an extended domain (e.g. a
// coset) rather than the standard domain.
//
// Panics if ctx or r is nil.
func (m *Module) NewExtensionColumn(ctx *ContextFrame, r *Round) *Column {
	if r == nil {
		panic("wiop: Module.NewExtensionColumn requires a non-nil Round")
	}
	return m.newColumn(ctx, true, r)
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
func (m *Module) NewPrecomputedColumn(ctx *ContextFrame, assignment *ConcreteVector) *Column {
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
	if assignment.promise != nil {
		panic("wiop: Module.NewPrecomputedColumn requires a non-promised assignment")
	}
	ctx.ID = newColumnID(m.index, len(m.Columns))
	pr := m.system.PrecomputedRound
	col := &Column{
		Context:     ctx,
		IsExtension: false,
		Annotations: make(Annotations),
		Module:      m,
		round:       &pr.Round,
	}
	assignment.promise = col.View()
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
	if c.Module.IsDynamic() {
		panic(fmt.Sprintf("wiop: Degree() called on dynamic-sized column %q", c.Context.Path()))
	}
	if !c.Module.IsSized() {
		panic(fmt.Sprintf("wiop: Degree() called on unsized column %q", c.Context.Path()))
	}
	return c.Module.Size() - 1
}

// DegreeFactor implements [Expression]. Returns 1: a column's degree is
// 1 * (n - 1) where n is the module size.
func (c *Column) DegreeFactor() int { return 1 }

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
//
// Panics if the owning module is dynamic (its size is per-Runtime; use the
// module's RuntimeSize instead) or has not yet been sized. Check IsSized()
// first.
func (cv *ColumnView) Size() int {
	m := cv.Column.Module
	if m.IsDynamic() {
		panic(fmt.Sprintf("wiop: Size() called on dynamic-sized column view of %q", cv.Column.Context.Path()))
	}
	if !m.IsSized() {
		panic(fmt.Sprintf("wiop: Size() called on unsized column view of %q", cv.Column.Context.Path()))
	}
	return m.Size()
}

// Degree implements [Expression]. Returns Size() - 1. Panics if the owning
// module has not been sized yet.
func (cv *ColumnView) Degree() int {
	if !cv.Column.Module.IsSized() {
		panic(fmt.Sprintf("wiop: Degree() called on unsized column view of %q", cv.Column.Context.Path()))
	}
	return cv.Column.Module.Size() - 1
}

// DegreeFactor implements [Expression]. Returns 1: a column view's degree is
// 1 * (n - 1) where n is the module size.
func (cv *ColumnView) DegreeFactor() int { return 1 }

// RowReader reads blocks of rows of a column view straight from its column's
// assignment, without materialising the padded, shifted column: row i is the
// module's padded row (i + ShiftingOffset) mod n, as [ColumnView.EvaluateVector]
// returns it. Rows outside the assignment's data read as its padding value,
// following the module's [PaddingDirection] (see [ConcreteVector.ElementAtN]);
// for a [PaddingDirectionLeft] assignment longer than n, its last n values are
// the data.
//
// A RowReader is a read-only value: it is safe for concurrent use as long as
// the assignment it reads is not mutated. Obtain one with [ColumnView.Rows] or
// [NewRowReader], once, out of any hot loop: [ColumnView.Rows] looks the
// assignment up under the runtime's lock.
type RowReader struct {
	plain field.Vec
	pad   field.Element
	n     int
	shift int // in [0, n)
	start int // padded row of plain[0]; negative for an over-full Left assignment
}

// NewRowReader returns a [RowReader] over the assignment a laid out as a
// column of n rows padded towards padding, cyclically shifted by shift (any
// integer, as [ColumnView.ShiftingOffset]).
func NewRowReader(a *ConcreteVector, padding PaddingDirection, n, shift int) RowReader {
	r := RowReader{plain: a.Plain, pad: a.Padding, n: n}
	if n > 0 {
		r.shift = ((shift % n) + n) % n
	}
	if padding == PaddingDirectionLeft {
		r.start = n - a.Plain.Len()
	}
	return r
}

// Rows returns a [RowReader] over the view, sized to its module's runtime
// size.
func (cv *ColumnView) Rows(rt *Runtime) RowReader {
	return NewRowReader(
		rt.GetColumnAssignment(cv.Column),
		cv.Column.Module.Padding,
		cv.Column.Module.RuntimeSize(rt),
		cv.ShiftingOffset,
	)
}

// Len returns the number of rows n.
func (r RowReader) Len() int { return r.n }

// IsBase reports whether the assignment holds base-field data, which
// [RowReader.ReadBase] requires. Extension data must be read with
// [RowReader.ReadExt].
func (r RowReader) IsBase() bool { return r.plain.IsBase() }

// ReadBase copies rows [lo, hi) into dst and returns dst[:hi-lo]. Panics if
// the assignment holds extension data, if the range is not within [0, n), or
// if dst is shorter than hi-lo.
func (r RowReader) ReadBase(lo, hi int, dst []field.Element) []field.Element {
	r.checkRange(lo, hi)
	dst = dst[:hi-lo]
	padded.Read(dst, r.plain.AsBase(), r.pad, r.start, r.n, r.first(lo))
	return dst
}

// ReadExt copies rows [lo, hi) into dst, in the extension field, and returns
// dst[:hi-lo]. Base-field data and the padding value are lifted. Panics if the
// range is not within [0, n), or if dst is shorter than hi-lo.
func (r RowReader) ReadExt(lo, hi int, dst []field.Ext) []field.Ext {
	r.checkRange(lo, hi)
	dst = dst[:hi-lo]
	if r.plain.IsBase() {
		padded.ReadLift(dst, r.plain.AsBase(), r.pad, r.start, r.n, r.first(lo))
	} else {
		padded.Read(dst, r.plain.AsExt(), field.Lift(r.pad), r.start, r.n, r.first(lo))
	}
	return dst
}

// first returns the padded row read as row lo.
func (r RowReader) first(lo int) int {
	if lo == r.n { // empty read at the end: any row in [0, n) will do
		return 0
	}
	idx := lo + r.shift
	if idx >= r.n {
		idx -= r.n
	}
	return idx
}

func (r RowReader) checkRange(lo, hi int) {
	if lo < 0 || hi < lo || hi > r.n {
		panic(fmt.Sprintf("wiop: RowReader: rows [%d, %d) out of bounds [0, %d)", lo, hi, r.n))
	}
}

// EvaluateVector implements [Expression]. Returns a full-sized concrete vector
// (length == module size) where logical row i holds the column value at
// physical row (i + ShiftingOffset) mod n, accounting for the module's padding.
//
// The vector is a fresh copy, written in one pass by a [RowReader]: bulk
// copies and fills, the shift applied by reading from the right row rather
// than by rotating afterwards.
func (cv *ColumnView) EvaluateVector(rt *Runtime) ConcreteVector {
	r := cv.Rows(rt)
	n := r.Len()

	var result field.Vec
	if cv.Column.IsExtension {
		result = field.VecFromExt(r.ReadExt(0, n, make([]field.Ext, n)))
	} else {
		result = field.VecFromBase(r.ReadBase(0, n, make([]field.Element, n)))
	}

	return ConcreteVector{
		Plain:   result,
		Padding: r.pad,
		promise: cv,
	}
}

// EvaluateVectorAsExt is like EvaluateVector but always returns a length-n
// []field.Ext. Base-field columns are lifted; extension columns are copied
// as-is. If n exceeds the module size, the remainder is filled with the
// lifted padding value.
func (cv *ColumnView) EvaluateVectorAsExt(rt *Runtime, n int) []field.Ext {
	r := cv.Rows(rt)
	out := make([]field.Ext, n)
	k := min(n, r.Len())
	r.ReadExt(0, k, out)
	field.VecFillExt(out[k:], field.Lift(r.pad))
	return out
}

// EvaluateRLCAsExt computes the random linear combination
//
//	out[i] = es[0][i] + α·es[1][i] + α²·es[2][i] + …
//
// entry-wise with Horner's method, folding from the last entry down
// (acc = acc·α + es[k]). Only the initial entry is lifted to the extension
// field; the remaining column views are consumed un-lifted, so a base-field
// column costs one base addition per row on top of the single extension
// multiplication per entry per row. Constant entries are added as scalars and
// never materialised as vectors; a trailing run of constants (where the
// canonical column order puts them) is additionally folded once into a single
// scalar that seeds the accumulator, costing no per-row multiplication at
// all. Panics if es is empty.
//
// The fold is parallelized over row chunks: rows are independent, and each
// worker runs the full per-entry fold on its own range, so the accumulator
// chunk stays cache-resident across entries.
func EvaluateRLCAsExt(rt *Runtime, alpha field.Ext, es []TableEntry, n int) []field.Ext {
	if len(es) == 0 {
		panic("wiop: EvaluateRLCAsExt called with no columns")
	}

	// Evaluate every column view once, up front: EvaluateVector may touch
	// shared runtime state, so it stays out of the parallel section. Constant
	// entries leave their plain slot empty and are read from consts.
	plains := make([]field.Vec, len(es))
	consts := make([]field.Element, len(es))
	isConst := make([]bool, len(es))
	for k, e := range es {
		if c, ok := EntryConst(e); ok {
			consts[k], isConst[k] = c, true
			continue
		}
		plains[k] = e.EvaluateVector(rt).Plain
	}

	// A constant suffix es[j..last] is the same for every row, so Horner folds
	// it to the single scalar Σ αᵏ⁻ʲ·c_k once, here, and it seeds the
	// accumulator: this saves last−j extension multiplications per row.
	last := len(es) - 1
	j := len(es)
	for j > 0 && isConst[j-1] {
		j--
	}
	var suffix field.Ext
	if j <= last {
		suffix = field.Lift(consts[last])
		for k := last - 1; k >= j; k-- {
			suffix.Mul(&suffix, &alpha)
			suffix.B0.A0.Add(&suffix.B0.A0, &consts[k])
		}
	}

	acc := make([]field.Ext, n)
	parallel.Execute(n, func(start, stop int) {
		// next is the highest entry still to fold into acc.
		next := j - 1
		switch {
		case j <= last:
			for i := start; i < stop; i++ {
				acc[i] = suffix
			}
		case plains[last].IsBase():
			next = last - 1
			for i, x := range plains[last].AsBase()[start:stop] {
				acc[start+i] = field.Lift(x)
			}
		default:
			next = last - 1
			copy(acc[start:stop], plains[last].AsExt()[start:stop])
		}
		for k := next; k >= 0; k-- {
			switch {
			case isConst[k]:
				c := consts[k]
				for i := start; i < stop; i++ {
					acc[i].Mul(&acc[i], &alpha)
					acc[i].B0.A0.Add(&acc[i].B0.A0, &c)
				}
			case plains[k].IsBase():
				field.VecScaleAddExtBase(acc[start:stop], alpha, plains[k].AsBase()[start:stop])
			default:
				field.VecScaleAddExtExt(acc[start:stop], alpha, plains[k].AsExt()[start:stop])
			}
		}
	})
	return acc
}

// EvaluateSingle implements [Expression]. Panics unconditionally: a column
// view is vector-valued and produces no scalar. Check IsMultiValued() before
// calling EvaluateSingle.
func (cv *ColumnView) EvaluateSingle(_ *Runtime) ConcreteField {
	panic("wiop: EvaluateSingle() cannot be called on a VectorPromise")
}

// ColumnPosition represents the evaluation of a [Column] at a single fixed
// row index. It implements [FieldPromise] (and thereby [Expression]),
// producing the scalar value Column[Position] when assigned.
//
// ColumnPosition is a pure value type with no identity: the same Column and
// Position always describe the same object. Construct via [Column.At].
type ColumnPosition struct {
	// Column is the parent column.
	Column *Column
	// Position is the row index into the column. Non-negative values index
	// from the start of the domain; negative values index from the end, with
	// −1 being the last row, −2 the second-to-last, etc. The end-indexed
	// form is the only way to open the endpoint of a dynamic module, where
	// the absolute row index is only known per-[Runtime]. The same
	// convention is shared with [Vanishing.CancelledPositions].
	Position int
}

// resolvedRow returns the absolute row index of this position against a
// runtime-known domain size n. Negative Position values are interpreted from
// the end (−1 ⇒ n−1, −2 ⇒ n−2, …). Panics if the resolved index is out of
// the [0, n) range.
func (cp *ColumnPosition) resolvedRow(n int) int {
	row := cp.Position
	if row < 0 {
		row += n
	}
	if row < 0 || row >= n {
		panic(fmt.Sprintf(
			"wiop: ColumnPosition: row %d (Position=%d, runtime size %d) out of bounds",
			row, cp.Position, n,
		))
	}
	return row
}

// At constructs a [ColumnPosition] for this column at the given row index.
// Non-negative pos indexes from the start of the domain. Negative pos
// indexes from the end and is the canonical way to address a dynamic
// module's endpoint (pos = −1 ⇒ last row). The result is a scalar
// [FieldPromise] evaluating to Column[pos].
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

// DegreeFactor implements [Expression]. Always returns 0: a column position is
// a scalar constant.
func (cp *ColumnPosition) DegreeFactor() int { return 0 }

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

// EvaluateVector implements [Expression]. Panics unconditionally: a column
// position is scalar and produces no vector. Check IsMultiValued() before
// calling.
func (cp *ColumnPosition) EvaluateVector(_ *Runtime) ConcreteVector {
	panic("wiop: EvaluateVector() cannot be called on a FieldPromise")
}

// EvaluateSingle implements [Expression]. Returns the value of the parent
// column at Position in the given runtime. Negative Position values are
// resolved from the end of the domain (see [ColumnPosition.Position]).
func (cp *ColumnPosition) EvaluateSingle(rt *Runtime) ConcreteField {
	m := cp.Column.Module
	n := m.RuntimeSize(rt)
	elem := rt.GetColumnAssignment(cp.Column).ElementAtN(m.Padding, n, cp.resolvedRow(n))
	return ConcreteField{Value: elem, promise: cp}
}

// Open creates a "local opening" of this column position: a prover-supplied
// scalar [Cell] (the result) pinned, by a constraint, to equal the column's
// value at this position — Result == Column[Position].
//
// An opening is not a standalone query type. It is realised as:
//
//   - a lazily-assigned result [Cell] (see [Round.NewLazyCell]) living in the
//     same round as the column; at prove time it resolves to Column[Position]
//     by reading the runtime column assignment. Its extension flag inherits
//     from the column.
//   - a scalar [Vanishing] Result − Column[Position], registered on the
//     column's module. Because the expression is scalar (a single
//     [ColumnPosition] leaf plus the result [Cell]), it is discharged by the
//     local-vanishing compiler, which lifts it to a domain-wide identity via a
//     Lagrange indicator at Position. This is what soundly binds the opening
//     into the proof (and gives it a gnark path) — the cell value alone is not
//     trusted.
//
// The returned [Cell] is the opening value; read it back with
// [Runtime.GetCellValue]. The prover does not need to assign it explicitly.
//
// Panics if ctx or the receiver is nil.
func (cp *ColumnPosition) Open(ctx *ContextFrame) *Cell {
	if cp == nil {
		panic("wiop: ColumnPosition.Open requires a non-nil ColumnPosition")
	}
	if ctx == nil {
		panic("wiop: ColumnPosition.Open requires a non-nil ContextFrame")
	}

	col := cp.Column
	result := col.Round().NewLazyCell(
		ctx.Childf("result"),
		col.IsExtension,
		func(rt *Runtime) field.Gen {
			m := col.Module
			n := m.RuntimeSize(rt)
			// Negative Position values are resolved from the end at runtime;
			// see [ColumnPosition.Position]. This is what allows callers to
			// open the endpoint of a dynamic module via At(-1).
			return rt.GetColumnAssignment(col).ElementAtN(m.Padding, n, cp.resolvedRow(n))
		},
	)

	col.Module.NewVanishing(ctx, Sub(result, cp))
	return result
}
