package bls

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
)

func TestBlsPairOnTrace(t *testing.T) {
	const path = "testdata/*.lt"
	if _, err := os.Stat(zkevmBin); errors.Is(err, os.ErrNotExist) {
		t.Skipf("skipping arithmetization integration tests, `%s` missing", zkevmBin)
	}
	neededColumns := []string{
		"ID",
		"CIRCUIT_SELECTOR_BLS_PAIRING_CHECK",
		"CIRCUIT_SELECTOR_G1_MEMBERSHIP",
		"CIRCUIT_SELECTOR_G2_MEMBERSHIP",
		"LIMB",
		"INDEX",
		"CT",
		"DATA_BLS_PAIRING_CHECK_FLAG",
		"RSLT_BLS_PAIRING_CHECK_FLAG",
		"SUCCESS_BIT",
	}
	limits := &Limits{
		NbMillerLoopInputInstances:   4,
		NbFinalExpInputInstances:     4,
		NbG1MembershipInputInstances: 8,
		NbG2MembershipInputInstances: 6,
		LimitMillerLoopCalls:         8,
		LimitFinalExpCalls:           2,
		LimitG1MembershipCalls:       8,
		LimitG2MembershipCalls:       8,
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
	var blsPair *BlsPair
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
						blsPair = newPair(b.CompiledIOP, limits, newPairDataSource(b.CompiledIOP))
						blsPair = blsPair.
							WithG1MembershipCircuit(b.CompiledIOP, query.PlonkRangeCheckOption(16, 6, true)).
							WithG2MembershipCircuit(b.CompiledIOP, query.PlonkRangeCheckOption(16, 6, true)).
							WithPairingCircuit(b.CompiledIOP, query.PlonkRangeCheckOption(16, 6, true))

					},
					integrationTestCompiler,
				)
			}
			proof := wizard.Prove(cmp,
				func(run *wizard.ProverRuntime) {
					assignColumns(t, run, cols, lastMaxLen)
					blsPair.Assign(run)
				})

			if err := wizard.Verify(cmp, proof); err != nil {
				t.Fatal("proof failed", err)
			}
			t.Log("proof succeeded")
		})
	}
}
