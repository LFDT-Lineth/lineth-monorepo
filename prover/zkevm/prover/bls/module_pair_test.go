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

func testBlsPair(t *testing.T, withCircuit bool) {
	limits := &Limits{
		NbMillerLoopInputInstances:   1,
		NbFinalExpInputInstances:     1,
		NbG1MembershipInputInstances: 1,
		NbG2MembershipInputInstances: 1,
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
	var blsPairSource *BlsPairDataSource
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
					blsPairSource = &BlsPairDataSource{
						ID:             ct.GetCommit(b, "ID"),
						CsPair:         ct.GetCommit(b, "CS_PAIRING_CHECK"),
						CsG1Membership: ct.GetCommit(b, "CS_G1_MEMBERSHIP"),
						CsG2Membership: ct.GetCommit(b, "CS_G2_MEMBERSHIP"),
						Limb:           ct.GetLimbsLe(b, "LIMB", limbs.NbLimbU128).AssertUint128(),
						Index:          ct.GetCommit(b, "INDEX"),
						Counter:        ct.GetCommit(b, "CT"),
						IsData:         ct.GetCommit(b, "DATA_PAIRING_CHECK"),
						IsRes:          ct.GetCommit(b, "RSLT_PAIRING_CHECK"),
						SuccessBit:     ct.GetCommit(b, "SUCCESS_BIT"),
					}
					blsPair = newPair(b.CompiledIOP, limits, blsPairSource)
					if withCircuit {
						blsPair = blsPair.
							WithG1MembershipCircuit(b.CompiledIOP, query.PlonkRangeCheckOption(16, 1, true)).
							WithG2MembershipCircuit(b.CompiledIOP, query.PlonkRangeCheckOption(16, 1, true)).
							WithPairingCircuit(b.CompiledIOP, query.PlonkRangeCheckOption(16, 1, true))
					}
				},
				dummy.Compile,
			)
			proof := wizard.Prove(cmp,
				func(run *wizard.ProverRuntime) {
					ct.Assign(run,
						blsPairSource.ID,
						blsPairSource.CsPair,
						blsPairSource.CsG1Membership,
						blsPairSource.CsG2Membership,
						blsPairSource.Limb,
						blsPairSource.Index,
						blsPairSource.Counter,
						blsPairSource.IsData,
						blsPairSource.IsRes,
						blsPairSource.SuccessBit,
					)
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
