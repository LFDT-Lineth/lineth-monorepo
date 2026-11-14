package fri

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestShapeOpeningProofMatchesHonestProof checks that the zero-data proof has
// the geometry of an honest proof, field by field, across every fixture family:
// plain and grouped commitments, pair leaves, twin batches, every cap height and
// a fixed FRI domain. The comparison runs on the unassigned circuit structs,
// which keep exactly the geometry (lengths, absent levels, row widths, revealed
// table sizes) and drop the values.
func TestShapeOpeningProofMatchesHonestProof(t *testing.T) {
	fixtures := map[string]func(t *testing.T) pcsOpenVerifyFixture{
		"open-verify":  newPCSOpenVerifyFixture,
		"twin-batches": newTwinBatchesFixture,
		"grouped/1":    func(t *testing.T) pcsOpenVerifyFixture { return newGroupedPCSFixture(t, 1) },
		"grouped/3":    func(t *testing.T) pcsOpenVerifyFixture { return newGroupedPCSFixture(t, 3) },
		"pair-leaves/1": func(t *testing.T) pcsOpenVerifyFixture {
			return newPairLeavesPCSFixture(t, 1)
		},
		"pair-leaves/3": func(t *testing.T) pcsOpenVerifyFixture {
			return newPairLeavesPCSFixture(t, 3)
		},
	}
	for capHeight := uint8(0); capHeight <= 3; capHeight++ {
		fixtures[fmt.Sprintf("circuit/cap=%d", capHeight)] = func(t *testing.T) pcsOpenVerifyFixture {
			return newPCSCircuitFixture(t, capHeight)
		}
		fixtures[fmt.Sprintf("circuit/cap=%d/fixed-domain+2", capHeight)] = func(t *testing.T) pcsOpenVerifyFixture {
			return newPCSCircuitFixtureAt(t, capHeight, 2)
		}
	}

	for name, build := range fixtures {
		t.Run(name, func(t *testing.T) {
			fx := build(t)
			shaped, err := fx.pcs.ShapeOpeningProof(fx.input.Shapes, fx.input.Shifts)
			require.NoError(t, err)
			require.Equal(t, AllocateGnarkOpeningProof(fx.proof), AllocateGnarkOpeningProof(shaped),
				"zero-data proof must have the honest proof's geometry")
		})
	}
}

// TestShapeOpeningProofComparisonIsSensitive guards the test above against
// passing vacuously: geometries that differ only in cap height must compare
// unequal.
func TestShapeOpeningProofComparisonIsSensitive(t *testing.T) {
	low, high := newPCSCircuitFixture(t, 0), newPCSCircuitFixture(t, 2)
	shaped, err := low.pcs.ShapeOpeningProof(low.input.Shapes, low.input.Shifts)
	require.NoError(t, err)
	require.NotEqual(t, AllocateGnarkOpeningProof(high.proof), AllocateGnarkOpeningProof(shaped))
}

// TestShapeOpeningProofLeavesPCSUntouched checks that the receiver keeps its
// registered openings: allocating a circuit must not disturb a prover that
// shares the PCS.
func TestShapeOpeningProofLeavesPCSUntouched(t *testing.T) {
	fx := newPCSOpenVerifyFixture(t)
	before := *fx.pcs
	_, err := fx.pcs.ShapeOpeningProof(fx.input.Shapes, fx.input.Shifts)
	require.NoError(t, err)
	require.Equal(t, before.openings, fx.pcs.openings)
	require.Equal(t, before.zeta, fx.pcs.zeta)
}

func TestShapeOpeningProofRejectsMismatchedInputs(t *testing.T) {
	fx := newPCSOpenVerifyFixture(t)
	_, err := fx.pcs.ShapeOpeningProof(fx.input.Shapes, fx.input.Shifts[:len(fx.input.Shifts)-1])
	require.ErrorContains(t, err, "shapes")
}
