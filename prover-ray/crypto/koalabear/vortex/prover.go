package vortex

import (
	smt "github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/smt"
	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/linea-monorepo/prover-ray/utils"
)

// Prove constructs an opening proof for the given encoded matrices at the positions in entryList.
//
// The linear combination U_alpha is sent in coefficient form (T elements)
// rather than evaluation form (T × blowup elements). It is computed from the
// T-length source polynomials (cheaper than from the N-length encoded matrices)
// and then converted to monomial coefficients via the T-point IFFT.
func Prove(
	params *Params,
	entryList []int,
	polys [][][]field.Element,
	encodedMatrices []EncodedMatrix,
	trees []*smt.Tree, alpha field.Ext) (*OpeningProof, [][]smt.Proof) {

	proof := &OpeningProof{}

	totalPolys := 0
	for _, pl := range polys {
		totalPolys += len(pl)
	}
	allPolys := make([][]field.Element, 0, totalPolys)
	for _, pl := range polys {
		allPolys = append(allPolys, pl...)
	}
	LinearCombination(proof, params.RsParams.Domains[0], allPolys, alpha)

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
