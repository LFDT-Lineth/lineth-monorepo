package vortex_bls12377

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/crypto/state-management/smt_bls12377"
	"github.com/consensys/linea-monorepo/prover/crypto/vortex"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
)

func getProofVortexNCommitmentsWithMerkle(t *testing.T, nCommitments, nbPolys, polySize, rate int, WithSis []bool) (
	*Params,
	*vortex.OpeningProof,
	vortex.VerifierInput,
	[]Commitment,
	[][]smt_bls12377.Proof,
) {

	var vi vortex.VerifierInput
	vi.X = fext.RandomElement()
	vi.Alpha = fext.RandomElement()
	vi.EntryList = []int{1, 5, 19, 645}
	vi.Ys = make([][]fext.Element, nCommitments)

	polyLists := make([][]smartvectors.SmartVector, nCommitments)
	for i := range polyLists {
		polys := make([]smartvectors.SmartVector, nbPolys)
		ys := make([]fext.Element, nbPolys)
		for j := range polys {
			polys[j] = smartvectors.Rand(polySize)
			ys[j] = smartvectors.EvaluateBasePolyLagrange(polys[j], vi.X)
		}
		polyLists[i] = polys
		vi.Ys[i] = ys
	}

	// Commits to it
	commitments := make([]Commitment, nCommitments)
	trees := make([]*smt_bls12377.Tree, nCommitments)
	logTwoDegree := 9
	logTwoBound := 16
	vortexInstance := NewParams(
		rate,
		polySize,
		nbPolys*nCommitments,
		logTwoDegree,
		logTwoBound)

	encodedMatrices := make([]EncodedMatrix, nCommitments)
	for j := range commitments {
		if WithSis[j] {
			encodedMatrices[j], commitments[j], trees[j], _ = vortexInstance.CommitMerkleWithSIS(polyLists[j])
		} else {
			encodedMatrices[j], commitments[j], trees[j], _ = vortexInstance.CommitMerkleWithoutSIS(polyLists[j])
		}
	}

	// Generate the proof
	proof, merkleProofs := Prove(vi.EntryList, encodedMatrices, trees, vi.Alpha)

	return &vortexInstance, proof, vi, commitments, merkleProofs
}

func TestVerifier(t *testing.T) {

	nCommitments := 4
	nbPolys := 15
	polySize := 1 << 10
	rate := 2
	WithSis := make([]bool, nCommitments)

	WithSis[0] = false
	WithSis[1] = true
	WithSis[2] = false
	WithSis[3] = false

	params, proof, vi, commitments, merkleProofs := getProofVortexNCommitmentsWithMerkle(t, nCommitments, nbPolys, polySize, rate, WithSis)

	err := Verify(params, proof, &vi, commitments, merkleProofs, WithSis)
	if err != nil {
		t.Fatal(err)
	}

}
