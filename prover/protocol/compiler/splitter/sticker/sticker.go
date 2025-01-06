package sticker

import (
	"fmt"
	"strings"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/variables"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/collection"
	"github.com/consensys/linea-monorepo/prover/utils/profiling"
)

// Stick columns by size
func Sticker(minSize, maxSize int) func(comp *wizard.CompiledIOP) {

	return func(comp *wizard.CompiledIOP) {
		// initialize the context
		ctx := newStickContext(comp, minSize, maxSize)
		// collect all the columns
		ctx.collectColums()
		// group them each into groups of columns
		ctx.createMapToNew()
		ctx.compileFixedEvaluation()
		ctx.compileArithmeticConstraints()

		comp.SubProvers.AppendToInner(comp.NumRounds()-1, func(run *wizard.ProverRuntime) {
			for _, compRound := range ctx.CompiledColumns {
				for _, list := range compRound.BySize {
					for _, h := range list {
						run.Columns.TryDel(h.GetColID())
					}
				}
			}
		})
	}
}

// Context of compilation containing all the intermediate
// values informations. The scope of the context is a single
// round of the protocol.
type stickContext struct {
	// The compiled IOP
	comp *wizard.CompiledIOP
	// All columns under the minSize are ignored
	MinSize, MaxSize int

	// Collects all the informations about the committed
	// columns
	CompiledColumns []struct {
		BySize map[int][]ifaces.Column
		ByNew  map[ifaces.ColID][]ifaces.Column
	}

	// Collects all informations relative to the
	// Map a column to its new embedding column
	News []struct {
		List     []ifaces.Column
		BySubCol map[ifaces.ColID]struct {
			NameNew  ifaces.ColID
			PosInNew int
		}
	}
}

// NewStickContext initializes a new context
func newStickContext(comp *wizard.CompiledIOP, minSize, maxSize int) stickContext {
	numRounds := comp.NumRounds()
	res := stickContext{
		comp:    comp,
		MinSize: minSize,
		MaxSize: maxSize,
		// initialize the slices with the final size directly
		CompiledColumns: make([]struct {
			BySize map[int][]ifaces.Column
			ByNew  map[ifaces.ColID][]ifaces.Column
		}, numRounds),
		News: make([]struct {
			List     []ifaces.Column
			BySubCol map[ifaces.ColID]struct {
				NameNew  ifaces.ColID
				PosInNew int
			}
		}, numRounds),
	}
	return res
}

// Collect all the columns to be compiled
func (ctx *stickContext) collectColums() {

	for round := 0; round < ctx.comp.NumRounds(); round++ {
		sizes := map[int][]ifaces.Column{}

		for _, colName := range ctx.comp.Columns.AllKeysAt(round) {

			status := ctx.comp.Columns.Status(colName)
			col := ctx.comp.Columns.GetHandle(colName)

			// If the column is ignored, we can just skip it. Also
			// if is is public we can as well ignore it. @alex It would
			// be nonetheless interesting to be able to differentiate
			// between proof objects from other "rounds of selfrecursion"
			// from proofs objects from the current round.
			if status == column.Ignored || status == column.Proof || status == column.VerifyingKey || status == column.Precomputed {
				continue
			}

			// If the sizes are either too small or too large, we ignore them
			if ctx.MinSize > col.Size() || col.Size() >= ctx.MaxSize {
				continue
			}

			// Mark it as ignored, so that it is no longer considered as
			// queryable.
			ctx.comp.Columns.MarkAsIgnored(colName)

			// Initialization clause of `sizes`
			if _, ok := sizes[col.Size()]; !ok {
				sizes[col.Size()] = []ifaces.Column{}
			}

			sizes[col.Size()] = append(sizes[col.Size()], col)
		}

		ctx.CompiledColumns[round].BySize = sizes
	}

}

// The cols must have the same size
func groupCols(cols []ifaces.Column, numToStick int) (groups [][]ifaces.Column) {

	numGroups := utils.DivCeil(len(cols), numToStick)
	groups = make([][]ifaces.Column, numGroups)

	size := cols[0].Size()

	for i, col := range cols {
		if col.Size() != size {
			utils.Panic(
				"column %v of size %v has been grouped with %v of size %v",
				col.GetColID(), col.Size(), cols[0].GetColID(), cols[0].Size(),
			)
		}
		groups[i/numToStick] = append(groups[i/numToStick], col)
	}

	lastGroup := &groups[len(groups)-1]
	zeroCol := verifiercol.NewConstantCol(field.Zero(), size)

	for i := len(*lastGroup); i < numToStick; i++ {
		*lastGroup = append(*lastGroup, zeroCol)
	}

	return groups
}

// Registers groups of columns in the sticky context
func insertNew(ctx *stickContext, round int, groups [][]ifaces.Column, news []ifaces.Column) {

	for i := range groups {
		group := groups[i]
		new := news[i]

		// Initialize the bySubCol if necessary
		if ctx.News[round].BySubCol == nil {
			ctx.News[round].BySubCol = map[ifaces.ColID]struct {
				NameNew  ifaces.ColID
				PosInNew int
			}{}
		}

		// Populate the bySubCol
		for posInNew, c := range group {
			ctx.News[round].BySubCol[c.GetColID()] = struct {
				NameNew  ifaces.ColID
				PosInNew int
			}{
				NameNew:  new.GetColID(),
				PosInNew: posInNew,
			}
		}

		// Populate the Column.ByNew
		if ctx.CompiledColumns[round].ByNew == nil {
			ctx.CompiledColumns[round].ByNew = make(map[ifaces.ColID][]ifaces.Column)
		}
		ctx.CompiledColumns[round].ByNew[new.GetColID()] = group

	}

	// And append the new columns to the global list
	ctx.News[round].List = append(ctx.News[round].List, news...)
}

func (ctx *stickContext) createMapToNew() {

	// Process the precomputed columns.
	for s := ctx.MinSize; s <= ctx.MaxSize; s *= 2 {
		// The list of columns of size `s` that we wish to group together
		cols_, ok := ctx.CompiledColumns[0].BySize[s]
		if !ok {
			// there are no columns of size `s`, we can keep
			// going with the next size since there is nothing
			// to do here.
			continue
		}

		// Filter out the precomputed columns
		cols := make([]ifaces.Column, 0, len(cols_))
		for _, col := range cols_ {
			if ctx.comp.Columns.Status(col.GetColID()) == column.Precomputed {
				cols = append(cols, col)
			}
		}

		if len(cols) == 0 {
			continue
		}

		// Organize the columns of size `s` into groups such that total size
		// of each group reaches the target size (MaxSize).
		groups := groupCols(cols, ctx.MaxSize/s)

		// Declare the new columns
		news := make([]ifaces.Column, len(groups))
		for i := range news {
			group := groups[i]
			values := make([][]field.Element, len(group))
			for j := range values {
				values[i] = smartvectors.IntoRegVec(ctx.comp.Precomputed.MustGet(group[j].GetColID()))
			}
			assignement := vector.Interleave(values...)
			ctx.comp.InsertPrecomputed(
				groupedName(group),
				smartvectors.NewRegular(assignement),
			)
		}

		// And append the new columns to the global list
		insertNew(ctx, 0, groups, news)
	}

	// Process all the non precomputed columns
	for round := 0; round < ctx.comp.NumRounds(); round++ {

		// Iterate over the columns sizes in ascending order
		for s := ctx.MinSize; s <= ctx.MaxSize; s *= 2 {

			// The list of columns of size `s` that we wish to group together
			cols_, ok := ctx.CompiledColumns[round].BySize[s]
			if !ok {
				// there are no columns of size `s`, we can keep
				// going with the next size since there is nothing
				// to do here.
				continue
			}

			// Filter out the precomputed columns
			cols := make([]ifaces.Column, 0, len(cols_))
			for _, col := range cols_ {
				if ctx.comp.Columns.Status(col.GetColID()) != column.Precomputed {
					cols = append(cols, col)
				}
			}

			if len(cols) == 0 {
				continue
			}

			// Organize the columns of size `s` into groups such that total size
			// of each group reaches the target size (MaxSize).
			groups := groupCols(cols, ctx.MaxSize/s)

			// Declare the new columns
			news := make([]ifaces.Column, len(groups))
			for i := range news {
				news[i] = ctx.comp.InsertCommit(
					round,
					groupedName(groups[i]),
					ctx.MaxSize,
				)
			}

			// And append the new columns to the global list
			insertNew(ctx, round, groups, news)
		}

		// assign the new columns
		round := round // deep copies the round to inject it in the closure

		ctx.comp.SubProvers.AppendToInner(round, func(run *wizard.ProverRuntime) {
			stopTimer := profiling.LogTimer("splitter compiler")
			defer stopTimer()
			for _, newName := range ctx.News[round].List {
				// Trick, in order to compute the assignment of newName we
				// extract the witness of the interleaving of the grouped
				// columns.
				group := ctx.CompiledColumns[round].ByNew[newName.GetColID()]
				witnesses := make([]smartvectors.SmartVector, len(group))
				for i := range witnesses {
					// If the column is allocated in the runtime (e.g. not a verifier column)
					// then we use a shallow copy of it.
					if run.Columns.Exists(group[i].GetColID()) {
						witnesses[i] = run.Columns.MustGet(group[i].GetColID())
						continue
					}
					// Else, we use the witness getting features attached to the column. (Which
					// is not memory efficient). That's why we do not use it for all columns.
					witnesses[i] = group[i].GetColAssignment(run)
				}
				assignement := smartvectors.
					AllocateRegular(len(group) * witnesses[0].Len()).(*smartvectors.Regular)
				for i := range group {
					for j := 0; j < witnesses[0].Len(); j++ {
						(*assignement)[i+j*len(group)] = witnesses[i].Get(j)
					}
				}
				run.AssignColumn(newName.GetColID(), assignement)
			}
		})
	}
}

// Compiling the local constraints
func (ctx *stickContext) compileArithmeticConstraints() {
	for _, qName := range ctx.comp.QueriesNoParams.AllUnignoredKeys() {

		q := ctx.comp.QueriesNoParams.Data(qName)

		// round of definition of the query to compile
		round := ctx.comp.QueriesNoParams.Round(qName)

		switch q := q.(type) {
		case query.LocalConstraint:

			// analyze the expression
			board := q.Board()
			metadata := board.ListVariableMetadata()
			report := analyzeExpr(*ctx, metadata)

			// detect if we should compile the expression or not
			if !report.anyColumnCompiled {
				continue
			}

			// in all cases, mark it as ignored
			ctx.comp.QueriesNoParams.MarkAsIgnored(qName)

			// detect if we can compile the expression
			if report.hasCompiledRepeated || report.hasCompiledInterleaved || !report.allColumnCompiled {
				continue
			}

			ctx.comp.InsertLocal(round, queryName(qName), ctx.replacedExpression(q.Expression, report))

		case query.GlobalConstraint:

			// analyze the expression
			board := q.Board()
			metadata := board.ListVariableMetadata()
			report := analyzeExpr(*ctx, metadata)

			// detect if we should compile the expression or not
			if !report.anyColumnCompiled {
				continue
			}

			// in all cases, mark it as ignored
			ctx.comp.QueriesNoParams.MarkAsIgnored(qName)

			// detect if we can compile the expression
			if report.hasCompiledRepeated || report.hasCompiledInterleaved || !report.allColumnCompiled {
				continue
			}

			replacedExpression := ctx.replacedExpression(q.Expression, report)
			replacedExpression = replacedExpression.Mul(variables.NewPeriodicSample(ctx.MaxSize/report.domainsSize, 0))

			ctx.comp.InsertGlobal(round, queryName(qName), replacedExpression, q.NoBoundCancel)

		default:
			utils.Panic("got an uncompilable query %++v", qName)
		}

	}
}

// Compiling the fixed evaluations is made by
func (ctx *stickContext) compileFixedEvaluation() {

	for _, qName := range ctx.comp.QueriesParams.AllUnignoredKeys() {

		// Filters out only the
		q, ok := ctx.comp.QueriesParams.Data(qName).(query.LocalOpening)
		if !ok {
			utils.Panic("got an uncompilable query name=%v type=%T", qName, q)
		}

		// Assumption, the query is not over an interleaved column
		parents := column.RootParents(q.Pol)
		if len(parents) > 1 {
			// We skip the compilation. We will not need it as we transition
			// to the log-derivative lookup.
			continue
		}

		parent := parents[0]
		if !isCompiledColumn(*ctx, parent) {
			continue
		}

		// mark the query as ignored
		ctx.comp.QueriesParams.MarkAsIgnored(qName)

		newCol := columnReplacement(*ctx, q.Pol)
		round := q.Pol.Round()

		newQ := ctx.comp.InsertLocalOpening(round, queryName(q.ID), newCol)

		// Registers the prover's step responsible for assigning the
		// new query @alex, it might be beneficial to run this in parallel
		// We don't do it because we think this is not necessary.
		ctx.comp.SubProvers.AppendToInner(round, func(run *wizard.ProverRuntime) {
			y := run.QueriesParams.MustGet(q.ID).(query.LocalOpeningParams).Y
			run.AssignLocalPoint(newQ.ID, y)
		})

		// The verifier ensures that the old and new queries have the same assignement
		ctx.comp.InsertVerifier(round, func(run *wizard.VerifierRuntime) error {
			oldParams := run.GetLocalPointEvalParams(q.ID)
			newParams := run.GetLocalPointEvalParams(queryName(q.ID))

			if oldParams != newParams {
				return fmt.Errorf("sticker verifier failed for local opening %v - %v", q.ID, queryName(q.ID))
			}

			return nil
		}, func(api frontend.API, run *wizard.WizardVerifierCircuit) {
			oldParams := run.GetLocalPointEvalParams(q.ID)
			newParams := run.GetLocalPointEvalParams(queryName(q.ID))
			api.AssertIsEqual(oldParams.Y, newParams.Y)
		})
	}

}

// Get the column replacement for an expression
func (ctx *stickContext) replacedExpression(
	expr *symbolic.Expression,
	r exprAnalysisReport,
) (
	newExpr *symbolic.Expression,
) {

	// we assume the expression has already been checked for compilability
	if !r.allColumnCompiled || r.hasCompiledInterleaved || r.hasCompiledRepeated {
		panic("the expression has not been checked for compilability")
	}

	board := expr.Board()
	metadata := board.ListVariableMetadata()
	replaceMap := collection.NewMapping[string, *symbolic.Expression]()

	for i := range metadata {
		switch m := metadata[i].(type) {
		case ifaces.Column:
			// it's always a compiled column
			newCol := columnReplacement(*ctx, m)
			replaceMap.InsertNew(m.String(), ifaces.ColumnAsVariable(newCol))
		case coin.Info, ifaces.Accessor:
			replaceMap.InsertNew(m.String(), symbolic.NewVariable(m))
		case variables.X:
			// @alex: this could be supported in theory though
			panic("unsupported, the value of `x` in the unsplit query and the split would be different")
		case variables.PeriodicSample:
			// there, we need to inflate the period and the offset
			scaling := ctx.MaxSize / r.domainsSize
			replaceMap.InsertNew(m.String(), variables.NewPeriodicSample(m.T*scaling, m.Offset*scaling))
		}
	}

	return expr.Replay(replaceMap)
}

func groupedName(group []ifaces.Column) ifaces.ColID {
	fmtted := make([]string, len(group))
	for i := range fmtted {
		fmtted[i] = group[i].String()
	}
	return ifaces.ColIDf("STICKER_%v", strings.Join(fmtted, "_"))
}

func queryName(oldQ ifaces.QueryID) ifaces.QueryID {
	return ifaces.QueryIDf("%v_STICKER", oldQ)
}

// collection of metadata for an expression that are relevant
// to how we want to compile a local or a global query
type exprAnalysisReport struct {
	// domain size of the expression
	domainsSize int
	// true if the expression contains a compiled interleaved column.
	// This will not be necessary anymore as we start supporting
	// fully the logderivative lookups technique.
	hasCompiledInterleaved bool
	// true if the expression contains a compiled repeated column.
	// in that case, we prefer to avoid compiling the expression
	// because this incurs a large blowup in the domain size. When
	// we transition completely to the log-derivative lookups. This
	// will not be necessary anymore as we will be able to remove
	// support for repeated and interleaved columns completely.
	hasCompiledRepeated bool
	// true if the expression contains a compiled column
	allColumnCompiled bool
	// true if any of the column is compiled
	anyColumnCompiled bool
}

func analyzeExpr(
	ctx stickContext,
	metadata []symbolic.Metadata,
) (r exprAnalysisReport) {

	r.allColumnCompiled = true
	for i := range metadata {
		switch m := metadata[i].(type) {
		case ifaces.Column:
			rootParents := column.RootParents(m)
			parent := rootParents[0]

			// save the domain size
			r.domainsSize = m.Size()

			// this detects if the column is compiled,
			// if so update the return arguments
			isCompiled := isCompiledColumn(ctx, parent)

			// set the flag to false
			r.allColumnCompiled = r.allColumnCompiled && isCompiled
			r.anyColumnCompiled = r.anyColumnCompiled || isCompiled

			// check if the column is a compiled interleaving
			if isCompiled && len(rootParents) > 1 {
				r.hasCompiledInterleaved = true
			}

			// check if the column has a repeat
			if isCompiled && len(rootParents) == 1 && parent.Size() < m.Size() {
				r.hasCompiledRepeated = true
			}
		}
	}

	if r.domainsSize == 0 {
		panic("found no columns in the expression")
	}

	return r
}

func isCompiledColumn(ctx stickContext, col ifaces.Column) bool {
	// this will panic if the column round is above everything
	_, ok := ctx.News[col.Round()].BySubCol[col.GetColID()]
	return ok
}

// Takes a abstract reference to a compiled column and returns
// and abstract reference to the
func columnReplacement(ctx stickContext, col ifaces.Column) ifaces.Column {

	// Extract the assumedly single col
	compiledCol := column.RootParents(col)[0]

	round := col.Round()
	parentInfo := ctx.News[round].BySubCol[compiledCol.GetColID()]
	newCol := ctx.comp.Columns.GetHandle(parentInfo.NameNew)

	// Shift the new column by the right position
	position := column.StackOffsets(col)

	scaling := newCol.Size() / compiledCol.Size()
	newPosition := scaling*position + parentInfo.PosInNew

	return column.Shift(newCol, newPosition)
}
