package poseidon2

import (
	"fmt"

	"github.com/consensys/gnark-crypto/field/koalabear/vortex"
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
	ExpectedHash [BlockSize]ifaces.Column
	// Column describing the hashing of each chunk
	ToHash, OldState, NewState, NewStateClean [BlockSize]ifaces.Column

	// Name of the optional columns

	// IsFullyActive is a boolean flag indicating whether
	// the columns are entirely used in for hashing.
	// Name of the created columns
	IsFullyActive bool
	// Optional precomputed column indicating
	// if a row is active or not. And when a
	// hash is finished
	IsActiveLarge, IsActiveRowExpected, IsEndOfHash, Ones ifaces.Column

	// ColChunks of the linear hash
	ColChunks, NumHash int
	// Round of declaration of the linear hash
	Round int
}

/*
Check a linear hash by chunk of columns
Inputs example:

  - numhash : 2

  - colChunks : 1

  - Input structure, we prepare the columns as follows before calling the function (last chunk of each hash input is left padded with zeroes if needed),
  each block of 8 elements is a block to be hashed.
    ToHash:
    [a₀,a₁,a₂,a₃,a₄,a₅,a₆,a₇, b₀,b₁,b₂,b₃,b₄,b₅,b₆,b₇]
    └─────── block a ──────┘  └─────── block b ──────┘

    // Split into 8 parallel columns:
    BlockToHash[0]: [a₀, b₀]  // First component of each block
    BlockToHash[1]: [a₁, b₁]  // Second component of each block
    ...
    BlockToHash[7]: [a₇, b₇]  // Eighth component of each block

  - expectedhash : (Hash(a₀,a₁,a₂,a₃,a₄,a₅,a₆,a₇) || Hash(b₀,b₁,b₂,b₃,b₄,b₅,b₆,b₇))
*/

func CheckLinearHash(
	comp *wizard.CompiledIOP,
	name string,
	colChunks int,
	tohash [BlockSize]ifaces.Column,
	numHash int,
	expectedHashes [BlockSize]ifaces.Column,
) {

	// Initialize the context
	ctx := linearHashCtx{
		Comp:         comp,
		Name:         name,
		ToHash:       tohash,
		ColChunks:    colChunks,
		NumHash:      numHash,
		ExpectedHash: expectedHashes,
	}

	// Get the rounds
	ctx.Round = utils.Max(tohash[0].Round(), expectedHashes[0].Round())
	ctx.IsFullyActive = utils.IsPowerOfTwo(numHash * colChunks)

	ctx.HashingCols()

	stackedNewStateClean := make([]ifaces.Column, 0, BlockSize)
	for block := 0; block < BlockSize; block++ {
		stackedNewStateClean = append(stackedNewStateClean,
			ctx.NewStateClean[block],
		)
	}

	stackedExpectedHash := make([]ifaces.Column, 0, BlockSize)
	for block := 0; block < BlockSize; block++ {
		stackedExpectedHash = append(stackedExpectedHash,
			ctx.ExpectedHash[block],
		)
	}

	if ctx.IsFullyActive {

		selector.CheckSubsample(
			comp,
			prefixWithLinearHash(comp, name, "RES_EXTRACTION"),
			stackedNewStateClean,
			stackedExpectedHash,
			colChunks-1,
		)

	} else {

		stackedNewStateClean = append(stackedNewStateClean,
			ctx.IsEndOfHash,
		)

		stackedExpectedHash = append(stackedExpectedHash,
			ctx.IsActiveExpected(),
		)

		ctx.Comp.InsertInclusion(
			ctx.Round,
			ifaces.QueryID(prefixWithLinearHash(comp, name, "RESULT_CHECK")),
			stackedNewStateClean,
			stackedExpectedHash,
		)
	}

}

func prefixWithLinearHash(comp *wizard.CompiledIOP, name, msg string, args ...any) string {
	args = append([]any{name, comp.SelfRecursionCount}, args...)
	return fmt.Sprintf("%v.LINEAR_HASH_%v_"+msg, args...)
}

type LinearHashProverAction struct {
	Ctx             *linearHashCtx
	OldStateID      [BlockSize]ifaces.ColID
	NewStateID      [BlockSize]ifaces.ColID
	NewStateCleanID [BlockSize]ifaces.ColID
}

func (a *LinearHashProverAction) Run(run *wizard.ProverRuntime) {
	var blocksWit [BlockSize]smartvectors.SmartVector
	var olds, news [BlockSize][]field.Element
	for i := 0; i < BlockSize; i++ {
		blocksWit[i] = a.Ctx.ToHash[i].GetColAssignment(run)

		olds[i] = make([]field.Element, a.Ctx.ColChunks*a.Ctx.NumHash)
		news[i] = make([]field.Element, a.Ctx.ColChunks*a.Ctx.NumHash)
	}
	var zeroBlock field.Octuplet
	parallel.Execute(a.Ctx.NumHash, func(start, stop int) {
		for hashID := start; hashID < stop; hashID++ {
			old := zeroBlock
			var currentBlock [BlockSize]field.Element
			for i := 0; i < a.Ctx.ColChunks; i++ {
				pos := hashID*a.Ctx.ColChunks + i
				for j := 0; j < BlockSize; j++ {
					currentBlock[j] = blocksWit[j].Get(pos)
				}
				new := vortex.CompressPoseidon2(old, currentBlock)
				for j := 0; j < BlockSize; j++ {
					olds[j][pos] = old[j]
					news[j][pos] = new[j]
				}
				old = new

			}

		}
	})

	padNew := vortex.CompressPoseidon2(zeroBlock, zeroBlock)

	var oldSV, newSV, newCleanSV [BlockSize]smartvectors.SmartVector
	for i := 0; i < BlockSize; i++ {
		oldSV[i] = smartvectors.RightZeroPadded(olds[i], a.Ctx.ToHash[i].Size())
		newSV[i] = smartvectors.RightPadded(news[i], padNew[i], a.Ctx.ToHash[i].Size())
		newCleanSV[i] = smartvectors.RightZeroPadded(vector.DeepCopy(news[i]), a.Ctx.ToHash[i].Size())

		run.AssignColumn(a.OldStateID[i], oldSV[i])
		run.AssignColumn(a.NewStateID[i], newSV[i])
		run.AssignColumn(a.NewStateCleanID[i], newCleanSV[i])
	}
}

// Declares assign and constraints the columns OldStates and NewStates
func (ctx *linearHashCtx) HashingCols() {

	for i := 0; i < BlockSize; i++ {
		// Registers the old states columns
		ctx.OldState[i] = ctx.Comp.InsertCommit(
			ctx.Round,
			ifaces.ColID(prefixWithLinearHash(ctx.Comp, ctx.Name, "OLD_STATE_%v_%v", ctx.ToHash[i].GetColID(), i)),
			ctx.ToHash[i].Size(),
			true,
		)

		ctx.NewState[i] = ctx.Comp.InsertCommit(
			ctx.Round,
			ifaces.ColID(prefixWithLinearHash(ctx.Comp, ctx.Name, "NEW_STATE_%v_%v", ctx.ToHash[i].GetColID(), i)),
			ctx.ToHash[i].Size(),
			true,
		)

		ctx.NewStateClean[i] = ctx.Comp.InsertCommit(
			ctx.Round,
			ifaces.ColIDf("%s", prefixWithLinearHash(ctx.Comp, ctx.Name, "NEW_STATE_CLEAN_%v", ctx.ToHash[i].GetColID())),
			ctx.ToHash[i].Size(),
			true,
		)
	}
	var olfStateID, newStateID, newStateCleanID [BlockSize]ifaces.ColID

	for i := 0; i < BlockSize; i++ {

		olfStateID[i] = ctx.OldState[i].GetColID()
		newStateID[i] = ctx.NewState[i].GetColID()
		newStateCleanID[i] = ctx.NewStateClean[i].GetColID()
	}
	ctx.Comp.RegisterProverAction(ctx.Round, &LinearHashProverAction{
		Ctx:             ctx,
		OldStateID:      olfStateID,
		NewStateID:      newStateID,
		NewStateCleanID: newStateCleanID,
	})

	// And registers queries for the initial values

	//
	// Propagation of the state within the chunks
	//
	for i := 0; i < BlockSize; i++ {
		expr := ifaces.ColumnAsVariable(ctx.NewState[i]).
			Mul(ctx.IsActiveVar()).
			Mul(ctx.IsNotEndOfHashVar()).
			Sub(ifaces.ColumnAsVariable(column.Shift(ctx.OldState[i], 1)))

		ctx.Comp.InsertGlobal(
			ctx.Round,
			ifaces.QueryID(prefixWithLinearHash(ctx.Comp, ctx.Name, "STATE_PROPAGATION_%v_%v", ctx.ToHash[i].GetColID(), i)),
			expr,
			true, // no bound cancel to also enforce the first value of old state to be zero
		)

		//
		// Cleaning the new state
		//
		ctx.Comp.InsertGlobal(
			ctx.Round,
			ifaces.QueryIDf("%s", prefixWithLinearHash(ctx.Comp, ctx.Name, "CLEAN_NEW_STATE_%v_%v", ctx.ToHash[i].GetColID(), i)),
			ctx.IsActiveVar().
				Mul(ifaces.ColumnAsVariable(ctx.NewState[i])).
				Sub(ifaces.ColumnAsVariable(ctx.NewStateClean[i])),
		)

	}
	//
	// Correctness of the blocks compressions
	//
	ctx.Comp.InsertPoseidon2(
		ctx.Round,
		ifaces.QueryID(prefixWithLinearHash(ctx.Comp, ctx.Name, "BLOCKS_COMPRESSION")),
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
				ctx.ExpectedHash[0].Size(),
			)
		}

		ctx.IsActiveRowExpected = ctx.Comp.InsertPrecomputed(
			ifaces.ColIDf("%s", prefixWithLinearHash(ctx.Comp, ctx.Name, "IS_ACTIVE_EXPECTED_%v", ctx.ToHash[0].GetColID())),
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
			ifaces.ColIDf("%s", prefixWithLinearHash(ctx.Comp, ctx.Name, "IS_ACTIVE_%v", ctx.ToHash[0].GetColID())),
			smartvectors.RightZeroPadded(
				vector.Repeat(field.One(), ctx.NumHash*ctx.ColChunks),
				ctx.ToHash[0].Size(),
			),
		)
	}

	return ifaces.ColumnAsVariable(ctx.IsActiveLarge)
}

func (ctx *linearHashCtx) IsEndOfHashVar() *symbolic.Expression {
	if ctx.IsFullyActive {
		// Always active
		return variables.NewPeriodicSample(ctx.ColChunks, ctx.ColChunks-1)
	}

	// Lazily registers the columns
	if ctx.IsEndOfHash == nil {

		window := make([]field.Element, ctx.NumHash*ctx.ColChunks)
		for i := range window {
			if i%ctx.ColChunks == ctx.ColChunks-1 {
				window[i].SetOne()
			}
		}

		ctx.IsEndOfHash = ctx.Comp.InsertPrecomputed(
			ifaces.ColIDf("%s", prefixWithLinearHash(ctx.Comp, ctx.Name, "IS_END_OF_HASH_%v", ctx.ToHash[0].GetColID())),
			smartvectors.RightZeroPadded(window, ctx.ToHash[0].Size()),
		)
	}

	return ifaces.ColumnAsVariable(ctx.IsEndOfHash)
}

func (ctx *linearHashCtx) IsNotEndOfHashVar() *symbolic.Expression {
	return symbolic.NewConstant(1).Sub(ctx.IsEndOfHashVar())
}

// PrepareToHashWitness pads a segment to the full chunked size and reshapes it
// into blockSize columns for hashing.
func PrepareToHashWitness(th [BlockSize][]field.Element, segment []field.Element, start int) [BlockSize][]field.Element {
	colSize := len(segment)
	for j := 0; j < BlockSize; j++ {
		// Allocate segments to TOHASH columns
		completeChunks := colSize / BlockSize
		for k := 0; k < completeChunks; k++ {
			th[j][k+start] = segment[k*BlockSize+j]
		}

		lastChunkElements := colSize % BlockSize
		lastChunkPadding := 0
		if lastChunkElements > 0 {
			lastChunkPadding = BlockSize - lastChunkElements

			k := completeChunks
			if j < lastChunkPadding {
				// Left padding
				th[j][k+start] = field.Zero()
			} else {
				// Actual data
				actualIdx := k*BlockSize + (j - lastChunkPadding)
				th[j][k+start] = segment[actualIdx]
			}
		}
	}
	return th
}
