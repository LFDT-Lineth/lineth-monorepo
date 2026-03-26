package merkle

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/bits"
	poseidon2 "github.com/consensys/linea-monorepo/prover/protocol/dedicated/poseidon2"
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
	Leaf [blockSize]ifaces.Column
	// Roots contains the Merkle roots
	Roots [blockSize]ifaces.Column
	// Position contains the positions of the alleged leaves
	Position [limbPerU64]ifaces.Column
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
	Lefts, Rights [][blockSize]*dedicated.TernaryCtx
	// Node contains the hash of nodes, obtained by hashing the left and
	// right.
	Nodes []*poseidon2.HashingCtx
}

// CheckFlatMerkleProofs checks a list of Merkle proof using an horizontal
// wizard arithmetization.
func CheckFlatMerkleProofs(comp *wizard.CompiledIOP, inputs FlatProofVerificationInputs) *FlatMerkleProofVerification {

	if err := checkColumnsAllHaveSameSize(&inputs); err != nil {
		panic(err)
	}

	ctx := &FlatMerkleProofVerification{
		FlatProofVerificationInputs: inputs,
		PosBits:                     bits.BitDecompose(comp, inputs.Position[:], len(inputs.Proof.Nodes[0])),
	}

	prevNode := inputs.Leaf
	var left, right [blockSize]*dedicated.TernaryCtx
	var leftResult, rightResult [blockSize]ifaces.Column
	for i := range inputs.Proof.Nodes[0] {
		for j := 0; j < blockSize; j++ {

			left[j] = dedicated.Ternary(comp, ctx.PosBits.Bits[i], inputs.Proof.Nodes[j][i], prevNode[j])
			right[j] = dedicated.Ternary(comp, ctx.PosBits.Bits[i], prevNode[j], inputs.Proof.Nodes[j][i])
			leftResult[j] = left[j].Result
			rightResult[j] = right[j].Result
		}

		node := poseidon2.HashOf(comp,
			append(leftResult[:], rightResult[:]...),
			fmt.Sprintf("FLAT_MERKLE_NODE_HASHING_%v", i),
		)

		prevNode = node.Result()
		ctx.Lefts = append(ctx.Lefts, left)
		ctx.Rights = append(ctx.Rights, right)
		ctx.Nodes = append(ctx.Nodes, node)
	}

	for i := 0; i < blockSize; i++ {
		// This check ensures that the computed and the provided root match. Note
		// that prevNode is the last node computed, hence the root.
		comp.InsertGlobal(
			max(prevNode[i].Round(), inputs.Roots[i].Round()),
			ifaces.QueryIDf("%v_ROOT_%v_MATCH", inputs.Name, i),
			symbolic.Mul(inputs.IsActive, symbolic.Sub(prevNode[i], inputs.Roots[i])),
		)
	}

	return ctx
}

// AddProofReuseConstraint adds a constraint on the reuse of the same proof
// across different rows of the Merkle proof verification module.
func (ctx *FlatMerkleProofVerification) AddProofReuseConstraint(comp *wizard.CompiledIOP, mustReuseForNext ifaces.Column) {

	for i := range ctx.Proof.Nodes[0] {
		for j := 0; j < blockSize; j++ {
			comp.InsertGlobal(
				max(mustReuseForNext.Round(), ctx.Proof.Nodes[j][0].Round()),
				ifaces.QueryIDf("%v_PROOF_%v_REUSE_%v", ctx.FlatProofVerificationInputs.Name, j, i),
				symbolic.Mul(
					mustReuseForNext,
					symbolic.Sub(
						ctx.Proof.Nodes[j][i],
						column.Shift(ctx.Proof.Nodes[j][i], 1),
					),
				),
			)
		}
	}
}

// Run implements the [wizard.ProverAction] interface and implements all the
// steps of the Merkle tree verification. It assigns node after node, all the
// columns needed to complete the Merkle tree verification from the leaf up
// to the root.
func (ctx *FlatMerkleProofVerification) Run(run *wizard.ProverRuntime) {

	ctx.PosBits.Run(run)

	for i := range ctx.PosBits.Bits {

		for j := 0; j < blockSize; j++ {
			ctx.Lefts[i][j].Run(run)
			ctx.Rights[i][j].Run(run)
		}
		ctx.Nodes[i].Run(run)
	}
}

// checkColumnsAllHaveSameSize checks if all the columns have the same size
// and returns an error if they don't.
func checkColumnsAllHaveSameSize(inp *FlatProofVerificationInputs) error {

	size := inp.Roots[0].Size()
	for j := 0; j < blockSize; j++ {
		for _, node := range inp.Proof.Nodes[j] {
			if node.Size() != size {
				return fmt.Errorf("all nodes must have the same size: root=%v proof=%v", size, node.Size())
			}
		}

		if inp.Leaf[j].Size() != size {
			return fmt.Errorf("all nodes must have the same size: root=%v leaf=%v", size, inp.Leaf[j].Size())
		}
	}
	for i := range inp.Position {
		if inp.Position[i].Size() != size {
			return fmt.Errorf("all nodes must have the same size: root=%v position=%v", size, inp.Position[i].Size())
		}
	}

	if inp.IsActive.Size() != size {
		return fmt.Errorf("all nodes must have the same size: root=%v isActive=%v", size, inp.IsActive.Size())
	}

	return nil
}
