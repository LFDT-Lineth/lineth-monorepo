package keccak

import (
	"crypto/rand"
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/keccak"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/distributed/pragmas"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/common"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/hash/generic"
	"github.com/stretchr/testify/assert"
)

// makes Define and Prove function for testing [NewKeccakOverBlocks]
func MakeTestCaseCustomizedKeccak(t *testing.T, providers [][]byte) (
	define wizard.DefineFunc,
	prover wizard.MainProverStep,
) {
	mod := &KeccakOverBlocks{}
	maxNumKeccakF := 16
	size := utils.NextPowerOfTwo(maxNumKeccakF * generic.KeccakUsecase.NbOfLanesPerBlock())

	define = func(builder *wizard.Builder) {
		comp := builder.CompiledIOP
		createCol := common.CreateColFn(comp, "Test_Customized_Keccak", size, pragmas.RightPadded)

		inp := KeccakOverBlockInputs{
			LaneInfo: LaneInfo{
				Lanes:                createCol("Lanes"),
				IsFirstLaneOfNewHash: createCol("IsFirstLaneOfNewHash"),
				IsLaneActive:         createCol("IsLaneActive"),
			},

			MaxNumKeccakF: maxNumKeccakF,
			Provider:      providers,
		}
		mod = NewKeccakOverBlocks(comp, inp)
	}

	prover = func(run *wizard.ProverRuntime) {

		AssignLaneInfo(run, &mod.Inputs.LaneInfo, mod.Inputs.Provider)
		mod.Run(run)

		// check the hash result
		permTrace := keccak.GenerateTrace(mod.Inputs.Provider)
		hi := mod.HashHi.GetColAssignment(run).IntoRegVecSaveAlloc()
		lo := mod.HashLo.GetColAssignment(run).IntoRegVecSaveAlloc()
		for i, expectedHash := range permTrace.HashOutPut {
			// hashHi := hash[:16] ,  hashLo := hash[16:]
			gotHashHi := hi[i].Bytes()
			gotHashLo := lo[i].Bytes()
			assert.Equal(t, expectedHash[:16], gotHashHi[16:])
			assert.Equal(t, expectedHash[16:], gotHashLo[16:])
		}

		for i := len(permTrace.HashOutPut); i < len(hi); i++ {
			assert.Equal(t, field.Zero(), hi[i])
			assert.Equal(t, field.Zero(), lo[i])
		}
	}

	return define, prover
}

func TestCustomizedKeccak(t *testing.T) {
	var providers [][]byte
	// generate 20 random slices of bytes
	for i := 0; i < 1; i++ {
		length := (i + 1) * generic.KeccakUsecase.BlockSizeBytes()
		// generate random bytes
		slice := make([]byte, length)
		rand.Read(slice)
		providers = append(providers, slice)
	}

	definer, prover := MakeTestCaseCustomizedKeccak(t, providers)
	comp := wizard.Compile(definer, dummy.Compile)

	proof := wizard.Prove(comp, prover)
	assert.NoErrorf(t, wizard.Verify(comp, proof), "invalid proof")
}
