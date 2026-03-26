package bls

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/limbs"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils/csvtraces"
)

func testPointEval(t *testing.T, withCircuit bool) {
	limits := &Limits{
		NbPointEvalInputInstances:        1,
		NbPointEvalFailureInputInstances: 1,
		LimitPointEvalCalls:              50,
		LimitPointEvalFailureCalls:       50,
	}
	files, err := filepath.Glob("testdata/bls_pointeval_inputs-[0-9]*.csv")
	if err != nil {
		t.Fatal(err)
	}
	switch len(files) {
	case 0:
		t.Fatal("no csv files found, please run `go generate` to generate the test data")
	case 1:
		t.Log("single CSV file found. For complete testing, generate all test files with `go generate`")
	}
	// we test all files found
	var cmp *wizard.CompiledIOP
	var bp *BlsPointEval
	var pointEvalSource *BlsPointEvalDataSource
	for _, file := range files {
		t.Run(file, func(t *testing.T) {
			f, err := os.Open(file)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			ct, err := csvtraces.NewCsvTrace(f)
			if err != nil {
				t.Fatal("failed to create csv trace", err)
			}
			cmp = wizard.Compile(
				func(b *wizard.Builder) {
					pointEvalSource = &BlsPointEvalDataSource{
						ID:                 ct.GetCommit(b, "ID"),
						CsPointEval:        ct.GetCommit(b, "CIRCUIT_SELECTOR_POINT_EVALUATION"),
						CsPointEvalInvalid: ct.GetCommit(b, "CIRCUIT_SELECTOR_POINT_EVALUATION_FAILURE"),
						Limb:               ct.GetLimbsLe(b, "LIMB", limbs.NbLimbU128).AssertUint128(),
						Index:              ct.GetCommit(b, "INDEX"),
						Counter:            ct.GetCommit(b, "CT"),
						IsData:             ct.GetCommit(b, "DATA_POINT_EVALUATION"),
						IsRes:              ct.GetCommit(b, "RSLT_POINT_EVALUATION"),
					}
					bp = newPointEval(b.CompiledIOP, limits, pointEvalSource)
					if withCircuit {
						bp = bp.WithPointEvalCircuit(b.CompiledIOP, query.PlonkRangeCheckOption(16, 1, true))
						bp = bp.WithPointEvalFailureCircuit(b.CompiledIOP, query.PlonkRangeCheckOption(16, 1, true))
					}
				},
				dummy.Compile,
			)

			proof := wizard.Prove(cmp,
				func(run *wizard.ProverRuntime) {
					ct.Assign(run,
						pointEvalSource.ID,
						pointEvalSource.CsPointEval,
						pointEvalSource.CsPointEvalInvalid,
						pointEvalSource.Limb,
						pointEvalSource.Index,
						pointEvalSource.Counter,
						pointEvalSource.IsData,
						pointEvalSource.IsRes,
					)
					bp.Assign(run)
				})

			if err := wizard.Verify(cmp, proof); err != nil {
				t.Fatal("proof failed", err)
			}
			t.Log("proof succeeded")
		})
	}
}

func TestPointEvalNoCircuit(t *testing.T) {
	testPointEval(t, false)
}

func TestPointEvalWithCircuit(t *testing.T) {
	skipIfRace(t)
	testPointEval(t, true)
}
