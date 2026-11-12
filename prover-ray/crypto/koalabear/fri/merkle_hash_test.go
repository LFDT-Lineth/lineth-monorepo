package fri

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/poseidon2"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/stretchr/testify/require"
)

// goldenKoalaBearDigest pins the KoalaBear-mode Merkle outputs of
// goldenFixtureProof to the values produced before MerkleHash existed, so that
// the default mode stays bit-identical.
const goldenKoalaBearDigest = "963960fee3211c8e4b8da5bd1d362586acdf461a9d2d91869de2db6812abd9c6"

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
	// Hash the canonical values explicitly: fmt's rendering of field elements
	// depends on the gnark-crypto version.
	h := sha256.New()
	var buf [8]byte
	for _, o := range append([]field.Octuplet{committed.Tree.Root()}, proof.FRIProof.RoundRoots...) {
		for _, x := range o {
			binary.BigEndian.PutUint64(buf[:], x.Uint64())
			h.Write(buf[:])
		}
	}
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

// TestPCSVerifyGnarkBNPacked runs the in-circuit verifier in BN-packed mode
// against the native one, over cap heights and a FixedDomain PCS (whose top
// rounds commit zero-layer trees), and checks tampering is rejected.
func TestPCSVerifyGnarkBNPacked(t *testing.T) {
	bn := WithMerkleHash(MerkleHashBNPacked)
	cases := []struct {
		capHeight, extraRounds uint8
	}{{0, 0}, {1, 0}, {pcsCircuitCapHeight, 0}, {pcsCircuitCapHeight, 1}}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("cap=%d/extra=%d", tc.capHeight, tc.extraRounds), func(t *testing.T) {
			fx := newPCSCircuitFixtureAt(t, tc.capHeight, tc.extraRounds, bn)
			template, assignment := circuitFor(fx, fx.proof)
			require.NoError(t, solvePCSCircuit(t, template, assignment))

			// a running-layer sibling and an input-tree sibling, each flipped
			tampered := fx.proof
			tampered.FRIProof.RunningQueries = append([]RunningQuery(nil), fx.proof.FRIProof.RunningQueries...)
			tampered.FRIProof.RunningQueries[0] = append(RunningQuery(nil), fx.proof.FRIProof.RunningQueries[0]...)
			br := tampered.FRIProof.RunningQueries[0][0]
			br.Siblings = append([]field.Octuplet(nil), br.Siblings...)
			br.Siblings[0][3].SetUint64(br.Siblings[0][3].Uint64() ^ 1)
			tampered.FRIProof.RunningQueries[0][0] = br
			require.Error(t, fx.pcs.Verify(fx.input, tampered))
			_, bad := circuitFor(fx, tampered)
			require.Error(t, solvePCSCircuit(t, template, bad))
		})
	}
}

// TestPCSVerifyGnarkConstraintsByHash prints the PLONK constraint count of the
// whole in-circuit PCS verifier in both modes on the same fixture.
func TestPCSVerifyGnarkConstraintsByHash(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles circuits")
	}
	count := func(opts ...Option) int {
		fx := newPCSCircuitFixtureAt(t, pcsCircuitCapHeight, 0, opts...)
		template, _ := circuitFor(fx, fx.proof)
		ccs, err := frontend.Compile(ecc.BN254.ScalarField(), scs.NewBuilder, template)
		require.NoError(t, err)
		return ccs.GetNbConstraints()
	}
	koala, bn := count(), count(WithMerkleHash(MerkleHashBNPacked))
	t.Logf("PCS verifier: koalabear=%d  bn-packed=%d  (%.2fx)", koala, bn, float64(koala)/float64(bn))
}
