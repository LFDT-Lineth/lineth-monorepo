package testtools

import (
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/test"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
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
}

// RunTestShouldPassWithGnark executes a test case expecting it to pass using
// the gnark verifier circuit in place of the normal verifier.
func RunTestShouldPassWithGnark(t *testing.T, tc Testcase, suite []func(comp *wizard.CompiledIOP)) {

	var (
		define = func(b *wizard.Builder) {
			tc.Define(b.CompiledIOP)
		}

		comp    = wizard.Compile(define, suite...)
		proof   = wizard.Prove(comp, tc.Assign)
		circuit = &verifierCircuit{
			C: wizard.AllocateWizardCircuit(comp, comp.NumRounds()),
		}
		assignment = &verifierCircuit{
			C: wizard.AssignVerifierCircuit(comp, proof, comp.NumRounds()),
		}
		solveErr = test.IsSolved(circuit, assignment, ecc.BLS12_377.ScalarField())
	)

	if solveErr != nil {
		t.Fatal(solveErr)
	}
}

func runTestShouldPass(t *testing.T, comp *wizard.CompiledIOP, prover wizard.MainProverStep) {

	var (
		proof = wizard.Prove(comp, prover)
		err   = wizard.Verify(comp, proof)
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
		proof = wizard.Prove(comp, prover)
	})

	if panicErr != nil {
		return
	}

	panicErr = utils.RecoverPanic(func() {
		verErr = wizard.Verify(comp, proof)
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
