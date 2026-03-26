package accumulator

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/crypto/state-management/smt_koalabear"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"

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

// valueToLimbs creates a list of field.Element where the last element of the list is a value provided
// to the function. The number of limbs is defined by limbsNb argument.
//
// Note: this is a test function and presumed that the function is used only in tests.
func valueToLimbs(limbsNb int, value uint32) (res []field.Element) {
	res = make([]field.Element, limbsNb)
	for i := range limbsNb - 1 {
		res[i] = field.Zero()
	}

	res[limbsNb-1] = field.NewElement(uint64(value))
	return res
}

// valuesToLimbRows creates a list of []field.Element where each []field.Element is a column within a limb.
// The last limb of each row is a value defined by the values argument of the function.
//
// Note: this is a test function and presumed that the function is used only in tests.
func valuesToLimbRows(limbsNb int, values ...uint32) (res [][]field.Element) {
	res = make([][]field.Element, limbsNb)

	for _, value := range values {
		limbs := valueToLimbs(limbsNb, value)
		for j, limb := range limbs {
			res[j] = append(res[j], limb)
		}
	}
	return res
}

// getLimbsFromRow gets a row of limbs of some field.Element.
func getLimbsFromRow(columns [][]field.Element, row int) (res []field.Element) {
	for _, limb := range columns {
		res = append(res, limb[row])
	}

	return res
}

func TestAssignInsert(t *testing.T) {

	builder := newAssignmentBuilder(testSetting)

	acc := statemanager.NewStorageTrie(types.EthAddress{})
	traceInsert := acc.InsertAndProve(types.FullBytes32FromHex("0x32"), types.FullBytes32FromHex("0x12"))

	pushInsertionRows(builder, traceInsert)

	assert.Equal(t, valuesToLimbRows(16, 0, 0, 2, 2, 1, 1), builder.positions[:])
	assert.Equal(t, vector.ForTest(1, 0, 0, 0, 0, 0), builder.isFirst)
	assert.Equal(t, vector.ForTest(1, 1, 1, 1, 1, 1), builder.isInsert)
	assert.Equal(t, valuesToLimbRows(16, 2, 2, 3, 3, 3, 3), builder.nextFreeNode[:])
	assert.Equal(t, valuesToLimbRows(16, 0, 0, 2, 0, 0, 0), builder.insertionPath[:])
	assert.Equal(t, vector.ForTest(0, 0, 1, 0, 0, 0), builder.isInsertRow3)
	assert.Equal(t, vector.ForTest(0, 0, 0, 0, 0, 0), builder.isDelete)
	assert.Equal(t, vector.ForTest(0, 0, 0, 0, 0, 0), builder.isUpdate)
	assert.Equal(t, vector.ForTest(0, 0, 0, 0, 0, 0), builder.isReadZero)
	assert.Equal(t, vector.ForTest(0, 0, 0, 0, 0, 0), builder.isReadNonZero)
	assert.Equal(t, vector.ForTest(1, 0, 1, 0, 1, 0), builder.useNextMerkleProof)
	assert.Equal(t, vector.ForTest(1, 1, 1, 1, 1, 1), builder.isActive)
	assert.Equal(t, valuesToLimbRows(16, 0, 1, 2, 3, 4, 5), builder.accumulatorCounter[:])
	for _, leavesLimbCol := range builder.leaves {
		for i := range leavesLimbCol {
			if i == 0 {
				continue
			}

			assert.NotEqual(t, leavesLimbCol[i-1], leavesLimbCol[i])
		}
	}

	assert.Equal(t, valueToLimbs(8, 0), getLimbsFromRow(builder.leaves[:], 2))
	assert.Equal(t, getLimbsFromRow(builder.roots[:], 1), getLimbsFromRow(builder.roots[:], 2))
	assert.Equal(t, getLimbsFromRow(builder.roots[:], 3), getLimbsFromRow(builder.roots[:], 4))
	assert.Equal(t, getLimbsFromRow(builder.positions[:], 0), getLimbsFromRow(builder.positions[:], 1))
	assert.Equal(t, getLimbsFromRow(builder.positions[:], 2), getLimbsFromRow(builder.positions[:], 3))
	assert.Equal(t, getLimbsFromRow(builder.positions[:], 4), getLimbsFromRow(builder.positions[:], 5))

	assertCorrectMerkleProof(t, builder)
	// Verify the Merkle proofs along with the reuse in the wizard
	assertCorrectMerkleProofsUsingWizard(t, builder)
}

func TestAssignUpdate(t *testing.T) {

	builder := newAssignmentBuilder(testSetting)

	acc := statemanager.NewStorageTrie(types.EthAddress{})
	acc.InsertAndProve(types.FullBytes32FromHex("0x32"), types.FullBytes32FromHex("0x12"))
	traceUpdate := acc.UpdateAndProve(types.FullBytes32FromHex("0x32"), types.FullBytes32FromHex("0x20"))
	pushUpdateRows(builder, traceUpdate)

	assert.Equal(t, valuesToLimbRows(16, 2, 2), builder.positions[:])
	assert.Equal(t, vector.ForTest(1, 0), builder.isFirst)
	assert.Equal(t, vector.ForTest(0, 0), builder.isInsert)
	assert.Equal(t, valuesToLimbRows(16, 3, 3), builder.nextFreeNode[:])
	assert.Equal(t, valuesToLimbRows(16, 0, 0), builder.insertionPath[:])
	assert.Equal(t, vector.ForTest(0, 0), builder.isInsertRow3)
	assert.Equal(t, vector.ForTest(0, 0), builder.isDelete)
	assert.Equal(t, vector.ForTest(1, 1), builder.isUpdate)
	assert.Equal(t, vector.ForTest(0, 0), builder.isReadZero)
	assert.Equal(t, vector.ForTest(0, 0), builder.isReadNonZero)
	assert.Equal(t, vector.ForTest(1, 0), builder.useNextMerkleProof)
	assert.Equal(t, vector.ForTest(1, 1), builder.isActive)
	assert.Equal(t, valuesToLimbRows(16, 0, 1), builder.accumulatorCounter[:])
	for _, leavesLimbCol := range builder.leaves {
		for i := range leavesLimbCol {
			if i == 0 {
				continue
			}

			assert.NotEqual(t, leavesLimbCol[i-1], leavesLimbCol[i])
		}
	}

	assert.Equal(t, getLimbsFromRow(builder.positions[:], 0), getLimbsFromRow(builder.positions[:], 1))
	assertCorrectMerkleProof(t, builder)
	// Verify the Merkle proofs along with the reuse in the wizard
	assertCorrectMerkleProofsUsingWizard(t, builder)
}

func TestAssignDelete(t *testing.T) {

	builder := newAssignmentBuilder(testSetting)

	acc := statemanager.NewStorageTrie(types.EthAddress{})
	acc.InsertAndProve(types.FullBytes32FromHex("0x32"), types.FullBytes32FromHex("0x12"))
	traceDelete := acc.DeleteAndProve(types.FullBytes32FromHex("0x32"))
	pushDeletionRows(builder, traceDelete)

	assert.Equal(t, valueToLimbs(8, 0), getLimbsFromRow(builder.leaves[:], 3))
	assert.Equal(t, getLimbsFromRow(builder.roots[:], 1), getLimbsFromRow(builder.roots[:], 2))
	assert.Equal(t, getLimbsFromRow(builder.roots[:], 3), getLimbsFromRow(builder.roots[:], 4))
	assert.Equal(t, vector.ForTest(1, 0, 0, 0, 0, 0), builder.isFirst)
	assert.Equal(t, vector.ForTest(0, 0, 0, 0, 0, 0), builder.isInsert)
	assert.Equal(t, valuesToLimbRows(16, 3, 3, 3, 3, 3, 3), builder.nextFreeNode[:])
	assert.Equal(t, valuesToLimbRows(16, 0, 0, 0, 0, 0, 0), builder.insertionPath[:])
	assert.Equal(t, vector.ForTest(0, 0, 0, 0, 0, 0), builder.isInsertRow3)
	assert.Equal(t, vector.ForTest(1, 1, 1, 1, 1, 1), builder.isDelete)
	assert.Equal(t, vector.ForTest(0, 0, 0, 0, 0, 0), builder.isUpdate)
	assert.Equal(t, vector.ForTest(0, 0, 0, 0, 0, 0), builder.isReadZero)
	assert.Equal(t, vector.ForTest(0, 0, 0, 0, 0, 0), builder.isReadNonZero)
	assert.Equal(t, vector.ForTest(1, 0, 1, 0, 1, 0), builder.useNextMerkleProof)
	assert.Equal(t, vector.ForTest(1, 1, 1, 1, 1, 1), builder.isActive)
	assert.Equal(t, valuesToLimbRows(16, 0, 1, 2, 3, 4, 5), builder.accumulatorCounter[:])
	for _, leavesLimbCol := range builder.leaves {
		for i := range leavesLimbCol {
			if i == 0 {
				continue
			}

			assert.NotEqual(t, leavesLimbCol[i-1], leavesLimbCol[i])
		}
	}

	assert.Equal(t, getLimbsFromRow(builder.positions[:], 0), getLimbsFromRow(builder.positions[:], 1))
	assert.Equal(t, getLimbsFromRow(builder.positions[:], 2), getLimbsFromRow(builder.positions[:], 3))
	assert.Equal(t, getLimbsFromRow(builder.positions[:], 4), getLimbsFromRow(builder.positions[:], 5))
	assertCorrectMerkleProof(t, builder)
	// Verify the Merkle proofs along with the reuse in the wizard
	assertCorrectMerkleProofsUsingWizard(t, builder)
}

func TestAssignReadZero(t *testing.T) {

	builder := newAssignmentBuilder(testSetting)

	acc := statemanager.NewStorageTrie(types.EthAddress{})
	traceReadZero := acc.ReadZeroAndProve(types.FullBytes32FromHex("0x32"))
	pushReadZeroRows(builder, traceReadZero)

	assert.Equal(t, getLimbsFromRow(builder.roots[:], 0), getLimbsFromRow(builder.roots[:], 1))
	assert.Equal(t, valuesToLimbRows(16, 0, 1), builder.positions[:])
	assert.Equal(t, vector.ForTest(1, 0), builder.isFirst)
	assert.Equal(t, vector.ForTest(0, 0), builder.isInsert)
	assert.Equal(t, valuesToLimbRows(16, 2, 2), builder.nextFreeNode[:])
	assert.Equal(t, valuesToLimbRows(16, 0, 0), builder.insertionPath[:])
	assert.Equal(t, vector.ForTest(0, 0), builder.isInsertRow3)
	assert.Equal(t, vector.ForTest(0, 0), builder.isDelete)
	assert.Equal(t, vector.ForTest(0, 0), builder.isUpdate)
	assert.Equal(t, vector.ForTest(1, 1), builder.isReadZero)
	assert.Equal(t, vector.ForTest(0, 0), builder.isReadNonZero)
	assert.Equal(t, vector.ForTest(0, 0), builder.useNextMerkleProof)
	assert.Equal(t, vector.ForTest(1, 1), builder.isActive)
	assert.Equal(t, valuesToLimbRows(16, 0, 1), builder.accumulatorCounter[:])
	for _, leavesLimbCol := range builder.leaves {
		for i := range leavesLimbCol {
			if i == 0 {
				continue
			}

			assert.NotEqual(t, leavesLimbCol[i-1], leavesLimbCol[i])
		}
	}

	assertCorrectMerkleProof(t, builder)
	// Verify the Merkle proofs along with the reuse in the wizard
	assertCorrectMerkleProofsUsingWizard(t, builder)
}

func TestAssignReadNonZero(t *testing.T) {

	builder := newAssignmentBuilder(testSetting)

	acc := statemanager.NewStorageTrie(types.EthAddress{})
	acc.InsertAndProve(types.FullBytes32FromHex("0x32"), types.FullBytes32FromHex("0x12"))
	traceReadNonZero := acc.ReadNonZeroAndProve(types.FullBytes32FromHex("0x32"))
	pushReadNonZeroRows(builder, traceReadNonZero)

	assert.Equal(t, getLimbsFromRow(builder.roots[:], 0), getLimbsFromRow(builder.roots[:], 1))
	assert.Equal(t, valuesToLimbRows(16, 2, 2), builder.positions[:])
	assert.Equal(t, vector.ForTest(1, 0), builder.isFirst)
	assert.Equal(t, vector.ForTest(0, 0), builder.isInsert)
	assert.Equal(t, valuesToLimbRows(16, 3, 3), builder.nextFreeNode[:])
	assert.Equal(t, valuesToLimbRows(16, 0, 0), builder.insertionPath[:])
	assert.Equal(t, vector.ForTest(0, 0), builder.isInsertRow3)
	assert.Equal(t, vector.ForTest(0, 0), builder.isDelete)
	assert.Equal(t, vector.ForTest(0, 0), builder.isUpdate)
	assert.Equal(t, vector.ForTest(0, 0), builder.isReadZero)
	assert.Equal(t, vector.ForTest(1, 1), builder.isReadNonZero)
	assert.Equal(t, vector.ForTest(0, 0), builder.useNextMerkleProof)
	assert.Equal(t, vector.ForTest(1, 1), builder.isActive)
	assert.Equal(t, valuesToLimbRows(16, 0, 1), builder.accumulatorCounter[:])

	assertCorrectMerkleProof(t, builder)
	// Verify the Merkle proofs along with the reuse in the wizard
	assertCorrectMerkleProofsUsingWizard(t, builder)
}

func assertCorrectMerkleProof(t *testing.T, builder *assignmentBuilder) {
	proofs := builder.proofs
	for i, proof := range proofs {
		leaf := field.Octuplet(getLimbsFromRow(builder.leaves[:], i))
		root := field.Octuplet(getLimbsFromRow(builder.roots[:], i))
		assert.Equal(t, nil, smt_koalabear.Verify(&proof, leaf, root))
	}
}

func assertCorrectMerkleProofsUsingWizard(t *testing.T, builder *assignmentBuilder) {

	var (
		merkleVerification    *merkle.FlatMerkleProofVerification
		size                  = utils.NextPowerOfTwo(builder.MaxNumProofs)
		proofcol              *merkle.FlatProof
		rootscol              [common.NbElemPerHash]ifaces.Column
		leavescol             [common.NbElemPerHash]ifaces.Column
		poscol                [common.NbElemForHasingU64]ifaces.Column
		useNextMerkleProofCol ifaces.Column
		isActiveCol           ifaces.Column
	)

	define := func(b *wizard.Builder) {
		proofcol = merkle.NewProof(b.CompiledIOP, 0, "PROOF", builder.MerkleTreeDepth, size)

		for i := 0; i < common.NbElemPerHash; i++ {
			rootscol[i] = b.RegisterCommit(ifaces.ColIDf("ROOTS_%d", i), size)
			leavescol[i] = b.RegisterCommit(ifaces.ColIDf("LEAVES_%d", i), size)
		}

		for i := 0; i < common.NbElemForHasingU64; i++ {
			poscol[i] = b.RegisterCommit(ifaces.ColIDf("POS_%d", i), size)
		}

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

		for i := 0; i < common.NbElemPerHash; i++ {
			run.AssignColumn(ifaces.ColIDf("ROOTS_%d", i), smartvectors.RightZeroPadded(builder.roots[i], size))
			run.AssignColumn(ifaces.ColIDf("LEAVES_%d", i), smartvectors.RightZeroPadded(builder.leaves[i], size))
		}

		for i := 0; i < len(builder.positions); i++ {
			run.AssignColumn(ifaces.ColIDf("POS_%d", i), smartvectors.RightZeroPadded(builder.positions[i], size))
		}
		run.AssignColumn("REUSE_NEXT_PROOF", smartvectors.RightZeroPadded(builder.useNextMerkleProof, size))
		run.AssignColumn("IS_ACTIVE", smartvectors.RightZeroPadded(builder.isActive, size))

		merkleVerification.Run(run)
	}

	comp := wizard.Compile(define, dummy.Compile)
	proof := wizard.Prove(comp, prove)
	err := wizard.Verify(comp, proof)

	require.NoError(t, err)
}
