package bls

import (
	"errors"
	"os"
	"testing"

	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/limbs"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils/csvtraces"
)

func testBlsMap(t *testing.T, withCircuit bool, g Group, path string, limits *Limits) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		t.Fatal("csv file does not exist, please run `go generate` to generate the test data")
	}
	if err != nil {
		t.Fatal("failed to open csv file", err)
	}
	defer f.Close()
	ct, err := csvtraces.NewCsvTrace(f)
	if err != nil {
		t.Fatal("failed to create csv trace", err)
	}
	var mapString string
	if g == G1 {
		mapString = "MAP_FP_TO_G1"
	} else {
		mapString = "MAP_FP2_TO_G2"
	}
	var blsMap *BlsMap
	var blsMapSource *BlsMapDataSource
	cmp := wizard.Compile(
		func(b *wizard.Builder) {
			blsMapSource = &BlsMapDataSource{
				ID:      ct.GetCommit(b, "ID"),
				CsMap:   ct.GetCommit(b, "CIRCUIT_SELECTOR_"+mapString),
				Index:   ct.GetCommit(b, "INDEX"),
				Counter: ct.GetCommit(b, "CT"),
				Limb:    ct.GetLimbsLe(b, "LIMB", limbs.NbLimbU128).AssertUint128(),
				IsData:  ct.GetCommit(b, "DATA_"+mapString),
				IsRes:   ct.GetCommit(b, "RSLT_"+mapString),
			}
			blsMap = newMap(b.CompiledIOP, g, limits, blsMapSource)
			if withCircuit {
				blsMap = blsMap.WithMapCircuit(b.CompiledIOP, query.PlonkRangeCheckOption(16, 1, true))
			}
		},
		dummy.Compile,
	)

	proof := wizard.Prove(cmp,
		func(run *wizard.ProverRuntime) {
			ct.Assign(run,
				blsMapSource.ID,
				blsMapSource.CsMap,
				blsMapSource.Index,
				blsMapSource.Counter,
				blsMapSource.Limb,
				blsMapSource.IsData,
				blsMapSource.IsRes,
			)
			blsMap.Assign(run)
		})

	if err := wizard.Verify(cmp, proof); err != nil {
		t.Fatal("proof failed", err)
	}
	t.Log("proof succeeded")
}

func TestBlsMapG1NoCircuit(t *testing.T) {
	limits := &Limits{
		NbG1MapToInputInstances: 8,
		LimitMapFpToG1Calls:     32,
	}
	testBlsMap(t, false, G1, "testdata/bls_g1_map_inputs.csv", limits)
}

func TestBlsMapG1WithCircuit(t *testing.T) {
	limits := &Limits{
		NbG1MapToInputInstances: 8,
		LimitMapFpToG1Calls:     32,
	}
	testBlsMap(t, true, G1, "testdata/bls_g1_map_inputs.csv", limits)
}

func TestBlsMapG2NoCircuit(t *testing.T) {
	limits := &Limits{
		NbG2MapToInputInstances: 2,
		LimitMapFp2ToG2Calls:    10,
	}
	testBlsMap(t, false, G2, "testdata/bls_g2_map_inputs.csv", limits)
}

func TestBlsMapG2WithCircuit(t *testing.T) {
	limits := &Limits{
		NbG2MapToInputInstances: 2,
		LimitMapFp2ToG2Calls:    10,
	}
	testBlsMap(t, true, G2, "testdata/bls_g2_map_inputs.csv", limits)
}
