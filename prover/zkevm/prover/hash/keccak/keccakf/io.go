package keccakf

import (
	"github.com/consensys/linea-monorepo/prover/crypto/keccak"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed/pragmas"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	commonconstraints "github.com/consensys/linea-monorepo/prover/zkevm/prover/common/common_constraints"
)

const (
	// number of slices in keccakf for base conversion
	numSlices = 16

	// number of lanes in the output of hash
	// digest is 256 bits that is 4 lanes of 64bits
	numLanesInHashOutPut = 4
)

type InputOutput struct {
	// it is 1 iff BlockBaseB is inserted
	IsBlockBaseB ifaces.Column
	// it is 1 iff firstBlock is inserted
	IsFirstBlock ifaces.Column
	//Sum of IsBlockBaseB and IsFirstBlock
	IsBlock ifaces.Column
	// it indicates where the hash result is located
	IsHashOutPut ifaces.Column
	// PiChiIota submodule contains the hash result, in specific columns and rows
	PiChiIota piChiIota
	// the hash result
	HashOutputSlicesBaseB [numLanesInHashOutPut][numSlices]ifaces.Column
	// active part of HashOutputSlicesBaseB
	IsActive ifaces.Column
}

/*
	The method newInput declares the columns specific to the submodule.

It also declares the constraints over the input/output of keccakF.

1. any permutation should be relevant to firstBlock or BlockBaseB (but not to both).

2. for an ongoing permutation the output of aIota is fed back to the state

3. for the permutations from the same hash the output of aIota is fed back to the state

Note: The initial state (of our implementation) is supposed to be the first Block.
No constraint in keccakF module is asserting to this fact.
Instead, it is guaranteed via a projection query between output of data-transfer module and keccakF

This is tanks to the fact that;     initial state (of original keccakF) = 0
And so,  FirstBlock XOR (original) initial state = our initial state.
Therefore we can directly project from the output of dataTransfer module to the keccakF state.
*/
func (io *InputOutput) newInput(comp *wizard.CompiledIOP, maxNumKeccakF int,
	mod Module,
) {
	lu := mod.Lookups
	io.PiChiIota = mod.PiChiIota
	input := mod.State

	// declare the columns
	io.declareColumnsInput(comp, maxNumKeccakF)

	// declare the constraints
	commonconstraints.MustBeActivationColumns(comp, mod.IsActive)
	// Binary Column
	commonconstraints.MustBeBinary(comp, io.IsBlock)
	commonconstraints.MustBeBinary(comp, io.IsBlockBaseB)
	commonconstraints.MustBeBinary(comp, io.IsFirstBlock)

	commonconstraints.MustZeroWhenInactive(comp, mod.IsActive,
		io.IsBlockBaseB,
		io.IsFirstBlock,
	)

	// IsBlock = isFirstBlock + isBlockBaseB
	comp.InsertGlobal(0, ifaces.QueryIDf("IsBlockMassage"),
		sym.Sub(io.IsBlock,
			sym.Add(io.IsFirstBlock, io.IsBlockBaseB),
		),
	)

	// usePrevIota = 1- (IsFirstBlock[i]+ IsBlockBaseB[i-1])
	comp.InsertGlobal(0, ifaces.QueryIDf("UsePrevIota_SET_TO_ZERO_OVER_BLOCKS"),
		sym.Mul(mod.IsActive,
			sym.Sub(
				sym.Add(io.IsFirstBlock, column.Shift(io.IsBlockBaseB, -1)),
				lu.DontUsePrevAIota.Natural,
			),
		),
	)

	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			m := 5*y + x
			if m < numLanesInBlock {
				// for a new hash (17 first columns of) the state is the first block
				comp.InsertGlobal(0, ifaces.QueryIDf("STATE_IS_SET_TO_FIRST_BLOCK_%v_%v", x, y),
					sym.Mul(io.IsFirstBlock,
						sym.Sub(input[x][y], mod.Blocks[m]),
					),
				)
			} else {
				//  the remaining columns of the state are set to zero
				comp.InsertGlobal(0, ifaces.QueryIDf("STATE_IS_SET_TO_ZERO_%v,%v", x, y),
					sym.Mul(io.IsFirstBlock, input[x][y]),
				)
			}
		}
	}

}

func (io *InputOutput) newOutput(comp *wizard.CompiledIOP, maxNumKeccakF int,
	mod Module,
) {
	lu := mod.Lookups
	io.PiChiIota = mod.PiChiIota
	input := mod.State

	io.declareColumnsOutput(comp, maxNumKeccakF)

	// IsActive has the activation form
	commonconstraints.MustBeActivationColumns(comp, io.IsActive)

	// IsHashOutPut[i] = 1 if either;
	// - IsFirstBlock[i+1]  = 1
	// - IsActive[i] = 1 and IsActive[i+1] = 0
	//
	//	Note: both conditions are incompatible
	comp.InsertGlobal(0,
		ifaces.QueryIDf("IS_HASH_OUTPUT_IS_WELL_SET"),
		sym.Sub(
			io.IsHashOutPut,
			column.Shift(io.IsFirstBlock, 1),
			sym.Sub(mod.IsActive, column.Shift(mod.IsActive, 1)),
		),
	)

	// constrains over the next sate;
	//  - for an ongoing permutation, the next state equals with the last aIota
	//
	//  - for the next permutation from the same hash, the state equals with the last aIota
	io.csNextState(comp, io.PiChiIota, lu, input)

	// constraints over hash output
	io.csHashOutput(comp)

}

// It declares the columns specific to the submodule.
func (io *InputOutput) declareColumnsInput(comp *wizard.CompiledIOP, maxNumKeccakF int) {
	var (
		size      = numRows(maxNumKeccakF)
		createCol = common.CreateColFn(comp, "KECCAKF_INPUT_MODULE", size, pragmas.RightPadded)
	)

	io.IsFirstBlock = createCol("IS_FIRST_BLOCK")
	io.IsBlockBaseB = createCol("IS_BLOCK_BaseB")
	io.IsBlock = createCol("IS_BLOCK")
}

// It declares the columns specific to the submodule.
func (io *InputOutput) declareColumnsOutput(comp *wizard.CompiledIOP, maxNumKeccakF int) {
	var (
		size      = utils.NextPowerOfTwo(maxNumKeccakF)
		createCol = common.CreateColFn(comp, "KECCAKF_OUTPUT_MODULE", size, pragmas.RightPadded)
	)
	for j := range io.HashOutputSlicesBaseB {
		for k := range io.HashOutputSlicesBaseB[0] {
			io.HashOutputSlicesBaseB[j][k] = createCol("HashOutPut_SlicesBaseB_%v_%v", j, k)
		}
	}
	io.IsActive = createCol("HASH_IS_ACTIVE")

	var (
		sizeState = numRows(maxNumKeccakF)
	)
	io.IsHashOutPut = comp.InsertCommit(0, ifaces.ColIDf("KECCAKF_IS_HASH_OUTPUT"), sizeState)
}

// The constrain over the next sate;
func (io *InputOutput) csNextState(
	comp *wizard.CompiledIOP,
	pci piChiIota,
	lu lookUpTables,
	input [5][5]ifaces.Column,
) {

	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			// recompose the slices to get aIota in BaseA
			recomposedAIota := BaseRecomposeSliceHandles(
				// Recall that the PiChiIota module performs all the steps pi-chi-iota
				// at once.
				pci.AIotaBaseASliced[x][y][:],
				BaseA,
				true,
			)
			// for an ongoing permutation or for permutations from the same hash;
			// impose that the previous aIota in base A should be equal with the state.
			usePrevIota := sym.Add(column.Shift(io.IsBlockBaseB, -1), sym.Sub(1, lu.DontUsePrevAIota.Natural)) // isBlockBaseB[i-1] + UsePrevAIota[i]
			comp.InsertGlobal(0, ifaces.QueryIDf("AIOTA_TO_A_%v_%v", x, y),
				sym.Mul(usePrevIota,
					sym.Sub(input[x][y], recomposedAIota)),
			)
		}
	}

}

// It connect the data-transfer module to the keccakf module via a projection query over the blocks.
func (io *InputOutput) csHashOutput(comp *wizard.CompiledIOP) {

	// constraints over info-module (outputs
	colA := append(io.PiChiIota.AIotaBaseBSliced[0][0][:], io.PiChiIota.AIotaBaseBSliced[1][0][:]...)
	colA = append(colA, io.PiChiIota.AIotaBaseBSliced[2][0][:]...)
	colA = append(colA, io.PiChiIota.AIotaBaseBSliced[3][0][:]...)

	colB := append(io.HashOutputSlicesBaseB[0][:], io.HashOutputSlicesBaseB[1][:]...)
	colB = append(colB, io.HashOutputSlicesBaseB[2][:]...)
	colB = append(colB, io.HashOutputSlicesBaseB[3][:]...)

	comp.InsertProjection(ifaces.QueryIDf("HashOutput_Projection"),
		query.ProjectionInput{ColumnA: colB,
			ColumnB: colA,
			FilterA: io.IsActive,
			FilterB: io.IsHashOutPut})
}

// It assigns the columns specific to the submodule.
func (io *InputOutput) assignBlockFlags(
	run *wizard.ProverRuntime,
	permTrace keccak.PermTraces,
) {
	var (
		isFirstBlock = common.NewVectorBuilder(io.IsFirstBlock)
		isBlockBaseB = common.NewVectorBuilder(io.IsBlockBaseB)
		isBlock      = common.NewVectorBuilder(io.IsBlock)
	)

	zeroes := make([]field.Element, keccak.NumRound-1)
	for i := range permTrace.IsNewHash {
		if permTrace.IsNewHash[i] {
			isFirstBlock.PushOne()
			isFirstBlock.PushSliceF(zeroes)
			// append 24 zeroes
			isBlockBaseB.PushSliceF(zeroes)
			isBlockBaseB.PushInt(0)
			// populate IsBlock
			isBlock.PushOne()
			isBlock.PushSliceF(zeroes)

		} else {
			isFirstBlock.PushZero()
			isFirstBlock.PushSliceF(zeroes)

			isBlockBaseB.OverWriteInt(1)
			// append 24 zeroes
			isBlockBaseB.PushSliceF(zeroes)
			isBlockBaseB.PushZero()

			//populate IsBlock
			isBlock.OverWriteInt(1)
			isBlock.PushSliceF(zeroes)
			isBlock.PushZero()
		}
	}

	isBlock.PadAndAssign(run)
	isFirstBlock.PadAndAssign(run)
	isBlockBaseB.PadAndAssign(run)

}

// It assigns the columns
func (io *InputOutput) assignHashOutPut(run *wizard.ProverRuntime, isBlockActive ifaces.Column) {
	var (
		aIota            = io.PiChiIota.AIotaBaseBSliced
		isFirstBlock     = io.IsFirstBlock.GetColAssignment(run).IntoRegVecSaveAlloc()
		isHashOutput     = common.NewVectorBuilder(io.IsHashOutPut)
		hashSlices       = make([][]*common.VectorBuilder, numLanesInHashOutPut)
		isActive         = common.NewVectorBuilder(io.IsActive)
		isBlockActiveWit = isBlockActive.GetColAssignment(run).IntoRegVecSaveAlloc()
	)
	// populate IsHashOutput
	for row := 1; row < len(isFirstBlock); row++ {
		if isBlockActiveWit[row].IsOne() {
			isHashOutput.PushField(isFirstBlock[row])
		}

	}
	isHashOutput.PushInt(1)
	isHashOutput.PadAndAssign(run)

	// populate HashOutputSlicesBaseB
	isHashOutputWit := isHashOutput.Slice()
	for j := range io.HashOutputSlicesBaseB {
		hashSlices[j] = make([]*common.VectorBuilder, numSlice)
		for k := range io.HashOutputSlicesBaseB[0] {
			hashSlices[j][k] = common.NewVectorBuilder(io.HashOutputSlicesBaseB[j][k])

			aIotaWit := aIota[j][0][k].GetColAssignment(run).IntoRegVecSaveAlloc()
			for i := range isHashOutputWit {
				if isHashOutputWit[i] == field.One() {
					hashSlices[j][k].PushField(aIotaWit[i])
				}
			}
			hashSlices[j][k].PadAndAssign(run)

		}
	}

	//populate is active
	for i := range isHashOutputWit {
		if isHashOutputWit[i].IsOne() {
			isActive.PushInt(1)

		}
	}
	isActive.PadAndAssign(run)
}
