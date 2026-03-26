package plonkinternal_test

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/consensys/gnark/backend/witness"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	plonk "github.com/consensys/linea-monorepo/prover/protocol/plonkinternal"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils/gnarkutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This is a simple test circuit whose only goal is to utilize the range-checker
// so it triggers the external range-checker of the Wizard. This circuit does
// not use all the witness variables in gates and assumes that the missing gates
// are added.
type testRangeCheckingCircuitIncomplete struct {
	A [10]frontend.Variable `gnark:",public"`
}

func (r *testRangeCheckingCircuitIncomplete) Define(api frontend.API) error {
	rangeChecker := api.(frontend.Rangechecker)

	for i := range r.A {
		rangeChecker.Check(r.A[i], 16)
	}
	return nil
}

// This is a simple test circuit whose only goal is to utilize the range-checker
// so it triggers the external range-checker of the Wizard. This circuit uses
// all the inputs in gates so there should exist a gate for each input.
type testRangeCheckingCircuitComplete struct {
	A [10]frontend.Variable `gnark:",public"`
}

func (r *testRangeCheckingCircuitComplete) Define(api frontend.API) error {
	rangeChecker := api.(frontend.Rangechecker)

	for i := range r.A {
		api.AssertIsDifferent(0, r.A[i])
	}

	for i := range r.A {
		rangeChecker.Check(r.A[i], 16)
	}
	return nil
}

// This is a simple test circuit whose only goal is to utilize the range-checker
// so it triggers the external range-checker of the Wizard. This circuit does
// not use all the internal variables in gates and automatic gate addition
// should error.
type testRangeCheckingCircuitIncompleteInternal struct {
	A [10]frontend.Variable
}

func (r *testRangeCheckingCircuitIncompleteInternal) Define(api frontend.API) error {
	rangeChecker := api.(frontend.Rangechecker)

	for i := range r.A {
		rangeChecker.Check(r.A[i], 16)
	}
	// create an internal variable which is not witness.
	res, err := api.Compiler().NewHint(DummyHint, 1, field.NewElement(0))
	if err != nil {
		return err
	}
	rangeChecker.Check(res[0], 16)
	return nil
}

func DummyHint(_ *big.Int, inputs []*big.Int, outputs []*big.Int) error {
	outputs[0].SetInt64(1)
	return nil
}

// This tests the correctness of the Plonk in Wizard implementation in a case
// where we use the external range-checker. The variables are in range, but the
// compiler should fail as the witness variables are not used in gates.
func TestRangeCheckIncompleteFails(t *testing.T) {

	circuit := &testRangeCheckingCircuitIncomplete{}

	assert.Panics(t, func() {
		wizard.Compile(
			func(build *wizard.Builder) {
				plonk.PlonkCheck(
					build.CompiledIOP,
					"PLONK",
					0,
					circuit,
					1,
					plonk.WithRangecheck(16, 1, false),
				)
			},
			dummy.Compile,
		)
	})
}

// This tests the correctness of the Plonk in Wizard implementation in a case
// where we use the external range-checker. We use the incomplete circuit but
// builder should add the missing gates.
func TestRangeCheckIncompleteSucceeds(t *testing.T) {

	circuit := &testRangeCheckingCircuitIncomplete{}

	var pa plonk.PlonkInWizardProverAction

	compiled := wizard.Compile(
		func(build *wizard.Builder) {
			ctx := plonk.PlonkCheck(
				build.CompiledIOP,
				"PLONK",
				0,
				circuit, 1,
				plonk.WithRangecheck(16, 1, true),
			)

			pa = ctx.GetPlonkProverAction()
		},
		dummy.Compile,
	)

	proof := wizard.Prove(compiled, func(run *wizard.ProverRuntime) {
		pa.Run(run, []witness.Witness{gnarkutil.WitnessFromNativeKoala(
			field.NewElement(1),
			field.NewElement(1),
			field.NewElement(1),
			field.NewElement(1),
			field.NewElement(1),
			field.NewElement(1),
			field.NewElement(1),
			field.NewElement(1),
			field.NewElement(1),
			field.NewElement(1),
		)})
	})
	err := wizard.Verify(compiled, proof)
	require.NoError(t, err)
}

// This tests the correctness of the Plonk in Wizard implementation in a case
// where we use the external range-checker. This is a negative test and the
// prover should not accept the provided witness.
func TestRangeCheckNegative(t *testing.T) {

	circuit := &testRangeCheckingCircuitIncomplete{}

	assignment := gnarkutil.WitnessFromNativeKoala(
		// 0x100000 = 2^5
		field.NewFromString("0x100000"),
		field.NewFromString("0x100000"),
		field.NewFromString("0x100000"),
		field.NewFromString("0x100000"),
		field.NewFromString("0x100000"),
		field.NewFromString("0x100000"),
		field.NewFromString("0x100000"),
		field.NewFromString("0x100000"),
		field.NewFromString("0x100000"),
		field.NewFromString("0x100000"),
	)
	var pa plonk.PlonkInWizardProverAction

	compiled := wizard.Compile(
		func(build *wizard.Builder) {
			ctx := plonk.PlonkCheck(
				build.CompiledIOP,
				"PLONK",
				0,
				circuit, 1,
				plonk.WithRangecheck(16, 1, true),
			)

			pa = ctx.GetPlonkProverAction()
		},
		dummy.Compile,
	)

	assert.Panics(t, func() {
		_ = wizard.Prove(compiled, func(run *wizard.ProverRuntime) {
			pa.Run(run, []witness.Witness{assignment})
		})
	},
		"The prover should refuse to build a proof for this assignment since "+
			"the range check is for 2**16 and we provide 2**100",
	)
}

// This tests the correctness of the Plonk in Wizard implementation in a case
// where we use the external range-checker. The variables are in range and the
// prover should accept the provided witness. The circuit is complete as all the
// witness variables are used in gates.
func TestRangeCheckCompleteSucceeds(t *testing.T) {

	circuit := &testRangeCheckingCircuitComplete{}

	assignment := gnarkutil.WitnessFromNativeKoala(
		field.NewElement(1),
		field.NewElement(1),
		field.NewElement(1),
		field.NewElement(1),
		field.NewElement(1),
		field.NewElement(1),
		field.NewElement(1),
		field.NewElement(1),
		field.NewElement(1),
		field.NewElement(1),
	)

	var pa plonk.PlonkInWizardProverAction

	compiled := wizard.Compile(
		func(build *wizard.Builder) {
			ctx := plonk.PlonkCheck(
				build.CompiledIOP,
				"PLONK",
				0,
				circuit,
				1,
				plonk.WithRangecheck(16, 1, false),
			)

			pa = ctx.GetPlonkProverAction()
		},
		dummy.Compile,
	)

	proof := wizard.Prove(compiled, func(run *wizard.ProverRuntime) {
		pa.Run(run, []witness.Witness{assignment})
	})
	err := wizard.Verify(compiled, proof)
	require.NoError(t, err)
}

// This tests the correctness of the Plonk in Wizard implementation in a case
// where we use the external range-checker. The variables are in range, but the
// compiler should fail as internal variable is not used in gates and automatic
// gate addition should error.
func TestRangeCheckIncompleteInternalFails(t *testing.T) {

	circuit := &testRangeCheckingCircuitIncompleteInternal{}

	assert.Panics(t, func() {
		wizard.Compile(
			func(build *wizard.Builder) {
				plonk.PlonkCheck(
					build.CompiledIOP,
					"PLONK",
					0,
					circuit, 1,
					plonk.WithRangecheck(16, 1, true),
				)

			},
			dummy.Compile,
		)
	})
}

// rangeCheckWithPublic is a simple circuit that uses the range-checker with a
// public variable. Public variables shift the traces to accomodate the
// inclusion of the public inputs and the range checker constraint extractor
// needs to accomodate that.
type rangeCheckWithPublic struct {
	A frontend.Variable `gnark:",public"`
	D frontend.Variable `gnark:",public"`
}

func (c *rangeCheckWithPublic) Define(api frontend.API) error {
	rc := api.(frontend.Rangechecker)
	// we range check the public variable (variable 0)
	rc.Check(c.D, 16)
	api.AssertIsDifferent(0, c.A)
	return nil
}

// This is a simple test for testing regression.
// Here, we use public inputs which shifts the witness trace.
func TestErrorCase(t *testing.T) {
	circuit := &rangeCheckWithPublic{}

	assignment := gnarkutil.WitnessFromNativeKoala(
		field.NewElement(1<<20),
		field.NewElement(2),
	)

	var pa plonk.PlonkInWizardProverAction

	compiled := wizard.Compile(
		func(build *wizard.Builder) {
			ctx := plonk.PlonkCheck(
				build.CompiledIOP,
				"PLONK",
				0,
				circuit, 1,
				plonk.WithRangecheck(16, 1, true),
			)

			pa = ctx.GetPlonkProverAction()
		},
		dummy.Compile,
	)

	wizard.Prove(compiled, func(run *wizard.ProverRuntime) {
		pa.Run(run, []witness.Witness{assignment})
	})
}

type testRangeCheckLRSyncCircuit struct {
	A frontend.Variable `gnark:",public"`
	B frontend.Variable `gnark:",public"`
}

func (c *testRangeCheckLRSyncCircuit) Define(api frontend.API) error {
	rc := api.(frontend.Rangechecker)
	d := api.Mul(c.A, c.B)
	api.AssertIsDifferent(d, 0)
	rc.Check(c.A, 16)
	rc.Check(c.B, 16)
	return nil
}

// This tests that we can successfully range check variables which are both used
// only in a single gate at L,R position
func TestRangeCheckLRSync(t *testing.T) {
	circuit := &testRangeCheckLRSyncCircuit{}

	assignment := gnarkutil.WitnessFromNativeKoala(
		field.NewElement(1),
		field.NewElement(1),
	)

	var pa plonk.PlonkInWizardProverAction

	compiled := wizard.Compile(
		func(build *wizard.Builder) {
			ctx := plonk.PlonkCheck(
				build.CompiledIOP,
				"PLONK",
				0,
				circuit,
				1,
				plonk.WithRangecheck(16, 1, false),
			)

			pa = ctx.GetPlonkProverAction()
		},
		dummy.Compile,
	)

	proof := wizard.Prove(compiled, func(run *wizard.ProverRuntime) {
		pa.Run(run, []witness.Witness{assignment})
	})
	err := wizard.Verify(compiled, proof)
	require.NoError(t, err)
}

type testRangeCheckOCircuit struct {
	A frontend.Variable
	B frontend.Variable
}

func (c *testRangeCheckOCircuit) Define(api frontend.API) error {
	rc := api.(frontend.Rangechecker)
	d := api.Mul(c.A, c.B)
	rc.Check(d, 16)
	return nil
}

// This tests that we can successfully range check O variable
func TestRangeCheckO(t *testing.T) {
	circuit := &testRangeCheckOCircuit{}

	assignment := gnarkutil.WitnessFromNativeKoala(
		field.NewElement(1),
		field.NewElement(1),
	)

	var pa plonk.PlonkInWizardProverAction

	compiled := wizard.Compile(
		func(build *wizard.Builder) {
			ctx := plonk.PlonkCheck(
				build.CompiledIOP,
				"PLONK",
				0,
				circuit,
				1,
				plonk.WithRangecheck(16, 1, false),
			)

			pa = ctx.GetPlonkProverAction()
		},
		dummy.Compile,
	)

	proof := wizard.Prove(compiled, func(run *wizard.ProverRuntime) {
		pa.Run(run, []witness.Witness{assignment})
	})
	err := wizard.Verify(compiled, proof)
	require.NoError(t, err)
}

// This test checks the cross-functionality between the fixed nb of rows
// and the external range-checker.
func TestRangeCheckWithFixedNbRows(t *testing.T) {
	circuit := &testRangeCheckLRSyncCircuit{}

	assignment := gnarkutil.WitnessFromNativeKoala(
		field.NewElement(1),
		field.NewElement(1),
	)

	var pa plonk.PlonkInWizardProverAction

	compiled := wizard.Compile(
		func(build *wizard.Builder) {
			ctx := plonk.PlonkCheck(
				build.CompiledIOP,
				"PLONK",
				0,
				circuit,
				1,
				plonk.WithRangecheck(16, 1, false),
				plonk.WithFixedNbRows(256),
			)

			pa = ctx.GetPlonkProverAction()
		},
		dummy.Compile,
	)

	proof := wizard.Prove(compiled, func(run *wizard.ProverRuntime) {
		pa.Run(run, []witness.Witness{assignment})
	})
	err := wizard.Verify(compiled, proof)
	require.NoError(t, err)
}

// testRangeCheckWideCommitCircuit is a simple circuit that
// range checks some public inputs and then commits to some
// function of them using wide commitments.
type testRangeCheckWideCommitCircuit struct {
	A [10]frontend.Variable `gnark:",public"`
}

func (c *testRangeCheckWideCommitCircuit) Define(api frontend.API) error {
	rangeChecker := api.(frontend.Rangechecker)

	for i := range c.A {
		api.AssertIsDifferent(0, c.A[i])
	}

	for i := range c.A {
		rangeChecker.Check(c.A[i], 16)
	}
	cmter, ok := api.(frontend.WideCommitter)
	if !ok {
		return fmt.Errorf("not a wide committer")
	}

	toCommit := make([]frontend.Variable, len(c.A))
	for i := range c.A {
		toCommit[i] = api.Mul(c.A[i], c.A[i])
	}
	cmt, err := cmter.WideCommit(fext.ExtensionDegree, toCommit...)
	if err != nil {
		return err
	}
	for i := range cmt {
		api.AssertIsDifferent(cmt[i], 0)
	}
	return nil
}

// TestRangeCheckerCompleteWithCommitment tests the correctness of the Plonk in
// Wizard implementation in a case where we use the external range-checker along
// with wide commitments. The variables are in range and the prover should
// accept the provided witness.
func TestRangeCheckerCompleteWithCommitment(t *testing.T) {

	circuit := &testRangeCheckWideCommitCircuit{}

	assignment := gnarkutil.WitnessFromNativeKoala(
		field.NewElement(1),
		field.NewElement(1),
		field.NewElement(1),
		field.NewElement(1),
		field.NewElement(1),
		field.NewElement(1),
		field.NewElement(1),
		field.NewElement(1),
		field.NewElement(1),
		field.NewElement(1),
	)

	var pa plonk.PlonkInWizardProverAction

	compiled := wizard.Compile(
		func(build *wizard.Builder) {
			ctx := plonk.PlonkCheck(
				build.CompiledIOP,
				"PLONK",
				0,
				circuit,
				1,
				plonk.WithRangecheck(16, 1, false),
			)

			pa = ctx.GetPlonkProverAction()
		},
		dummy.Compile,
	)

	proof := wizard.Prove(compiled, func(run *wizard.ProverRuntime) {
		pa.Run(run, []witness.Witness{assignment})
	})
	err := wizard.Verify(compiled, proof)
	require.NoError(t, err)
}
