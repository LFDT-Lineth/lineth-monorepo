package bls

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
)

func testBlsMsmOnTrace(t *testing.T, g Group, path string, limits *Limits) {
	if _, err := os.Stat(zkevmBin); errors.Is(err, os.ErrNotExist) {
		t.Skipf("skipping arithmetization integration tests, `%s` missing", zkevmBin)
	}
	neededColumns := []string{
		"ID",
		"CIRCUIT_SELECTOR_BLS_" + g.String() + "_MSM",
		"CIRCUIT_SELECTOR_" + g.String() + "_MEMBERSHIP",
		"LIMB",
		"INDEX",
		"CT",
		"DATA_BLS_" + g.String() + "_MSM_FLAG",
		"RSLT_BLS_" + g.String() + "_MSM_FLAG",
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
	var blsMsm *BlsMsm
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
						blsMsm = newMsm(b.CompiledIOP, g, limits, newMsmDataSource(b.CompiledIOP, g))
						blsMsm = blsMsm.
							WithGroupMembershipCircuit(b.CompiledIOP, query.PlonkRangeCheckOption(16, 6, true)).
							WithMsmCircuit(b.CompiledIOP, query.PlonkRangeCheckOption(16, 6, true))
					},
					integrationTestCompiler,
				)
			}
			proof := wizard.Prove(cmp,
				func(run *wizard.ProverRuntime) {
					assignColumns(t, run, cols, lastMaxLen)
					blsMsm.Assign(run)
				})

			if err := wizard.Verify(cmp, proof); err != nil {
				t.Fatal("proof failed", err)
			}
			t.Log("proof succeeded")
		})
	}
}

func TestBlsG1MsmOnTrace(t *testing.T) {
	limits := &Limits{
		NbG1MulInputInstances:        8,
		NbG1MembershipInputInstances: 8,
		LimitG1MsmCalls:              4,
		LimitG1MembershipCalls:       8,
	}
	testBlsMsmOnTrace(t, G1, "testdata/*.lt", limits)
}

func TestBlsG2MsmOnTrace(t *testing.T) {
	limits := &Limits{
		NbG2MulInputInstances:        6,
		NbG2MembershipInputInstances: 6,
		LimitG2MsmCalls:              4,
		LimitG2MembershipCalls:       8,
	}
	testBlsMsmOnTrace(t, G2, "testdata/*.lt", limits)
}
