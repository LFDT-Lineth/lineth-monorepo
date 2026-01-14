package bls

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
)

func testBlsAddOnTrace(t *testing.T, g Group, path string, limits *Limits) {
	if _, err := os.Stat(zkevmBin); errors.Is(err, os.ErrNotExist) {
		t.Skipf("skipping arithmetization integration tests, `%s` missing", zkevmBin)
	}
	neededColumns := []string{
		"ID",
		"CIRCUIT_SELECTOR_BLS_" + g.String() + "_ADD",
		"LIMB",
		"INDEX",
		"CT",
		"CIRCUIT_SELECTOR_" + g.StringCurve() + "_MEMBERSHIP",
		"DATA_BLS_" + g.String() + "_ADD_FLAG",
		"RSLT_BLS_" + g.String() + "_ADD_FLAG",
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
	var blsAdd *BlsAdd
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
						blsAdd = newAdd(b.CompiledIOP, g, limits, newAddDataSource(b.CompiledIOP, g))
						blsAdd = blsAdd.
							WithAddCircuit(b.CompiledIOP, query.PlonkRangeCheckOption(16, 6, true)).
							WithCurveMembershipCircuit(b.CompiledIOP, query.PlonkRangeCheckOption(16, 6, true))
					},
					integrationTestCompiler,
				)
			}
			proof := wizard.Prove(cmp,
				func(run *wizard.ProverRuntime) {
					assignColumns(t, run, cols, lastMaxLen)
					blsAdd.Assign(run)
				})

			if err := wizard.Verify(cmp, proof); err != nil {
				t.Fatal("proof failed", err)
			}
			t.Log("proof succeeded")
		})
	}
}

func TestBlsG1AddOnTrace(t *testing.T) {
	limits := &Limits{
		NbG1AddInputInstances:        16,
		NbC1MembershipInputInstances: 16,
		LimitG1AddCalls:              8,
		LimitC1MembershipCalls:       8,
	}
	testBlsAddOnTrace(t, G1, "testdata/*.lt", limits)
}

func TestBlsG2AddOnTrace(t *testing.T) {
	limits := &Limits{
		NbG2AddInputInstances:        16,
		NbC2MembershipInputInstances: 16,
		LimitG2AddCalls:              8,
		LimitC2MembershipCalls:       8,
	}
	testBlsAddOnTrace(t, G2, "testdata/*.lt", limits)
}
