package keccak

import (
	"github.com/consensys/linea-monorepo/prover/crypto/keccak"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/generic"
	keccakf "github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/keccak/keccakf"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/keccak/keccakf/iokeccakf"
)

// KeccakOverBlockInputs stores the inputs required for [NewKeccakOverBlocks]
type KeccakOverBlockInputs struct {
	LaneInfo    iokeccakf.LaneInfo
	KeccakfSize int // size of the keccakf Module (in number of rows)
	// used for the assignments
	Provider [][]byte
}

// KeccakOverBlocks stores the result of the hash and the [wizard.ProverAction] of all the submodules.
type KeccakOverBlocks struct {
	Inputs  KeccakOverBlockInputs
	Blocks  *iokeccakf.KeccakFBlocks
	Outputs *iokeccakf.KeccakFOutputs
	KeccakF *keccakf.Module
}

// NewKeccakOverBlocks implements the utilities for proving keccak hash over the given blocks.
// It assumes that the padding and packing of the stream into blocks is done correctly,and thus directly uses the blocks.
func NewKeccakOverBlocks(comp *wizard.CompiledIOP, inp KeccakOverBlockInputs) *KeccakOverBlocks {

	var (
		// create the blocks for keccakf
		blocks = iokeccakf.NewKeccakFBlocks(comp, inp.LaneInfo, inp.KeccakfSize)

		// run keccakF
		keccakf = keccakf.NewModule(comp, keccakf.KeccakfInputs{
			Blocks:       blocks.Blocks,
			IsBlock:      blocks.IsBlock,
			IsFirstBlock: blocks.IsFirstBlock,
			IsBlockBaseB: blocks.IsBlockBaseB,
			IsActive:     blocks.IsBlockActive,
			KeccakfSize:  blocks.KeccakfSize,
		})

		// get the hash result
		hashResult = iokeccakf.NewOutputKeccakF(comp, keccakf.BackToThetaOrOutput.StateNext, keccakf.BackToThetaOrOutput.IsBase2)
	)

	// set the module
	m := &KeccakOverBlocks{
		Inputs:  inp,
		Blocks:  blocks,
		Outputs: hashResult,
		KeccakF: keccakf,
	}

	return m
}

// It implements [wizard.ProverAction] for customizedKeccak.
func (m *KeccakOverBlocks) Run(run *wizard.ProverRuntime) {
	// first, construct the traces for the accumulated Provider
	permTrace := keccak.GenerateTrace(m.Inputs.Provider)
	m.Blocks.Run(run, permTrace)     // assign the blocks
	m.KeccakF.Assign(run, permTrace) // assign keccakf module
	m.Outputs.Run(run)               // assign the output module
}

// AssignLaneInfo a helper function that assigns the  LaneInfo from the stream.
func AssignLaneInfo(run *wizard.ProverRuntime, l *iokeccakf.LaneInfo, in [][]byte) {
	var (
		lanes                = common.NewVectorBuilder(l.Lane)
		isBeginningOfNewHash = common.NewVectorBuilder(l.IsBeginningOfNewHash)
		isLaneActive         = common.NewVectorBuilder(l.IsLaneActive)
		numRowsPerBlock      = (generic.KeccakUsecase.LaneSizeBytes() / common.LimbBytes) * generic.KeccakUsecase.NbOfLanesPerBlock()
	)

	for i := range in {
		// pad and turn into lanes
		nbBlocks := 1 + len(in[i])/136
		for j := 0; j < nbBlocks; j++ {
			var block [136]byte
			copy(block[:], in[i][j*136:])
			if j == nbBlocks-1 {
				block[len(in[i])-j*136] = 1 // dst
				block[135] |= 0x80          // end marker
			}
			for k := 0; k < numRowsPerBlock; k++ {
				if k == 0 && j == 0 {
					isBeginningOfNewHash.PushInt(1)
				} else {
					isBeginningOfNewHash.PushInt(0)
				}
				isLaneActive.PushInt(1)
				f := *new(field.Element).SetBytes(block[k*common.LimbBytes : k*common.LimbBytes+common.LimbBytes])
				lanes.PushField(f)
			}
		}

	}

	// assign lane-info
	lanes.PadAndAssign(run)
	isBeginningOfNewHash.PadAndAssign(run)
	isLaneActive.PadAndAssign(run)
}
