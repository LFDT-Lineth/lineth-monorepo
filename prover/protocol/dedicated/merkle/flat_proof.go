package merkle

import (
	"github.com/consensys/linea-monorepo/prover/crypto/state-management/smt_koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils/types"
)

// FlatProof is a collection of columns representing a Merkle proof in a flat
// manner.
type FlatProof struct {
	Nodes [blockSize][]ifaces.Column
}

// Depth returns the depth of the tree represented by the proof
func (p *FlatProof) Depth() int {
	return len(p.Nodes[0])
}

// NumRow returns the number of rows of the [FlatProof]
func (p *FlatProof) NumRow() int {
	return p.Nodes[0][0].Size()
}

// NewProof instantiates a new [FlatProof] object in the wizard
func NewProof(comp *wizard.CompiledIOP, round int, name string, depth, numRows int) *FlatProof {
	proof := &FlatProof{}
	for i := 0; i < depth; i++ {
		for j := 0; j < blockSize; j++ {
			node := comp.InsertCommit(round, ifaces.ColIDf("%v_NODE_%v_%v", name, i, j), numRows, true)
			proof.Nodes[j] = append(proof.Nodes[j], node)
		}
	}
	return proof
}

// WithStatus changes the status of all the columns of the [FlatProof] to
// "status" and returns a pointer to the receiver.
func (p *FlatProof) WithStatus(comp *wizard.CompiledIOP, status column.Status) *FlatProof {
	for j := 0; j < blockSize; j++ {
		for i := range p.Nodes[j] {
			comp.Columns.SetStatus(p.Nodes[j][i].GetColID(), status)
		}
	}
	return p
}

// Assign assigns the proof columns to the [FlatProof] object from a list of
// [merkletree.Proof] objects. The columns are zero-padded on the right.
// Each leaf in the proof is converted to a [8]field.Element and then appended to assignment
func (p *FlatProof) Assign(run *wizard.ProverRuntime, proofs []smt_koalabear.Proof) {

	var assignment [blockSize][][]field.Element

	for i := 0; i < blockSize; i++ {
		assignment[i] = make([][]field.Element, p.Depth())

	}
	for i := range proofs {
		for j := range proofs[i].Siblings {
			nodeAsFr := proofs[i].Siblings[j]
			for k := 0; k < blockSize; k++ {
				assignment[k][j] = append(assignment[k][j], nodeAsFr[k])
			}
		}
	}

	for j := 0; j < blockSize; j++ {
		for i := range p.Nodes[j] {
			run.AssignColumn(p.Nodes[j][i].GetColID(), smartvectors.RightZeroPadded(assignment[j][i], p.NumRow()))
		}
	}
}

// Unpack reads the assignment of a proof and returns it as a list of
// [smt.Proof] objects. The function assumes that the columns are all
// padded in the same fashion.
//
// The function also takes as additional input a smart-vector containing
// the positions corresponding for each proofs.
// Every [8]field.Element is converted back to a leaf in the Siblings
func (p *FlatProof) Unpack(run ifaces.Runtime, pos smartvectors.SmartVector) []smt_koalabear.Proof {

	var (
		proofs = make([]smt_koalabear.Proof, 0)
		// The assumption here is two-fold: first, this relies on the
		// fact that we know all the columns are structured and padded the same
		start, stop = smartvectors.CoWindowRange(p.Nodes[0][0].GetColAssignment(run))
	)

	for i := start; i <= stop; i++ {

		newProof := smt_koalabear.Proof{
			Path:     field.ToInt(pos.GetPtr(i)),
			Siblings: make([]types.KoalaOctuplet, len(p.Nodes[0])),
		}

		for n := range p.Nodes[0] {
			var node [blockSize]field.Element
			for j := 0; j < blockSize; j++ {
				node[j] = p.Nodes[j][n].GetColAssignmentAt(run, i)
			}
			newProof.Siblings[n] = node
		}

		proofs = append(proofs, newProof)
	}

	return proofs
}

// // UnpackGnark unpacks the proof into a list of [smt.GnarkProof] objects. The
// // function also takes a list of positions to use to fill the [Path] field
// // of the proof.
// func (p *FlatProof) UnpackGnark(run ifaces.GnarkRuntime, entryList []frontend.Variable) []smt_bls12377.GnarkProof {

// 	var (
// 		proofs   = make([]smt_bls12377.GnarkProof, 0)
// 		nbProofs = len(entryList)
// 	)

// 	for i := 0; i < nbProofs; i++ {

// 		newProof := smt_bls12377.GnarkProof{
// 			Path:     entryList[i],
// 			Siblings: make([]frontend.Variable, len(p.Nodes)),
// 		}

// 		for j := range p.Nodes[0] {
// 			var node [blockSize]koalagnark.Var

// 			for k := 0; k < blockSize; k++ {
// 				node[k] = p.Nodes[k][j].GetColAssignmentGnarkAt(run, i)
// 			}
// 			// TODO @thomas fixme
// 			newProof.Siblings[j] = encoding.Encode8WVsToFV(api, node)
// 		}

// 		proofs = append(proofs, newProof)
// 	}

// 	return proofs
// }
