package packing

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/generic"
	"github.com/stretchr/testify/assert"
)

// It generates Define and Assign function of Packing module, for testing
func makeTestCaseLaneRepacking(uc generic.HashingUsecase) (
	define wizard.DefineFunc,
	prover wizard.MainProverStep,
) {
	var (
		// max number of blocks that can be extracted from limbs
		// if the number of blocks passes the max, newPack() would panic.
		maxNumBlock = 5
		// if the blockSize is not consistent with PackingParam, newPack() would panic.
		blockSize = uc.BlockSizeBytes()
		// for testing; used to populate the importation columns
		// since we have at least one block per hash, the umber of hashes should be less than maxNumBlocks
		// max number of limbs
		numHash = 2
		size    = utils.NextPowerOfTwo(maxNumBlock * blockSize)
	)

	imported := Importation{}
	decomposed := decomposition{}
	spaghetti := spaghettiCtx{}
	l := laneRepacking{}

	define = func(build *wizard.Builder) {
		comp := build.CompiledIOP

		imported = createImportationColumns(comp, size)

		pckInp := PackingInput{
			MaxNumBlocks: maxNumBlock,
			PackingParam: uc,
			Imported:     imported,
		}

		inp := &decompositionInputs{
			Param:    pckInp.PackingParam,
			Imported: imported,
			Name:     "TEST_SPAGHETTI",
		}

		decomposed = decomposition{
			Inputs: inp,
			Size:   size,
		}

		// commit to decomposition Columns; no constraint
		decomposed.insertCommit(comp)

		spaghetti = spaghettiMaker(comp, decomposed, imported.IsNewHash)

		// constraints
		l = newLane(comp, spaghetti, pckInp)

	}
	prover = func(run *wizard.ProverRuntime) {

		// assign importation columns
		assignImportationColumns(run, &imported, numHash, blockSize, size)
		decomposed.assignMainColumns(run)
		// assign filter
		assignFilter(run, decomposed)
		l.Assign(run)
	}
	return define, prover
}

func TestLaneRepacking(t *testing.T) {
	for _, uc := range testCases {
		t.Run(uc.Name, func(t *testing.T) {
			define, prover := makeTestCaseLaneRepacking(uc.UseCase)
			comp := wizard.Compile(define, dummy.Compile)
			proof := wizard.Prove(comp, prover)
			assert.NoErrorf(t, wizard.Verify(comp, proof), "invalid proof")
		})
	}
}

func assignFilter(run *wizard.ProverRuntime, decomposed decomposition) {
	var (
		size   = decomposed.Size
		filter = make([]*common.VectorBuilder, nbDecomposedLen)
	)
	for j := range decomposed.DecomposedLen {
		filter[j] = common.NewVectorBuilder(decomposed.Filter[j])
		decomposedLen := decomposed.DecomposedLen[j].GetColAssignment(run).IntoRegVecSaveAlloc()
		for row := 0; row < size; row++ {
			if decomposedLen[row].IsZero() {
				filter[j].PushInt(0)
			} else {
				filter[j].PushInt(1)
			}
		}
		filter[j].PadAndAssign(run, field.Zero())
	}
}
