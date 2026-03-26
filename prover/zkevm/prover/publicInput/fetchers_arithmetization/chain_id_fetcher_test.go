package fetchers_arithmetization

import (
	"math/big"
	"testing"

	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/limbs"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	arith "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/arith_struct"
	util "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/utilities"
	"github.com/stretchr/testify/assert"
)

// TestChainIDFetcher tests the fetching of the timestamp data
func TestChainIDFetcher(t *testing.T) {

	// initialize sample block data from a mock test data CSV file
	ctBlockData := util.InitializeCsv("../testdata/blockdata_mock.csv", t)
	var (
		bdc     *arith.BlockDataCols
		fetcher ChainIDFetcher
	)

	cmp := wizard.Compile(func(b *wizard.Builder) {
		// register sample arithmetization columns
		bdc, _, _ = arith.DefineTestingArithModules(b, ctBlockData, nil, nil)
		// create a new timestamp fetcher
		fetcher = NewChainIDFetcher(b.CompiledIOP, "CHAIN_ID_FETCHER_FROM_ARITH", bdc)
		// constrain the timestamp fetcher
		DefineChainIDFetcher(b.CompiledIOP, &fetcher, "CHAIN_ID_FETCHER_FROM_ARITH", bdc)
	}, dummy.Compile)
	proof := wizard.Prove(cmp, func(run *wizard.ProverRuntime) {
		// assign the CSV columns
		arith.AssignTestingArithModules(run, ctBlockData, nil, nil, bdc, nil, nil)
		// assign the timestamp fetcher
		AssignChainIDFetcher(run, &fetcher, bdc)
		// two simple sanity checks based on the mock test data
		chainID := limbs.NewLimbsFromRawUnsafe[limbs.BigEndian]("CHAIN_ID", fetcher.ChainID[:]).GetRowAsBigInt(run, 0)
		expectedChainID, ok := new(big.Int).SetString("fefc", 16)
		if !ok {
			t.Fatalf("error converting string to big.Int")
		}

		// This removes the padding zeroes
		chainID.Rsh(chainID, 112)

		assert.Equal(t, chainID.Text(16), expectedChainID.Text(16), "ChainID value is incorrect")
	})
	if err := wizard.Verify(cmp, proof); err != nil {
		t.Fatal("proof failed", err)
	}
	t.Log("proof succeeded")
}
