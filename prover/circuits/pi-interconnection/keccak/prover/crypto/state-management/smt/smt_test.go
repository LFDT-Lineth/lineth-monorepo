package smt_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/state-management/hashtypes"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/state-management/smt"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/vector"
	. "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/types"
	"github.com/stretchr/testify/require"
)

// Deterministically creates random Bytes32s
func RandBytes32(pos int) Bytes32 {
	res := Bytes32{}
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

func TestMerkleProofNative(t *testing.T) {
	config := &smt.Config{
		HashFunc: hashtypes.Keccak,
		Depth:    40,
	}

	tree := smt.NewEmptyTree(config)

	// Only contains empty leaves
	for pos := 0; pos < 1000; pos++ {
		// Make a valid Bytes32
		oldLeaf, _ := tree.GetLeaf(pos)
		proof, _ := tree.Prove(pos)

		// Directly verify the proof
		valid := proof.Verify(config, oldLeaf, tree.Root)
		require.Truef(t, valid, "pos #%v, proof #%v", pos, proof)
	}
}

func BenchmarkBuildComplete(b *testing.B) {
	config := &smt.Config{
		HashFunc: hashtypes.MiMC,
		Depth:    20,
	}

	// Generate random field elements and cast them into Bytes32es
	leavesFr := vector.Rand(1 << config.Depth)
	leaves := make([]Bytes32, len(leavesFr))
	for i := range leaves {
		leaves[i] = Bytes32(leavesFr[i].Bytes())
	}

	for b.Loop() {
		_ = smt.BuildComplete(leaves, config.HashFunc)
	}
}
