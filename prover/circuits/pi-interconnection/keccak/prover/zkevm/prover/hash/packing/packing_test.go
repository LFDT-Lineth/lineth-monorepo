package packing

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/hash/generic"
	"github.com/stretchr/testify/assert"
)

// It generates Define and Assign function of Packing module, for testing
func makeTestCasePackingModule(uc generic.HashingUsecase) (
	define wizard.DefineFunc,
	prover wizard.MainProverStep,
) {
	var (
		// max number of blocks that can be extracted from limbs
		// if the number of blocks passes the max, newPack() would panic.
		maxNumBlock = 36
		// if the blockSize is not consistent with PackingParam, newPack() would panic.
		blockSize = uc.BlockSizeBytes()
		// for testing; used to populate the importation columns
		// since we have at least one block per hash, the umber of hashes should be less than maxNumBlocks
		numHash = 15
		// max number of limbs
		size = utils.NextPowerOfTwo(maxNumBlock * blockSize)
	)

	imported := Importation{}
	pck := &Packing{}

	define = func(build *wizard.Builder) {
		comp := build.CompiledIOP
		imported = createImportationColumns(comp, size)
		inp := PackingInput{
			MaxNumBlocks: maxNumBlock,
			PackingParam: uc,
			Imported:     imported,
			Name:         "TESTING",
		}

		pck = NewPack(comp, inp)
	}
	prover = func(run *wizard.ProverRuntime) {
		var (
			imported  = pck.Inputs.Imported
			blockSize = pck.Inputs.PackingParam.BlockSizeBytes()
		)
		// assign the importation columns
		assignImportationColumns(run, &imported, numHash, blockSize, size)

		// assign all the Packing module.
		pck.Run(run)
	}
	return define, prover
}

func TestPackingModule(t *testing.T) {
	for _, uc := range testCases {
		t.Run(uc.Name, func(t *testing.T) {
			define, prover := makeTestCasePackingModule(uc.UseCase)
			comp := wizard.Compile(define, dummy.Compile)
			proof := wizard.Prove(comp, prover)
			assert.NoErrorf(t, wizard.Verify(comp, proof), "invalid proof")
		})
	}
}

var testCases = []struct {
	Name    string
	UseCase generic.HashingUsecase
}{
	{
		Name:    "Keccak",
		UseCase: generic.KeccakUsecase,
	},
	{
		Name:    "Sha2",
		UseCase: generic.Sha2Usecase,
	},
	{
		Name:    "MiMC",
		UseCase: generic.MiMCUsecase,
	},
}
