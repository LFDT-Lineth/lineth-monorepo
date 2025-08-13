package mimc

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/crypto/mimc"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/selector"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/variables"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/parallel"
)

/*
Holds intermediate values for the linear hash wizard
*/
type linearHashCtx struct {
	// The compiled IOP
	Comp *wizard.CompiledIOP

	Name string

	// Output column, which containing the result each
	// individual hash.
	ExpectedHash ifaces.Column
	// Column describing the hashing of each chunk
	ToHash, OldState, NewState, NewStateClean ifaces.Column

	// Name of the optional columns

	// IsFullyActive is a boolean flag indicating whether
	// the columns are entirely used in for hashing.
	// Name of the created columns
	IsFullyActive bool
	// Optional precomputed column indicating
	// if a row is active or not. And when a
	// hash is finished
	IsActiveLarge, IsActiveRowExpected, IsEndOfHash, Ones ifaces.Column

	// Period of the linear hash
	Period, NumHash int
	// Round of declaration of the linear hash
	Round int
}

/*
Check a linear hashby chunk of columns

  - tohash : (a, b, c, d) || (e, f, g, h)

  - period : 4 // indicates that we should hash chunks of 4 entries

  - expectedhash : (Hash(a, b, c, d) || Hash(e, f, g, h))
*/
func CheckLinearHash(
	comp *wizard.CompiledIOP,
	name string,
	tohash ifaces.Column,
	period int, numHash int,
	expectedHashes ifaces.Column,
) {

	// Initialize the context
	ctx := linearHashCtx{
		Comp:         comp,
		Name:         name,
		ToHash:       tohash,
		Period:       period,
		NumHash:      numHash,
		ExpectedHash: expectedHashes,
	}

	// Get the rounds
	ctx.Round = utils.Max(tohash.Round(), expectedHashes.Round())
	ctx.IsFullyActive = utils.IsPowerOfTwo(numHash * period)

	ctx.HashingCols()

	if ctx.IsFullyActive {
		selector.CheckSubsample(
			comp,
			prefixWithLinearHash(comp, name, "RES_EXTRACTION"),
			[]ifaces.Column{ctx.NewStateClean},
			[]ifaces.Column{ctx.ExpectedHash},
			period-1,
		)
	} else {
		ctx.Comp.InsertInclusion(
			ctx.Round,
			ifaces.QueryID(prefixWithLinearHash(comp, name, "RESULT_CHECK_%v", tohash.GetColID())),
			[]ifaces.Column{ctx.IsEndOfHash, ctx.NewStateClean},
			[]ifaces.Column{ctx.IsActiveExpected(), ctx.ExpectedHash},
		)
	}

}

func prefixWithLinearHash(comp *wizard.CompiledIOP, name, msg string, args ...any) string {
	args = append([]any{name, comp.SelfRecursionCount}, args...)
	return fmt.Sprintf("%v.LINEAR_HASH_%v_"+msg, args...)
}

type LinearHashProverAction struct {
	Ctx             *linearHashCtx
	OldStateID      ifaces.ColID
	NewStateID      ifaces.ColID
	NewStateCleanID ifaces.ColID
}

func (a *LinearHashProverAction) Run(run *wizard.ProverRuntime) {
	blocksWit := a.Ctx.ToHash.GetColAssignment(run)
	olds := make([]field.Element, a.Ctx.Period*a.Ctx.NumHash)
	news := make([]field.Element, a.Ctx.Period*a.Ctx.NumHash)

	parallel.Execute(a.Ctx.NumHash, func(start, stop int) {
		for hashID := start; hashID < stop; hashID++ {
			old := field.Zero()
			for i := 0; i < a.Ctx.Period; i++ {
				pos := hashID*a.Ctx.Period + i
				currentBlock := blocksWit.Get(pos)
				new := mimc.BlockCompression(old, currentBlock)
				olds[pos] = old
				news[pos] = new
				old = new
			}
		}
	})

	padNew := mimc.BlockCompression(field.Zero(), field.Zero())
	oldSV := smartvectors.RightZeroPadded(olds, a.Ctx.ToHash.Size())
	newSV := smartvectors.RightPadded(news, padNew, a.Ctx.ToHash.Size())
	newCleanSV := smartvectors.RightZeroPadded(vector.DeepCopy(news), a.Ctx.ToHash.Size())

	run.AssignColumn(a.OldStateID, oldSV)
	run.AssignColumn(a.NewStateID, newSV)
	run.AssignColumn(a.NewStateCleanID, newCleanSV)
}

// Declares assign and constraints the columns OldStates and NewStates
func (ctx *linearHashCtx) HashingCols() {

	// Registers the old states columns
	ctx.OldState = ctx.Comp.InsertCommit(
		ctx.Round,
		ifaces.ColID(prefixWithLinearHash(ctx.Comp, ctx.Name, "OLD_STATE_%v", ctx.ToHash.GetColID())),
		ctx.ToHash.Size(),
	)

	ctx.NewState = ctx.Comp.InsertCommit(
		ctx.Round,
		ifaces.ColID(prefixWithLinearHash(ctx.Comp, ctx.Name, "NEW_STATE_%v", ctx.ToHash.GetColID())),
		ctx.ToHash.Size(),
	)

	ctx.NewStateClean = ctx.Comp.InsertCommit(
		ctx.Round,
		ifaces.ColID(prefixWithLinearHash(ctx.Comp, ctx.Name, "NEW_STATE_CLEAN_%v", ctx.ToHash.GetColID())),
		ctx.ToHash.Size(),
	)

	ctx.Comp.RegisterProverAction(ctx.Round, &LinearHashProverAction{
		Ctx:             ctx,
		OldStateID:      ctx.OldState.GetColID(),
		NewStateID:      ctx.NewState.GetColID(),
		NewStateCleanID: ctx.NewStateClean.GetColID(),
	})

	// And registers queries for the initial values

	//
	// Propagation of the state within the chunks
	//
	expr := ifaces.ColumnAsVariable(ctx.NewState).
		Mul(ctx.IsActiveVar()).
		Mul(ctx.IsNotEndOfHashVar()).
		Sub(ifaces.ColumnAsVariable(column.Shift(ctx.OldState, 1)))

	ctx.Comp.InsertGlobal(
		ctx.Round,
		ifaces.QueryID(prefixWithLinearHash(ctx.Comp, ctx.Name, "STATE_PROPAGATION_%v", ctx.ToHash.GetColID())),
		expr,
		true, // no bound cancel to also enforce the first value of old state to be zero
	)

	//
	// Cleaning the new state
	//
	ctx.Comp.InsertGlobal(
		ctx.Round,
		ifaces.QueryID(prefixWithLinearHash(ctx.Comp, ctx.Name, "CLEAN_NEW_STATE_%v", ctx.ToHash.GetColID())),
		ctx.IsActiveVar().
			Mul(ifaces.ColumnAsVariable(ctx.NewState)).
			Sub(ifaces.ColumnAsVariable(ctx.NewStateClean)),
	)

	//
	// Correctness of the blocks compressions
	//
	ctx.Comp.InsertMiMC(
		ctx.Round,
		ifaces.QueryID(prefixWithLinearHash(ctx.Comp, ctx.Name, "BLOCKS_COMPRESSION_%v", ctx.ToHash.GetColID())),
		ctx.ToHash, ctx.OldState, ctx.NewState, nil,
	)

}

func (ctx *linearHashCtx) IsActiveExpected() ifaces.Column {
	if ctx.IsFullyActive {
		// Always active
		panic("asked isActiveExpected but the module is fully active")
	}

	// Lazily registers the columns
	if ctx.IsActiveRowExpected == nil {

		var assignment smartvectors.SmartVector

		if utils.IsPowerOfTwo(ctx.NumHash) {
			assignment = smartvectors.NewConstant(field.One(), ctx.NumHash)
		} else {
			assignment = smartvectors.RightZeroPadded(
				vector.Repeat(field.One(), ctx.NumHash),
				ctx.ExpectedHash.Size(),
			)
		}

		ctx.IsActiveRowExpected = ctx.Comp.InsertPrecomputed(
			ifaces.ColID(prefixWithLinearHash(ctx.Comp, ctx.Name, "IS_ACTIVE_EXPECTED_%v", ctx.ToHash.GetColID())),
			assignment,
		)
	}

	return ctx.IsActiveRowExpected
}

func (ctx *linearHashCtx) IsActiveVar() *symbolic.Expression {
	if ctx.IsFullyActive {
		// Always active
		return symbolic.NewConstant(1)
	}

	// Lazily registers the columns
	if ctx.IsActiveLarge == nil {
		ctx.IsActiveLarge = ctx.Comp.InsertPrecomputed(
			ifaces.ColID(prefixWithLinearHash(ctx.Comp, ctx.Name, "IS_ACTIVE_%v", ctx.ToHash.GetColID())),
			smartvectors.RightZeroPadded(
				vector.Repeat(field.One(), ctx.NumHash*ctx.Period),
				ctx.ToHash.Size(),
			),
		)
	}

	return ifaces.ColumnAsVariable(ctx.IsActiveLarge)
}

func (ctx *linearHashCtx) IsEndOfHashVar() *symbolic.Expression {
	if ctx.IsFullyActive {
		// Always active
		return variables.NewPeriodicSample(ctx.Period, ctx.Period-1)
	}

	// Lazily registers the columns
	if ctx.IsEndOfHash == nil {

		window := make([]field.Element, ctx.NumHash*ctx.Period)
		for i := range window {
			if i%ctx.Period == ctx.Period-1 {
				window[i].SetOne()
			}
		}

		ctx.IsEndOfHash = ctx.Comp.InsertPrecomputed(
			ifaces.ColID(prefixWithLinearHash(ctx.Comp, ctx.Name, "IS_END_OF_HASH_%v", ctx.ToHash.GetColID())),
			smartvectors.RightZeroPadded(window, ctx.ToHash.Size()),
		)
	}

	return ifaces.ColumnAsVariable(ctx.IsEndOfHash)
}

func (ctx *linearHashCtx) IsNotEndOfHashVar() *symbolic.Expression {
	return symbolic.NewConstant(1).Sub(ctx.IsEndOfHashVar())
}
