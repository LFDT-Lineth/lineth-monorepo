// The keccak package implements the utilities for proving the hash over a single provider.
// The provider of type [generic.GenericByteModule] encodes the inputs and outputs of hash (related to the same module).
// The inputs and outputs are respectively embedded inside [generic.GenDataModule], and [generic.GenInfoModule].
package keccak

import (
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/generic"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/importpad"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/packing"
)

// KeccakSingleProviderInput stores the inputs for [NewKeccakSingleProvider]
type KeccakSingleProviderInput struct {
	MaxNumKeccakF int
	Provider      generic.GenericByteModule
}

// KeccakSingleProvider stores the hash result and [wizard.ProverAction] of the submodules.
type KeccakSingleProvider struct {
	Inputs         *KeccakSingleProviderInput
	HashHi, HashLo ifaces.Column
	// indicates the active part of HashHi/HashLo
	IsActive      ifaces.Column
	MaxNumKeccakF int

	// prover actions for  internal modules
	Pa_importPad *importpad.Importation
	Pa_packing   *packing.Packing
	Pa_cKeccak   *KeccakOverBlocks
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
			LaneInfo: LaneInfo{
				Lanes:                packing.Repacked.Lanes,
				IsFirstLaneOfNewHash: packing.Repacked.IsFirstLaneOfNewHash,
				IsLaneActive:         packing.Repacked.IsLaneActive,
			},

			MaxNumKeccakF: maxNumKeccakF,
		}
		cKeccak = NewKeccakOverBlocks(comp, cKeccakInp)
	)

	comp.InsertProjection("KECCAK_RES_HI",
		query.ProjectionInput{ColumnA: []ifaces.Column{cKeccak.HashHi},
			ColumnB: []ifaces.Column{inp.Provider.Info.HashHi},
			FilterA: cKeccak.IsActive,
			FilterB: inp.Provider.Info.IsHashHi})

	comp.InsertProjection("KECCAK_RES_LO",
		query.ProjectionInput{ColumnA: []ifaces.Column{cKeccak.HashLo},
			ColumnB: []ifaces.Column{inp.Provider.Info.HashLo},
			FilterA: cKeccak.IsActive,
			FilterB: inp.Provider.Info.IsHashLo})

	// set the module
	m := &KeccakSingleProvider{
		Inputs:        &inp,
		MaxNumKeccakF: maxNumKeccakF,
		HashHi:        cKeccak.HashHi,
		HashLo:        cKeccak.HashLo,
		IsActive:      cKeccak.IsActive,
		Pa_importPad:  imported,
		Pa_packing:    packing,
		Pa_cKeccak:    cKeccak,
	}

	return m
}

// It implements [wizard.ProverAction] for keccak.
func (m *KeccakSingleProvider) Run(run *wizard.ProverRuntime) {

	// assign ImportAndPad module
	m.Pa_importPad.Run(run)
	// assign packing module
	m.Pa_packing.Run(run)
	providerBytes := m.Inputs.Provider.Data.ScanStreams(run)
	m.Pa_cKeccak.Inputs.Provider = providerBytes
	m.Pa_cKeccak.Run(run)
}

func isBlock(col ifaces.Column) []ifaces.Column {
	var isBlock []ifaces.Column
	for j := 0; j < generic.KeccakUsecase.NbOfLanesPerBlock(); j++ {
		isBlock = append(isBlock, col)
	}
	return isBlock
}
