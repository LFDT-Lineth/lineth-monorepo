package fri

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
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

// TestShapeOpeningProofIndependentOfQueryPositions checks the assumption
// ShapeOpeningProof rests on: query positions select values, never shapes.
// The shape is built with every query at position 0 (all queries colliding),
// whereas real positions are pseudo-random and mostly distinct. Each fixture's
// real committed data is reopened under random fold challenges and random query
// positions (and once under pairwise distinct positions, when the domain allows
// it), and the geometry must match the zero-position shape every time. A prover
// optimisation that deduplicates queries or shares Merkle path nodes between
// them would break this test before it broke the gnark verifier circuit.
func TestShapeOpeningProofIndependentOfQueryPositions(t *testing.T) {
	fixtures := map[string]func(t *testing.T) pcsOpenVerifyFixture{
		"twin-batches": newTwinBatchesFixture,
		"grouped/3":    func(t *testing.T) pcsOpenVerifyFixture { return newGroupedPCSFixture(t, 3) },
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

	const numSeeds = 16
	for name, build := range fixtures {
		t.Run(name, func(t *testing.T) {
			fx := build(t)
			shaped, err := fx.pcs.ShapeOpeningProof(fx.input.Shapes, fx.input.Shifts)
			require.NoError(t, err)
			want := AllocateGnarkOpeningProof(shaped)

			for seed := range uint64(numSeeds + 1) {
				distinct := seed == numSeeds
				proof, positions := reopenAtRandomPositions(t, fx, seed, distinct)
				require.Equal(t, want, AllocateGnarkOpeningProof(proof),
					"positions %v: real proof geometry differs from the zero-position shape", positions)
			}
		})
	}
}

// reopenAtRandomPositions reopens fx's committed batches with its claims and
// zeta, folding under random challenges and querying random positions of the
// (possibly restricted) codeword domain. With distinct, the positions are
// pairwise distinct when the domain is large enough.
func reopenAtRandomPositions(t *testing.T, fx pcsOpenVerifyFixture, seed uint64, distinct bool) (OpeningProof, []int) {
	t.Helper()
	pcs := fx.pcs
	pcs.Reset()
	defer pcs.Reset()

	// Some fixtures commit once and open the same committed state as several
	// batches; fx.committed then holds it once.
	for i := range fx.input.Shifts {
		committed := fx.committed[min(i, len(fx.committed)-1)]
		require.NoError(t, pcs.AddOpening(committed, fx.input.Zeta, fx.input.Shifts[i], fx.input.ClaimedValues[i]))
	}
	restricted, err := pcs.restrictToOpenings()
	require.NoError(t, err)
	domain := 1 << restricted.Params.LogCodewordSize

	prng := rand.New(rand.NewPCG(seed, 0x5eed))
	state, err := pcs.NewProverState()
	require.NoError(t, err)
	for state.HasNext() {
		state.Fold(field.PseudoRandExt(prng))
	}

	n := int(pcs.Params.NumQueries)
	var positions []int
	if distinct && n <= domain {
		positions = prng.Perm(domain)[:n]
	} else {
		positions = make([]int, n)
		for k := range positions {
			positions[k] = prng.IntN(domain)
		}
	}
	return pcs.Open(state, positions), positions
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
