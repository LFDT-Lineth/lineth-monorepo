package gen_acc

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/hash/generic"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/hash/generic/testdata"
	"github.com/stretchr/testify/assert"
)

// It generates Define and Assign function of Data module, for testing
func makeTestCaseDataModule(c []makeTestCase) (
	define wizard.DefineFunc,
	prover wizard.MainProverStep,
) {
	maxNumKeccakF := 8

	d := &GenericDataAccumulator{}
	gdms := make([]generic.GenDataModule, len(c))

	define = func(build *wizard.Builder) {
		comp := build.CompiledIOP
		for i := range gdms {
			gdms[i] = testdata.CreateGenDataModule(comp, c[i].Name, c[i].Size)
		}

		inp := GenericAccumulatorInputs{
			ProvidersData: gdms,
			MaxNumKeccakF: maxNumKeccakF,
		}
		d = NewGenericDataAccumulator(comp, inp)
	}
	prover = func(run *wizard.ProverRuntime) {
		for i := range gdms {
			testdata.GenerateAndAssignGenDataModule(run, &gdms[i], c[i].HashNum, c[i].ToHash, true)
		}
		d.Run(run)
	}
	return define, prover
}

func TestDataModule(t *testing.T) {
	define, prover := makeTestCaseDataModule(testCases)
	comp := wizard.Compile(define, dummy.Compile)

	proof := wizard.Prove(comp, prover)
	assert.NoErrorf(t, wizard.Verify(comp, proof), "invalid proof")
}

type makeTestCase struct {
	Name    string
	Size    int
	HashNum []int
	ToHash  []int
}

var testCases = []makeTestCase{
	{
		Name:    "GenDataModule1",
		Size:    8,
		HashNum: []int{1, 1, 1, 1, 2},
		ToHash:  []int{1, 0, 1, 0, 1},
	},
	{
		Name:    "GenDataModule2",
		Size:    16,
		HashNum: []int{1, 1, 1, 1, 1, 1, 2, 3, 3, 3},
		ToHash:  []int{1, 0, 1, 0, 1, 1, 1, 1, 0, 0},
	},
	{
		Name:    "GenDataModule3",
		Size:    32,
		HashNum: []int{1, 1, 1, 1, 1, 1, 2, 3, 3, 3, 4, 4, 4, 4, 4},
		ToHash:  []int{1, 0, 1, 0, 1, 1, 1, 1, 0, 0, 1, 1, 0, 1, 1},
	},
}
