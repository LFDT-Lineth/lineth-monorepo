//go:build ignore

package bls

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
)

func testBlsMapOnTrace(t *testing.T, g Group, path string, limits *Limits) {
	if _, err := os.Stat(zkevmBin); errors.Is(err, os.ErrNotExist) {
		t.Skipf("skipping arithmetization integration tests, `%s` missing", zkevmBin)
	}
	var mapString string
	if g == G1 {
		mapString = "MAP_FP_TO_G1"
	} else {
		mapString = "MAP_FP2_TO_G2"
	}
	neededColumns := []string{
		"ID",
		"CIRCUIT_SELECTOR_BLS_" + mapString,
		"INDEX",
		"CT",
		"LIMB",
		"DATA_BLS_" + mapString + "_FLAG",
		"RSLT_BLS_" + mapString + "_FLAG",
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
	var blsMap *BlsMap
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
						blsMap = newMap(b.CompiledIOP, g, limits, newMapDataSource(b.CompiledIOP, g))
						blsMap = blsMap.WithMapCircuit(b.CompiledIOP, query.PlonkRangeCheckOption(16, 1, true))
					},
					integrationTestCompiler,
				)
			}
			proof := wizard.Prove(cmp,
				func(run *wizard.ProverRuntime) {
					assignColumns(t, run, cols, lastMaxLen)
					blsMap.Assign(run)
				})

			if err := wizard.Verify(cmp, proof); err != nil {
				t.Fatal("proof failed", err)
			}
			t.Log("proof succeeded")
		})
	}
}

func TestBlsG1MapOnTrace(t *testing.T) {
	limits := &Limits{
		NbG1MapToInputInstances: 16,
		LimitMapFpToG1Calls:     4,
	}
	testBlsMapOnTrace(t, G1, "testdata/*.lt", limits)
}

func TestBlsG2MapOnTrace(t *testing.T) {
	limits := &Limits{
		NbG2MapToInputInstances: 5,
		LimitMapFp2ToG2Calls:    4,
	}
	testBlsMapOnTrace(t, G2, "testdata/*.lt", limits)
}
