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
	dpathHMax       = "hmax"     // H, on a path's first row
	dpathGroups     = "groups"   // G, on a path's last row
	dpathLeafTag    = "leaf_tag" // the tables' tags
	dpathOrderTag   = "order_tag"
	dpathHeightsTag = "heights_tag"
	dpathSibTag     = "sib_tag"
	dpathBitsTag    = "bits_tag"
	dpathL          = "l" // L, the number of bits of the queried position
	dpathAddrRoot   = "addr_root"
)

// Witness columns of the dpath chip.
const (
	dpathKind    = "kind"    // 1 on a fold row, 0 on a level row
	dpathHc      = "hc"      // the height before the row
	dpathStarted = "started" // a leaf has been taken before the row
	dpathFc      = "fc"      // the number of fold rows before the row
	dpathPi      = "pi"      // the group folded (fold rows)
	dpathHg      = "hg"      // its height (fold rows)
	dpathB       = "b"       // the direction bit (level rows)
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
// leaf); then it compresses the level with its sibling, ordered by bit L − h
// of the queried position (h → h − 1). The rows above the tree's actual
// height, before the first fold, copy. This is fri.Tree's hashing: a leaf
// chains its size's groups, and a node chains the groups injected at its
// height after compressing its children.
type DynamicPath struct {
	// Leaves[g] is group g's leaf digest at the queried position; Heights[g]
	// its height, a scalar in [1, H].
	Leaves, Heights Table
	// Order[k] is the k-th group folded: the groups by decreasing height,
	// equal heights by increasing index, which is the order fri.Tree chains
	// them in (the groups of a size in module order). The chip relies on it:
	// the caller checks that it is that permutation. An index past G reads no
	// entry of Leaves and Heights, so it cannot verify, but a repeated group
	// could.
	Order Table
	// Siblings[h−1] is the sibling at height h (h = 1..H), anything above the
	// tree's actual height; H = Siblings.Len().
	Siblings Table
	// Bits[j] is bit j of the queried position, j < L; the direction bit at
	// height h is Bits[L − h].
	Bits Table
	L    int
	Root int
}

type dpathInst struct {
	p DynamicPath
}

// dpathRow is one executed row of a dynamic path.
type dpathRow struct {
	fold         bool
	hc, fc       int
	started      bool
	pi, hg       int
	b            uint64
	cur, op, out Cell
}

// MerklePathDynamic checks a DynamicPath: that the path from the groups'
// leaves, at their heights, through the siblings, reaches the root. It takes
// one row per height (H) and per group (G), whatever the heights.
func (m *Machine) MerklePathDynamic(p DynamicPath) {
	H, G := p.Siblings.Len(), p.Order.Len()
	if p.Leaves.Len() != G || p.Heights.Len() != G {
		panic(fmt.Sprintf("MerklePathDynamic: %d leaves, %d heights, %d groups ordered", p.Leaves.Len(), p.Heights.Len(), G))
	}
	if p.L < H || p.Bits.Len() < p.L {
		panic(fmt.Sprintf("MerklePathDynamic: %d bits for a position of %d bits and %d heights", p.Bits.Len(), p.L, H))
	}
	for _, t := range []Table{p.Leaves, p.Heights, p.Order, p.Siblings, p.Bits} {
		m.checkTable(t)
	}
	m.read(p.Root)
	m.dpaths = append(m.dpaths, dpathInst{p: p})
	m.record(instrDPath, len(m.dpaths)-1)
}

// numDPathRows is the number of rows the dynamic paths take, H + G each: the
// dpath chip's rows, and an upper bound on their compressions.
func (m *Machine) numDPathRows() int {
	n := 0
	for _, d := range m.dpaths {
		n += d.p.Siblings.Len() + d.p.Order.Len()
	}
	return n
}

// entry returns the value of entry i of the table t.
func (m *Machine) entry(r *Run, t Table, i int) Cell { return r.values[m.tables[t.tag].src[i]] }

// execDPath builds the path's rows from the tables' values (the heights and
// the order decide which rows fold), counts its reads of the tables' entries,
// and records its compressions for the P2 core. It fails if the heights and
// the order do not fold every group, in non-increasing heights within [1, H]:
// no rows follow from them. A path that misses the root executes, and its
// proof does not verify (the last row reads the root), so that tests can
// check the circuit rejects it.
func (m *Machine) execDPath(idx int, r *Run) error {
	p := m.dpaths[idx].p
	H, G := p.Siblings.Len(), p.Order.Len()
	scalar := func(t Table, i int) int {
		e := m.entry(r, t, i)
		return int(e[0].Uint64())
	}
	count := func(t Table, i int) {
		if r.tableReads == nil {
			r.tableReads = map[int][]uint64{}
		}
		if r.tableReads[t.tag] == nil {
			r.tableReads[t.tag] = make([]uint64, t.n)
		}
		r.tableReads[t.tag][i]++
	}
	var rows []dpathRow
	var p2 [][P2Width]field.Element
	var cur Cell
	started, fc := false, 0
	for hc := H; hc >= 1; hc-- {
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
			row := dpathRow{fold: true, hc: hc, fc: fc, started: started, pi: pi, hg: hg, cur: cur, op: m.entry(r, p.Leaves, pi)}
			row.out = row.op
			if started {
				p2 = append(p2, p2Input(cur, row.op))
				row.out = compressCell(cur, row.op)
			}
			rows = append(rows, row)
			cur, started = row.out, true
			fc++
		}
		// the level hc → hc − 1
		count(p.Siblings, hc-1)
		count(p.Bits, p.L-hc)
		bc := m.entry(r, p.Bits, p.L-hc)
		b := bc[0].Uint64()
		row := dpathRow{hc: hc, fc: fc, started: started, b: b, cur: cur, op: m.entry(r, p.Siblings, hc-1)}
		row.out = cur
		if started {
			left, right := cur, row.op
			if b == 1 {
				left, right = right, left
			}
			p2 = append(p2, p2Input(left, right))
			row.out = compressCell(left, right)
		}
		rows = append(rows, row)
		cur = row.out
	}
	if fc != G {
		return fmt.Errorf("MerklePathDynamic: %d of %d groups folded (heights in [1, %d], in order)", fc, G, H)
	}
	r.p2 = append(r.p2, p2...)
	if r.dpathRows == nil {
		r.dpathRows = map[int][]dpathRow{}
	}
	r.dpathRows[idx] = rows
	return nil
}

// dpathChip: one row per row of a dynamic path, H + G per path.
//
// Witness: kind, hc, started, fc, pi, hg, b, cur0..7, op0..7, out0..7 (see
// their names). Setup: active, first, last, hmax (H, first row), groups (G,
// last row), the five tables' tags, l (L), addr_root (last row).
//
// Constraints (transitions on every row of a path but its last):
//
//	kind, b, started ∈ {0, 1},  kind·b = 0
//	first: hc = H, started = 0, fc = 0
//	hc[+1] = hc − (1 − kind),  fc[+1] = fc + kind
//	started[+1] = started + kind − started·kind,  cur[+1] = out
//	kind·(hg − hc) = 0                      a group folds at its height
//	last: kind = 0, hc = 1, fc = G          every level, every group
//	(1 − started)·(out − cur − kind·(op − cur)) = 0
//	                                        before a leaf: copy; the first leaf
//
// and, once started, out = Compress(b ? (op, cur) : (cur, op)), looked up in
// the P2 core (a fold row has b = 0: its leaf on the right). Reads, on the
// tables bus: the leaf, entry pi of Leaves, pi, entry fc of Order, and hg,
// entry pi of Heights (fold rows); the sibling, entry hc − 1 of Siblings, and
// b, entry L − hc of Bits (level rows). On the memory bus: the root, with
// out, on the last row. Each table read is tagged by its table, so an index
// out of a table matches no entry: these reads cannot reach another table's
// entries whatever the indices. That the order is the permutation folding
// each group once remains the caller's check. Every constraint has degree at
// most 3.
//
// The padding rows have no setup selector: kind and started are witness
// selectors there too. Their tags match no table, so their table reads cannot
// reach a table, and started ∈ {0, 1} keeps their P2 read from becoming a
// write of a compression that is not one.
type dpathChip struct{ m *Machine }

func (c dpathChip) name() string { return dpathMod }

func (c dpathChip) rows() int { return c.m.numDPathRows() }

func (c dpathChip) setup(cs *cols) {
	cs.declare(dpathActive, dpathFirst, dpathLast, dpathHMax, dpathGroups, dpathLeafTag, dpathOrderTag,
		dpathHeightsTag, dpathSibTag, dpathBitsTag, dpathL, dpathAddrRoot)
	row := 0
	for _, d := range c.m.dpaths {
		p := d.p
		n := p.Siblings.Len() + p.Order.Len()
		for k := range n {
			cs.set(dpathActive, row, 1)
			cs.set(dpathLeafTag, row, uint64(p.Leaves.tag))
			cs.set(dpathOrderTag, row, uint64(p.Order.tag))
			cs.set(dpathHeightsTag, row, uint64(p.Heights.tag))
			cs.set(dpathSibTag, row, uint64(p.Siblings.tag))
			cs.set(dpathBitsTag, row, uint64(p.Bits.tag))
			cs.set(dpathL, row, uint64(p.L))
			if k == 0 {
				cs.set(dpathFirst, row, 1)
				cs.set(dpathHMax, row, uint64(p.Siblings.Len()))
			}
			if k == n-1 {
				cs.set(dpathLast, row, 1)
				cs.set(dpathGroups, row, uint64(p.Order.Len()))
				cs.set(dpathAddrRoot, row, uint64(p.Root))
			}
			row++
		}
	}
	// The padding rows read with a tag that matches no table. Their selectors
	// are witness columns (kind, and active − kind is −1 where kind = 1), so
	// with the zero tags of the other setup columns they would read, or write,
	// the entries of the program's first table: a padding row could inject a
	// fake entry that a Lookup reads. With no table of their tag, what they
	// read or write balances only against other padding rows.
	none := uint64(len(c.m.tables))
	for ; row < cs.n; row++ {
		for _, tag := range []string{dpathLeafTag, dpathOrderTag, dpathHeightsTag, dpathSibTag, dpathBitsTag} {
			cs.set(tag, row, none)
		}
	}
}

func (c dpathChip) define(m *module, bus *Bus) error {
	first, last, active := m.s(dpathFirst), m.s(dpathLast), m.s(dpathActive)
	kind, hc, started, fc := m.w(dpathKind), m.w(dpathHc), m.w(dpathStarted), m.w(dpathFc)
	pi, hg, bit := m.w(dpathPi), m.w(dpathHg), m.w(dpathB)
	cur, op, out := m.lanes(dpathCur, CellWidth), m.lanes(dpathOp, CellWidth), m.lanes(dpathOut, CellWidth)
	one := constant(1)
	notKind := wiop.Sub(one, kind)
	trans := wiop.Sub(active, last)

	for _, x := range []wiop.Expression{kind, bit, started} {
		m.assertZero(wiop.Mul(x, wiop.Sub(x, one)))
	}
	m.assertZero(wiop.Mul(kind, bit))
	// The path's boundary. Each of these five follows from the others: rows
	// go hc − 1 per level and fc + 1 per fold over H + G rows, so any three of
	// hc = H, fc = 0 (first) and hc = 1, fc = G (last) give the fourth, and a
	// fold on the last row would read Order[G], which matches no entry. They
	// state the path's ends explicitly.
	m.assertZero(wiop.Mul(first, wiop.Sub(hc, m.s(dpathHMax))))
	m.assertZero(wiop.Mul(first, started))
	m.assertZero(wiop.Mul(first, fc))
	m.assertZero(wiop.Mul(trans, wiop.Add(wiop.Sub(m.wShift(dpathHc, 1), hc), notKind)))
	m.assertZero(wiop.Mul(trans, wiop.Sub(wiop.Sub(m.wShift(dpathFc, 1), fc), kind)))
	startedNext := wiop.Sub(wiop.Add(started, kind), wiop.Mul(started, kind))
	m.assertZero(wiop.Mul(trans, wiop.Sub(m.wShift(dpathStarted, 1), startedNext)))
	m.assertZero(wiop.Mul(kind, wiop.Sub(hg, hc)))
	m.assertZero(wiop.Mul(last, kind))
	m.assertZero(wiop.Mul(last, wiop.Sub(hc, one)))
	m.assertZero(wiop.Mul(last, wiop.Sub(fc, m.s(dpathGroups))))
	for i := range CellWidth {
		m.assertZero(wiop.Mul(trans, wiop.Sub(m.wShift(laneName(dpathCur, i), 1), out[i])))
		copied := wiop.Sub(wiop.Sub(out[i], cur[i]), wiop.Mul(kind, wiop.Sub(op[i], cur[i])))
		m.assertZero(wiop.Mul(wiop.Sub(one, started), copied))
	}

	bus.ReadEntry(m, m.s(dpathLeafTag), pi, cellLanes(op), kind)
	bus.ReadEntry(m, m.s(dpathSibTag), wiop.Sub(hc, one), cellLanes(op), wiop.Sub(active, kind))
	bus.ReadEntry(m, m.s(dpathOrderTag), fc, cellOf(pi), kind)
	bus.ReadEntry(m, m.s(dpathHeightsTag), pi, cellOf(hg), kind)
	bus.ReadEntry(m, m.s(dpathBitsTag), wiop.Sub(m.s(dpathL), hc), cellOf(bit), wiop.Sub(active, kind))
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
	cs.declare(dpathKind, dpathHc, dpathStarted, dpathFc, dpathPi, dpathHg, dpathB)
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
			cs.set(dpathHg, row, uint64(dr.hg))
			cs.set(dpathB, row, dr.b)
			cs.setCell(dpathCur, row, dr.cur)
			cs.setCell(dpathOp, row, dr.op)
			cs.setCell(dpathOut, row, dr.out)
			row++
		}
	}
	return nil
}
