package p256verify

import (
	"errors"
	"os"
	"testing"

	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils/csvtraces"
)

func testP256Verify(t *testing.T, withCircuit bool, path string, limits *Limits) {
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
	var p256Verify *P256Verify
	cmp := wizard.Compile(
		func(b *wizard.Builder) {
			p256VerifySource := &P256VerifyDataSource{
				ID:       ct.GetCommit(b, "ID"),
				CS:       ct.GetCommit(b, "CIRCUIT_SELECTOR_P256_VERIFY"),
				Limb:     ct.GetCommit(b, "LIMB"),
				Index:    ct.GetCommit(b, "INDEX"),
				IsData:   ct.GetCommit(b, "DATA_P256_VERIFY_FLAG"),
				IsResult: ct.GetCommit(b, "RSLT_P256_VERIFY_FLAG"),
			}
			p256Verify = newP256Verify(b.CompiledIOP, limits, p256VerifySource)
			if withCircuit {
				p256Verify = p256Verify.
					WithCircuit(b.CompiledIOP, query.PlonkRangeCheckOption(16, 6, true))
			}
		},
		dummy.Compile,
	)

	proof := wizard.Prove(cmp,
		func(run *wizard.ProverRuntime) {
			ct.Assign(run, "ID", "CIRCUIT_SELECTOR_P256_VERIFY", "LIMB", "INDEX", "DATA_P256_VERIFY_FLAG", "RSLT_P256_VERIFY_FLAG")
			p256Verify.Assign(run)
		})

	if err := wizard.Verify(cmp, proof); err != nil {
		t.Fatal("proof failed", err)
	}
	t.Log("proof succeeded")
}

func TestP256VerifyNoCircuit(t *testing.T) {
	limits := &Limits{
		NbInputInstances: 6,
		LimitCalls:       640,
	}
	testP256Verify(t, false, "testdata/p256verify_inputs.csv", limits)
}

func TestP256VerifyWithCircuit(t *testing.T) {
	limits := &Limits{
		NbInputInstances: 6,
		LimitCalls:       640,
	}
	testP256Verify(t, true, "testdata/p256verify_inputs.csv", limits)
}
