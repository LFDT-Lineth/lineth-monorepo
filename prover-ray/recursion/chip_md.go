package recursion

import (
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// Setup columns of the md chip.
const (
	mdActive   = "active"
	mdFirst    = "first"
	mdAddrInit = "addr_init"
	mdAddrBlk  = "addr_blk"
	mdAddrOut  = "addr_out"
	mdMultOut  = "m_out"
)

// Witness columns of the md chip.
const (
	mdSt = "st" // lanes: the state before the block
	mdB  = "b"  // lanes: the block
	mdD  = "d"  // lanes: the state after it
)

type mdInst struct {
	init   int   // the initial state's cell
	blocks []int // the block cells
	out    slot  // the digest
}

// MDHash returns the digest of the blocks with ray's Merkle–Damgård hash
// (poseidon2.MDHasher) from the zero state: for blocks laid out by Pack, the
// MDHasher digest of the packed elements.
func (m *Machine) MDHash(blocks []int) int { return m.MDHashFrom(m.Const(Cell{}), blocks) }

// MDHashFrom returns the digest of the blocks from the state in the cell
// state: compressing them one by one, state = Compress(state, block). After a
// Fiat–Shamir challenge, ray's transcript hashes from the challenge's digest
// and its buffer holds the safeguard zero: the next challenge is
// MDHashFrom(digest, PackAt(1, …)). With no block, the digest is the state.
func (m *Machine) MDHashFrom(state int, blocks []int) int {
	if len(blocks) == 0 {
		return state
	}
	m.read(state)
	for _, b := range blocks {
		m.read(b)
	}
	in := mdInst{init: state, blocks: append([]int(nil), blocks...), out: m.wr()}
	m.mds = append(m.mds, in)
	m.record(instrMD, len(m.mds)-1)
	return in.out.addr
}

// p2Input returns the permutation input of the compression of block from
// state: state ‖ block.
func p2Input(state, block Cell) [P2Width]field.Element {
	var x [P2Width]field.Element
	copy(x[:p2Half], state[:])
	copy(x[p2Half:], block[:])
	return x
}

// execMD computes the digest, and records the compressions for the P2 core.
func (m *Machine) execMD(d mdInst, r *Run) {
	st := r.values[d.init]
	for _, b := range d.blocks {
		blk := r.values[b]
		r.p2 = append(r.p2, p2Input(st, blk))
		st = compressCell(st, blk)
	}
	r.values[d.out.addr] = st
}

// mdChip: one row per block of an MDHash.
//
// Witness: st0..7 (the state before the block), b0..7 (the block), d0..7 (the
// state after it). Setup: active, first (the hash's first row), addr_init
// (the initial state, read on the first row), addr_blk (the block),
// addr_out, m_out (the digest, written on the last row).
//
//	(active − first)·(st − d[−1]) = 0      the state chains from row to row
//
// Bus: reads st at addr_init (first row), b at addr_blk; looks the
// compression (st ‖ b) → d up in the P2 core; writes d at addr_out.
type mdChip struct{ m *Machine }

func (c mdChip) name() string { return mdMod }

func (c mdChip) rows() int {
	n := 0
	for _, d := range c.m.mds {
		n += len(d.blocks)
	}
	return n
}

func (c mdChip) setup(cs *cols) {
	cs.declare(mdActive, mdFirst, mdAddrInit, mdAddrBlk, mdAddrOut, mdMultOut)
	row := 0
	for _, d := range c.m.mds {
		for i, b := range d.blocks {
			cs.set(mdActive, row, 1)
			cs.set(mdAddrBlk, row, uint64(b))
			if i == 0 {
				cs.set(mdFirst, row, 1)
				cs.set(mdAddrInit, row, uint64(d.init))
			}
			if i == len(d.blocks)-1 {
				cs.set(mdAddrOut, row, uint64(d.out.addr))
				cs.setInt(mdMultOut, row, c.m.mult(d.out))
			}
			row++
		}
	}
}

func (c mdChip) define(m *module, bus *Bus) error {
	active, first := m.s(mdActive), m.s(mdFirst)
	st, b, d := m.lanes(mdSt, CellWidth), m.lanes(mdB, CellWidth), m.lanes(mdD, CellWidth)
	notFirst := wiop.Sub(active, first)
	for i := range CellWidth {
		m.assertZero(wiop.Mul(notFirst, wiop.Sub(st[i], m.wShift(laneName(mdD, i), -1))))
	}
	bus.Read(m, m.s(mdAddrInit), cellLanes(st), first)
	bus.Read(m, m.s(mdAddrBlk), cellLanes(b), active)
	var in [P2Width]wiop.Expression
	copy(in[:p2Half], st)
	copy(in[p2Half:], b)
	bus.ReadP2(m, in, [p2Half]wiop.Expression(d), active)
	bus.Write(m, m.s(mdAddrOut), cellLanes(d), m.s(mdMultOut))
	return nil
}

func (c mdChip) trace(cs *cols, r *Run) error {
	for _, x := range []string{mdSt, mdB, mdD} {
		cs.declareLanes(x, CellWidth)
	}
	row := 0
	for _, d := range c.m.mds {
		st := r.values[d.init]
		for _, b := range d.blocks {
			blk := r.values[b]
			next := compressCell(st, blk)
			cs.setCell(mdSt, row, st)
			cs.setCell(mdB, row, blk)
			cs.setCell(mdD, row, next)
			st = next
			row++
		}
	}
	return nil
}
