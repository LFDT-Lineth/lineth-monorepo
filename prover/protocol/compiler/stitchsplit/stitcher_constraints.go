package stitchsplit

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/consensys/gnark-crypto/field/koalabear/fft"
	"github.com/consensys/gnark/frontend"
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
)

func (ctx StitchingContext) constraints() {
	ctx.LocalOpening()
	ctx.LocalGlobalConstraints()

	// This iteration is undeterministic order but it should not result in
	// non-determinism in the wizard.CompiledIOP because we append different
	// rounds.
	for round, qs := range ctx.ExplicitlyVerifiedQueries {
		ctx.Comp.RegisterVerifierAction(round, &QueryVerifierAction{
			Qs: *qs,
		})
	}

}

func (ctx StitchingContext) LocalOpening() {

	// Ignore the LocalOpening queries over the subColumns.
	for _, qName := range ctx.Comp.QueriesParams.AllUnignoredKeys() {
		// Filters out only the LocalOpening
		q, ok := ctx.Comp.QueriesParams.Data(qName).(query.LocalOpening)
		if !ok {
			utils.Panic("got an uncompilable query %v", qName)
		}

		round := ctx.Comp.QueriesParams.Round(q.ID)

		if q.Pol.Size() < ctx.MinSize {
			//sanity-check: column should be public
			verifiercol.AssertIsPublicCol(ctx.Comp, q.Pol)
			// Ask the verifier to directly check the query
			ctx.addExplicitlyVerifiedQuery(round, q)
			// mark the query as ignored
			ctx.Comp.QueriesParams.MarkAsIgnored(q.ID)

			// And skip the rest of the compilation : we are done
			continue
		}

		if !isColEligibleStitching(ctx.Stitchings, q.Pol) {
			continue
		}

		switch m := q.Pol.(type) {
		case verifiercol.VerifierCol:
			utils.Panic("unsupported, received a localOpening over the verifier column %v", m.GetColID())
		}
		// mark the query as ignored
		ctx.Comp.QueriesParams.MarkAsIgnored(qName)

		// get the stitching column associated with the sub column q.Poly.
		stitchingCol := getStitchingCol(ctx, q.Pol)
		if stitchingCol == nil {
			utils.Panic("stitching col is nil")
		}

		newQ := ctx.Comp.InsertLocalOpening(round, queryNameStitcher(q.ID), stitchingCol)

		// Registers the prover's step responsible for assigning the new query
		ctx.Comp.RegisterProverAction(round, &AssignLocalPointProverAction{
			QID:  q.ID,
			NewQ: newQ.ID,
		})
	}

}

func (ctx StitchingContext) LocalGlobalConstraints() {
	for _, qName := range ctx.Comp.QueriesNoParams.AllUnignoredKeys() {

		q := ctx.Comp.QueriesNoParams.Data(qName)
		// round of definition of the query to compile
		round := ctx.Comp.QueriesNoParams.Round(qName)

		var board symbolic.ExpressionBoard

		switch q := q.(type) {
		case query.LocalConstraint:
			board = q.Board()
			if q.DomainSize < ctx.MinSize {
				// Sanity-check : at this point all the parameters of the query
				// should have a public status. Indeed, prior to compiling the
				// constraints to work
				metadatas := board.ListVariableMetadata()
				for _, metadata := range metadatas {
					if h, ok := metadata.(ifaces.Column); ok {
						verifiercol.AssertIsPublicCol(ctx.Comp, h)
					}
				}
				ctx.addExplicitlyVerifiedQuery(round, q)
				// mark the query as ignored
				ctx.Comp.QueriesNoParams.MarkAsIgnored(qName)
				continue
			}

			// If the domainsize is larger than the max size, we cannot stitch it.
			if q.DomainSize > ctx.MaxSize {
				continue
			}

			// detect if the expression is eligible;
			// i.e., it contains columns of proper size with status Precomputed, committed, or verifiercol.
			isEligible := IsExprEligible(isColEligibleStitching, ctx.Stitchings, board, compilerTypeStitch)
			if !isEligible {
				continue
			}

			// if the associated expression is eligible to the stitching, mark the query, over the sub columns, as ignored.
			ctx.Comp.QueriesNoParams.MarkAsIgnored(qName)

			// adjust the query over the stitching columns
			ctx.Comp.InsertLocal(round, queryNameStitcher(qName), ctx.adjustExpression(q.Expression, q.DomainSize, false, false, nil))

		case query.GlobalConstraint:
			board = q.Board()
			if q.DomainSize < ctx.MinSize {

				// Sanity-check : at this point all the parameters of the query
				// should have a public status. Indeed, prior to compiling the
				// constraints to work
				metadatas := board.ListVariableMetadata()
				for _, metadata := range metadatas {
					if h, ok := metadata.(ifaces.Column); ok {
						verifiercol.AssertIsPublicCol(ctx.Comp, h)
					}
				}
				ctx.addExplicitlyVerifiedQuery(round, q)
				// mark the query as ignored
				ctx.Comp.QueriesNoParams.MarkAsIgnored(qName)
				continue
			}
			// If the domainsize is larger than the max size, we cannot stitch it.
			if q.DomainSize > ctx.MaxSize {
				continue
			}
			// detect if the expression is over the eligible columns.
			isEligible := IsExprEligible(isColEligibleStitching, ctx.Stitchings, board, compilerTypeStitch)
			if !isEligible {
				continue
			}

			// if the associated expression is eligible to the stitching, mark the query, over the sub columns, as ignored.
			ctx.Comp.QueriesNoParams.MarkAsIgnored(qName)

			// adjust the query over the stitching columns
			ctx.Comp.InsertGlobal(round, queryNameStitcher(qName),
				ctx.adjustExpression(q.Expression, q.DomainSize, true, q.NoBoundCancel, q.OffsetRangeOverrides),
				true)

		default:
			utils.Panic("got an uncompilable query %++v", qName)
		}
	}
}

// Takes a sub column and returns the stitching column.
// The stitching column is shifted in such a way that the first row agrees with the first row of the sub column.
// i.e., such stitching column agrees with the sub column up to a subsampling with offset zero.
// The col should only be either verifiercol or eligible col.
// option is always empty, and used only for the recursive calls over the shifted columns.
func getStitchingCol(ctx StitchingContext, col ifaces.Column, option ...int) ifaces.Column {
	var (
		stitchingCol ifaces.Column
		newOffset    int
		round        = col.Round()
	)

	switch m := col.(type) {

	// case: constant column. The column may directly be expanded
	case verifiercol.ConstCol:
		// Sometime, we may want to shift a constant column by a non-zero offset
		// to cancel the constraint at the first or last positions.
		var res ifaces.Column
		if m.F.IsBase {
			res = verifiercol.NewConstantCol(m.F.Base, ctx.MaxSize, m.Name+"_STITCHED")
		} else {
			// Preserve extension-field constant value when GenericFieldElem holds an extension
			res = verifiercol.NewConstantColExt(m.F.Ext, ctx.MaxSize, m.Name+"_STITCHED")
		}
		if len(option) != 0 {
			res = column.Shift(res, option[0])
		}
		return res

	// case: verifier columns without shift
	case verifiercol.VerifierCol:
		scaling := ctx.MaxSize / col.Size()
		if scaling < 1 {
			utils.Panic("cannot expand verifier/proof column %v: size=%v > MaxSize=%v", col.GetColID(), m.Size(), ctx.MaxSize)
		}
		stitchingCol = verifiercol.ExpandedProofOrVerifyingKeyColWithZero{
			Col:       m,
			Expansion: scaling,
		}
		if len(option) != 0 {
			// if it is a shifted veriferCol, set the offset for shifting the expanded column
			newOffset = option[0] * scaling
		}
		return column.Shift(stitchingCol, newOffset)

	case column.Natural:
		// find the stitching column
		switch m.Status() {
		case column.Proof, column.VerifyingKey:
			scaling := ctx.MaxSize / m.Size()
			if scaling < 1 {
				utils.Panic("cannot expand proof/verifying-key column %v: size=%v > MaxSize=%v", m.GetColID(), m.Size(), ctx.MaxSize)
			}
			stitchingCol = verifiercol.ExpandedProofOrVerifyingKeyColWithZero{
				Col:       col,
				Expansion: scaling,
			}
			if len(option) != 0 {
				// if it is a shifted veriferCol, set the offset for shifting the expanded column
				newOffset = option[0] * scaling
			}
			return column.Shift(stitchingCol, newOffset)
		// reminder: subcols are ignored after stitching
		case column.Committed, column.Precomputed, column.Ignored:
			subColInfo := ctx.Stitchings[round].BySubCol[col.GetColID()]
			stitchingCol = ctx.Comp.Columns.GetHandle(subColInfo.NameBigCol)
			scaling := stitchingCol.Size() / col.Size()
			if len(option) != 0 {
				newOffset = option[0] * scaling
			}
			newOffset = newOffset + subColInfo.PosInBigCol
			return column.Shift(stitchingCol, newOffset)
		default:
			utils.Panic("unsupported column status %v", m.Status())
		}

	case column.Shifted:
		// Shift the stitching column by the right position
		offset := column.StackOffsets(col)
		col = column.RootParents(col)
		res := getStitchingCol(ctx, col, offset)
		if res == nil {
			utils.Panic("stitching col is nil")
		}
		return res

	default:

		panic("unsupported")

	}
	return nil
}

func queryNameStitcher(oldQ ifaces.QueryID) ifaces.QueryID {
	return ifaces.QueryIDf("%v_STITCHER", oldQ)
}

// it adjusts the expression, that is among sub columns, by replacing the sub columns with their stitching columns.
// for the verfiercol instead of stitching, they are expanded to reach the proper size.
// This is due to the fact that the verifiercols are not tracked by the compiler and can not be stitched
// via [scanAndClassifyEligibleColumns].
func (ctx *StitchingContext) adjustExpression(
	expr *symbolic.Expression, domainSize int,
	isGlobalConstraint bool,
	originalHasNoBoundCancel bool,
	overrideOffsetRange *utils.Range,
) (
	newExpr *symbolic.Expression,
) {

	var (
		board        = expr.Board()
		metadata     = board.ListVariableMetadata()
		replaceMap   = collection.NewMapping[string, *symbolic.Expression]()
		stitchingCol ifaces.Column
	)

	for i := range metadata {
		switch m := metadata[i].(type) {
		case ifaces.Column:
			// it's always a compiled column
			stitchingCol = getStitchingCol(*ctx, m)
			if stitchingCol == nil {
				utils.Panic("stitching col is nil")
			}
			replaceMap.InsertNew(m.String(), ifaces.ColumnAsVariable(stitchingCol))
		case coin.Info, ifaces.Accessor:
			replaceMap.InsertNew(m.String(), symbolic.NewVariable(m))
		case variables.X:
			panic("unsupported, the value of `x` in the unsplit query and the split would be different")
		case variables.PeriodicSample:
			// there, we need to inflate the period and the offset
			scaling := ctx.MaxSize / domainSize
			replaceMap.InsertNew(m.String(), variables.NewPeriodicSample(m.T*scaling, m.Offset*scaling))
		default:
			utils.Panic("unsupported metadata type %T", m)
		}
	}

	newExpr = expr.Replay(replaceMap)
	if isGlobalConstraint {
		scaling := ctx.MaxSize / domainSize
		// for the global constraints, check the constraint only over the subSampeling of the columns.
		newExpr = symbolic.Mul(newExpr, variables.NewPeriodicSample(scaling, 0))

		if !originalHasNoBoundCancel {
			// add the bound cancelation
			factorTop, factorBottom := getBoundCancelledExpression(expr, domainSize, scaling, overrideOffsetRange)
			factors := []any{newExpr}
			if factorTop != nil {
				factors = append(factors, factorTop)
			}
			if factorBottom != nil {
				factors = append(factors, factorBottom)
			}
			newExpr = symbolic.MulNoSimplify(factors...)
		}
	}

	return newExpr
}

type QueryVerifierAction struct {
	Qs []ifaces.Query
}

func (a *QueryVerifierAction) Run(vr wizard.Runtime) error {
	var mainErr error
	for i := range a.Qs {
		if err := a.Qs[i].Check(vr); err != nil {
			err = fmt.Errorf("queryName=%v - %w", a.Qs[i].Name(), err)
			mainErr = errors.Join(mainErr, err)
		}
	}
	if mainErr != nil {
		return fmt.Errorf("stitching.QueryVerifierAction failed: %w", mainErr)
	}
	return nil
}

func (a *QueryVerifierAction) RunGnark(api frontend.API, wvc wizard.GnarkRuntime) {
	for i := range a.Qs {
		a.Qs[i].CheckGnark(api, wvc)
	}
}

func (ctx *StitchingContext) addExplicitlyVerifiedQuery(round int, q ifaces.Query) {
	if ctx.ExplicitlyVerifiedQueries[round] == nil {
		ctx.ExplicitlyVerifiedQueries[round] = new([]ifaces.Query)
	}
	*ctx.ExplicitlyVerifiedQueries[round] = append(*ctx.ExplicitlyVerifiedQueries[round], q)
}

// getBoundCancelledExpression computes the "bound cancelled expression" for the
// constraint cs. Namely, the constraints expression is multiplied by terms of the
// form X-\omega^k to cancel the expression at position "k" if required. If the
// constraint uses the "noBoundCancel" feature, then the constraint expression is
// directly returned.
func getBoundCancelledExpression(originalExpr *symbolic.Expression, originalDomainSize int, scaling int, overrideOffsetRange *utils.Range) (factorTop, factorBottom *symbolic.Expression) {

	var (
		cancelRange = query.MinMaxOffsetOfExpression(originalExpr)
		x           = variables.NewXVar()
		omega, _    = fft.Generator(uint64(originalDomainSize * scaling))
		// factorTop and factorBottom are used to store the terms of the form
		// X-\omega^k. They are constructed, disabling simplifications and in
		// a way that helps the evaluator to regroup shared subexpressions.
		factorCount = 0
	)

	if overrideOffsetRange != nil {
		cancelRange = *overrideOffsetRange
	}

	if cancelRange.Min < 0 {
		// Cancels the expression on the range [0, -cancelRange.Min)
		for i := 0; i < -cancelRange.Min; i++ {

			factorCount++
			var root field.Element
			root.Exp(omega, big.NewInt(int64(i*scaling)))
			term := symbolic.Sub(x, root)

			if factorBottom == nil {
				factorBottom = term
			} else {
				factorBottom = symbolic.MulNoSimplify(factorBottom, term)
			}
		}
	}

	if cancelRange.Max > 0 {
		// Cancels the expression on the range (N-cancelRange.Max-1, N-1]
		for i := 0; i < cancelRange.Max; i++ {

			factorCount++
			var root field.Element
			point := originalDomainSize - i - 1
			root.Exp(omega, big.NewInt(int64(point*scaling)))
			term := symbolic.Sub(x, root)

			if factorTop == nil {
				factorTop = term
			} else {
				factorTop = symbolic.MulNoSimplify(factorTop, term)
			}
		}
	}

	return factorTop, factorBottom
}
