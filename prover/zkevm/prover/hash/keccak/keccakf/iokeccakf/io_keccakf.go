package iokeccakf

import (
	"github.com/consensys/linea-monorepo/prover/crypto/keccak"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils/parallel"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	commonconstraints "github.com/consensys/linea-monorepo/prover/zkevm/prover/common/common_constraints"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/generic"
	baseconversion "github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/keccak/base_conversion"
	kcommon "github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/keccak/keccakf/common"
)

const (
	NbOfRowsPerLane = 4
	NumSlices       = kcommon.NumSlices
	MAXNBYTE        = 2
)

type LaneInfo struct {
	Lane                 ifaces.Column // from packing
	IsBeginningOfNewHash ifaces.Column
	IsLaneActive         ifaces.Column
}

type KeccakFBlocks struct {
	Inputs           LaneInfo
	IsFromFirstBlock ifaces.Column // built from isBeginningOfNewHash and rowsPerBlock of keccakf
	IsFromBlockBaseB ifaces.Column
	Blocks           [][NumSlices]ifaces.Column // the blocks of the message to be absorbed. first blocks of messages are located in positions 0 mod 24 and are represented in base clean 12, other blocks of message are located in positions 23 mod 24 and are represented in base clean 11. otherwise the blocks are zero.
	IsBlock          ifaces.Column              // indicates whether the row corresponds to a block
	// prover action for base conversion
	Bc            *baseconversion.ToBaseX
	IsFirstBlock  ifaces.Column
	IsBlockBaseB  ifaces.Column
	IsBlockActive ifaces.Column // active part of the blocks (technicaly it is the active part of the keccakf module).
	KeccakfSize   int           // size of the keccakf module
	// ColRound is a repeated pattern, used to skip errors in the audit phase of the
	// distributed prover
	ColRound *dedicated.RepeatedPattern
}

// it first applies to-basex to get laneX, then a projection query to map lanex to blocks
func NewKeccakFBlocks(comp *wizard.CompiledIOP, inputs LaneInfo, keccakfSize int) *KeccakFBlocks {

	var (
		laneSize          = inputs.Lane.Size()
		params            = generic.KeccakUsecase
		nbOfLanesPerBlock = params.NbOfLanesPerBlock()
		nbOfRowsPerBlock  = nbOfLanesPerBlock * NbOfRowsPerLane
		allBlocks         = []ifaces.Column{}

		io = &KeccakFBlocks{
			Inputs:      inputs,
			KeccakfSize: keccakfSize,
		}
	)

	io.IsFromFirstBlock = comp.InsertCommit(0, ifaces.ColIDf("IsFromFirstBlock"), laneSize, true)
	io.IsFromBlockBaseB = comp.InsertCommit(0, ifaces.ColIDf("IsFromBlockBaseB"), laneSize, true)
	io.IsBlock = comp.InsertCommit(0, ifaces.ColIDf("IsBlock"), keccakfSize, true)
	io.IsFirstBlock = comp.InsertCommit(0, ifaces.ColIDf("IsFirstBlock"), keccakfSize, true)
	io.IsBlockBaseB = comp.InsertCommit(0, ifaces.ColIDf("IsBlockBaseB"), keccakfSize, true)
	io.IsBlockActive = comp.InsertCommit(0, ifaces.ColIDf("IsBlockActive"), keccakfSize, true)
	io.ColRound = dedicated.NewRepeatedPattern(
		comp,
		0,
		vector.PeriodicOne(keccak.NumRound, keccakfSize),
		verifiercol.NewConstantCol(field.One(), keccakfSize, "ColRound_KeccakFRound"),
		"COL_ROUND",
	)
	colRound := io.ColRound.Natural

	io.Blocks = make([][NumSlices]ifaces.Column, nbOfLanesPerBlock)
	for i := range io.Blocks {
		for j := 0; j < NumSlices; j++ {
			io.Blocks[i][j] = comp.InsertCommit(0,
				ifaces.ColIDf("KeccakFBlock_%d_%d", i, j),
				keccakfSize, true)
		}
	}

	// check isFirstBlock and isBlockBaseBaseB are correctly built
	// isFromFirstBlock is well formed
	// isFromFirstBlock = sum_j Shift(l.isBeginningOfNewHash,-j) for j:=0,...,
	s := sym.NewConstant(0)
	for j := 0; j < nbOfRowsPerBlock; j++ {
		s = sym.Add(
			s, column.Shift(io.Inputs.IsBeginningOfNewHash, -j),
		)
	}
	comp.InsertGlobal(0, ifaces.QueryIDf("IsFromFirstBlock"),
		sym.Sub(s, io.IsFromFirstBlock))

	commonconstraints.MustBeMutuallyExclusiveBinaryFlags(comp,
		io.Inputs.IsLaneActive,
		[]ifaces.Column{io.IsFromFirstBlock, io.IsFromBlockBaseB},
	)

	commonconstraints.MustBeMutuallyExclusiveBinaryFlags(comp,
		io.IsBlock,
		[]ifaces.Column{io.IsFirstBlock, io.IsBlockBaseB},
	)

	commonconstraints.MustZeroWhenInactive(comp, io.IsBlockActive, io.IsBlock)

	comp.InsertGlobal(0, ifaces.QueryIDf("BLOCKS_POSITIONS_CHECK"),
		sym.Mul(io.IsBlockActive,
			sym.Sub(
				sym.Add(io.IsFirstBlock, column.Shift(io.IsBlockBaseB, -1)),
				colRound,
			),
		),
	)

	commonconstraints.MustBeActivationColumns(comp, io.IsBlockActive)

	for i := 0; i < nbOfLanesPerBlock; i++ {
		allBlocks = append(allBlocks, io.Blocks[i][:]...)
	}

	// apply to-base-x to get lanes in bases required for keccakf
	io.Bc = baseconversion.NewToBaseX(comp, baseconversion.ToBaseXInputs{
		Lane:           inputs.Lane,
		IsLaneActive:   inputs.IsLaneActive,
		BaseX:          []int{4, 11},
		NbBitsPerBaseX: 8,
		IsBaseX:        []ifaces.Column{io.IsFromFirstBlock, io.IsFromBlockBaseB},
	})

	// to flatten lane columns over blocks columns, we use projection query
	columnsA := make([][]ifaces.Column, len(io.Bc.LaneX))
	filterA := make([]ifaces.Column, len(io.Bc.LaneX))
	for i, col := range io.Bc.LaneX {
		columnsA[i] = []ifaces.Column{col}
		filterA[i] = io.Inputs.IsLaneActive
	}
	columnsB := make([][]ifaces.Column, len(allBlocks))
	filterB := make([]ifaces.Column, len(allBlocks))
	for i := range allBlocks {
		columnsB[i] = []ifaces.Column{allBlocks[i]}
		filterB[i] = io.IsBlock
	}

	// projection query to get blocks from lanes
	comp.InsertProjection(ifaces.QueryIDf("KeccakFBlocksFromLanes"), query.ProjectionMultiAryInput{
		ColumnsA: columnsA,
		ColumnsB: columnsB,
		FiltersA: filterA,
		FiltersB: filterB,
	})
	return io

}

func (io *KeccakFBlocks) Run(run *wizard.ProverRuntime, traces keccak.PermTraces) {
	var (
		laneSize             = io.Inputs.Lane.Size()
		isBeginningOfNewHash = io.Inputs.IsBeginningOfNewHash.GetColAssignment(run).IntoRegVecSaveAlloc()
		param                = generic.KeccakUsecase
		nbOfRowsPerLane      = param.LaneSizeBytes() / MAXNBYTE
		numRowsPerBlock      = param.NbOfLanesPerBlock() * nbOfRowsPerLane
		isActive             = io.Inputs.IsLaneActive.GetColAssignment(run).IntoRegVecSaveAlloc()
		colIsFromFirstBlock  = common.NewVectorBuilder(io.IsFromFirstBlock)
		colIsFromOtherBlocks = common.NewVectorBuilder(io.IsFromBlockBaseB)
		ones                 = vector.Repeat(field.One(), numRowsPerBlock)
	)

	// assign the columns isFromFirstBlock, isFromBlockBaseB
	for j := 0; j < laneSize; j++ {
		if isBeginningOfNewHash[j].IsOne() {
			colIsFromFirstBlock.PushSliceF(ones)
			j = j + (numRowsPerBlock - 1)
		} else {
			colIsFromFirstBlock.PushInt(0)
		}
	}

	isNotFirstBlock := make([]field.Element, laneSize)
	vector.Sub(isNotFirstBlock, isActive, colIsFromFirstBlock.Slice())
	colIsFromOtherBlocks.PushSliceF(isNotFirstBlock)

	colIsFromFirstBlock.PadAndAssign(run, field.Zero())
	colIsFromOtherBlocks.PadAndAssign(run, field.Zero())

	// run base conversion to get laneX from lane
	io.Bc.Run(run)
	// assign the repeated pattern for colRound
	io.ColRound.Assign(run)
	// assign the blocks and flags
	io.AssignBlockFlags(run, traces)
	io.AssignBlocks(run, traces)

}

// Assigns blocks using the keccak traces.
func (mod *KeccakFBlocks) AssignBlocks(
	run *wizard.ProverRuntime,
	traces keccak.PermTraces,
) {

	var (
		colSize      = mod.KeccakfSize
		numKeccakF   = len(traces.KeccakFInps)
		unpaddedSize = numKeccakF * keccak.NumRound
	)

	run.AssignColumn(
		mod.IsBlockActive.GetColID(),
		smartvectors.RightZeroPadded(
			vector.Repeat(field.One(), unpaddedSize),
			colSize,
		),
	)

	// Assign the block in BaseB.
	blocksVal := [kcommon.NumLanesInBlock][kcommon.NumSlices][]field.Element{}

	for m := range blocksVal {
		for z := 0; z < kcommon.NumSlices; z++ {
			blocksVal[m][z] = make([]field.Element, unpaddedSize)
		}
	}

	parallel.Execute(numKeccakF, func(start, stop int) {

		for nperm := start; nperm < stop; nperm++ {

			currBlock := traces.Blocks[nperm]

			for r := 0; r < keccak.NumRound; r++ {
				// Current row that we are assigning
				currRow := nperm*keccak.NumRound + r

				// Retro-actively assign the block in BaseB if we are not on
				// the first row. The condition over nperm is to ensure that we
				// do not underflow although in practice isNewHash[0] will
				// always be true because this is the first perm of the first
				// hash by definition.
				if r == 0 && nperm > 0 && !traces.IsNewHash[nperm] {
					block := kcommon.CleanBaseBlock(currBlock, &kcommon.BaseChiFr)
					for m := 0; m < kcommon.NumLanesInBlock; m++ {
						for z := 0; z < kcommon.NumSlices; z++ {
							blocksVal[m][z][currRow-1] = block[m][z]
						}
					}
				}
				//assign the firstBlock in BaseA
				if r == 0 && traces.IsNewHash[nperm] {
					block := kcommon.CleanBaseBlock(currBlock, &kcommon.BaseThetaFr)
					for m := 0; m < kcommon.NumLanesInBlock; m++ {
						for z := 0; z < kcommon.NumSlices; z++ {
							blocksVal[m][z][currRow] = block[m][z]
						}
					}
				}
			}
		}
	})

	for m := 0; m < kcommon.NumLanesInBlock; m++ {
		for z := 0; z < kcommon.NumSlices; z++ {
			run.AssignColumn(
				mod.Blocks[m][z].GetColID(),
				smartvectors.RightZeroPadded(blocksVal[m][z], colSize),
			)
		}
	}

}

// Assigns the block flags using the keccak traces.
func (mod *KeccakFBlocks) AssignBlockFlags(
	run *wizard.ProverRuntime,
	permTrace keccak.PermTraces,
) {
	var (
		isFirstBlock = common.NewVectorBuilder(mod.IsFirstBlock)
		isBlockBaseB = common.NewVectorBuilder(mod.IsBlockBaseB)
		isBlock      = common.NewVectorBuilder(mod.IsBlock)
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
