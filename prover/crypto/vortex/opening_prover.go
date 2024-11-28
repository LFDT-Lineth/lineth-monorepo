package vortex

import (
	"github.com/consensys/linea-monorepo/prover/crypto/state-management/smt"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/parallel"
)

// OpeningProof represents an opening proof for a Vortex commitment. The proof
// possibly relates to several commitments simultaneously. This corresponds to
// the batch settings.
type OpeningProof struct {
	// Columns against which the linear combination is checked (the i-th
	// entry is the EntryList[i]-th column). The columns may as well be
	// dispatched in several matrices. Columns [i][j][k] returns the k-th entry
	// of the j-th selected column of the i-th commitment
	Columns [][][]field.Element

	// Linear combination of the Reed-Solomon encoded polynomials to open.
	LinearCombination smartvectors.SmartVector

	// MerkleProofs store a list of [smt.Proof] (Merkle proofs) allegedly
	// attesting the membership of the columns in the commitment tree.
	//
	// MerkleProofs[i][j] corresponds to the Merkle proof attesting the j-th
	// column of the i-th commitment root hash.
	MerkleProofs [][]smt.Proof
}

// InitOpeningWithLC initiates the construction of a Vortex proof by returning the
// encoding of the linear combinations of the committed row-vectors contained
// in committedSV by the successive powers of randomCoin.
//
// The returned proof is partially assigned and must be completed using
// [WithEntryList] to conclude the opening protocol.
//
// In the batch settings, the committedSV must be provided as the flattened
// list of the committed matrices. This contrasts with the API of the other
// functions and is motivated by the fact that this is simpler to construct in
// our settings.
func (params *Params) InitOpeningWithLC(committedSV []smartvectors.SmartVector, randomCoin field.Element) *OpeningProof {
	proof := OpeningProof{}

	if len(committedSV) == 0 {
		utils.Panic("attempted to open an empty witness")
	}

	// Compute the linear combination
	linComb := make([]field.Element, params.NbColumns)

	parallel.ExecuteChunky(len(linComb), func(start, stop int) {
		subTask := make([]smartvectors.SmartVector, 0, len(committedSV))
		for i := range committedSV {
			subTask = append(subTask, committedSV[i].SubVector(start, stop))
		}
		// Collect the result in the larger slice at the end

		subResult := smartvectors.PolyEval(subTask, randomCoin)
		subResult.WriteInSlice(linComb[start:stop])
	})

	linCombSV := smartvectors.NewRegular(linComb)
	proof.LinearCombination = params.rsEncode(linCombSV, nil)
	return &proof
}

// Complete completes the proof adding the columns pointed by entryList
// (implictly the random positions pointed to by the verifier).
func (proof *OpeningProof) Complete(
	entryList []int,
	committedMatrices []EncodedMatrix,
	trees []*smt.Tree,
) {

	if len(entryList) == 0 {
		utils.Panic("empty entry list")
	}

	selectedColumns := make([][][]field.Element, len(committedMatrices))

	for i := range committedMatrices {
		selectedColumns[i] = make([][]field.Element, len(entryList))
		for j := range entryList {
			col := make([]field.Element, len(committedMatrices[i]))
			for k := range committedMatrices[i] {
				col[k] = committedMatrices[i][k].Get(entryList[j])
			}
			selectedColumns[i][j] = col
		}
	}

	numTrees := len(trees)
	proofs := make([][]smt.Proof, numTrees)

	// Generate the proofs for each tree and each entry
	for treeID, tree := range trees {
		proofs[treeID] = make([]smt.Proof, len(entryList))
		for k, entry := range entryList {
			var err error
			proofs[treeID][k], err = tree.Prove(entry)
			if err != nil {
				utils.Panic("invalid entry leaf: %v", err.Error())
			}
		}
	}

	proof.MerkleProofs = proofs
	proof.Columns = selectedColumns
}
