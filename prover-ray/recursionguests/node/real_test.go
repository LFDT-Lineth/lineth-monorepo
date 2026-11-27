package node_test

import (
	"fmt"
	"maps"
	"math/rand/v2"
	"os"
	"slices"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/poseidon2"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/recursion/verifier"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/recursionguests/node"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
	"github.com/stretchr/testify/require"
)

// PROTOTYPE (#4405): the node programs on the real in-machine verifier.

// shardSystem stands for the shard system: a compiled system with a dynamic
// module, whose size every proof chooses, and the public inputs of a layout,
// which it leaves free. It is compiled size-oblivious, as the shard system
// will be.
type shardSystem struct {
	sys  *wiop.System
	a, b *wiop.Column
	pubs []*wiop.Cell
}

func newShardSystem(t *testing.T, l node.Layout, queries int) shardSystem {
	t.Helper()
	sys := wiop.NewSystemf("shard")
	r0 := sys.NewRound()
	ctx := sys.Context
	trace := sys.NewDynamicModule(ctx.Childf("trace"), wiop.PaddingDirectionRight)
	s := shardSystem{sys: sys}
	s.a, s.b = trace.NewColumn(ctx.Childf("a"), r0), trace.NewColumn(ctx.Childf("b"), r0)
	trace.NewVanishing(ctx.Childf("square"), wiop.Sub(s.b.View(), wiop.Mul(s.a.View(), s.a.View())))
	for pos, role := range l {
		cell := r0.NewCell(ctx.Childf("pub-%d", pos), role.Ext)
		sys.RegisterPublicInputs(role.Tag, cell, role.Index)
		s.pubs = append(s.pubs, cell)
	}
	require.NoError(t, compilers.CompileFull(sys,
		compilers.ConfigurePCS(pcs.WithFRINumQueries(queries), pcs.WithSizeOblivious())), "the shard system should compile")
	return s
}

// prove proves a shard of n rows with the public inputs pub.
func (s shardSystem) prove(t *testing.T, n int, pub wiop.PublicInput) node.Child {
	t.Helper()
	c := s.proveWith(t, n, pub, nil)
	require.NoError(t, s.sys.Verify(c.Proof, c.Pub), "the shard proof should verify natively")
	return c
}

// proveWith is prove with the column b changed by tamper: a proof of a false
// statement, if it breaks b = a².
func (s shardSystem) proveWith(t *testing.T, n int, pub wiop.PublicInput, tamper func(b []field.Element)) node.Child {
	t.Helper()
	a, b := make([]field.Element, n), make([]field.Element, n)
	for i := range n {
		a[i].SetUint64(uint64(i + 3))
		b[i].Mul(&a[i], &a[i])
	}
	if tamper != nil {
		tamper(b)
	}
	proof, got := s.sys.Prove(func(rt *wiop.Runtime) {
		rt.AssignColumn(s.a, &wiop.ConcreteVector{Plain: field.VecFromBase(a)})
		rt.AssignColumn(s.b, &wiop.ConcreteVector{Plain: field.VecFromBase(b)})
		for pos, cell := range s.pubs {
			rt.AssignCell(cell, pub[pos])
		}
	})
	return node.Child{Proof: proof, Pub: got}
}

// realTree compiles the three programs over the shard system with the real
// verifier.
func realTree(t *testing.T, l node.Layout, s shardSystem, opts ...compilers.Option) tree {
	t.Helper()
	leaf, err := node.NewLeaf(l, s.sys, verifier.New, opts...)
	require.NoError(t, err, "the leaf program")
	internal, err := node.NewInternal(l, leaf, verifier.New, opts...)
	require.NoError(t, err, "the internal program")
	root, err := node.NewRoot(l, leaf, internal, verifier.New, opts...)
	require.NoError(t, err, "the root program")
	return tree{leaf, internal, root}
}

// balancedShards returns the public inputs of n shards whose bus products
// multiply to one and whose seed is the digest of their contributions added
// up: what the root accepts.
func balancedShards(r *rand.Rand, l node.Layout, n int) []wiop.PublicInput {
	res := make([]wiop.PublicInput, n)
	for i := range res {
		res[i] = childPub(r, l)
	}
	sum := want(l, field.Octuplet{}, res...)
	h := poseidon2.NewMDHasher()
	for pos, role := range l {
		switch role.Op {
		case node.OpAdd:
			h.WriteElements(sum[pos].AsBase())
		case node.OpMul:
			// the last shard's product cancels the others'
			others := sum[pos].AsExt()
			last := res[n-1][pos].AsExt()
			var inv field.Ext
			inv.Inverse(&others)
			inv.Mul(&inv, &last)
			res[n-1][pos] = field.ElemFromExt(inv)
		}
	}
	seed := h.SumDigest()
	for _, pub := range res {
		for pos, role := range l {
			if role.Op == node.OpSeed {
				pub[pos] = field.ElemFromBase(seed[role.Index])
			}
		}
	}
	return res
}

// TestReal_Tree runs a conglomeration tree on the real in-machine verifier:
// shard proofs of different sizes, leaf nodes, an internal node over leaf
// nodes, an internal node over an internal node and a leaf node, and the
// root. No proof is verified in Go by the host but the root's.
func TestReal_Tree(t *testing.T) {
	r := rand.New(rand.NewPCG(21, 22))
	l := testLayout(1, true)
	s := newShardSystem(t, l, 2)
	tr := realTree(t, l, s, testOpts...)
	t.Logf("leaf heights     %v", node.LogHeights(tr.leaf.Sys))
	t.Logf("internal heights %v", node.LogHeights(tr.internal.Sys))
	t.Logf("root heights     %v", node.LogHeights(tr.root.Sys))
	require.NotEqual(t, node.LogHeights(tr.leaf.Sys), node.LogHeights(tr.internal.Sys),
		"the leaf and internal programs should differ in a height, or the test checks less")

	pubs := balancedShards(r, l, 4)
	sizes := []int{4, 8, 16, 4}
	shards := make([]node.Child, len(pubs))
	for i := range shards {
		shards[i] = s.prove(t, sizes[i], pubs[i])
	}

	leafOf := func(children ...node.Child) node.Child {
		proof, pub, err := tr.leaf.ProveChildren(field.Octuplet{}, children...)
		require.NoError(t, err, "a leaf node")
		return node.Child{Proof: proof, Pub: pub, Leaf: true}
	}
	key, box := tr.internal.Key(), tr.internal.Box()
	leafA, leafB, leafC := leafOf(shards[0], shards[1]), leafOf(shards[2]), leafOf(shards[3])
	requirePubEqual(t, want(l, field.Octuplet{}, pubs[0], pubs[1]), leafA.Pub, "the leaf should combine its shards")

	proof, pub, err := tr.internal.ProveChildren(key, leafA, leafB)
	require.NoError(t, err, "an internal node over leaf nodes")
	requirePubEqual(t, want(l, box, pubs[0], pubs[1], pubs[2]), pub, "the internal node should combine the leaves")
	internal1 := node.Child{Proof: proof, Pub: pub}

	proof, pub, err = tr.internal.ProveChildren(key, internal1, leafC)
	require.NoError(t, err, "an internal node over an internal node and a leaf node")
	requirePubEqual(t, want(l, box, pubs...), pub, "the internal node should combine both kinds")
	internal2 := node.Child{Proof: proof, Pub: pub}

	proof, pub, err = tr.root.ProveChildren(key, internal2)
	require.NoError(t, err, "the root over an internal node")
	require.NoError(t, tr.root.Verify(proof, pub), "the root proof should verify")
	requirePubEqual(t, want(l, box, pubs...), pub, "the root should expose the tree's values")

	t.Run("rejects an invalid child", func(t *testing.T) {
		// A shard proof of a false statement (b is not a²): the host lays it
		// out, and the leaf program rejects it.
		bad := s.proveWith(t, 8, pubs[0], func(b []field.Element) { b[3].SetUint64(1) })
		require.Error(t, s.sys.Verify(bad.Proof, bad.Pub), "the bad shard proof should not verify natively")
		_, _, err := tr.leaf.ProveChildren(field.Octuplet{}, bad)
		require.ErrorContains(t, err, "verifying the node proof", "the leaf should reject an invalid shard proof")
		// A leaf node proof with one claimed cell changed.
		forged := leafB
		forged.Proof.Cells = maps.Clone(leafB.Proof.Cells)
		for id, x := range forged.Proof.Cells {
			if x.IsBase() {
				forged.Proof.Cells[id] = x.Add(field.ElemOne())
				break
			}
		}
		_, _, err = tr.internal.ProveChildren(key, leafA, forged)
		require.Error(t, err, "the internal node should reject a forged leaf node proof")
	})

	t.Run("rejects", func(t *testing.T) {
		// An internal node claimed to be a leaf node: verified under the
		// leaf program's key and heights.
		asLeaf := internal1
		asLeaf.Leaf = true
		_, _, err := tr.internal.ProveChildren(key, asLeaf, leafC)
		require.Error(t, err, "an internal node flagged as a leaf should be rejected")
		// A leaf node claimed to be an internal node.
		asNode := leafC
		asNode.Leaf = false
		_, _, err = tr.internal.ProveChildren(key, internal1, asNode)
		require.Error(t, err, "a leaf node flagged as a node should be rejected")
		// A foreign key for the node children.
		foreign := key
		foreign[0].Add(&foreign[0], new(field.Element).SetOne())
		_, _, err = tr.internal.ProveChildren(foreign, internal1, leafC)
		require.Error(t, err, "a node child under a foreign key should be rejected")
		// A height of the internal program changed in its inputs: its node
		// child is then verified under another height.
		cells, err := tr.internal.Inputs(key, internal1, leafC)
		require.NoError(t, err)
		cells[1][0].Add(&cells[1][0], new(field.Element).SetOne())
		_, err = tr.internal.Execute(cells)
		if err == nil {
			run, _ := tr.internal.Execute(cells)
			nodeProof, nodePub := tr.internal.Prove(run)
			err = tr.internal.Verify(nodeProof, nodePub)
		}
		require.Error(t, err, "a node child under another height should be rejected")
		// A height changed over leaf children only: the node proves, with
		// another box, which the root rejects.
		cells, err = tr.internal.Inputs(key, leafA, leafB, leafC)
		require.NoError(t, err)
		cells[1][0].Add(&cells[1][0], new(field.Element).SetOne())
		run, err := tr.internal.Execute(cells)
		require.NoError(t, err, "the leaf children are verified under the leaf program's heights")
		nodeProof, nodePub := tr.internal.Prove(run)
		require.NoError(t, tr.internal.Verify(nodeProof, nodePub), "the node proves, under another box")
		got, err := node.KeyOf(nodePub)
		require.NoError(t, err)
		require.NotEqual(t, box, got, "the box should name the heights")
		_, _, err = tr.root.ProveChildren(key, node.Child{Proof: nodeProof, Pub: nodePub})
		require.Error(t, err, "the root should reject a node of another box")
	})
}

// TestReal_Measure builds, without proving, the three programs at the
// production query count, and prints the rows of their chips against the
// machine's cap of 2^22 rows. It is slow: set MEASURE=1 to run it.
func TestReal_Measure(t *testing.T) {
	if os.Getenv("MEASURE") == "" {
		t.Skip("set MEASURE=1 to build the programs at 229 queries")
	}
	l := testLayout(1, true)
	s := newShardSystem(t, l, 229)
	tr := realTree(t, l, s) // the default query count, 229
	for _, p := range []struct {
		name string
		prog *node.Program
	}{{"leaf (stand-in shard)", tr.leaf}, {"internal", tr.internal}, {"root", tr.root}} {
		rows := p.prog.ChipRows()
		names := slices.Sorted(maps.Keys(rows))
		slices.SortFunc(names, func(a, b string) int { return rows[b] - rows[a] })
		line := ""
		for _, name := range names {
			line += fmt.Sprintf(" %s=%d(%.0f%%)", name, rows[name], 100*float64(rows[name])/float64(1<<22))
		}
		fmt.Printf("%s:%s\n", p.name, line)
	}
}
