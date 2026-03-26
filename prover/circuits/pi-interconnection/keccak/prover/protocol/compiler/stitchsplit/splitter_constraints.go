package stitchsplit

import (
	"reflect"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/variables"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/collection"
)

func (ctx SplitterContext) constraints() {
	ctx.LocalOpening()
	ctx.LocalGlobalConstraints()
}

type AssignLocalPointProverAction struct {
	QID  ifaces.QueryID
	NewQ ifaces.QueryID
}

func (a *AssignLocalPointProverAction) Run(run *wizard.ProverRuntime) {
	y := run.QueriesParams.MustGet(a.QID).(query.LocalOpeningParams).Y
	run.AssignLocalPoint(a.NewQ, y)
}

func (ctx SplitterContext) LocalOpening() {
	for _, qName := range ctx.Comp.QueriesParams.AllUnignoredKeys() {
		// Filters out only the LocalOpening
		q, ok := ctx.Comp.QueriesParams.Data(qName).(query.LocalOpening)
		if !ok {
			utils.Panic("got an uncompilable query %v", qName)
		}

		round := ctx.Comp.QueriesParams.Round(q.ID)

		if !isColEligibleSplitting(ctx.Splittings, q.Pol) {
			continue
		}
		// mark the query as ignored
		ctx.Comp.QueriesParams.MarkAsIgnored(qName)
		// Get the sub column
		subCol := getSubColForLocal(ctx, q.Pol, 0)
		// apply the local constrain over the subCol
		newQ := ctx.Comp.InsertLocalOpening(round, queryNameSplitter(q.ID), subCol)

		// Registers the prover's step responsible for assigning the new query
		ctx.Comp.RegisterProverAction(round, &AssignLocalPointProverAction{
			QID:  q.ID,
			NewQ: newQ.ID,
		})
	}
}

func (ctx SplitterContext) LocalGlobalConstraints() {
	for _, qName := range ctx.Comp.QueriesNoParams.AllUnignoredKeys() {

		q := ctx.Comp.QueriesNoParams.Data(qName)
		// round of definition of the query to compile
		round := ctx.Comp.QueriesNoParams.Round(qName)

		var board symbolic.ExpressionBoard

		switch q := q.(type) {
		case query.LocalConstraint:
			board = q.Board()

			// detect if the expression is eligible;
			// i.e., it contains columns of proper size with status Precomputed, committed, or verifiercol.
			isEligible, unSupported := IsExprEligible(isColEligibleSplitting, ctx.Splittings, board, compilerTypeSplit)

			if unSupported {
				panic("unSupported")
			}

			if !isEligible {
				continue
			}

			// if the associated expression is eligible to the stitching, mark the query, over the sub columns, as ignored.
			ctx.Comp.QueriesNoParams.MarkAsIgnored(qName)

			// adjust the query over the sub columns
			ctx.Comp.InsertLocal(round, queryNameSplitter(qName), ctx.adjustExpressionForLocal(q.Expression, 0))

		case query.GlobalConstraint:
			board = q.Board()

			// detect if the expression is eligible;
			// i.e., it contains columns of proper size with status Precomputed, committed, or verifiercol.
			isEligible, unSupported := IsExprEligible(isColEligibleSplitting, ctx.Splittings, board, compilerTypeSplit)

			if unSupported {
				panic("unSupported")
			}

			if !isEligible {
				continue
			}

			// if the associated expression is eligible to the splitting, mark the query as ignored.
			ctx.Comp.QueriesNoParams.MarkAsIgnored(qName)

			// adjust the query over the sub columns
			numSlots := q.DomainSize / ctx.Size
			for slot := 0; slot < numSlots; slot++ {

				ctx.Comp.InsertGlobal(round,
					ifaces.QueryIDf("%v_SPLITTER_GLOBALQ_SLOT_%v", q.ID, slot),
					ctx.adjustExpressionForGlobal(q.Expression, slot),
				)

				ctx.localQueriesForGapsInGlobal(q, slot, numSlots)
			}

		default:
			utils.Panic("got an uncompilable query %++v", qName)
		}
	}
}

// it checks if a column registered in the compiler has the proper size and state for splitting.
func isColEligibleSplitting(splittings MultiSummary, col ifaces.Column) bool {
	natural := column.RootParents(col)
	_, found := splittings[col.Round()].ByBigCol[natural.GetColID()]
	return found
}

// It finds the subCol containing the first row of col,
// it then shifts the subCol so that its first row equals with the first row of col.
func getSubColForLocal(ctx SplitterContext, col ifaces.Column, posInCol int) ifaces.Column {
	round := col.Round()
	// Sanity-check : only for the edge-case h.Size() < ctx.size
	if col.Size() < ctx.Size && posInCol != 0 {
		utils.Panic("We have h.Size (%v) < ctx.size (%v) but num (%v) != 0 for %v", col.Size(), ctx.Size, posInCol, col.GetColID())
	}

	if !col.IsComposite() {
		switch col := col.(type) {
		case verifiercol.VerifierCol:
			// Create the split in live
			return col.Split(ctx.Comp, posInCol*ctx.Size, (posInCol+1)*ctx.Size)
		default:
			// No changes : it means this is a normal column and
			// we shall take the corresponding slice.
			return ctx.Splittings[round].ByBigCol[col.GetColID()][posInCol]
		}
	}

	switch inner := col.(type) {
	case column.Shifted:
		// Shift the subparent, if the offset is larger than the subparent
		// we repercute it on the num
		if inner.Offset < -ctx.Size {
			utils.Panic("unsupported, the offset is too negative")
		}

		// find the subCol that contain the first row of col
		position := utils.PositiveMod(column.StackOffsets(col), col.Size())
		subColID, offsetOfSubCol := position/ctx.Size, position%ctx.Size

		// The subCol is linked to the "root" of q.Pol (i.e., natural column associated with col)
		parent := getSubColForLocal(ctx, inner.Parent, posInCol+subColID)
		return column.Shift(parent, offsetOfSubCol)

	default:
		utils.Panic("unexpected type %v", reflect.TypeOf(inner))
	}
	panic("unreachable")
}

// For the column 'col' and the given 'posInCol',
// it returns the subColumn from the natural column located in position 'posInNatural'.
// where the posInNatural is calculated via the offset in Col.
func getSubColForGlobal(ctx SplitterContext, col ifaces.Column, posInCol int) ifaces.Column {
	// Sanity-check : only for the edge-case h.Size() < ctx.size
	round := col.Round()
	if col.Size() < ctx.Size {
		if posInCol > 0 {
			utils.Panic(
				"tried to get share #%v of column %v, but this is an undersized column %v",
				posInCol, col.GetColID(), col.Size(),
			)
		}

		// Not a split  column : returns the input directly
		return col
	}

	if !col.IsComposite() {
		switch m := col.(type) {
		case verifiercol.VerifierCol:
			// Create the split in live
			return m.Split(ctx.Comp, posInCol*ctx.Size, (posInCol+1)*ctx.Size)
		default:
			// No changes; natural column
			return ctx.Splittings[round].ByBigCol[col.GetColID()][posInCol]
		}
	}

	switch inner := col.(type) {
	// Shift the subparent, if the offset is larger than the subparent
	// we repercute it on the num
	case column.Shifted:

		// This works fine assuming h.Size() > ctx.size
		var (
			offset         = inner.Offset
			maxNumSubCol   = col.Size() / ctx.Size
			subColID       = (posInCol + (offset / ctx.Size)) % maxNumSubCol
			offsetOfSubCol = utils.PositiveMod(offset, ctx.Size)
		)
		// This indicates that the offset is so large
		if subColID < 0 {
			subColID = (col.Size() / ctx.Size) + subColID
		}
		// The resulting offset should keep the same sign as the old one. This is
		// because the sign indicates which range of position is touched by
		// bound cancellation.
		if offsetOfSubCol*offset < 0 {
			offsetOfSubCol -= ctx.Size
		}
		parent := getSubColForGlobal(ctx, inner.Parent, subColID)
		return column.Shift(parent, offsetOfSubCol)

	default:
		utils.Panic("unexpected type %v", reflect.TypeOf(inner))
	}
	panic("unreachable")
}

func queryNameSplitter(oldQ ifaces.QueryID) ifaces.QueryID {
	return ifaces.QueryIDf("%v_SPLITTER", oldQ)
}

// it shift all the columns inside the expression by shift and then applies the local constraints.
func (ctx SplitterContext) adjustExpressionForLocal(
	expr *symbolic.Expression, shift int,
) *symbolic.Expression {

	board := expr.Board()
	metadatas := board.ListVariableMetadata()
	translationMap := collection.NewMapping[string, *symbolic.Expression]()

	for _, metadata := range metadatas {
		// Replace the expression by the one

		switch m := metadata.(type) {
		case ifaces.Column:
			subCol := getSubColForLocal(ctx, column.Shift(m, shift), 0)
			translationMap.InsertNew(m.String(), ifaces.ColumnAsVariable(subCol))
		// @Azam why we need these cases?
		case coin.Info, ifaces.Accessor:
			translationMap.InsertNew(m.String(), symbolic.NewVariable(m))
		case variables.X:
			panic("unsupported, the value of `x` in the unsplit query and the split would be different")
		case variables.PeriodicSample:
			// for PeriodicSampling offset is always positive in forward direction, thus we need (-shift)
			newSample := variables.NewPeriodicSample(m.T, utils.PositiveMod(m.Offset-shift, m.T))
			translationMap.InsertNew(m.String(), newSample)
		default:
			// Repass the same variable
			translationMap.InsertNew(m.String(), symbolic.NewVariable(metadata))
		}
	}

	newExpr := expr.Replay(translationMap)
	return newExpr
}

func (ctx SplitterContext) adjustExpressionForGlobal(
	expr *symbolic.Expression, slot int,
) *symbolic.Expression {
	board := expr.Board()
	metadatas := board.ListVariableMetadata()
	translationMap := collection.NewMapping[string, *symbolic.Expression]()

	for _, metadata := range metadatas {

		// For each slot, get the expression obtained by replacing the commitment
		// by the appropriated column.

		switch m := metadata.(type) {
		case ifaces.Column:
			// Pass the same variable
			subCol := getSubColForGlobal(ctx, m, slot)
			// Sanity-check : the subHandle should have the target size
			if subCol.Size() != ctx.Size {
				utils.Panic(
					"outgoing column %v should have size %v but has size %v (ingoing column was %v, with size %v)",
					subCol.GetColID(), ctx.Size, subCol.Size(), m.GetColID(), m.Size(),
				)
			}
			translationMap.InsertNew(m.String(), ifaces.ColumnAsVariable(subCol))
		case variables.X:
			utils.Panic("unsupported, the value of `x` in the unsplit query and the split would be different")
		case variables.PeriodicSample:
			// Check that the period is not larger than the domain size. If
			// the period is smaller this is a no-op because the period does
			// not change.
			translated := symbolic.NewVariable(metadata)

			if m.T > ctx.Size {

				// Here, there are two possibilities. (1) The current slot is
				// on a portion of the Periodic sample where everything is
				// zero or (2) the current slot matchs a portion of the
				// periodic sampling containing a 1. To determine which is
				// the current situation, we need to find out where the slot
				// is located compared to the period.
				var (
					slotStartAt = (slot * ctx.Size) % m.T
					slotStopAt  = slotStartAt + ctx.Size
				)

				if m.Offset >= slotStartAt && m.Offset < slotStopAt {
					translated = variables.NewPeriodicSample(ctx.Size, m.Offset%ctx.Size)
				} else {
					translated = symbolic.NewConstant(0)
				}
			}

			// And we can just pass it over because the period does not change
			translationMap.InsertNew(m.String(), translated)
		default:
			// Repass the same variable (for coins or other types of single-valued variable)
			translationMap.InsertNew(m.String(), symbolic.NewVariable(metadata))
		}

	}
	return expr.Replay(translationMap)
}

func (ctx SplitterContext) localQueriesForGapsInGlobal(q query.GlobalConstraint, slot, numSlots int) {

	// Now, we need to cancel the expression at the beginning and/or the end
	// For the first one, only cancel the end. For the last one, only cancel
	// the beginning.
	offsetRange := query.MinMaxOffset(q.Expression)
	round := ctx.Comp.QueriesNoParams.Round(q.ID)
	nextStart := 0

	if offsetRange.Min < 0 {
		for i := 0; i > offsetRange.Min; i-- {

			if slot > 0 || q.NoBoundCancel {
				// And fill the gap with a local constraint
				newQName := ifaces.QueryIDf("%v_LOCAL_GAPS_NEG_OFFSET_%v", q.ID, i)
				if ctx.Comp.QueriesNoParams.Exists(newQName) {
					continue
				}
				// adjust the query over the sub columns
				ctx.Comp.InsertLocal(round,
					newQName,
					ctx.adjustExpressionForLocal(q.Expression, slot*ctx.Size-i))
			}
		}
		if offsetRange.Max > 0 {
			nextStart = 1
		}
	}

	if offsetRange.Max > 0 {
		for i := nextStart; i < offsetRange.Max; i++ {
			point := ctx.Size - i - 1 // point at which we want to cancel the constraint
			// And fill the gap with a local constraint
			if slot < numSlots-1 || q.NoBoundCancel {
				newQName := ifaces.QueryIDf("%v_LOCAL_GAPS_POS_OFFSET_%v_%v", q.ID, slot, i)
				if ctx.Comp.QueriesNoParams.Exists(newQName) {
					continue
				}

				shift := slot*ctx.Size + point
				ctx.Comp.InsertLocal(round,
					newQName,
					ctx.adjustExpressionForLocal(q.Expression, shift))
			}
		}
	}
}
