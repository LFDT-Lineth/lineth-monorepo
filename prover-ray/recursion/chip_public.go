package recursion

import (
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// Setup columns of the public chip.
const (
	publicAddr = "addr"
	publicSel  = "sel"
	publicV    = "v" // lanes
)

// publicInst is one cell exposed as public inputs: lanes 0..kind-1 of the
// cell at s.addr, lane l under tag_(first+l).
type publicInst struct {
	s     slot
	kind  int
	tag   wiop.PublicInputTag
	first int
}

// Public exposes lanes 0..kind-1 of the cell at addr as public inputs of the
// compiled system, lane l under the tag suffixed first+l (a scalar is tag_first,
// an E6 value the six base cells tag_first..tag_first+5). The public chip
// reads the cell over the memory bus into its own lanes, which are opened at
// its row, so the public inputs equal the cell. Lanes beyond the kind are
// provably zero (the witness chip's z1/z6 constraints, the E6 chip's zero
// lanes), so exposing only the kind's lanes loses nothing. Public inputs are
// laid out in the order of the Public calls.
func (m *Machine) Public(addr, kind int, tag wiop.PublicInputTag, first int) {
	checkCellKind(kind)
	m.publics = append(m.publics, publicInst{s: m.rd(addr), kind: kind, tag: tag, first: first})
}

// publicChip: one row per Public call. Setup: addr, sel. Witness: v0..v7, the
// cell read on the bus at addr, whose first lanes are opened as public inputs.
type publicChip struct{ m *Machine }

func (c publicChip) name() string { return publicMod }

func (c publicChip) rows() int { return len(c.m.publics) }

func (c publicChip) setup(cs *cols) {
	cs.declare(publicAddr, publicSel)
	for row, p := range c.m.publics {
		cs.set(publicAddr, row, uint64(p.s.addr))
		cs.set(publicSel, row, 1)
	}
}

func (c publicChip) define(m *module, bus *Bus) error {
	v := m.lanes(publicV, CellWidth)
	bus.Read(m, m.s(publicAddr), cellLanes(v), m.s(publicSel))
	for row, p := range c.m.publics {
		for l := range p.kind {
			col := m.witness[laneName(publicV, l)]
			cell := col.At(row).Open(m.sys.Context.Childf("%s.open-%d-%d", m.name, row, l))
			m.sys.RegisterPublicInputs(p.tag, cell, p.first+l)
		}
	}
	return nil
}

func (c publicChip) trace(cs *cols, r *Run) error {
	cs.declareLanes(publicV, CellWidth)
	for row, p := range c.m.publics {
		cs.setCell(publicV, row, r.values[p.s.addr])
	}
	return nil
}
