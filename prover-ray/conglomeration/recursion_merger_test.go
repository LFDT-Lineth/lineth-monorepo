package conglomeration

import (
	"testing"

	multisethashing "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/multiset_hashing"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/recursionguests/node"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/recursionguests/verifier"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/messagebus"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/wioptest"
	"github.com/stretchr/testify/require"
)

func newTestRecursionMerger(t *testing.T, sys *wiop.System) *RecursionMerger {
	t.Helper()
	merger, err := NewRecursionMerger(sys, verifier.NewMock, wioptest.TestingCompileOptions()...)
	require.NoError(t, err)
	return merger
}

// requireMerged checks that n is a merged node over sys covering coverage,
// whose proof verifies under program (the leaf or internal program), whose
// bus product is bus and whose key box is program's: empty for the leaf
// program, the internal key otherwise.
func requireMerged(t *testing.T, merger *RecursionMerger, program *node.Program, n Node, coverage []uint, bus field.Gen) {
	t.Helper()
	require.Equal(t, Merged, n.Kind, "a merge should yield a merged node")
	require.Equal(t, coverage, n.Coverage, "the node should cover every child")
	require.NoError(t, program.Verify(n.Proof, n.Pub), "the node proof should verify")
	requireElemEqual(t, bus, messagebus.GetProducts(merger.sys, n.Pub)[0], "the bus product should be multiplied")
	key, err := node.KeyOf(n.Pub)
	require.NoError(t, err)
	want := field.Octuplet{}
	if program != merger.leaf {
		want = merger.internal.Key()
	}
	require.Equal(t, want, key, "the key box should be the program's")
}

// TestRecursionMerger_Merge checks the merges of shard proofs and merged
// nodes, and the rejections of disagreeing or malformed children.
func TestRecursionMerger_Merge(t *testing.T) {
	sys := testSystem(false)
	merger := newTestRecursionMerger(t, sys)

	t.Run("two shards", func(t *testing.T) {
		n, err := merger.Merge([]Node{shardNode(sys, 0, elem(2)), shardNode(sys, 1, elem(3))})
		require.NoError(t, err)
		requireMerged(t, merger, merger.leaf, n, []uint{0, 1}, elem(6))
	})

	t.Run("lifted shard", func(t *testing.T) {
		n, err := merger.Merge([]Node{shardNode(sys, 3, elem(5))})
		require.NoError(t, err)
		requireMerged(t, merger, merger.leaf, n, []uint{3}, elem(5))
	})

	t.Run("full arity, then two merged nodes", func(t *testing.T) {
		children := make([]Node, RecursionTreeArity)
		for i := range children {
			children[i] = shardNode(sys, uint(i), elem(2))
		}
		first, err := merger.Merge(children)
		require.NoError(t, err)
		requireMerged(t, merger, merger.leaf, first, []uint{0, 1, 2, 3, 4, 5}, elem(64))
		second, err := merger.Merge([]Node{shardNode(sys, 6, elem(3)), shardNode(sys, 7, elem(5))})
		require.NoError(t, err)
		n, err := merger.Merge([]Node{first, second})
		require.NoError(t, err)
		requireMerged(t, merger, merger.internal, n, []uint{0, 1, 2, 3, 4, 5, 6, 7}, elem(64*15))

		// Any depth: an internal node and a leaf node merge together.
		third, err := merger.Merge([]Node{shardNode(sys, 8, elem(7))})
		require.NoError(t, err)
		deeper, err := merger.Merge([]Node{n, third})
		require.NoError(t, err)
		requireMerged(t, merger, merger.internal, deeper, []uint{0, 1, 2, 3, 4, 5, 6, 7, 8}, elem(64*15*7))
	})

	merged, err := merger.Merge([]Node{shardNode(sys, 2, elem(1))})
	require.NoError(t, err)
	foreign, err := merger.Merge([]Node{shardNode(sys, 3, elem(1))})
	require.NoError(t, err)
	foreign.Pub[len(foreign.Pub)-1] = elem(9)
	forged := func(pos int, v field.Gen) Node {
		n := shardNode(sys, 1, elem(1))
		n.Pub[pos] = v
		return n
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
			wantErr: "public input 1 is not a base field element"},
		{name: "overlapping coverage", children: []Node{shardNode(sys, 0, elem(1)), shardNode(sys, 0, elem(1))}, wantErr: "covered twice"},
		{name: "mixed kinds", children: []Node{shardNode(sys, 0, elem(1)), merged}, wantErr: "of different kinds"},
		{name: "wrong length", children: []Node{{Coverage: []uint{1}, Pub: make(wiop.PublicInput, 5)}}, wantErr: "has 5 public inputs, want 41"},
		{name: "shard-sized merged node", children: []Node{merged, {Kind: Merged, Coverage: []uint{4}, Pub: shardNode(sys, 4, elem(1)).Pub}},
			wantErr: "has 41 public inputs, want 49"},
		{name: "foreign key box", children: []Node{merged, foreign}, wantErr: "carries a foreign key box"},
		{name: "too many children", children: make([]Node, RecursionTreeArity+1), wantErr: "merging 7 nodes"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := merger.Merge(tc.children)
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

// TestRecursionMerger_Finalize checks the root merge of merged nodes, leaf
// and internal ones alike: complete coverage and a balanced bus, proved by
// the root program.
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
	third, err := merger.Merge([]Node{shardNode(sys, 2, elem(1))})
	require.NoError(t, err)
	internal, err := merger.Merge([]Node{balanced, third})
	require.NoError(t, err)
	fourth, err := merger.Merge([]Node{shardNode(sys, 3, elem(1))})
	require.NoError(t, err)

	cases := []struct {
		name      string
		children  []Node
		numShards uint
		wantErr   string
	}{
		{name: "balanced node", children: []Node{balanced}, numShards: 2},
		{name: "lifted balanced shard", children: []Node{lifted}, numShards: 1},
		{name: "internal node", children: []Node{internal}, numShards: 3},
		{name: "two leaf nodes", children: []Node{balanced, third}, numShards: 3},
		{name: "internal and leaf nodes", children: []Node{internal, fourth}, numShards: 4},
		{name: "unbalanced bus", children: []Node{unbalanced}, numShards: 2, wantErr: "finalizing the root"},
		{name: "missing shard", children: []Node{balanced}, numShards: 3, wantErr: "missing shard 2"},
		{name: "overlapping children", children: []Node{balanced, lifted}, numShards: 2, wantErr: "covered twice"},
		{name: "shard proof", children: []Node{shardNode(sys, 0, elem(1))}, numShards: 1, wantErr: "want a merged node"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, err := merger.Finalize(tc.children, tc.numShards)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.NoError(t, merger.VerifyRoot(root), "the root proof should verify")
			requireElemEqual(t, elem(1), messagebus.GetProducts(sys, root.Pub)[0], "the root should expose a balanced bus")
			key, err := node.KeyOf(root.Pub)
			require.NoError(t, err)
			require.Equal(t, merger.internal.Key(), key, "the root should expose the internal key")
		})
	}
}

// TestRecursionMerger_VerifiesChildren checks that a child whose proof does
// not verify in Go is rejected: a shard proof with a size for a module the
// shard system has not, and a merged node carrying another node's proof.
func TestRecursionMerger_VerifiesChildren(t *testing.T) {
	sys := testSystem(false)
	merger := newTestRecursionMerger(t, sys)
	forged := shardNode(sys, 0, elem(1))
	forged.Proof.DynamicSizes = map[int]int{0: 8}
	_, err := merger.Merge([]Node{forged})
	require.ErrorContains(t, err, "verifying the proof in Go")

	a, err := merger.Merge([]Node{shardNode(sys, 0, elem(1))})
	require.NoError(t, err)
	b, err := merger.Merge([]Node{shardNode(sys, 1, elem(2))})
	require.NoError(t, err)
	b.Proof = a.Proof
	_, err = merger.Merge([]Node{a, b})
	require.ErrorContains(t, err, "verifying the proof in Go")
}

// TestRecursionMerger_SharedRandomness checks that the contributions add up
// in a merge and that the root's γ must be derived from them, named by the
// merger before a proof is attempted (the root program's own rejection is
// node.TestNew_Seed).
func TestRecursionMerger_SharedRandomness(t *testing.T) {
	sys := testSystem(true)
	merger := newTestRecursionMerger(t, sys)
	a := multisethashing.Hash(field.Octuplet{field.NewElement(1)})
	b := multisethashing.Hash(field.Octuplet{field.NewElement(2)})
	sum := multisethashing.Combine(a, b)
	gamma := multisethashing.ToSeed(sum)

	n, err := merger.Merge([]Node{seededShardNode(sys, 0, gamma, a), seededShardNode(sys, 1, gamma, b)})
	require.NoError(t, err)
	require.Equal(t, sum, messagebus.GetSharedRandomnessContribution(sys, n.Pub), "the contributions should add up")
	require.Equal(t, gamma, messagebus.GetSharedRandomnessSeed(sys, n.Pub), "the seed should be propagated")
	_, err = merger.Finalize([]Node{n}, 2)
	require.NoError(t, err, "a seed derived from the contributions should finalize")
	la, err := merger.Merge([]Node{seededShardNode(sys, 0, gamma, a)})
	require.NoError(t, err)
	lb, err := merger.Merge([]Node{seededShardNode(sys, 1, gamma, b)})
	require.NoError(t, err)
	_, err = merger.Finalize([]Node{la, lb}, 2)
	require.NoError(t, err, "the root merge should add the children's contributions before deriving the seed")

	_, err = merger.Merge([]Node{seededShardNode(sys, 0, gamma, a), seededShardNode(sys, 1, field.Octuplet{}, b)})
	require.ErrorContains(t, err, "have different SharedRandomnessSeed_0")

	lifted, err := merger.Merge([]Node{seededShardNode(sys, 0, gamma, a)})
	require.NoError(t, err)
	_, err = merger.Finalize([]Node{lifted}, 1)
	require.ErrorContains(t, err, "not derived from the shards' contributions")
}

// TestRecursionMerger_Reduce reduces more shards than two full leaf merges
// to a root through the reducer, merges running in the background: two full
// leaf merges and a lifted shard, an internal node over them, then the root.
// The bus products balance only over all the shards, and γ derives from all
// the contributions.
func TestRecursionMerger_Reduce(t *testing.T) {
	sys := testSystem(true)
	merger := newTestRecursionMerger(t, sys)
	const numShards = 2*RecursionTreeArity + 1
	contributions := make([]multisethashing.MSetHash, numShards)
	sum := multisethashing.Identity()
	for i := range contributions {
		contributions[i] = multisethashing.Hash(field.Octuplet{field.NewElement(uint64(i + 1))})
		sum = multisethashing.Combine(sum, contributions[i])
	}
	gamma := multisethashing.ToSeed(sum)
	nodes := make([]Node, numShards)
	product := elem(1)
	for i := range nodes {
		nodes[i] = seededShardNode(sys, uint(i), gamma, contributions[i])
		bus := elem(uint(i + 2))
		if i == numShards-1 {
			bus = product.Inverse()
		}
		nodes[i].Pub[busPos] = bus
		product = product.Mul(bus)
	}

	last := func(children []Node) (Node, error) { return merger.Finalize(children, numShards) }
	root, err := reduceWith(merger, 2, last, nodes...)
	require.NoError(t, err)
	require.NoError(t, merger.VerifyRoot(root), "the root proof should verify")
	coverage := make([]uint, numShards)
	for i := range coverage {
		coverage[i] = uint(i)
	}
	require.Equal(t, coverage, root.Coverage, "the root should cover every shard")
	requireElemEqual(t, elem(1), messagebus.GetProducts(sys, root.Pub)[0], "the root should expose a balanced bus")
	require.Equal(t, sum, messagebus.GetSharedRandomnessContribution(sys, root.Pub), "the contributions should add up")
	require.Equal(t, gamma, messagebus.GetSharedRandomnessSeed(sys, root.Pub), "the seed should be propagated")
	key, err := node.KeyOf(root.Pub)
	require.NoError(t, err)
	require.Equal(t, merger.internal.Key(), key, "the root should expose the internal key")
}
