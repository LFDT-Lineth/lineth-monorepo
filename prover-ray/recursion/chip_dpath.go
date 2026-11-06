package recursion

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// Setup columns of the dpath chip.
const (
	dpathActive     = "active"
	dpathFirst      = "first"
	dpathLast       = "last"
	dpathLeafTag    = "leaf_tag" // the tables' tags
	dpathOrderTag   = "order_tag"
	dpathHeightsTag = "heights_tag"
	dpathSibTag     = "sib_tag"
	dpathAddrIdx    = "addr_idx"
	dpathAddrRoot   = "addr_root"
)

// Witness columns of the dpath chip.
const (
	dpathKind    = "kind"    // 1 on a fold row, 0 on a level row
	dpathHc      = "hc"      // the height before the row
	dpathStarted = "started" // a leaf has been taken before the row
	dpathFc      = "fc"      // the number of fold rows before the row
	dpathPi      = "pi"      // the group folded (fold rows)
	dpathB       = "b"       // the direction bit (level rows)
	dpathIdx     = "idx"     // the index bits left to consume
	dpathCur     = "cur"     // lanes: the running hash entering the row
	dpathOp      = "op"      // lanes: the sibling or the group's leaf
	dpathOut     = "out"     // lanes: the running hash leaving the row
)

// DynamicPath is a Merkle path in one of ray's mixed-height trees whose shape
// is data: the groups' heights are cells, not constants. That is the input
// tree of a grouped commitment (pcs.WithModuleGroups) whose modules' sizes are
// inputs: a module of log size ℓ at blowup 2^r is a group of height ℓ + r − 1
// (its 2^(ℓ+r−1) row pairs are the nodes at that height, counted from the
// root, height 0).
//
// The path goes from the maximum height H up to the root, whatever the tree's
// actual height. At every height h, it first folds the groups of height h, in
// order: the first fold of the path starts it with that group's leaf, every
// other fold compresses the running hash with the group's leaf, Compress(cur,
// leaf); then, above the root, it compresses the level with its sibling,
// ordered by bit H − h of the index (h → h − 1). The rows above the tree's
// actual height, before the first fold, copy. This is fri.Tree's hashing: a
// leaf chains its size's groups, and a node chains the groups injected at its
// height after compressing its children, the root (height 0) included.
type DynamicPath struct {
	// Leaves[g] is group g's leaf digest at the queried position; Heights[g]
	// its height, a scalar in [0, H].
	Leaves, Heights Table
	// Order[k] is the k-th group folded: the groups by decreasing height,
	// equal heights by increasing index, which is the order fri.Tree chains
	// them in (the groups of a size in module order). The chip does not check
	// it: the caller checks that it is that permutation. An index past G reads
	// no entry of Leaves and Heights, but a repeated group could, leaving
	// another group's leaf unauthenticated.
	Order Table
	// Siblings[h−1] is the sibling at height h (h = 1..H), anything above the
	// tree's actual height; H = Siblings.Len(), at most field.Bits − 1.
	Siblings Table
	// Index is the cell of the queried position's index at height H (lane
	// 0), below 2^H: the queried position of L bits shifted right by L − H.
	// Its bit H − h is the direction bit at height h.
	Index int
	Root  int
}

// dpathRow is one executed row of a dynamic path.
type dpathRow struct {
	fold         bool
	hc, fc       int
	started      bool
	pi           int
	b, idx       uint64
	cur, op, out Cell
}

// MerklePathDynamic checks a DynamicPath: that the path from the groups'
// leaves, at their heights, through the siblings and the index's bits,
// reaches the root. It takes one row per height (H) and per group (G),
// whatever the heights.
func (m *Machine) MerklePathDynamic(p DynamicPath) {
	H, G := p.Siblings.Len(), p.Order.Len()
	if p.Leaves.Len() != G || p.Heights.Len() != G {
		panic(fmt.Sprintf("MerklePathDynamic: %d leaves, %d heights, %d groups ordered", p.Leaves.Len(), p.Heights.Len(), G))
	}
	// idx = Σ b_i·2^i is injective only below the field's modulus
	if H >= field.Bits {
		panic(fmt.Sprintf("MerklePathDynamic: a tree of height %d, want at most %d", H, field.Bits-1))
	}
	for _, t := range []Table{p.Leaves, p.Heights, p.Order, p.Siblings} {
		m.checkTable(t)
	}
	m.read(p.Index)
	m.read(p.Root)
	m.dpaths = append(m.dpaths, p)
	m.record(instrDPath, len(m.dpaths)-1)
}

// numDPathRows is the number of rows the dynamic paths take, H + G each: the
// dpath chip's rows.
func (m *Machine) numDPathRows() int {
	n := 0
	for _, p := range m.dpaths {
		n += p.Siblings.Len() + p.Order.Len()
	}
	return n
}

// numDPathHashes bounds the dynamic paths' compressions: H + G − 1 each, every
// row but the first fold (and the copies before it).
func (m *Machine) numDPathHashes() int {
	n := 0
	for _, p := range m.dpaths {
		n += p.Siblings.Len() + p.Order.Len() - 1
	}
	return n
}

// entry returns the value of entry i of the table t.
func (m *Machine) entry(r *Run, t Table, i int) Cell { return r.values[m.tables[t.tag].src[i]] }

// execDPath builds the path's rows from the tables' values (the heights and
// the order decide which rows fold), counts its reads of the tables' entries,
// and records its compressions for the P2 core. It fails if the heights and
// the order do not fold every group, in non-increasing heights within [0, H],
// or if the index is not below 2^H: no rows follow from them. A path that
// misses the root executes, and its proof does not verify (the last row reads
// the root), so that tests can check the circuit rejects it.
func (m *Machine) execDPath(i int, r *Run) error {
	p := m.dpaths[i]
	H, G := p.Siblings.Len(), p.Order.Len()
	scalar := func(t Table, k int) int {
		e := m.entry(r, t, k)
		return int(e[0].Uint64())
	}
	if r.tableReads == nil {
		r.tableReads = make([][]uint64, len(m.tables))
	}
	count := func(t Table, k int) {
		if r.tableReads[t.tag] == nil {
			r.tableReads[t.tag] = make([]uint64, t.n)
		}
		r.tableReads[t.tag][k]++
	}
	index := r.values[p.Index][0].Uint64()
	if index>>H != 0 {
		return fmt.Errorf("MerklePathDynamic: index %d does not fit a tree of height %d", index, H)
	}
	var rows []dpathRow
	var p2 [][P2Width]field.Element
	var cur Cell
	started, fc := false, 0
	for hc := H; hc >= 0; hc-- {
		// the groups of height hc, in order
		for fc < G {
			pi := scalar(p.Order, fc)
			if pi >= G {
				return fmt.Errorf("MerklePathDynamic: order entry %d is group %d, of %d", fc, pi, G)
			}
			hg := scalar(p.Heights, pi)
			if hg > hc {
				return fmt.Errorf("MerklePathDynamic: group %d of height %d after height %d", pi, hg, hc)
			}
			if hg < hc {
				break
			}
			count(p.Order, fc)
			count(p.Heights, pi)
			count(p.Leaves, pi)
			row := dpathRow{fold: true, hc: hc, fc: fc, started: started, pi: pi, idx: index, cur: cur}
			row.op = m.entry(r, p.Leaves, pi)
			row.out = row.op
			if started {
				p2 = append(p2, p2Input(cur, row.op))
				row.out = compressCell(cur, row.op)
			}
			rows = append(rows, row)
			cur, started = row.out, true
			fc++
		}
		if hc == 0 {
			break
		}
		// the level hc → hc − 1
		count(p.Siblings, hc-1)
		row := dpathRow{hc: hc, fc: fc, started: started, b: index & 1, idx: index, cur: cur}
		row.op = m.entry(r, p.Siblings, hc-1)
		row.out = cur
		if started {
			left, right := cur, row.op
			if row.b == 1 {
				left, right = right, left
			}
			p2 = append(p2, p2Input(left, right))
			row.out = compressCell(left, right)
		}
		rows = append(rows, row)
		cur = row.out
		index >>= 1
	}
	if fc != G {
		return fmt.Errorf("MerklePathDynamic: %d of %d groups folded (heights in [0, %d], in order)", fc, G, H)
	}
	r.p2 = append(r.p2, p2...)
	if r.dpathRows == nil {
		r.dpathRows = make([][]dpathRow, len(m.dpaths))
	}
	r.dpathRows[i] = rows
	return nil
}

// dpathChip: one row per row of a dynamic path, H + G per path.
//
// Witness: kind, hc, started, fc, pi, b, idx, cur0..7, op0..7, out0..7 (see
// their names). Setup: active, first, last, the four tables' tags, addr_idx
// (first row), addr_root (last row).
//
// Constraints (transitions on every row of a path but its last):
//
//	kind, b ∈ {0, 1},  kind·b = 0
//	(1 − active)·kind = 0,  (1 − active)·started = 0    padding rows are inert
//	first: started = 0
//	hc[+1] = hc − (1 − kind),  fc[+1] = fc + kind
//	started[+1] = started + kind − started·kind,  cur[+1] = out
//	idx = b + (2 − kind)·idx[+1]           a level consumes a bit, a fold none
//	last: idx = b                          the index is below 2^H
//	(1 − started)·(out − cur − kind·(op − cur)) = 0
//	                                       before a leaf: copy; the first leaf
//
// and, once started, out = Compress(b ? (op, cur) : (cur, op)), looked up in
// the P2 core (a fold row has b = 0: its leaf on the right). Reads, on the
// tables bus: the leaf, entry pi of Leaves, pi, entry fc of Order, and hc,
// entry pi of Heights, so that a group folds at its height (fold rows); the
// sibling, entry hc − 1 of Siblings (level rows). On the memory bus: the index
// on the first row, the root, with out, on the last row.
//
// The path's ends follow from the tables' bounds: an index out of a table
// matches no entry. A fold reads Order at fc, which a fold increases by one,
// so at most G rows fold; a level reads Siblings at hc − 1, which a level
// decreases by one, so at most H rows are levels. A path has H + G rows: the
// folds read Order[0 … G − 1] once each, and the levels go from height H down
// to 1, the folds of height 0 after them. started is boolean (it starts at 0,
// and kind is boolean), and padding rows do no bus access at all. Every
// constraint has degree at most 3.
type dpathChip struct{ m *Machine }

func (c dpathChip) name() string { return dpathMod }

func (c dpathChip) rows() int { return c.m.numDPathRows() }

func (c dpathChip) setup(cs *cols) {
	cs.declare(dpathActive, dpathFirst, dpathLast, dpathLeafTag, dpathOrderTag, dpathHeightsTag, dpathSibTag,
		dpathAddrIdx, dpathAddrRoot)
	row := 0
	for _, p := range c.m.dpaths {
		n := p.Siblings.Len() + p.Order.Len()
		for k := range n {
			cs.set(dpathActive, row, 1)
			cs.set(dpathLeafTag, row, uint64(p.Leaves.tag))
			cs.set(dpathOrderTag, row, uint64(p.Order.tag))
			cs.set(dpathHeightsTag, row, uint64(p.Heights.tag))
			cs.set(dpathSibTag, row, uint64(p.Siblings.tag))
			if k == 0 {
				cs.set(dpathFirst, row, 1)
				cs.set(dpathAddrIdx, row, uint64(p.Index))
			}
			if k == n-1 {
				cs.set(dpathLast, row, 1)
				cs.set(dpathAddrRoot, row, uint64(p.Root))
			}
			row++
		}
	}
}

func (c dpathChip) define(m *module, bus *Bus) error {
	first, last, active := m.s(dpathFirst), m.s(dpathLast), m.s(dpathActive)
	kind, hc, started, fc := m.w(dpathKind), m.w(dpathHc), m.w(dpathStarted), m.w(dpathFc)
	pi, bit, idx := m.w(dpathPi), m.w(dpathB), m.w(dpathIdx)
	cur, op, out := m.lanes(dpathCur, CellWidth), m.lanes(dpathOp, CellWidth), m.lanes(dpathOut, CellWidth)
	one := constant(1)
	trans := wiop.Sub(active, last)

	for _, x := range []wiop.Expression{kind, bit} {
		m.assertZero(wiop.Mul(x, wiop.Sub(x, one)))
	}
	m.assertZero(wiop.Mul(kind, bit))
	// padding rows: no fold, no leaf, so no bus access
	m.assertZero(wiop.Mul(wiop.Sub(one, active), kind))
	m.assertZero(wiop.Mul(wiop.Sub(one, active), started))
	m.assertZero(wiop.Mul(first, started))
	m.assertZero(wiop.Mul(trans, wiop.Add(wiop.Sub(m.wShift(dpathHc, 1), hc), wiop.Sub(one, kind))))
	m.assertZero(wiop.Mul(trans, wiop.Sub(wiop.Sub(m.wShift(dpathFc, 1), fc), kind)))
	startedNext := wiop.Sub(wiop.Add(started, kind), wiop.Mul(started, kind))
	m.assertZero(wiop.Mul(trans, wiop.Sub(m.wShift(dpathStarted, 1), startedNext)))
	next := wiop.Mul(wiop.Mul(trans, wiop.Sub(constant(2), kind)), m.wShift(dpathIdx, 1))
	m.assertZero(wiop.Sub(wiop.Mul(active, wiop.Sub(idx, bit)), next))
	for i := range CellWidth {
		m.assertZero(wiop.Mul(trans, wiop.Sub(m.wShift(laneName(dpathCur, i), 1), out[i])))
		copied := wiop.Sub(wiop.Sub(out[i], cur[i]), wiop.Mul(kind, wiop.Sub(op[i], cur[i])))
		m.assertZero(wiop.Mul(wiop.Sub(one, started), copied))
	}

	bus.ReadEntry(m, m.s(dpathLeafTag), pi, cellLanes(op), kind)
	bus.ReadEntry(m, m.s(dpathOrderTag), fc, cellOf(pi), kind)
	bus.ReadEntry(m, m.s(dpathHeightsTag), pi, cellOf(hc), kind)
	bus.ReadEntry(m, m.s(dpathSibTag), wiop.Sub(hc, one), cellLanes(op), wiop.Sub(active, kind))
	bus.Read(m, m.s(dpathAddrIdx), cellOf(idx), first)
	bus.Read(m, m.s(dpathAddrRoot), cellLanes(out), last)

	// out = Compress(left, right), left = cur + b·(op − cur), right = op + b·(cur − op)
	var in [P2Width]wiop.Expression
	for i := range CellWidth {
		in[i] = wiop.Add(cur[i], wiop.Mul(bit, wiop.Sub(op[i], cur[i])))
		in[p2Half+i] = wiop.Add(op[i], wiop.Mul(bit, wiop.Sub(cur[i], op[i])))
	}
	bus.ReadP2(m, in, [p2Half]wiop.Expression(out), started)
	return nil
}

func (c dpathChip) trace(cs *cols, r *Run) error {
	cs.declare(dpathKind, dpathHc, dpathStarted, dpathFc, dpathPi, dpathB, dpathIdx)
	for _, x := range []string{dpathCur, dpathOp, dpathOut} {
		cs.declareLanes(x, CellWidth)
	}
	row := 0
	for i := range c.m.dpaths {
		for _, dr := range r.dpathRows[i] {
			if dr.fold {
				cs.set(dpathKind, row, 1)
			}
			if dr.started {
				cs.set(dpathStarted, row, 1)
			}
			cs.set(dpathHc, row, uint64(dr.hc))
			cs.set(dpathFc, row, uint64(dr.fc))
			cs.set(dpathPi, row, uint64(dr.pi))
			cs.set(dpathB, row, dr.b)
			cs.set(dpathIdx, row, dr.idx)
			cs.setCell(dpathCur, row, dr.cur)
			cs.setCell(dpathOp, row, dr.op)
			cs.setCell(dpathOut, row, dr.out)
			row++
		}
	}
	return nil
}
