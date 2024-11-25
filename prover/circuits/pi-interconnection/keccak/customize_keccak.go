package keccak

import (
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"

	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/generic"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/keccak"
)

type module struct {
	keccak keccak.KeccakOverBlocks
}

// NewCustomizedKeccak declares the columns and the constraints for proving hash over EXPECTED blocks.
// The correctness of original blocks is checked outside of the module,
// where one also should assert that the expected blocks are the same as correct original blocks.
func NewCustomizedKeccak(comp *wizard.CompiledIOP, maxNbKeccakF int) *module {
	var (
		size = utils.NextPowerOfTwo(generic.KeccakUsecase.NbOfLanesPerBlock() * maxNbKeccakF)

		inp = keccak.KeccakOverBlockInputs{
			LaneInfo: keccak.LaneInfo{
				Lanes:                comp.InsertProof(0, "Lane", size),
				IsFirstLaneOfNewHash: comp.InsertProof(0, "IsFirstLaneOfNewHash", size),
				IsLaneActive:         comp.InsertProof(0, "IsLaneActive", size),
			},
			MaxNumKeccakF: maxNbKeccakF,
		}
		m = keccak.NewKeccakOverBlocks(comp, inp)
	)

	comp.Columns.SetStatus(m.HashHi.GetColID(), column.Proof)
	comp.Columns.SetStatus(m.HashLo.GetColID(), column.Proof)

	return &module{
		keccak: *m,
	}
}

// AssignCustomizedKeccak assigns the module.
func (m *module) AssignCustomizedKeccak(run *wizard.ProverRuntime, providers [][]byte) {

	//assign Lane-Info
	keccak.AssignLaneInfo(run, &m.keccak.Inputs.LaneInfo, providers)
	// assign keccak
	m.keccak.Inputs.Provider = providers
	m.keccak.Run(run)

}
