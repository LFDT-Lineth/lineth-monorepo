package publicInput

import (
	"github.com/consensys/linea-monorepo/prover/protocol/accessors"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/generic"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/importpad"
	pack "github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/packing"
	arith "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/arith_struct"
	edc "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/execution_data_collector"
	fetch "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/fetchers_arithmetization"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/logs"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/statemanager/statesummary"
	"github.com/ethereum/go-ethereum/common"
)

var (
	DataNbBytes                  = "DataNbBytes"
	DataChecksum                 = "DataChecksum"
	L2MessageHash                = "L2MessageHash"
	InitialStateRootHash         = "InitialStateRootHash"
	FinalStateRootHash           = "FinalStateRootHash"
	InitialBlockNumber           = "InitialBlockNumber"
	FinalBlockNumber             = "FinalBlockNumber"
	InitialBlockTimestamp        = "InitialBlockTimestamp"
	FinalBlockTimestamp          = "FinalBlockTimestamp"
	FirstRollingHashUpdate_0     = "FirstRollingHashUpdate_0"
	FirstRollingHashUpdate_1     = "FirstRollingHashUpdate_1"
	LastRollingHashUpdate_0      = "LastRollingHashUpdate_0"
	LastRollingHashUpdate_1      = "LastRollingHashUpdate_1"
	FirstRollingHashUpdateNumber = "FirstRollingHashUpdateNumber"
	LastRollingHashNumberUpdate  = "LastRollingHashNumberUpdate"
	ChainID                      = "ChainID"
	NBytesChainID                = "NBytesChainID"
	L2MessageServiceAddrHi       = "L2MessageServiceAddrHi"
	L2MessageServiceAddrLo       = "L2MessageServiceAddrLo"
)

// PublicInput collects a number of submodules responsible for collecting the
// wizard witness data holding the public inputs of the execution circuit.
type PublicInput struct {
	Inputs             InputModules
	Aux                AuxiliaryModules
	TimestampFetcher   fetch.TimestampFetcher
	RootHashFetcher    fetch.RootHashFetcher
	RollingHashFetcher logs.RollingSelector
	LogHasher          logs.LogHasher
	ExecMiMCHasher     edc.MIMCHasher
	DataNbBytes        ifaces.Column
	ChainID            ifaces.Column
	ChainIDNBytes      ifaces.Column
	Extractor          FunctionalInputExtractor
}

// AuxiliaryModules are intermediary modules needed to assign the data in the PublicInput
type AuxiliaryModules struct {
	fetchedL2L1, fetchedRollingMsg, fetchedRollingHash logs.ExtractedData
	logSelectors                                       logs.Selectors
	blockTxnMetadata                                   fetch.BlockTxnMetadata
	txnDataFetcher                                     fetch.TxnDataFetcher
	rlpTxnFetcher                                      fetch.RlpTxnFetcher
	execDataCollector                                  edc.ExecutionDataCollector
	execDataCollectorPadding                           wizard.ProverAction
	execDataCollectorPacking                           pack.Packing
}

// Settings contains options for proving and verifying that the public inputs are computed properly.
type Settings struct {
	Name string
}

// InputModules groups several arithmetization modules needed to compute the public input.
type InputModules struct {
	BlockData    *arith.BlockDataCols
	TxnData      *arith.TxnData
	RlpTxn       *arith.RlpTxn
	LogCols      logs.LogColumns
	StateSummary *statesummary.Module
}

// NewPublicInputZkEVM constructs and returns a [PublicInput] module using the
// right columns from the arithmetization by their names.
func NewPublicInputZkEVM(comp *wizard.CompiledIOP, settings *Settings, ss *statesummary.Module) PublicInput {

	getCol := func(s string) ifaces.Column {
		return comp.Columns.GetHandle(ifaces.ColID(s))
	}

	settings.Name = "PUBLIC_INPUT"

	return newPublicInput(
		comp,
		&InputModules{
			BlockData: &arith.BlockDataCols{
				RelBlock:   getCol("blockdata.REL_BLOCK"),
				Inst:       getCol("blockdata.INST"),
				Ct:         getCol("blockdata.CT"),
				DataHi:     getCol("blockdata.DATA_HI"),
				DataLo:     getCol("blockdata.DATA_LO"),
				FirstBlock: getCol("blockdata.FIRST_BLOCK_NUMBER"),
			},
			TxnData: &arith.TxnData{
				AbsTxNum:        getCol("txndata.ABS_TX_NUM"),
				AbsTxNumMax:     getCol("txndata.ABS_TX_NUM_MAX"),
				Ct:              getCol("txndata.CT"),
				FromHi:          getCol("txndata.FROM_HI"),
				FromLo:          getCol("txndata.FROM_LO"),
				IsLastTxOfBlock: getCol("txndata.IS_LAST_TX_OF_BLOCK"),
				RelBlock:        getCol("txndata.REL_BLOCK"),
				RelTxNum:        getCol("txndata.REL_TX_NUM"),
				RelTxNumMax:     getCol("txndata.REL_TX_NUM_MAX"),
			},
			RlpTxn: &arith.RlpTxn{
				AbsTxNum:       getCol("rlptxn.ABS_TX_NUM"),
				AbsTxNumMax:    getCol("rlptxn.ABS_TX_NUM_INFINY"),
				ToHashByProver: getCol("rlptxn.TO_HASH_BY_PROVER"),
				Limb:           getCol("rlptxn.LIMB"),
				NBytes:         getCol("rlptxn.nBYTES"),
				Done:           getCol("rlptxn.DONE"),
				IsPhaseChainID: getCol("rlptxn.IS_PHASE_CHAIN_ID"),
			},
			LogCols: logs.LogColumns{
				IsLog0:       getCol("loginfo.IS_LOG_X_0"),
				IsLog1:       getCol("loginfo.IS_LOG_X_1"),
				IsLog2:       getCol("loginfo.IS_LOG_X_2"),
				IsLog3:       getCol("loginfo.IS_LOG_X_3"),
				IsLog4:       getCol("loginfo.IS_LOG_X_4"),
				AbsLogNum:    getCol("loginfo.ABS_LOG_NUM"),
				AbsLogNumMax: getCol("loginfo.ABS_LOG_NUM_MAX"),
				Ct:           getCol("loginfo.CT"),
				DataHi:       getCol("loginfo.DATA_HI"),
				DataLo:       getCol("loginfo.DATA_LO"),
				TxEmitsLogs:  getCol("loginfo.TXN_EMITS_LOGS"),
			},
			StateSummary: ss,
		},
		*settings)
}

// newPublicInput receives as input a series of modules and returns a *PublicInput and
// an *AuxiliaryModules struct. The AuxiliaryModules are intermediary modules needed to
// both define and assign the PublicInput.
func newPublicInput(
	comp *wizard.CompiledIOP,
	inp *InputModules,
	settings Settings,
) PublicInput {

	if len(settings.Name) == 0 {
		utils.Panic("no name was provided in settings: %++v", settings)
	}

	// Timestamps
	timestampFetcher := fetch.NewTimestampFetcher(comp, "PUBLIC_INPUT_TIMESTAMP_FETCHER", inp.BlockData)
	fetch.DefineTimestampFetcher(comp, &timestampFetcher, "PUBLIC_INPUT_TIMESTAMP_FETCHER", inp.BlockData)

	// Logs: Fetchers, Selectors and Hasher
	fetchedL2L1 := logs.NewExtractedData(comp, inp.LogCols.Ct.Size(), "PUBLIC_INPUT_L2L1LOGS")
	fetchedRollingMsg := logs.NewExtractedData(comp, inp.LogCols.Ct.Size(), "PUBLIC_INPUT_ROLLING_MSG")
	fetchedRollingHash := logs.NewExtractedData(comp, inp.LogCols.Ct.Size(), "PUBLIC_INPUT_ROLLING_HASH")
	logSelectors := logs.NewSelectorColumns(comp, inp.LogCols)
	logHasherL2l1 := logs.NewLogHasher(comp, inp.LogCols.Ct.Size(), "PUBLIC_INPUT_L2L1LOGS")
	rollingSelector := logs.NewRollingSelector(comp, "PUBLIC_INPUT_ROLLING_SEL")

	// Define Logs: Fetchers, Selectors and Hasher
	logs.DefineExtractedData(comp, inp.LogCols, logSelectors, fetchedL2L1, logs.L2L1)
	logs.DefineExtractedData(comp, inp.LogCols, logSelectors, fetchedRollingMsg, logs.RollingMsgNo)
	logs.DefineExtractedData(comp, inp.LogCols, logSelectors, fetchedRollingHash, logs.RollingHash)
	logs.DefineHasher(comp, logHasherL2l1, "PUBLIC_INPUT_L2L1LOGS", fetchedL2L1)
	logs.DefineRollingSelector(comp, rollingSelector, "PUBLIC_INPUT_ROLLING_SEL", fetchedRollingHash, fetchedRollingMsg)

	// RootHash fetcher from the StateSummary
	rootHashFetcher := fetch.NewRootHashFetcher(comp, "PUBLIC_INPUT_ROOT_HASH_FETCHER")
	fetch.DefineRootHashFetcher(comp, rootHashFetcher, "PUBLIC_INPUT_ROOT_HASH_FETCHER", *inp.StateSummary)

	// Metadata fetcher
	blockTxnMeta := fetch.NewBlockTxnMetadata(comp, "BLOCK_TX_METADATA", inp.TxnData)
	fetch.DefineBlockTxnMetaData(comp, &blockTxnMeta, "BLOCK_TX_METADATA", inp.TxnData)

	// TxnData fetcher
	txnDataFetcher := fetch.NewTxnDataFetcher(comp, "PUBLIC_INPUT_TXN_DATA_FETCHER", inp.TxnData)
	fetch.DefineTxnDataFetcher(comp, &txnDataFetcher, "PUBLIC_INPUT_TXN_DATA_FETCHER", inp.TxnData)

	// RlpTxn fetcher
	rlpFetcher := fetch.NewRlpTxnFetcher(comp, "PUBLIC_INPUT_RLP_TXN_FETCHER", inp.RlpTxn)
	fetch.DefineRlpTxnFetcher(comp, &rlpFetcher, "PUBLIC_INPUT_RLP_TXN_FETCHER", inp.RlpTxn)

	// ExecutionDataCollector
	limbColSize := edc.GetSummarySize(inp.TxnData, inp.RlpTxn)
	limbColSize = 2 * limbColSize // we need to artificially blow up the column size by 2, or padding will fail
	execDataCollector := edc.NewExecutionDataCollector(comp, "EXECUTION_DATA_COLLECTOR", limbColSize)
	edc.DefineExecutionDataCollector(comp, &execDataCollector, "EXECUTION_DATA_COLLECTOR", timestampFetcher, blockTxnMeta, txnDataFetcher, rlpFetcher)

	// ExecutionDataCollector: Padding
	importInp := importpad.ImportAndPadInputs{
		Name: settings.Name,
		Src: generic.GenericByteModule{Data: generic.GenDataModule{
			HashNum: execDataCollector.HashNum,
			Index:   execDataCollector.Ct,
			ToHash:  execDataCollector.IsActive,
			NBytes:  execDataCollector.NoBytes,
			Limb:    execDataCollector.Limb,
		}},
		PaddingStrategy: generic.MiMCUsecase,
	}
	padding := importpad.ImportAndPad(comp, importInp, limbColSize)

	// ExecutionDataCollector: Packing
	packingInp := pack.PackingInput{
		MaxNumBlocks: execDataCollector.BlockID.Size(),
		PackingParam: generic.MiMCUsecase,
		Imported: pack.Importation{
			Limb:      padding.Limbs,
			NByte:     padding.NBytes,
			IsNewHash: padding.IsNewHash,
			IsActive:  padding.IsActive,
		},
		Name: "EXECUTION_DATA_MIMC",
	}
	packingMod := pack.NewPack(comp, packingInp)

	// ExecutionDataCollector: Hashing
	mimcHasher := edc.NewMIMCHasher(comp, packingMod.Repacked.Lanes, packingMod.Repacked.IsLaneActive, "MIMC_HASHER")
	mimcHasher.DefineHasher(comp, "EXECUTION_DATA_COLLECTOR_MIMC_HASHER")

	publicInput := PublicInput{
		TimestampFetcher:   timestampFetcher,
		RootHashFetcher:    rootHashFetcher,
		RollingHashFetcher: rollingSelector,
		LogHasher:          logHasherL2l1,
		ExecMiMCHasher:     *mimcHasher,
		DataNbBytes:        execDataCollector.FinalTotalBytesCounter,
		ChainID:            rlpFetcher.ChainID,
		ChainIDNBytes:      rlpFetcher.NBytesChainID,
		Inputs:             *inp,
		Aux: AuxiliaryModules{
			fetchedL2L1:              fetchedL2L1,
			fetchedRollingMsg:        fetchedRollingMsg,
			fetchedRollingHash:       fetchedRollingHash,
			logSelectors:             logSelectors,
			blockTxnMetadata:         blockTxnMeta,
			txnDataFetcher:           txnDataFetcher,
			rlpTxnFetcher:            rlpFetcher,
			execDataCollector:        execDataCollector,
			execDataCollectorPadding: padding,
			execDataCollectorPacking: *packingMod,
		},
	}

	publicInput.generateExtractor(comp)

	return publicInput
}

// Assign both a PublicInput and AuxiliaryModules using data from InputModules.
// The AuxiliaryModules are intermediary modules needed to both define and assign the PublicInput.
func (pub *PublicInput) Assign(run *wizard.ProverRuntime, l2BridgeAddress common.Address) {

	var (
		inp = pub.Inputs
		aux = pub.Aux
	)

	// assign the timestamp module
	fetch.AssignTimestampFetcher(run, pub.TimestampFetcher, inp.BlockData)
	// assign the log modules
	aux.logSelectors.Assign(run, l2BridgeAddress)
	logs.AssignExtractedData(run, inp.LogCols, aux.logSelectors, aux.fetchedL2L1, logs.L2L1)
	logs.AssignExtractedData(run, inp.LogCols, aux.logSelectors, aux.fetchedRollingMsg, logs.RollingMsgNo)
	logs.AssignExtractedData(run, inp.LogCols, aux.logSelectors, aux.fetchedRollingHash, logs.RollingHash)
	logs.AssignHasher(run, pub.LogHasher, aux.fetchedL2L1)
	logs.AssignRollingSelector(run, pub.RollingHashFetcher, aux.fetchedRollingHash, aux.fetchedRollingMsg)
	// assign the root hash fetcher
	fetch.AssignRootHashFetcher(run, pub.RootHashFetcher, *inp.StateSummary)
	// assign the execution data collector's necessary fetchers
	fetch.AssignBlockTxnMetadata(run, aux.blockTxnMetadata, inp.TxnData)
	fetch.AssignTxnDataFetcher(run, aux.txnDataFetcher, inp.TxnData)
	fetch.AssignRlpTxnFetcher(run, &aux.rlpTxnFetcher, inp.RlpTxn)
	// assign the ExecutionDataCollector
	edc.AssignExecutionDataCollector(run, aux.execDataCollector, pub.TimestampFetcher, aux.blockTxnMetadata, aux.txnDataFetcher, aux.rlpTxnFetcher)
	aux.execDataCollectorPadding.Run(run)
	aux.execDataCollectorPacking.Run(run)
	pub.ExecMiMCHasher.AssignHasher(run)
	pub.Extractor.Run(run)
}

// GetExtractor returns [FunctionalInputExtractor] giving access to the totality
// of the public inputs recovered by the public input module.
func (pi *PublicInput) generateExtractor(comp *wizard.CompiledIOP) {

	createNewLocalOpening := func(col ifaces.Column) query.LocalOpening {
		return comp.InsertLocalOpening(0, ifaces.QueryIDf("%s_%s", "PUBLIC_INPUT_LOCAL_OPENING", col.GetColID()), col)
	}

	initialRollingHash := [2]query.LocalOpening{
		createNewLocalOpening(pi.RollingHashFetcher.FirstHi),
		createNewLocalOpening(pi.RollingHashFetcher.FirstLo),
	}

	finalRollingHash := [2]query.LocalOpening{
		createNewLocalOpening(pi.RollingHashFetcher.LastHi),
		createNewLocalOpening(pi.RollingHashFetcher.LastLo),
	}

	pi.Extractor = FunctionalInputExtractor{
		DataNbBytes:                  createNewLocalOpening(pi.DataNbBytes),
		DataChecksum:                 createNewLocalOpening(pi.ExecMiMCHasher.HashFinal),
		L2MessageHash:                createNewLocalOpening(pi.LogHasher.HashFinal),
		InitialStateRootHash:         createNewLocalOpening(pi.RootHashFetcher.First),
		FinalStateRootHash:           createNewLocalOpening(pi.RootHashFetcher.Last),
		InitialBlockNumber:           createNewLocalOpening(pi.TimestampFetcher.FirstBlockID),
		FinalBlockNumber:             createNewLocalOpening(pi.TimestampFetcher.LastBlockID),
		InitialBlockTimestamp:        createNewLocalOpening(pi.TimestampFetcher.First),
		FinalBlockTimestamp:          createNewLocalOpening(pi.TimestampFetcher.Last),
		FirstRollingHashUpdate:       initialRollingHash,
		LastRollingHashUpdate:        finalRollingHash,
		FirstRollingHashUpdateNumber: createNewLocalOpening(pi.RollingHashFetcher.FirstMessageNo),
		LastRollingHashUpdateNumber:  createNewLocalOpening(pi.RollingHashFetcher.LastMessageNo),
		ChainID:                      createNewLocalOpening(pi.ChainID),
		NBytesChainID:                createNewLocalOpening(pi.ChainIDNBytes),
		L2MessageServiceAddrHi:       accessors.NewFromPublicColumn(pi.Aux.logSelectors.L2BridgeAddressColHI, 0),
		L2MessageServiceAddrLo:       accessors.NewFromPublicColumn(pi.Aux.logSelectors.L2BridgeAddressColLo, 0),
	}

	comp.PublicInputs = append(comp.PublicInputs,
		wizard.PublicInput{Name: DataNbBytes, Acc: accessors.NewLocalOpeningAccessor(pi.Extractor.DataNbBytes, 0)},
		wizard.PublicInput{Name: DataChecksum, Acc: accessors.NewLocalOpeningAccessor(pi.Extractor.DataChecksum, 0)},
		wizard.PublicInput{Name: L2MessageHash, Acc: accessors.NewLocalOpeningAccessor(pi.Extractor.L2MessageHash, 0)},
		wizard.PublicInput{Name: InitialStateRootHash, Acc: accessors.NewLocalOpeningAccessor(pi.Extractor.InitialStateRootHash, 0)},
		wizard.PublicInput{Name: FinalStateRootHash, Acc: accessors.NewLocalOpeningAccessor(pi.Extractor.FinalStateRootHash, 0)},
		wizard.PublicInput{Name: InitialBlockNumber, Acc: accessors.NewLocalOpeningAccessor(pi.Extractor.InitialBlockNumber, 0)},
		wizard.PublicInput{Name: FinalBlockNumber, Acc: accessors.NewLocalOpeningAccessor(pi.Extractor.FinalBlockNumber, 0)},
		wizard.PublicInput{Name: InitialBlockTimestamp, Acc: accessors.NewLocalOpeningAccessor(pi.Extractor.InitialBlockTimestamp, 0)},
		wizard.PublicInput{Name: FinalBlockTimestamp, Acc: accessors.NewLocalOpeningAccessor(pi.Extractor.FinalBlockTimestamp, 0)},
		wizard.PublicInput{Name: FirstRollingHashUpdate_0, Acc: accessors.NewLocalOpeningAccessor(pi.Extractor.FirstRollingHashUpdate[0], 0)},
		wizard.PublicInput{Name: FirstRollingHashUpdate_1, Acc: accessors.NewLocalOpeningAccessor(pi.Extractor.FirstRollingHashUpdate[1], 0)},
		wizard.PublicInput{Name: LastRollingHashUpdate_0, Acc: accessors.NewLocalOpeningAccessor(pi.Extractor.LastRollingHashUpdate[0], 0)},
		wizard.PublicInput{Name: LastRollingHashUpdate_1, Acc: accessors.NewLocalOpeningAccessor(pi.Extractor.LastRollingHashUpdate[1], 0)},
		wizard.PublicInput{Name: FirstRollingHashUpdateNumber, Acc: accessors.NewLocalOpeningAccessor(pi.Extractor.FirstRollingHashUpdateNumber, 0)},
		wizard.PublicInput{Name: LastRollingHashNumberUpdate, Acc: accessors.NewLocalOpeningAccessor(pi.Extractor.LastRollingHashUpdateNumber, 0)},
		wizard.PublicInput{Name: ChainID, Acc: accessors.NewLocalOpeningAccessor(pi.Extractor.ChainID, 0)},
		wizard.PublicInput{Name: NBytesChainID, Acc: accessors.NewLocalOpeningAccessor(pi.Extractor.NBytesChainID, 0)},
		wizard.PublicInput{Name: L2MessageServiceAddrHi, Acc: pi.Extractor.L2MessageServiceAddrHi},
		wizard.PublicInput{Name: L2MessageServiceAddrLo, Acc: pi.Extractor.L2MessageServiceAddrLo},
	)
}
