package fetchers_arithmetization

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	arith "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/arith_struct"
	util "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/utilities"
)

// TestTxnDataFetcher tests the fetching of the sender address data
func TestTxnDataFetcher(t *testing.T) {

	// initialize sample TxnData from a mock test data CSV file
	ctTxnData := util.InitializeCsv("../testdata/txndata_mock.csv", t)
	var (
		txd     *arith.TxnData
		fetcher TxnDataFetcher
	)

	cmp := wizard.Compile(func(b *wizard.Builder) {
		// register sample arithmetization columns
		_, txd, _ = arith.DefineTestingArithModules(b, nil, ctTxnData, nil)
		// create a new txn data fetcher
		fetcher = NewTxnDataFetcher(b.CompiledIOP, "TXN_DATA_FETCHER_FROM_ARITH", txd)
		// constrain the fetcher
		DefineTxnDataFetcher(b.CompiledIOP, &fetcher, "TXN_DATA_FETCHER_FROM_ARITH", txd)
	}, dummy.Compile)
	proof := wizard.Prove(cmp, func(run *wizard.ProverRuntime) {
		// assign the CSV columns
		arith.AssignTestingArithModules(run, nil, ctTxnData, nil, nil, txd, nil)
		// assign the txn data fetcher
		AssignTxnDataFetcher(run, fetcher, txd)
	})
	if err := wizard.Verify(cmp, proof); err != nil {
		t.Fatal("proof failed", err)
	}
	t.Log("proof succeeded")
}
