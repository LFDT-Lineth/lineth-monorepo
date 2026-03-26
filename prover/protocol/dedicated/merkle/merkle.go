package merkle

import (
	"github.com/consensys/linea-monorepo/prover/crypto/state-management/smt_koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/parallel"
)

const (
	width      = 16
	blockSize  = 8
	limbPerU64 = 16
)

// Wizard gadget allowing to verify a Merkle proof
// See : https://github.com/consensys/linea-monorepo/issues/67

// The default function to be used in the self recursion and other places
func MerkleProofCheck(
	// compiled IOP
	comp *wizard.CompiledIOP,
	// name of the Merkle proof check instance
	name string,
	// depth of the tree
	depth, numProofs int,
	// column representing the proofs. If the number
	// of proof is a non-power of two, roots, leaves and pos
	// should be padded by zeros.
	pos ifaces.Column,
	proofs, roots, leaves [blockSize]ifaces.Column,
) {
	merkleProofCheck(comp, name, depth, numProofs, pos, proofs, roots, leaves, nil, nil, nil, false)
}

// The merkle proof check function with the reuse merkle proof check feature, used in the
// Accumulator module

func MerkleProofCheckWithReuse(
	// compiled IOP
	comp *wizard.CompiledIOP,
	// name of the Merkle proof check instance
	name string,
	// depth of the tree
	depth, numProofs int,
	// column representing the proofs. If the number
	// of proof is a non-power of two, all columns are padded
	// by zeros to the right so that the length becomes the next power of two.
	pos ifaces.Column,
	proofs, roots, leaves [blockSize]ifaces.Column,

	UseNextMerkleProof, IsActive, counter ifaces.Column,
) {
	merkleProofCheck(comp, name, depth, numProofs, pos, proofs, roots, leaves, UseNextMerkleProof, IsActive, counter, true)
}

type MerkleProofProverAction struct {
	Cm     *ComputeMod
	Leaves [blockSize]ifaces.Column
	Pos    ifaces.Column
}

func (a *MerkleProofProverAction) Run(run *wizard.ProverRuntime) {
	var leaves [blockSize]smartvectors.SmartVector
	for i := 0; i < blockSize; i++ {
		leaves[i] = a.Leaves[i].GetColAssignment(run)
	}
	pos := a.Pos.GetColAssignment(run)
	a.Cm.assign(run, pos, leaves)
}

func merkleProofCheck(
	// compiled IOP
	comp *wizard.CompiledIOP,
	// name of the Merkle proof check instance
	name string,
	// depth of the tree
	depth, numProofs int,
	// column representing the proofs. If the number
	// of proof is a non-power of two, all columns are padded
	// by zeros to the right so that the length becomes the next power of two.
	pos ifaces.Column,
	proofs, roots, leaves [blockSize]ifaces.Column,

	useNextMerkleProof, isActiveAccumulator, counter ifaces.Column,
	// variable indicating whether we want to check if the contiguous Merkle
	// proofs are from the same tree
	useNextProof bool,
) {

	round := column.MaxRound(proofs[0], roots[0], leaves[0], pos)
	// define the compute module
	cm := ComputeMod{}
	cm.Cols.Proof = proofs
	cm.WithOptProofReuseCheck = useNextProof
	if useNextProof {
		cm.Cols.UseNextMerkleProof = useNextMerkleProof
		cm.Cols.IsActiveAccumulator = isActiveAccumulator
	}
	cm.Define(comp, round, name, numProofs, depth)

	// define the result module
	rm := ResultMod{}
	rm.Roots = roots
	rm.Leaf = leaves
	rm.Pos = pos
	rm.withOptProofReuseCheck = useNextProof
	rm.Depth = depth

	rm.Define(comp, round, name, numProofs, depth, useNextMerkleProof, isActiveAccumulator, counter)

	// define the lookup relation

	// Build the including columns:
	// - Non-repeated: NewProof.Natural, PosAcc (appear once)
	// - Repeated per block: Curr[i], Root[i] (8 each)
	// Total: 2 + 8 + 8 = 18 columns
	includingCols := make([]ifaces.Column, 0, 2+2*blockSize)
	includingCols = append(includingCols, cm.Cols.NewProof.Natural, cm.Cols.PosAcc)
	for i := 0; i < blockSize; i++ {
		includingCols = append(includingCols, cm.Cols.Curr[i])
	}
	for i := 0; i < blockSize; i++ {
		includingCols = append(includingCols, cm.Cols.Root[i])
	}

	// Build the included columns:
	// - Non-repeated: IsActive, Pos (appear once)
	// - Repeated per block: Leaf[i], Roots[i] (8 each)
	// Total: 2 + 8 + 8 = 18 columns
	includedCols := make([]ifaces.Column, 0, 2+2*blockSize)
	includedCols = append(includedCols, rm.IsActive, rm.Pos)
	for i := 0; i < blockSize; i++ {
		includedCols = append(includedCols, rm.Leaf[i])
	}
	for i := 0; i < blockSize; i++ {
		includedCols = append(includedCols, rm.Roots[i])
	}

	// Single inclusion query with all columns stacked (no repeated columns)
	comp.InsertInclusion(
		round,
		ifaces.QueryIDf("MERKLE_MODULE_LOOKUP_%v", name),
		includingCols,
		includedCols,
	)

	// define the optional lookup relation for columns coming from the accumulator module
	// The first lookup column act as a filter and select the last row of a segment in the
	// computed mode.
	if useNextProof {
		comp.InsertInclusion(round,
			ifaces.QueryIDf("MERKLE_MODULE_LOOKUP_FOR_USE_NEXT_PROOF_%v", name),
			[]ifaces.Column{cm.Cols.NewProof.Natural, cm.Cols.UseNextMerkleProofExpanded, cm.Cols.IsActiveExpanded, cm.Cols.SegmentCounter},
			[]ifaces.Column{rm.IsActive, rm.UseNextMerkleProof, rm.IsActive, rm.Counter},
		)
	}

	// assigns the compute module
	comp.RegisterProverAction(round, &MerkleProofProverAction{
		Cm:     &cm,
		Leaves: leaves,
		Pos:    pos,
	})
}

// pack a list of merkle-proofs into a single vector
func PackMerkleProofs(proofs []smt_koalabear.Proof) [blockSize]smartvectors.SmartVector {

	numProofs := len(proofs)
	depth := len(proofs[0].Siblings)
	numRows := utils.NextPowerOfTwo(numProofs * depth)

	res := [blockSize][]field.Element{}
	for i := 0; i < blockSize; i++ {
		res[i] = make([]field.Element, numProofs*depth)
	}

	numProofWritten := 0

	for i := range proofs {
		for j := range proofs[i].Siblings {
			// assertion, all proofs have the assumed depth
			if len(proofs[i].Siblings) != depth {
				utils.Panic("expected depth %v, got %v", depth, len(proofs[i].Siblings))
			}
			proofentry := proofs[i].Siblings[depth-j-1]
			for coord := range res {
				res[coord][numProofWritten*depth+j] = proofentry[coord]
			}
		}
		numProofWritten++
	}
	resSV := [blockSize]smartvectors.SmartVector{}
	for i := range res {
		resSV[i] = smartvectors.RightZeroPadded(res[i], numRows)
	}

	return resSV
}
func Transpose(vecOct []field.Octuplet) [][]field.Element {
	n := len(vecOct)

	// Pre-allocate all column data
	columns := make([][]field.Element, 8)
	for i := range columns {
		columns[i] = make([]field.Element, n)
	}

	// Fill columns in parallel
	parallel.Execute(n, func(start, end int) {
		for row := start; row < end; row++ {
			for col := 0; col < 8; col++ {
				columns[col][row] = vecOct[row][col]
			}
		}
	})
	return columns

}
