package byte32cmp

import (
	"fmt"
	"sync"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/parallel"
)

// MultiLimbCmp is a dedicated wizard which can compare two [LimbColumns] and
// thereby constructing indicator columns indicating whether the first operand
// is lower, greater or if the two operands are equals.
//
// This object implements the [wizard.ProverAction] interface and is meant to
// be run to compute the assignment of the returned column by [CmpMultiLimbs].
type MultiLimbCmp struct {

	// IsGreater and IsLower are columns to be assigned by the context. The
	// reason isEqual is missing is because its handling is defered
	// to a dedicated sub-context.
	IsGreater, IsLower ifaces.Column

	// IsEqualCtx is the dedicated [wizard.ProverAction] responsible for
	// assigning the returned isEqual column.
	IsEqualCtx *dedicated.IsZeroCtx

	// nonNegative syndrom is an internal column created such that it should
	// always represent a number of size 1 << numLimbs. It is constructed using
	// the result of the other limb-by-limb comparison.
	NonNegativeSyndrom ifaces.Column

	// SyndromBoard is an expression board used to assign the non-negative
	// syndromColumn its is the same as (isGreater - isLower).
	SyndromBoard sym.ExpressionBoard

	// The SubCtxs are the [wizard.ProverAction] responsible for doing the
	// assignment part for each limb-by-limb comparison.
	SubCtxs []wizard.ProverAction
}

// CmpMultiLimbs returns three columns: isGreater, isEqual and isLower which
// are mutually-exclusive pre-constrainted binary columns and a
// [wizard.ProverAction] computing their assignement. The returned columns
// indicates whether the words represented by `a` are greater, equal or
// smaller compared to `b`. It can be used to perform comparison between large
// numbers.
//
// The function assumes that `a` and `b` are already constrained to be well-formed.
//
// The function works by using the [CmpSmallCols] limb-wise to obtain a partial
// comparison limb-by-limbs (gi, ei, li). From there, isEqual is constrained
// by "all the ei == 1" <==> "isEqual = 1" using a [dedicated.IsZero]
// sub-routine. isGreater and isLower are constrained using what we call a
// syndrom = \sum_i (gi - li) 2**i. We have that isGreater = 1 <=> syndrom > 0
// and isLower <=> syndrom < 0.
//
// The function is limited to 64 limbs.
func CmpMultiLimbs(comp *wizard.CompiledIOP, a, b LimbColumns) (isGreater, isEqual, isLower ifaces.Column, pa wizard.ProverAction) {

	if len(a.Limbs) != len(b.Limbs) {
		utils.Panic("a and b don't have the same number of limbs: %v %v", len(a.Limbs), len(b.Limbs))
	}

	if len(a.Limbs) == 0 || len(b.Limbs) == 0 {
		utils.Panic("a and b cannot have zero limbs %v %v", len(a.Limbs), len(b.Limbs))
	}

	if len(a.Limbs) > 31 || len(b.Limbs) > 31 {
		utils.Panic("a and b cannot have more than 31 limbs, the syndrom will overflow. a has %v limbs and b has %v limbs", len(a.Limbs), len(b.Limbs))
	}

	if a.LimbBitSize != b.LimbBitSize {
		utils.Panic("a and b don't use the same limb-bit-size: %v %v", a.LimbBitSize, b.LimbBitSize)
	}

	if a.IsBigEndian != b.IsBigEndian {
		utils.Panic("a and b don't have the same endianness")
	}

	var (
		nRows   = ifaces.AssertSameLength(a.Limbs...)
		nRowsB  = ifaces.AssertSameLength(b.Limbs...)
		ctxName = func(subName string) string {
			return fmt.Sprintf("CMP_MULTI_LIMB_%v_%v", comp.Columns.NumEntriesTotal(), subName)
		}
	)

	if nRows != nRowsB {
		utils.Panic("a and b must have the same length")
	}

	var (
		isBigEndian     = a.IsBigEndian
		roundA          = column.MaxRound(a.Limbs...)
		round           = max(roundA, column.MaxRound(b.Limbs...))
		numLimbs        = len(a.Limbs)
		numBitsPerLimbs = a.LimbBitSize
		ctx             = &MultiLimbCmp{
			IsGreater:          comp.InsertCommit(round, ifaces.ColIDf("%s", ctxName("IS_GREATER")), nRows, true),
			IsLower:            comp.InsertCommit(round, ifaces.ColIDf("%s", ctxName("IS_LOWER")), nRows, true),
			NonNegativeSyndrom: comp.InsertCommit(round, ifaces.ColIDf("%s", ctxName("MUST_BE_POSITIVE")), nRows, true),
		}

		syndromExpression = sym.NewConstant(0)
		allLimbsEqual     = sym.NewConstant(0)
	)

	for i := 0; i < numLimbs; i++ {
		var (
			g, e, l, lCtx = CmpSmallCols(comp, a.Limbs[i], b.Limbs[i], numBitsPerLimbs)
		)

		ctx.SubCtxs = append(ctx.SubCtxs, lCtx)
		allLimbsEqual = sym.Add(allLimbsEqual, sym.Sub(e, 1))

		factor := 1 << i
		if isBigEndian {
			factor = 1 << (numLimbs - i - 1)
		}

		syndromExpression = sym.Add(
			syndromExpression,
			sym.Mul(factor, sym.Sub(g, l)),
		)
	}

	comp.InsertGlobal(
		round,
		ifaces.QueryID(ctxName("IS_GREATER_IS_BINARY")),
		sym.Mul(ctx.IsGreater, sym.Sub(ctx.IsGreater, 1)),
	)

	comp.InsertGlobal(
		round,
		ifaces.QueryID(ctxName("IS_LOWER_IS_BINARY")),
		sym.Mul(ctx.IsLower, sym.Sub(ctx.IsLower, 1)),
	)

	ctx.IsEqualCtx = dedicated.IsZero(comp, allLimbsEqual)
	isEqual = ctx.IsEqualCtx.IsZero

	comp.InsertGlobal(
		round,
		ifaces.QueryIDf("%s", ctxName("FLAGS_MUTUALLY_EXCLUSIVE")),
		sym.Sub(1, ctx.IsGreater, isEqual, ctx.IsLower),
	)

	ctx.SyndromBoard = syndromExpression.Board()

	comp.InsertGlobal(
		round,
		ifaces.QueryID(ctxName("COMPUTE_NN_SYNDROME")),
		sym.Sub(
			ctx.NonNegativeSyndrom,
			sym.Mul(
				sym.Sub(ctx.IsGreater, ctx.IsLower),
				syndromExpression,
			),
		),
	)

	comp.InsertRange(
		round,
		ifaces.QueryID(ctxName("RANGE_CHECK_NN_SYNDROM")),
		ctx.NonNegativeSyndrom,
		1<<numLimbs,
	)

	return ctx.IsGreater, isEqual, ctx.IsLower, ctx
}

// Run implements the [wizard.ProverAction] interface.
func (mCmp *MultiLimbCmp) Run(run *wizard.ProverRuntime) {

	// This will assign the per-limbs comparision contexts
	parallel.Execute(len(mCmp.SubCtxs), func(start, stop int) {
		for i := start; i < stop; i++ {
			mCmp.SubCtxs[i].Run(run)
		}
	})

	// This will assign the IsEqual column. It can be done in parallel of the
	// the rest. But it requires the per-limb context to be run prior to this.
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		mCmp.IsEqualCtx.Run(run)
		wg.Done()
	}()

	var (
		syndrom   = column.EvalExprColumn(run, mCmp.SyndromBoard)
		isGreater = make([]field.Element, mCmp.IsGreater.Size())
		isLower   = make([]field.Element, mCmp.IsLower.Size())
		nnSyndrom = make([]field.Element, mCmp.IsLower.Size())
	)

	for i := range isGreater {
		var (
			sF = syndrom.Get(i)
		)

		// sf is positive
		if !sF.LexicographicallyLargest() && !sF.IsZero() {

			isGreater[i] = field.One()
			nnSyndrom[i] = sF
		}

		// sf is negitive
		if sF.LexicographicallyLargest() {

			isLower[i] = field.One()
			nnSyndrom[i].Neg(&sF)
		}

	}

	run.AssignColumn(mCmp.IsGreater.GetColID(), smartvectors.NewRegular(isGreater))
	run.AssignColumn(mCmp.IsLower.GetColID(), smartvectors.NewRegular(isLower))
	run.AssignColumn(mCmp.NonNegativeSyndrom.GetColID(), smartvectors.NewRegular(nnSyndrom))

	wg.Wait()
}
