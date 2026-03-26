package plonkinwizard_test

import (
	"strconv"
	"testing"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/plonkinwizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/stretchr/testify/assert"
)

// testCircuit is a dummy circuit used for testing the Plonk in Wizard
// compiler. It simply checks that all positions of [testCircuit.A] are consecutive.
// Thus, [1, 2, 3] is a valid assignment and [1, 1, 0] is not.
type testCircuit struct {
	A [3]frontend.Variable `gnark:",public"`
}

// Define implements the [frontend.Circuit] interface
func (c *testCircuit) Define(api frontend.API) error {
	api.AssertIsEqual(api.Add(1, c.A[0]), c.A[1])
	api.AssertIsEqual(api.Add(1, c.A[1]), c.A[2])
	return nil
}

var (
	// circAssignmentValid represents a valid assignment to the circuit
	// and the padding to the next power of two as found in portions of
	// the [query.PlonkInWizardData.Data].
	circAssignmentValid = vector.ForTest(1, 2, 3, 0)
	// circAssignmentInvalid represents an invalid assignment to the circuit
	// and the padding to the next power of two as found in portions of
	// the [query.PlonkInWizardData.Data]. This is used to generated
	// invalid test assignments.
	circAssignmentInvalid = vector.ForTest(1, 1, 0, 0)
	// circAssignmentBadPadding represents a portion of the assignment
	// incorrectly padded. This should make one of the constraint fail.
	circAssignmentBadPadding = vector.ForTest(1, 2, 3, 1)
	// circAssignAllZeros represents a portion of the assignment where
	// inputs are zero and this is an invalid assignment.
	circAssignAllZeros = vector.ForTest(0, 0, 0, 0)
)

var testCases = []struct {
	name       string
	portions   [][]field.Element
	shouldFail bool
}{
	{
		name: "valid",
		portions: [][]field.Element{
			circAssignmentValid,
			circAssignmentValid,
		},
	},
	{
		name: "last-is-invalid",
		portions: [][]field.Element{
			circAssignmentValid,
			circAssignmentInvalid,
		},
		shouldFail: true,
	},
	{
		name: "all-are-invalid",
		portions: [][]field.Element{
			circAssignmentInvalid,
			circAssignmentInvalid,
		},
		shouldFail: true,
	},
	{
		name: "last-is-all-zeroes",
		portions: [][]field.Element{
			circAssignmentValid,
			circAssignAllZeros,
		},
		shouldFail: true,
	},
	{
		name: "first-has-bad-padding",
		portions: [][]field.Element{
			circAssignmentBadPadding,
			circAssignmentValid,
		},
		shouldFail: true,
	},
	{
		name: "all-zeroes",
		portions: [][]field.Element{
			circAssignAllZeros,
			circAssignAllZeros,
		},
		shouldFail: true,
	},
}

// generateAssignment returns a list of smart-vectors to assign the following
// testcase specifications. size is the length of the assignment and
// portions represents the "per-instance" sections of the active part
// of the data column to generate.
func generateAssignment(portions [][]field.Element, size int) (data, sel smartvectors.SmartVector) {

	res := make([]field.Element, 0, len(portions[0])*len(portions))
	for i := range portions {
		res = append(res, portions[i]...)
	}

	data = smartvectors.RightZeroPadded(res, size)
	sel = smartvectors.RightZeroPadded(vector.Repeat(field.NewElement(1), len(res)), size)

	return data, sel
}

// TestPlonkWizardCompiler tests the following scenarios for the Plonk in Wizard
// compiler:
//
// - test cases with different sizes of the data column
// - test cases with different numbers of portions
// - test cases with different active/inactive portions
// - test cases with different padding of the active part
// - test cases with different prover compilers
//
// The test cases are represented as a slice of structs, where each struct
// has the following fields:
//
//   - name: the name of the test case
//   - portions: a slice of smart-vectors, where each smart-vector represents a
//     portion of the active part of the data column
//   - shouldFail: a boolean indicating whether the test case should fail or
//     not
func TestPlonkWizardCompiler(t *testing.T) {

	for _, size := range []int{8, 16} {
		t.Run("size-"+strconv.Itoa(size), func(t *testing.T) {
			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {

					define := func(b *wizard.Builder) {

						var (
							data = b.RegisterCommit("DATA", size)
							sel  = b.RegisterCommit("SEL", size)
						)

						b.InsertPlonkInWizard(query.NewPlonkInWizard(
							ifaces.QueryID("PIW"),
							data,
							sel,
							&testCircuit{},
							nil,
						))
					}

					prover := func(run *wizard.ProverRuntime) {
						data, sel := generateAssignment(tc.portions, size)
						run.AssignColumn("DATA", data)
						run.AssignColumn("SEL", sel)
					}

					t.Run("dummy-compiler", func(t *testing.T) {

						comp := wizard.Compile(define, dummy.Compile)

						if tc.shouldFail {

							hasFailed := false

							// wizard.Prover may panic in case we provide an invalid assignment.
							// We want to catch that and fail the test if that does not happens
							// and the verifier passed but we don't want to make the test panic.
							func() {

								defer func() {
									if r := recover(); r != nil {
										hasFailed = true
									}
								}()

								proof := wizard.Prove(comp, prover)
								if err := wizard.Verify(comp, proof); err != nil {
									hasFailed = true
								}
							}()

							assert.True(t, hasFailed, "should have failed")
							return
						}

						if !tc.shouldFail {
							proof := wizard.Prove(comp, prover)
							if err := wizard.Verify(comp, proof); err != nil {
								t.Errorf("the verifier failed: %v", err)
							}
							return
						}
					})

					t.Run("plonk-in-wizard-compiler", func(t *testing.T) {
						comp := wizard.Compile(define, plonkinwizard.Compile, dummy.Compile)

						if tc.shouldFail {

							hasFailed := false

							// wizard.Prover may panic in case we provide an invalid assignment.
							// We want to catch that and fail the test if that does not happens
							// and the verifier passed but we don't want to make the test panic.
							func() {

								defer func() {
									if r := recover(); r != nil {
										hasFailed = true
									}
								}()

								proof := wizard.Prove(comp, prover)
								if err := wizard.Verify(comp, proof); err != nil {
									hasFailed = true
								}
							}()

							assert.True(t, hasFailed, "should have failed")
							return
						}

						if !tc.shouldFail {
							proof := wizard.Prove(comp, prover)
							if err := wizard.Verify(comp, proof); err != nil {
								t.Errorf("the verifier failed: %v", err)
							}
							return
						}
					})
				})
			}
		})
	}
}
