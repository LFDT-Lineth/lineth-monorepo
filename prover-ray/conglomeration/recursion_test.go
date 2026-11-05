package conglomeration

import (
	"testing"

	multisethashing "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/multiset_hashing"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/messagebus"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/wioptest"
	"github.com/stretchr/testify/require"
)

func newTestRecursionMerger(t *testing.T, sys *wiop.System) *RecursionMerger {
	t.Helper()
	merger, err := NewRecursionMerger(sys, wioptest.TestingCompileOptions()...)
	require.NoError(t, err)
	return merger
}

// requireMerged checks that node is a merged node over sys covering coverage,
// whose proof verifies under the node program and whose bus product is bus.
func requireMerged(t *testing.T, merger *RecursionMerger, node Node, coverage []uint, bus field.Gen) {
	t.Helper()
	require.Equal(t, Merged, node.Kind, "a merge should yield a merged node")
	require.Equal(t, coverage, node.Coverage, "the node should cover every child")
	require.NoError(t, merger.node.Verify(node.Proof, node.Pub), "the node proof should verify")
	requireElemEqual(t, bus, messagebus.GetProducts(merger.sys, node.Pub)[0], "the bus product should be multiplied")
}

// TestRecursionMerger_Merge checks the merges of shard proofs and merged
// nodes, and the rejections of disagreeing or malformed children.
func TestRecursionMerger_Merge(t *testing.T) {
	sys := testSystem(false)
	merger := newTestRecursionMerger(t, sys)

	t.Run("two shards", func(t *testing.T) {
		node, err := merger.Merge([]Node{shardNode(sys, 0, elem(2)), shardNode(sys, 1, elem(3))})
		require.NoError(t, err)
		requireMerged(t, merger, node, []uint{0, 1}, elem(6))
	})

	t.Run("lifted shard", func(t *testing.T) {
		node, err := merger.Merge([]Node{shardNode(sys, 3, elem(5))})
		require.NoError(t, err)
		requireMerged(t, merger, node, []uint{3}, elem(5))
	})

	t.Run("full arity, then two merged nodes", func(t *testing.T) {
		children := make([]Node, RecursionTreeArity)
		for i := range children {
			children[i] = shardNode(sys, uint(i), elem(2))
		}
		first, err := merger.Merge(children)
		require.NoError(t, err)
		requireMerged(t, merger, first, []uint{0, 1, 2, 3, 4, 5}, elem(64))
		second, err := merger.Merge([]Node{shardNode(sys, 6, elem(3)), shardNode(sys, 7, elem(5))})
		require.NoError(t, err)
		node, err := merger.Merge([]Node{first, second})
		require.NoError(t, err)
		requireMerged(t, merger, node, []uint{0, 1, 2, 3, 4, 5, 6, 7}, elem(64*15))
	})

	merged := shardNode(sys, 2, elem(1))
	merged.Kind = Merged
	forged := func(pos int, v field.Gen) Node {
		node := shardNode(sys, 1, elem(1))
		node.Pub[pos] = v
		return node
	}
	cases := []struct {
		name     string
		children []Node
		wantErr  string
	}{
		{name: "forged output byte", children: []Node{shardNode(sys, 0, elem(1)), forged(guestOutputPos+5, elem(8))},
			wantErr: "nodes [0] and [1] have different GuestPublicOutputs_5"},
		{name: "forged VK limb", children: []Node{shardNode(sys, 0, elem(1)), forged(programVKPos+2, elem(8))},
			wantErr: "nodes [0] and [1] have different ProgramVerificationKey_2"},
		{name: "extension VK limb", children: []Node{forged(programVKPos, field.ElemFromExt(elem(7).AsExt()))},
			wantErr: "ProgramVerificationKey_0 is not a base field element"},
		{name: "overlapping coverage", children: []Node{shardNode(sys, 0, elem(1)), shardNode(sys, 0, elem(1))}, wantErr: "covered twice"},
		{name: "mixed kinds", children: []Node{shardNode(sys, 0, elem(1)), merged}, wantErr: "of different kinds"},
		{name: "wrong length", children: []Node{{Coverage: []uint{1}, Pub: make(wiop.PublicInput, 5)}}, wantErr: "has 5 public inputs"},
		{name: "too many children", children: make([]Node, RecursionTreeArity+1), wantErr: "merging 7 nodes"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := merger.Merge(tc.children)
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

// TestRecursionMerger_Finalize checks the root checks on a merged node: complete
// coverage and a balanced bus, proved by the root program.
func TestRecursionMerger_Finalize(t *testing.T) {
	sys := testSystem(false)
	merger := newTestRecursionMerger(t, sys)
	inv := elem(7).Inverse()
	balanced, err := merger.Merge([]Node{shardNode(sys, 0, elem(7)), shardNode(sys, 1, inv)})
	require.NoError(t, err)
	unbalanced, err := merger.Merge([]Node{shardNode(sys, 0, elem(7)), shardNode(sys, 1, elem(7))})
	require.NoError(t, err)
	lifted, err := merger.Merge([]Node{shardNode(sys, 0, elem(1))})
	require.NoError(t, err)

	cases := []struct {
		name      string
		root      Node
		numShards uint
		wantErr   string
	}{
		{name: "balanced node", root: balanced, numShards: 2},
		{name: "lifted balanced shard", root: lifted, numShards: 1},
		{name: "unbalanced bus", root: unbalanced, numShards: 2, wantErr: "finalizing the root"},
		{name: "missing shard", root: balanced, numShards: 3, wantErr: "missing shard 2"},
		{name: "shard proof", root: shardNode(sys, 0, elem(1)), numShards: 1, wantErr: "want a merged node"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, err := merger.Finalize(tc.root, tc.numShards)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.NoError(t, merger.VerifyRoot(root), "the root proof should verify")
			requireElemEqual(t, elem(1), messagebus.GetProducts(sys, root.Pub)[0], "the root should expose a balanced bus")
		})
	}
}

// TestRecursionMerger_SharedRandomness checks that the contributions add up
// in a merge and that the root's γ must be derived from them.
func TestRecursionMerger_SharedRandomness(t *testing.T) {
	sys := testSystem(true)
	merger := newTestRecursionMerger(t, sys)
	a := multisethashing.Hash(field.Octuplet{field.NewElement(1)})
	b := multisethashing.Hash(field.Octuplet{field.NewElement(2)})
	sum := multisethashing.Combine(a, b)
	gamma := multisethashing.ToSeed(sum)

	node, err := merger.Merge([]Node{seededShardNode(sys, 0, gamma, a), seededShardNode(sys, 1, gamma, b)})
	require.NoError(t, err)
	require.Equal(t, sum, messagebus.GetSharedRandomnessContribution(sys, node.Pub), "the contributions should add up")
	require.Equal(t, gamma, messagebus.GetSharedRandomnessSeed(sys, node.Pub), "the seed should be propagated")
	_, err = merger.Finalize(node, 2)
	require.NoError(t, err, "a seed derived from the contributions should finalize")

	_, err = merger.Merge([]Node{seededShardNode(sys, 0, gamma, a), seededShardNode(sys, 1, field.Octuplet{}, b)})
	require.ErrorContains(t, err, "have different SharedRandomnessSeed_0")

	lifted, err := merger.Merge([]Node{seededShardNode(sys, 0, gamma, a)})
	require.NoError(t, err)
	_, err = merger.Finalize(lifted, 1)
	require.ErrorContains(t, err, "not derived from the shards' contributions")
}
