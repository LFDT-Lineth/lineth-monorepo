package iokeccakf

import (
	"crypto/rand"
	"testing"

	"github.com/consensys/linea-monorepo/prover/crypto/keccak"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/generic"
	"github.com/stretchr/testify/assert"
)

func TestKeccakFBlocks(t *testing.T) {
	var (
		io                = &KeccakFBlocks{}
		numBlocks         = 4
		numRowsPerBlock   = generic.KeccakUsecase.NbOfLanesPerBlock() * NbOfRowsPerLane
		laneEffectiveSize = numRowsPerBlock * numBlocks
		laneSize          = utils.NextPowerOfTwo(laneEffectiveSize)
		keccakfSize       = utils.NextPowerOfTwo(numBlocks * keccak.NumRound)
	)

	define := func(build *wizard.Builder) {

		//declare input columns here
		lane := build.CompiledIOP.InsertCommit(0, "LANE", laneSize, true)
		isBeginningOfNewHash := build.CompiledIOP.InsertCommit(0, "IS_BEGINNING_OF_NEW_HASH", laneSize, true)
		isLaneActive := build.CompiledIOP.InsertCommit(0, "IS_LANE_ACTIVE", laneSize, true)

		io = NewKeccakFBlocks(build.CompiledIOP, LaneInfo{
			Lane:                 lane,
			IsBeginningOfNewHash: isBeginningOfNewHash,
			IsLaneActive:         isLaneActive,
		},
			keccakfSize)
	}

	prover := func(run *wizard.ProverRuntime) {
		// note that we are assigning the columns with what are expected values, so that verifying the constraints is enough
		// namely, we dont need to examine the assigned values from the module against the expected values.
		var providers [][]byte
		// generate 20 random slices of bytes
		for i := 0; i < 1; i++ {
			length := (i + 1) * generic.KeccakUsecase.BlockSizeBytes()
			// generate random bytes
			slice := make([]byte, length-8)
			rand.Read(slice)
			providers = append(providers, slice)
		}

		AssignLaneInfo(run, &io.Inputs, providers)
		traces := keccak.GenerateTrace(providers)

		// assign io module
		io.Run(run, traces)

	}

	compiled := wizard.Compile(define, dummy.Compile)
	proof := wizard.Prove(compiled, prover)
	assert.NoErrorf(t, wizard.Verify(compiled, proof), "ioKeccakF-verifier failed")

}

// / AssignLaneInfo a helper function that assigns the  LaneInfo from the stream.
func AssignLaneInfo(run *wizard.ProverRuntime, l *LaneInfo, in [][]byte) {
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
