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

func testBlsAdd(t *testing.T, withCircuit bool, g Group, path string, limits *Limits) {
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
	var blsAdd *BlsAdd
	var blsAddSource *BlsAddDataSource
	cmp := wizard.Compile(
		func(b *wizard.Builder) {
			blsAddSource = &BlsAddDataSource{
				CsAdd:             ct.GetCommit(b, "CIRCUIT_SELECTOR_"+g.String()+"_ADD"),
				Limb:              ct.GetLimbsLe(b, "LIMB", limbs.NbLimbU128).AssertUint128(),
				Index:             ct.GetCommit(b, "INDEX"),
				Counter:           ct.GetCommit(b, "CT"),
				CsCurveMembership: ct.GetCommit(b, "CIRCUIT_SELECTOR_"+g.StringCurve()+"_MEMBERSHIP"),
				IsData:            ct.GetCommit(b, "DATA_"+g.String()+"_ADD"),
				IsRes:             ct.GetCommit(b, "RSLT_"+g.String()+"_ADD"),
			}
			blsAdd = newAdd(b.CompiledIOP, g, limits, blsAddSource)
			if withCircuit {
				blsAdd = blsAdd.
					WithAddCircuit(b.CompiledIOP, query.PlonkRangeCheckOption(16, 2, true)).
					WithCurveMembershipCircuit(b.CompiledIOP, query.PlonkRangeCheckOption(16, 2, true))
			}
		},
		dummy.Compile,
	)

	proof := wizard.Prove(cmp,
		func(run *wizard.ProverRuntime) {
			ct.Assign(run,
				blsAdd.BlsAddDataSource.CsAdd,
				blsAdd.BlsAddDataSource.Limb,
				blsAdd.BlsAddDataSource.Index,
				blsAdd.BlsAddDataSource.Counter,
				blsAdd.BlsAddDataSource.CsCurveMembership,
				blsAdd.BlsAddDataSource.IsData,
				blsAdd.BlsAddDataSource.IsRes,
			)
			blsAdd.Assign(run)
		})

	if err := wizard.Verify(cmp, proof); err != nil {
		t.Fatal("proof failed", err)
	}
	t.Log("proof succeeded")
}

func TestBlsG1AddNoCircuit(t *testing.T) {
	limits := &Limits{
		NbG1AddInputInstances:        16,
		NbC1MembershipInputInstances: 16,
		LimitG1AddCalls:              32,
		LimitC1MembershipCalls:       32,
	}
	testBlsAdd(t, false, G1, "testdata/bls_g1_add_inputs.csv", limits)
}

func TestBlsG1AddWithCircuit(t *testing.T) {
	limits := &Limits{
		NbG1AddInputInstances:        16,
		NbC1MembershipInputInstances: 16,
		LimitG1AddCalls:              32,
		LimitC1MembershipCalls:       32,
	}
	testBlsAdd(t, true, G1, "testdata/bls_g1_add_inputs.csv", limits)
}

func TestBlsG2AddNoCircuit(t *testing.T) {
	limits := &Limits{
		NbG2AddInputInstances:        16,
		NbC2MembershipInputInstances: 16,
		LimitG2AddCalls:              32,
		LimitC2MembershipCalls:       32,
	}
	testBlsAdd(t, false, G2, "testdata/bls_g2_add_inputs.csv", limits)
}

func TestBlsG2AddWithCircuit(t *testing.T) {
	limits := &Limits{
		NbG2AddInputInstances:        16,
		NbC2MembershipInputInstances: 16,
		LimitG2AddCalls:              32,
		LimitC2MembershipCalls:       32,
	}
	testBlsAdd(t, true, G2, "testdata/bls_g2_add_inputs.csv", limits)
}
