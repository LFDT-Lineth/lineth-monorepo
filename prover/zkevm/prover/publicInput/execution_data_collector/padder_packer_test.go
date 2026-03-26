package execution_data_collector

import (
	"fmt"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	arith "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/arith_struct"
	fetch "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/fetchers_arithmetization"
	util "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/utilities"
	"testing"
)

func TestDefineAndAssignmentPadderPacker(t *testing.T) {
	testCaseBytes := [][]string{
		{"0x00001900", "0x00000000", "0x00000000", "0x00000000", "0x00000000", "0x00000000", "0x00000000", "0x00000000"},
		{"0x00001234", "0x00005678", "0x00003333", "0x00004567", "0x00004444", "0x00003456", "0x00007891", "0x00002345"},
		{"0x00001111", "0x00005432", "0x00001987", "0x00006543", "0x00002198", "0x00000000", "0x00000000", "0x00000000"},
		{"0x00001929", "0x00003949", "0x00005969", "0x00007989", "0x00001213", "0x00001415", "0x00000000", "0x00000000"},
	}

	testCaseNBytes := []int{
		1, 16, 10, 12,
	}

	t.Run(fmt.Sprintf("testcase"), func(t *testing.T) {

		size := 4
		inputNoBytes := make([]field.Element, size)
		inputIsActive := make([]field.Element, size)
		inputLimbs := make([][]field.Element, common.NbLimbU128)
		for j := 0; j < common.NbLimbU128; j++ {
			inputLimbs[j] = make([]field.Element, size)
		}

		for index := 0; index < size; index++ {
			if testCaseNBytes[index] > 0 {
				inputNoBytes[index].SetUint64(uint64(testCaseNBytes[index]))
				inputIsActive[index].SetOne()
				for j := 0; j < common.NbLimbU128; j++ {
					inputLimbs[j][index] = field.NewFromString(testCaseBytes[index][j])
				}
			}

		}

		testLimbs := make([]ifaces.Column, 8)
		var (
			testNoBytes, testIsActive ifaces.Column
			ppp                       PadderPacker
		)

		define := func(b *wizard.Builder) {
			for j := 0; j < common.NbLimbU128; j++ {
				testLimbs[j] = util.CreateCol("TEST_PADDER", fmt.Sprintf("PACKER_LIMBS_%d", j), size, b.CompiledIOP)
			}
			testNoBytes = util.CreateCol("TEST_PADDER_PACKER", "NO_BYTES", size, b.CompiledIOP)
			testIsActive = util.CreateCol("TEST_PADDER_PACKER", "IS_ACTIVE", size, b.CompiledIOP)
			ppp = NewPadderPacker(b.CompiledIOP, [8]ifaces.Column(testLimbs), testNoBytes, testIsActive, "TEST_PADDER_PACKER")
			DefinePadderPacker(b.CompiledIOP, &ppp, "TEST_PADDER_PACKER")
		}

		prove := func(run *wizard.ProverRuntime) {
			for j := 0; j < common.NbLimbU128; j++ {
				run.AssignColumn(testLimbs[j].GetColID(), smartvectors.NewRegular(inputLimbs[j]))
			}
			run.AssignColumn(testNoBytes.GetColID(), smartvectors.NewRegular(inputNoBytes))
			run.AssignColumn(testIsActive.GetColID(), smartvectors.NewRegular(inputIsActive))
			AssignPadderPacker(run, ppp)

		}

		comp := wizard.Compile(define, dummy.Compile)
		proof := wizard.Prove(comp, prove)
		err := wizard.Verify(comp, proof)

		if err != nil {
			t.Fatalf("verification failed: %v", err)
		}
	})

}

func TestPadderPackerOnExecutionDataCollector(t *testing.T) {
	ctBlockData := util.InitializeCsv("../testdata/blockdata_mock.csv", t)
	ctTxnData := util.InitializeCsv("../testdata/txndata_mock.csv", t)
	ctRlpTxn := util.InitializeCsv("../testdata/rlp_txn_mock.csv", t)
	blockHashList := [1 << 10]types.FullBytes32{}

	var (
		edc              *ExecutionDataCollector
		btm              fetch.BlockTxnMetadata
		blockDataFetcher *fetch.BlockDataFetcher
		txnDataFetcher   fetch.TxnDataFetcher
		rlpTxnFetcher    fetch.RlpTxnFetcher
		txd              *arith.TxnData
		bdc              *arith.BlockDataCols
		rt               *arith.RlpTxn
		chainIDFetcher   fetch.ChainIDFetcher
		ppp              PadderPacker
	)

	define := func(b *wizard.Builder) {
		// define the arith test modules
		bdc, txd, rt = arith.DefineTestingArithModules(b, ctBlockData, ctTxnData, ctRlpTxn)
		// create and define a metadata fetcher
		btm = fetch.NewBlockTxnMetadata(b.CompiledIOP, "BLOCK_TX_METADATA", txd)
		fetch.DefineBlockTxnMetaData(b.CompiledIOP, &btm, "BLOCK_TX_METADATA", txd)
		// create a new timestamp fetcher
		blockDataFetcher = fetch.NewBlockDataFetcher(b.CompiledIOP, "TIMESTAMP_FETCHER_FROM_ARITH", bdc)
		// constrain the timestamp fetcher
		fetch.DefineBlockDataFetcher(b.CompiledIOP, blockDataFetcher, "TIMESTAMP_FETCHER_FROM_ARITH", bdc)
		txnDataFetcher = fetch.NewTxnDataFetcher(b.CompiledIOP, "TXN_DATA_FETCHER_FROM_ARITH", txd)
		fetch.DefineTxnDataFetcher(b.CompiledIOP, &txnDataFetcher, "TXN_DATA_FETCHER_FROM_ARITH", txd)

		rlpTxnFetcher = fetch.NewRlpTxnFetcher(b.CompiledIOP, "RLP_TXN_FETCHER_FROM_ARITH", rt)
		// constrain the fetcher
		fetch.DefineRlpTxnFetcher(b.CompiledIOP, &rlpTxnFetcher, "RLP_TXN_FETCHER_FROM_ARITH", rt)

		// ChainIDFetcher
		chainIDFetcher = fetch.NewChainIDFetcher(b.CompiledIOP, "PUBLIC_INPUT_CHAIN_ID_FETCHER", bdc)
		fetch.DefineChainIDFetcher(b.CompiledIOP, &chainIDFetcher, "PUBLIC_INPUT_CHAIN_ID_FETCHER", bdc)

		limbColSize := GetSummarySize(txd, rt)
		edc = NewExecutionDataCollector(b.CompiledIOP, "EXECUTION_DATA_COLLECTOR", limbColSize)
		DefineExecutionDataCollector(b.CompiledIOP, edc, "EXECUTION_DATA_COLLECTOR", blockDataFetcher, btm, txnDataFetcher, rlpTxnFetcher)

		ppp = NewPadderPacker(b.CompiledIOP, edc.Limbs, edc.NoBytes, edc.IsActive, "TEST_PADDER_PACKER")
		DefinePadderPacker(b.CompiledIOP, &ppp, "TEST_PADDER_PACKER")
	}

	prove := func(run *wizard.ProverRuntime) {
		arith.AssignTestingArithModules(run, ctBlockData, ctTxnData, ctRlpTxn, bdc, txd, rt)
		fetch.AssignBlockDataFetcher(run, blockDataFetcher, bdc)
		fetch.AssignBlockTxnMetadata(run, btm, txd)
		fetch.AssignTxnDataFetcher(run, txnDataFetcher, txd)
		fetch.AssignRlpTxnFetcher(run, &rlpTxnFetcher, rt)
		fetch.AssignChainIDFetcher(run, &chainIDFetcher, bdc)
		AssignExecutionDataCollector(run, edc, blockDataFetcher, btm, txnDataFetcher, rlpTxnFetcher, blockHashList[:])
		AssignPadderPacker(run, ppp)
	}

	comp := wizard.Compile(define, dummy.Compile)
	proof := wizard.Prove(comp, prove)
	err := wizard.Verify(comp, proof)

	if err != nil {
		t.Fatalf("verification failed: %v", err)
	}
}
