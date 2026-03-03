package distributed

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/sirupsen/logrus"
)

// AssignManuallyShiftedColumn implements [wizard.ProverAction] and
// is responsible for assigning the manually shifted columns with the right values at the right round.
type AssignManualShifts struct {
	ManualShifts []*dedicated.ManuallyShifted
}

// Run implements [wizard.ProverAction] interface
func (a *AssignManualShifts) Run(runtime *wizard.ProverRuntime) {
	for i := range a.ManualShifts {
		a.ManualShifts[i].GetColAssignment(runtime)
	}
}

// shiftCacheKey deduplicates ManuallyShifted columns by (base column, offset).
type shiftCacheKey struct {
	colID  ifaces.ColID
	offset int
}

// getOrCreateManuallyShifted returns a cached ManuallyShifted column or creates
// a new one. It unwraps any Shifted wrapper via column.RootParents so that
// ManuallyShift always receives a Natural column, preventing double-shift bugs.
func getOrCreateManuallyShifted(comp *wizard.CompiledIOP, col ifaces.Column, offset int, cache map[shiftCacheKey]*dedicated.ManuallyShifted) *dedicated.ManuallyShifted {
	root := column.RootParents(col) // unwrap Shifted → Natural
	key := shiftCacheKey{root.GetColID(), offset}
	if ms, ok := cache[key]; ok {
		return ms
	}
	ms := dedicated.ManuallyShift(comp, root, offset, fmt.Sprintf("MANUALLY_SHIFTED_%v", root.GetColID()))
	cache[key] = ms
	return ms
}

/*
compileManualShifter creates a small compiler job iterating on every queries that are not ignored.
- for lookup, permutation, projection: whenever we meet a shifted column, we mark the query as ignored
- we create a replacement column for every problematic columns involved in the query, i.e., a shifted column
- we create a new equivalent query, replacing the shifted columns by the manually shifted ones
- we schedule the assignment of the manually shifted columns for the right round
*/
func compileManualShifter(comp *wizard.CompiledIOP) {
	logrus.Info("compiling manual shifter")
	defer logrus.Info("finished compiling manual shifter")

	cache := make(map[shiftCacheKey]*dedicated.ManuallyShifted)

	for roundId := 0; roundId < comp.NumRounds(); roundId++ {
		for _, qName := range comp.QueriesNoParams.AllKeysAt(roundId) {
			if comp.QueriesNoParams.IsIgnored(qName) {
				continue
			}
			q := comp.QueriesNoParams.Data(qName)

			switch q := q.(type) {
			case query.Inclusion:
				if len(q.GetShiftedRelatedColumns()) > 0 {
					replayQueryWithManualShiftInclusion(comp, qName, len(q.GetShiftedRelatedColumns()), cache)
					continue
				}
				continue
			case query.Permutation:
				if len(q.GetShiftedRelatedColumns()) > 0 {
					replayQueryWithManualShiftPermutation(comp, qName, len(q.GetShiftedRelatedColumns()), cache)
					continue
				}
				continue
			case query.Projection:
				if len(q.GetShiftedRelatedColumns()) > 0 {
					replayQueryWithManualShiftProjection(comp, qName, len(q.GetShiftedRelatedColumns()), cache)
					continue
				}
				continue
			default:
				continue
			}
		}
	}
}

// replayQueryWithManualShiftInclusion creates a new inclusion query with the same semantics as the original one, but replacing the shifted columns by manually shifted ones. It also marks the original query as ignored and schedule the assignment of the manually shifted columns at the right round.
func replayQueryWithManualShiftInclusion(comp *wizard.CompiledIOP, qName ifaces.QueryID, numShift int, cache map[shiftCacheKey]*dedicated.ManuallyShifted) {
	// sanity check: make sure it is an inclusion query
	q, ok := comp.QueriesNoParams.Data(qName).(query.Inclusion)
	if !ok {
		panic("replayQueryWithManualShiftInclusion called on a non-inclusion query")
	}

	shiftedCols := make([]*dedicated.ManuallyShifted, 0, numShift)
	newID := ifaces.QueryIDf("%v_WITH_MANUALLY_SHIFTED_COL", q.ID)

	// Build replacement includedFilter
	newIncludedFilter := q.IncludedFilter
	if q.IncludedFilter != nil && q.IncludedFilter.IsComposite() {
		if col, ok := q.IncludedFilter.(column.Shifted); ok {
			ms := getOrCreateManuallyShifted(comp, q.IncludedFilter, col.Offset, cache)
			shiftedCols = append(shiftedCols, ms)
			newIncludedFilter = ms.Natural
		}
	}

	// Build replacement included columns
	newIncluded := make([]ifaces.Column, len(q.Included))
	for i, included := range q.Included {
		newIncluded[i] = included
		if included.IsComposite() {
			if col, ok := included.(column.Shifted); ok {
				ms := getOrCreateManuallyShifted(comp, included, col.Offset, cache)
				shiftedCols = append(shiftedCols, ms)
				newIncluded[i] = ms.Natural
			}
		}
	}

	// Build replacement includingFilter and including columns
	var newIncludingFilter []ifaces.Column
	if q.IncludingFilter != nil {
		newIncludingFilter = make([]ifaces.Column, len(q.IncludingFilter))
	}
	newIncluding := make([][]ifaces.Column, len(q.Including))
	for frag := range q.Including {
		if q.IncludingFilter != nil && q.IncludingFilter[frag] != nil {
			newIncludingFilter[frag] = q.IncludingFilter[frag]
			if q.IncludingFilter[frag].IsComposite() {
				if col, ok := q.IncludingFilter[frag].(column.Shifted); ok {
					ms := getOrCreateManuallyShifted(comp, q.IncludingFilter[frag], col.Offset, cache)
					shiftedCols = append(shiftedCols, ms)
					newIncludingFilter[frag] = ms.Natural
				}
			}
		}

		newIncluding[frag] = make([]ifaces.Column, len(q.Including[frag]))
		for i, including := range q.Including[frag] {
			newIncluding[frag][i] = including
			if including.IsComposite() {
				if col, ok := including.(column.Shifted); ok {
					ms := getOrCreateManuallyShifted(comp, including, col.Offset, cache)
					shiftedCols = append(shiftedCols, ms)
					newIncluding[frag][i] = ms.Natural
				}
			}
		}
	}

	// Use constructor so the query gets a proper UUID (required for serialization dedup)
	newQ := query.NewInclusion(newID, newIncluded, newIncluding, newIncludedFilter, newIncludingFilter)

	comp.QueriesNoParams.AddToRound(q.Included[0].Round(), newQ.ID, newQ)
	comp.RegisterProverAction(q.Included[0].Round(),
		&AssignManualShifts{ManualShifts: shiftedCols})
	comp.QueriesNoParams.MarkAsIgnored(qName)
}

// replayQueryWithManualShiftPermutation creates a new permutation query with the same semantics as the original one, but replacing the shifted columns by manually shifted ones. It also marks the original query as ignored and schedule the assignment of the manually shifted columns at the right round.
func replayQueryWithManualShiftPermutation(comp *wizard.CompiledIOP, qName ifaces.QueryID, numShift int, cache map[shiftCacheKey]*dedicated.ManuallyShifted) {
	// sanity check: make sure it is a permutation query
	q, ok := comp.QueriesNoParams.Data(qName).(query.Permutation)
	if !ok {
		panic("replayQueryWithManualShiftPermutation called on a non-permutation query")
	}

	shiftedCols := make([]*dedicated.ManuallyShifted, 0, numShift)
	newID := ifaces.QueryIDf("%v_WITH_MANUALLY_SHIFTED_COL", q.ID)

	// Build replacement A columns
	newA := make([][]ifaces.Column, len(q.A))
	for frag := range q.A {
		newA[frag] = make([]ifaces.Column, len(q.A[frag]))
		for i, col := range q.A[frag] {
			newA[frag][i] = col
			if col.IsComposite() {
				if col, ok := col.(column.Shifted); ok {
					ms := getOrCreateManuallyShifted(comp, col, col.Offset, cache)
					shiftedCols = append(shiftedCols, ms)
					newA[frag][i] = ms.Natural
				}
			}
		}
	}

	// Build replacement B columns
	newB := make([][]ifaces.Column, len(q.B))
	for frag := range q.B {
		newB[frag] = make([]ifaces.Column, len(q.B[frag]))
		for i, col := range q.B[frag] {
			newB[frag][i] = col
			if col.IsComposite() {
				if col, ok := col.(column.Shifted); ok {
					ms := getOrCreateManuallyShifted(comp, col, col.Offset, cache)
					shiftedCols = append(shiftedCols, ms)
					newB[frag][i] = ms.Natural
				}
			}
		}
	}

	// Use constructor so the query gets a proper UUID (required for serialization dedup)
	newQ := query.NewPermutation(newID, newA, newB)

	comp.QueriesNoParams.AddToRound(q.A[0][0].Round(), newQ.ID, newQ)
	comp.RegisterProverAction(q.A[0][0].Round(),
		&AssignManualShifts{ManualShifts: shiftedCols})
	comp.QueriesNoParams.MarkAsIgnored(q.ID)
}

// replayQueryWithManualShiftProjection creates a new projection query with the same semantics as the original one, but replacing the shifted columns by manually shifted ones. It also marks the original query as ignored and schedule the assignment of the manually shifted columns at the right round.
func replayQueryWithManualShiftProjection(comp *wizard.CompiledIOP, qName ifaces.QueryID, numShift int, cache map[shiftCacheKey]*dedicated.ManuallyShifted) {
	// sanity check: make sure it is a projection query
	q, ok := comp.QueriesNoParams.Data(qName).(query.Projection)
	if !ok {
		panic("replayQueryWithManualShiftProjection called on a non-projection query")
	}

	shiftedCols := make([]*dedicated.ManuallyShifted, 0, numShift)
	newID := ifaces.QueryIDf("%v_WITH_MANUALLY_SHIFTED_COL", q.ID)

	// Helper to replace shifted columns in a 2D column slice
	replaceShifted2D := func(cols [][]ifaces.Column) [][]ifaces.Column {
		out := make([][]ifaces.Column, len(cols))
		for i := range cols {
			out[i] = make([]ifaces.Column, len(cols[i]))
			for j, col := range cols[i] {
				out[i][j] = col
				if col.IsComposite() {
					if col, ok := col.(column.Shifted); ok {
						ms := getOrCreateManuallyShifted(comp, col, col.Offset, cache)
						shiftedCols = append(shiftedCols, ms)
						out[i][j] = ms.Natural
					}
				}
			}
		}
		return out
	}

	// Helper to replace shifted columns in a 1D filter slice
	replaceShiftedFilters := func(filters []ifaces.Column) []ifaces.Column {
		if filters == nil {
			return nil
		}
		out := make([]ifaces.Column, len(filters))
		for i, f := range filters {
			out[i] = f
			if f.IsComposite() {
				if col, ok := f.(column.Shifted); ok {
					ms := getOrCreateManuallyShifted(comp, f, col.Offset, cache)
					shiftedCols = append(shiftedCols, ms)
					out[i] = ms.Natural
				}
			}
		}
		return out
	}

	inp := query.ProjectionMultiAryInput{
		FiltersA: replaceShiftedFilters(q.Inp.FiltersA),
		FiltersB: replaceShiftedFilters(q.Inp.FiltersB),
		ColumnsA: replaceShifted2D(q.Inp.ColumnsA),
		ColumnsB: replaceShifted2D(q.Inp.ColumnsB),
	}

	// Use constructor so the query gets a proper UUID (required for serialization dedup)
	newQ := query.NewProjectionMultiAry(q.Round, newID, inp)

	comp.QueriesNoParams.AddToRound(q.Round, newQ.ID, newQ)
	comp.RegisterProverAction(q.Round,
		&AssignManualShifts{ManualShifts: shiftedCols})
	comp.QueriesNoParams.MarkAsIgnored(qName)
}
