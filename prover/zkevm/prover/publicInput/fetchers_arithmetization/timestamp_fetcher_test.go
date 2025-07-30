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

// TestTimestampFetcher tests the fetching of the timestamp data
func TestTimestampFetcher(t *testing.T) {

	// initialize sample block data from a mock test data CSV file
	ctBlockData := util.InitializeCsv("../testdata/blockdata_mock.csv", t)
	var (
		bdc     *arith.BlockDataCols
		fetcher *TimestampFetcher
	)

	cmp := wizard.Compile(func(b *wizard.Builder) {
		// register sample arithmetization columns
		bdc, _, _ = arith.DefineTestingArithModules(b, ctBlockData, nil, nil)
		// create a new timestamp fetcher
		fetcher = NewTimestampFetcher(b.CompiledIOP, "TIMESTAMP_FETCHER_FROM_ARITH", bdc)
		// constrain the timestamp fetcher
		DefineTimestampFetcher(b.CompiledIOP, fetcher, "TIMESTAMP_FETCHER_FROM_ARITH", bdc)
	}, dummy.Compile)
	proof := wizard.Prove(cmp, func(run *wizard.ProverRuntime) {
		// assign the CSV columns
		arith.AssignTestingArithModules(run, ctBlockData, nil, nil)
		// assign the timestamp fetcher
		AssignTimestampFetcher(run, fetcher, bdc)
		// two simple sanity checks based on the mock test data
		assert.Equal(t, fetcher.First.GetColAssignmentAt(run, 0), field.NewElement(0xa))
		assert.Equal(t, fetcher.Last.GetColAssignmentAt(run, 0), field.NewElement(0xcd))
	})
	if err := wizard.Verify(cmp, proof); err != nil {
		t.Fatal("proof failed", err)
	}
	t.Log("proof succeeded")
}
