package recursion

import (
	"fmt"
	"math/big"
	"math/bits"
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/fri"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/stretchr/testify/require"
)

// merkleCase is a Merkle path of an uncapped ray tree: a leaf, its siblings
// (leaf level first), the digests injected at each level (inj[i] at the level
// of 2^i nodes, root level 0), the index and the root.
type merkleCase struct {
	leaf     Cell
	leafInj  []Cell // digests injected into the leaf (a grouped tree's other bottom groups)
	siblings []Cell
	inj      [][]Cell
	index    int
	root     Cell
}

// randomMerkleCase draws a path of the given depth, with up to maxInj digests
// injected at each internal level, and computes its root as ray's trees do.
// The root is checked with fri.Branch.AuthenticateToCap, so the cases follow
// ray's convention and not this test's reading of it.
func randomMerkleCase(t *testing.T, seed uint64, depth, maxInj int) merkleCase {
	t.Helper()
	rnd := rand.New(rand.NewPCG(seed, 7))
	rc := func() Cell {
		var c Cell
		for i := range c {
			c[i] = field.NewElement(rnd.Uint64())
		}
		return c
	}
	c := merkleCase{leaf: rc(), index: rnd.IntN(1 << depth), inj: make([][]Cell, depth)}
	for range depth {
		c.siblings = append(c.siblings, rc())
	}
	for lvl := range depth {
		for range rnd.IntN(maxInj + 1) {
			c.inj[lvl] = append(c.inj[lvl], rc())
		}
	}
	// the root, from the leaf: level k's sibling produces the node at the
	// level of 2^(depth−k−1) nodes, into which inj[depth−k−1] is injected
	cur, idx := c.leaf, c.index
	for k, s := range c.siblings {
		left, right := cur, s
		if idx&1 == 1 {
			left, right = s, cur
		}
		cur = compressCell(left, right)
		for _, d := range c.inj[depth-k-1] {
			cur = compressCell(cur, d)
		}
		idx >>= 1
	}
	c.root = cur

	branch := fri.Branch{Leaf: field.Octuplet(c.leaf), AuxSiblings: make([][]field.Octuplet, depth)}
	for k := depth - 1; k >= 0; k-- {
		branch.Siblings = append(branch.Siblings, field.Octuplet(c.siblings[k]))
	}
	for lvl, ds := range c.inj {
		for _, d := range ds {
			branch.AuxSiblings[lvl] = append(branch.AuxSiblings[lvl], field.Octuplet(d))
		}
	}
	require.NoError(t, branch.AuthenticateToCap(c.index, []field.Octuplet{field.Octuplet(c.root)}),
		"the test's root must be ray's")
	return c
}

// merkleProgram builds a MerklePath (or MerklePathX with gInv, if x) for a case's
// shape; its [x⁻¹, b0] cell is copied, so that it is read on the bus.
func merkleProgram(c merkleCase, x bool, gInv field.Element) (m *Machine, in []Cell, index, root, xb int) {
	m = &Machine{}
	cp := withCopy(m)
	depth := len(c.siblings)
	leaf := m.InputDigest()
	in = append(in, c.leaf)
	sibs := make([]int, depth)
	for k := range sibs {
		sibs[k] = m.InputDigest()
		in = append(in, c.siblings[k])
	}
	var injections []Injection
	for _, d := range c.leafInj {
		injections = append(injections, Injection{Width: 1 << depth, Digest: m.InputDigest()})
		in = append(in, d)
	}
	for lvl := depth - 1; lvl >= 0; lvl-- { // non-increasing widths
		for _, d := range c.inj[lvl] {
			injections = append(injections, Injection{Width: 1 << lvl, Digest: m.InputDigest()})
			in = append(in, d)
		}
	}
	index, root = m.InputScalar(), m.InputDigest()
	in = append(in, ScalarCell(field.NewElement(uint64(c.index))), c.root)
	xb = -1
	if x {
		xb = m.MerklePathX(leaf, sibs, index, root, gInv, injections...)
		cp.Copy(xb)
	} else {
		m.MerklePath(leaf, sibs, index, root, injections...)
	}
	return m, in, index, root, xb
}

// TestMerklePath: paths of several depths, with no injection, one or several
// per level, prove and verify; and a path of a fri.NewTree, opened by ray.
func TestMerklePath(t *testing.T) {
	for _, tc := range []struct{ depth, maxInj int }{{1, 0}, {1, 2}, {3, 1}, {5, 2}} {
		t.Run(fmt.Sprintf("depth=%d,inj=%d", tc.depth, tc.maxInj), func(t *testing.T) {
			c := randomMerkleCase(t, uint64(10*tc.depth+tc.maxInj), tc.depth, tc.maxInj)
			m, in, _, _, _ := merkleProgram(c, false, field.Element{})
			p, err := m.Compile(testPCSOpts...)
			require.NoError(t, err)
			r, err := p.Execute(in)
			require.NoError(t, err)
			require.NoError(t, proveAndVerify(p, r))
		})
	}

	t.Run("fri.NewTree", func(t *testing.T) {
		// 16 leaves, a table of 4 digests injected at the level of 4 nodes
		// and one of 2 at the level of 2
		rnd := rand.New(rand.NewPCG(3, 3))
		level := func(n int) []field.Octuplet {
			res := make([]field.Octuplet, n)
			for i := range res {
				for j := range res[i] {
					res[i][j] = field.NewElement(rnd.Uint64())
				}
			}
			return res
		}
		tree := fri.NewTree([][]field.Octuplet{nil, level(2), level(4), nil, level(16)})
		const idx = 11
		branch := tree.OpenBranch(idx)
		c := merkleCase{leaf: Cell(branch.Leaf), index: idx, root: Cell(tree.Root()), inj: make([][]Cell, len(branch.Siblings))}
		for k := len(branch.Siblings) - 1; k >= 0; k-- {
			c.siblings = append(c.siblings, Cell(branch.Siblings[k]))
		}
		for lvl, ds := range branch.AuxSiblings {
			for _, d := range ds {
				c.inj[lvl] = append(c.inj[lvl], Cell(d))
			}
		}
		require.NotEmpty(t, c.inj[1], "the tree injects at the level of 2 nodes")
		m, in, _, _, _ := merkleProgram(c, false, field.Element{})
		p, err := m.Compile(testPCSOpts...)
		require.NoError(t, err)
		r, err := p.Execute(in)
		require.NoError(t, err)
		require.NoError(t, proveAndVerify(p, r))
	})
}

// TestMerklePathX: [x⁻¹, b0] is gInv^bitrev(index) over the path's depth and
// the index's low bit, for every index of a depth-4 path with injections.
func TestMerklePathX(t *testing.T) {
	const depth = 4
	gInv := field.NewElement(7)
	c := randomMerkleCase(t, 4, depth, 1)
	m, _, _, _, xb := merkleProgram(c, true, gInv)
	p, err := m.Compile(testPCSOpts...)
	require.NoError(t, err)
	for idx := range 1 << depth {
		ci := randomMerkleCaseAt(t, c, idx)
		_, in, _, _, _ := merkleProgram(ci, true, gInv)
		r, err := p.Execute(in)
		require.NoError(t, err)
		rev := bits.Reverse64(uint64(idx)) >> (64 - depth)
		var want field.Element
		want.Exp(gInv, new(big.Int).SetUint64(rev))
		require.Equal(t, want, r.Value(xb)[0], "x⁻¹ at index %d", idx)
		require.Equal(t, field.NewElement(uint64(idx&1)), r.Value(xb)[1], "b0 at index %d", idx)
		if idx == 5 {
			require.NoError(t, proveAndVerify(p, r))
		}
	}
}

// randomMerkleCaseAt returns c moved to another index: same leaf, siblings and
// injections, and the root of that path.
func randomMerkleCaseAt(t *testing.T, c merkleCase, idx int) merkleCase {
	t.Helper()
	c.index = idx
	cur := c.leaf
	depth := len(c.siblings)
	for k, s := range c.siblings {
		left, right := cur, s
		if idx>>k&1 == 1 {
			left, right = s, cur
		}
		cur = compressCell(left, right)
		for _, d := range c.inj[depth-k-1] {
			cur = compressCell(cur, d)
		}
	}
	c.root = cur
	return c
}

// TestMerklePathExecuteRejects: execution fails on an index beyond the depth or
// a path that does not reach the root.
func TestMerklePathExecuteRejects(t *testing.T) {
	c := randomMerkleCase(t, 5, 3, 1)
	m, in, _, _, _ := merkleProgram(c, false, field.Element{})
	p, err := m.Compile(testPCSOpts...)
	require.NoError(t, err)

	big := append([]Cell(nil), in...)
	big[len(big)-2] = ScalarCell(field.NewElement(uint64(c.index + 8)))
	_, err = p.Execute(big)
	require.ErrorContains(t, err, "does not fit")

	wrong := append([]Cell(nil), in...)
	wrong[len(wrong)-1][0].SetUint64(1)
	_, err = p.Execute(wrong)
	require.ErrorContains(t, err, "does not reach the root")
}

// TestMerklePathRejects: cheats on a depth-3 MerklePathX with injections,
// consistent on the memory bus (values changed in the run, then traced), are
// rejected: a wrong root, a flipped direction bit, a wrong sibling (its
// compressions not in the P2 core), an index beyond the depth with the same
// low bits, a wrong x⁻¹ and a wrong b0.
func TestMerklePathRejects(t *testing.T) {
	c := randomMerkleCase(t, 6, 3, 1)
	gInv := field.NewElement(5)
	for name, cheat := range map[string]func(p *Program, r *Run, index, root, xb int){
		"root": func(p *Program, r *Run, _, root, _ int) {
			r.values[root][0].SetUint64(1)
			require.NoError(t, p.trace(r))
		},
		"direction bit": func(p *Program, r *Run, _, _, _ int) {
			b := r.Witness(p, merkleMod, merkleB)
			var one field.Element
			one.SetOne()
			b[0].Sub(&one, &b[0])
		},
		"sibling": func(p *Program, r *Run, _, _, _ int) {
			r.values[c.siblingAddr(p)][3].SetUint64(9)
			require.NoError(t, p.trace(r))
		},
		"index beyond the depth": func(p *Program, r *Run, index, _, _ int) {
			r.values[index][0].SetUint64(uint64(c.index + 8))
			require.NoError(t, p.trace(r))
		},
		"x": func(p *Program, r *Run, _, _, xb int) {
			r.values[xb][0].SetUint64(3)
			require.NoError(t, p.trace(r))
			x := r.Witness(p, merkleMod, merkleX)
			x[c.lastRow()].SetUint64(3)
		},
		"b0": func(p *Program, r *Run, _, _, xb int) {
			var one field.Element
			one.SetOne()
			r.values[xb][1].Sub(&one, &r.values[xb][1])
			require.NoError(t, p.trace(r))
			b0 := r.Witness(p, merkleMod, merkleB0)
			for row := range c.lastRow() + 1 {
				b0[row] = r.values[xb][1]
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			m, in, index, root, xb := merkleProgram(c, true, gInv)
			p, err := m.Compile(testPCSOpts...)
			require.NoError(t, err)
			r, err := p.Execute(in)
			require.NoError(t, err)
			require.NoError(t, proveAndVerify(p, r), "honest")
			cheat(p, r, index, root, xb)
			require.Error(t, proveAndVerify(p, r), "cheat accepted")
		})
	}
}

// siblingAddr is the address of the case's first sibling in merkleProgram:
// the leaf is input 0, the siblings follow.
func (c merkleCase) siblingAddr(*Program) int { return 1 }

// lastRow is the merkle chip's last row of the case's path.
func (c merkleCase) lastRow() int {
	n := len(c.siblings) + len(c.leafInj)
	for _, ds := range c.inj {
		n += len(ds)
	}
	return n - 1
}

// TestMerklePathRejectsFreeBits covers the constraints that no honest-looking
// path exercises: where the two operands of a compression are equal, the
// direction bit does not change the hash, and only the bit's own constraints
// keep the index bound to the path.
//   - non-boolean bit: a depth-1 path whose sibling equals its leaf, claiming
//     index 2 with b = 2 (the same compression);
//   - injection bit: a depth-1 path whose injected digest equals the running
//     hash, claiming index 2 with b = 1 on the injection row (an injection
//     consuming a bit, same compression).
func TestMerklePathRejectsFreeBits(t *testing.T) {
	leaf := cell(3)
	node := compressCell(leaf, cell(4))
	for name, tc := range map[string]struct {
		c     merkleCase
		cheat func(p *Program, r *Run)
	}{
		"non-boolean bit": {
			c: merkleCase{leaf: leaf, siblings: []Cell{leaf}, inj: make([][]Cell, 1), root: compressCell(leaf, leaf)},
			cheat: func(p *Program, r *Run) {
				r.Witness(p, merkleMod, merkleB)[0].SetUint64(2)
				r.Witness(p, merkleMod, merkleB0)[0].SetUint64(2)
			},
		},
		"injection bit": {
			c: merkleCase{leaf: leaf, siblings: []Cell{cell(4)}, inj: [][]Cell{{node}}, root: compressCell(node, node)},
			cheat: func(p *Program, r *Run) {
				r.Witness(p, merkleMod, merkleB)[1].SetOne()
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			m, in, index, _, _ := merkleProgram(tc.c, false, field.Element{})
			p, err := m.Compile(testPCSOpts...)
			require.NoError(t, err)
			r, err := p.Execute(in)
			require.NoError(t, err)
			require.NoError(t, proveAndVerify(p, r), "honest")
			r.values[index][0].SetUint64(2)
			require.NoError(t, p.trace(r))
			tc.cheat(p, r)
			require.Error(t, proveAndVerify(p, r), "index 2 accepted for a depth-1 path")
		})
	}
}

// TestMerklePathRejectsOutputs: [x⁻¹, b0] changed consistently along the path,
// so that only the first row's constraints see it: x⁻¹ scaled on every row,
// x⁻¹ and the first row's factor f scaled together, and b0 flipped on every
// row after the first level row.
func TestMerklePathRejectsOutputs(t *testing.T) {
	c := randomMerkleCase(t, 8, 3, 0)
	gInv := field.NewElement(5)
	for name, cheat := range map[string]func(p *Program, r *Run, xb int){
		"x on every row, f on the first": func(p *Program, r *Run, xb int) {
			two := field.NewElement(2)
			r.values[xb][0].Mul(&r.values[xb][0], &two)
			require.NoError(t, p.trace(r))
			f := r.Witness(p, merkleMod, merkleF)
			f[0].Mul(&f[0], &two)
			x := r.Witness(p, merkleMod, merkleX)
			for row := range c.lastRow() + 1 {
				x[row].Mul(&x[row], &two)
			}
		},
		"x on every row": func(p *Program, r *Run, xb int) {
			two := field.NewElement(2)
			r.values[xb][0].Mul(&r.values[xb][0], &two)
			require.NoError(t, p.trace(r))
			x := r.Witness(p, merkleMod, merkleX)
			for row := range c.lastRow() + 1 {
				x[row].Mul(&x[row], &two)
			}
		},
		"b0 after the first level": func(p *Program, r *Run, xb int) {
			var one field.Element
			one.SetOne()
			r.values[xb][1].Sub(&one, &r.values[xb][1])
			require.NoError(t, p.trace(r))
			b0 := r.Witness(p, merkleMod, merkleB0)
			for row := 1; row <= c.lastRow(); row++ {
				b0[row] = r.values[xb][1]
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			m, in, _, _, xb := merkleProgram(c, true, gInv)
			p, err := m.Compile(testPCSOpts...)
			require.NoError(t, err)
			r, err := p.Execute(in)
			require.NoError(t, err)
			require.NoError(t, proveAndVerify(p, r), "honest")
			cheat(p, r, xb)
			require.Error(t, proveAndVerify(p, r), "cheat accepted")
		})
	}
}

// groupedTree is a grouped (size-oblivious) ray tree, as pcs.WithModuleGroups
// commits one: 16 rows at the bottom in 2 groups, 8 rows in 2 groups, none of 4
// and 2 rows in 2 groups. The bottom leaf j is Compress(h₁(j), h₂(j)), h_k(j)
// being group k's digest of rows 2j and 2j+1, and the smaller sizes are
// injected group by group.
func groupedTree() fri.MultiSizeTable {
	rnd := rand.New(rand.NewPCG(11, 11))
	col := func(n int) []field.Element {
		res := make([]field.Element, n)
		for i := range res {
			res[i] = field.NewElement(rnd.Uint64())
		}
		return res
	}
	sized := func(n int) fri.SizedTable {
		return fri.SizedTable{
			Base:   [][]field.Element{col(n), col(n), col(n)},
			Groups: []fri.GroupShape{{BaseWidth: 2, Ordinal: 0}, {BaseWidth: 1, Ordinal: 1}},
		}
	}
	return fri.MultiSizeTable{{}, sized(2), {}, sized(8), sized(16)}
}

// groupDigests returns the digests of a size's groups at pair j: the leaf j of
// a commitment of that group alone, which is the group's digest unchained.
func groupDigests(table fri.MultiSizeTable, size, j int) []Cell {
	sized := table[size]
	res := make([]Cell, 0, len(sized.Groups))
	base := 0
	for _, g := range sized.Groups {
		alone := make(fri.MultiSizeTable, size+1)
		alone[size] = fri.SizedTable{Base: sized.Base[base : base+g.BaseWidth], Groups: []fri.GroupShape{{BaseWidth: g.BaseWidth}}}
		res = append(res, Cell(alone.Merkleize().OpenBranch(j).Leaf))
		base += g.BaseWidth
	}
	return res
}

// groupedCase is the path of leaf j of the grouped tree: leaf h₁(j), h₂(j)
// injected into the leaf, then the branch's siblings and injections.
func groupedCase(t *testing.T, table fri.MultiSizeTable, tree *fri.Tree, j int) merkleCase {
	t.Helper()
	bottom := groupDigests(table, len(table)-1, j)
	branch := tree.OpenBranch(j)
	require.Equal(t, Cell(branch.Leaf), compressCell(bottom[0], bottom[1]), "the leaf chains the bottom groups")
	depth := len(branch.Siblings)
	c := merkleCase{leaf: bottom[0], leafInj: bottom[1:], index: j, root: Cell(tree.Root()), inj: make([][]Cell, depth)}
	for k := depth - 1; k >= 0; k-- {
		c.siblings = append(c.siblings, Cell(branch.Siblings[k]))
	}
	for lvl, ds := range branch.AuxSiblings {
		for _, d := range ds {
			c.inj[lvl] = append(c.inj[lvl], Cell(d))
		}
	}
	return c
}

// TestMerklePathGrouped: every path of a grouped tree, with two digests
// injected into the leaf, two at an inner level and two at the root, proves;
// and swapping two digests injected at one level is rejected, on execution and,
// claimed consistently, by the proof.
func TestMerklePathGrouped(t *testing.T) {
	table := groupedTree()
	tree := table.Merkleize()
	for j := range tree.NumLeaves() {
		c := groupedCase(t, table, tree, j)
		require.Len(t, c.inj[2], 2, "two groups injected at the level of 4 nodes")
		require.Len(t, c.inj[0], 2, "two groups injected at the root")
		m, in, _, _, _ := merkleProgram(c, false, field.Element{})
		p, err := m.Compile(testPCSOpts...)
		require.NoError(t, err)
		r, err := p.Execute(in)
		require.NoError(t, err, "leaf %d", j)
		require.NoError(t, proveAndVerify(p, r), "leaf %d", j)

		if j != 5 {
			continue
		}
		swapped := c
		swapped.inj = append([][]Cell(nil), c.inj...)
		swapped.inj[2] = []Cell{c.inj[2][1], c.inj[2][0]}
		_, sin, _, _, _ := merkleProgram(swapped, false, field.Element{})
		_, err = p.Execute(sin)
		require.ErrorContains(t, err, "does not reach the root", "swapped injections executed")

		// the swap claimed consistently: the two injected cells exchange
		// their values, and the trace follows
		first := 1 + len(c.leafInj) + len(c.siblings) // the first injected digest at the level of 4 nodes
		r.values[first], r.values[first+1] = r.values[first+1], r.values[first]
		require.NoError(t, p.trace(r))
		require.Error(t, proveAndVerify(p, r), "swapped injections accepted")
	}
}

// TestMerklePathChaining: leaves 4 and 5 of a tree share every ancestor. Path
// 5 opens a forged leaf: its leaf-level row compresses the forged leaf (that
// compression is put in the P2 core), and its rows above are path 4's, which
// reach the root. Only cur = out[−1] rejects it.
func TestMerklePathChaining(t *testing.T) {
	const depth = 3
	rnd := rand.New(rand.NewPCG(12, 12))
	leaves := make([]field.Octuplet, 1<<depth)
	for i := range leaves {
		for k := range leaves[i] {
			leaves[i][k] = field.NewElement(rnd.Uint64())
		}
	}
	tree := fri.NewTree([][]field.Octuplet{nil, nil, nil, leaves})
	m := &Machine{}
	var in []Cell
	input := func(c Cell) int {
		in = append(in, c)
		return m.InputDigest()
	}
	var leafAddr [2]int
	for k, j := range []int{4, 5} {
		branch := tree.OpenBranch(j)
		leafAddr[k] = input(Cell(branch.Leaf))
		sibs := make([]int, depth)
		for lvl := range depth {
			sibs[lvl] = input(Cell(branch.Siblings[depth-1-lvl]))
		}
		in = append(in, ScalarCell(field.NewElement(uint64(j))))
		index := m.InputScalar()
		root := input(Cell(tree.Root()))
		m.MerklePath(leafAddr[k], sibs, index, root)
	}
	p, err := m.Compile(testPCSOpts...)
	require.NoError(t, err)
	r, err := p.Execute(in)
	require.NoError(t, err)
	require.NoError(t, proveAndVerify(p, r), "honest")

	honest := map[string][]field.Element{}
	for _, name := range []string{merkleCur, merkleS, merkleOut} {
		for i := range CellWidth {
			lane := laneName(name, i)
			honest[lane] = append([]field.Element(nil), r.Witness(p, merkleMod, lane)...)
		}
	}
	forged := r.values[leafAddr[1]]
	forged[0].SetUint64(9)
	r.values[leafAddr[1]] = forged
	sibling := r.values[leafAddr[0]] // leaf 4, leaf 5's sibling
	r.p2[depth] = p2Input(sibling, forged)
	require.NoError(t, p.trace(r))
	// path 5's rows above the leaf level: path 4's
	for lane, vals := range honest {
		w := r.Witness(p, merkleMod, lane)
		for k := 1; k < depth; k++ {
			w[depth+k] = vals[k]
		}
	}
	require.Error(t, proveAndVerify(p, r), "a forged leaf accepted")
}

// merkleWitness copies the merkle chip's witness columns.
func merkleWitness(p *Program, r *Run) map[string][]field.Element {
	w := map[string][]field.Element{}
	save := func(name string) {
		w[name] = append([]field.Element(nil), r.Witness(p, merkleMod, name)...)
	}
	for _, name := range []string{merkleB, merkleIdx, merkleF, merkleX, merkleB0} {
		save(name)
	}
	for _, x := range []string{merkleCur, merkleS, merkleOut} {
		for i := range CellWidth {
			save(laneName(x, i))
		}
	}
	return w
}

// TestMerklePathRejectsUnbound: what only one read or lookup of the chip sees.
//   - a leaf, index or sibling cell that differs from the path the chip proves:
//     the cell changes in the run and the other chips follow, but the merkle
//     chip keeps the honest path, so only its read of the cell rejects it;
//   - a forged node: row 0's output and row 1's input are replaced by the same
//     digest, so the chain holds and only the P2 lookup rejects it.
func TestMerklePathRejectsUnbound(t *testing.T) {
	c := randomMerkleCase(t, 6, 3, 1)
	changeCell := func(p *Program, r *Run, addr int, change func(v *Cell)) {
		honest := merkleWitness(p, r)
		change(&r.values[addr])
		require.NoError(t, p.trace(r))
		for name, w := range honest {
			copy(r.Witness(p, merkleMod, name), w)
		}
	}
	for name, cheat := range map[string]func(p *Program, r *Run, index int){
		"leaf cell": func(p *Program, r *Run, _ int) { // the leaf is input 0
			changeCell(p, r, 0, func(v *Cell) { v[0].SetUint64(77) })
		},
		"index cell": func(p *Program, r *Run, index int) {
			changeCell(p, r, index, func(v *Cell) { v[0].SetUint64(uint64(c.index ^ 1)) })
		},
		"sibling cell": func(p *Program, r *Run, _ int) { // the first sibling is input 1
			changeCell(p, r, 1, func(v *Cell) { v[3].SetUint64(9) })
		},
		"forged node": func(p *Program, r *Run, _ int) {
			for i := range CellWidth {
				out := r.Witness(p, merkleMod, laneName(merkleOut, i))
				out[0].SetUint64(1000 + uint64(i))
				r.Witness(p, merkleMod, laneName(merkleCur, i))[1] = out[0]
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			m, in, index, _, _ := merkleProgram(c, false, field.Element{})
			p, err := m.Compile(testPCSOpts...)
			require.NoError(t, err)
			r, err := p.Execute(in)
			require.NoError(t, err)
			require.NoError(t, proveAndVerify(p, r), "honest")
			cheat(p, r, index)
			require.Error(t, proveAndVerify(p, r), "cheat accepted")
		})
	}
}

// TestMerklePathDepthBounds: a path has 1 to field.Bits−1 levels, since
// idx = Σ bᵢ·2ⁱ is injective only below the field's modulus.
func TestMerklePathDepthBounds(t *testing.T) {
	for _, tc := range []struct {
		depth int
		ok    bool
	}{{0, false}, {1, true}, {field.Bits - 1, true}, {field.Bits, false}} {
		t.Run(fmt.Sprint(tc.depth), func(t *testing.T) {
			m := &Machine{}
			leaf, index, root := m.InputDigest(), m.InputScalar(), m.InputDigest()
			siblings := make([]int, tc.depth)
			for i := range siblings {
				siblings[i] = m.InputDigest()
			}
			build := func() { m.MerklePath(leaf, siblings, index, root) }
			if tc.ok {
				require.NotPanics(t, build)
			} else {
				require.Panics(t, build)
			}
		})
	}
}
