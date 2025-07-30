//go:build !fuzzlight

package ecarith

import (
	"os"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend"
	"github.com/consensys/gnark/test"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils/csvtraces"
)

var (

	// 8812078025088706800239420563438698135256154119486538953196282662928678739599
	dummyPxLo = "283099484937911152101840152952769175183"
	dummyPxHi = "25896369843742486929410689099938763911"

	// 19384817048585926091202754628019327561583315513931387211365745455438290666381
	dummyPyLo = "111183359799337868641726371639121593229"
	dummyPyHi = "56966857330840811282595055882070803917"

	// 6392244365320561037829193157981140850021222049710818941199853670416625377985
	dummyScalarLo = "245773738942695872223709139042640798401"
	dummyScalarHi = "18785117851274795320712487213606100714"

	// 18286867191893102909980352899825705524248643499324573950097278735467138404960
	dummyRxLo = "116681329789095301534161201352163349088"
	dummyRxHi = "53740272695769426816068725118027597037"

	// 9901600500795379168368190931083365122151061434285910708538887599132533626517
	dummyRyLo = "44262458709865108276042827186833757845"
	dummyRyHi = "29098188631960252798063832986963793337"
)

func TestMultiEcMulCircuit(t *testing.T) {
	limits := &Limits{
		NbInputInstances:   2,
		NbCircuitInstances: 0, // not used in this test
	}
	circuit := NewECMulCircuit(limits)
	assignment := NewECMulCircuit(limits)
	instanceAssignment := ECMulInstance{
		P_X_hi: dummyPxHi,
		P_X_lo: dummyPxLo,
		P_Y_hi: dummyPyHi,
		P_Y_lo: dummyPyLo,

		R_X_hi: dummyRxHi,
		R_X_lo: dummyRxLo,
		R_Y_hi: dummyRyHi,
		R_Y_lo: dummyRyLo,

		N_hi: dummyScalarHi,
		N_lo: dummyScalarLo,
	}
	for i := 0; i < limits.NbInputInstances; i++ {
		assignment.Instances[i] = instanceAssignment
	}

	// 403569 constraints
	assert := test.NewAssert(t)
	assert.SolvingSucceeded(
		circuit,
		assignment,
		test.NoFuzzing(),
		test.NoSerializationChecks(),
		test.WithBackends(backend.PLONK),
		test.WithCurves(ecc.BLS12_377))

}

func TestEcMulIntegration(t *testing.T) {
	f, err := os.Open("testdata/ecmul_test.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	limits := &Limits{
		NbInputInstances:   3,
		NbCircuitInstances: 2,
	}
	ct, err := csvtraces.NewCsvTrace(f, csvtraces.WithNbRows(limits.sizeEcMulIntegration()))
	if err != nil {
		t.Fatal(err)
	}
	var ecMul *EcMul
	var ecMulSource *EcDataMulSource
	cmp := wizard.Compile(
		func(b *wizard.Builder) {
			ecMulSource = &EcDataMulSource{
				CsEcMul: ct.GetCommit(b, "CS_MUL"),
				Limb:    ct.GetCommit(b, "LIMB"),
				Index:   ct.GetCommit(b, "INDEX"),
				IsData:  ct.GetCommit(b, "IS_DATA"),
				IsRes:   ct.GetCommit(b, "IS_RES"),
			}
			ecMul = newEcMul(b.CompiledIOP, limits, ecMulSource, []query.PlonkOption{query.PlonkRangeCheckOption(16, 6, true)})
		},
		dummy.Compile,
	)

	proof := wizard.Prove(cmp,
		func(run *wizard.ProverRuntime) {
			ct.Assign(run, "CS_MUL", "LIMB", "INDEX", "IS_DATA", "IS_RES")
			ecMul.Assign(run)
		})

	if err := wizard.Verify(cmp, proof); err != nil {
		t.Fatal("proof failed", err)
	}

	t.Log("proof succeeded")
}
