package merkle

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/bits"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/mimc"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/symbolic"
)

// FlatProofVerificationInputs collects parameters helpful for defining a Merkle-tree verification
// wizard.
type FlatProofVerificationInputs struct {
	// Name is the name of the Merkle tree verification instance
	Name string
	// Proof are the columns reserved for storing the Merkle proofs
	Proof FlatProof
	// Leaf contains the alleged leaves
	Leaf ifaces.Column
	// Roots contains the Merkle roots
	Roots ifaces.Column
	// Position contains the positions of the alleged leaves
	Position ifaces.Column
	// Use for looking up and selecting only the the columns containing the
	// root in the ComputeMod.
	IsActive ifaces.Column
}

// FlatMerkleProofVerification represents a Merkle-tree verification wizard
// instance. It contains all the elements
type FlatMerkleProofVerification struct {
	// FlatProofVerificationInputs provides all the parameters needed to define the Merkle tree
	FlatProofVerificationInputs
	// PosBits contains the bit decomposition of the position
	PosBits *bits.BitDecomposed
	// Lefts and Rights respectively represent the left and the right
	// children of the current node being hashed. They are constructed
	// as ternaries
	Lefts, Rights []*dedicated.TernaryCtx
	// Node contains the hash of nodes, obtained by hashing the left and
	// right.
	Nodes []*mimc.HashingCtx
}

// CheckFlatMerkleProofs checks a list of Merkle proof using an horizontal
// wizard arithmetization.
func CheckFlatMerkleProofs(comp *wizard.CompiledIOP, inputs FlatProofVerificationInputs) *FlatMerkleProofVerification {

	if err := checkColumnsAllHaveSameSize(&inputs); err != nil {
		panic(err)
	}

	ctx := &FlatMerkleProofVerification{
		FlatProofVerificationInputs: inputs,
		PosBits:                     bits.BitDecompose(comp, inputs.Position, len(inputs.Proof.Nodes)),
	}

	prevNode := inputs.Leaf

	for i := range inputs.Proof.Nodes {

		var (
			left  = dedicated.Ternary(comp, ctx.PosBits.Bits[i], inputs.Proof.Nodes[i], prevNode)
			right = dedicated.Ternary(comp, ctx.PosBits.Bits[i], prevNode, inputs.Proof.Nodes[i])
			node  = mimc.HashOf(comp, []ifaces.Column{left.Result, right.Result})
		)

		prevNode = node.Result()
		ctx.Lefts = append(ctx.Lefts, left)
		ctx.Rights = append(ctx.Rights, right)
		ctx.Nodes = append(ctx.Nodes, node)
	}

	// This check ensures that the computed and the provided root match. Note
	// that prevNode is the last node computed, hence the root.
	comp.InsertGlobal(
		max(prevNode.Round(), inputs.Roots.Round()),
		ifaces.QueryIDf("%v_ROOT_MATCH", inputs.Name),
		symbolic.Mul(inputs.IsActive, symbolic.Sub(prevNode, inputs.Roots)),
	)

	return ctx
}

// AddProofReuseConstraint adds a constraint on the reuse of the same proof
// across different rows of the Merkle proof verification module.
func (ctx *FlatMerkleProofVerification) AddProofReuseConstraint(comp *wizard.CompiledIOP, mustReuseForNext ifaces.Column) {

	for i := range ctx.Proof.Nodes {
		comp.InsertGlobal(
			max(mustReuseForNext.Round(), ctx.Proof.Nodes[0].Round()),
			ifaces.QueryIDf("%v_PROOF_REUSE_%v", ctx.FlatProofVerificationInputs.Name, i),
			symbolic.Mul(
				mustReuseForNext,
				symbolic.Sub(
					ctx.Proof.Nodes[i],
					column.Shift(ctx.Proof.Nodes[i], 1),
				),
			),
		)
	}
}

// Run implements the [wizard.ProverAction] interface and implements all the
// steps of the Merkle tree verification. It assigns node after node, all the
// columns needed to complete the Merkle tree verification from the leaf up
// to the root.
func (ctx *FlatMerkleProofVerification) Run(run *wizard.ProverRuntime) {

	ctx.PosBits.Run(run)

	for i := range ctx.PosBits.Bits {

		ctx.Lefts[i].Run(run)
		ctx.Rights[i].Run(run)
		ctx.Nodes[i].Run(run)
	}
}

// checkColumnsAllHaveSameSize checks if all the columns have the same size
// and returns an error if they don't.
func checkColumnsAllHaveSameSize(inp *FlatProofVerificationInputs) error {

	size := inp.Roots.Size()
	for _, node := range inp.Proof.Nodes {
		if node.Size() != size {
			return fmt.Errorf("all nodes must have the same size: root=%v proof=%v", size, node.Size())
		}
	}

	if inp.Leaf.Size() != size {
		return fmt.Errorf("all nodes must have the same size: root=%v leaf=%v", size, inp.Leaf.Size())
	}

	if inp.Position.Size() != size {
		return fmt.Errorf("all nodes must have the same size: root=%v position=%v", size, inp.Position.Size())
	}

	if inp.IsActive.Size() != size {
		return fmt.Errorf("all nodes must have the same size: root=%v isActive=%v", size, inp.IsActive.Size())
	}

	return nil
}
