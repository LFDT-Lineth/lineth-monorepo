package keccak

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/generic"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/generic/testdata"
	"github.com/stretchr/testify/assert"
)

func MakeTestCaseKeccak(t *testing.T, c makeTestCaseGBM) (
	define wizard.DefineFunc,
	prover wizard.MainProverStep,
) {
	mod := &KeccakSingleProvider{}
	maxNumKeccakF := 12
	gdm := generic.GenDataModule{}
	gim := generic.GenInfoModule{}

	define = func(builder *wizard.Builder) {
		comp := builder.CompiledIOP
		gdm = testdata.CreateGenDataModule(comp, c.Name, c.SizeData, 8)
		gim = testdata.CreateGenInfoModule(comp, c.Name, c.SizeInfo, 8)

		inp := KeccakSingleProviderInput{
			MaxNumKeccakF: maxNumKeccakF,
			Provider: generic.GenericByteModule{
				Data: gdm,
				Info: gim},
		}
		mod = NewKeccakSingleProvider(comp, inp)
	}

	prover = func(run *wizard.ProverRuntime) {

		testdata.GenerateAndAssignGenDataModule(run, &gdm, c.HashNum, c.ToHash, true)
		// expected hash is embedded inside gim columns.
		testdata.GenerateAndAssignGenInfoModule(run, &gim, gdm, c.IsHashHi, c.IsHashLo)

		mod.Run(run)
	}

	return define, prover
}

func TestKeccak(t *testing.T) {
	definer, prover := MakeTestCaseKeccak(t, testCasesGBMSingleProvider)
	comp := wizard.Compile(definer, dummy.Compile)

	proof := wizard.Prove(comp, prover)
	assert.NoErrorf(t, wizard.Verify(comp, proof), "invalid proof")

}

var testCasesGBMSingleProvider = makeTestCaseGBM{

	Name:     "GenDataModule3",
	SizeData: 32,
	HashNum:  []int{1, 1, 1, 1, 1, 1, 2, 3, 3, 3, 4, 4, 4, 4, 4},
	ToHash:   []int{1, 0, 1, 0, 1, 1, 1, 1, 0, 0, 1, 1, 0, 1, 1},
	SizeInfo: 8,
	IsHashHi: []int{1, 0, 0, 1, 1, 1, 0},
	IsHashLo: []int{0, 1, 0, 0, 1, 1, 1}, // shift

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
