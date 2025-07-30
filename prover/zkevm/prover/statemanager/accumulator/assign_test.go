package accumulator

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/backend/execution/statemanager"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/merkle"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testSetting = Settings{
	MaxNumProofs:    16,
	MerkleTreeDepth: 40,
}

func TestAssignInsert(t *testing.T) {

	builder := newAssignmentBuilder(testSetting)

	acc := statemanager.NewStorageTrie(statemanager.MIMC_CONFIG, types.EthAddress{})
	traceInsert := acc.InsertAndProve(types.FullBytes32FromHex("0x32"), types.FullBytes32FromHex("0x12"))

	pushInsertionRows(builder, traceInsert)

	assert.Equal(t, vector.ForTest(0, 0, 2, 2, 1, 1), builder.positions)
	assert.Equal(t, vector.ForTest(1, 0, 0, 0, 0, 0), builder.isFirst)
	assert.Equal(t, vector.ForTest(1, 1, 1, 1, 1, 1), builder.isInsert)
	assert.Equal(t, vector.ForTest(2, 2, 3, 3, 3, 3), builder.nextFreeNode)
	assert.Equal(t, vector.ForTest(0, 0, 2, 0, 0, 0), builder.insertionPath)
	assert.Equal(t, vector.ForTest(0, 0, 1, 0, 0, 0), builder.isInsertRow3)
	assert.Equal(t, vector.ForTest(0, 0, 0, 0, 0, 0), builder.isDelete)
	assert.Equal(t, vector.ForTest(0, 0, 0, 0, 0, 0), builder.isUpdate)
	assert.Equal(t, vector.ForTest(0, 0, 0, 0, 0, 0), builder.isReadZero)
	assert.Equal(t, vector.ForTest(0, 0, 0, 0, 0, 0), builder.isReadNonZero)
	assert.Equal(t, vector.ForTest(1, 0, 1, 0, 1, 0), builder.useNextMerkleProof)
	assert.Equal(t, vector.ForTest(1, 1, 1, 1, 1, 1), builder.isActive)
	assert.Equal(t, vector.ForTest(0, 1, 2, 3, 4, 5), builder.accumulatorCounter)
	for i := range builder.leaves {
		if i == 0 {
			continue
		}
		assert.NotEqual(t, builder.leaves[i-1], builder.leaves[i])
	}

	assert.Equal(t, field.Zero(), builder.leaves[2])
	assert.Equal(t, builder.roots[1], builder.roots[2])
	assert.Equal(t, builder.roots[3], builder.roots[4])
	assert.Equal(t, builder.positions[0], builder.positions[1])
	assert.Equal(t, builder.positions[2], builder.positions[3])
	assert.Equal(t, builder.positions[4], builder.positions[5])
	assertCorrectMerkleProof(t, builder)
	// Verify the Merkle proofs along with the reuse in the wizard
	assertCorrectMerkleProofsUsingWizard(t, builder)

}

func TestAssignUpdate(t *testing.T) {

	builder := newAssignmentBuilder(testSetting)

	acc := statemanager.NewStorageTrie(statemanager.MIMC_CONFIG, types.EthAddress{})
	acc.InsertAndProve(types.FullBytes32FromHex("0x32"), types.FullBytes32FromHex("0x12"))
	traceUpdate := acc.UpdateAndProve(types.FullBytes32FromHex("0x32"), types.FullBytes32FromHex("0x20"))
	pushUpdateRows(builder, traceUpdate)

	assert.Equal(t, vector.ForTest(2, 2), builder.positions)
	assert.Equal(t, vector.ForTest(1, 0), builder.isFirst)
	assert.Equal(t, vector.ForTest(0, 0), builder.isInsert)
	assert.Equal(t, vector.ForTest(3, 3), builder.nextFreeNode)
	assert.Equal(t, vector.ForTest(0, 0), builder.insertionPath)
	assert.Equal(t, vector.ForTest(0, 0), builder.isInsertRow3)
	assert.Equal(t, vector.ForTest(0, 0), builder.isDelete)
	assert.Equal(t, vector.ForTest(1, 1), builder.isUpdate)
	assert.Equal(t, vector.ForTest(0, 0), builder.isReadZero)
	assert.Equal(t, vector.ForTest(0, 0), builder.isReadNonZero)
	assert.Equal(t, vector.ForTest(1, 0), builder.useNextMerkleProof)
	assert.Equal(t, vector.ForTest(1, 1), builder.isActive)
	assert.Equal(t, vector.ForTest(0, 1), builder.accumulatorCounter)
	for i := range builder.leaves {
		if i == 0 {
			continue
		}
		assert.NotEqual(t, builder.leaves[i-1], builder.leaves[i])
	}

	assert.Equal(t, builder.positions[0], builder.positions[1])
	assertCorrectMerkleProof(t, builder)
	// Verify the Merkle proofs along with the reuse in the wizard
	assertCorrectMerkleProofsUsingWizard(t, builder)
}

func TestAssignDelete(t *testing.T) {

	builder := newAssignmentBuilder(testSetting)

	acc := statemanager.NewStorageTrie(statemanager.MIMC_CONFIG, types.EthAddress{})
	acc.InsertAndProve(types.FullBytes32FromHex("0x32"), types.FullBytes32FromHex("0x12"))
	traceDelete := acc.DeleteAndProve(types.FullBytes32FromHex("0x32"))
	pushDeletionRows(builder, traceDelete)

	assert.Equal(t, field.Zero(), builder.leaves[3])
	assert.Equal(t, builder.roots[1], builder.roots[2])
	assert.Equal(t, builder.roots[3], builder.roots[4])
	assert.Equal(t, vector.ForTest(1, 0, 0, 0, 0, 0), builder.isFirst)
	assert.Equal(t, vector.ForTest(0, 0, 0, 0, 0, 0), builder.isInsert)
	assert.Equal(t, vector.ForTest(3, 3, 3, 3, 3, 3), builder.nextFreeNode)
	assert.Equal(t, vector.ForTest(0, 0, 0, 0, 0, 0), builder.insertionPath)
	assert.Equal(t, vector.ForTest(0, 0, 0, 0, 0, 0), builder.isInsertRow3)
	assert.Equal(t, vector.ForTest(1, 1, 1, 1, 1, 1), builder.isDelete)
	assert.Equal(t, vector.ForTest(0, 0, 0, 0, 0, 0), builder.isUpdate)
	assert.Equal(t, vector.ForTest(0, 0, 0, 0, 0, 0), builder.isReadZero)
	assert.Equal(t, vector.ForTest(0, 0, 0, 0, 0, 0), builder.isReadNonZero)
	assert.Equal(t, vector.ForTest(1, 0, 1, 0, 1, 0), builder.useNextMerkleProof)
	assert.Equal(t, vector.ForTest(1, 1, 1, 1, 1, 1), builder.isActive)
	assert.Equal(t, vector.ForTest(0, 1, 2, 3, 4, 5), builder.accumulatorCounter)
	for i := range builder.leaves {
		if i == 0 {
			continue
		}
		assert.NotEqual(t, builder.leaves[i-1], builder.leaves[i])
	}

	assert.Equal(t, builder.positions[0], builder.positions[1])
	assert.Equal(t, builder.positions[2], builder.positions[3])
	assert.Equal(t, builder.positions[4], builder.positions[5])
	assertCorrectMerkleProof(t, builder)
	// Verify the Merkle proofs along with the reuse in the wizard
	assertCorrectMerkleProofsUsingWizard(t, builder)
}

func TestAssignReadZero(t *testing.T) {

	builder := newAssignmentBuilder(testSetting)

	acc := statemanager.NewStorageTrie(statemanager.MIMC_CONFIG, types.EthAddress{})
	traceReadZero := acc.ReadZeroAndProve(types.FullBytes32FromHex("0x32"))
	pushReadZeroRows(builder, traceReadZero)

	assert.Equal(t, builder.roots[0], builder.roots[1])
	assert.Equal(t, vector.ForTest(0, 1), builder.positions)
	assert.Equal(t, vector.ForTest(1, 0), builder.isFirst)
	assert.Equal(t, vector.ForTest(0, 0), builder.isInsert)
	assert.Equal(t, vector.ForTest(2, 2), builder.nextFreeNode)
	assert.Equal(t, vector.ForTest(0, 0), builder.insertionPath)
	assert.Equal(t, vector.ForTest(0, 0), builder.isInsertRow3)
	assert.Equal(t, vector.ForTest(0, 0), builder.isDelete)
	assert.Equal(t, vector.ForTest(0, 0), builder.isUpdate)
	assert.Equal(t, vector.ForTest(1, 1), builder.isReadZero)
	assert.Equal(t, vector.ForTest(0, 0), builder.isReadNonZero)
	assert.Equal(t, vector.ForTest(0, 0), builder.useNextMerkleProof)
	assert.Equal(t, vector.ForTest(1, 1), builder.isActive)
	assert.Equal(t, vector.ForTest(0, 1), builder.accumulatorCounter)
	for i := range builder.leaves {
		if i == 0 {
			continue
		}
		assert.NotEqual(t, builder.leaves[i-1], builder.leaves[i])
	}

	assertCorrectMerkleProof(t, builder)
	// Verify the Merkle proofs along with the reuse in the wizard
	assertCorrectMerkleProofsUsingWizard(t, builder)
}

func TestAssignReadNonZero(t *testing.T) {

	builder := newAssignmentBuilder(testSetting)

	acc := statemanager.NewStorageTrie(statemanager.MIMC_CONFIG, types.EthAddress{})
	acc.InsertAndProve(types.FullBytes32FromHex("0x32"), types.FullBytes32FromHex("0x12"))
	traceReadNonZero := acc.ReadNonZeroAndProve(types.FullBytes32FromHex("0x32"))
	pushReadNonZeroRows(builder, traceReadNonZero)

	assert.Equal(t, builder.roots[0], builder.roots[1])
	assert.Equal(t, vector.ForTest(2, 2), builder.positions)
	assert.Equal(t, vector.ForTest(1, 0), builder.isFirst)
	assert.Equal(t, vector.ForTest(0, 0), builder.isInsert)
	assert.Equal(t, vector.ForTest(3, 3), builder.nextFreeNode)
	assert.Equal(t, vector.ForTest(0, 0), builder.insertionPath)
	assert.Equal(t, vector.ForTest(0, 0), builder.isInsertRow3)
	assert.Equal(t, vector.ForTest(0, 0), builder.isDelete)
	assert.Equal(t, vector.ForTest(0, 0), builder.isUpdate)
	assert.Equal(t, vector.ForTest(0, 0), builder.isReadZero)
	assert.Equal(t, vector.ForTest(1, 1), builder.isReadNonZero)
	assert.Equal(t, vector.ForTest(0, 0), builder.useNextMerkleProof)
	assert.Equal(t, vector.ForTest(1, 1), builder.isActive)
	assert.Equal(t, vector.ForTest(0, 1), builder.accumulatorCounter)

	assertCorrectMerkleProof(t, builder)
	// Verify the Merkle proofs along with the reuse in the wizard
	assertCorrectMerkleProofsUsingWizard(t, builder)
}

func assertCorrectMerkleProof(t *testing.T, builder *assignmentBuilder) {
	proofs := builder.proofs
	for i, proof := range proofs {
		assert.Equal(t, true, proof.Verify(statemanager.MIMC_CONFIG, builder.leaves[i].Bytes(), builder.roots[i].Bytes()))
	}
}

func assertCorrectMerkleProofsUsingWizard(t *testing.T, builder *assignmentBuilder) {

	var (
		merkleVerification    *merkle.FlatMerkleProofVerification
		size                  = utils.NextPowerOfTwo(builder.MaxNumProofs)
		proofcol              *merkle.FlatProof
		rootscol              ifaces.Column
		leavescol             ifaces.Column
		poscol                ifaces.Column
		useNextMerkleProofCol ifaces.Column
		isActiveCol           ifaces.Column
	)

	define := func(b *wizard.Builder) {
		proofcol = merkle.NewProof(b.CompiledIOP, 0, "PROOF", builder.MerkleTreeDepth, size)
		rootscol = b.RegisterCommit("ROOTS", size)
		leavescol = b.RegisterCommit("LEAVES", size)
		poscol = b.RegisterCommit("POS", size)
		useNextMerkleProofCol = b.RegisterCommit("REUSE_NEXT_PROOF", size)
		isActiveCol = b.RegisterCommit("IS_ACTIVE", size)

		merkleVerification = merkle.CheckFlatMerkleProofs(
			b.CompiledIOP,
			merkle.FlatProofVerificationInputs{
				Name:     "TEST",
				Proof:    *proofcol,
				Roots:    rootscol,
				Leaf:     leavescol,
				Position: poscol,
				IsActive: isActiveCol,
			},
		)

		merkleVerification.AddProofReuseConstraint(b.CompiledIOP, useNextMerkleProofCol)
	}

	prove := func(run *wizard.ProverRuntime) {

		proofcol.Assign(run, builder.proofs)
		run.AssignColumn("ROOTS", smartvectors.RightZeroPadded(builder.roots, size))
		run.AssignColumn("LEAVES", smartvectors.RightZeroPadded(builder.leaves, size))
		run.AssignColumn("POS", smartvectors.RightZeroPadded(builder.positions, size))
		run.AssignColumn("REUSE_NEXT_PROOF", smartvectors.RightZeroPadded(builder.useNextMerkleProof, size))
		run.AssignColumn("IS_ACTIVE", smartvectors.RightZeroPadded(builder.isActive, size))

		merkleVerification.Run(run)
	}

	comp := wizard.Compile(define, dummy.Compile)
	proof := wizard.Prove(comp, prove)
	err := wizard.Verify(comp, proof)

	require.NoError(t, err)
}
