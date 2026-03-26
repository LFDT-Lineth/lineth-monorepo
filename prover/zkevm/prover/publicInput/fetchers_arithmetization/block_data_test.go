package fetchers_arithmetization

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	arith "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/arith_struct"
	util "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/utilities"
	"github.com/stretchr/testify/assert"
)

// TestBlockDataFetcher tests the fetching of the timestamp data
func TestBlockDataFetcher(t *testing.T) {

	// initialize sample block data from a mock test data CSV file
	ctBlockData := util.InitializeCsv("../testdata/blockdata_mock.csv", t)
	var (
		bdc     *arith.BlockDataCols
		fetcher *BlockDataFetcher
	)

	cmp := wizard.Compile(func(b *wizard.Builder) {
		// register sample arithmetization columns
		bdc, _, _ = arith.DefineTestingArithModules(b, ctBlockData, nil, nil)
		// create a new timestamp fetcher
		fetcher = NewBlockDataFetcher(b.CompiledIOP, "TIMESTAMP_FETCHER_FROM_ARITH", bdc)
		// constrain the timestamp fetcher
		DefineBlockDataFetcher(b.CompiledIOP, fetcher, "TIMESTAMP_FETCHER_FROM_ARITH", bdc)
	}, dummy.Compile)
	proof := wizard.Prove(cmp, func(run *wizard.ProverRuntime) {
		// assign the CSV columns
		arith.AssignTestingArithModules(run, ctBlockData, nil, nil, bdc, nil, nil)
		// assign the timestamp fetcher
		AssignBlockDataFetcher(run, fetcher, bdc)
		// two simple sanity checks based on the mock test data
		nbLimbs := len(fetcher.FirstTimestamp)
		assert.Equal(t, fetcher.FirstTimestamp[nbLimbs-1].GetColAssignmentAt(run, 0), field.NewElement(0xa))
		assert.Equal(t, fetcher.LastTimestamp[nbLimbs-1].GetColAssignmentAt(run, 0), field.NewElement(0xcd))
		for i := range nbLimbs - 1 {
			assert.Equal(t, fetcher.FirstTimestamp[i].GetColAssignmentAt(run, 0), field.Zero())
			assert.Equal(t, fetcher.LastTimestamp[i].GetColAssignmentAt(run, 0), field.Zero())
		}
	})
	if err := wizard.Verify(cmp, proof); err != nil {
		t.Fatal("proof failed", err)
	}
	t.Log("proof succeeded")
}
