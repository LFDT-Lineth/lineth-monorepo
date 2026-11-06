package recursion

import (
	"fmt"
	"math/rand/v2"
	"sort"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/fri"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/stretchr/testify/require"
)

// The dynamic path circuit of these tests: G groups, heights up to H.
const (
	dpG = 4
	dpH = 8
)

// dpathValues are the input values of a dynamic path: the groups' leaves and
// heights, the order, the siblings by height, the index and the root.
type dpathValues struct {
	leaves  []Cell
	heights []int
	order   []int
	sibs    []Cell
	index   uint64
	root    Cell
}

// canonicalOrder returns the groups by decreasing height, equal heights by
// increasing index.
func canonicalOrder(heights []int) []int {
	order := make([]int, len(heights))
	for g := range order {
		order[g] = g
	}
	sort.SliceStable(order, func(a, b int) bool { return heights[order[a]] > heights[order[b]] })
	return order
}

// dpRand is a source of random cells.
type dpRand struct{ *rand.Rand }

func (r dpRand) cell() Cell {
	var c Cell
	for i := range c {
		c[i] = field.NewElement(r.Uint64())
	}
	return c
}

// dpathTree builds one of ray's mixed-height trees with the groups at the
// given heights (in [0, H]), as fri.Tree hashes it: a leaf chains the digests
// of the groups of the top height, in order, and a node at height h chains,
// after compressing its children, the digests of the groups of height h (the
// root, h = 0, included). It opens it at a random leaf, checks the opening
// with fri.Branch.AuthenticateToCap, and returns the dynamic path's values for
// it.
func dpathTree(t *testing.T, heights []int, seed uint64) dpathValues {
	t.Helper()
	rnd := dpRand{rand.New(rand.NewPCG(seed, 31))}
	order := canonicalOrder(heights)
	top := heights[order[0]]
	digests := make([][]Cell, len(heights))
	for g, h := range heights {
		for range 1 << h {
			digests[g] = append(digests[g], rnd.cell())
		}
	}
	// level[h] holds the 2^h nodes at height h
	level := make([][]Cell, top+1)
	level[top] = make([]Cell, 1<<top)
	for j := range level[top] {
		started := false
		for _, g := range order {
			if heights[g] != top {
				continue
			}
			if !started {
				level[top][j], started = digests[g][j], true
			} else {
				level[top][j] = compressCell(level[top][j], digests[g][j])
			}
		}
	}
	for h := top; h >= 1; h-- {
		level[h-1] = make([]Cell, 1<<(h-1))
		for k := range level[h-1] {
			node := compressCell(level[h][2*k], level[h][2*k+1])
			for _, g := range order {
				if heights[g] == h-1 {
					node = compressCell(node, digests[g][k])
				}
			}
			level[h-1][k] = node
		}
	}

	i := rnd.IntN(1 << top)
	branch := fri.Branch{Leaf: field.Octuplet(level[top][i]), AuxSiblings: make([][]field.Octuplet, top)}
	for h := 1; h <= top; h++ {
		branch.Siblings = append(branch.Siblings, field.Octuplet(level[h][(i>>(top-h))^1]))
	}
	for l := range top {
		for _, g := range order {
			if heights[g] == l {
				branch.AuxSiblings[l] = append(branch.AuxSiblings[l], field.Octuplet(digests[g][i>>(top-l)]))
			}
		}
	}
	if top > 0 {
		require.NoError(t, branch.AuthenticateToCap(i, []field.Octuplet{field.Octuplet(level[0][0])}),
			"the test's tree must be ray's")
	}

	v := dpathValues{leaves: make([]Cell, len(heights)), heights: heights, order: order, sibs: make([]Cell, dpH), root: level[0][0]}
	for g, h := range heights {
		v.leaves[g] = digests[g][i>>(top-h)]
	}
	for h := 1; h <= top; h++ {
		v.sibs[h-1] = level[h][(i>>(top-h))^1]
	}
	// the index at height H: the leaf's index, then bits below the tree
	v.index = uint64(i)<<(dpH-top) | rnd.Uint64N(1<<(dpH-top))
	return v
}

// cells returns the input cells of v, in the circuit's input order.
func (v dpathValues) cells() []Cell {
	var res []Cell
	res = append(res, v.leaves...)
	for _, h := range v.heights {
		res = append(res, ScalarCell(field.NewElement(uint64(h))))
	}
	for _, g := range v.order {
		res = append(res, ScalarCell(field.NewElement(uint64(g))))
	}
	res = append(res, v.sibs...)
	return append(res, ScalarCell(field.NewElement(v.index)), v.root)
}

// Input cells of dpathCircuit (inputs are the machine's first cells), and its
// tables, in creation order.
const (
	dpLeafCell   = 0
	dpHeightCell = dpLeafCell + dpG
	dpOrderCell  = dpHeightCell + dpG
	dpSibCell    = dpOrderCell + dpG
	dpIndexCell  = dpSibCell + dpH
	dpRootCell   = dpIndexCell + 1
)

const (
	dpLeafTable = iota
	dpHeightTable
	dpOrderTable
	dpSibTable
)

// dpathCircuit is one dynamic path over tables of input cells, in the order
// of dpathValues.cells.
func dpathCircuit() *Machine {
	m := &Machine{}
	dpathCircuitOn(m)
	return m
}

// dpathCircuitOn adds dpathCircuit's path to m.
func dpathCircuitOn(m *Machine) {
	inputs := func(n, kind int) []int {
		res := make([]int, n)
		for i := range res {
			res[i] = m.input(kind)
		}
		return res
	}
	leaves := m.Table(inputs(dpG, KindDigest))
	heights, order := m.Table(inputs(dpG, KindScalar)), m.Table(inputs(dpG, KindScalar))
	sibs := m.Table(inputs(dpH, KindDigest))
	index, root := m.input(KindScalar), m.input(KindDigest)
	m.MerklePathDynamic(DynamicPath{Leaves: leaves, Heights: heights, Order: order, Siblings: sibs, Index: index, Root: root})
}

// TestDynamicPath: one compiled circuit checks the openings of trees of every
// shape: distinct heights, all groups at the top, a top below H, the top group
// changing, all at height 1, a group at H, groups injected at the root
// (height 0), and a tree of one pair.
func TestDynamicPath(t *testing.T) {
	p, err := dpathCircuit().Compile(testPCSOpts...)
	require.NoError(t, err)
	for i, hs := range [][]int{{5, 3, 3, 2}, {8, 8, 8, 8}, {4, 4, 4, 4}, {2, 6, 6, 1}, {1, 1, 1, 1}, {8, 1, 4, 4},
		{3, 7, 3, 7}, {3, 0, 2, 0}, {0, 0, 0, 0}} {
		v := dpathTree(t, hs, uint64(i))
		r, err := p.Execute(v.cells())
		require.NoError(t, err, "heights %v", hs)
		require.NoError(t, proveAndVerify(p, r), "heights %v", hs)
	}
}

// groupedTable is a grouped commitment (pcs.WithModuleGroups) of dpG modules,
// module g of log size logSizes[g] at blowup 2, with g+1 base columns: a
// group per module, the groups of a size in module order.
func groupedTable(logSizes []int, seed uint64) fri.MultiSizeTable {
	rnd := rand.New(rand.NewPCG(seed, 5))
	top := 0
	for _, l := range logSizes {
		top = max(top, l+1)
	}
	table := make(fri.MultiSizeTable, top+1)
	for g, l := range logSizes {
		rows := 1 << (l + 1) // the codeword, at blowup 2
		sized := &table[l+1]
		for range g + 1 {
			col := make([]field.Element, rows)
			for i := range col {
				col[i] = field.NewElement(rnd.Uint64())
			}
			sized.Base = append(sized.Base, col)
		}
		sized.Groups = append(sized.Groups, fri.GroupShape{BaseWidth: g + 1, Ordinal: g})
	}
	return table
}

// groupedValues returns the dynamic path's values for the opening of a grouped
// commitment's tree at index s (of dpH bits): module g is the group of height
// its log size (its row pairs, at blowup 2), its leaf its digest at its pair on
// the path, which a commitment of that group alone gives.
func groupedValues(t *testing.T, table fri.MultiSizeTable, logSizes []int, s uint64) dpathValues {
	t.Helper()
	tree := table.Merkleize()
	top := len(table) - 2 // the bottom's pairs
	i := int(s >> (dpH - top))
	v := dpathValues{heights: make([]int, len(logSizes)), sibs: make([]Cell, dpH), index: s, root: Cell(tree.Root())}
	for g, l := range logSizes {
		v.heights[g] = l
		size := l + 1
		col := 0
		for _, gs := range table[size].Groups {
			if gs.Ordinal == g {
				break
			}
			col += gs.BaseWidth
		}
		alone := make(fri.MultiSizeTable, size+1)
		alone[size] = fri.SizedTable{Base: table[size].Base[col : col+g+1], Groups: []fri.GroupShape{{BaseWidth: g + 1}}}
		v.leaves = append(v.leaves, Cell(alone.Merkleize().OpenBranch(i>>(top-l)).Leaf))
	}
	v.order = canonicalOrder(v.heights)
	branch := tree.OpenBranch(i)
	for h := 1; h <= top; h++ {
		v.sibs[h-1] = Cell(branch.Siblings[h-1])
	}
	return v
}

// TestDynamicPathGroupedTrees: one compiled circuit verifies openings of real
// grouped commitments (fri.MultiSizeTable.Merkleize) of the same modules at
// different sizes (modules of size 1, injected at the root, included), the
// trees' heights, the groups' order and which groups share a height all
// changing, at several positions.
func TestDynamicPathGroupedTrees(t *testing.T) {
	p, err := dpathCircuit().Compile(testPCSOpts...)
	require.NoError(t, err)
	for k, logSizes := range [][]int{{4, 3, 3, 1}, {2, 5, 1, 5}, {1, 1, 1, 1}, {6, 2, 7, 2}, {3, 0, 2, 0}} {
		table := groupedTable(logSizes, uint64(k))
		for _, s := range []uint64{0, 1<<dpH - 1, 0xa5} {
			v := groupedValues(t, table, logSizes, s)
			r, err := p.Execute(v.cells())
			require.NoError(t, err, "log sizes %v, index %#x", logSizes, s)
			require.NoError(t, proveAndVerify(p, r), "log sizes %v, index %#x", logSizes, s)
		}
	}
}

// TestDynamicPathOrderIsTheCallers: the chip does not check the order, which
// is the caller's check. A path folding group 1 twice and group 2 never, with
// the root that path reaches, verifies: group 2's leaf is not authenticated.
func TestDynamicPathOrderIsTheCallers(t *testing.T) {
	p, err := dpathCircuit().Compile(testPCSOpts...)
	require.NoError(t, err)
	v := dpathTree(t, []int{5, 3, 3, 2}, 7)
	v.order = []int{0, 1, 1, 3}
	r, err := p.Execute(v.cells())
	require.NoError(t, err)
	rows := r.dpathRows[0]
	v.root = rows[len(rows)-1].out
	r, err = p.Execute(v.cells())
	require.NoError(t, err)
	require.NoError(t, proveAndVerify(p, r), "the chip accepts any order; the caller checks it")
}

// TestDynamicPathRejects: statements of the tree of heights 5, 3, 3, 2 with
// one value wrong, which execute: the circuit rejects them (the root read,
// for all of these, since the path then reaches another root; the forgeries
// below isolate each check). Execution rejects orders whose heights
// increase, heights above H and an index of more than H bits: no rows follow
// from them.
func TestDynamicPathRejects(t *testing.T) {
	hs := []int{5, 3, 3, 2}
	p, err := dpathCircuit().Compile(testPCSOpts...)
	require.NoError(t, err)
	for name, edit := range map[string]func(v *dpathValues){
		// group 2 claimed at height 2: folded at the wrong level
		"wrong height": func(v *dpathValues) {
			v.heights = []int{5, 3, 2, 2}
			v.order = []int{0, 1, 2, 3}
		},
		"sibling": func(v *dpathValues) { v.sibs[2][0].SetUint64(9) },
		// the direction bit of height 3: bit H − 3 of the index
		"direction bit": func(v *dpathValues) { v.index ^= 1 << (dpH - 3) },
		"root":          func(v *dpathValues) { v.root[1].SetUint64(9) },
		"leaf":          func(v *dpathValues) { v.leaves[3][2].SetUint64(9) },
	} {
		t.Run(name, func(t *testing.T) {
			v := dpathTree(t, hs, 7)
			v.heights = append([]int(nil), v.heights...)
			v.sibs = append([]Cell(nil), v.sibs...)
			v.leaves = append([]Cell(nil), v.leaves...)
			edit(&v)
			r, err := p.Execute(v.cells())
			require.NoError(t, err, "not executed")
			require.Error(t, proveAndVerify(p, r), "tampered opening accepted")
		})
	}

	for name, edit := range map[string]func(v *dpathValues){
		"increasing order": func(v *dpathValues) { v.order = []int{3, 0, 1, 2} },
		"height above H":   func(v *dpathValues) { v.heights = []int{dpH + 1, 3, 3, 2} },
		"index above 2^H":  func(v *dpathValues) { v.index += 1 << dpH },
	} {
		v := dpathTree(t, hs, 7)
		v.heights = append([]int(nil), v.heights...)
		edit(&v)
		_, err := p.Execute(v.cells())
		require.Error(t, err, name)
	}
}

// TestDynamicPathHeightBound: a tree has at most field.Bits − 1 levels, since
// idx = Σ bᵢ·2ⁱ is injective only below the field's modulus.
func TestDynamicPathHeightBound(t *testing.T) {
	for _, tc := range []struct {
		height int
		ok     bool
	}{{1, true}, {field.Bits - 1, true}, {field.Bits, false}} {
		t.Run(fmt.Sprint(tc.height), func(t *testing.T) {
			m := &Machine{}
			table := func(n int) Table {
				cells := make([]int, n)
				for i := range cells {
					cells[i] = m.InputScalar()
				}
				return m.Table(cells)
			}
			p := DynamicPath{Leaves: table(1), Heights: table(1), Order: table(1), Siblings: table(tc.height),
				Index: m.InputScalar(), Root: m.InputDigest()}
			build := func() { m.MerklePathDynamic(p) }
			if tc.ok {
				require.NotPanics(t, build)
			} else {
				require.Panics(t, build)
			}
		})
	}
}

// dpCompress returns a row's compression and its P2 input: its running hash
// and operand, ordered by its direction bit.
func dpCompress(row dpathRow) (Cell, [P2Width]field.Element) {
	left, right := row.cur, row.op
	if row.b == 1 {
		left, right = right, left
	}
	return compressCell(left, right), p2Input(left, right)
}

// replay recomputes a forged path from its rows' kinds, bits, operands and
// started flags, as the chip's constraints but the forged one require: the
// running hash (a copy before the first leaf, the first leaf, then
// compressions, recorded for the P2 core), the root it reaches, and the index
// bits it consumes (the index cell, from the last row's bit, plus extra on
// the last row, up).
func replay(r *Run, rows []dpathRow, extra uint64) {
	r.p2 = r.p2[:0]
	cur := rows[0].cur
	for k := range rows {
		rows[k].cur = cur
		switch {
		case rows[k].started:
			var in [P2Width]field.Element
			rows[k].out, in = dpCompress(rows[k])
			r.p2 = append(r.p2, in)
		case rows[k].fold:
			rows[k].out = rows[k].op
		default:
			rows[k].out = cur
		}
		cur = rows[k].out
	}
	r.values[dpRootCell] = cur
	idx := rows[len(rows)-1].b + extra
	rows[len(rows)-1].idx = idx
	for k := len(rows) - 2; k >= 0; k-- {
		if rows[k].fold {
			idx += rows[k].b
		} else {
			idx = rows[k].b + 2*idx
		}
		rows[k].idx = idx
	}
	r.values[dpIndexCell] = ScalarCell(field.NewElement(idx))
}

// TestDynamicPathRejectsForged: on the tree of heights 5, 3, 3, 2, whose rows
// are three copies (heights 8, 7, 6), group 0's fold (row 3, the path's first
// leaf), levels 5 and 4, the folds of groups 1 and 2 at height 3, level 3,
// group 3's fold, levels 2 and 1, the run is forged so that only one check of
// the chip sees it: its rows, the tables' read counts, the P2 core's
// compressions and the cells change consistently, and the trace follows.
// Every check of the chip but kind's booleanity (which keeps the fold and
// level weights 0 or 1) rejects one here or in TestDynamicPathRejectsPadding.
func TestDynamicPathRejectsForged(t *testing.T) {
	// the compressions start at row 4, the first after the first leaf
	const firstHash = 4
	for name, forge := range map[string]func(r *Run, rows []dpathRow){
		// b boolean: a bit of 2 on a copy row, where nothing hashes
		"non-boolean bit on a copy row": func(r *Run, rows []dpathRow) {
			rows[0].b = 2
			replay(r, rows, 0)
		},
		// kind·b = 0: group 1 folded with its leaf on the left
		"fold direction": func(r *Run, rows []dpathRow) {
			rows[6].b = 1
			replay(r, rows, 0)
		},
		// the index's bound: an index of H + 1 bits, the extra one left on
		// the last row
		"index above 2^H": func(r *Run, rows []dpathRow) { replay(r, rows, 2) },
		// copy before the leaf: the first fold's output is not its leaf
		"first fold's output": func(r *Run, rows []dpathRow) {
			r.values[dpLeafCell][0].SetUint64(77)
			rows[3].op = r.values[dpLeafCell]
		},
		// cur = out[−1]: group 3's fold forged, the next row unchanged
		"chain after a fold": func(r *Run, rows []dpathRow) {
			r.values[dpLeafCell+3][0].SetUint64(77)
			rows[9].op = r.values[dpLeafCell+3]
			var in [P2Width]field.Element
			rows[9].out, in = dpCompress(rows[9])
			r.p2[9-firstHash] = in
		},
		// hc transition: a copy row at another height, reading that height's
		// sibling (zero, like its own)
		"height of a copy row": func(r *Run, rows []dpathRow) {
			if c := r.tableReads[dpSibTable]; c != nil {
				c[6]--
				c[5]++
			}
			rows[1].hc = 6
		},
		// fc transition: a level row's fold count, which it reads nothing with
		"fold count of a level row": func(_ *Run, rows []dpathRow) { rows[4].fc = 3 },
		// started transition: a copy row hashes, the next one copies again,
		// and the first fold takes its leaf whatever the running hash
		"started on a copy row": func(r *Run, rows []dpathRow) {
			var in [P2Width]field.Element
			rows[1].started = true
			rows[1].out, in = dpCompress(rows[1])
			r.p2 = append(r.p2, in)
			rows[2].cur, rows[2].out = rows[1].out, rows[1].out
			rows[3].cur = rows[1].out
		},
		// started = 0 on the first row: every row hashes, the copies above
		// the tree included, and the statement's root is that path's
		"started from the first row": func(r *Run, rows []dpathRow) {
			for k := range rows {
				rows[k].started = true
			}
			replay(r, rows, 0)
		},
		// the reads: a cell differs from what the rows use
		"leaf cell":    func(r *Run, _ []dpathRow) { r.values[dpLeafCell+1][2].SetUint64(77) },
		"sibling cell": func(r *Run, _ []dpathRow) { r.values[dpSibCell+2][2].SetUint64(77) },
		"order cell": func(r *Run, _ []dpathRow) {
			r.values[dpOrderCell+1], r.values[dpOrderCell+2] = r.values[dpOrderCell+2], r.values[dpOrderCell+1]
		},
		// group 3 claimed at height 5 while it folds at height 2
		"height cell": func(r *Run, _ []dpathRow) { r.values[dpHeightCell+3] = ScalarCell(field.NewElement(5)) },
		"index cell": func(r *Run, _ []dpathRow) {
			r.values[dpIndexCell][0].SetUint64(r.values[dpIndexCell][0].Uint64() ^ 1<<(dpH-3))
		},
		// the P2 lookup: a node replaced, the chain kept
		"forged node": func(_ *Run, rows []dpathRow) {
			rows[5].out[0].SetUint64(77)
			rows[6].cur = rows[5].out
		},
	} {
		t.Run(name, func(t *testing.T) {
			p, err := dpathCircuit().Compile(testPCSOpts...)
			require.NoError(t, err)
			r, err := p.Execute(dpathTree(t, []int{5, 3, 3, 2}, 7).cells())
			require.NoError(t, err)
			require.NoError(t, proveAndVerify(p, r), "honest")
			forge(r, r.dpathRows[0])
			require.NoError(t, p.trace(r))
			require.Error(t, proveAndVerify(p, r), "forged run accepted")
		})
	}
}

// setRow sets the dpath chip's witness on a row, past the trace: the module's
// padding rows, which the trace leaves zero.
func setRow(p *Program, r *Run, row int, cols map[string]field.Element, cells map[string]Cell) {
	for name, v := range cols {
		r.Witness(p, dpathMod, name)[row] = v
	}
	for prefix, c := range cells {
		for i := range CellWidth {
			r.Witness(p, dpathMod, laneName(prefix, i))[row] = c[i]
		}
	}
}

// TestDynamicPathRejectsPadding: the module's padding rows (16 rows for 12
// path rows) have no setup selector, and kind and started are witness
// selectors there too: only (1 − active)·started = 0 and (1 − active)·kind = 0
// keep a padding row from accessing the buses.
//   - started = −1: a padding row's P2 read of weight −1 writes a compression
//     that is not one, for a path that copies at level 4 instead of hashing
//     it (the statement's root being that path's);
//   - kind = 1: a padding row's sibling read, of weight active − kind = −1,
//     writes a table entry. Table 0 here is a table of scalars [1, 3, 9, 4],
//     read by a Lookup at index 2. With the zero tags of padding, the row
//     reads its group 1 at entry 0 (fc = 0), its leaf and its height 3 at
//     entry 1, and writes the entry (2, 3): the Lookup would read 3 instead
//     of 9.
func TestDynamicPathRejectsPadding(t *testing.T) {
	const firstHash, padding = 4, 12
	t.Run("started", func(t *testing.T) {
		p, err := dpathCircuit().Compile(testPCSOpts...)
		require.NoError(t, err)
		r, err := p.Execute(dpathTree(t, []int{5, 3, 3, 2}, 7).cells())
		require.NoError(t, err)
		require.NoError(t, proveAndVerify(p, r), "honest")
		rows := r.dpathRows[0]
		skipped := rows[5] // level 4: copied instead of hashed
		rows[5].out = rows[5].cur
		p2 := append([][P2Width]field.Element(nil), r.p2[:5-firstHash]...)
		for k := 6; k < len(rows); k++ {
			rows[k].cur = rows[k-1].out
			var in [P2Width]field.Element
			rows[k].out, in = dpCompress(rows[k])
			p2 = append(p2, in)
		}
		r.p2 = p2
		r.values[dpRootCell] = rows[len(rows)-1].out
		require.NoError(t, p.trace(r))
		minusOne := field.NewElement(0)
		one := field.One()
		minusOne.Sub(&minusOne, &one)
		// the padding row writes the copied level's tuple: inputs ordered by
		// its bit, output its running hash
		setRow(p, r, padding, map[string]field.Element{dpathStarted: minusOne, dpathB: field.NewElement(skipped.b)},
			map[string]Cell{dpathCur: skipped.cur, dpathOp: skipped.op, dpathOut: skipped.cur})
		require.Error(t, proveAndVerify(p, r), "a padding row wrote a compression")
	})

	t.Run("kind", func(t *testing.T) {
		m := &Machine{}
		cp := withCopy(m)
		scalars := []uint64{1, 3, 9, 4}
		t0cells := make([]int, 0, len(scalars))
		for range scalars {
			t0cells = append(t0cells, m.input(KindScalar))
		}
		t0 := m.Table(t0cells) // tag 0
		dpathCircuitOn(m)
		idx := m.input(KindScalar)
		v := m.Lookup(t0, idx)
		cp.Copy(v)
		p, err := m.Compile(testPCSOpts...)
		require.NoError(t, err)
		in := make([]Cell, 0, len(scalars))
		for _, x := range scalars {
			in = append(in, ScalarCell(field.NewElement(x)))
		}
		in = append(in, dpathTree(t, []int{5, 3, 3, 2}, 7).cells()...)
		in = append(in, ScalarCell(field.NewElement(2)))
		r, err := p.Execute(in)
		require.NoError(t, err)
		require.Equal(t, field.NewElement(9), r.Value(v)[0])
		require.NoError(t, proveAndVerify(p, r), "honest")

		r.values[v] = ScalarCell(field.NewElement(3)) // T0[2] read as 3, not 9
		require.NoError(t, p.trace(r))
		cnt := r.Witness(p, tableMod, tableCnt)
		cnt[0].SetUint64(1) // entry 0 read by the padding row's order read
		cnt[1].SetUint64(2) // entry 1 by its leaf and height reads
		cnt[2].SetZero()    // entry 2 by none
		three := field.NewElement(3)
		setRow(p, r, padding, map[string]field.Element{dpathKind: field.One(), dpathPi: field.One(), dpathHc: three},
			map[string]Cell{dpathOp: ScalarCell(three), dpathOut: ScalarCell(three)})
		require.Error(t, proveAndVerify(p, r), "a padding row injected a table entry")
	})
}
