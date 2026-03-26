package base_conversion

import (
	"crypto/rand"
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/distributed/pragmas"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/common"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/hash/generic"
	"github.com/stretchr/testify/assert"
)

func makeTestCaseBaseConversionInput() (
	define wizard.DefineFunc,
	prover wizard.MainProverStep,
) {
	numBlocks := 15
	b := &BlockBaseConversion{}
	define = func(build *wizard.Builder) {
		var (
			comp      = build.CompiledIOP
			keccak    = generic.KeccakUsecase
			size      = utils.NextPowerOfTwo(keccak.NbOfLanesPerBlock() * numBlocks)
			createCol = common.CreateColFn(comp, "BASE_CONVERSION_TEST", size, pragmas.RightPadded)
		)

		inp := BlockBaseConversionInputs{
			Lane:                 createCol("LANE"),
			IsFirstLaneOfNewHash: createCol("IS_FIRST_LANE_NEW_HASH"),
			IsLaneActive:         createCol("IS_ACTIVE"),
			Lookup:               NewLookupTables(comp),
		}
		b = NewBlockBaseConversion(comp, inp)

	}
	prover = func(run *wizard.ProverRuntime) {

		b.assignInputs(run, numBlocks)
		b.Run(run)

	}
	return define, prover
}
func TestBaseConversionInput(t *testing.T) {
	define, prover := makeTestCaseBaseConversionInput()
	comp := wizard.Compile(define, dummy.Compile)
	proof := wizard.Prove(comp, prover)
	assert.NoErrorf(t, wizard.Verify(comp, proof), "invalid proof")
}

func (b *BlockBaseConversion) assignInputs(run *wizard.ProverRuntime, numBlocks int) {
	var (
		lane              = common.NewVectorBuilder(b.Inputs.Lane)
		isFirst           = common.NewVectorBuilder(b.Inputs.IsFirstLaneOfNewHash)
		isActive          = common.NewVectorBuilder(b.Inputs.IsLaneActive)
		nbOfLanesPerBlock = generic.KeccakUsecase.NbOfLanesPerBlock()
		effectiveSize     = nbOfLanesPerBlock * numBlocks
	)

	for row := 0; row < effectiveSize; row++ {
		// input lanes are uint64 big-endian
		// choose 8 random bytes
		b := make([]byte, 8)
		rand.Read(b)
		f := *new(field.Element).SetBytes(b)
		lane.PushField(f)
		isActive.PushInt(1)
		if row%nbOfLanesPerBlock == 0 && (row/nbOfLanesPerBlock == 0) {
			isFirst.PushInt(1)
		} else {
			isFirst.PushInt(0)
		}
	}

	lane.PadAndAssign(run, field.Zero())
	isFirst.PadAndAssign(run, field.Zero())
	isActive.PadAndAssign(run, field.Zero())
}
