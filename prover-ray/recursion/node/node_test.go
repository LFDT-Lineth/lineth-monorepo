package node_test

import (
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/recursion"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/recursion/node"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/wioptest"
	"github.com/stretchr/testify/require"
)

var testOpts = wioptest.TestingCompileOptions()

// randExt returns a random E6 value.
func randExt(r *rand.Rand) field.Ext {
	var c recursion.Cell
	for i := range recursion.KindE6 {
		c[i].SetUint64(r.Uint64())
	}
	return recursion.CellE6(c)
}

// children returns n children of the layout sharing a VK and an output, with
// random bus products.
func children(r *rand.Rand, l node.Layout, n int) []node.Child {
	vk := make([]field.Element, l.NumVK)
	for i := range vk {
		vk[i].SetUint64(uint64(100 + i))
	}
	out := make([]field.Element, l.NumOut)
	for k := range out {
		out[k].SetUint64(uint64(200 + k))
	}
	cs := make([]node.Child, n)
	for c := range cs {
		cs[c] = node.Child{Bus: make([]field.Ext, l.NumBus), VK: vk, Out: out}
		for h := range cs[c].Bus {
			cs[c].Bus[h] = randExt(r)
		}
	}
	return cs
}

// requireExtEqual checks two E6 values are equal.
func requireExtEqual(t *testing.T, want, got field.Ext, msg string, args ...any) {
	t.Helper()
	require.True(t, want.Equal(&got), append([]any{msg}, args...)...)
}

// TestNew_Aggregates checks that the node program multiplies the children's
// bus products and propagates their VK and output, for several bus widths.
func TestNew_Aggregates(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	for _, numBus := range []int{1, 2, 5} {
		t.Run("", func(t *testing.T) {
			l := node.Layout{NumBus: numBus, NumVK: 8, NumOut: 32}
			p, err := node.New(l, 2, false, testOpts...)
			require.NoError(t, err)
			require.Equal(t, 6*numBus+40, p.NumPublicInputs(), "six lanes per bus product, 8 VK limbs, 32 output bytes")

			for range 2 {
				cs := children(r, l, 2)
				_, pub, err := p.ProveChildren(cs...)
				require.NoError(t, err)
				got, err := p.Decode(pub)
				require.NoError(t, err)
				for h := range numBus {
					var want field.Ext
					want.Mul(&cs[0].Bus[h], &cs[1].Bus[h])
					requireExtEqual(t, want, got.Bus[h], "bus product %d", h)
				}
				require.Equal(t, cs[0].VK, got.VK, "the VK should be propagated")
				require.Equal(t, cs[0].Out, got.Out, "the output should be propagated")
			}
		})
	}
}

// TestNew_Rejects checks that children disagreeing on the VK or the output,
// or an unbalanced root, fail at verification.
func TestNew_Rejects(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	l := node.Layout{NumBus: 2, NumVK: 8, NumOut: 32}
	nodeP, err := node.New(l, 2, false, testOpts...)
	require.NoError(t, err)
	rootP, err := node.New(l, 1, true, testOpts...)
	require.NoError(t, err)

	cases := []struct {
		name string
		run  func() error
	}{
		{name: "different VK limb", run: func() error {
			cs := children(r, l, 2)
			cs[1].VK = append([]field.Element{}, cs[1].VK...)
			cs[1].VK[3].SetUint64(9)
			_, _, err := nodeP.ProveChildren(cs...)
			return err
		}},
		{name: "different output byte", run: func() error {
			cs := children(r, l, 2)
			cs[1].Out = append([]field.Element{}, cs[1].Out...)
			cs[1].Out[31].SetUint64(9)
			_, _, err := nodeP.ProveChildren(cs...)
			return err
		}},
		{name: "unbalanced root", run: func() error {
			cs := children(r, l, 1)
			_, _, err := rootP.ProveChildren(cs...)
			return err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.ErrorContains(t, tc.run(), "verifying the node proof")
		})
	}
}

// TestNew_Root checks that a balanced child proves at the root and that its
// values round-trip through the public inputs.
func TestNew_Root(t *testing.T) {
	r := rand.New(rand.NewPCG(5, 6))
	l := node.Layout{NumBus: 3, NumVK: 8, NumOut: 32}
	p, err := node.New(l, 1, true, testOpts...)
	require.NoError(t, err)

	cs := children(r, l, 1)
	for h := range cs[0].Bus {
		cs[0].Bus[h].SetOne()
	}
	_, pub, err := p.ProveChildren(cs...)
	require.NoError(t, err)
	got, err := p.Decode(pub)
	require.NoError(t, err)
	require.Equal(t, cs[0], got, "the root should expose its child's values")
}

// TestNew_InvalidArguments checks the argument validation of New, Inputs and
// Decode.
func TestNew_InvalidArguments(t *testing.T) {
	l := node.Layout{NumBus: 1, NumVK: 2, NumOut: 2}
	p, err := node.New(l, 2, false, testOpts...)
	require.NoError(t, err)

	cases := []struct {
		name    string
		run     func() error
		wantErr string
	}{
		{name: "three children", run: func() error {
			_, err := node.New(l, 3, false, testOpts...)
			return err
		}, wantErr: "3 children"},
		{name: "no bus handle", run: func() error {
			_, err := node.New(node.Layout{NumBus: 0}, 2, false, testOpts...)
			return err
		}, wantErr: "invalid layout"},
		{name: "wrong child count", run: func() error {
			_, err := p.Inputs(node.Child{})
			return err
		}, wantErr: "1 children, the program takes 2"},
		{name: "wrong child width", run: func() error {
			c := node.Child{Bus: make([]field.Ext, 1), VK: make([]field.Element, 1), Out: make([]field.Element, 2)}
			_, err := p.Inputs(c, c)
			return err
		}, wantErr: "1 VK limbs"},
		{name: "wrong public input count", run: func() error {
			_, err := p.Decode(make([]field.Gen, 3))
			return err
		}, wantErr: "3 public inputs"},
		{name: "extension lane in a VK position", run: func() error {
			pub := make([]field.Gen, p.NumPublicInputs())
			pub[p.Positions().VK[0]] = field.ElemFromExt(field.Ext{})
			_, err := p.Decode(pub)
			return err
		}, wantErr: "not a base field element"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.ErrorContains(t, tc.run(), tc.wantErr)
		})
	}
}
