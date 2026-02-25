package keccak

import (
	"github.com/consensys/linea-monorepo/prover/crypto/keccak"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/keccak/base_conversion"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/keccak/keccakf"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/packing/dedicated/spaghettifier"
)

type LaneInfo struct {
	Lanes                ifaces.Column
	IsFirstLaneOfNewHash ifaces.Column
	IsLaneActive         ifaces.Column
}

// KeccakOverBlockInputs stores the inputs required for [NewKeccakOverBlocks]
type KeccakOverBlockInputs struct {
	LaneInfo      LaneInfo
	MaxNumKeccakF int

	// used for the assignments
	Provider [][]byte
}

// KeccakOverBlocks stores the result of the hash and the [wizard.ProverAction] of all the submodules.
type KeccakOverBlocks struct {
	Inputs         *KeccakOverBlockInputs
	HashHi, HashLo ifaces.Column
	// it indicates the active part of HashHi/HashLo
	IsActive      ifaces.Column
	MaxNumKeccakF int

	// prover actions for  internal modules
	Pa_blockBaseConversion *base_conversion.BlockBaseConversion
	Pa_hashBaseConversion  *base_conversion.HashBaseConversion
	Pa_spaghetti           *spaghettifier.Spaghettification
	KeccakF                keccakf.Module
}

// NewKeccakOverBlocks implements the utilities for proving keccak hash over the given blocks.
// It assumes that the padding and packing of the stream into blocks is done correctly,and thus directly uses the blocks.
func NewKeccakOverBlocks(comp *wizard.CompiledIOP, inp KeccakOverBlockInputs) *KeccakOverBlocks {
	var (
		maxNumKeccakF        = inp.MaxNumKeccakF
		lookupBaseConversion = base_conversion.NewLookupTables(comp)

		// apply base conversion over the blocks
		inpBcBlock = base_conversion.BlockBaseConversionInputs{
			Lane:                 inp.LaneInfo.Lanes,
			IsFirstLaneOfNewHash: inp.LaneInfo.IsFirstLaneOfNewHash,
			IsLaneActive:         inp.LaneInfo.IsLaneActive,
			Lookup:               lookupBaseConversion,
		}

		bcForBlock = base_conversion.NewBlockBaseConversion(comp, inpBcBlock)

		// run keccakF
		keccakf = keccakf.NewModule(comp, 0, maxNumKeccakF)

		// bring the hash result to the natural base (uint)
		inpBcHash = base_conversion.HashBaseConversionInput{
			LimbsHiB: append(
				keccakf.IO.HashOutputSlicesBaseB[0][:],
				keccakf.IO.HashOutputSlicesBaseB[1][:]...,
			),

			LimbsLoB: append(
				keccakf.IO.HashOutputSlicesBaseB[2][:],
				keccakf.IO.HashOutputSlicesBaseB[3][:]...,
			),
			IsActive:      keccakf.IO.IsActive,
			MaxNumKeccakF: maxNumKeccakF,
			Lookup:        lookupBaseConversion,
		}

		bcForHash = base_conversion.NewHashBaseConversion(comp, inpBcHash)
	)

	// keccakF does not directly take the blocks, but rather build them via a trace
	// thus, we need to check that the blocks in keccakf matches the one from base conversion.
	// blocks in keccakf are the spaghetti form of LaneX.
	inpSpaghetti := spaghettifier.SpaghettificationInput{
		Name:          "KECCAK_OVER_BLOCKS",
		ContentMatrix: [][]ifaces.Column{keccakf.Blocks[:]},
		Filter:        isBlock(keccakf.IO.IsBlock),
		SpaghettiSize: bcForBlock.LaneX.Size(),
	}

	blockSpaghetti := spaghettifier.Spaghettify(comp, inpSpaghetti)
	comp.InsertGlobal(0, "BLOCK_Is_LANEX",
		symbolic.Sub(blockSpaghetti.ContentSpaghetti[0], bcForBlock.LaneX),
	)

	// set the module
	m := &KeccakOverBlocks{
		Inputs:                 &inp,
		MaxNumKeccakF:          maxNumKeccakF,
		HashHi:                 bcForHash.HashHi,
		HashLo:                 bcForHash.HashLo,
		IsActive:               bcForHash.IsActive,
		Pa_blockBaseConversion: bcForBlock,
		KeccakF:                keccakf,
		Pa_hashBaseConversion:  bcForHash,
		Pa_spaghetti:           blockSpaghetti,
	}

	return m
}

// It implements [wizard.ProverAction] for customizedKeccak.
func (m *KeccakOverBlocks) Run(run *wizard.ProverRuntime) {
	// assign blockBaseConversion
	m.Pa_blockBaseConversion.Run(run)
	// assign keccakF
	// first, construct the traces for the accumulated Provider
	permTrace := keccak.GenerateTrace(m.Inputs.Provider)
	m.KeccakF.Assign(run, permTrace)
	// assign HashBaseConversion
	m.Pa_hashBaseConversion.Run(run)
	//assign blockSpaghetti
	m.Pa_spaghetti.Run(run)
}

// AssignLaneInfo a helper function that assigns the blocks (i.e., LaneInfo)) from the stream.
// Blocks then can be used by the customizedKeccak.
func AssignLaneInfo(run *wizard.ProverRuntime, l *LaneInfo, in [][]byte) {
	var (
		lanes                = common.NewVectorBuilder(l.Lanes)
		isFirstLaneOfNewHash = common.NewVectorBuilder(l.IsFirstLaneOfNewHash)
		isLaneActive         = common.NewVectorBuilder(l.IsLaneActive)
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
			for k := 0; k < 17; k++ {
				if k == 0 && j == 0 {
					isFirstLaneOfNewHash.PushInt(1)
				} else {
					isFirstLaneOfNewHash.PushInt(0)
				}
				isLaneActive.PushInt(1)
				f := *new(field.Element).SetBytes(block[k*8 : k*8+8])
				lanes.PushField(f)
			}
		}

	}

	// assign lane-info
	lanes.PadAndAssign(run)
	isFirstLaneOfNewHash.PadAndAssign(run)
	isLaneActive.PadAndAssign(run)
}
