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

// randExt returns a random E6 value.
func randExt(r *rand.Rand) field.Ext {
	var c recursion.Cell
	for i := range recursion.KindE6 {
		c[i].SetUint64(r.Uint64())
	}
	return recursion.CellE6(c)
}

// childPub returns the public inputs of a child of the layout: random bus
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

// want returns the public inputs a node of the children should expose.
func want(l node.Layout, children []wiop.PublicInput) wiop.PublicInput {
	pub := append(wiop.PublicInput{}, children[0]...)
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

// TestNew_Aggregates checks that a node proof exposes the layout, with the
// products multiplied, the contributions added and the shared values
// propagated, for 1 to Arity children.
func TestNew_Aggregates(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	l := testLayout(2, true)
	p, err := node.New(l, false, testOpts...)
	require.NoError(t, err)
	for pos, role := range l {
		_, got := p.Sys.LookupPublicInputByTag(role.Tag, role.Index)
		require.Equal(t, pos, got, "%s_%d should be at its position in the layout", role.Tag, role.Index)
	}

	for _, n := range []int{1, 2, node.Arity} {
		children := make([]wiop.PublicInput, n)
		for c := range children {
			children[c] = childPub(r, l)
		}
		_, pub, err := p.ProveChildren(children...)
		require.NoError(t, err, "%d children", n)
		requirePubEqual(t, want(l, children), pub, "the node should combine its children")
	}
}

// TestNew_Rejects checks that children disagreeing on a propagated value, or
// an unbalanced root, fail at verification.
func TestNew_Rejects(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	l := testLayout(1, false)
	nodeP, err := node.New(l, false, testOpts...)
	require.NoError(t, err)
	rootP, err := node.New(l, true, testOpts...)
	require.NoError(t, err)

	cases := []struct {
		name string
		run  func() error
	}{
		{name: "different VK limb", run: func() error {
			a, b := childPub(r, l), childPub(r, l)
			b[5] = b[5].Add(field.ElemOne())
			_, _, err := nodeP.ProveChildren(a, b)
			return err
		}},
		{name: "different output byte", run: func() error {
			a, b := childPub(r, l), childPub(r, l)
			b[0] = b[0].Add(field.ElemOne())
			_, _, err := nodeP.ProveChildren(a, b)
			return err
		}},
		{name: "unbalanced root", run: func() error {
			_, _, err := rootP.ProveChildren(childPub(r, l))
			return err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.ErrorContains(t, tc.run(), "verifying the node proof")
		})
	}

	t.Run("balanced root", func(t *testing.T) {
		a, b := childPub(r, l), childPub(r, l)
		bus := a[6].AsExt()
		var inv field.Ext
		inv.Inverse(&bus)
		b[6] = field.ElemFromExt(inv)
		_, pub, err := rootP.ProveChildren(a, b)
		require.NoError(t, err)
		product := pub[6].AsExt()
		require.True(t, product.IsOne(), "the root should expose a balanced bus")
	})
}

// TestNew_InvalidArguments checks the rejections of malformed layouts and
// children.
func TestNew_InvalidArguments(t *testing.T) {
	r := rand.New(rand.NewPCG(5, 6))
	l := testLayout(1, false)
	p, err := node.New(l, false, testOpts...)
	require.NoError(t, err)

	_, err = node.New(nil, false, testOpts...)
	require.ErrorContains(t, err, "empty layout")

	tooMany := make([]wiop.PublicInput, node.Arity+1)
	for c := range tooMany {
		tooMany[c] = childPub(r, l)
	}
	short := childPub(r, l)[:3]
	extVK := childPub(r, l)
	extVK[5] = field.ElemFromExt(extVK[5].AsExt())
	cases := []struct {
		name     string
		children []wiop.PublicInput
		wantErr  string
	}{
		{name: "no child", children: nil, wantErr: "0 children, want 1 to 6"},
		{name: "too many children", children: tooMany, wantErr: "7 children, want 1 to 6"},
		{name: "wrong length", children: []wiop.PublicInput{short}, wantErr: "3 public inputs, want 7"},
		{name: "extension VK limb", children: []wiop.PublicInput{extVK}, wantErr: "VK_1 is not a base field element"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := p.Inputs(tc.children...)
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

	_, err = node.LayoutOf(sys, map[wiop.PublicInputTag]node.Op{"A": node.OpMul})
	require.ErrorContains(t, err, "public input 0 (B_0) has no role")
}
