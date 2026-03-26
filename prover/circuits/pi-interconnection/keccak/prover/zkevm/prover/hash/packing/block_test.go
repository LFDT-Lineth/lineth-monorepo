package packing

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/common"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/hash/generic"
	"github.com/stretchr/testify/assert"
)

// It generates Define and Assign function of Packing module, for testing
func makeTestCaseBlockModule(uc generic.HashingUsecase) (
	define wizard.DefineFunc,
	prover wizard.MainProverStep,
) {
	var (
		// max number of blocks that can be extracted from limbs
		// if the number of blocks passes the max, newPack() would panic.
		maxNumBlock = 103
		// if the blockSize is not consistent with PackingParam, newPack() would panic.
		nbOfLanesPerBlock = uc.BlockSizeBytes()
		size              = utils.NextPowerOfTwo(maxNumBlock * nbOfLanesPerBlock)
		effectiveSize     = maxNumBlock * nbOfLanesPerBlock
	)

	block := block{}
	var isActive, isFirstLaneOfHash ifaces.Column

	define = func(build *wizard.Builder) {
		comp := build.CompiledIOP

		// commit to isActive
		isActive = comp.InsertCommit(0, "IsActive", size)
		isFirstLaneOfHash = comp.InsertCommit(0, "IsFirstLaneOfHash", size)

		inp := blockInput{
			Lanes: laneRepacking{
				IsLaneActive:         isActive,
				Size:                 size,
				IsFirstLaneOfNewHash: isFirstLaneOfHash,
				Inputs: &laneRepackingInputs{
					PckInp: PackingInput{Name: "TEST"},
				},
			},
			Param: uc,
		}

		// constraints
		block = newBlock(comp, inp)

	}
	prover = func(run *wizard.ProverRuntime) {

		// assign isActive
		col := vector.Repeat(field.One(), effectiveSize)
		run.AssignColumn(isActive.GetColID(), smartvectors.RightZeroPadded(col, size))
		// assign isFirstLaneOfHash
		isFirst := common.NewVectorBuilder(isFirstLaneOfHash)
		for i := 0; i < effectiveSize; i++ {
			if i%nbOfLanesPerBlock == 0 && i/nbOfLanesPerBlock == 2 {
				isFirst.PushInt(1)
			} else {
				isFirst.PushInt(0)
			}

		}
		isFirst.PadAndAssign(run)

		block.Assign(run)
	}
	return define, prover
}

func TestBlockModule(t *testing.T) {
	for _, uc := range testCases {
		t.Run(uc.Name, func(t *testing.T) {
			define, prover := makeTestCaseBlockModule(uc.UseCase)
			comp := wizard.Compile(define, dummy.Compile)
			proof := wizard.Prove(comp, prover)
			assert.NoErrorf(t, wizard.Verify(comp, proof), "invalid proof")
		},
		)
	}
}
