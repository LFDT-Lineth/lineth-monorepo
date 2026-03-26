package keccak

import (
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/ringsis"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/cleanup"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/logdata"
	mimcComp "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/mimc"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/selfrecursion"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/vortex"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/hash/generic"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/hash/keccak"
)

type module struct {
	keccak keccak.KeccakOverBlocks
}

// NewCustomizedKeccak declares the columns and the constraints for proving hash over EXPECTED blocks.
// The correctness of original blocks is checked outside the module,
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

func WizardCompilationParameters() []func(iop *wizard.CompiledIOP) {
	var (
		sisInstance = ringsis.Params{LogTwoBound: 16, LogTwoDegree: 6}

		fullCompilationSuite = []func(iop *wizard.CompiledIOP){

			compiler.Arcane(
				compiler.WithTargetColSize(1<<18),
				compiler.WithStitcherMinSize(1<<8),
			),
			logdata.Log("after vortex"),
			vortex.Compile(
				2,
				vortex.ForceNumOpenedColumns(256),
				vortex.WithSISParams(&sisInstance),
			),

			selfrecursion.SelfRecurse,
			cleanup.CleanUp,
			mimcComp.CompileMiMC,
			compiler.Arcane(
				compiler.WithTargetColSize(1<<16),
				compiler.WithStitcherMinSize(1<<8),
			),
			vortex.Compile(
				8,
				vortex.ForceNumOpenedColumns(64),
				vortex.WithSISParams(&sisInstance),
			),

			selfrecursion.SelfRecurse,
			cleanup.CleanUp,
			mimcComp.CompileMiMC,
			compiler.Arcane(
				compiler.WithTargetColSize(1<<13),
				compiler.WithStitcherMinSize(1<<8),
			),
			vortex.Compile(
				8,
				vortex.ForceNumOpenedColumns(64),
				vortex.WithOptionalSISHashingThreshold(1<<20),
			),
		}
	)

	return fullCompilationSuite

}
