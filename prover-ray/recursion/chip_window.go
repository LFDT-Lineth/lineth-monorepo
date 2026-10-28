package recursion

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// Setup columns of the window chip.
const (
	windowActive = "active"
	windowAddrP  = "addr_p"
	windowAddrQ  = "addr_q"
	windowSelQ   = "sel_q"
	windowAddrI  = "addr_i"
	windowMultI  = "m_i" // −1 for a Pack item (read), the read count for a Lanes output (write)
	windowOff    = "off" // lanes
	windowLen    = "len" // lanes 1..7
)

// Witness columns of the window chip.
const (
	windowP = "p" // lanes
	windowQ = "q" // lanes
	windowI = "i" // lanes
)

// Streams. Pack lays items out as the blocks ray's Merkle–Damgård hash
// (poseidon2.MDHasher, and fiatshamir.FiatShamir on top of it) compresses:
// the hasher's buffer holds start elements already (0 for a fresh hasher, 1
// after a Fiat–Shamir challenge: its safeguard zero), the items' elements
// follow, 8 per block, and the last partial block is left-padded with zeros
// (as MDHasher.SumDigest does). Pack pins every lane that is not an item's to
// zero, so the hasher compresses the blocks as they are, from its chained
// state (zero, or the previous digest), with no padding to handle.

// streamCells is the number of cells of a stream of length elements.
func streamCells(length int) int { return (length + CellWidth - 1) / CellWidth }

// MDBlocks returns the blocks Pack lays out for the elements xs after start
// buffered zeros: those MDHasher compresses (see Streams).
func MDBlocks(start int, xs []field.Element) []Cell {
	flat := append(make([]field.Element, start), xs...)
	res := make([]Cell, streamCells(len(flat)))
	for c := range res {
		for j := range CellWidth {
			if p := c*CellWidth + j; p < len(flat) {
				res[c][j] = flat[p]
			}
		}
	}
	if rem := len(flat) % CellWidth; rem != 0 {
		res[len(res)-1] = rotate(res[len(res)-1], rem)
	}
	return res
}

// rotate returns the cell whose lane j is lane (j + r) mod 8 of c: the last
// partial block, its r elements right-aligned.
func rotate(c Cell, r int) Cell {
	var res Cell
	for j := range CellWidth {
		res[j] = c[(j+r)%CellWidth]
	}
	return res
}

type packInst struct {
	start        int
	items, kinds []int
	stream       []int // the stream cells, right-padded, written by the witness chip
	last         int   // the left-aligned last block (rotation of the last stream cell), or -1
}

type lanesInst struct {
	src, off, kind, out int
}

type windowInst struct {
	p, q      int  // q < 0 when the item fits in p
	item      slot // read (a Pack item) or written (a Lanes output)
	off, kind int
}

// Pack lays the items (cells of the given kinds: their first kind lanes) out
// as a fresh hasher's blocks (see Streams), and returns them: hashing them
// from the zero state hashes the items' elements, as MDHasher does. An item's
// kind must cover its cell's kind (KindE6 for an E6 cell, not KindScalar):
// its window reads it with zero lanes beyond the item's kind, which are not
// hashed, so proving fails otherwise.
func (m *Machine) Pack(items, kinds []int) []int { return m.PackAt(0, items, kinds) }

// PackAt is Pack after start elements already in the hasher's buffer, zero:
// PackAt(1, …) lays out the elements a Fiat–Shamir transcript absorbs after a
// challenge, following its safeguard zero, to compress from the challenge's
// digest.
//
// The items are laid out contiguously, after start zero lanes, in stream cells
// (hints) checked against them by window rows. Zero-pin window rows (reading
// the zero constant) pin the start lanes and the right padding of the last
// cell; one more window row writes the last block, the last cell rotated so
// that its elements are right-aligned (it reads the cell twice: W = L‖L).
func (m *Machine) PackAt(start int, items, kinds []int) []int {
	if len(items) != len(kinds) {
		panic(fmt.Sprintf("Pack: %d items, %d kinds", len(items), len(kinds)))
	}
	if start < 0 || start >= CellWidth {
		panic(fmt.Sprintf("Pack: start %d", start))
	}
	length := start
	for _, k := range kinds {
		checkKind(k)
		length += k
	}
	stream := make([]int, streamCells(length))
	for i := range stream {
		stream[i] = m.hint(KindDigest)
	}
	in := packInst{start: start, items: items, kinds: kinds, stream: stream, last: -1}
	blocks := append([]int(nil), stream...)
	rem := length % CellWidth
	var last slot
	if rem != 0 {
		last = m.wr()
		in.last = last.addr
		blocks[len(blocks)-1] = last.addr
	}
	m.packs = append(m.packs, in)
	m.record(instrPack, len(m.packs)-1)

	zero := m.Const(Cell{})
	if start > 0 {
		m.window(stream, 0, start, m.rd(zero))
	}
	g := start
	for i, it := range items {
		m.window(stream, g, kinds[i], m.rd(it))
		g += kinds[i]
	}
	if rem != 0 {
		l := stream[len(stream)-1]
		m.window([]int{l}, rem, CellWidth-rem, m.rd(zero))
		m.window([]int{l, l}, rem, CellWidth, last)
	}
	return blocks
}

func (m *Machine) execPack(p packInst, r *Run) {
	flat := make([]field.Element, p.start)
	for i, it := range p.items {
		flat = append(flat, r.values[it][:p.kinds[i]]...)
	}
	for c := range p.stream {
		var v Cell
		for j := range CellWidth {
			if q := c*CellWidth + j; q < len(flat) {
				v[j] = flat[q]
			}
		}
		r.values[p.stream[c]] = v
	}
	if p.last >= 0 {
		r.values[p.last] = rotate(r.values[p.stream[len(p.stream)-1]], len(flat)%CellWidth)
	}
}

// Lanes returns a cell of the given kind (scalar or E6) holding lanes
// off..off+kind−1 of the cell src, in its lanes 0..kind−1 and zero elsewhere:
// for instance the E6 value of a challenge digest (Lanes(d, 0, KindE6)), or
// one of its lanes as a scalar. A window row reads src and writes it, zero
// beyond its kind.
func (m *Machine) Lanes(src, off, kind int) int {
	checkCellKind(kind)
	if off < 0 || off+kind > CellWidth {
		panic(fmt.Sprintf("Lanes: lanes %d..%d out of a cell", off, off+kind-1))
	}
	out := m.wr()
	m.lanes = append(m.lanes, lanesInst{src, off, kind, out.addr})
	m.record(instrLanes, len(m.lanes)-1)
	m.window([]int{src}, off, kind, out)
	return out.addr
}

func (m *Machine) execLanes(l lanesInst, r *Run) {
	var c Cell
	copy(c[:l.kind], r.values[l.src][l.off:l.off+l.kind])
	r.values[l.out] = c
}

// window adds a window row: the item (read, or written) is the kind elements
// of the stream starting at element offset g, and zero beyond.
func (m *Machine) window(stream []int, g, kind int, item slot) {
	checkKind(kind)
	c, off := g/CellWidth, g%CellWidth
	w := windowInst{p: stream[c], q: -1, item: item, off: off, kind: kind}
	m.read(w.p)
	if off+kind > CellWidth {
		w.q = stream[c+1]
		m.read(w.q)
	}
	m.windows = append(m.windows, w)
}

func checkKind(k int) {
	if k < 1 || k > CellWidth {
		panic(fmt.Sprintf("item kind %d", k))
	}
}

// windowChip: one row per item of a packed stream (read), per zero pin of a
// stream (reading the zero constant), per last block of a stream (written:
// W = L‖L, the last cell twice), and per Lanes output (written).
//
// Witness: p0..7, q0..7 (two consecutive stream cells, the window W = p‖q),
// i0..7 (the item). Setup: active, addr_p, addr_q, sel_q (q is read), addr_i,
// m_i (the item's signed multiplicity: −1 for a read, the read count for a
// write), off0..7 (one-hot offset of the item in p), and len1..7 (len_k = 1
// when the item has more than k lanes). For each lane k of the item,
//
//	i_k = len_k·Σ_o off_o·W_(o+k),
//
// with len_0 = 1: the item's lanes beyond its kind are zero, so a Lanes output
// is typed by its row, and a Pack item must have no lane beyond the kind it is
// packed with. Bus: reads p, q (if sel_q); reads or writes the item.
type windowChip struct{ m *Machine }

func (c windowChip) name() string { return windowMod }

func (c windowChip) rows() int { return len(c.m.windows) }

func (c windowChip) setup(cs *cols) {
	cs.declare(windowActive, windowAddrP, windowAddrQ, windowSelQ, windowAddrI, windowMultI)
	cs.declareLanes(windowOff, CellWidth)
	for k := 1; k < CellWidth; k++ {
		cs.declare(laneName(windowLen, k))
	}
	for row, w := range c.m.windows {
		cs.set(windowActive, row, 1)
		cs.set(windowAddrP, row, uint64(w.p))
		if w.q >= 0 {
			cs.set(windowAddrQ, row, uint64(w.q))
			cs.set(windowSelQ, row, 1)
		}
		cs.set(windowAddrI, row, uint64(w.item.addr))
		cs.setInt(windowMultI, row, c.m.mult(w.item))
		cs.set(laneName(windowOff, w.off), row, 1)
		for k := 1; k < w.kind; k++ {
			cs.set(laneName(windowLen, k), row, 1)
		}
	}
}

func (c windowChip) define(m *module, bus *Bus) error {
	active := m.s(windowActive)
	bus.Read(m, m.s(windowAddrP), cellLanes(m.lanes(windowP, CellWidth)), active)
	bus.Read(m, m.s(windowAddrQ), cellLanes(m.lanes(windowQ, CellWidth)), m.s(windowSelQ))
	bus.Write(m, m.s(windowAddrI), cellLanes(m.lanes(windowI, CellWidth)), m.s(windowMultI))

	window := func(j int) wiop.Expression {
		if j < CellWidth {
			return m.w(laneName(windowP, j))
		}
		return m.w(laneName(windowQ, j-CellWidth))
	}
	for k := range CellWidth {
		terms := make([]wiop.Expression, CellWidth)
		for o := range CellWidth {
			terms[o] = wiop.Mul(m.s(laneName(windowOff, o)), window(o+k))
		}
		w := sum(terms...)
		if k > 0 {
			w = wiop.Mul(m.s(laneName(windowLen, k)), w)
		}
		m.assertZero(wiop.Sub(m.w(laneName(windowI, k)), w))
	}
	return nil
}

func (c windowChip) trace(cs *cols, r *Run) error {
	for _, x := range []string{windowP, windowQ, windowI} {
		cs.declareLanes(x, CellWidth)
	}
	for row, w := range c.m.windows {
		cs.setCell(windowP, row, r.values[w.p])
		if w.q >= 0 {
			cs.setCell(windowQ, row, r.values[w.q])
		}
		cs.setCell(windowI, row, r.values[w.item.addr])
	}
	return nil
}
