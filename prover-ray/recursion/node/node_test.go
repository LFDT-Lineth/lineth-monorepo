package node_test

import (
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/recursion"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/recursion/node"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/wioptest"
	"github.com/stretchr/testify/require"
)

var testOpts = wioptest.TestingCompileOptions()

// testLayout is the layout of a shard proof with numBus bus handles: 4 output
// bytes, 2 VK limbs, the extension bus products, then, when seeded, a 2-limb
// seed and a 3-limb contribution.
func testLayout(numBus int, seeded bool) node.Layout {
	var l node.Layout
	add := func(tag wiop.PublicInputTag, n int, ext bool, op node.Op) {
		for i := range n {
			l = append(l, node.Role{Tag: tag, Index: i, Ext: ext, Op: op})
		}
	}
	add("Out", 4, false, node.OpEqual)
	add("VK", 2, false, node.OpEqual)
	add("Bus", numBus, true, node.OpMul)
	if seeded {
		add("Seed", 2, false, node.OpEqual)
		add("Contribution", 3, false, node.OpAdd)
	}
	return l
}

// tree compiles the three programs of a shard layout.
type tree struct {
	leaf, internal, root *node.Program
}

func newTree(t *testing.T, l node.Layout) tree {
	t.Helper()
	leaf, err := node.NewLeaf(l, testOpts...)
	require.NoError(t, err)
	internal, err := node.NewInternal(l, testOpts...)
	require.NoError(t, err)
	root, err := node.NewRoot(l, internal.Key(), testOpts...)
	require.NoError(t, err)
	return tree{leaf, internal, root}
}

// randExt returns a random E6 value.
func randExt(r *rand.Rand) field.Ext {
	var c recursion.Cell
	for i := range recursion.KindE6 {
		c[i].SetUint64(r.Uint64())
	}
	return recursion.CellE6(c)
}

// childPub returns the public inputs of a shard of the layout: random bus
// products and contributions, and the values 100+pos of the equal roles,
// shared by every child.
func childPub(r *rand.Rand, l node.Layout) wiop.PublicInput {
	pub := make(wiop.PublicInput, len(l))
	for pos, role := range l {
		switch {
		case role.Ext:
			pub[pos] = field.ElemFromExt(randExt(r))
		case role.Op == node.OpAdd:
			pub[pos] = field.ElemFromBase(field.NewElement(r.Uint64() % 1000))
		default:
			pub[pos] = field.ElemFromBase(field.NewElement(uint64(100 + pos)))
		}
	}
	return pub
}

// want returns the public inputs a node of the children should expose over
// the shard layout l: the children combined, then the key box.
func want(l node.Layout, key field.Octuplet, children ...wiop.PublicInput) wiop.PublicInput {
	pub := append(wiop.PublicInput{}, children[0][:len(l)]...)
	for pos, role := range l {
		for _, child := range children[1:] {
			switch role.Op {
			case node.OpMul:
				pub[pos] = pub[pos].Mul(child[pos])
			case node.OpAdd:
				pub[pos] = pub[pos].Add(child[pos])
			}
		}
	}
	for _, limb := range key {
		pub = append(pub, field.ElemFromBase(limb))
	}
	return pub
}

func requirePubEqual(t *testing.T, want, got wiop.PublicInput, msg string) {
	t.Helper()
	require.Len(t, got, len(want), msg)
	for pos := range want {
		diff := got[pos].Sub(want[pos])
		require.True(t, diff.IsZero(), "%s: public input %d", msg, pos)
		require.Equal(t, want[pos].IsBase(), got[pos].IsBase(), "%s: public input %d width", msg, pos)
	}
}

// shards returns n shards of the layout.
func shards(r *rand.Rand, l node.Layout, n int) []node.Child {
	children := make([]node.Child, n)
	for c := range children {
		children[c] = node.Child{Pub: childPub(r, l)}
	}
	return children
}

// pubs returns the public inputs of children.
func pubs(children []node.Child) []wiop.PublicInput {
	res := make([]wiop.PublicInput, len(children))
	for c, child := range children {
		res[c] = child.Pub
	}
	return res
}

// TestNew_Aggregates checks that a node proof exposes the node layout, with
// the products multiplied, the contributions added, the shared values
// propagated and the key box set, for 1 to Arity children of a leaf, then of
// an internal node over leaf and internal nodes.
func TestNew_Aggregates(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	l := testLayout(2, true)
	tr := newTree(t, l)
	require.NotEqual(t, field.Octuplet{}, tr.leaf.Key(), "a program should have a key")
	require.NotEqual(t, tr.leaf.Key(), tr.internal.Key(), "the programs should have different keys")
	require.NotEqual(t, tr.internal.Key(), tr.root.Key(), "the programs should have different keys")
	for _, p := range []*node.Program{tr.leaf, tr.internal, tr.root} {
		for pos, role := range p.Layout {
			_, got := p.Sys.LookupPublicInputByTag(role.Tag, role.Index)
			require.Equal(t, pos, got, "%s_%d should be at its position in the layout", role.Tag, role.Index)
		}
	}

	var leaves []node.Child
	for _, n := range []int{1, 2, node.Arity} {
		children := shards(r, l, n)
		_, pub, err := tr.leaf.ProveChildren(field.Octuplet{}, children...)
		require.NoError(t, err, "%d shards", n)
		requirePubEqual(t, want(l, field.Octuplet{}, pubs(children)...), pub, "the leaf should combine its shards")
		leaves = append(leaves, node.Child{Pub: pub, Leaf: true})
	}

	key := tr.internal.Key()
	_, pub, err := tr.internal.ProveChildren(key, leaves...)
	require.NoError(t, err, "an internal node over leaves")
	requirePubEqual(t, want(l, key, pubs(leaves)...), pub, "the internal node should combine the leaves")
	internal := node.Child{Pub: pub}

	children := append([]node.Child{internal}, leaves...)
	_, pub, err = tr.internal.ProveChildren(key, children...)
	require.NoError(t, err, "an internal node over an internal node and leaves")
	requirePubEqual(t, want(l, key, pubs(children)...), pub, "the internal node should combine both kinds")
	got, err := node.KeyOf(pub)
	require.NoError(t, err)
	require.Equal(t, key, got, "the key box should be the internal key")
}

// TestNew_Rejects checks that children disagreeing on a propagated value, a
// key box breaking the key rule, a root with a foreign key box or an
// unbalanced root fail at verification.
func TestNew_Rejects(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	l := testLayout(1, false)
	tr := newTree(t, l)
	key := tr.internal.Key()
	leaf := func() node.Child {
		_, pub, err := tr.leaf.ProveChildren(field.Octuplet{}, shards(r, l, 1)...)
		require.NoError(t, err)
		return node.Child{Pub: pub, Leaf: true}
	}
	internal := func() node.Child {
		_, pub, err := tr.internal.ProveChildren(key, leaf())
		require.NoError(t, err)
		return node.Child{Pub: pub}
	}
	foreign := key
	foreign[0].Add(&foreign[0], new(field.Element).SetOne())

	cases := []struct {
		name string
		run  func() error
	}{
		{name: "different VK limb", run: func() error {
			a, b := shards(r, l, 2)[0], shards(r, l, 1)[0]
			b.Pub[5] = b.Pub[5].Add(field.ElemOne())
			_, _, err := tr.leaf.ProveChildren(field.Octuplet{}, a, b)
			return err
		}},
		{name: "different output byte", run: func() error {
			a, b := shards(r, l, 2)[0], shards(r, l, 1)[0]
			b.Pub[0] = b.Pub[0].Add(field.ElemOne())
			_, _, err := tr.leaf.ProveChildren(field.Octuplet{}, a, b)
			return err
		}},
		{name: "node child with a different key box", run: func() error {
			_, _, err := tr.internal.ProveChildren(foreign, internal())
			return err
		}},
		{name: "node child flagged as a leaf", run: func() error {
			child := internal()
			child.Leaf = true
			_, _, err := tr.internal.ProveChildren(key, child)
			return err
		}},
		{name: "leaf child flagged as a node", run: func() error {
			child := leaf()
			child.Leaf = false
			_, _, err := tr.internal.ProveChildren(key, child)
			return err
		}},
		{name: "root with a foreign key box", run: func() error {
			_, _, err := tr.root.ProveChildren(foreign, leaf())
			return err
		}},
		{name: "unbalanced root", run: func() error {
			_, _, err := tr.root.ProveChildren(key, leaf())
			return err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.ErrorContains(t, tc.run(), "verifying the node proof")
		})
	}

	t.Run("balanced root", func(t *testing.T) {
		a, b := shards(r, l, 2)[0], shards(r, l, 1)[0]
		bus := a.Pub[6].AsExt()
		var inv field.Ext
		inv.Inverse(&bus)
		b.Pub[6] = field.ElemFromExt(inv)
		_, pub, err := tr.leaf.ProveChildren(field.Octuplet{}, a, b)
		require.NoError(t, err)
		_, pub, err = tr.root.ProveChildren(key, node.Child{Pub: pub, Leaf: true})
		require.NoError(t, err)
		product := pub[6].AsExt()
		require.True(t, product.IsOne(), "the root should expose a balanced bus")
		got, err := node.KeyOf(pub)
		require.NoError(t, err)
		require.Equal(t, key, got, "the root should expose the internal key")
	})
}

// TestNew_InvalidArguments checks the rejections of malformed layouts and
// children.
func TestNew_InvalidArguments(t *testing.T) {
	r := rand.New(rand.NewPCG(5, 6))
	l := testLayout(1, false)
	tr := newTree(t, l)

	_, err := node.NewLeaf(nil, testOpts...)
	require.ErrorContains(t, err, "empty layout")
	_, err = node.NewLeaf(l.WithKey(), testOpts...)
	require.ErrorContains(t, err, "has a key box")
	_, err = node.KeyOf(childPub(r, l)[:3])
	require.ErrorContains(t, err, "no key box")

	short := childPub(r, l)[:3]
	extVK := childPub(r, l)
	extVK[5] = field.ElemFromExt(extVK[5].AsExt())
	cases := []struct {
		name     string
		program  *node.Program
		children []node.Child
		wantErr  string
	}{
		{name: "no child", program: tr.leaf, children: nil, wantErr: "0 children, want 1 to 6"},
		{name: "too many children", program: tr.leaf, children: shards(r, l, node.Arity+1), wantErr: "7 children, want 1 to 6"},
		{name: "wrong length", program: tr.leaf, children: []node.Child{{Pub: short}}, wantErr: "3 public inputs, want 7"},
		{name: "shard as a node child", program: tr.internal, children: shards(r, l, 1), wantErr: "7 public inputs, want 15"},
		{name: "extension VK limb", program: tr.leaf, children: []node.Child{{Pub: extVK}}, wantErr: "VK_1 is not a base field element"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.program.Inputs(field.Octuplet{}, tc.children...)
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

// TestLayoutOf checks that a system's public inputs are laid out by tag and
// position, and that a public input without a role is rejected.
func TestLayoutOf(t *testing.T) {
	sys := wiop.NewSystemf("layout-test")
	r0 := sys.NewRound()
	ctx := sys.Context.Childf("test")
	sys.RegisterPublicInputs("B", r0.NewCell(ctx.Childf("b0"), false), 0)
	sys.RegisterPublicInputs("A", r0.NewCell(ctx.Childf("a"), true), 0)
	sys.RegisterPublicInputs("B", r0.NewCell(ctx.Childf("b1"), false), 1)

	l, err := node.LayoutOf(sys, map[wiop.PublicInputTag]node.Op{"A": node.OpMul, "B": node.OpEqual})
	require.NoError(t, err)
	require.Equal(t, node.Layout{
		{Tag: "B", Index: 0, Op: node.OpEqual},
		{Tag: "A", Index: 0, Ext: true, Op: node.OpMul},
		{Tag: "B", Index: 1, Op: node.OpEqual},
	}, l, "roles should be at the positions of their public inputs")
	require.Len(t, l.WithKey(), 3+node.NumKey, "the node layout should append the key box")

	_, err = node.LayoutOf(sys, map[wiop.PublicInputTag]node.Op{"A": node.OpMul})
	require.ErrorContains(t, err, "public input 0 (B_0) has no role")
}
