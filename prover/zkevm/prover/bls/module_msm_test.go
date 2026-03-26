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

func testBlsMsm(t *testing.T, withCircuit bool, g Group, path string, limits *Limits) {
	files, err := filepath.Glob(path)
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
	var blsMsm *BlsMsm
	var blsMsmSource *BlsMsmDataSource
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
					blsMsmSource = &BlsMsmDataSource{
						ID:           ct.GetCommit(b, "ID"),
						CsMul:        ct.GetCommit(b, "CIRCUIT_SELECTOR_"+g.String()+"_MSM"),
						CsMembership: ct.GetCommit(b, "CIRCUIT_SELECTOR_"+g.String()+"_MEMBERSHIP"),
						Limb:         ct.GetLimbsLe(b, "LIMB", limbs.NbLimbU128).AssertUint128(),
						Index:        ct.GetCommit(b, "INDEX"),
						Counter:      ct.GetCommit(b, "CT"),
						IsData:       ct.GetCommit(b, "DATA_"+g.String()+"_MSM"),
						IsRes:        ct.GetCommit(b, "RSLT_"+g.String()+"_MSM"),
					}
					blsMsm = newMsm(b.CompiledIOP, g, limits, blsMsmSource)
					if withCircuit {
						blsMsm = blsMsm.
							WithGroupMembershipCircuit(b.CompiledIOP, query.PlonkRangeCheckOption(16, 1, true)).
							WithMsmCircuit(b.CompiledIOP, query.PlonkRangeCheckOption(16, 1, true))
					}
				},
				dummy.Compile,
			)

			proof := wizard.Prove(cmp,
				func(run *wizard.ProverRuntime) {
					ct.Assign(run,
						blsMsmSource.ID,
						blsMsmSource.CsMul,
						blsMsmSource.CsMembership,
						blsMsmSource.Limb,
						blsMsmSource.Index,
						blsMsmSource.Counter,
						blsMsmSource.IsData,
						blsMsmSource.IsRes,
					)
					blsMsm.Assign(run)
				})
			if err := wizard.Verify(cmp, proof); err != nil {
				t.Fatal("proof failed", err)
			}
			t.Log("proof succeeded")
		})
	}
}

func TestBlsG1MsmNoCircuit(t *testing.T) {
	limits := &Limits{
		NbG1MulInputInstances:        3,
		NbG1MembershipInputInstances: 6,
		LimitG1MsmCalls:              128,
		LimitG1MembershipCalls:       128,
	}
	testBlsMsm(t, false, G1, "testdata/bls_g1_msm_inputs-[0-9]*.csv", limits)
}

func TestBlsG1MsmWithCircuit(t *testing.T) {
	limits := &Limits{
		NbG1MulInputInstances:        3,
		NbG1MembershipInputInstances: 6,
		LimitG1MsmCalls:              128,
		LimitG1MembershipCalls:       128,
	}
	testBlsMsm(t, true, G1, "testdata/bls_g1_msm_inputs-[0-9]*.csv", limits)
}

func TestBlsG2MsmNoCircuit(t *testing.T) {
	limits := &Limits{
		NbG2MulInputInstances:        2,
		NbG2MembershipInputInstances: 6,
		LimitG2MsmCalls:              128,
		LimitG2MembershipCalls:       128,
	}
	testBlsMsm(t, false, G2, "testdata/bls_g2_msm_inputs-[0-9]*.csv", limits)
}

func TestBlsG2MsmWithCircuit(t *testing.T) {
	limits := &Limits{
		NbG2MulInputInstances:        2,
		NbG2MembershipInputInstances: 6,
		LimitG2MsmCalls:              128,
		LimitG2MembershipCalls:       128,
	}
	testBlsMsm(t, true, G2, "testdata/bls_g2_msm_inputs-[0-9]*.csv", limits)
}
