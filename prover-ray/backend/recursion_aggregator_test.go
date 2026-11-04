package backend

import (
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/recursion"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/wioptest"
	"github.com/stretchr/testify/require"
)

// shardTestLayout is a shard-proof layout with one bus handle: the extension
// bus product, then 8 VK limbs, then 32 output bytes.
var shardTestLayout = piLayout{
	bus:             []int{0},
	programVK:       seq(1, 8),
	guestOutput:     seq(9, 32),
	numPublicInputs: 41,
}

func seq(first, n int) []int {
	s := make([]int, n)
	for i := range s {
		s[i] = first + i
	}
	return s
}

func randExt(r *rand.Rand) field.Ext {
	var c recursion.Cell
	for i := range recursion.KindE6 {
		c[i].SetUint64(r.Uint64())
	}
	return recursion.CellE6(c)
}

// shardTestNode is a shard proof of shard i, with the given bus product and a
// VK and output shared by every shard.
func shardTestNode(i uint, bus field.Ext) Node {
	pub := make(wiop.PublicInput, shardTestLayout.numPublicInputs)
	pub[0] = field.ElemFromExt(bus)
	for k, pos := range shardTestLayout.programVK {
		pub[pos] = field.ElemFromBase(field.NewElement(uint64(100 + k)))
	}
	for k, pos := range shardTestLayout.guestOutput {
		pub[pos] = field.ElemFromBase(field.NewElement(uint64(200 + k)))
	}
	return Node{Coverage: []uint{i}, Pub: pub}
}

func newTestRecursionAggregator(t *testing.T) *recursionAggregator {
	t.Helper()
	agg, err := newRecursionAggregator(shardTestLayout, wioptest.TestingCompileOptions()...)
	require.NoError(t, err)
	return agg
}

// TestRecursionAggregator_Aggregate checks the aggregation of shard and node
// proofs, and the rejections of disagreeing or malformed children.
func TestRecursionAggregator_Aggregate(t *testing.T) {
	agg := newTestRecursionAggregator(t)
	r := rand.New(rand.NewPCG(7, 8))
	a, b, c := randExt(r), randExt(r), randExt(r)

	t.Run("two shards", func(t *testing.T) {
		n, err := agg.Aggregate(shardTestNode(0, a), shardTestNode(1, b))
		require.NoError(t, err)
		require.Equal(t, []uint{0, 1}, n.Coverage)
		require.NoError(t, agg.node.Verify(n.Proof, n.Pub), "the node proof should verify")
		got, err := agg.node.Decode(n.Pub)
		require.NoError(t, err)
		var want field.Ext
		want.Mul(&a, &b)
		require.True(t, want.Equal(&got.Bus[0]), "the bus products should be multiplied")
	})

	t.Run("a node and a shard", func(t *testing.T) {
		n01, err := agg.Aggregate(shardTestNode(0, a), shardTestNode(1, b))
		require.NoError(t, err)
		n, err := agg.Aggregate(shardTestNode(2, c), n01)
		require.NoError(t, err)
		require.Equal(t, []uint{0, 1, 2}, n.Coverage)
		got, err := agg.node.Decode(n.Pub)
		require.NoError(t, err)
		var want field.Ext
		want.Mul(&a, &b)
		want.Mul(&want, &c)
		require.True(t, want.Equal(&got.Bus[0]), "the three bus products should be multiplied")
	})

	cases := []struct {
		name        string
		left, right Node
		wantErr     string
	}{
		{name: "forged output byte", left: shardTestNode(0, a), right: forged(shardTestNode(1, b), shardTestLayout.guestOutput[5]),
			wantErr: "verifying the node proof"},
		{name: "forged VK limb", left: shardTestNode(0, a), right: forged(shardTestNode(1, b), shardTestLayout.programVK[2]),
			wantErr: "verifying the node proof"},
		{name: "overlapping coverage", left: shardTestNode(0, a), right: shardTestNode(0, b), wantErr: "covered twice"},
		{name: "unknown public-input length", left: shardTestNode(0, a), right: Node{Coverage: []uint{1}, Pub: make(wiop.PublicInput, 5)},
			wantErr: "neither a shard proof"},
		{name: "extension VK limb", left: shardTestNode(0, a), right: extensionAt(shardTestNode(1, b), shardTestLayout.programVK[0]),
			wantErr: "not a base field element"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := agg.Aggregate(tc.left, tc.right)
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

// forged returns n with the public input at pos incremented.
func forged(n Node, pos int) Node {
	n.Pub = append(wiop.PublicInput{}, n.Pub...)
	n.Pub[pos] = n.Pub[pos].Add(field.ElemOne())
	return n
}

// extensionAt returns n with the public input at pos tagged as an extension.
func extensionAt(n Node, pos int) Node {
	n.Pub = append(wiop.PublicInput{}, n.Pub...)
	n.Pub[pos] = field.ElemFromExt(n.Pub[pos].AsExt())
	return n
}

// TestRecursionAggregator_Finalize checks the root checks: complete coverage
// and a balanced bus, on a node proof or a single shard proof.
func TestRecursionAggregator_Finalize(t *testing.T) {
	agg := newTestRecursionAggregator(t)
	r := rand.New(rand.NewPCG(9, 10))
	a := randExt(r)
	var inv field.Ext
	inv.Inverse(&a)
	var one field.Ext
	one.SetOne()

	balanced, err := agg.Aggregate(shardTestNode(0, a), shardTestNode(1, inv))
	require.NoError(t, err)
	unbalanced, err := agg.Aggregate(shardTestNode(0, a), shardTestNode(1, a))
	require.NoError(t, err)

	cases := []struct {
		name      string
		root      Node
		numShards uint
		wantErr   string
	}{
		{name: "balanced node", root: balanced, numShards: 2},
		{name: "single balanced shard", root: shardTestNode(0, one), numShards: 1},
		{name: "unbalanced bus", root: unbalanced, numShards: 2, wantErr: "finalizing the root"},
		{name: "missing shard", root: balanced, numShards: 3, wantErr: "missing shard 2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, err := agg.Finalize(tc.root, tc.numShards)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.NoError(t, agg.root.Verify(root.Proof, root.Pub), "the root proof should verify")
			got, err := agg.root.Decode(root.Pub)
			require.NoError(t, err)
			require.True(t, got.Bus[0].IsOne(), "the root should expose a balanced bus")
		})
	}
}
