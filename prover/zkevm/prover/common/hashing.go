package common

import (
	"strconv"

	"github.com/consensys/linea-monorepo/prover/crypto/poseidon2_koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/parallel"
)

// HashingCtx is the wizard context responsible for hashing a table row-wise.
// It implements the [wizard.ProverAction] interface and is used to assign the
// InputColumns field.
type HashingCtx struct {
	// InputCols is the list of columns forming a table for which the current
	// context is computing the rows.
	InputCols [][NbElemPerHash]ifaces.Column
	// IntermediateHashes stores the intermediate values of the hasher.
	IntermediateHashes [][NbElemPerHash]ifaces.Column
}

// HashOf returns an [ifaces.Column] object containing the hash of the inputs
// columns and a [wizard.ProverAction] object responsible for assigning all
// the column taking part in justifying the returned column as well as the
// returned column itself.
func HashOf(comp *wizard.CompiledIOP, inputCols [][NbElemPerHash]ifaces.Column) ([NbElemPerHash]ifaces.Column, wizard.ProverAction) {

	var (
		ctx = &HashingCtx{
			InputCols:          inputCols,
			IntermediateHashes: make([][NbElemPerHash]ifaces.Column, len(inputCols)),
		}
		round     = column.MaxRound(inputCols[0][:]...)
		ctxID     = len(comp.ListCommitments())
		numRows   = ifaces.AssertSameLength(inputCols[0][:]...)
		prevState [NbElemPerHash]ifaces.Column
	)

	for i := range inputCols {
		round = max(round, column.MaxRound(inputCols[i][:]...))
		if numRows != ifaces.AssertSameLength(inputCols[i][:]...) {
			utils.Panic("all input columns must have the same length")
		}
	}

	for i := range prevState {
		prevState[i] = verifiercol.NewConstantCol(field.Zero(), numRows, "hash-of-"+strconv.Itoa(ctxID))
	}

	for i := range ctx.IntermediateHashes {
		for j := range NbElemPerHash {

			ctx.IntermediateHashes[i][j] = comp.InsertCommit(
				round,
				ifaces.ColIDf("HASHING_%v_%v_%v", ctxID, i, j),
				numRows,
				true,
			)
		}

		comp.InsertPoseidon2(
			round,
			ifaces.QueryIDf("HASHING_%v_%v", ctxID, i),
			inputCols[i], prevState, ctx.IntermediateHashes[i],
			nil,
		)

		prevState = ctx.IntermediateHashes[i]
	}

	return ctx.IntermediateHashes[len(inputCols)-1], ctx
}

// Run implements the [wizard.ProverAction] interface
func (ctx *HashingCtx) Run(run *wizard.ProverRuntime) {

	var (
		numRow       = ctx.InputCols[0][0].Size()
		numCol       = len(ctx.InputCols)
		inputs       = make([][NbElemPerHash][]field.Element, numCol)
		interm       = make([][NbElemPerHash][]field.Element, numCol)
		initialState [NbElemPerHash][]field.Element
	)

	for k := range NbElemPerHash {
		for i := range interm {
			inputs[i][k] = ctx.InputCols[i][k].GetColAssignment(run).IntoRegVecSaveAlloc()
			interm[i][k] = make([]field.Element, numRow)
		}
		initialState[k] = make([]field.Element, numRow)
	}

	parallel.Execute(numRow, func(start, stop int) {
		prevState := initialState
		for i := range interm {
			poseidon2VecCompression(prevState, inputs[i], interm[i], start, stop)
			prevState = interm[i]
		}
	})

	for i := range interm {
		for k := range NbElemPerHash {
			run.AssignColumn(
				ctx.IntermediateHashes[i][k].GetColID(),
				smartvectors.NewRegular(interm[i][k]),
			)
		}
	}
}

func poseidon2VecCompression(oldState, block, newState [NbElemPerHash][]field.Element, from, to int) {

	if len(oldState) == 0 {
		return
	}

	var rowBlock, rowOldState [NbElemPerHash]field.Element

	for i := from; i < to; i++ {
		for k := range NbElemPerHash {
			rowBlock[k] = block[k][i]
			rowOldState[k] = oldState[k][i]
		}
		newStateRow := poseidon2_koalabear.Compress(rowOldState, rowBlock)
		for k := range NbElemPerHash {
			newState[k][i] = newStateRow[k]
		}
	}
}
