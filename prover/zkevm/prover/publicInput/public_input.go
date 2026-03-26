package publicInput

import (
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/protocol/accessors"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/expr_handle"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/functionals"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/limbs"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/consensys/linea-monorepo/prover/zkevm/arithmetization"
	pack "github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/packing"
	arith "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/arith_struct"
	edc "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/execution_data_collector"
	fetch "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/fetchers_arithmetization"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/logs"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/statemanager/statesummary"
	"github.com/ethereum/go-ethereum/common"
)

const (
	DataNbBytes                  = "DataNbBytes"
	DataChecksum                 = "DataChecksum"
	L2MessageHash                = "L2MessageHash"
	InitialStateRootHash         = "InitialStateRootHash"
	FinalStateRootHash           = "FinalStateRootHash"
	InitialBlockNumber           = "InitialBlockNumber"
	FinalBlockNumber             = "FinalBlockNumber"
	InitialBlockTimestamp        = "InitialBlockTimestamp"
	FinalBlockTimestamp          = "FinalBlockTimestamp"
	FirstRollingHashUpdate       = "FirstRollingHashUpdate"
	LastRollingHashUpdate        = "LastRollingHashUpdate"
	FirstRollingHashUpdateNumber = "FirstRollingHashUpdateNumber"
	LastRollingHashUpdateNumber  = "LastRollingHashNumberUpdate"
	ChainID                      = "ChainID"
	NBytesChainID                = "NBytesChainID"
	L2MessageServiceAddr         = "L2MessageServiceAddr"
	CoinBase                     = "CoinBase"
	BaseFee                      = "BaseFee"
	ExecDataSchwarzZipfelX       = "ExecDataSchwarzZipfelX"
	ExecDataSchwarzZipfelY       = "ExecDataSchwarzZipfelY"
)

// PublicInput collects a number of submodules responsible for collecting the
// wizard witness data holding the public inputs of the execution circuit.
type PublicInput struct {
	Settings                  Settings
	Inputs                    InputModules
	Aux                       AuxiliaryModules
	BlockDataFetcher          *fetch.BlockDataFetcher
	RootHashFetcher           *fetch.RootHashFetcher
	RollingHashFetcher        *logs.RollingSelector
	L2L1LogCompacter          *dedicated.Compactification
	ExecPoseidonHasher        edc.PoseidonHasher
	DataNbBytes               ifaces.Column
	ChainIDFetcher            fetch.ChainIDFetcher
	Extractor                 FunctionalInputExtractor
	ExecDataSchwarzZipfelX    ifaces.Column
	ExecDataSchwarzZipfelY    ifaces.Accessor
	ExecDataSchwarzZipfelEval *functionals.CoeffEvalProverAction
}

// AuxiliaryModules are intermediary modules needed to assign the data in the PublicInput
type AuxiliaryModules struct {
	FetchedL2L1, FetchedRollingMsg, FetchedRollingHash logs.ExtractedData
	LogSelectors                                       logs.Selectors
	BlockTxnMetadata                                   fetch.BlockTxnMetadata
	TxnDataFetcher                                     fetch.TxnDataFetcher
	RlpTxnFetcher                                      fetch.RlpTxnFetcher
	ChainIDFetcher                                     fetch.ChainIDFetcher
	ExecDataCollector                                  *edc.ExecutionDataCollector
	ExecDataCollectorPadding                           wizard.ProverAction
	ExecDataCollectorPacking                           pack.Packing
	PadderPacker                                       edc.PadderPacker
	// FlatExecData is a column that interleaves all 8 OuterColumns into a
	// single column for the Schwarz-Zipfel polynomial evaluation. The layout
	// is: FlatExecData[8*r+k] = OuterColumns[k][r], which produces the same
	// coefficient sequence as the raw execution data byte pairs.
	FlatExecData ifaces.Column
}

// Settings contains options for proving and verifying that the public inputs are computed properly.
type Settings struct {
	Name          string
	BlockL2L1Logs int
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
func NewPublicInputZkEVM(comp *wizard.CompiledIOP, settings *Settings, ss *statesummary.Module, a *arithmetization.Arithmetization) PublicInput {

	settings.Name = "PUBLIC_INPUT"

	inputModules := newPublicInput(
		comp,
		&InputModules{
			BlockData: &arith.BlockDataCols{
				RelBlock: a.ColumnOf(comp, "blockdata", "REL_BLOCK"),
				Inst:     a.ColumnOf(comp, "blockdata", "INST"),
				Ct:       a.ColumnOf(comp, "blockdata", "CT"),
				Data: limbs.FuseLimbs(
					a.GetLimbsOfU128Be(comp, "blockdata", "DATA_HI").AsDynSize(),
					a.GetLimbsOfU128Be(comp, "blockdata", "DATA_LO").AsDynSize(),
				).AssertUint256(),
				FirstBlock: a.GetLimbsOfU48Be(comp, "blockdata", "FIRST_BLOCK_NUMBER").LimbsArr3(),
			},
			TxnData: &arith.TxnData{
				AbsTxNum:    a.MashedColumnOf(comp, "txndata", "USER_TXN_NUMBER"),
				AbsTxNumMax: a.ColumnOf(comp, "txndata", "prover___USER_TXN_NUMBER_MAX"),
				Ct:          a.ColumnOf(comp, "txndata", "CT"),
				From: limbs.FuseLimbs(
					a.GetLimbsOfU32Be(comp, "txndata.hub", "FROM_ADDRESS_HI").AsDynSize(),
					a.GetLimbsOfU128Be(comp, "txndata.hub", "FROM_ADDRESS_LO").AsDynSize(),
				).LimbsArr10(),
				IsLastTxOfBlock: a.ColumnOf(comp, "txndata", "prover___IS_LAST_USER_TXN_OF_BLOCK"),
				RelBlock:        a.ColumnOf(comp, "txndata", "BLK_NUMBER"),
				RelTxNum:        a.ColumnOf(comp, "txndata", "prover___RELATIVE_USER_TXN_NUMBER"),
				RelTxNumMax:     a.ColumnOf(comp, "txndata", "prover___RELATIVE_USER_TXN_NUMBER_MAX"),
				USER:            a.ColumnOf(comp, "txndata", "USER"),
				Selector:        a.ColumnOf(comp, "txndata", "HUB"),
				SYSI:            a.ColumnOf(comp, "txndata", "SYSI"),
				SYSF:            a.ColumnOf(comp, "txndata", "SYSF"),
			},
			RlpTxn: &arith.RlpTxn{
				AbsTxNum:       a.ColumnOf(comp, "rlptxn", "USER_TXN_NUMBER"),
				AbsTxNumMax:    a.ColumnOf(comp, "rlptxn", "prover___USER_TXN_NUMBER_MAX"),
				ToHashByProver: a.ColumnOf(comp, "rlptxn", "TO_HASH_BY_PROVER"),
				Limbs:          a.GetLimbsOfU128Be(comp, "rlptxn", "cmpLIMB").LimbsArr8(),
				NBytes:         a.ColumnOf(comp, "rlptxn", "cmpLIMB_SIZE"),
				TxnPerspective: a.ColumnOf(comp, "rlptxn", "TXN"),
			},
			LogCols: logs.LogColumns{
				IsLog0:       a.ColumnOf(comp, "loginfo", "IS_LOG_X_0"),
				IsLog1:       a.ColumnOf(comp, "loginfo", "IS_LOG_X_1"),
				IsLog2:       a.ColumnOf(comp, "loginfo", "IS_LOG_X_2"),
				IsLog3:       a.ColumnOf(comp, "loginfo", "IS_LOG_X_3"),
				IsLog4:       a.ColumnOf(comp, "loginfo", "IS_LOG_X_4"),
				AbsLogNum:    a.MashedColumnOf(comp, "loginfo", "ABS_LOG_NUM"),
				AbsLogNumMax: a.MashedColumnOf(comp, "loginfo", "ABS_LOG_NUM_MAX"),
				Ct:           a.ColumnOf(comp, "loginfo", "CT"),
				Data:         a.GetLimbsOfU128Be(comp, "loginfo", "DATA_LO").ZeroExtendToSize(16).LimbsArr16(),
				TxEmitsLogs:  a.ColumnOf(comp, "loginfo", "TXN_EMITS_LOGS"),
			},
			StateSummary: ss,
		},
		*settings,
	)

	return inputModules
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
	blockDataFetcher := fetch.NewBlockDataFetcher(comp, "PUBLIC_INPUT_TIMESTAMP_FETCHER", inp.BlockData)
	fetch.DefineBlockDataFetcher(comp, blockDataFetcher, "PUBLIC_INPUT_TIMESTAMP_FETCHER", inp.BlockData)

	// Logs: Fetchers, Selectors and Hasher
	fetchedL2L1 := logs.NewExtractedData(comp, inp.LogCols.Ct.Size(), "PUBLIC_INPUT_L2L1LOGS")
	fetchedRollingMsg := logs.NewExtractedData(comp, inp.LogCols.Ct.Size(), "PUBLIC_INPUT_ROLLING_MSG")
	fetchedRollingHash := logs.NewExtractedData(comp, inp.LogCols.Ct.Size(), "PUBLIC_INPUT_ROLLING_HASH")
	logSelectors := logs.NewSelectorColumns(comp, inp.LogCols)
	l2L1LogLoader := logs.NewL2L1LogLoader(comp, inp.LogCols.Ct.Size(), "PUBLIC_INPUT_L2L1LOGS", fetchedL2L1)
	rollingSelector := logs.NewRollingSelector(comp, "PUBLIC_INPUT_ROLLING_SEL", fetchedRollingHash.Data[0].Size())

	// Define Logs: Fetchers, Selectors and Hasher
	logs.DefineExtractedData(comp, inp.LogCols, logSelectors, fetchedL2L1, logs.L2L1)
	logs.DefineExtractedData(comp, inp.LogCols, logSelectors, fetchedRollingMsg, logs.RollingMsgNo)
	logs.DefineExtractedData(comp, inp.LogCols, logSelectors, fetchedRollingHash, logs.RollingHash)
	logs.DefineRollingSelector(comp, rollingSelector, "PUBLIC_INPUT_ROLLING_SEL", fetchedRollingHash, fetchedRollingMsg)

	// RootHash fetcher from the StateSummary
	rootHashFetcher := fetch.NewRootHashFetcher(comp, "PUBLIC_INPUT_ROOT_HASH_FETCHER", inp.StateSummary.IsActive.Size())
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

	// ChainID fetcher
	chainIDFetcher := fetch.NewChainIDFetcher(comp, "PUBLIC_INPUT_CHAIN_ID_FETCHER", inp.BlockData)
	fetch.DefineChainIDFetcher(comp, &chainIDFetcher, "PUBLIC_INPUT_CHAIN_ID_FETCHER", inp.BlockData)

	// ExecutionDataCollector
	limbColSize := edc.GetSummarySize(inp.TxnData, inp.RlpTxn)
	limbColSize = 4 * limbColSize // we need to artificially blow up the column size by 2, or padding will fail
	execDataCollector := edc.NewExecutionDataCollector(comp, "EXECUTION_DATA_COLLECTOR", limbColSize)
	edc.DefineExecutionDataCollector(comp, execDataCollector, "EXECUTION_DATA_COLLECTOR", blockDataFetcher, blockTxnMeta, txnDataFetcher, rlpFetcher)

	padderPacker := edc.NewPadderPacker(comp, execDataCollector.Limbs, execDataCollector.NoBytes, execDataCollector.IsActive, "PADDER_PACKER_FOR_EXECUTION_DATA_COLLECTOR")
	edc.DefinePadderPacker(comp, &padderPacker, "POSEIDON_PADDER_PACKER_FOR_EXECUTION_DATA_COLLECTOR")

	// ExecutionDataCollector: Hashing
	poseidonHasher := edc.NewPoseidonHasher(comp, padderPacker.OuterColumns, padderPacker.OuterIsActive, "EXECUTION_DATA_COLLECTOR_POSEIDON2_HASHER")
	edc.DefinePoseidonHasher(comp, poseidonHasher, "EXECUTION_DATA_COLLECTOR_POSEIDON2_HASHER")

	// ExecutionDataCollector evaluation: we create a flattened column that
	// interleaves all 8 OuterColumns so that the Schwarz-Zipfel polynomial
	// evaluation covers ALL execution data (not just 1/8). The layout is
	// flatExecData[8*r+k] = OuterColumns[k][r], which matches the byte-pair
	// ordering used by the outer circuit and the off-circuit computation.
	outerColSize := padderPacker.OuterColumns[0].Size()
	flatExecData := comp.InsertCommit(0, ifaces.ColIDf("PUBLIC_INPUT_FLAT_EXEC_DATA_FOR_SZ"), 8*outerColSize, true)

	execDataSchwarzZipfelX := comp.InsertProof(0, "PUBLIC_INPUT_EXEC_DATA_SCHWARZ_ZIPFEL_X", 1, false)
	execDataSchwarzZipfelEval, exacDataSchwarzZipfelY := functionals.CoeffEvalNoRegisterPA(
		comp,
		"PUBLIC_INPUT_EXEC_DATA_SCHWARZ_ZIPFEL_X_EVAL",
		accessors.NewFromPublicColumn(execDataSchwarzZipfelX, 0),
		flatExecData,
	)

	publicInput := PublicInput{
		BlockDataFetcher:          blockDataFetcher,
		RootHashFetcher:           rootHashFetcher,
		RollingHashFetcher:        rollingSelector,
		L2L1LogCompacter:          l2L1LogLoader,
		ExecPoseidonHasher:        poseidonHasher,
		DataNbBytes:               execDataCollector.FinalTotalBytesCounter,
		ChainIDFetcher:            chainIDFetcher,
		ExecDataSchwarzZipfelX:    execDataSchwarzZipfelX,
		ExecDataSchwarzZipfelY:    exacDataSchwarzZipfelY,
		ExecDataSchwarzZipfelEval: execDataSchwarzZipfelEval,
		Inputs:                    *inp,
		Settings:                  settings,
		Aux: AuxiliaryModules{
			FetchedL2L1:        fetchedL2L1,
			FetchedRollingMsg:  fetchedRollingMsg,
			FetchedRollingHash: fetchedRollingHash,
			LogSelectors:       logSelectors,
			BlockTxnMetadata:   blockTxnMeta,
			TxnDataFetcher:     txnDataFetcher,
			RlpTxnFetcher:      rlpFetcher,
			ExecDataCollector:  execDataCollector,
			PadderPacker:       padderPacker,
			ChainIDFetcher:     chainIDFetcher,
			FlatExecData:       flatExecData,
		},
	}

	publicInput.generateExtractor(comp)

	return publicInput
}

// Assign both a PublicInput and AuxiliaryModules using data from InputModules.
// The AuxiliaryModules are intermediary modules needed to both define and assign the PublicInput.
func (pub *PublicInput) Assign(run *wizard.ProverRuntime, l2BridgeAddress common.Address, blockHashList []types.FullBytes32, execDataSchwarzZipfelX fext.Element) {

	var (
		inp = pub.Inputs
		aux = pub.Aux
	)

	// assign the timestamp module
	fetch.AssignBlockDataFetcher(run, pub.BlockDataFetcher, inp.BlockData)
	// assign the log modules
	aux.LogSelectors.Assign(run, l2BridgeAddress)
	logs.AssignExtractedData(run, inp.LogCols, aux.LogSelectors, aux.FetchedL2L1, logs.L2L1)
	logs.AssignExtractedData(run, inp.LogCols, aux.LogSelectors, aux.FetchedRollingMsg, logs.RollingMsgNo)
	logs.AssignExtractedData(run, inp.LogCols, aux.LogSelectors, aux.FetchedRollingHash, logs.RollingHash)
	pub.L2L1LogCompacter.Run(run)
	logs.AssignRollingSelector(run, pub.RollingHashFetcher, aux.FetchedRollingHash, aux.FetchedRollingMsg)
	// assign the root hash fetcher
	fetch.AssignRootHashFetcher(run, pub.RootHashFetcher, *inp.StateSummary)
	// assign the execution data collector's necessary fetchers
	fetch.AssignBlockTxnMetadata(run, aux.BlockTxnMetadata, inp.TxnData)
	fetch.AssignTxnDataFetcher(run, aux.TxnDataFetcher, inp.TxnData)
	fetch.AssignRlpTxnFetcher(run, &aux.RlpTxnFetcher, inp.RlpTxn)
	fetch.AssignChainIDFetcher(run, &pub.ChainIDFetcher, inp.BlockData)

	// assign the ExecutionDataCollector
	edc.AssignExecutionDataCollector(run, aux.ExecDataCollector, pub.BlockDataFetcher, aux.BlockTxnMetadata, aux.TxnDataFetcher, aux.RlpTxnFetcher, blockHashList)

	edc.AssignPadderPacker(run, aux.PadderPacker)
	// assign the hasher
	edc.AssignPoseidonHasher(run, pub.ExecPoseidonHasher, aux.PadderPacker.OuterColumns, aux.PadderPacker.OuterIsActive)

	// Assign the flattened execution data column for Schwarz-Zipfel.
	// Interleave all 8 OuterColumns: FlatExecData[8*r+k] = OuterColumns[k][r]
	{
		outerSize := aux.PadderPacker.OuterColumns[0].Size()
		flatData := make([]field.Element, 8*outerSize)
		for k := 0; k < 8; k++ {
			col := aux.PadderPacker.OuterColumns[k].GetColAssignment(run)
			for r := 0; r < outerSize; r++ {
				flatData[8*r+k] = col.Get(r)
			}
		}
		run.AssignColumn(pub.Aux.FlatExecData.GetColID(), smartvectors.NewRegular(flatData))
	}

	// assign the schwharz-zipfel work
	run.AssignColumn(pub.ExecDataSchwarzZipfelX.GetColID(), smartvectors.NewRegularExt([]fext.Element{execDataSchwarzZipfelX}))
	pub.ExecDataSchwarzZipfelEval.Run(run)

	pub.Extractor.Run(run)

	// Assign the mashed up log columns that have not been assigned
	_ = expr_handle.GetExprHandleAssignment(run, pub.Inputs.TxnData.AbsTxNum)
	_ = expr_handle.GetExprHandleAssignment(run, pub.Inputs.LogCols.AbsLogNum)
	_ = expr_handle.GetExprHandleAssignment(run, pub.Inputs.LogCols.AbsLogNumMax)
}
