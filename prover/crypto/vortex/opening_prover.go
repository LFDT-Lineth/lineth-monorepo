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

	if len(committedSV) == 0 {
		utils.Panic("attempted to open an empty witness")
	}

	// Compute the linear combination
	linComb := make([]field.Element, params.NbColumns)

	parallel.Execute(len(linComb), func(start, stop int) {

		x := field.One()
		scratch := make(field.Vector, stop-start)
		localLinComb := make(field.Vector, stop-start)
		for i := range committedSV {
			_sv := committedSV[i]
			// we distinguish the case of a regular vector and constant to avoid
			// unnecessary allocations and copies
			switch _svt := _sv.(type) {
			case *smartvectors.Constant:
				cst := _svt.Value
				cst.Mul(&cst, &x)
				for j := range localLinComb {
					localLinComb[j].Add(&localLinComb[j], &cst)
				}
				x.Mul(&x, &randomCoin)
				continue
			default:
				sv := _svt.SubVector(start, stop)
				sv.WriteInSlice(scratch)
			}
			scratch.ScalarMul(scratch, &x)
			localLinComb.Add(localLinComb, scratch)
			x.Mul(&x, &randomCoin)

		}
		copy(linComb[start:stop], localLinComb)
	})

	linCombSV := smartvectors.NewRegular(linComb)

	return &OpeningProof{
		LinearCombination: params.rsEncode(linCombSV),
	}
}

// InitOpeningFromAlreadyEncodedLC initiates the construction of a Vortex proof
// by returning the encoding of the linear combinations of the committed
// row-vectors contained in committedSV by the successive powers of randomCoin.
//
// The returned proof is partially assigned and must be completed using
// [WithEntryList] to conclude the opening protocol.
func (params *Params) InitOpeningFromAlreadyEncodedLC(rsCommittedSV EncodedMatrix, randomCoin field.Element) *OpeningProof {

	if len(rsCommittedSV) == 0 {
		utils.Panic("attempted to open an empty witness")
	}

	// Compute the linear combination
	linComb := make([]field.Element, params.NumEncodedCols())

	parallel.ExecuteChunky(len(linComb), func(start, stop int) {
		subTask := make([]smartvectors.SmartVector, 0, len(rsCommittedSV))
		for i := range rsCommittedSV {
			subTask = append(subTask, rsCommittedSV[i].SubVector(start, stop))
		}

		// Collect the result in the larger slice at the end
		subResult := smartvectors.PolyEval(subTask, randomCoin)
		subResult.WriteInSlice(linComb[start:stop])
	})

	return &OpeningProof{
		LinearCombination: smartvectors.NewRegular(linComb),
	}
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

	proof.MerkleProofs = proofs
	proof.Columns = selectedColumns
}
