package poseidon2

import (
	"strconv"

	"github.com/consensys/gnark-crypto/field/koalabear/vortex"
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

const BlockSize = poseidon2_koalabear.BlockSize

// HashingCtx is the wizard context responsible for hashing a table row-wise.
// It implements the [wizard.ProverAction] interface and is used to assign the
// InputColumns field.
type HashingCtx struct {
	// InputCols is the list of columns forming a table for which the current
	// context is computing the rows.
	InputCols [][BlockSize]ifaces.Column
	// IntermediateHashes stores the intermediate values of the hasher.
	IntermediateHashes [][BlockSize]ifaces.Column
	// Name is the name of the current context.
	Name string
}

// MaxOctRound round of declaration for a list of commitment
func MaxOctRound(handles ...[BlockSize]ifaces.Column) int {
	res := 0
	for _, handle := range handles {
		res = utils.Max(res, handle[0].Round())
	}
	return res
}

// AssertSameLength is a utility function comparing the Size of all the columns
// in `list` , panicking if the lengths are not all the same and returning the
// shared length otherwise.
func AssertOctSameLength(list ...[BlockSize]ifaces.Column) int {
	if len(list) == 0 {
		panic("passed an empty leaf")
	}

	res := list[0][0].Size()
	for i := range list {
		if list[i][0].Size() != res {
			utils.Panic("the column %v (size %v) does not have the same size as column 0 (size %v)", i, list[i][0].Size(), res)
		}
	}

	return res
}

// HashOf returns an [ifaces.Column] object containing the hash of the inputs
// columns and a [wizard.ProverAction] object responsible for assigning all
// the column taking part in justifying the returned column as well as the
// returned column itself.
func HashOf(comp *wizard.CompiledIOP, inputCols []ifaces.Column, name string) *HashingCtx {

	var (
		inputBlocks = splitColumns(inputCols)
		round       = column.MaxRound(inputCols...)
		ctxID       = len(comp.ListCommitments())
		numRows     = ifaces.AssertSameLength(inputCols...)

		ctx = &HashingCtx{
			Name:               name,
			InputCols:          inputBlocks,
			IntermediateHashes: make([][BlockSize]ifaces.Column, len(inputBlocks)),
		}

		prevState [BlockSize]ifaces.Column
	)

	for i := 0; i < BlockSize; i++ {
		prevState[i] = verifiercol.NewConstantCol(field.Zero(), numRows, strconv.Itoa(ctxID))
	}

	for i := range ctx.IntermediateHashes {

		subName := ifaces.ColIDf("HASHING_%v_%v", ctxID, i)
		if i == len(ctx.IntermediateHashes)-1 {
			subName = ifaces.ColIDf("%v_%v", name, i)
		}

		for j := 0; j < BlockSize; j++ {
			ctx.IntermediateHashes[i][j] = comp.InsertCommit(
				round,
				ifaces.ColIDf("%v_%v", subName, j),
				numRows,
				true,
			)
		}

		comp.InsertPoseidon2(
			round,
			ifaces.QueryIDf("%v_%v", subName, i),
			ctx.InputCols[i], prevState, ctx.IntermediateHashes[i],
			nil,
		)

		prevState = ctx.IntermediateHashes[i]
	}

	return ctx
}

// Result returns the column containing the result of the hashing.
func (ctx *HashingCtx) Result() [BlockSize]ifaces.Column {
	return ctx.IntermediateHashes[len(ctx.IntermediateHashes)-1]
}

// Run implements the [wizard.ProverAction] interface
func (ctx *HashingCtx) Run(run *wizard.ProverRuntime) {

	var (
		numRow = ctx.InputCols[0][0].Size()
		numCol = len(ctx.InputCols)
		inputs = make([][BlockSize]smartvectors.SmartVector, len(ctx.InputCols))
		interm = make([][]field.Octuplet, numCol)
	)

	for i := range inputs {
		for j := range inputs[i] {
			inputs[i][j] = ctx.InputCols[i][j].GetColAssignment(run)
		}
	}

	rangeStart, rangeStop := smartvectors.CoCompactRange(flattenBlocks(inputs)...)

	for i := range interm {
		interm[i] = make([]field.Octuplet, rangeStop-rangeStart)
	}

	parallel.Execute(rangeStop-rangeStart, func(t0, t1 int) {

		var (
			prevState = make([]field.Octuplet, t1-t0)
			start     = rangeStart + t0
			stop      = rangeStart + t1
		)

		for i := range interm {
			for k := start; k < stop; k++ {
				var block [BlockSize]field.Element
				for j := 0; j < BlockSize; j++ {
					block[j] = inputs[i][j].Get(k)
				}
				interm[i][k] = vortex.CompressPoseidon2(prevState[k-start], block)
			}
			prevState = interm[i][start:stop]
		}
	})

	for i := range interm {
		for j := 0; j < BlockSize; j++ {
			intermSlice := make([]field.Element, len(interm[i]))
			for k := range interm[i] {
				intermSlice[k] = interm[i][k][j]
			}
			run.AssignColumn(
				ctx.IntermediateHashes[i][j].GetColID(),
				smartvectors.FromCompactWithRange(intermSlice, rangeStart, rangeStop, numRow),
			)
		}
	}
}

// flattenBlocks flattens the input slice of blocks into a single slice of columns.
func flattenBlocks[T any](blocks [][poseidon2_koalabear.BlockSize]T) []T {
	var res []T
	for _, block := range blocks {
		res = append(res, block[:]...)
	}
	return res
}

// splitColumns splits the input slice into subarrays of size poseidon2.BlockSize. If the input is not divisible by the size, it appends constant verifier columns to the input to make it divisible by the size.
func splitColumns(input []ifaces.Column) [][poseidon2_koalabear.BlockSize]ifaces.Column {

	var (
		blockSize = poseidon2_koalabear.BlockSize
		constCol  = verifiercol.NewConstantCol(field.Zero(), input[0].Size(), "CONSTANT_COLUMN")
		res       = [][poseidon2_koalabear.BlockSize]ifaces.Column{}
	)

	for len(input) > 0 {
		var buf [poseidon2_koalabear.BlockSize]ifaces.Column
		if len(input) > len(buf) {
			copy(buf[:], input[:blockSize])
			input = input[blockSize:]
		} else {
			// left padding with zeroes
			for j := 0; j < len(buf)-len(input); j++ {
				buf[j] = constCol
			}
			copy(buf[len(buf)-len(input):], input)
			input = input[:0]
		}
		res = append(res, buf)
	}

	return res
}
