package lookuptologderivsum

import (
	"fmt"
	"strings"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// includedSpec captures one A-side fragment from a single Inclusion query:
// the list of column views that form the fragment together with an optional
// row selector. It is the prover-ray equivalent of the (S, sFilter) pair
// used by the linea/logderivativesum compiler.
type includedSpec struct {
	// cols is the ordered list of columns forming the A fragment.
	cols []*wiop.ColumnView
	// selector is the optional A-side filter; nil means the fragment is fully
	// active.
	selector *wiop.ColumnView
}

// includingTable captures one lookup-table fragment (B fragment) of a group of
// Inclusion queries. Two queries with the same canonicalIncludingKeyMulti
// share the group's includingTables and therefore share its per-fragment
// multiplicity columns M.
type includingTable struct {
	// cols is the ordered list of columns forming the B fragment.
	cols []*wiop.ColumnView
	// selector is the optional B-side filter; nil means the table fragment
	// has no filter.
	selector *wiop.ColumnView
	// module is the module owning every column in cols (and the selector, if
	// any).
	module *wiop.Module
}

// width reports the number of columns in the lookup-table fragment.
func (t includingTable) width() int { return len(t.cols) }

// canonicalIncludingKeyMulti returns a deterministic identity key for a whole
// B side, i.e. the ordered union of lookup-table fragments of a single
// Inclusion query. Two queries that target the same ordered set of fragments
// (each with the same columns, shifts and selector) produce the same key and
// therefore share the group's per-fragment multiplicity columns.
//
// The fragment order is significant: it fixes the fragment index each M column
// is bound to, so [F1,F2] and [F2,F1] are deliberately distinct keys. This
// mirrors linea/logderivativesum, where NameTable concatenates the fragments
// of a lookup table in their declaration order.
func canonicalIncludingKeyMulti(tables []wiop.Table) string {
	var sb strings.Builder
	for _, tab := range tables {
		writeFragmentKey(&sb, tab)
		sb.WriteByte('#') // fragment separator
	}
	return sb.String()
}

// writeFragmentKey appends the canonical identity of a single fragment to sb:
// the underlying [wiop.Column] pointer addresses, the per-view shifting
// offsets, and the optional selector identity. Two fragments emit the same
// bytes iff every component matches, mirroring the grouping semantics of the
// linea/logderivativesum compiler's NameTable-based grouping.
func writeFragmentKey(sb *strings.Builder, tab wiop.Table) {
	for _, cv := range tab.Columns {
		fmt.Fprintf(sb, "%p@%d|", cv.Column, cv.ShiftingOffset)
	}
	sb.WriteByte(';')
	if tab.Selector != nil {
		fmt.Fprintf(sb, "sel=%p@%d", tab.Selector.Column, tab.Selector.ShiftingOffset)
	} else {
		sb.WriteString("sel=nil")
	}
}

// lookupGroup collects every Inclusion query that targets the same B side
// (the same ordered union of lookup-table fragments). Within a group the
// compiler emits one multiplicity column per B fragment and a single fraction
// set (one −M/(γ+RLC) term per B fragment plus one term per A fragment).
type lookupGroup struct {
	// includings holds the group's B-side fragments in declaration order. The
	// slice index is the fragment index that each M column is bound to.
	includings []includingTable
	included   []includedSpec
	// queries lists the source Inclusion queries folded into this subgroup, in
	// declaration order and without duplicates. The compiler uses it to
	// precheck each query's row limit, to build error messages, and to mark
	// every contributing query as reduced once the group has been compiled.
	queries []*wiop.TableRelationQuery
	// witnessRound is the latest round across every column referenced by the
	// group's including and included fragments. M, α, γ live in
	// witnessRound + 1; the LogDerivativeSum result lives in
	// witnessRound + 2.
	witnessRound *wiop.Round
}

// addIncluded appends a single A-side fragment to the group.
func (g *lookupGroup) addIncluded(tab wiop.Table) {
	g.included = append(g.included, includedSpec{
		cols:     tab.Columns,
		selector: tab.Selector,
	})
}

// updateWitnessRound bumps the recorded witness round if r is later than the
// currently recorded one. A nil r is a no-op.
//
// The PrecomputedRound is intentionally skipped: precomputed columns are
// available "at every round" so they don't constrain when the M column must
// be committed. If they did, the M column would end up registered against
// PrecomputedRound (whose [Round.Columns] is expected to track ONLY
// precomputed columns paired with parallel entries in
// [PrecomputedRound.PrecomputedValues]), which corrupts the precomputed-round
// invariant and crashes [wiop.NewRuntime]. Callers that touch only
// precomputed columns are left with a nil witnessRound; the compiler
// defaults to the first interactive round in that case (see [Compile]).
func (g *lookupGroup) updateWitnessRound(r *wiop.Round) {
	if r == nil {
		return
	}
	if isPrecomputedRound(r) {
		return
	}
	if g.witnessRound == nil || r.ID > g.witnessRound.ID {
		g.witnessRound = r
	}
}

// isPrecomputedRound reports whether r is the PrecomputedRound of its
// owning system. Precomputed columns return a [*wiop.Round] that points at
// the embedded Round field of [wiop.PrecomputedRound], so identity is the
// reliable check.
func isPrecomputedRound(r *wiop.Round) bool {
	if r == nil || r.System() == nil {
		return false
	}
	return r == &r.System().PrecomputedRound.Round
}

// allIncludingColumnsShareModule reports whether every column in the
// including-side fragment (plus its optional selector) lives on the same
// module. This is already enforced by [wiop.NewTable] / [wiop.NewFilteredTable],
// but the compiler re-asserts it as a defensive check.
func allIncludingColumnsShareModule(tab includingTable) bool {
	for _, cv := range tab.cols {
		if cv.Column.Module != tab.module {
			return false
		}
	}
	if tab.selector != nil && tab.selector.Column.Module != tab.module {
		return false
	}
	return true
}
