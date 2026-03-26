package mimc

import (
	"strconv"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/mimc"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/parallel"
)

// HashingCtx is the wizard context responsible for hashing a table row-wise.
// It implements the [wizard.ProverAction] interface and is used to assign the
// InputColumns field.
type HashingCtx struct {
	// InputCols is the list of columns forming a table for which the current
	// context is computing the rows.
	InputCols []ifaces.Column
	// IntermediateHashes stores the intermediate values of the hasher.
	IntermediateHashes []ifaces.Column
}

// HashOf returns an [ifaces.Column] object containing the hash of the inputs
// columns and a [wizard.ProverAction] object responsible for assigning all
// the column taking part in justifying the returned column as well as the
// returned column itself.
func HashOf(comp *wizard.CompiledIOP, inputCols []ifaces.Column) *HashingCtx {

	var (
		ctx = &HashingCtx{
			InputCols:          inputCols,
			IntermediateHashes: make([]ifaces.Column, len(inputCols)),
		}
		round     = column.MaxRound(inputCols...)
		ctxID     = len(comp.ListCommitments())
		numRows   = ifaces.AssertSameLength(inputCols...)
		prevState = verifiercol.NewConstantCol(field.Zero(), numRows, strconv.Itoa(ctxID))
	)

	for i := range ctx.IntermediateHashes {
		ctx.IntermediateHashes[i] = comp.InsertCommit(
			round,
			ifaces.ColIDf("HASHING_%v_%v", ctxID, i),
			numRows,
		)

		comp.InsertMiMC(
			round,
			ifaces.QueryIDf("HASHING_%v_%v", ctxID, i),
			inputCols[i], prevState, ctx.IntermediateHashes[i],
			nil,
		)

		prevState = ctx.IntermediateHashes[i]
	}

	return ctx
}

// Result returns the column containing the result of the hashing.
func (ctx *HashingCtx) Result() ifaces.Column {
	return ctx.IntermediateHashes[len(ctx.IntermediateHashes)-1]
}

// Run implements the [wizard.ProverAction] interface
func (ctx *HashingCtx) Run(run *wizard.ProverRuntime) {

	var (
		numRow = ctx.InputCols[0].Size()
		numCol = len(ctx.InputCols)
		inputs = make([]smartvectors.SmartVector, numCol)
		interm = make([][]field.Element, numCol)
	)

	for i := range interm {
		inputs[i] = ctx.InputCols[i].GetColAssignment(run)
	}

	rangeStart, rangeStop := smartvectors.CoCompactRange(inputs...)

	for i := range interm {
		interm[i] = make([]field.Element, rangeStop-rangeStart)
	}

	parallel.Execute(rangeStop-rangeStart, func(t0, t1 int) {

		var (
			prevState = make([]field.Element, t1-t0)
			start     = rangeStart + t0
			stop      = rangeStart + t1
		)

		for i := range interm {
			for k := start; k < stop; k++ {
				interm[i][k] = mimc.BlockCompression(prevState[k-start], inputs[i].Get(k))
			}
			prevState = interm[i][start:stop]
		}
	})

	for i := range interm {
		run.AssignColumn(
			ctx.IntermediateHashes[i].GetColID(),
			smartvectors.FromCompactWithRange(interm[i], rangeStart, rangeStop, numRow),
		)
	}
}
