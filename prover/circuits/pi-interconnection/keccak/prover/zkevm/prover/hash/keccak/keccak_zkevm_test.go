package keccak

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/hash/generic"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/hash/generic/testdata"
	"github.com/stretchr/testify/assert"
)

func MakeTestCaseKeccakZkEVM(t *testing.T, c []makeTestCaseGBM) (
	define wizard.DefineFunc,
	prover wizard.MainProverStep,
) {
	mod := &KeccakZkEVM{}
	maxNumKeccakF := 12
	gdms := make([]generic.GenDataModule, len(c))
	gims := make([]generic.GenInfoModule, len(c))
	gbm := make([]generic.GenericByteModule, len(c))

	define = func(builder *wizard.Builder) {
		comp := builder.CompiledIOP
		for i := range gdms {
			gdms[i] = testdata.CreateGenDataModule(comp, c[i].Name, c[i].SizeData)
			gims[i] = testdata.CreateGenInfoModule(comp, c[i].Name, c[i].SizeInfo)
			gbm[i] = generic.GenericByteModule{
				Data: gdms[i],
				Info: gims[i],
			}
		}

		mod = newKeccakZkEvm(
			comp,
			Settings{MaxNumKeccakf: maxNumKeccakF},
			gbm,
		)
	}

	prover = func(run *wizard.ProverRuntime) {
		for i := range gdms {
			testdata.GenerateAndAssignGenDataModule(run, &gdms[i], c[i].HashNum, c[i].ToHash, true)
			// expected hash is embedded inside gim columns.
			testdata.GenerateAndAssignGenInfoModule(run, &gims[i], gdms[i], c[i].IsHashHi, c[i].IsHashLo)
		}
		mod.Run(run)

	}

	return define, prover
}

func TestKeccakZkEVM(t *testing.T) {
	definer, prover := MakeTestCaseKeccakZkEVM(t, testCasesGBMMultiProvider)
	comp := wizard.Compile(definer, dummy.Compile)

	proof := wizard.Prove(comp, prover)
	assert.NoErrorf(t, wizard.Verify(comp, proof), "invalid proof")
}

type makeTestCaseGBM struct {
	Name     string
	SizeData int
	HashNum  []int
	ToHash   []int
	SizeInfo int
	IsHashHi []int
	IsHashLo []int
}

var testCasesGBMMultiProvider = []makeTestCaseGBM{
	{
		Name:     "GenDataModule1",
		SizeData: 8,
		HashNum:  []int{1, 1, 1, 1, 2},
		ToHash:   []int{1, 0, 1, 0, 1},
		SizeInfo: 4,
		IsHashHi: []int{0, 1, 0, 1}, // # ones = number of hash from above
		IsHashLo: []int{1, 0, 0, 1},
	},
	{
		Name:     "GenDataModule2",
		SizeData: 16,
		HashNum:  []int{1, 1, 1, 1, 1, 1, 2, 3, 3, 3},
		ToHash:   []int{1, 0, 1, 0, 1, 1, 1, 1, 0, 0},
		SizeInfo: 8,
		IsHashHi: []int{0, 1, 1, 0, 0, 1},
		IsHashLo: []int{0, 1, 1, 0, 0, 1}, // same
	},
	{
		Name:     "GenDataModule3",
		SizeData: 32,
		HashNum:  []int{1, 1, 1, 1, 1, 1, 2, 3, 3, 3, 4, 4, 4, 4, 4},
		ToHash:   []int{1, 0, 1, 0, 1, 1, 1, 1, 0, 0, 1, 1, 0, 1, 1},
		SizeInfo: 8,
		IsHashHi: []int{1, 0, 0, 1, 1, 1, 0},
		IsHashLo: []int{0, 1, 0, 0, 1, 1, 1}, // shift
	},
}
