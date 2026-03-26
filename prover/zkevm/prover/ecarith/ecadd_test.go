package ecarith

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/limbs"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils/csvtraces"
)

func TestEcAddIntegration(t *testing.T) {
	limits := &Limits{
		NbInputInstances:   1,
		NbCircuitInstances: 1,
	}
	ct := csvtraces.MustOpenCsvFile("testdata/ecadd_test.csv")
	var ecAdd *EcAdd
	var ecAddSource *EcDataAddSource
	cmp := wizard.Compile(
		func(b *wizard.Builder) {
			ecAddSource = &EcDataAddSource{
				CsEcAdd: ct.GetCommit(b, "CS_ADD"),
				Index:   ct.GetCommit(b, "INDEX"),
				IsData:  ct.GetCommit(b, "IS_DATA"),
				IsRes:   ct.GetCommit(b, "IS_RES"),
				Limbs:   ct.GetLimbsLe(b, "LIMB", limbs.NbLimbU128).AssertUint128(),
			}

			ecAdd = newEcAdd(b.CompiledIOP, limits, ecAddSource, []query.PlonkOption{query.PlonkRangeCheckOption(16, 1, true)})
		},
		dummy.Compile,
	)

	proof := wizard.Prove(cmp,
		func(run *wizard.ProverRuntime) {
			ct.Assign(run,
				ecAddSource.CsEcAdd,
				ecAddSource.Limbs,
				ecAddSource.Index,
				ecAddSource.IsData,
				ecAddSource.IsRes,
			)
			ecAdd.Assign(run)
		})

	if err := wizard.Verify(cmp, proof); err != nil {
		t.Fatal("proof failed", err)
	}

	t.Log("proof succeeded")
}
