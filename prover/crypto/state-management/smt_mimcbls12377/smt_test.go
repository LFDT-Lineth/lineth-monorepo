package smt_test

import (
	"testing"

	hashtypes "github.com/consensys/linea-monorepo/prover/crypto/state-management/hashtypes_legacy"
	smt "github.com/consensys/linea-monorepo/prover/crypto/state-management/smt_mimcbls12377"
	"github.com/consensys/linea-monorepo/prover/maths/bls12377/vector"
	. "github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/stretchr/testify/require"
)

// Deterministically creates randomBls12377Frs
func RandBlsBls12377Fr(pos int) Bls12377Fr {
	res := Bls12377Fr{}
	for i := range res {
		res[i] = byte(pos ^ (i + pos*156) ^ (pos + i*256) ^ (i * pos))
	}
	return res
}

// Creates a empty SMT, test its root hash in hard and tests that
// all the leaves are zero.
func TestTreeInitialization(t *testing.T) {

	config := &smt.Config{
		HashFunc: hashtypes.Keccak,
		Depth:    40,
	}

	tree := smt.NewEmptyTree(config)

	// Only contains empty leaves
	for pos := 0; pos < 1000; pos++ {
		x, _ := tree.GetLeaf(pos)
		require.Equal(t, x, smt.EmptyLeaf())
	}
}

func TestTreeUpdateLeaf(t *testing.T) {
	config := &smt.Config{
		HashFunc: hashtypes.Keccak,
		Depth:    40,
	}

	tree := smt.NewEmptyTree(config)

	// Only contains empty leaves
	for pos := 0; pos < 1000; pos++ {
		// Make a validBls12377Fr
		newLeaf := RandBlsBls12377Fr(pos)
		tree.Update(pos, newLeaf)
		recovered, _ := tree.GetLeaf(pos)
		require.Equal(t, newLeaf, recovered)
	}
}

func TestMerkleProofNative(t *testing.T) {
	config := &smt.Config{
		HashFunc: hashtypes.Keccak,
		Depth:    40,
	}

	tree := smt.NewEmptyTree(config)

	// Only contains empty leaves
	for pos := 0; pos < 1000; pos++ {
		// Make a validBls12377Fr
		oldLeaf, _ := tree.GetLeaf(pos)
		proof, _ := tree.Prove(pos)

		// Directly verify the proof
		valid := proof.Verify(config, oldLeaf, tree.Root)
		require.Truef(t, valid, "pos #%v, proof #%v", pos, proof)
	}
}

func TestMerkleProofWithUpdate(t *testing.T) {
	config := &smt.Config{
		HashFunc: hashtypes.Keccak,
		Depth:    40,
	}

	tree := smt.NewEmptyTree(config)

	// Only contains empty leaves
	for pos := 0; pos < 1000; pos++ {
		proof, _ := tree.Prove(pos)

		// Updat the leaf with a random-looking value before
		// checking the proof
		newLeaf := RandBlsBls12377Fr(pos)
		tree.Update(pos, newLeaf)

		// After updating the old proof should still be valid
		// (because we only changed the current leaf)
		valid := proof.Verify(config, newLeaf, tree.Root)
		require.Truef(t, valid, "pos #%v", pos)
	}
}

func TestBuildFromScratch(t *testing.T) {

	config := &smt.Config{
		HashFunc: hashtypes.Keccak,
		Depth:    10,
	}

	// Generate random field elements and cast them intoBls12377Fres
	leavesFr := vector.Rand(1 << config.Depth)
	leaves := make([]Bls12377Fr, len(leavesFr))
	for i := range leaves {
		leaves[i] = Bls12377Fr(leavesFr[i].Bytes())
	}

	// And generate the
	tree := smt.BuildComplete(leaves, config.HashFunc)

	// Test-Merkle tests the merkle proof point by point
	for i := range leaves {
		proof, _ := tree.Prove(i)
		ok := proof.Verify(config, leaves[i], tree.Root)

		if !ok {
			t.Fatalf("failed to verify pos %v", i)
		}
	}

	// Build the same tree by adding the leaves one by one
	oneByoneTree := smt.NewEmptyTree(config)
	for i := range leaves {
		oneByoneTree.Update(i, leaves[i])
	}

	// We should obtain the same roots
	require.Equal(t, oneByoneTree.Root.Hex(), tree.Root.Hex())

}

func BenchmarkBuildComplete(b *testing.B) {
	config := &smt.Config{
		HashFunc: hashtypes.MiMC,
		Depth:    20,
	}

	// Generate random field elements and cast them intoBls12377Fres
	leavesFr := vector.Rand(1 << config.Depth)
	leaves := make([]Bls12377Fr, len(leavesFr))
	for i := range leaves {
		leaves[i] = Bls12377Fr(leavesFr[i].Bytes())
	}

	for b.Loop() {
		_ = smt.BuildComplete(leaves, config.HashFunc)
	}
}
