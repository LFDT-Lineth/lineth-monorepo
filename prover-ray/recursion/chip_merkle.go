package recursion

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// Setup columns of the merkle chip.
const (
	merkleFirst    = "first"
	merkleLast     = "last"
	merkleActive   = "active"
	merkleInj      = "inj"
	merkleLvl0     = "lvl0"
	merkleLeafAddr = "leaf_addr"
	merkleIdxAddr  = "idx_addr"
	merkleSibAddr  = "sib_addr"
	merkleRootAddr = "root_addr"
	merkleG        = "g"
	merkleXbAddr   = "xb_addr"
	merkleXbMult   = "xb_mult"
)

// Witness columns of the merkle chip.
const (
	merkleB   = "b"
	merkleIdx = "idx"
	merkleX   = "x"
	merkleB0  = "b0"
	merkleCur = "cur" // lanes
	merkleS   = "s"   // lanes
	merkleOut = "out" // lanes
)

type pathInst struct {
	leaf, index, root int
	steps             []pathStep
	gens              []field.Element // x⁻¹ accumulator constants per level, nil if unused
	xb                slot            // the cell [x⁻¹, b0], if gens != nil
}

// pathStep is one row of a path: a level (compress the running hash with the
// sibling, ordered by the index bit) or an injection (compress it with the
// injection leaf, running hash on the left).
type pathStep struct {
	sib int  // the sibling or injection leaf
	lvl int  // the level, for a level step
	inj bool // an injection step
}

// Injection is a digest folded into a Merkle path after the level whose
// width (number of nodes) is Width: the running hash h becomes
// Compress(h, Leaf). In ray's mixed-height trees (fri.Tree), a node with an
// auxiliary digest is Compress(Compress(left, right), aux): the auxiliary
// digest is an injection at the node's level, whose width is the number of
// nodes at that depth. A width equal to the number of leaves folds it into
// the leaf itself.
type Injection struct {
	Width int
	Leaf  int
}

// MerklePath checks that the digest at leaf is the leaf of index (lane 0 of
// the index cell, below 2^depth) under the digest at root, with siblings
// leaf-level first, and the injections of a mixed-height tree (in schedule
// order: non-increasing widths). The root may be any digest cell: a tree's
// root, or a node of its cap (see fri.MerkleCap), looked up by the caller.
func (m *Machine) MerklePath(leaf int, siblings []int, index int, root int, injections ...Injection) {
	steps, err := pathSteps(siblings, injections)
	if err != nil {
		panic(fmt.Sprintf("MerklePath: %v", err))
	}
	if len(steps) == 0 {
		panic("MerklePath: no level and no injection")
	}
	m.read(leaf)
	m.read(index)
	m.read(root)
	for _, st := range steps {
		m.read(st.sib)
	}
	m.paths = append(m.paths, pathInst{leaf: leaf, index: index, root: root, steps: steps})
	m.record(instrPath, len(m.paths)-1)
}

// pathSteps orders the rows of a path: the leaf-level injections, then every
// level followed by the injections at the width above it.
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
	fold := func(width int) {
		for ; next < len(injections) && injections[next].Width == width; next++ {
			steps = append(steps, pathStep{sib: injections[next].Leaf, inj: true})
		}
	}
	fold(1 << d)
	for k, s := range siblings {
		steps = append(steps, pathStep{sib: s, lvl: k})
		fold(1 << (d - k - 1))
	}
	if next != len(injections) {
		return nil, fmt.Errorf("injection of width %d does not fit a path of depth %d", injections[next].Width, d)
	}
	return steps, nil
}

// MerklePathX is MerklePath for the Merkle tree of a FRI layer over a domain
// of generator g⁻¹ = gInv: it also returns the cell [x⁻¹, b0], where x⁻¹ =
// gInv^bitrev(index) (bit-reversed over the path depth) is the inverse of the
// fold point of the queried pair, and b0 is the index's low bit.
func (m *Machine) MerklePathX(leaf int, siblings []int, index, root int, gInv field.Element,
	injections ...Injection) int {
	m.MerklePath(leaf, siblings, index, root, injections...)
	p := &m.paths[len(m.paths)-1]
	d := len(siblings)
	for lvl := range d {
		g := gInv
		for range d - 1 - lvl {
			g.Square(&g)
		}
		p.gens = append(p.gens, g)
	}
	p.xb = m.wr()
	return p.xb.addr
}

// pathBit is the direction bit of a path step: the index's low bit on a
// level, 0 on an injection.
func pathBit(st pathStep, index uint64) uint64 {
	if st.inj {
		return 0
	}
	return index & 1
}

// execPath records the path's compressions for the P2 core, and computes the
// [x⁻¹, b0] cell of a MerklePathX.
func (m *Machine) execPath(p pathInst, r *Run) {
	cur, index := r.values[p.leaf], r.values[p.index][0].Uint64()
	for _, st := range p.steps {
		left, right := cur, r.values[st.sib]
		if pathBit(st, index) == 1 {
			left, right = right, left
		}
		r.p2 = append(r.p2, p2Input(left, right))
		cur = compressCell(left, right)
		if !st.inj {
			index >>= 1
		}
	}
	if p.gens == nil {
		return
	}
	idx := r.values[p.index][0].Uint64()
	var xb Cell
	xb[0].SetOne()
	for lvl, g := range p.gens {
		if idx>>lvl&1 == 1 {
			xb[0].Mul(&xb[0], &g)
		}
	}
	xb[1].SetUint64(idx & 1)
	r.values[p.xb.addr] = xb
}

// merkleChip: one row per step of a Merkle path (see pathSteps): a level,
// leaf level first, or an injection.
//
// Witness: cur0..7 (the running hash entering the row), s0..7 (the sibling,
// or the injection leaf), b (the direction bit), idx (the index left to
// consume), out0..7, x (the x⁻¹ accumulator), b0 (the index's low bit).
// Setup: first, last, active, inj (an injection row), lvl0 (the first level
// row), leaf_addr, idx_addr, sib_addr, root_addr, g (the accumulator constant
// of the level, 1 on injection rows), xb_addr and xb_mult (the cell [x⁻¹, b0],
// written on the last row).
//
//	b·(b − 1) = 0,  inj·b = 0
//	idx = b + (active − last)·(2 − inj)·idx[+1]   a level consumes a bit, an
//	                             injection none; on the last row idx = b (the
//	                             index is below 2^depth)
//	cur = out[−1]                on every row but the first of a path
//	x = 1 + b·(g − 1)            on the first row
//	x = x[−1]·(1 + b·(g − 1))    on the others
//	b0 = b on the first level row, b0 = b0[−1] on the following rows
//
// and out = Compress(b ? (s, cur) : (cur, s)), looked up in the P2 core. The
// first row reads the leaf digest and the index (a scalar); the last row
// reads the root with its own output. (active − first) and (active − last)
// are 0 at a path's ends and on padding rows, so no recurrence reads across a
// path end or the wrap-around.
type merkleChip struct{ m *Machine }

func (c merkleChip) name() string { return merkleMod }

func (c merkleChip) rows() int {
	n := 0
	for _, p := range c.m.paths {
		n += len(p.steps)
	}
	return n
}

func (c merkleChip) setup(cs *cols) {
	cs.declare(merkleFirst, merkleLast, merkleActive, merkleInj, merkleLvl0, merkleLeafAddr,
		merkleIdxAddr, merkleSibAddr, merkleRootAddr, merkleG, merkleXbAddr, merkleXbMult)
	row := 0
	for _, p := range c.m.paths {
		for k, st := range p.steps {
			cs.set(merkleActive, row, 1)
			cs.set(merkleSibAddr, row, uint64(st.sib))
			if k == 0 {
				cs.set(merkleFirst, row, 1)
				cs.set(merkleLeafAddr, row, uint64(p.leaf))
				cs.set(merkleIdxAddr, row, uint64(p.index))
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
				cs.set(merkleRootAddr, row, uint64(p.root))
				if p.gens != nil {
					cs.set(merkleXbAddr, row, uint64(p.xb.addr))
					cs.setInt(merkleXbMult, row, c.m.mult(p.xb))
				}
			}
			row++
		}
	}
}

func (c merkleChip) define(m *module, bus *Bus) error {
	first, last, active := m.s(merkleFirst), m.s(merkleLast), m.s(merkleActive)
	inj, lvl0 := m.s(merkleInj), m.s(merkleLvl0)
	bit, idx := m.w(merkleB), m.w(merkleIdx)
	notFirst, notLast := wiop.Sub(active, first), wiop.Sub(active, last)

	m.assertZero(wiop.Mul(bit, wiop.Sub(bit, constant(1))))
	m.assertZero(wiop.Mul(inj, bit))
	m.assertZero(wiop.Sub(wiop.Sub(idx, bit), prod(notLast, wiop.Sub(constant(2), inj), m.wShift(merkleIdx, 1))))
	cur, s, out := m.lanes(merkleCur, CellWidth), m.lanes(merkleS, CellWidth), m.lanes(merkleOut, CellWidth)
	for i := range CellWidth {
		m.assertZero(wiop.Mul(notFirst, wiop.Sub(cur[i], m.wShift(laneName(merkleOut, i), -1))))
	}
	// x⁻¹ = Π_lvl (b_lvl ? g_lvl : 1), with g_lvl = gInv^(2^(depth−1−lvl)).
	x, b0 := m.w(merkleX), m.w(merkleB0)
	factor := wiop.Add(constant(1), wiop.Mul(bit, wiop.Sub(m.s(merkleG), constant(1))))
	m.assertZero(wiop.Mul(first, wiop.Sub(x, factor)))
	m.assertZero(wiop.Mul(notFirst, wiop.Sub(x, wiop.Mul(m.wShift(merkleX, -1), factor))))
	m.assertZero(wiop.Mul(lvl0, wiop.Sub(b0, bit)))
	m.assertZero(prod(notFirst, wiop.Sub(constant(1), lvl0), wiop.Sub(b0, m.wShift(merkleB0, -1))))

	bus.Read(m, m.s(merkleLeafAddr), cellLanes(cur), first)
	bus.Read(m, m.s(merkleIdxAddr), cellOf(idx), first)
	bus.Read(m, m.s(merkleSibAddr), cellLanes(s), active)
	bus.Read(m, m.s(merkleRootAddr), cellLanes(out), last)
	bus.Write(m, m.s(merkleXbAddr), cellOf(x, b0), m.s(merkleXbMult))

	// the compression of (left, right), ordered by the bit
	var in [P2Width]wiop.Expression
	for i := range p2Half {
		in[i] = wiop.Add(cur[i], wiop.Mul(bit, wiop.Sub(s[i], cur[i])))
		in[p2Half+i] = wiop.Add(s[i], wiop.Mul(bit, wiop.Sub(cur[i], s[i])))
	}
	bus.ReadP2(m, in, [p2Half]wiop.Expression(out), active)
	return nil
}

func (c merkleChip) trace(cs *cols, r *Run) error {
	cs.declare(merkleB, merkleIdx, merkleX, merkleB0)
	for _, x := range []string{merkleCur, merkleS, merkleOut} {
		cs.declareLanes(x, CellWidth)
	}
	row := 0
	for _, p := range c.m.paths {
		cur := r.values[p.leaf]
		index := r.values[p.index][0].Uint64()
		b0 := index & 1
		x := field.One()
		for _, st := range p.steps {
			sib := r.values[st.sib]
			b := pathBit(st, index)
			if b == 1 && p.gens != nil {
				x.Mul(&x, &p.gens[st.lvl])
			}
			left, right := cur, sib
			if b == 1 {
				left, right = sib, cur
			}
			out := compressCell(left, right)
			cs.set(merkleB, row, b)
			cs.set(merkleIdx, row, index)
			cs.set(merkleB0, row, b0)
			cs.setElem(merkleX, row, x)
			cs.setCell(merkleCur, row, cur)
			cs.setCell(merkleS, row, sib)
			cs.setCell(merkleOut, row, out)
			cur = out
			if !st.inj {
				index >>= 1
			}
			row++
		}
	}
	return nil
}
