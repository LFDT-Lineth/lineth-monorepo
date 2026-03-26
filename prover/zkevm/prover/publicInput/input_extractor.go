package publicInput

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/protocol/accessors"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
)

const (
	PublicInputExtractorMetadata = "FunctionalInputExtractor"
)

// FunctionalInputExtractor is a collection of values making up the public
// inputs.
type FunctionalInputExtractor struct {

	// DataNbBytes fetches the byte size of the execution data. It is important
	// to include it as the execution data hashing would be vulnerable to padding
	// attacks.
	DataNbBytes wizard.PublicInput

	// DataChecksum returns the hash of the execution data
	DataChecksum [common.NbLimbU128]wizard.PublicInput
	DataSZX      wizard.PublicInput
	DataSZY      wizard.PublicInput
	L2Messages   [][common.NbLimbU256]wizard.PublicInput

	// InitialStateRootHash and FinalStateRootHash are resp the initial and
	// root hash of the state for the
	InitialStateRootHash, FinalStateRootHash                  [common.NbElemPerHash]wizard.PublicInput
	InitialBlockNumber, FinalBlockNumber                      [common.NbLimbU48]wizard.PublicInput
	InitialBlockTimestamp, FinalBlockTimestamp                [common.NbLimbU128]wizard.PublicInput
	FirstRollingHashUpdate, LastRollingHashUpdate             [common.NbLimbU256]wizard.PublicInput
	FirstRollingHashUpdateNumber, LastRollingHashUpdateNumber [common.NbLimbU128]wizard.PublicInput

	ChainID              [common.NbLimbU256]wizard.PublicInput
	NBytesChainID        wizard.PublicInput
	L2MessageServiceAddr [common.NbLimbEthAddress]wizard.PublicInput
	CoinBase             [common.NbLimbEthAddress]wizard.PublicInput
	BaseFee              [common.NbLimbU128]wizard.PublicInput
}

// FunctionalInputExtractor is a collection over LocalOpeningQueries that can be
// used to check the values contained in the Wizard witness are consistent with
// the statement of the outer-proof.

// Run crawls the fields of fie using the reflect package
func (fie *FunctionalInputExtractor) Run(run *wizard.ProverRuntime) {

	assignLO := func(pi wizard.PublicInput) {
		q, ok := pi.Acc.(*accessors.FromLocalOpeningYAccessor)
		if !ok {
			utils.Panic("pi.Acc is not a LocalOpening")
		}
		run.AssignLocalPoint(q.Q.ID, q.Q.Pol.GetColAssignmentAt(run, 0))
	}

	assignLOs := func(qs []wizard.PublicInput) {
		for _, q := range qs {
			assignLO(q)
		}
	}

	assignLO(fie.DataNbBytes)
	assignLO(fie.NBytesChainID)
	assignLOs(fie.DataChecksum[:])
	assignLOs(fie.InitialStateRootHash[:])
	assignLOs(fie.FinalStateRootHash[:])
	assignLOs(fie.InitialBlockNumber[:])
	assignLOs(fie.FinalBlockNumber[:])
	assignLOs(fie.InitialBlockTimestamp[:])
	assignLOs(fie.FinalBlockTimestamp[:])
	assignLOs(fie.FirstRollingHashUpdate[:])
	assignLOs(fie.LastRollingHashUpdate[:])
	assignLOs(fie.FirstRollingHashUpdateNumber[:])
	assignLOs(fie.LastRollingHashUpdateNumber[:])
	assignLOs(fie.ChainID[:])
	assignLOs(fie.L2MessageServiceAddr[:])
	assignLOs(fie.CoinBase[:])
	assignLOs(fie.BaseFee[:])

	for i := range fie.L2Messages {
		assignLOs(fie.L2Messages[i][:])
	}
}

// GetExtractor returns [FunctionalInputExtractor] giving access to the totality
// of the public inputs recovered by the public input module.
func (pi *PublicInput) generateExtractor(comp *wizard.CompiledIOP) {

	newLoPublicInput := func(col ifaces.Column, name string) wizard.PublicInput {
		q := comp.InsertLocalOpening(0, ifaces.QueryIDf("%s_%s", "PUBLIC_INPUT_LOCAL_OPENING", name), col)
		pi := comp.InsertPublicInput(
			name,
			accessors.NewLocalOpeningAccessor(q, 0),
		)
		return pi
	}

	newLoPublicInputs16 := func(cols [16]ifaces.Column, name string) [16]wizard.PublicInput {
		var pis [16]wizard.PublicInput
		for i, col := range cols {
			pis[i] = newLoPublicInput(col, fmt.Sprintf("%s_%d", name, i))
		}
		return pis
	}

	newLoPublicInputs10 := func(cols [10]ifaces.Column, name string) [10]wizard.PublicInput {
		var pis [10]wizard.PublicInput
		for i, col := range cols {
			pis[i] = newLoPublicInput(col, fmt.Sprintf("%s_%d", name, i))
		}
		return pis
	}

	newLoPublicInputs8 := func(cols [8]ifaces.Column, name string) [8]wizard.PublicInput {
		var pis [8]wizard.PublicInput
		for i, col := range cols {
			pis[i] = newLoPublicInput(col, fmt.Sprintf("%s_%d", name, i))
		}
		return pis
	}

	newLoPublicInputs3 := func(cols [3]ifaces.Column, name string) [3]wizard.PublicInput {
		var pis [3]wizard.PublicInput
		for i, col := range cols {
			pis[i] = newLoPublicInput(col, fmt.Sprintf("%s_%d", name, i))
		}
		return pis
	}

	pi.Extractor = FunctionalInputExtractor{
		DataNbBytes:                  newLoPublicInput(pi.DataNbBytes, DataNbBytes),
		NBytesChainID:                newLoPublicInput(pi.ChainIDFetcher.NBytesChainID, NBytesChainID),
		DataChecksum:                 newLoPublicInputs8(pi.ExecPoseidonHasher.HashFinal, DataChecksum),
		InitialStateRootHash:         newLoPublicInputs8(pi.RootHashFetcher.First, InitialStateRootHash),
		FinalStateRootHash:           newLoPublicInputs8(pi.RootHashFetcher.Last, FinalStateRootHash),
		InitialBlockNumber:           newLoPublicInputs3(pi.BlockDataFetcher.FirstBlockID, InitialBlockNumber),
		FinalBlockNumber:             newLoPublicInputs3(pi.BlockDataFetcher.LastBlockID, FinalBlockNumber),
		InitialBlockTimestamp:        newLoPublicInputs8(pi.BlockDataFetcher.FirstTimestamp, InitialBlockTimestamp),
		FinalBlockTimestamp:          newLoPublicInputs8(pi.BlockDataFetcher.LastTimestamp, FinalBlockTimestamp),
		FirstRollingHashUpdate:       newLoPublicInputs16(pi.RollingHashFetcher.First, FirstRollingHashUpdate),
		LastRollingHashUpdate:        newLoPublicInputs16(pi.RollingHashFetcher.Last, LastRollingHashUpdate),
		FirstRollingHashUpdateNumber: newLoPublicInputs8(pi.RollingHashFetcher.FirstMessageNo, FirstRollingHashUpdateNumber),
		LastRollingHashUpdateNumber:  newLoPublicInputs8(pi.RollingHashFetcher.LastMessageNo, LastRollingHashUpdateNumber),
		ChainID:                      newLoPublicInputs16(pi.ChainIDFetcher.ChainID, ChainID),
		L2MessageServiceAddr:         newLoPublicInputs10(pi.Aux.LogSelectors.L2BridgeAddressCol, L2MessageServiceAddr),
		CoinBase:                     newLoPublicInputs10(pi.BlockDataFetcher.CoinBase, CoinBase),
		BaseFee:                      newLoPublicInputs8(pi.BlockDataFetcher.BaseFee, BaseFee),
		DataSZX:                      comp.InsertPublicInput(ExecDataSchwarzZipfelX, accessors.NewFromPublicColumn(pi.ExecDataSchwarzZipfelX, 0)),
		DataSZY:                      comp.InsertPublicInput(ExecDataSchwarzZipfelY, pi.ExecDataSchwarzZipfelY),
	}

	pi.Extractor.L2Messages = make([][16]wizard.PublicInput, pi.Settings.BlockL2L1Logs)

	for i := range pi.Extractor.L2Messages {
		for j := range pi.Extractor.L2Messages[i] {
			col := pi.L2L1LogCompacter.CompactifiedColumns[j]
			col = column.Shift(col, i)
			pi.Extractor.L2Messages[i][j] = newLoPublicInput(
				col, fmt.Sprintf("L2Messages_MsgNo-%d_Limb-%d", i, j),
			)
		}
	}

	comp.ExtraData[PublicInputExtractorMetadata] = &pi.Extractor
}
