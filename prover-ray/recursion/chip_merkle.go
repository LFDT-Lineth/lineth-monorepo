package recursion

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// Setup columns of the merkle chip.
const (
	merkleActive   = "active"
	merkleFirst    = "first"
	merkleLast     = "last"
	merkleInj      = "inj"  // an injection row
	merkleLvl0     = "lvl0" // the path's first level row
	merkleAddrLeaf = "addr_leaf"
	merkleAddrIdx  = "addr_idx"
	merkleAddrSib  = "addr_sib"
	merkleAddrRoot = "addr_root"
	merkleG        = "g" // the level's x⁻¹ factor, 1 on injection rows and in MerklePath
	merkleAddrXb   = "addr_xb"
	merkleMultXb   = "m_xb"
)

// Witness columns of the merkle chip.
const (
	merkleB   = "b"   // the direction bit
	merkleIdx = "idx" // the index left to consume
	merkleF   = "f"   // the x⁻¹ factor of the row, 1 + b·(g − 1)
	merkleX   = "x"   // the x⁻¹ accumulator
	merkleB0  = "b0"  // the index's low bit
	merkleCur = "cur" // lanes: the running hash entering the row
	merkleS   = "s"   // lanes: the sibling, or the injected digest
	merkleOut = "out" // lanes: the running hash leaving the row
)

// Injection is a digest injected into a Merkle path, as ray's mixed-height
// trees do (fri.Tree): after the compression that produces the path's node at
// the level of Width nodes, the running hash h becomes Compress(h, Digest).
// Several injections at one level are applied in order. A width equal to the
// number of leaves injects into the leaf itself.
type Injection struct {
	Width  int
	Digest int
}

type pathInst struct {
	leaf, index, root int
	steps             []pathStep
	gens              []field.Element // x⁻¹ factor of every level, nil for MerklePath
	xb                slot            // [x⁻¹, b0], for MerklePathX
}

// pathStep is one row of a path: a level (compress the running hash with the
// sibling, ordered by the index bit) or an injection (compress it with the
// injected digest, running hash on the left).
type pathStep struct {
	op  int  // the sibling or the injected digest
	lvl int  // the level, leaf level 0, for a level step
	inj bool // an injection step
}

// MerklePath checks that the digest at leaf is leaf number index (lane 0 of
// the index cell) of the tree whose root is at root: siblings are given leaf
// level first, injections in the tree's order (non-increasing widths, and at
// one width in injection order). It matches fri.Branch.AuthenticateToCap with
// an uncapped tree: at each level the running hash h becomes
// Compress(h, sibling), or Compress(sibling, h) when the index bit is 1, then
// Compress(h, d) for each digest d injected at that level.
func (m *Machine) MerklePath(leaf int, siblings []int, index int, root int, injections ...Injection) {
	m.merklePath(leaf, siblings, index, root, injections)
}

// MerklePathX is MerklePath for a tree whose leaf i is the value at position i
// of a codeword in bit-reversed order (a FRI layer) over a domain of generator
// g = gInv⁻¹. It also returns the cell [x⁻¹, b0]: x⁻¹ = gInv^bitrev(index), the
// bit reversal over the path's depth, is the inverse of the queried position's
// domain point, and b0 is the index's low bit.
//
// The path must go up to the tree's root: its depth is the tree's height. A
// path stopping at a Merkle cap's frontier would leave the frontier's index
// bits out of x⁻¹, and give the remaining ones the wrong exponents.
func (m *Machine) MerklePathX(
	leaf int, siblings []int, index int, root int, gInv field.Element, injections ...Injection,
) int {
	p := m.merklePath(leaf, siblings, index, root, injections)
	d := len(siblings)
	p.gens = make([]field.Element, d)
	for lvl := range d {
		g := gInv
		for range d - 1 - lvl {
			g.Square(&g)
		}
		p.gens[lvl] = g
	}
	p.xb = m.wr()
	return p.xb.addr
}

func (m *Machine) merklePath(leaf int, siblings []int, index int, root int, injections []Injection) *pathInst {
	steps, err := pathSteps(siblings, injections)
	if err != nil {
		panic(fmt.Sprintf("MerklePath: %v", err))
	}
	// idx = Σ b_i·2^i is injective only below the field's modulus: at most
	// field.Bits − 1 levels
	if d := len(siblings); d == 0 || d >= field.Bits {
		panic(fmt.Sprintf("MerklePath: a path of depth %d, want 1 to %d", d, field.Bits-1))
	}
	m.read(leaf)
	m.read(index)
	m.read(root)
	for _, st := range steps {
		m.read(st.op)
	}
	m.paths = append(m.paths, pathInst{leaf: leaf, index: index, root: root, steps: steps})
	m.record(instrPath, len(m.paths)-1)
	return &m.paths[len(m.paths)-1]
}

// pathSteps orders the rows of a path: the leaf-level injections, then every
// level followed by the injections at the width of the node it produces.
func pathSteps(siblings []int, injections []Injection) ([]pathStep, error) {
	d := len(siblings)
	prev := 1 << d
	for k, inj := range injections {
		w := inj.Width
		if w <= 0 || w&(w-1) != 0 || w > prev {
			return nil, fmt.Errorf("injection %d: width %d after %d "+
				"(widths must be non-increasing powers of two, at most 2^depth)", k, w, prev)
		}
		prev = w
	}
	var steps []pathStep
	next := 0
	inject := func(width int) {
		for ; next < len(injections) && injections[next].Width == width; next++ {
			steps = append(steps, pathStep{op: injections[next].Digest, inj: true})
		}
	}
	inject(1 << d)
	for k, s := range siblings {
		steps = append(steps, pathStep{op: s, lvl: k})
		inject(1 << (d - k - 1))
	}
	if next != len(injections) {
		return nil, fmt.Errorf("injection of width %d does not fit a path of depth %d", injections[next].Width, d)
	}
	return steps, nil
}

// numPathSteps is the number of rows the paths take: the merkle chip's rows,
// and their compressions in the P2 core.
func (m *Machine) numPathSteps() int {
	n := 0
	for _, p := range m.paths {
		n += len(p.steps)
	}
	return n
}

// pathRow is one row of a path, as executed.
type pathRow struct {
	cur, op, out Cell
	b, idx       uint64
	x            field.Element // x⁻¹ after the row
}

// walk runs a path from its leaf: the rows, and the running hash it ends with.
func (p pathInst) walk(r *Run) []pathRow {
	cur := r.values[p.leaf]
	index := r.values[p.index][0].Uint64()
	x := field.One()
	rows := make([]pathRow, len(p.steps))
	for k, st := range p.steps {
		op := r.values[st.op]
		bit := index & 1
		if st.inj {
			bit = 0
		}
		if bit == 1 && p.gens != nil {
			x.Mul(&x, &p.gens[st.lvl])
		}
		left, right := cur, op
		if bit == 1 {
			left, right = op, cur
		}
		out := compressCell(left, right)
		rows[k] = pathRow{cur: cur, op: op, out: out, b: bit, idx: index, x: x}
		cur = out
		if !st.inj {
			index >>= 1
		}
	}
	return rows
}

// execPath records the path's compressions for the P2 core and computes its
// [x⁻¹, b0] cell. It fails if the index does not fit the path's depth or the
// path does not reach the root, which no trace could prove.
func (m *Machine) execPath(p pathInst, r *Run) error {
	depth := 0
	for _, st := range p.steps {
		if !st.inj {
			depth++
		}
	}
	index := r.values[p.index][0].Uint64()
	if index>>depth != 0 {
		return fmt.Errorf("MerklePath: index %d does not fit a path of depth %d", index, depth)
	}
	rows := p.walk(r)
	for _, row := range rows {
		left, right := row.cur, row.op
		if row.b == 1 {
			left, right = row.op, row.cur
		}
		r.p2 = append(r.p2, p2Input(left, right))
	}
	if rows[len(rows)-1].out != r.values[p.root] {
		return fmt.Errorf("MerklePath: the path of leaf %d does not reach the root", index)
	}
	if p.gens != nil {
		var xb Cell
		xb[0] = rows[len(rows)-1].x
		xb[1].SetUint64(index & 1)
		r.values[p.xb.addr] = xb
	}
	return nil
}

// merkleChip: one row per step of a Merkle path (see pathSteps), leaf level
// first.
//
// Witness: cur0..7 (the running hash entering the row), s0..7 (the sibling
// or the injected digest), b (the direction bit), idx (the index left to
// consume), out0..7, f (the row's x⁻¹ factor), x (the x⁻¹ accumulator), b0
// (the index's low bit). Setup: active, first, last, inj, lvl0, addr_leaf,
// addr_idx (read on the first row), addr_sib, addr_root (read on the last
// row, with out), g (the level's x⁻¹ factor), addr_xb, m_xb ([x⁻¹, b0],
// written on the last row of a MerklePathX).
//
//	b·(b − 1) = 0,  inj·b = 0
//	idx = b + (2 − inj)·idx[+1]   on every row of a path but its last: a level
//	                              consumes a bit, an injection none
//	idx = b                       on the last row (the index is below 2^depth)
//	cur = out[−1]                 on every row of a path but its first
//	f = 1 + b·(g − 1)
//	x = f on the first row, x = x[−1]·f on the others
//	b0 = b on the first level row, b0 = b0[−1] on the following rows
//
// and out = Compress(b ? (s, cur) : (cur, s)), looked up in the P2 core. The
// selectors (active, first, last, lvl0) are zero on padding rows, so every
// transition is confined to its path: none reads across a path end or the
// wrap-around. Every constraint has degree at most 3.
type merkleChip struct{ m *Machine }

func (c merkleChip) name() string { return merkleMod }

func (c merkleChip) rows() int { return c.m.numPathSteps() }

func (c merkleChip) setup(cs *cols) {
	cs.declare(merkleActive, merkleFirst, merkleLast, merkleInj, merkleLvl0, merkleAddrLeaf, merkleAddrIdx,
		merkleAddrSib, merkleAddrRoot, merkleG, merkleAddrXb, merkleMultXb)
	row := 0
	for _, p := range c.m.paths {
		for k, st := range p.steps {
			cs.set(merkleActive, row, 1)
			cs.set(merkleAddrSib, row, uint64(st.op))
			if k == 0 {
				cs.set(merkleFirst, row, 1)
				cs.set(merkleAddrLeaf, row, uint64(p.leaf))
				cs.set(merkleAddrIdx, row, uint64(p.index))
			}
			g := field.One()
			if st.inj {
				cs.set(merkleInj, row, 1)
			} else {
				if st.lvl == 0 {
					cs.set(merkleLvl0, row, 1)
				}
				if p.gens != nil {
					g = p.gens[st.lvl]
				}
			}
			cs.setElem(merkleG, row, g)
			if k == len(p.steps)-1 {
				cs.set(merkleLast, row, 1)
				cs.set(merkleAddrRoot, row, uint64(p.root))
				if p.gens != nil {
					cs.set(merkleAddrXb, row, uint64(p.xb.addr))
					cs.setInt(merkleMultXb, row, c.m.mult(p.xb))
				}
			}
			row++
		}
	}
}

func (c merkleChip) define(m *module, bus *Bus) error {
	active, first, last := m.s(merkleActive), m.s(merkleFirst), m.s(merkleLast)
	inj, lvl0, g := m.s(merkleInj), m.s(merkleLvl0), m.s(merkleG)
	bit, idx, f, x, b0 := m.w(merkleB), m.w(merkleIdx), m.w(merkleF), m.w(merkleX), m.w(merkleB0)
	cur, s, out := m.lanes(merkleCur, CellWidth), m.lanes(merkleS, CellWidth), m.lanes(merkleOut, CellWidth)
	notFirst := wiop.Sub(active, first)

	m.assertZero(wiop.Mul(bit, wiop.Sub(bit, constant(1))))
	m.assertZero(wiop.Mul(inj, bit))
	// (active − last) is 1 on every path row but the last, 0 elsewhere
	next := wiop.Mul(wiop.Mul(wiop.Sub(active, last), wiop.Sub(constant(2), inj)), m.wShift(merkleIdx, 1))
	m.assertZero(wiop.Sub(wiop.Sub(idx, bit), next))
	for i := range CellWidth {
		m.assertZero(wiop.Mul(notFirst, wiop.Sub(cur[i], m.wShift(laneName(merkleOut, i), -1))))
	}
	// x⁻¹ = Π_lvl (b_lvl ? g_lvl : 1)
	m.assertZero(wiop.Sub(f, wiop.Add(constant(1), wiop.Mul(bit, wiop.Sub(g, constant(1))))))
	m.assertZero(wiop.Mul(first, wiop.Sub(x, f)))
	m.assertZero(wiop.Mul(notFirst, wiop.Sub(x, wiop.Mul(m.wShift(merkleX, -1), f))))
	m.assertZero(wiop.Mul(lvl0, wiop.Sub(b0, bit)))
	m.assertZero(wiop.Mul(wiop.Mul(notFirst, wiop.Sub(constant(1), lvl0)), wiop.Sub(b0, m.wShift(merkleB0, -1))))

	bus.Read(m, m.s(merkleAddrLeaf), cellLanes(cur), first)
	bus.Read(m, m.s(merkleAddrIdx), cellOf(idx), first)
	bus.Read(m, m.s(merkleAddrSib), cellLanes(s), active)
	bus.Read(m, m.s(merkleAddrRoot), cellLanes(out), last)
	bus.Write(m, m.s(merkleAddrXb), cellOf(x, b0), m.s(merkleMultXb))

	// out = Compress(left, right), left = cur + b·(s − cur), right = s + b·(cur − s)
	var in [P2Width]wiop.Expression
	for i := range CellWidth {
		in[i] = wiop.Add(cur[i], wiop.Mul(bit, wiop.Sub(s[i], cur[i])))
		in[p2Half+i] = wiop.Add(s[i], wiop.Mul(bit, wiop.Sub(cur[i], s[i])))
	}
	bus.ReadP2(m, in, [p2Half]wiop.Expression(out), active)
	return nil
}

func (c merkleChip) trace(cs *cols, r *Run) error {
	cs.declare(merkleB, merkleIdx, merkleF, merkleX, merkleB0)
	for _, x := range []string{merkleCur, merkleS, merkleOut} {
		cs.declareLanes(x, CellWidth)
	}
	row := 0
	for _, p := range c.m.paths {
		b0 := r.values[p.index][0].Uint64() & 1
		for k, pr := range p.walk(r) {
			st := p.steps[k]
			fv := field.One()
			if pr.b == 1 && !st.inj && p.gens != nil {
				fv = p.gens[st.lvl]
			}
			cs.set(merkleB, row, pr.b)
			cs.set(merkleIdx, row, pr.idx)
			cs.setElem(merkleF, row, fv)
			cs.setElem(merkleX, row, pr.x)
			cs.set(merkleB0, row, b0)
			cs.setCell(merkleCur, row, pr.cur)
			cs.setCell(merkleS, row, pr.op)
			cs.setCell(merkleOut, row, pr.out)
			row++
		}
	}
	// padding rows: b = 0, so f = 1 (g is 0 there)
	for ; row < cs.n; row++ {
		cs.set(merkleF, row, 1)
	}
	return nil
}
