package bls

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
)

func TestPointEvalOnTrace(t *testing.T) {
	const path = "testdata/*.lt"
	if _, err := os.Stat(zkevmBin); errors.Is(err, os.ErrNotExist) {
		t.Skipf("skipping arithmetization integration tests, `%s` missing", zkevmBin)
	}
	neededColumns := []string{
		"ID",
		"CIRCUIT_SELECTOR_POINT_EVALUATION",
		"CIRCUIT_SELECTOR_POINT_EVALUATION_FAILURE",
		"LIMB",
		"INDEX",
		"CT",
		"DATA_POINT_EVALUATION_FLAG",
		"RSLT_POINT_EVALUATION_FLAG",
	}
	limits := &Limits{
		NbPointEvalInputInstances:        2,
		NbPointEvalFailureInputInstances: 2,
		LimitPointEvalCalls:              1,
		LimitPointEvalFailureCalls:       2,
	}
	files, err := filepath.Glob(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Skipf("no trace files found matching regexp \"%s\", skipping trace-based tests", path)
	}
	zkbinf, zkSchema, mapping := parseZkEvmBin(t, zkevmBin)
	var cmp *wizard.CompiledIOP
	var bp *BlsPointEval
	lastMaxLen := uint(0)
	for _, file := range files {
		t.Run(file, func(t *testing.T) {
			expandedTrace := parseExpandedTrace(t, file, zkbinf, zkSchema, mapping)
			cols, maxLen := parseColumns(t, expandedTrace, neededColumns)
			if cmp == nil || lastMaxLen < maxLen {
				lastMaxLen = maxLen
				cmp = wizard.Compile(
					func(b *wizard.Builder) {
						registerColumns(t, b, cols, maxLen)
						bp = newPointEval(b.CompiledIOP, limits, newPointEvalDataSource(b.CompiledIOP))
						bp = bp.WithPointEvalCircuit(b.CompiledIOP, query.PlonkRangeCheckOption(16, 6, true))
						bp = bp.WithPointEvalFailureCircuit(b.CompiledIOP, query.PlonkRangeCheckOption(16, 6, true))
					},
					integrationTestCompiler,
				)
			}
			proof := wizard.Prove(cmp,
				func(run *wizard.ProverRuntime) {
					assignColumns(t, run, cols, lastMaxLen)
					bp.Assign(run)
				})

			if err := wizard.Verify(cmp, proof); err != nil {
				t.Fatal("proof failed", err)
			}
			t.Log("proof succeeded")
		})
	}
}
