//go:build !fuzzlight

package accumulator_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/crypto/state-management/accumulator"
	"github.com/consensys/linea-monorepo/prover/crypto/state-management/smt_koalabear"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitializationPoseidon2(t *testing.T) {
	// Just check that the code returns
	acc := newTestAccumulatorPoseidon2DummyVal()
	ver := acc.VerifierState()

	// The next free nodes are well initialized
	assert.Equal(t, int64(2), acc.NextFreeNode, "bad next free node for the prover state")
	assert.Equal(t, int64(2), ver.NextFreeNode, "bad next free node for the verifier state")

	// The roots are consistent
	assert.Equal(t, acc.SubTreeRoot(), ver.SubTreeRoot, "inconsistent roots")

	headHash := accumulator.Head().Hash()
	tailHash := accumulator.Tail().Hash()

	// First leaf is head
	assert.Equal(t, acc.Tree.MustGetLeaf(0), accumulator.Head().Hash().ToOctuplet())
	assert.Equal(t, acc.Tree.MustGetLeaf(1), accumulator.Tail().Hash().ToOctuplet())

	// Can we prover membership of the leaf
	proofHead := acc.Tree.MustProve(0)
	smt_koalabear.Verify(&proofHead, headHash.ToOctuplet(), acc.SubTreeRoot().ToOctuplet())

	proofTail := acc.Tree.MustProve(1)
	smt_koalabear.Verify(&proofTail, tailHash.ToOctuplet(), acc.SubTreeRoot().ToOctuplet())
}

func TestInsertionPoseidon2(t *testing.T) {

	// Performs an insertion
	acc := newTestAccumulatorPoseidon2DummyVal()
	ver := acc.VerifierState()

	for i := 0; i < numRepetion; i++ {
		trace := acc.InsertAndProve(dumkey(i), dumval(i))
		err := ver.VerifyInsertion(trace)
		require.NoErrorf(t, err, "check #%v - trace %++v", i, trace)
	}

	// Roots of the verifier should be correct
	assert.Equal(t, acc.NextFreeNode, ver.NextFreeNode)
	assert.Equal(t, acc.SubTreeRoot(), ver.SubTreeRoot)
}

func TestReadZeroPoseidon2(t *testing.T) {

	// Performs an insertion
	acc := newTestAccumulatorPoseidon2DummyVal()
	ver := acc.VerifierState()

	for i := 0; i < numRepetion; i++ {
		key := dumkey(i)
		trace := acc.ReadZeroAndProve(key)
		err := ver.ReadZeroVerify(trace)
		require.NoErrorf(t, err, "check #%v - trace %++v", i, trace)
	}

	// Roots of the verifier should be correct
	assert.Equal(t, acc.NextFreeNode, ver.NextFreeNode)
	assert.Equal(t, acc.SubTreeRoot(), ver.SubTreeRoot)
}

func TestReadNonZeroPoseidon2(t *testing.T) {

	// Performs an insertion
	acc := newTestAccumulatorPoseidon2DummyVal()

	// Fill the tree
	for i := 0; i < numRepetion; i++ {
		_ = acc.InsertAndProve(dumkey(i), dumval(i))
	}

	// Snapshot the verifier after the insertions because of the verifier
	ver := acc.VerifierState()

	for i := 0; i < numRepetion; i++ {
		trace := acc.ReadNonZeroAndProve(dumkey(i))
		err := ver.ReadNonZeroVerify(trace)
		require.NoErrorf(t, err, "check #%v - trace %++v", i, trace)
		require.Equal(t, dumval(i), trace.Value)
	}

	// Roots of the verifier should be correct
	assert.Equal(t, acc.NextFreeNode, ver.NextFreeNode)
	assert.Equal(t, acc.SubTreeRoot(), ver.SubTreeRoot)
}

func TestUpdatePoseidon2(t *testing.T) {
	// Performs an insertion
	acc := newTestAccumulatorPoseidon2DummyVal()

	// Fill the tree
	for i := 0; i < numRepetion; i++ {
		_ = acc.InsertAndProve(dumkey(i), dumval(i))
	}

	// Snapshot the verifier after the insertions because of the verifier
	ver := acc.VerifierState()

	for i := 0; i < numRepetion; i++ {
		trace := acc.UpdateAndProve(dumkey(i), dumval(i+1000))
		err := ver.UpdateVerify(trace)
		require.NoErrorf(t, err, "check #%v - trace %++v", i, trace)
	}

	// Roots of the verifier should be correct
	assert.Equal(t, acc.NextFreeNode, ver.NextFreeNode)
	assert.Equal(t, acc.SubTreeRoot(), ver.SubTreeRoot)
}

func TestDeletionPoseidon2(t *testing.T) {
	// Performs an insertion
	acc := newTestAccumulatorPoseidon2DummyVal()

	// Fill the tree
	for i := 0; i < numRepetion; i++ {
		_ = acc.InsertAndProve(dumkey(i), dumval(i))
	}

	// Snapshot the verifier after the insertions because of the verifier
	ver := acc.VerifierState()

	for i := 0; i < numRepetion; i++ {
		trace := acc.DeleteAndProve(dumkey(i))
		err := ver.VerifyDeletion(trace)
		require.NoErrorf(t, err, "check #%v - trace %++v", i, trace)
	}

	// Roots of the verifier should be correct
	assert.Equal(t, acc.NextFreeNode, ver.NextFreeNode)
	assert.Equal(t, acc.SubTreeRoot(), ver.SubTreeRoot)
}
