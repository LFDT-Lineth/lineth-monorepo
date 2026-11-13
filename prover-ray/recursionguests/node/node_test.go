package node_test

import (
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/poseidon2"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/recursion"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/recursion/verifier"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/recursionguests/node"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/wioptest"
	"github.com/stretchr/testify/require"
)

var testOpts = wioptest.TestingCompileOptions()

// shardKey is the key the shard proofs of the tests are claimed under.
var shardKey = field.Octuplet{field.NewElement(42)}

// stubSystem is a bare system of the layout l under key, uncompiled, whose
// Go verification accepts any public inputs with an empty proof: the shard
// system of the tests, whose shard proofs are public inputs without a proof,
// so that what rejects a shard child is the circuit. A node child carries
// the real proof of its program.
func stubSystem(l node.Layout, key field.Octuplet) *wiop.System {
	sys := wiop.NewSystemf("stub")
	r0 := sys.NewRound()
	ctx := sys.Context.Childf("pub")
	for pos, role := range l {
		sys.RegisterPublicInputs(role.Tag, r0.NewCell(ctx.Childf("%d", pos), role.Ext), role.Index)
	}
	sys.PrecomputedCommitment = key
	return sys
}

// testLayout is the layout of a shard proof with numBus bus handles: 4 output
// bytes, 2 VK limbs, the extension bus products, then, when seeded, a
// digest-sized seed and a 3-limb contribution.
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
		add("Seed", node.NumKey, false, node.OpSeed)
		add("Contribution", 3, false, node.OpAdd)
	}
	return l
}

// tree compiles the three programs of a shard layout over a stub shard
// system under shardKey.
type tree struct {
	leaf, internal, root *node.Program
}

func newTree(t *testing.T, l node.Layout) tree {
	t.Helper()
	leaf, err := node.NewLeaf(l, stubSystem(l, shardKey), verifier.NewMock, testOpts...)
	require.NoError(t, err)
	internal, err := node.NewInternal(l, leaf, verifier.NewMock, testOpts...)
	require.NoError(t, err)
	root, err := node.NewRoot(l, leaf, internal, verifier.NewMock, testOpts...)
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
		proof, pub, err := tr.leaf.ProveChildren(field.Octuplet{}, children...)
		require.NoError(t, err, "%d shards", n)
		requirePubEqual(t, want(l, field.Octuplet{}, pubs(children)...), pub, "the leaf should combine its shards")
		leaves = append(leaves, node.Child{Proof: proof, Pub: pub, Leaf: true})
	}

	key := tr.internal.Key()
	proof, pub, err := tr.internal.ProveChildren(key, leaves...)
	require.NoError(t, err, "an internal node over leaves")
	requirePubEqual(t, want(l, key, pubs(leaves)...), pub, "the internal node should combine the leaves")
	internal := node.Child{Proof: proof, Pub: pub}

	children := append([]node.Child{internal}, leaves...)
	_, pub, err = tr.internal.ProveChildren(key, children...)
	require.NoError(t, err, "an internal node over an internal node and leaves")
	requirePubEqual(t, want(l, key, pubs(children)...), pub, "the internal node should combine both kinds")
	got, err := node.KeyOf(pub)
	require.NoError(t, err)
	require.Equal(t, key, got, "the key box should be the internal key")
}

// TestNew_Rejects checks that children disagreeing on a propagated value, a
// root with a foreign key box or an unbalanced root fail at verification, and
// that a node child claimed under a key its proof does not verify under, a
// foreign box or the wrong leaf flag, fails when its proof is laid out.
func TestNew_Rejects(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	l := testLayout(1, false)
	tr := newTree(t, l)
	key := tr.internal.Key()
	leaf := func() node.Child {
		proof, pub, err := tr.leaf.ProveChildren(field.Octuplet{}, shards(r, l, 1)...)
		require.NoError(t, err)
		return node.Child{Proof: proof, Pub: pub, Leaf: true}
	}
	internal := func() node.Child {
		proof, pub, err := tr.internal.ProveChildren(key, leaf())
		require.NoError(t, err)
		return node.Child{Proof: proof, Pub: pub}
	}
	foreign := key
	foreign[0].Add(&foreign[0], new(field.Element).SetOne())
	const circuit, inGo = "verifying the node proof", "verifying the proof in Go"

	cases := []struct {
		name, wantErr string
		run           func() error
	}{
		{name: "different VK limb", wantErr: circuit, run: func() error {
			a, b := shards(r, l, 2)[0], shards(r, l, 1)[0]
			b.Pub[5] = b.Pub[5].Add(field.ElemOne())
			_, _, err := tr.leaf.ProveChildren(field.Octuplet{}, a, b)
			return err
		}},
		{name: "different output byte", wantErr: circuit, run: func() error {
			a, b := shards(r, l, 2)[0], shards(r, l, 1)[0]
			b.Pub[0] = b.Pub[0].Add(field.ElemOne())
			_, _, err := tr.leaf.ProveChildren(field.Octuplet{}, a, b)
			return err
		}},
		{name: "node child with a different key box", wantErr: circuit, run: func() error {
			_, _, err := tr.internal.ProveChildren(foreign, internal())
			return err
		}},
		{name: "node child flagged as a leaf", wantErr: inGo, run: func() error {
			child := internal()
			child.Leaf = true
			_, _, err := tr.internal.ProveChildren(key, child)
			return err
		}},
		{name: "leaf child flagged as a node", wantErr: inGo, run: func() error {
			child := leaf()
			child.Leaf = false
			_, _, err := tr.internal.ProveChildren(key, child)
			return err
		}},
		{name: "root with a foreign key box", wantErr: circuit, run: func() error {
			_, _, err := tr.root.ProveChildren(foreign, leaf())
			return err
		}},
		{name: "unbalanced root", wantErr: circuit, run: func() error {
			_, _, err := tr.root.ProveChildren(key, leaf())
			return err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.ErrorContains(t, tc.run(), tc.wantErr)
		})
	}

	t.Run("balanced root", func(t *testing.T) {
		a, b := shards(r, l, 2)[0], shards(r, l, 1)[0]
		bus := a.Pub[6].AsExt()
		var inv field.Ext
		inv.Inverse(&bus)
		b.Pub[6] = field.ElemFromExt(inv)
		proof, pub, err := tr.leaf.ProveChildren(field.Octuplet{}, a, b)
		require.NoError(t, err)
		_, pub, err = tr.root.ProveChildren(key, node.Child{Proof: proof, Pub: pub, Leaf: true})
		require.NoError(t, err)
		product := pub[6].AsExt()
		require.True(t, product.IsOne(), "the root should expose a balanced bus")
		got, err := node.KeyOf(pub)
		require.NoError(t, err)
		require.Equal(t, key, got, "the root should expose the internal key")
	})
}

// TestNew_Seed checks that the root accepts the seed that is the digest of
// the contributions added over the shards, and rejects any other.
func TestNew_Seed(t *testing.T) {
	r := rand.New(rand.NewPCG(7, 8))
	l := testLayout(0, true)
	tr := newTree(t, l)
	key := tr.internal.Key()
	setSeed := func(children []node.Child, seed field.Octuplet) {
		for _, child := range children {
			for pos, role := range l {
				if role.Op == node.OpSeed {
					child.Pub[pos] = field.ElemFromBase(seed[role.Index])
				}
			}
		}
	}
	// prove merges the shards into a leaf node and finalizes it.
	prove := func(children []node.Child) error {
		proof, pub, err := tr.leaf.ProveChildren(field.Octuplet{}, children...)
		if err != nil {
			return err
		}
		_, _, err = tr.root.ProveChildren(key, node.Child{Proof: proof, Pub: pub, Leaf: true})
		return err
	}

	children := shards(r, l, 2)
	sum := want(l, field.Octuplet{}, pubs(children)...)
	h := poseidon2.NewMDHasher()
	for pos, role := range l {
		if role.Op == node.OpAdd {
			h.WriteElements(sum[pos].AsBase())
		}
	}
	seed := h.SumDigest()
	setSeed(children, seed)
	require.NoError(t, prove(children), "the root should accept the seed derived from the contributions")

	seed[0].Add(&seed[0], new(field.Element).SetOne())
	setSeed(children, seed)
	require.ErrorContains(t, prove(children), "verifying the node proof", "the root should reject another seed")
}

// TestNew_BindsChildren checks that a child's public inputs enter the node
// only through the verifier: a public input changed after the mock verdict
// is laid out fails at verification.
func TestNew_BindsChildren(t *testing.T) {
	r := rand.New(rand.NewPCG(9, 10))
	l := testLayout(1, false)
	tr := newTree(t, l)
	cells, err := tr.leaf.Inputs(field.Octuplet{}, shards(r, l, 1)...)
	require.NoError(t, err)
	// The first child's cells: its active flag, then its public inputs.
	cells[1][0].Add(&cells[1][0], new(field.Element).SetOne())
	run, err := tr.leaf.Execute(cells)
	require.NoError(t, err)
	proof, pub := tr.leaf.Prove(run)
	require.Error(t, tr.leaf.Verify(proof, pub), "a public input changed after the verdict should fail")
}

// TestNew_InvalidArguments checks the rejections of malformed layouts and
// children.
func TestNew_InvalidArguments(t *testing.T) {
	r := rand.New(rand.NewPCG(5, 6))
	l := testLayout(1, false)
	tr := newTree(t, l)
	sys := stubSystem(l, shardKey)

	_, err := node.NewLeaf(nil, sys, verifier.NewMock, testOpts...)
	require.ErrorContains(t, err, "empty layout")
	_, err = node.NewLeaf(l, nil, verifier.NewMock, testOpts...)
	require.ErrorContains(t, err, "no shard system")
	_, err = node.NewInternal(l, tr.internal, verifier.NewMock, testOpts...)
	require.ErrorContains(t, err, "no leaf program")
	_, err = node.NewRoot(l, tr.leaf, tr.leaf, verifier.NewMock, testOpts...)
	require.ErrorContains(t, err, "no internal program")
	_, err = node.NewLeaf(l.WithKey(), sys, verifier.NewMock, testOpts...)
	require.ErrorContains(t, err, "has a key box")
	_, err = node.KeyOf(childPub(r, l)[:3])
	require.ErrorContains(t, err, "no key box")
	shortSeed := append(testLayout(1, false), node.Role{Tag: "Seed", Index: 0, Op: node.OpSeed})
	_, err = node.NewLeaf(shortSeed, sys, verifier.NewMock, testOpts...)
	require.ErrorContains(t, err, "1 seed limbs, want 8")
	noContribution := testLayout(1, false)
	for i := range node.NumKey {
		noContribution = append(noContribution, node.Role{Tag: "Seed", Index: i, Op: node.OpSeed})
	}
	_, err = node.NewLeaf(noContribution, sys, verifier.NewMock, testOpts...)
	require.ErrorContains(t, err, "no contribution")

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
		{name: "extension VK limb", program: tr.leaf, children: []node.Child{{Pub: extVK}}, wantErr: "public input 5 is not a base field element"},
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
