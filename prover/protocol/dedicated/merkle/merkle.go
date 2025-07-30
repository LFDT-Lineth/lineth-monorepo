package merkle

import (
	"github.com/consensys/linea-monorepo/prover/crypto/state-management/smt"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
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
	proofs, roots, leaves, pos ifaces.Column,
) {
	merkleProofCheck(comp, name, depth, numProofs, proofs, roots, leaves, pos, nil, nil, nil, false)
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
	proofs, roots, leaves, pos, UseNextMerkleProof, IsActive, counter ifaces.Column,
) {
	merkleProofCheck(comp, name, depth, numProofs, proofs, roots, leaves, pos, UseNextMerkleProof, IsActive, counter, true)
}

type MerkleProofProverAction struct {
	Cm     *ComputeMod
	Leaves ifaces.Column
	Pos    ifaces.Column
}

func (a *MerkleProofProverAction) Run(run *wizard.ProverRuntime) {
	leaves := a.Leaves.GetColAssignment(run)
	pos := a.Pos.GetColAssignment(run)
	a.Cm.assign(run, leaves, pos)
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
	proofs, roots, leaves, pos, useNextMerkleProof, isActiveAccumulator, counter ifaces.Column,
	// variable indicating whether we want to check if the contiguous Merkle
	// proofs are from the same tree
	useNextProof bool,
) {

	round := column.MaxRound(proofs, roots, leaves, pos)
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
	comp.InsertInclusion(
		round,
		ifaces.QueryIDf("MERKLE_MODULE_LOOKUP_%v", name),
		[]ifaces.Column{cm.Cols.NewProof.Natural, cm.Cols.Curr, cm.Cols.PosAcc, cm.Cols.Root},
		[]ifaces.Column{rm.IsActive, rm.Leaf, rm.Pos, rm.Roots},
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
func PackMerkleProofs(proofs []smt.Proof) smartvectors.SmartVector {

	numProofs := len(proofs)
	depth := len(proofs[0].Siblings)
	numRows := utils.NextPowerOfTwo(numProofs * depth)

	res := make([]field.Element, 0, numProofs*depth)
	for i := range proofs {
		for j := range proofs[i].Siblings {
			// assertion, all proofs have the assumed depth
			if len(proofs[i].Siblings) != depth {
				utils.Panic("expected depth %v, got %v", depth, len(proofs[i].Siblings))
			}
			proofentry := proofs[i].Siblings[depth-j-1]
			var x field.Element
			x.SetBytes(proofentry[:])
			res = append(res, x)
		}
	}

	return smartvectors.RightZeroPadded(res, numRows)
}
