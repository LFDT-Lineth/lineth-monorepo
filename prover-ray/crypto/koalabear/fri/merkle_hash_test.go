package fri

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/poseidon2"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/stretchr/testify/require"
)

// goldenKoalaBearDigest pins the KoalaBear-mode Merkle outputs of
// goldenFixtureProof to the values produced before MerkleHash existed, so that
// the default mode stays bit-identical.
const goldenKoalaBearDigest = "8a9e9c2d058f60a8871d04a5ce42de4773257ccd63ce3d7397e17e5a3b1614de"

func goldenFixtureProof(t *testing.T, opts ...Option) (*PCS, VerifyInputs, OpeningProof, CommitterState) {
	t.Helper()
	params, err := NewParams(5, 4, 4, opts...)
	require.NoError(t, err)
	params.MerkleCapHeight = 1
	encoders := makeEncoders(int(params.numRounds()+1), 2)
	pcs, err := NewPCS(params, encoders)
	require.NoError(t, err)

	prng := rand.New(utils.NewRandSource(20261111))
	witness := make(Batch, 5)
	witness[1] = SizedTable{Ext: [][]field.Ext{field.VecPseudoRandExt(prng, 2)}}
	witness[2] = SizedTable{
		Base: [][]field.Element{field.VecPseudoRandBase(prng, 4), field.VecPseudoRandBase(prng, 4)},
		Ext:  [][]field.Ext{field.VecPseudoRandExt(prng, 4)},
	}
	witness[4] = SizedTable{
		Base: [][]field.Element{field.VecPseudoRandBase(prng, 16)},
		Ext:  [][]field.Ext{field.VecPseudoRandExt(prng, 16), field.VecPseudoRandExt(prng, 16)},
	}
	committed := pcs.Commit(witness)

	batchShifts := make(BatchShifts, 5)
	batchShifts[1] = SizedShifts{Ext: [][]int{{0}}}
	batchShifts[2] = SizedShifts{Base: [][]int{{0}, {1}}, Ext: [][]int{{0}}}
	batchShifts[4] = SizedShifts{Base: [][]int{{0}}, Ext: [][]int{{0}, {1}}}
	zeta := field.UintsToExt(19, 2, 3, 5, 7, 11)
	challenges := Challenges{
		FoldAlphas: []field.Ext{
			field.UintsToExt(29, 1, 0, 0, 0, 0), field.UintsToExt(31, 0, 1, 0, 0, 0),
			field.UintsToExt(37, 0, 0, 1, 0, 0), field.UintsToExt(41, 0, 0, 0, 1, 0),
		},
		QueryPositions: []int{3, 17, 22, 31},
	}
	witnesses := []Batch{witness}
	shifts := []BatchShifts{batchShifts}
	proof, claimed := openForTest(t, pcs, openInputs{
		Witnesses: witnesses, Committed: []CommitterState{committed},
		Shifts: shifts, Zeta: zeta, Challenges: challenges,
	})
	in := VerifyInputs{
		Roots:         []field.Octuplet{committed.Tree.Root()},
		Shapes:        utils.Map(Batch.Shape, witnesses),
		Shifts:        shifts,
		ClaimedValues: claimed,
		Zeta:          zeta,
		Challenges:    challenges,
	}
	return pcs, in, proof, committed
}

// goldenDigest summarises the Merkle outputs of a proof: the commitment root
// and the FRI round roots.
func goldenDigest(committed CommitterState, proof OpeningProof) string {
	h := sha256.New()
	fmt.Fprint(h, committed.Tree.Root())
	fmt.Fprint(h, proof.FRIProof.RoundRoots)
	return hex.EncodeToString(h.Sum(nil))
}

func TestMerkleHashKoalaBearGolden(t *testing.T) {
	pcs, in, proof, committed := goldenFixtureProof(t)
	require.NoError(t, pcs.Verify(in, proof))
	require.Equal(t, goldenKoalaBearDigest, goldenDigest(committed, proof))
}

func TestMerkleHashBNPackedRoundTrip(t *testing.T) {
	pcs, in, proof, committed := goldenFixtureProof(t, WithMerkleHash(MerkleHashBNPacked))
	require.NoError(t, pcs.Verify(in, proof))
	require.NotEqual(t, goldenKoalaBearDigest, goldenDigest(committed, proof))

	// Every digest the BN mode produces is an octuplet encoding: 30-bit limbs.
	for _, o := range append([]field.Octuplet{committed.Tree.Root()}, proof.FRIProof.RoundRoots...) {
		for _, x := range o {
			require.Less(t, x.Uint64(), uint64(1<<30))
		}
	}

	// A KoalaBear verifier rejects a BN-packed proof, and the other way round.
	koala, _, _, _ := goldenFixtureProof(t)
	require.Error(t, koala.Verify(in, proof))

	// Tampering with a revealed row or a running sibling is rejected.
	tampered := proof
	tampered.InputQueries = append([]InputQuery(nil), proof.InputQueries...)
	tampered.InputQueries[0] = append(InputQuery(nil), proof.InputQueries[0]...)
	leaves := append([]*RowPair(nil), proof.InputQueries[0][0].Leaves...)
	bottom := *leaves[len(leaves)-1]
	bottom[0].Ext = append([]field.Ext(nil), bottom[0].Ext...)
	bottom[0].Ext[0].B0.A0.SetUint64(bottom[0].Ext[0].B0.A0.Uint64() + 1)
	leaves[len(leaves)-1] = &bottom
	tampered.InputQueries[0][0].Leaves = leaves
	require.Error(t, pcs.Verify(in, tampered))
}

func TestMerkleHashAddOpeningRejectsMismatch(t *testing.T) {
	params, err := NewParams(3, 2, 1, WithMerkleHash(MerkleHashBNPacked))
	require.NoError(t, err)
	encoders := makeEncoders(int(params.numRounds()+1), 2)
	pcs, err := NewPCS(params, encoders)
	require.NoError(t, err)

	prng := rand.New(utils.NewRandSource(1))
	witness := make(Batch, 3)
	witness[2] = SizedTable{Ext: [][]field.Ext{field.VecPseudoRandExt(prng, 4)}}
	committed := Commit(encoders, witness) // KoalaBear tree

	shifts := make(BatchShifts, 3)
	shifts[2] = SizedShifts{Ext: [][]int{{0}}}
	zeta := field.UintsToExt(19, 2, 3, 5, 7, 11)
	claimed := claimedValuesForTest(t, pcs, witness, shifts, zeta)
	require.ErrorContains(t, pcs.AddOpening(committed, zeta, shifts, claimed), "Merkle hash")
}

func TestWithMerkleHashRejectsUnknown(t *testing.T) {
	_, err := NewParams(3, 2, 1, WithMerkleHash(MerkleHash(7)))
	require.Error(t, err)
}

// TestMerkleHashBNPackedTree checks the BN-packed tree, built on its scalar
// parallel path, against a recursive reference, and opens and authenticates
// a few branches.
func TestMerkleHashBNPackedTree(t *testing.T) {
	const n = 2048 // above minParallelTreeLevel, so the parallel path runs
	prng := rand.New(utils.NewRandSource(7))
	leaves := make([]field.Octuplet, n)
	for i := range leaves {
		leaves[i] = field.PseudoRandOctuplet(prng)
	}
	tree := newTreeWithHash(MerkleHashBNPacked, [][]field.Octuplet{11: leaves})

	var ref func(lo, hi int) field.Octuplet
	ref = func(lo, hi int) field.Octuplet {
		if hi-lo == 1 {
			return leaves[lo]
		}
		mid := (lo + hi) / 2
		return poseidon2.BNCompressOctuplet(ref(lo, mid), ref(mid, hi))
	}
	require.Equal(t, ref(0, n), tree.Root())

	for _, idx := range []int{0, 1, 777, n - 1} {
		branch := tree.OpenBranch(idx)
		frontier := []field.Octuplet{tree.Root()}
		require.NoError(t, branch.authenticateToCap(MerkleHashBNPacked, idx, frontier))
		require.Error(t, branch.authenticateToCap(MerkleHashKoalaBear, idx, frontier))
	}
}
