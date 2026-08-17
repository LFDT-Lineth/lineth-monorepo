package pi_interconnection

import (
	"math/big"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/test"
	dataavailability "github.com/consensys/linea-monorepo/prover/circuits/dataavailability/v2"
	"github.com/consensys/linea-monorepo/prover/circuits/internal"
	"github.com/stretchr/testify/require"
)

const toCrumbsHintName = "github.com/consensys/linea-monorepo/prover/circuits/internal.toCrumbsHint"

// fpiPairCircuit holds two DA FPIs, calls RangeCheck on both, asserts their Sum
// values are equal, and requires the Y[1] and Eip4844Enabled fields to differ.
// This models the flag-alias forgery: same packed scalar, different (Y1, flag) pairs.
type fpiPairCircuit struct {
	Honest dataavailability.FunctionalPublicInputQSnark
	Forged dataavailability.FunctionalPublicInputQSnark
}

func (c *fpiPairCircuit) Define(api frontend.API) error {
	c.Honest.RangeCheck(api)
	c.Forged.RangeCheck(api)
	api.AssertIsEqual(c.Honest.Sum(api), c.Forged.Sum(api))
	api.AssertIsDifferent(c.Honest.Y[1], c.Forged.Y[1])
	api.AssertIsDifferent(c.Honest.Eip4844Enabled, c.Forged.Eip4844Enabled)
	return nil
}

func makeFPI(y1, flag int) dataavailability.FunctionalPublicInputQSnark {
	var x [32]frontend.Variable
	for i := range x {
		x[i] = i + 1
	}
	return dataavailability.FunctionalPublicInputQSnark{
		X:              x,
		Y:              [2]frontend.Variable{77, y1},
		SnarkHash:      1234,
		Eip4844Enabled: flag,
		NbBatches:      0,
		AllBatchesSum:  5678,
	}
}

// TestFPIFlagAliasForgeryRejected verifies that the (Y1+1, flag-256) alias is
// rejected now that RangeCheck asserts Eip4844Enabled is boolean.
func TestFPIFlagAliasForgeryRejected(t *testing.T) {
	internal.RegisterHints()

	assignment := &fpiPairCircuit{
		Honest: makeFPI(100, 1),
		Forged: makeFPI(101, -255), // same packed scalar; flag=-255 is not boolean
	}
	require.Error(
		t,
		test.IsSolved(&fpiPairCircuit{}, assignment, ecc.BLS12_377.ScalarField()),
		"flag=-255 alias must be rejected after AssertIsBoolean is added to RangeCheck",
	)
}

// TestEvalClaimCrumbForgeryRejected verifies that a forged toCrumbsHint cannot
// produce bytes inconsistent with Y[1] now that ToCrumbs asserts recomposition.
func TestEvalClaimCrumbForgeryRejected(t *testing.T) {
	internal.RegisterHints()

	// Witness: Encoded=[0,0] (Y[1]=0), but ExpectedRecoded has last byte = 1.
	var expectedRecoded [32]frontend.Variable
	for i := range expectedRecoded {
		expectedRecoded[i] = 0
	}
	expectedRecoded[31] = 1

	assignment := &fr377EncodedFr381ToBytesCircuit{
		Encoded:         [2]frontend.Variable{0, 0},
		ExpectedRecoded: expectedRecoded,
	}

	ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &fr377EncodedFr381ToBytesCircuit{})
	require.NoError(t, err)

	witness, err := frontend.NewWitness(assignment, ecc.BLS12_377.ScalarField())
	require.NoError(t, err)

	// Honest crumbs encode Y[1]=0 as all-zero bytes, so ExpectedRecoded[31]=1 fails.
	require.Error(t, ccs.IsSolved(witness), "honest crumbs encode Y[1]=0 as all-zero bytes")

	// With a forged hint (crumbs[0]=1, sum=1 ≠ Y[1]=0) the recomposition
	// constraint must now also reject the witness.
	hintID := daUnderbindingFindHintID(t, toCrumbsHintName)
	forgedHint := func(_ *big.Int, _ []*big.Int, outputs []*big.Int) error {
		for i := range outputs {
			outputs[i].SetUint64(0)
		}
		outputs[0].SetUint64(1)
		return nil
	}
	require.Error(
		t,
		ccs.IsSolved(witness, solver.OverrideHint(hintID, forgedHint)),
		"recomposition constraint must reject forged crumbs after the fix",
	)
}

func daUnderbindingFindHintID(t *testing.T, name string) solver.HintID {
	t.Helper()
	for _, hintFn := range solver.GetRegisteredHints() {
		if solver.GetHintName(hintFn) == name {
			return solver.GetHintID(hintFn)
		}
	}
	t.Fatalf("hint %q is not registered", name)
	return 0
}
