package recursion

import (
	"math/rand/v2"
	"sort"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/fri"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/stretchr/testify/require"
)

// The dynamic path circuit of these tests: G groups, heights up to H,
// positions of L bits.
const (
	dpG = 4
	dpH = 8
	dpL = 10
)

// dpathValues are the input values of a dynamic path: the groups' leaves and
// heights, the order, the siblings by height, the position's bits and the
// root.
type dpathValues struct {
	leaves  []Cell
	heights []int
	order   []int
	sibs    []Cell
	bits    []uint64
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
// given heights, as fri.Tree hashes it: a leaf chains the digests of the
// groups of the top height, in order, and a node at height h chains, after
// compressing its children, the digests of the groups of height h. It opens
// it at a random leaf, checks the opening with fri.Branch.AuthenticateToCap,
// and returns the dynamic path's values for it.
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
	require.NoError(t, branch.AuthenticateToCap(i, []field.Octuplet{field.Octuplet(level[0][0])}),
		"the test's tree must be ray's")

	v := dpathValues{leaves: make([]Cell, len(heights)), heights: heights, order: order, sibs: make([]Cell, dpH), root: level[0][0]}
	for g, h := range heights {
		v.leaves[g] = digests[g][i>>(top-h)]
	}
	for h := 1; h <= top; h++ {
		v.sibs[h-1] = level[h][(i>>(top-h))^1]
	}
	v.setPosition(uint64(i)<<(dpL-top) | rnd.Uint64N(1<<(dpL-top)))
	return v
}

// setPosition sets the bits of the queried position.
func (v *dpathValues) setPosition(s uint64) {
	v.bits = v.bits[:0]
	for j := range dpL {
		v.bits = append(v.bits, s>>j&1)
	}
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
	for _, b := range v.bits {
		res = append(res, ScalarCell(field.NewElement(b)))
	}
	return append(res, v.root)
}

// dpathCircuit is one dynamic path over tables of input cells, in the order
// of dpathValues.cells; the root is copied, so that it is read on the bus
// besides the path's own read.
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
	sibs, bits := m.Table(inputs(dpH, KindDigest)), m.Table(inputs(dpL, KindScalar))
	root := m.input(KindDigest)
	m.MerklePathDynamic(DynamicPath{Leaves: leaves, Heights: heights, Order: order, Siblings: sibs, Bits: bits, L: dpL, Root: root})
}

// TestDynamicPath: one compiled circuit checks the openings of trees of every
// shape: distinct heights, all groups at the top, a top below H, the top group
// changing, all at height 1, a group at H.
func TestDynamicPath(t *testing.T) {
	p, err := dpathCircuit().Compile(testPCSOpts...)
	require.NoError(t, err)
	for i, hs := range [][]int{{5, 3, 3, 2}, {8, 8, 8, 8}, {4, 4, 4, 4}, {2, 6, 6, 1}, {1, 1, 1, 1}, {8, 1, 4, 4}, {3, 7, 3, 7}} {
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
// commitment's tree at position s (of dpL bits): module g is the group of
// height log size + 1 − 1 (its row pairs), its leaf its digest at its pair on
// the path, which a commitment of that group alone gives.
func groupedValues(t *testing.T, table fri.MultiSizeTable, logSizes []int, s uint64) dpathValues {
	t.Helper()
	tree := table.Merkleize()
	top := len(table) - 2 // the bottom's pairs
	i := int(s >> (dpL - top))
	v := dpathValues{heights: make([]int, len(logSizes)), sibs: make([]Cell, dpH), root: Cell(tree.Root())}
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
	v.setPosition(s)
	return v
}

// TestDynamicPathGroupedTrees: one compiled circuit verifies openings of real
// grouped commitments (fri.MultiSizeTable.Merkleize) of the same modules at
// different sizes, the trees' heights, the groups' order and which groups
// share a height all changing, at several positions.
func TestDynamicPathGroupedTrees(t *testing.T) {
	p, err := dpathCircuit().Compile(testPCSOpts...)
	require.NoError(t, err)
	for k, logSizes := range [][]int{{4, 3, 3, 1}, {2, 5, 1, 5}, {1, 1, 1, 1}, {6, 2, 7, 2}} {
		table := groupedTable(logSizes, uint64(k))
		for _, s := range []uint64{0, 1<<dpL - 1, 0x2a5} {
			v := groupedValues(t, table, logSizes, s)
			r, err := p.Execute(v.cells())
			require.NoError(t, err, "log sizes %v, position %#x", logSizes, s)
			require.NoError(t, proveAndVerify(p, r), "log sizes %v, position %#x", logSizes, s)
		}
	}
}

// TestDynamicPathRejects: openings the circuit must reject, on the tree of
// heights 5, 3, 3, 2: statements that execute (the circuit, not the
// executor, rejects them).
func TestDynamicPathRejects(t *testing.T) {
	hs := []int{5, 3, 3, 2}
	p, err := dpathCircuit().Compile(testPCSOpts...)
	require.NoError(t, err)
	for name, edit := range map[string]func(v *dpathValues){
		// groups 1 and 2 share height 3: folded in the other order
		"order of equal heights": func(v *dpathValues) { v.order = []int{0, 2, 1, 3} },
		// group 1 folded twice, group 2 never
		"group twice": func(v *dpathValues) { v.order = []int{0, 1, 1, 3} },
		// group 2 claimed at height 2: folded at the wrong level
		"wrong height": func(v *dpathValues) {
			v.heights = []int{5, 3, 2, 2}
			v.order = []int{0, 1, 2, 3}
		},
		"sibling": func(v *dpathValues) { v.sibs[2][0].SetUint64(9) },
		// the direction bit of height 3: bit L − 3 of the position
		"direction bit": func(v *dpathValues) { v.bits[dpL-3] ^= 1 },
		"root":          func(v *dpathValues) { v.root[1].SetUint64(9) },
		"leaf":          func(v *dpathValues) { v.leaves[3][2].SetUint64(9) },
	} {
		t.Run(name, func(t *testing.T) {
			v := dpathTree(t, hs, 7)
			v.heights = append([]int(nil), v.heights...)
			v.sibs = append([]Cell(nil), v.sibs...)
			v.leaves = append([]Cell(nil), v.leaves...)
			v.bits = append([]uint64(nil), v.bits...)
			edit(&v)
			r, err := p.Execute(v.cells())
			require.NoError(t, err, "not executed")
			require.Error(t, proveAndVerify(p, r), "tampered opening accepted")
		})
	}

	// execution rejects orders whose heights increase, and heights out of
	// [1, H]: no rows follow from them
	for name, edit := range map[string]func(v *dpathValues){
		"increasing order": func(v *dpathValues) { v.order = []int{3, 0, 1, 2} },
		"height 0":         func(v *dpathValues) { v.heights = []int{5, 3, 3, 0} },
		"height above H":   func(v *dpathValues) { v.heights = []int{dpH + 1, 3, 3, 2} },
	} {
		v := dpathTree(t, hs, 7)
		v.heights = append([]int(nil), v.heights...)
		edit(&v)
		_, err := p.Execute(v.cells())
		require.Error(t, err, name)
	}
}

// TestDynamicPathRejectsTrace: traces tampered past the executor, on the tree
// of heights 5, 3, 3, 2, whose rows are H − 5 = 3 copies, then group 0 (the
// leaf) at row 3.
func TestDynamicPathRejectsTrace(t *testing.T) {
	for name, tamper := range map[string]func(p *Program, r *Run){
		// a fold with its leaf on the left
		"fold direction": func(p *Program, r *Run) { r.Witness(p, dpathMod, dpathB)[3].SetOne() },
		// a copy row (before the leaf) that changes the running value
		"copy": func(p *Program, r *Run) {
			out := r.Witness(p, dpathMod, laneName(dpathOut, 0))
			one := field.One()
			out[1].Add(&out[1], &one)
		},
		// a level skipped: a level row turned into a fold
		"kind": func(p *Program, r *Run) { r.Witness(p, dpathMod, dpathKind)[2].SetOne() },
	} {
		t.Run(name, func(t *testing.T) {
			p, err := dpathCircuit().Compile(testPCSOpts...)
			require.NoError(t, err)
			r, err := p.Execute(dpathTree(t, []int{5, 3, 3, 2}, 7).cells())
			require.NoError(t, err)
			require.NoError(t, proveAndVerify(p, r), "honest")
			tamper(p, r)
			require.Error(t, proveAndVerify(p, r), "tampered trace accepted")
		})
	}
}

// TestDynamicPathFoldsAtItsHeight: a group must fold at the height its
// Heights entry gives. The tree of heights 5, 3, 3, 2 holds group 2 at height
// 3. Claiming height 2 for it, a forged trace still folds it at height 3 (row
// 7: 3 copies, group 0, two levels, group 1, group 2), where the tree needs
// it, and reads the claimed 2 consistently (the input cell, the table's
// entry, the row's hg): only the fold-height constraint rejects it.
func TestDynamicPathFoldsAtItsHeight(t *testing.T) {
	p, err := dpathCircuit().Compile(testPCSOpts...)
	require.NoError(t, err)
	v := dpathTree(t, []int{5, 3, 3, 2}, 7)
	r, err := p.Execute(v.cells())
	require.NoError(t, err)
	require.NoError(t, proveAndVerify(p, r), "honest")
	heightCell := dpG + 2 // the input cell of heights[2]
	r.values[heightCell] = ScalarCell(field.NewElement(2))
	rows := r.dpathRows[0]
	require.True(t, rows[7].fold && rows[7].pi == 2, "row 7 folds group 2")
	require.NoError(t, p.trace(r)) // the witness and table chips follow the cell
	r.Witness(p, dpathMod, dpathHg)[7] = field.NewElement(2)
	require.Error(t, proveAndVerify(p, r), "group folded at another height than its claimed one")
}

// Input cells of dpathCircuit (inputs are the machine's first cells): the
// leaves, heights, order, siblings and bits tables, then the root.
const (
	dpLeafCell   = 0
	dpHeightCell = dpLeafCell + dpG
	dpOrderCell  = dpHeightCell + dpG
	dpSibCell    = dpOrderCell + dpG
	dpBitCell    = dpSibCell + dpH
	dpRootCell   = dpBitCell + dpL
)

// dpCompress returns a row's compression and its P2 input: its running hash
// and operand, ordered by its direction bit.
func dpCompress(row dpathRow) (Cell, [P2Width]field.Element) {
	left, right := row.cur, row.op
	if row.b == 1 {
		left, right = right, left
	}
	return compressCell(left, right), p2Input(left, right)
}

// TestDynamicPathRejectsForged: on the tree of heights 5, 3, 3, 2, whose rows
// are three copies (heights 8, 7, 6), group 0's fold (row 3, the path's first
// leaf), levels 5 and 4, the folds of groups 1 and 2 at height 3, level 3,
// group 3's fold, levels 2 and 1, the run is forged so that only one check of
// the chip sees it: its rows, the tables' read counts, the P2 core's
// compressions and the cells change consistently, and the trace follows.
// Every check that a forgery can isolate rejects one here.
func TestDynamicPathRejectsForged(t *testing.T) {
	// the compressions start at row 4, the first after the first leaf
	const firstHash = 4
	for name, forge := range map[string]func(r *Run, rows []dpathRow){
		// b boolean: a bit of 2 on a copy row, where nothing hashes
		"non-boolean bit on a copy row": func(r *Run, rows []dpathRow) {
			r.values[dpBitCell+dpL-dpH] = ScalarCell(field.NewElement(2))
			rows[0].b = 2
		},
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
		// sibling (zero, like its own) and bit
		"height of a copy row": func(r *Run, rows []dpathRow) {
			moveRead(r, dpSibTable, 6, 5)
			moveRead(r, dpBitTable, dpL-7, dpL-6)
			rows[1].hc = 6
			rows[1].b = r.values[dpBitCell+dpL-6][0].Uint64()
		},
		// fc transition: a level row's fold count, which it reads nothing with
		"fold count of a level row": func(_ *Run, rows []dpathRow) { rows[4].fc = 3 },
		// started transition: a copy row hashes, the next one copies again
		"started on a copy row": func(r *Run, rows []dpathRow) {
			var in [P2Width]field.Element
			rows[1].started = true
			rows[1].out, in = dpCompress(rows[1])
			r.p2 = append(r.p2, in)
			rows[2].cur, rows[2].out = rows[1].out, rows[1].out
			rows[3].cur = rows[1].out // the first fold takes its leaf whatever cur
		},
		// started = 0 on the first row: every row hashes, the copies above
		// the tree included, and the statement's root is that path's
		"started from the first row": func(r *Run, rows []dpathRow) {
			r.p2 = r.p2[:0]
			var cur Cell
			for k := range rows {
				rows[k].started, rows[k].cur = true, cur
				var in [P2Width]field.Element
				rows[k].out, in = dpCompress(rows[k])
				r.p2 = append(r.p2, in)
				cur = rows[k].out
			}
			r.values[dpRootCell] = cur
		},
		// the reads: a table cell differs from what the rows use
		"leaf cell":    func(r *Run, _ []dpathRow) { r.values[dpLeafCell+1][2].SetUint64(77) },
		"sibling cell": func(r *Run, _ []dpathRow) { r.values[dpSibCell+2][2].SetUint64(77) },
		"order cell": func(r *Run, _ []dpathRow) {
			r.values[dpOrderCell+1], r.values[dpOrderCell+2] = r.values[dpOrderCell+2], r.values[dpOrderCell+1]
		},
		"height cell": func(r *Run, _ []dpathRow) { r.values[dpHeightCell+3] = ScalarCell(field.NewElement(5)) },
		"bit cell": func(r *Run, _ []dpathRow) {
			r.values[dpBitCell+dpL-3][0].SetUint64(r.values[dpBitCell+dpL-3][0].Uint64() ^ 1)
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

// The tables of dpathCircuit, in creation order.
const (
	dpLeafTable = iota
	dpHeightTable
	dpOrderTable
	dpSibTable
	dpBitTable
)

// moveRead moves one counted read of a table from entry i to entry j, if the
// run counts that table's reads.
func moveRead(r *Run, table, i, j int) {
	if reads := r.tableReads[table]; reads != nil {
		reads[i]--
		reads[j]++
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

// TestDynamicPathRejectsPadding: the module's padding rows (12 path rows,
// 16 rows) have no setup selector, and kind and started are witness
// selectors there: only their booleanity keeps a padding row from turning a
// read into a write.
//   - started = −1: a padding row supplies a compression that is not one, to
//     a path that copies at level 4 instead of hashing it (the statement's
//     root being that path's);
//   - kind = −1: a padding row writes the table entries that a mirror row,
//     kind = 1, reads (a net zero).
func TestDynamicPathRejectsPadding(t *testing.T) {
	const firstHash, padding = 4, 12
	minusOne := field.NewElement(0)
	one := field.One()
	minusOne.Sub(&minusOne, &one)
	for name, forge := range map[string]func(p *Program, r *Run){
		"started": func(p *Program, r *Run) {
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
			// the padding row writes the copied level's tuple: inputs ordered
			// by its bit, output its running hash
			setRow(p, r, padding, map[string]field.Element{dpathStarted: minusOne, dpathB: field.NewElement(skipped.b)},
				map[string]Cell{dpathCur: skipped.cur, dpathOp: skipped.op, dpathOut: skipped.cur})
		},
		"kind": func(p *Program, r *Run) {
			require.NoError(t, p.trace(r))
			x := cell(5)
			var neg Cell
			for i := range CellWidth {
				neg[i].Sub(&neg[i], &x[i])
			}
			// both: pi = 0, fc = 0, hg = hc = 1, b = 0, started = 0, op = x;
			// out follows the copy relation, out = cur + kind·(op − cur)
			setRow(p, r, padding, map[string]field.Element{dpathKind: minusOne, dpathHg: one, dpathHc: one},
				map[string]Cell{dpathOp: x, dpathOut: neg})
			setRow(p, r, padding+1, map[string]field.Element{dpathKind: one, dpathHg: one, dpathHc: one},
				map[string]Cell{dpathOp: x, dpathOut: x})
		},
	} {
		t.Run(name, func(t *testing.T) {
			p, err := dpathCircuit().Compile(testPCSOpts...)
			require.NoError(t, err)
			r, err := p.Execute(dpathTree(t, []int{5, 3, 3, 2}, 7).cells())
			require.NoError(t, err)
			require.NoError(t, proveAndVerify(p, r), "honest")
			forge(p, r)
			require.Error(t, proveAndVerify(p, r), "forged padding accepted")
		})
	}
}

// TestDynamicPathPaddingTags: the dpath chip's padding rows must not reach a
// table. Here table 0 is a table of scalars [3, 7, 9, 4], read by a Lookup at
// index 2. A padding row with kind = 1 reads entry 0 twice (its leaf and
// height reads, both 3) and writes, with its level reads (weight active −
// kind = −1), the entry (2, 3) and an entry its order read consumes: with
// zero tags it would inject T0[2] = 3, and the Lookup would read 3 instead of
// 9 in a proof that verifies. The padding rows' tags match no table, so it
// cannot.
func TestDynamicPathPaddingTags(t *testing.T) {
	m := &Machine{}
	cp := withCopy(m)
	scalars := []uint64{3, 7, 9, 4}
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
	cnt[0].SetUint64(2) // entry 0 read by the padding row twice
	cnt[2].SetZero()    // entry 2 read by none
	minus3 := field.NewElement(0)
	three := field.NewElement(3)
	minus3.Sub(&minus3, &three)
	setRow(p, r, 12, map[string]field.Element{dpathKind: field.One(), dpathHc: three, dpathHg: three, dpathFc: minus3},
		map[string]Cell{dpathOp: ScalarCell(three), dpathOut: ScalarCell(three)})
	require.Error(t, proveAndVerify(p, r), "a padding row injected a table entry")
}
