package wiop

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
)

// TableRelationKind identifies the relational predicate asserted by a
// [TableRelation] query.
type TableRelationKind int

const (
	// TableRelationPermutation asserts that the multiset of rows in A equals
	// the multiset of rows in B. No selectors are permitted on either side.
	// The total row count of A and B must match; this is verified at runtime
	// by Check rather than at construction, because modules may be unsized
	// when the query is registered.
	//
	// To encode a fixed permutation B = S(A): construct the permuted-B side
	// as a precomputed column (apply S to A's concrete assignment offline) and
	// pass it directly to [System.NewPermutation].
	TableRelationPermutation TableRelationKind = iota
	// TableRelationInclusion asserts that every selected row of A[0] appears
	// in the union of selected rows across all B fragments.
	TableRelationInclusion
)

// String implements [fmt.Stringer].
func (k TableRelationKind) String() string {
	switch k {
	case TableRelationPermutation:
		return "Permutation"
	case TableRelationInclusion:
		return "Inclusion"
	default:
		return fmt.Sprintf("TableRelationKind(%d)", int(k))
	}
}

// Table is an ordered group of same-module column views with an optional row
// selector. A nil Selector is semantically equivalent to an all-ones column:
// every row is selected. Table is a value type and carries no identity.
//
// All Columns (and Selector, if non-nil) must reference columns belonging to
// the same module. This invariant is enforced at construction by [NewTable]
// and [NewFilteredTable].
type Table struct {
	// Columns is the ordered list of column views forming the table. Contains
	// at least one entry.
	Columns []*ColumnView
	// Selector is an optional binary column marking which rows participate in
	// the relation (1 = selected, 0 = skipped). Nil means all rows are selected.
	Selector *ColumnView
}

// NewTable constructs an unfiltered Table (nil Selector) from the given column
// views. All columns must belong to the same module.
//
// Panics if columns is empty or if they do not all share a module.
func NewTable(columns ...*ColumnView) Table {
	return newTable(nil, columns)
}

// NewFilteredTable constructs a filtered Table with the given selector and
// column views. All columns and the selector must belong to the same module.
//
// Panics if columns is empty, selector is nil, or the columns and selector do
// not all share a module.
func NewFilteredTable(selector *ColumnView, columns ...*ColumnView) Table {
	if selector == nil {
		panic("wiop: NewFilteredTable requires a non-nil selector; use NewTable for unfiltered tables")
	}
	return newTable(selector, columns)
}

// newTable is the shared constructor used by [NewTable] and [NewFilteredTable].
// It validates the module-consistency invariant and builds the Table value.
func newTable(selector *ColumnView, columns []*ColumnView) Table {
	if len(columns) == 0 {
		panic("wiop: Table requires at least one column")
	}
	m := columns[0].Module()
	for i, cv := range columns[1:] {
		if cv.Module() != m {
			panic(fmt.Sprintf(
				"wiop: Table column [%d] belongs to module %q but column [0] belongs to module %q; all columns must share a module",
				i+1, cv.Module().Context.Path(), m.Context.Path(),
			))
		}
	}
	if selector != nil && selector.Module() != m {
		//nolint
		panic(fmt.Sprintf(
			"wiop: Table selector belongs to module %q but columns belong to module %q; selector and columns must share a module",
			selector.Module().Context.Path(), m.Context.Path(),
		))
	}
	return Table{Columns: columns, Selector: selector}
}

// Module returns the module shared by all columns in this Table. It is always
// non-nil for a well-formed Table.
func (t Table) Module() *Module { return t.Columns[0].Module() }

// Round returns the latest [Round] among all columns (and the Selector, if
// non-nil) in this Table. Returns nil only for a zero-value Table.
func (t Table) Round() *Round {
	var best *Round
	updateBest := func(r *Round) {
		if r != nil && (best == nil || r.ID > best.ID) {
			best = r
		}
	}
	for _, cv := range t.Columns {
		updateBest(cv.Round())
	}
	if t.Selector != nil {
		updateBest(t.Selector.Round())
	}
	return best
}

// Width returns the number of columns in this Table.
func (t Table) Width() int { return len(t.Columns) }

// TableRelation is a [Query] asserting a relational predicate between two
// ordered lists of table fragments (A and B). The predicate semantics are
// controlled by [TableRelation.Kind]:
//
//   - Permutation: A and B, treated as multisets of rows, are equal. No
//     selectors are permitted.
//   - Inclusion: every selected row of A appears in the union of selected
//     rows across all B fragments.
//
// TableRelation does not implement [GnarkCheckableQuery]: neither predicate
// can be verified inside a gnark circuit. A compiler pass must reduce them
// before gnark verification.
type TableRelation struct {
	baseQuery
	// Kind identifies the relational predicate being asserted.
	Kind TableRelationKind
	// A is the left-hand side of the relation.
	A []Table
	// B is the right-hand side of the relation.
	B []Table
}

// Round implements [Query]. Returns the latest [Round] across all columns in
// A and B, including selectors.
func (tr *TableRelation) Round() *Round {
	var best *Round
	for _, tables := range [2][]Table{tr.A, tr.B} {
		for _, tab := range tables {
			r := tab.Round()
			if r != nil && (best == nil || r.ID > best.ID) {
				best = r
			}
		}
	}
	return best
}

// Check implements [Query]. Dispatches to [checkPermutation] or
// [checkInclusion] depending on [TableRelation.Kind].
func (tr *TableRelation) Check(rt Runtime) error {
	switch tr.Kind {
	case TableRelationPermutation:
		return tr.checkPermutation(rt)
	case TableRelationInclusion:
		return tr.checkInclusion(rt)
	default:
		return fmt.Errorf("wiop: TableRelation(%s).Check: unknown kind %v", tr.context.Path(), tr.Kind)
	}
}

// checkPermutation verifies that A and B are equal as multisets of rows.
//
// This is a probabilistic check: a random extension-field scalar alpha is
// sampled, and each row is hashed to a single field element via Horner's rule.
// A collision (two distinct rows mapping to the same hash) causes a false
// negative with probability at most (total rows) / |field|, which is
// negligible for realistic table sizes. A is used to populate a counter map;
// B decrements it. Any non-zero counter at the end signals a mismatch.
//
// When all column views in a table have zero shift and the module has
// directional padding, identical padding rows are batch-added/-subtracted as
// a single count entry rather than being iterated row-by-row.
func (tr *TableRelation) checkPermutation(rt Runtime) error {
	alpha := field.RandomElemExt()
	counts := make(map[field.Ext]int)
	for _, tab := range tr.A {
		permTableCountInto(counts, 1, alpha, rt, tab)
	}
	for _, tab := range tr.B {
		permTableCountInto(counts, -1, alpha, rt, tab)
	}
	for _, c := range counts {
		if c != 0 {
			return fmt.Errorf(
				"wiop: TableRelation(%s).Check: Permutation multiset mismatch",
				tr.context.Path(),
			)
		}
	}
	return nil
}

// permTableCountInto adds sign*(1 per row) into counts for every row in tab.
// When all column views have zero shift and the module has directional padding,
// the gap identical padding rows are registered as a single entry with count
// sign*gap instead of iterating them individually.
func permTableCountInto(counts map[field.Ext]int, sign int, alpha field.Gen, rt Runtime, tab Table) {
	n := tab.Module().Size()
	m := tab.Module()

	if m.Padding == PaddingDirectionNone || !tableHasZeroShift(tab) {
		for row := range n {
			counts[tableRowHash(alpha, rt, tab.Columns, row, n)] += sign
		}
		return
	}

	plainLen := rt.GetColumnAssignment(tab.Columns[0].Column).Plain[0].Len()
	gap := n - plainLen
	var dataStart int
	if m.Padding == PaddingDirectionLeft {
		dataStart = gap
	}

	if gap > 0 {
		// All padding rows produce the same per-column values, so their row
		// hashes are equal. Use the anchor row as a representative.
		anchor := padAnchorRow(m.Padding, plainLen)
		counts[tableRowHash(alpha, rt, tab.Columns, anchor, n)] += sign * gap
	}
	for row := dataStart; row < dataStart+plainLen; row++ {
		counts[tableRowHash(alpha, rt, tab.Columns, row, n)] += sign
	}
}

// checkInclusion verifies that every selected row of A appears in the union of
// selected rows across all B fragments.
//
// This is a probabilistic check: a random extension-field scalar alpha is
// sampled and used to hash rows via Horner's rule. A hash collision causes a
// false negative with probability at most (total rows) / |field|, which is
// negligible for realistic table sizes. B's selected rows populate a set; each
// selected A row is then probed against it.
//
// When all column views and the selector in a table have zero shift and the
// module has directional padding, all padding rows produce the same row hash
// and the same selector value. Rather than iterating the gap identical padding
// rows, the first padding row (the anchor) is probed once: if selected and
// absent from B, the check fails immediately; if present, every other selected
// padding row is also satisfied.
func (tr *TableRelation) checkInclusion(rt Runtime) error {
	alpha := field.RandomElemExt()
	bSet := make(map[field.Ext]struct{})
	for _, tab := range tr.B {
		inclusionBuildSet(bSet, alpha, rt, tab)
	}
	for _, tab := range tr.A {
		if err := inclusionCheckSet(bSet, alpha, rt, tab, tr.context.Path()); err != nil {
			return err
		}
	}
	return nil
}

// inclusionBuildSet adds the hashes of all selected rows of tab to bSet.
// Padding rows are handled with a single anchor probe when applicable.
func inclusionBuildSet(bSet map[field.Ext]struct{}, alpha field.Gen, rt Runtime, tab Table) {
	n := tab.Module().Size()
	m := tab.Module()

	if m.Padding == PaddingDirectionNone || !tableHasZeroShift(tab) {
		for row := range n {
			if tab.Selector != nil {
				if sel := tableElemAt(rt, tab.Selector, row, n); sel.IsZero() {
					continue
				}
			}
			bSet[tableRowHash(alpha, rt, tab.Columns, row, n)] = struct{}{}
		}
		return
	}

	plainLen := rt.GetColumnAssignment(tab.Columns[0].Column).Plain[0].Len()
	gap := n - plainLen
	var dataStart int
	if m.Padding == PaddingDirectionLeft {
		dataStart = gap
	}

	if gap > 0 {
		// All padding rows share the same selector value and row hash.
		// Probe the anchor once and add the hash at most once.
		anchor := padAnchorRow(m.Padding, plainLen)
		paddingSelected := tab.Selector == nil
		if tab.Selector != nil {
			sel := tableElemAt(rt, tab.Selector, anchor, n)
			paddingSelected = !sel.IsZero()
		}
		if paddingSelected {
			bSet[tableRowHash(alpha, rt, tab.Columns, anchor, n)] = struct{}{}
		}
	}
	for row := dataStart; row < dataStart+plainLen; row++ {
		if tab.Selector != nil {
			if sel := tableElemAt(rt, tab.Selector, row, n); sel.IsZero() {
				continue
			}
		}
		bSet[tableRowHash(alpha, rt, tab.Columns, row, n)] = struct{}{}
	}
}

// inclusionCheckSet verifies that all selected rows of tab are present in bSet.
// Padding rows are checked with a single anchor probe when applicable.
func inclusionCheckSet(bSet map[field.Ext]struct{}, alpha field.Gen, rt Runtime, tab Table, path string) error {
	n := tab.Module().Size()
	m := tab.Module()

	if m.Padding == PaddingDirectionNone || !tableHasZeroShift(tab) {
		for row := range n {
			if tab.Selector != nil {
				if sel := tableElemAt(rt, tab.Selector, row, n); sel.IsZero() {
					continue
				}
			}
			if _, ok := bSet[tableRowHash(alpha, rt, tab.Columns, row, n)]; !ok {
				return fmt.Errorf(
					"wiop: TableRelation(%s).Check: Inclusion failed: a row from A is absent from B",
					path,
				)
			}
		}
		return nil
	}

	plainLen := rt.GetColumnAssignment(tab.Columns[0].Column).Plain[0].Len()
	gap := n - plainLen
	var dataStart int
	if m.Padding == PaddingDirectionLeft {
		dataStart = gap
	}

	if gap > 0 {
		// If the anchor padding row is selected and absent from B, all other
		// selected padding rows would also fail — check once and return early.
		anchor := padAnchorRow(m.Padding, plainLen)
		paddingSelected := tab.Selector == nil
		if tab.Selector != nil {
			sel := tableElemAt(rt, tab.Selector, anchor, n)
			paddingSelected = !sel.IsZero()
		}
		if paddingSelected {
			if _, ok := bSet[tableRowHash(alpha, rt, tab.Columns, anchor, n)]; !ok {
				return fmt.Errorf(
					"wiop: TableRelation(%s).Check: Inclusion failed: a row from A is absent from B",
					path,
				)
			}
		}
	}
	for row := dataStart; row < dataStart+plainLen; row++ {
		if tab.Selector != nil {
			if sel := tableElemAt(rt, tab.Selector, row, n); sel.IsZero() {
				continue
			}
		}
		if _, ok := bSet[tableRowHash(alpha, rt, tab.Columns, row, n)]; !ok {
			return fmt.Errorf(
				"wiop: TableRelation(%s).Check: Inclusion failed: a row from A is absent from B",
				path,
			)
		}
	}
	return nil
}

// tableRowHash computes a Horner linear combination of all column values at
// logical row idx, using alpha as the mixing scalar. Returns the raw [field.Ext]
// value for use as a map key.
func tableRowHash(alpha field.Gen, rt Runtime, cols []*ColumnView, idx, n int) field.Ext {
	var acc field.Gen
	for _, cv := range cols {
		acc = acc.Mul(alpha).Add(tableElemAt(rt, cv, idx, n))
	}
	return acc.Ext
}

// tableElemAt returns the field element at logical row idx in cv's concrete
// assignment, applying the cyclic shift and the module's padding semantics.
// n is the module size.
func tableElemAt(rt Runtime, cv *ColumnView, idx, n int) field.Gen {
	phys := ((idx+cv.ShiftingOffset)%n + n) % n
	return rt.GetColumnAssignment(cv.Column).ElementAt(cv.Column.Module, phys)
}

// tableHasZeroShift reports whether all column views and the selector (if
// present) in tab have ShiftingOffset == 0. This is the precondition for the
// padding-row batching optimisations in permutation and inclusion checks.
func tableHasZeroShift(tab Table) bool {
	for _, cv := range tab.Columns {
		if cv.ShiftingOffset != 0 {
			return false
		}
	}
	return tab.Selector == nil || tab.Selector.ShiftingOffset == 0
}

// padAnchorRow returns the index of the first padding row, used as a
// representative of all identical padding rows when all shifts are zero.
//   - PaddingDirectionLeft:  padding occupies [0, dataStart); anchor is 0.
//   - PaddingDirectionRight: padding occupies [plainLen, n); anchor is plainLen.
func padAnchorRow(pd PaddingDirection, plainLen int) int {
	if pd == PaddingDirectionLeft {
		return 0
	}
	return plainLen
}

// NewPermutation constructs and registers a Permutation [TableRelation] on sys.
// The query asserts that A and B, as multisets of rows, are equal.
//
// Invariants enforced at construction:
//   - A and B are non-empty.
//   - No Table in A or B carries a Selector.
//   - All fragments across A and B have the same column width.
//
// The total row count equality (Σrows(A) == Σrows(B)) cannot be checked at
// construction time because modules may be unsized; it is deferred to Check.
//
// To encode a fixed permutation B = S(A): construct the permuted-B columns as
// precomputed columns (applying S to A's concrete assignment at setup time) and
// pass them here as the B side.
//
// Panics on any of the above invariant violations.
func (sys *System) NewPermutation(ctx *ContextFrame, A, B []Table) *TableRelation {
	if ctx == nil {
		panic("wiop: System.NewPermutation requires a non-nil ContextFrame")
	}
	validateNonEmpty("NewPermutation", "A", A)
	validateNonEmpty("NewPermutation", "B", B)
	validateNoSelectors("NewPermutation", A)
	validateNoSelectors("NewPermutation", B)
	width := A[0].Width()
	validateUniformWidth("NewPermutation", width, A)
	validateUniformWidth("NewPermutation", width, B)
	return sys.newTableRelation(ctx, TableRelationPermutation, A, B)
}

// NewInclusion constructs and registers an Inclusion [TableRelation] on sys.
// The query asserts that every selected row of included appears in the union
// of selected rows across all including fragments.
//
// Invariants enforced at construction:
//   - included is non-empty.
//   - including is non-empty.
//   - All including fragments have the same column width as included.
//
// Panics on any of the above invariant violations or if ctx is nil.
func (sys *System) NewInclusion(ctx *ContextFrame, included []Table, including []Table) *TableRelation {
	if ctx == nil {
		panic("wiop: System.NewInclusion requires a non-nil ContextFrame")
	}
	validateNonEmpty("NewInclusion", "included", included)
	validateNonEmpty("NewInclusion", "including", including)
	validateUniformWidth("NewInclusion/included-same-width", included[0].Width(), including)
	validateUniformWidth("NewInclusion/including-same-width", included[0].Width(), included)
	return sys.newTableRelation(ctx, TableRelationInclusion, included, including)
}

// newTableRelation is the shared registration step used by all TableRelation
// constructors. It builds the struct, appends it to sys.TableRelations, and
// returns it.
func (sys *System) newTableRelation(ctx *ContextFrame, kind TableRelationKind, A, B []Table) *TableRelation {
	tr := &TableRelation{
		baseQuery: baseQuery{
			context:     ctx,
			Annotations: make(Annotations),
		},
		Kind: kind,
		A:    A,
		B:    B,
	}
	sys.TableRelations = append(sys.TableRelations, tr)
	return tr
}

// validateNonEmpty panics if tables is empty.
func validateNonEmpty(caller, side string, tables []Table) {
	if len(tables) == 0 {
		panic(fmt.Sprintf("wiop: System.%s: %s must have at least one fragment", caller, side))
	}
}

// validateNoSelectors panics if any Table in tables carries a non-nil Selector.
func validateNoSelectors(caller string, tables []Table) {
	for i, t := range tables {
		if t.Selector != nil {
			panic(fmt.Sprintf(
				"wiop: System.%s: fragment %d carries a Selector; Permutation does not support selectors",
				caller, i,
			))
		}
	}
}

// validateUniformWidth panics if any Table in tables has a Width different from
// expectedWidth.
func validateUniformWidth(caller string, expectedWidth int, tables []Table) {
	for i, t := range tables {
		if t.Width() != expectedWidth {
			panic(fmt.Sprintf(
				"wiop: System.%s: fragment %d has width %d but expected %d; all fragments must have the same column width",
				caller, i, t.Width(), expectedWidth,
			))
		}
	}
}
