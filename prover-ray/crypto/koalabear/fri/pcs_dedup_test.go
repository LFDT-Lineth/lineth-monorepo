package fri

import (
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/stretchr/testify/require"
)

// identicalBatchesFixture commits the same table twice, so both batches have
// the same Merkle root, plus a third, distinct batch. The twins span two sizes,
// so each is visited at two levels but must still get a single input tree. It
// returns the proof opened with root deduplication on or off, and the matching
// verifier inputs.
func identicalBatchesFixture(t *testing.T, disableDedup bool) (*PCS, VerifyInputs, OpeningProof) {
	t.Helper()

	params, err := NewParams(3, 2, 1)
	require.NoError(t, err)
	pcs, err := NewPCS(params, makeEncoders(int(params.numRounds()+1), 2))
	require.NoError(t, err)
	if disableDedup {
		pcs.DisableRootDedup()
	}

	prng := rand.New(utils.NewRandSource(20261016))
	twin := make(Batch, 3) // indexed by size log2: nothing at 2^0, one column of size 2 and one of size 4
	twin[1] = SizedTable{Base: [][]field.Element{field.VecPseudoRandBase(prng, 2)}}
	twin[2] = SizedTable{Ext: [][]field.Ext{field.VecPseudoRandExt(prng, 4)}}
	// other has one column of size 4, matching nothing in twin, so its root differs.
	other := make(Batch, 3)
	other[2] = SizedTable{Ext: [][]field.Ext{field.VecPseudoRandExt(prng, 4)}} // prng has moved on: different values from twin[2]
	witnesses := []Batch{twin, twin, other}

	committed := make([]CommitterState, len(witnesses))
	roots := make([]field.Octuplet, len(witnesses))
	shifts := make([]BatchShifts, len(witnesses))
	for i, w := range witnesses {
		committed[i] = pcs.Commit(w)
		roots[i] = committed[i].Tree.Root()
		shifts[i] = make(BatchShifts, 3)
		shifts[i][2] = SizedShifts{Ext: [][]int{{0}}}
		if w[1].Base != nil {
			shifts[i][1] = SizedShifts{Base: [][]int{{0}}}
		}
	}
	require.Equal(t, roots[0], roots[1], "identical tables must share a root")
	require.NotEqual(t, roots[0], roots[2])

	zeta := field.UintsToExt(19, 2, 3, 5, 7, 11)
	challenges := Challenges{
		FoldAlphas:     []field.Ext{field.UintsToExt(29, 1, 0, 0, 0, 0), field.UintsToExt(31, 0, 1, 0, 0, 0)},
		QueryPositions: []int{3},
	}
	proof, claimed := openForTest(t, pcs, openInputs{
		Witnesses:  witnesses,
		Committed:  committed,
		Shifts:     shifts,
		Zeta:       zeta,
		Challenges: challenges,
	})
	return pcs, VerifyInputs{
		Roots:         roots,
		Shapes:        utils.Map(Batch.Shape, witnesses),
		Shifts:        shifts,
		ClaimedValues: claimed,
		Zeta:          zeta,
		Challenges:    challenges,
	}, proof
}

// TestPCSRootDedup checks both modes on batches with equal roots: by default
// they share one input tree, with DisableRootDedup each batch gets its own,
// and a verifier configured for the other mode rejects the proof on shape.
func TestPCSRootDedup(t *testing.T) {
	t.Run("DedupEnabled", func(t *testing.T) {
		pcs, in, proof := identicalBatchesFixture(t, false)
		require.Len(t, proof.InputCaps, 2, "equal roots must share one input tree")
		require.Len(t, proof.InputQueries[0], 2)
		require.NoError(t, pcs.Verify(in, proof))
	})
	t.Run("DedupDisabled", func(t *testing.T) {
		pcs, in, proof := identicalBatchesFixture(t, true)
		require.Len(t, proof.InputCaps, 3, "every batch must get its own input tree")
		require.Len(t, proof.InputQueries[0], 3)
		require.NoError(t, pcs.Verify(in, proof))
	})
	t.Run("mode-mismatch", func(t *testing.T) {
		dedupPCS, in, dedupProof := identicalBatchesFixture(t, false)
		noDedupPCS, _, noDedupProof := identicalBatchesFixture(t, true)
		require.ErrorContains(t, noDedupPCS.Verify(in, dedupProof), "input caps")
		require.ErrorContains(t, dedupPCS.Verify(in, noDedupProof), "input caps")
	})
}
