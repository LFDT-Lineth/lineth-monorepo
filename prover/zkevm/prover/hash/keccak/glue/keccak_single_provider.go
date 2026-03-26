// The keccak package implements the utilities for proving the hash over a single provider.
// The provider of type [generic.GenericByteModule] encodes the inputs and outputs of hash (related to the same module).
// The inputs and outputs are respectively embedded inside [generic.GenDataModule], and [generic.GenInfoModule].
package keccak

import (
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/generic"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/importpad"
	keccakf "github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/keccak/keccakf"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/keccak/keccakf/iokeccakf"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/packing"
)

// KeccakSingleProviderInput stores the inputs for [NewKeccakSingleProvider]
type KeccakSingleProviderInput struct {
	MaxNumKeccakF int
	Provider      generic.GenericByteModule
}

// KeccakSingleProvider stores the hash result and [wizard.ProverAction] of the submodules.
type KeccakSingleProvider struct {
	Inputs        *KeccakSingleProviderInput
	MaxNumKeccakF int

	// prover actions for  internal modules
	ImportPad        *importpad.Importation
	Packing          *packing.Packing
	KeccakOverBlocks *KeccakOverBlocks
}

// NewKeccakSingleProvider implements the utilities for proving keccak hash
// over the streams which are encoded inside a set of structs [generic.GenDataModule].
// It calls;
// -  Padding module to insure the correct padding of the streams.
// -  packing module to insure the correct packing of padded-stream into blocks.
// -  keccakOverBlocks to insures the correct hash computation over the given blocks.
func NewKeccakSingleProvider(comp *wizard.CompiledIOP, inp KeccakSingleProviderInput) *KeccakSingleProvider {
	var (
		maxNumKeccakF = inp.MaxNumKeccakF
		size          = utils.NextPowerOfTwo(maxNumKeccakF * generic.KeccakUsecase.BlockSizeBytes())

		// apply import and pad
		inpImportPadd = importpad.ImportAndPadInputs{
			Name: "KECCAK",
			Src: generic.GenericByteModule{
				Data: inp.Provider.Data,
			},
			PaddingStrategy: generic.KeccakUsecase,
		}

		imported = importpad.ImportAndPad(comp, inpImportPadd, size)

		// apply packing
		inpPck = packing.PackingInput{
			MaxNumBlocks: maxNumKeccakF,
			PackingParam: generic.KeccakUsecase,
			Imported: packing.Importation{
				Limb:      imported.Limbs,
				NByte:     imported.NBytes,
				IsNewHash: imported.IsNewHash,
				IsActive:  imported.IsActive,
			},
			Name: "KECCAK",
		}

		packing = packing.NewPack(comp, inpPck)

		// apply customized keccak over the blocks
		cKeccakInp = KeccakOverBlockInputs{
			LaneInfo: iokeccakf.LaneInfo{
				Lane:                 packing.Repacked.Lanes,
				IsBeginningOfNewHash: packing.Repacked.IsBeginningOfNewHash,
				IsLaneActive:         packing.Repacked.IsLaneActive,
			},

			KeccakfSize: keccakf.NumRows(maxNumKeccakF),
		}

		cKeccak = NewKeccakOverBlocks(comp, cKeccakInp)
	)

	if inp.Provider.Info.HashHi.NumLimbs() != common.NbLimbU128 {
		panic("len(inp.Provider.Info.HashHi) != common.NbLimbU128")
	}

	comp.InsertProjection("KECCAK_RES_HI",
		query.ProjectionInput{
			ColumnA: cKeccak.Outputs.Hash[:8],
			ColumnB: inp.Provider.Info.HashHi.ToBigEndianLimbs().GetLimbs(),
			FilterA: cKeccak.Outputs.IsHash,
			FilterB: inp.Provider.Info.IsHashHi,
		},
	)

	if inp.Provider.Info.HashLo.NumLimbs() != common.NbLimbU128 {
		panic("len(inp.Provider.Info.HashLo) != common.NbLimbU128")
	}

	comp.InsertProjection("KECCAK_RES_LO",
		query.ProjectionInput{
			ColumnA: cKeccak.Outputs.Hash[common.NbLimbU128:],
			ColumnB: inp.Provider.Info.HashLo.ToBigEndianLimbs().GetLimbs(),
			FilterA: cKeccak.Outputs.IsHash,
			FilterB: inp.Provider.Info.IsHashLo,
		},
	)

	// set the module
	m := &KeccakSingleProvider{
		Inputs:           &inp,
		MaxNumKeccakF:    maxNumKeccakF,
		ImportPad:        imported,
		Packing:          packing,
		KeccakOverBlocks: cKeccak,
	}

	return m
}

// It implements [wizard.ProverAction] for keccak.
func (m *KeccakSingleProvider) Run(run *wizard.ProverRuntime) {

	// assign ImportAndPad module
	m.ImportPad.Run(run)
	// assign packing module
	m.Packing.Run(run)
	// assign keccak over blocks module
	providerBytes := m.Inputs.Provider.Data.ScanStreams(run)
	m.KeccakOverBlocks.Inputs.Provider = providerBytes
	m.KeccakOverBlocks.Run(run)
}
