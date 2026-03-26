package vortex_koalabear

import (
	"github.com/consensys/linea-monorepo/prover/crypto/state-management/smt_koalabear"
	"github.com/consensys/linea-monorepo/prover/crypto/vortex"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/utils"
)

func Prove(
	entryList []int,
	encodedMatrices []EncodedMatrix,
	trees []*smt_koalabear.Tree, alpha fext.Element) (*vortex.OpeningProof, [][]smt_koalabear.Proof) {

	proof := &vortex.OpeningProof{}

	_encodedMatrices := make([]smartvectors.SmartVector, 0, len(encodedMatrices))
	for _, m := range encodedMatrices {
		_encodedMatrices = append(_encodedMatrices, m...)
	}
	vortex.LinearCombination(proof, _encodedMatrices, alpha)

	merkleProofs := SelectColumnsAndMerkleProofs(proof, entryList, encodedMatrices, trees)

	return proof, merkleProofs
}

// SelectColumnsAndMerkleProofs completes the proof adding the columns pointed by entryList
// (implictly the random positions pointed to by the verifier).
func SelectColumnsAndMerkleProofs(
	proof *vortex.OpeningProof,
	entryList []int,
	committedMatrices []EncodedMatrix,
	trees []*smt_koalabear.Tree,
) [][]smt_koalabear.Proof {

	if len(entryList) == 0 {
		utils.Panic("empty entry list")
	}

	proof.Columns = make([][][]field.Element, len(committedMatrices))

	for i := range committedMatrices {
		proof.Columns[i] = make([][]field.Element, len(entryList))
		for j := range entryList {
			col := make([]field.Element, len(committedMatrices[i]))
			for k := range committedMatrices[i] {
				col[k] = committedMatrices[i][k].Get(entryList[j])
			}
			proof.Columns[i][j] = col
		}
	}

	numTrees := len(trees)
	proofs := make([][]smt_koalabear.Proof, numTrees)

	// Generate the proofs for each tree and each entry
	for treeID, tree := range trees {
		if tree == nil {
			utils.Panic("tree is nil")
		}
		proofs[treeID] = make([]smt_koalabear.Proof, len(entryList))
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
