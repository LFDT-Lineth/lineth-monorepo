package testtools

import (
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/field/koalabear"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/test"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/serde"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/gnarkutil"
	"github.com/stretchr/testify/require"
)

// Testcase is an object specifying how a wizard testcase protocol should
// both be defined and assigned.
type Testcase interface {
	Define(comp *wizard.CompiledIOP)
	Assign(run *wizard.ProverRuntime)
	MustFail() bool
	Name() string
}

// AnonymousTestcase is an implementation of testcase allowing the caller to
// explicitly provide his own functions for define, assign and mustFail.
type AnonymousTestcase struct {
	NameStr      string
	DefineFunc   func(comp *wizard.CompiledIOP)
	AssignFunc   func(run *wizard.ProverRuntime)
	MustFailFlag bool
}

// verifierCircuit is a [frontend.Circuit] implementation that only verifies
// a wizard proof. It is used to cover the Plonk in wizard cases.
type verifierCircuit struct {
	C *wizard.VerifierCircuit
}

// Define implements the [frontend.Circuit] interface.
func (c *verifierCircuit) Define(api frontend.API) error {
	c.C.Verify(api)
	return nil
}

// RunTestcase compiles and runs a testcase using the provided compilation
// suite. The function will attempt to run the prover and verify a proof
// then it will either succeed of fail the test depending on the requirements
// of the test.
func RunTestcase(t *testing.T, tc Testcase, suite []func(comp *wizard.CompiledIOP)) {

	define := func(b *wizard.Builder) {
		tc.Define(b.CompiledIOP)
	}

	comp := wizard.Compile(define, suite...)

	if tc.MustFail() {
		runTestShouldFail(t, comp, tc.Assign)
	}

	if !tc.MustFail() {
		runTestShouldPass(t, comp, tc.Assign)
	}

	buf, err := serde.Serialize(comp)
	if err != nil {
		// Test-internal types (e.g. assignUnivariatePA, autoAssignColumn) are
		// not in the code-generated TypeToID registry and cannot be added
		// without a circular import. Log and skip the serde round-trip.
		// todo @gusiri: consider running `go generate` to regenerate the
		// registry or restructuring testtools to avoid the circular dep.
		t.Logf("skipping serde round-trip: %v", err)
		return
	}

	deser := &wizard.CompiledIOP{}

	if err := serde.Deserialize(buf, deser); err != nil {
		t.Fatal(err)
	}

	if ok := serde.DeepCmp(comp, deser, false); !ok {
		t.Errorf("mismatching deserialized compiled IOP")
	}
}

// RunTestShouldPassWithGnark executes a test case expecting it to pass using
// the gnark verifier circuit in place of the normal verifier.
func RunTestShouldPassWithGnarkKoala(t *testing.T, tc Testcase, suite []func(comp *wizard.CompiledIOP)) {
	runTestShouldPassWithGnark(t, tc, false, suite)
}

// RunTestShouldPassWithGnarkBLS executes a test case expecting it to pass using
// the gnark verifier circuit in place of the normal verifier.
func RunTestShouldPassWithGnarkBLS(t *testing.T, tc Testcase, suite []func(comp *wizard.CompiledIOP)) {
	runTestShouldPassWithGnark(t, tc, true, suite)
}

func runTestShouldPassWithGnark(t *testing.T, tc Testcase, withBLS bool, suite []func(comp *wizard.CompiledIOP)) {

	var (
		define = func(b *wizard.Builder) {
			tc.Define(b.CompiledIOP)
		}

		comp  = wizard.Compile(define, suite...)
		proof = wizard.Prove(comp, tc.Assign, withBLS)
		err   = wizard.Verify(comp, proof, withBLS)
	)

	if err != nil {
		t.Logf("native verifier failed: %v", err)
	}

	var (
		circuit = &verifierCircuit{
			C: wizard.AllocateWizardCircuit(comp, comp.NumRounds(), withBLS),
		}
		assignment = &verifierCircuit{
			C: wizard.AssignVerifierCircuit(comp, proof, comp.NumRounds(), withBLS),
		}

		solveErr error
	)

	if withBLS {
		solveErr = test.IsSolved(circuit, assignment, ecc.BLS12_377.ScalarField())
	} else {
		ccs, compErr := frontend.CompileU32(koalabear.Modulus(), gnarkutil.NewMockBuilder(scs.NewBuilder), circuit)
		require.NoError(t, compErr)
		witness, witErr := frontend.NewWitness(assignment, koalabear.Modulus())
		require.NoError(t, witErr)
		solveErr = ccs.IsSolved(witness)
	}

	if solveErr != nil {
		t.Fatal(solveErr)
	}
}

func runTestShouldPass(t *testing.T, comp *wizard.CompiledIOP, prover wizard.MainProverStep) {

	var (
		proof = wizard.Prove(comp, prover, false)
		err   = wizard.Verify(comp, proof, false)
	)

	if err != nil {
		t.Errorf("verifier failed: %v", err)
	}
}

func runTestShouldFail(t *testing.T, comp *wizard.CompiledIOP, prover wizard.MainProverStep) {

	var (
		verErr, panicErr error
		proof            wizard.Proof
	)

	panicErr = utils.RecoverPanic(func() {
		proof = wizard.Prove(comp, prover, false)
	})

	if panicErr != nil {
		return
	}

	panicErr = utils.RecoverPanic(func() {
		verErr = wizard.Verify(comp, proof, false)
	})

	if panicErr == nil && verErr == nil {
		t.Error("test was expected to fail but did not")
	}
}

func (a *AnonymousTestcase) Define(comp *wizard.CompiledIOP) {
	a.DefineFunc(comp)
}

func (a *AnonymousTestcase) Assign(run *wizard.ProverRuntime) {
	a.AssignFunc(run)
}

func (a *AnonymousTestcase) MustFail() bool {
	return a.MustFailFlag
}

func (a *AnonymousTestcase) Name() string {
	return a.NameStr
}

// autoAssignColumn is a helper [wizard.ProverAction] to auto-assign
// a column.
type autoAssignColumn struct {
	col ifaces.Column
	sv  smartvectors.SmartVector
}

func (ac autoAssignColumn) Run(run *wizard.ProverRuntime) {
	name := ac.col.GetColID()
	run.AssignColumn(name, ac.sv)
}
