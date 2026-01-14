package bls

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils/csvtraces"
)

func testBlsPair(t *testing.T, withCircuit bool) {
	limits := &Limits{
		NbMillerLoopInputInstances:   4,
		NbFinalExpInputInstances:     4,
		NbG1MembershipInputInstances: 4,
		NbG2MembershipInputInstances: 4,
		LimitMillerLoopCalls:         32,
		LimitFinalExpCalls:           32,
		LimitG1MembershipCalls:       64,
		LimitG2MembershipCalls:       64,
	}
	files, err := filepath.Glob("testdata/bls_pairing_inputs-[0-9]*.csv")
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
	var blsPair *BlsPair
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
			if cmp == nil {
				cmp = wizard.Compile(
					func(b *wizard.Builder) {
						blsPairSource := &BlsPairDataSource{
							ID:             ct.GetCommit(b, "ID"),
							CsPair:         ct.GetCommit(b, "CS_PAIRING_CHECK"),
							CsG1Membership: ct.GetCommit(b, "CS_G1_MEMBERSHIP"),
							CsG2Membership: ct.GetCommit(b, "CS_G2_MEMBERSHIP"),
							Limb:           ct.GetCommit(b, "LIMB"),
							Index:          ct.GetCommit(b, "INDEX"),
							Counter:        ct.GetCommit(b, "CT"),
							IsData:         ct.GetCommit(b, "DATA_PAIRING_CHECK"),
							IsRes:          ct.GetCommit(b, "RSLT_PAIRING_CHECK"),
							SuccessBit:     ct.GetCommit(b, "SUCCESS_BIT"),
						}
						blsPair = newPair(b.CompiledIOP, limits, blsPairSource)
						if withCircuit {
							blsPair = blsPair.
								WithG1MembershipCircuit(b.CompiledIOP, query.PlonkRangeCheckOption(16, 6, true)).
								WithG2MembershipCircuit(b.CompiledIOP, query.PlonkRangeCheckOption(16, 6, true)).
								WithPairingCircuit(b.CompiledIOP, query.PlonkRangeCheckOption(16, 6, true))
						}
					},
					dummy.Compile,
				)
			}
			proof := wizard.Prove(cmp,
				func(run *wizard.ProverRuntime) {
					ct.Assign(run, "ID", "CS_PAIRING_CHECK", "CS_G1_MEMBERSHIP", "CS_G2_MEMBERSHIP", "LIMB", "INDEX", "CT", "DATA_PAIRING_CHECK", "RSLT_PAIRING_CHECK", "SUCCESS_BIT")
					blsPair.Assign(run)
				})
			if err := wizard.Verify(cmp, proof); err != nil {
				t.Fatal("proof failed", err)
			}
			t.Log("proof succeeded")
		})
	}
}

func TestBlsPairNoCircuit(t *testing.T) {
	testBlsPair(t, false)
}

func TestBlsPairWithCircuit(t *testing.T) {
	testBlsPair(t, true)
}
