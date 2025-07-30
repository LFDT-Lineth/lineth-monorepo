package merkle

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/crypto/state-management/smt"
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
	Nodes []ifaces.Column
}

// Depth returns the depth of the tree represented by the proof
func (p *FlatProof) Depth() int {
	return len(p.Nodes)
}

// NumRow returns the number of rows of the [FlatProof]
func (p *FlatProof) NumRow() int {
	return p.Nodes[0].Size()
}

// NewProof instantiates a new [FlatProof] object in the wizard
func NewProof(comp *wizard.CompiledIOP, round int, name string, depth, numRows int) *FlatProof {
	proof := &FlatProof{}
	for i := 0; i < depth; i++ {
		node := comp.InsertCommit(round, ifaces.ColIDf("%v_NODE_%v", name, i), numRows)
		proof.Nodes = append(proof.Nodes, node)
	}
	return proof
}

// WithStatus changes the status of all the columns of the [FlatProof] to
// "status" and returns a pointer to the receiver.
func (p *FlatProof) WithStatus(comp *wizard.CompiledIOP, status column.Status) *FlatProof {
	for i := range p.Nodes {
		comp.Columns.SetStatus(p.Nodes[i].GetColID(), status)
	}
	return p
}

// Assign assigns the proof columns to the [FlatProof] object from a list of
// [merkletree.Proof] objects. The columns are zero-padded on the right.
func (p *FlatProof) Assign(run *wizard.ProverRuntime, proofs []smt.Proof) {

	assignment := make([][]field.Element, p.Depth())

	for i := range proofs {
		for j := range proofs[i].Siblings {
			var nodeAsFr field.Element
			nodeAsFr.SetBytes(proofs[i].Siblings[j][:])
			assignment[j] = append(assignment[j], nodeAsFr)
		}
	}

	for i := range p.Nodes {
		run.AssignColumn(p.Nodes[i].GetColID(), smartvectors.RightZeroPadded(assignment[i], p.NumRow()))
	}
}

// Unpack reads the assignment of a proof and returns it as a list of
// [smt.Proof] objects. The function assumes that the columns are all
// padded in the same fashion.
//
// The function also takes as additional input a smart-vector containing
// the positions corresponding for each proofs.
func (p *FlatProof) Unpack(run ifaces.Runtime, pos smartvectors.SmartVector) []smt.Proof {

	var (
		proofs = make([]smt.Proof, 0)
		// The assumption here is two-fold: first, this relies on the
		// fact that we know all the columns are structured and padded the same
		start, stop = smartvectors.CoWindowRange(p.Nodes[0].GetColAssignment(run))
	)

	for i := start; i <= stop; i++ {

		newProof := smt.Proof{
			Path:     field.ToInt(pos.GetPtr(i)),
			Siblings: make([]types.Bytes32, len(p.Nodes)),
		}

		for n := range p.Nodes {
			node := p.Nodes[n].GetColAssignmentAt(run, i)
			newProof.Siblings[n].SetField(node)
		}

		proofs = append(proofs, newProof)
	}

	return proofs
}

// UnpackGnark unpacks the proof into a list of [smt.GnarkProof] objects. The
// function also takes a list of positions to use to fill the [Path] field
// of the proof.
func (p *FlatProof) UnpackGnark(run ifaces.GnarkRuntime, entryList []frontend.Variable) []smt.GnarkProof {

	var (
		proofs   = make([]smt.GnarkProof, 0)
		nbProofs = len(entryList)
	)

	for i := 0; i < nbProofs; i++ {

		newProof := smt.GnarkProof{
			Path:     entryList[i],
			Siblings: make([]frontend.Variable, len(p.Nodes)),
		}

		for j := range p.Nodes {
			node := p.Nodes[i].GetColAssignmentGnarkAt(run, i)
			newProof.Siblings[j] = node
		}

		proofs = append(proofs, newProof)
	}

	return proofs
}
