package fri

import (
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/stretchr/testify/require"
)

// newTwinBatchesFixture opens three batches: two twins committing the same
// table over two sizes, hence the same Merkle root, and a distinct, shorter
// one. Every batch gets its own input tree, so the proof carries three input
// caps and three openings per query, in layout order: twin 0, twin 1 (both
// first seen at the top size), then the distinct batch.
func newTwinBatchesFixture(t *testing.T) pcsOpenVerifyFixture {
	t.Helper()

	params, err := NewParams(4, 3, pcsCircuitNumQueries)
	require.NoError(t, err)
	params.MerkleCapHeight = pcsCircuitCapHeight
	pcs, err := NewPCS(params, makeEncoders(int(params.numRounds()+1), 2))
	require.NoError(t, err)

	prng := rand.New(utils.NewRandSource(20261110))
	twin := make(Batch, 4)
	twin[2] = SizedTable{Ext: [][]field.Ext{field.VecPseudoRandExt(prng, 4)}}
	twin[3] = SizedTable{Ext: [][]field.Ext{
		field.VecPseudoRandExt(prng, 8),
		field.VecPseudoRandExt(prng, 8),
	}}
	other := make(Batch, 3)
	other[1] = SizedTable{Ext: [][]field.Ext{field.VecPseudoRandExt(prng, 2)}}
	other[2] = SizedTable{Ext: [][]field.Ext{field.VecPseudoRandExt(prng, 4)}}

	twinCommitted := pcs.Commit(twin)
	witnesses := []Batch{twin, twin, other}
	committed := []CommitterState{twinCommitted, twinCommitted, pcs.Commit(other)}

	twinShifts := make(BatchShifts, 4)
	twinShifts[2] = SizedShifts{Ext: [][]int{{0}}}
	twinShifts[3] = SizedShifts{Ext: [][]int{{0}, {1}}}
	otherShifts := make(BatchShifts, 3)
	otherShifts[1] = SizedShifts{Ext: [][]int{{0}}}
	otherShifts[2] = SizedShifts{Ext: [][]int{{1}}}
	shifts := []BatchShifts{twinShifts, twinShifts, otherShifts}

	zeta := field.UintsToExt(19, 2, 3, 5, 7, 11)
	challenges := Challenges{
		FoldAlphas: []field.Ext{
			field.UintsToExt(29, 1, 0, 0, 0, 0),
			field.UintsToExt(31, 0, 1, 0, 0, 0),
			field.UintsToExt(37, 0, 0, 1, 0, 0),
		},
		QueryPositions: []int{3, 9, 12, 6},
	}
	proof, claimed := openForTest(t, pcs, openInputs{
		Witnesses:  witnesses,
		Committed:  committed,
		Shifts:     shifts,
		Zeta:       zeta,
		Challenges: challenges,
	})

	roots := utils.Map(func(c CommitterState) field.Octuplet { return c.Tree.Root() }, committed)
	require.Equal(t, roots[0], roots[1], "the twins must share a Merkle root")
	require.NotEqual(t, roots[0], roots[2], "the third batch must have its own root")

	return pcsOpenVerifyFixture{
		pcs:       pcs,
		committed: committed,
		input: VerifyInputs{
			Roots:         roots,
			Shapes:        utils.Map(Batch.Shape, witnesses),
			Shifts:        shifts,
			ClaimedValues: claimed,
			Zeta:          zeta,
			Challenges:    challenges,
		},
		proof: proof,
	}
}

// dedupedShape returns the proof in the shape a root-deduplicating prover
// would emit: the second twin's input tree (cap and per-query opening) dropped.
func dedupedShape(p OpeningProof) OpeningProof {
	const dropped = 1
	res := p
	res.InputCaps = append(append([]InputCap(nil), p.InputCaps[:dropped]...), p.InputCaps[dropped+1:]...)
	res.InputQueries = make([]InputQuery, len(p.InputQueries))
	for q, query := range p.InputQueries {
		res.InputQueries[q] = append(append(InputQuery(nil), query[:dropped]...), query[dropped+1:]...)
	}
	return res
}

// TestPCSTwinBatchesGetSeparateInputTrees checks that batches with equal roots
// are never merged: one input tree per batch, accepted natively.
func TestPCSTwinBatchesGetSeparateInputTrees(t *testing.T) {
	fx := newTwinBatchesFixture(t)

	require.Len(t, fx.proof.InputCaps, 3, "one input cap per batch, twins included")
	require.Len(t, fx.proof.InputQueries, pcsCircuitNumQueries)
	for q, query := range fx.proof.InputQueries {
		require.Len(t, query, 3, "query %d: one input opening per batch, twins included", q)
	}
	require.NoError(t, fx.pcs.Verify(fx.input, fx.proof), "native verifier must accept twin batches")

	require.ErrorContains(t, fx.pcs.Verify(fx.input, dedupedShape(fx.proof)),
		"got 2 input caps, want 3",
		"native verifier must reject the old deduplicated shape")
}

// TestPCSVerifyGnarkTwinBatches is the circuit counterpart: the circuit accepts
// the twin-batch proof, closing the completeness gap with the native verifier,
// and rejects the old deduplicated shape.
func TestPCSVerifyGnarkTwinBatches(t *testing.T) {
	fx := newTwinBatchesFixture(t)

	template, assignment := circuitFor(fx, fx.proof)
	require.NoError(t, solvePCSCircuit(t, template, assignment),
		"twin-batch proof must satisfy the verifier circuit")

	deduped := dedupedShape(fx.proof)
	// Build template and assignment from the same (deduplicated) geometry so the
	// witness is well-formed and the rejection comes from VerifyGnark itself.
	dedupedFixture := fx
	dedupedFixture.proof = deduped
	template, assignment = circuitFor(dedupedFixture, deduped)
	require.ErrorContains(t, compileAndSolvePCSCircuit(template, assignment),
		"got 2 input caps, want 3",
		"the old deduplicated shape must be rejected by VerifyGnark")
}

// droppedTwinOpening returns the proof with all three input caps kept but the
// second twin's per-query opening dropped: a partially deduplicated shape that
// passes the cap-count check and must be caught by the per-query check.
func droppedTwinOpening(p OpeningProof) OpeningProof {
	const dropped = 1
	res := p
	res.InputQueries = make([]InputQuery, len(p.InputQueries))
	for q, query := range p.InputQueries {
		res.InputQueries[q] = append(append(InputQuery(nil), query[:dropped]...), query[dropped+1:]...)
	}
	return res
}

// TestPCSTwinBatchesRejectDroppedOpening checks that both verifiers reject a
// proof that keeps one input cap per batch but omits a twin's query opening.
func TestPCSTwinBatchesRejectDroppedOpening(t *testing.T) {
	fx := newTwinBatchesFixture(t)
	dropped := droppedTwinOpening(fx.proof)
	require.Len(t, dropped.InputCaps, 3, "all input caps must be kept")

	require.ErrorContains(t, fx.pcs.Verify(fx.input, dropped),
		"input query has 2 tree openings, want 3",
		"native verifier must reject a missing twin opening")

	droppedFixture := fx
	droppedFixture.proof = dropped
	template, assignment := circuitFor(droppedFixture, dropped)
	require.ErrorContains(t, compileAndSolvePCSCircuit(template, assignment),
		"input query has 2 tree openings, want 3",
		"circuit must reject a missing twin opening")
}
