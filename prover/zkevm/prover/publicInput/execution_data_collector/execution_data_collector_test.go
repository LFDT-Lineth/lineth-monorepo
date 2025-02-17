package execution_data_collector

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	arith "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/arith_struct"
	fetch "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/fetchers_arithmetization"
	util "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/utilities"
)

// TestAssignmentExecutionDataCollector tests whether the execution data collector
// defines its constraints and assigns its columns without any errors.
func TestDefineAndAssignmentExecutionDataCollector(t *testing.T) {
	ctBlockData := util.InitializeCsv("../testdata/blockdata_mock.csv", t)
	ctTxnData := util.InitializeCsv("../testdata/txndata_mock.csv", t)
	ctRlpTxn := util.InitializeCsv("../testdata/rlp_txn_mock.csv", t)
	blockHashList := [1 << 10]types.FullBytes32{}

	var (
		edc              ExecutionDataCollector
		btm              fetch.BlockTxnMetadata
		timestampFetcher fetch.TimestampFetcher
		txnDataFetcher   fetch.TxnDataFetcher
		rlpTxnFetcher    fetch.RlpTxnFetcher
		txd              *arith.TxnData
		bdc              *arith.BlockDataCols
		rt               *arith.RlpTxn
	)

	define := func(b *wizard.Builder) {
		// define the arith test modules
		bdc, txd, rt = arith.DefineTestingArithModules(b, ctBlockData, ctTxnData, ctRlpTxn)
		// create and define a metadata fetcher
		btm = fetch.NewBlockTxnMetadata(b.CompiledIOP, "BLOCK_TX_METADATA", txd)
		fetch.DefineBlockTxnMetaData(b.CompiledIOP, &btm, "BLOCK_TX_METADATA", txd)
		// create a new timestamp fetcher
		timestampFetcher = fetch.NewTimestampFetcher(b.CompiledIOP, "TIMESTAMP_FETCHER_FROM_ARITH", bdc)
		// constrain the timestamp fetcher
		fetch.DefineTimestampFetcher(b.CompiledIOP, &timestampFetcher, "TIMESTAMP_FETCHER_FROM_ARITH", bdc)
		txnDataFetcher = fetch.NewTxnDataFetcher(b.CompiledIOP, "TXN_DATA_FETCHER_FROM_ARITH", txd)
		fetch.DefineTxnDataFetcher(b.CompiledIOP, &txnDataFetcher, "TXN_DATA_FETCHER_FROM_ARITH", txd)

		rlpTxnFetcher = fetch.NewRlpTxnFetcher(b.CompiledIOP, "RLP_TXN_FETCHER_FROM_ARITH", rt)
		// constrain the fetcher
		fetch.DefineRlpTxnFetcher(b.CompiledIOP, &rlpTxnFetcher, "RLP_TXN_FETCHER_FROM_ARITH", rt)

		limbColSize := GetSummarySize(txd, rt)
		edc = NewExecutionDataCollector(b.CompiledIOP, "EXECUTION_DATA_COLLECTOR", limbColSize)
		DefineExecutionDataCollector(b.CompiledIOP, &edc, "EXECUTION_DATA_COLLECTOR", timestampFetcher, btm, txnDataFetcher, rlpTxnFetcher)
	}

	prove := func(run *wizard.ProverRuntime) {
		arith.AssignTestingArithModules(run, ctBlockData, ctTxnData, ctRlpTxn)
		fetch.AssignTimestampFetcher(run, timestampFetcher, bdc)
		fetch.AssignBlockTxnMetadata(run, btm, txd)
		fetch.AssignTxnDataFetcher(run, txnDataFetcher, txd)
		fetch.AssignRlpTxnFetcher(run, &rlpTxnFetcher, rt)
		AssignExecutionDataCollector(run, edc, timestampFetcher, btm, txnDataFetcher, rlpTxnFetcher, blockHashList[:])
	}

	comp := wizard.Compile(define, dummy.Compile)
	proof := wizard.Prove(comp, prove)
	err := wizard.Verify(comp, proof)

	if err != nil {
		t.Fatalf("verification failed: %v", err)
	}
}
