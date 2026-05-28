package lookuptologderivsum

import (
	"fmt"
	"strings"

	"github.com/consensys/linea-monorepo/prover-ray/wiop"
)

// includedSpec captures one A-side fragment from a single Inclusion query:
// the list of column views that form the fragment together with an optional
// row selector. It is the prover-ray equivalent of the (S, sFilter) pair
// used by the linea/logderivativesum compiler.
type includedSpec struct {
	// query is the source [wiop.TableRelation] this fragment came from. The
	// compiler keeps it around to mark every contributing query as reduced
	// once the group has been compiled.
	query *wiop.LookupQuery
	// cols is the ordered list of columns forming the A fragment.
	cols []*wiop.ColumnView
	// selector is the optional A-side filter; nil means the fragment is fully
	// active.
	selector *wiop.ColumnView
}

// includingTable captures the lookup-table side (B fragment) of a group of
// Inclusion queries. Two queries with the same canonicalIncludingKey share an
// includingTable and therefore share a single multiplicity column M.
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

// canonicalIncludingKey returns a deterministic identity key for a B-side
// table fragment so that distinct Inclusion queries that target the same
// underlying lookup table can be grouped together.
//
// The key combines the underlying [wiop.Column] pointer addresses, the
// per-view shifting offsets, and the optional selector identity. Two
// fragments produce the same key iff every component matches; this matches
// the grouping semantics of the linea/logderivativesum compiler's
// NameTable-based grouping.
func canonicalIncludingKey(tab wiop.Table) string {
	var sb strings.Builder
	for _, cv := range tab.Columns {
		fmt.Fprintf(&sb, "%p@%d|", cv.Column, cv.ShiftingOffset)
	}
	sb.WriteByte(';')
	if tab.Selector != nil {
		fmt.Fprintf(&sb, "sel=%p@%d", tab.Selector.Column, tab.Selector.ShiftingOffset)
	} else {
		sb.WriteString("sel=nil")
	}
	return sb.String()
}

// lookupGroup collects every Inclusion query that targets the same B-side
// fragment. Within a group the compiler emits a single multiplicity column
// (per fragment) and a single fraction set.
type lookupGroup struct {
	including includingTable
	included  []includedSpec
	// witnessRound is the latest round across every column referenced by the
	// group's including and included fragments. M, α, γ live in
	// witnessRound + 1; the LogDerivativeSum result lives in
	// witnessRound + 2.
	witnessRound *wiop.Round
}

// addIncluded appends a single A-side fragment to the group, taken from the
// given query.
func (g *lookupGroup) addIncluded(q *wiop.LookupQuery, tab wiop.Table) {
	g.included = append(g.included, includedSpec{
		query:    q,
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
