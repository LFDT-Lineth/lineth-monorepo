package gnarkutil

import (
	"math/big"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/stretchr/testify/require"
)

type toBytes32WrapperCircuit struct {
	V frontend.Variable
}

func (c *toBytes32WrapperCircuit) Define(api frontend.API) error {
	ToBytes32(api, c.V)
	return nil
}

// TestToBytes32RecompositionEnforced verifies that ToBytes32 rejects a forged hint
// whose bytes do not recompose to the input value.
func TestToBytes32RecompositionEnforced(t *testing.T) {
	RegisterHintsAndGkrGates()

	ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &toBytes32WrapperCircuit{})
	require.NoError(t, err)

	witness, err := frontend.NewWitness(&toBytes32WrapperCircuit{V: 0}, ecc.BLS12_377.ScalarField())
	require.NoError(t, err)

	// Honest hint (all-zero bytes for V=0) must be accepted.
	require.NoError(t, ccs.IsSolved(witness), "honest all-zero bytes for V=0 must be accepted")

	// Forged hint: bytes[31]=1 makes recomposed value=1 ≠ V=0; must be rejected.
	hintID := findBreakIntoBytesHintID(t)
	forgedHint := func(_ *big.Int, _ []*big.Int, outputs []*big.Int) error {
		for i := range outputs {
			outputs[i].SetUint64(0)
		}
		outputs[len(outputs)-1].SetUint64(1) // LSB byte != 0, but V=0
		return nil
	}
	require.Error(
		t,
		ccs.IsSolved(witness, solver.OverrideHint(hintID, forgedHint)),
		"bytes summing to 1 must be rejected when V=0",
	)
}

func findBreakIntoBytesHintID(t *testing.T) solver.HintID {
	t.Helper()
	const name = "github.com/consensys/linea-monorepo/prover/utils/gnarkutil.breakIntoBytesHint"
	for _, hintFn := range solver.GetRegisteredHints() {
		if solver.GetHintName(hintFn) == name {
			return solver.GetHintID(hintFn)
		}
	}
	t.Fatalf("hint %q is not registered", name)
	return 0
}
