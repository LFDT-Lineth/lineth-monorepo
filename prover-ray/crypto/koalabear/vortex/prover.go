package vortex

import (
	smt "github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/smt"
	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/linea-monorepo/prover-ray/utils"
)

// Prove constructs an opening proof for the given encoded matrices at the positions in entryList.
func Prove(
	entryList []int,
	encodedMatrices []EncodedMatrix,
	trees []*smt.Tree, alpha field.Ext) (*OpeningProof, [][]smt.Proof) {

	proof := &OpeningProof{}

	_encodedMatrices := make([][]field.Element, 0, len(encodedMatrices))
	for _, m := range encodedMatrices {
		_encodedMatrices = append(_encodedMatrices, m...)
	}
	LinearCombination(proof, _encodedMatrices, alpha)

	merkleProofs := SelectColumnsAndMerkleProofs(proof, entryList, encodedMatrices, trees)

	return proof, merkleProofs
}

// SelectColumnsAndMerkleProofs completes the proof adding the columns pointed by entryList
// (implictly the random positions pointed to by the verifier).
func SelectColumnsAndMerkleProofs(
	proof *OpeningProof,
	entryList []int,
	committedMatrices []EncodedMatrix,
	trees []*smt.Tree,
) [][]smt.Proof {

	if len(entryList) == 0 {
		utils.Panic("empty entry list")
	}

	proof.Columns = make([][][]field.Element, len(committedMatrices))

	for i := range committedMatrices {
		proof.Columns[i] = make([][]field.Element, len(entryList))
		for j := range entryList {
			col := make([]field.Element, len(committedMatrices[i]))
			for k := range committedMatrices[i] {
				col[k] = committedMatrices[i][k][entryList[j]]
			}
			proof.Columns[i][j] = col
		}
	}

	numTrees := len(trees)
	proofs := make([][]smt.Proof, numTrees)

	// Generate the proofs for each tree and each entry
	for treeID, tree := range trees {
		if tree == nil {
			utils.Panic("tree is nil")
		}
		proofs[treeID] = make([]smt.Proof, len(entryList))
		for k, entry := range entryList {
			var err error
			proofs[treeID][k], err = tree.Prove(entry)
			if err != nil {
				utils.Panic("invalid entry leaf: %v", err.Error())
			}
		}
	}

	return proofs
}
