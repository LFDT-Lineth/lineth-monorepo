package smt_koalabear

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/stretchr/testify/require"
)

// Creates a empty SMT, test its root hash in hard and tests that
// all the leaves are zero.
func TestTreeInitialization(t *testing.T) {

	tree := NewEmptyTree()

	// Only contains empty leaves
	for pos := 0; pos < 1000; pos++ {
		x, _ := tree.GetLeaf(pos)
		require.Equal(t, x, EmptyLeaf())
	}
}

func TestTreeUpdateLeaf(t *testing.T) {

	tree := NewEmptyTree()

	// Only contains empty leaves
	for pos := 0; pos < 1000; pos++ {
		var newLeaf field.Octuplet
		for i := range newLeaf {
			newLeaf[i] = field.RandomElement()
		}
		tree.Update(pos, newLeaf)
		recovered, _ := tree.GetLeaf(pos)
		require.Equal(t, newLeaf, recovered)
	}
}
func TestMerkleProofNative(t *testing.T) {

	tree := NewEmptyTree()

	// Only contains empty leaves
	for pos := 0; pos < 1000; pos++ {
		oldLeaf, _ := tree.GetLeaf(pos)
		proof, _ := tree.Prove(pos)

		// Directly verify the proof
		err := Verify(&proof, oldLeaf, tree.Root)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestMerkleProofWithUpdate(t *testing.T) {

	tree := NewEmptyTree()

	// Only contains empty leaves
	for pos := 0; pos < 1000; pos++ {
		proof, _ := tree.Prove(pos)

		// Updat the leaf with a random-looking value before
		// checking the proof
		var newLeaf field.Octuplet
		for i := range newLeaf {
			newLeaf[i] = field.RandomElement()
		}
		tree.Update(pos, newLeaf)

		err := Verify(&proof, newLeaf, tree.Root)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestBuildFromScratch(t *testing.T) {

	depth := 8

	// Generate random field elements and cast them into field.Octuplet
	// every 8 field elements forms a leaf
	leavesFr := make([]field.Element, 8*(1<<depth))
	for i := 0; i < len(leavesFr); i++ {
		leavesFr[i] = field.RandomElement()
	}

	leaves := make([]field.Octuplet, len(leavesFr)/8)
	for i := range leaves {
		var arr [8]field.Element
		copy(arr[:], leavesFr[8*i:8*i+8])
		leaves[i] = arr
	}

	// And generate the
	tree := NewTree(leaves)

	// Test-Merkle tests the merkle proof point by point
	for i := range leaves {
		proof, _ := tree.Prove(i)
		err := Verify(&proof, leaves[i], tree.Root)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Build the same tree by adding the leaves one by one
	oneByoneTree := NewEmptyTree(depth)
	for i := range leaves {
		oneByoneTree.Update(i, leaves[i])
	}

	// We should obtain the same roots
	require.Equal(t, oneByoneTree.Root, tree.Root)

}

// BenchmarkNewTree benchmarks the creation of a SMT from scratch
func BenchmarkNewTree(b *testing.B) {

	const depth = 18
	const numLeaves = 1 << depth
	leaves := make([]field.Octuplet, numLeaves)
	// Generate random leaves
	for i := 0; i < numLeaves; i++ {
		for j := 0; j < len(field.Octuplet{}); j++ {
			leaves[i][j] = field.RandomElement()
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewTree(leaves)
	}
}
