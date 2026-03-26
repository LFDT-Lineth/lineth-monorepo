package gen_acc

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed/pragmas"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/limbs"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/generic"
	"github.com/stretchr/testify/assert"
)

// It generates Define and Assign function of Data module, for testing
func makeTestCaseInfoModule(c []makeInfoTestCase) (
	define wizard.DefineFunc,
	prover wizard.MainProverStep,
) {

	var (
		maxNumKeccakF = 16
		d             = &GenericInfoAccumulator{}
		gdms          = make([]generic.GenInfoModule, len(c))
	)

	define = func(build *wizard.Builder) {
		comp := build.CompiledIOP
		for i := range gdms {
			createCol := common.CreateColFn(comp, "TESTING_INFO_ACCUMULATOR", c[i].Size, pragmas.RightPadded)
			gdms[i] = generic.GenInfoModule{
				HashHi:   limbs.NewUint128Be(comp, ifaces.ColIDf("Hash_Hi_%v", i), c[i].Size),
				HashLo:   limbs.NewUint128Be(comp, ifaces.ColIDf("Hash_Lo_%v", i), c[i].Size),
				IsHashLo: createCol("Is_Hash_Lo_%v", i),
				IsHashHi: createCol("Is_Hash_Hi_%v", i),
			}
		}

		inp := GenericAccumulatorInputs{
			ProvidersInfo: gdms,
			MaxNumKeccakF: maxNumKeccakF,
		}
		d = NewGenericInfoAccumulator(comp, inp)
	}
	prover = func(run *wizard.ProverRuntime) {
		for i := range gdms {
			generateAndAssignGenInfoModule(run, &gdms[i], c[i])
		}
		d.Run(run)
	}
	return define, prover
}

func TestInfoModule(t *testing.T) {
	define, prover := makeTestCaseInfoModule(infoTestCases)
	comp := wizard.Compile(define, dummy.Compile)

	proof := wizard.Prove(comp, prover)
	assert.NoErrorf(t, wizard.Verify(comp, proof), "invalid proof")
}

type makeInfoTestCase struct {
	Name     string
	Size     int
	HashHi   []int
	HashLo   []int
	IsHashHi []int
	IsHashLo []int
}

var infoTestCases = []makeInfoTestCase{
	{
		Name:     "GenDataModule1",
		Size:     8,
		HashHi:   []int{17, 19, 1, 3, 2},
		HashLo:   []int{14, 1, 1, 0, 7},
		IsHashHi: []int{1, 0, 1, 1, 0},
		IsHashLo: []int{1, 0, 1, 1, 0},
	},
	{
		Name:     "GenDataModule2",
		Size:     16,
		HashHi:   []int{1, 89, 1, 1, 6, 1, 2, 3, 90, 3},
		HashLo:   []int{17, 34, 1, 1, 9, 21, 2, 3, 44, 11},
		IsHashHi: []int{1, 0, 1, 0, 1, 1, 1, 1, 0, 0},
		IsHashLo: []int{0, 1, 0, 1, 0, 1, 1, 1, 1, 0}, // shift
	},
	{
		Name:     "GenDataModule3",
		Size:     16,
		HashHi:   []int{1, 89, 1, 1, 6, 1, 2, 3, 90, 3, 4, 0},
		HashLo:   []int{1, 89, 1, 1, 6, 1, 2, 3, 90, 3, 4, 0}, // same
		IsHashHi: []int{1, 0, 1, 0, 1, 1, 1, 1, 0, 0, 1, 0},
		IsHashLo: []int{0, 1, 0, 1, 0, 1, 1, 1, 1, 0, 0, 1}, // shift
	},
}

func generateAndAssignGenInfoModule(run *wizard.ProverRuntime, gbm *generic.GenInfoModule, c makeInfoTestCase) {
	hashHi := limbs.NewVectorBuilder(gbm.HashHi.AsDynSize())
	hashLo := limbs.NewVectorBuilder(gbm.HashLo.AsDynSize())
	isHashHi := common.NewVectorBuilder(gbm.IsHashHi)
	isHashLo := common.NewVectorBuilder(gbm.IsHashLo)
	for i := range c.HashHi {
		hashHi.PushInt(c.HashHi[i])
		hashLo.PushInt(c.HashLo[i])
		isHashHi.PushInt(c.IsHashHi[i])
		isHashLo.PushInt(c.IsHashLo[i])
	}
	hashHi.PadAndAssignZero(run)
	hashLo.PadAndAssignZero(run)
	isHashHi.PadAndAssign(run)
	isHashLo.PadAndAssign(run)

}
