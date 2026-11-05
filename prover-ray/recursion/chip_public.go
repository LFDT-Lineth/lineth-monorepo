package recursion

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// Setup columns of the public chip.
const (
	publicAddr = "addr"
	publicSel  = "sel"
	publicV    = "v" // lanes
)

// publicInst is one cell exposed as a public input: the cell at s.addr, of
// the given kind, under tag_index.
type publicInst struct {
	s     slot
	kind  int
	tag   wiop.PublicInputTag
	index int
}

// Public exposes the cell at addr as the public input tag_index of the
// compiled system: a scalar (KindScalar) as a base public input, an E6 value
// (KindE6) as one extension public input. The public chip reads the cell over
// the memory bus into its own lanes, which are opened at its row. A scalar's
// public input is the opening of lane 0. An E6 value's is an extension cell
// bound to the six lane openings by a verifier action: wiop has no extension
// constant, so the value cannot be recomposed from the lanes in a constraint,
// and the message-bus product of a shard proof is exposed the same way. Public
// inputs are laid out in the order of the Public calls. A digest has no
// public-input shape: Public panics on KindDigest.
func (m *Machine) Public(addr, kind int, tag wiop.PublicInputTag, index int) {
	if kind != KindScalar && kind != KindE6 {
		panic(fmt.Sprintf("Public: kind %d, want a scalar or an E6 value", kind))
	}
	m.publics = append(m.publics, publicInst{s: m.rd(addr), kind: kind, tag: tag, index: index})
}

// publicChip: one row per Public call. Setup: addr, sel. Witness: v0..v7, the
// cell read on the bus at addr, whose first lanes are opened.
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
		ctx := m.sys.Context.Childf("%s.public-%d", m.name, row)
		lanes := make([]*wiop.Cell, p.kind)
		for l := range lanes {
			lanes[l] = m.witness[laneName(publicV, l)].At(row).Open(ctx.Childf("lane-%d", l))
		}
		if p.kind == KindScalar {
			m.sys.RegisterPublicInputs(p.tag, lanes[0], p.index)
			continue
		}
		// The lane openings stay proof messages; the public input is the E6
		// value they recompose, declared in their round so that Fiat-Shamir
		// absorbs them together.
		ext := m.round.NewLazyCell(ctx.Childf("e6"), true, func(rt *wiop.Runtime) field.Gen {
			var cell Cell
			for l, lane := range lanes {
				cell[l] = rt.GetCellValue(lane).AsBase()
			}
			return field.ElemFromExt(CellE6(cell))
		})
		m.sys.RegisterPublicInputs(p.tag, ext, p.index)
		m.round.RegisterVerifierAction(&publicE6Check{cell: ext, lanes: lanes})
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

// publicE6Check binds an E6 public input to the six lane openings of its
// public chip row: the public input must be the E6 value the lanes recompose.
// The verifier reads the lanes from the proof and the value from the public
// inputs and compares them, as grandproduct.FinalProductCheck does for a bus
// product. It is a native check only, as nothing wraps a program in a gnark
// circuit; the Zig verifier of verifier-ray needs its counterpart.
type publicE6Check struct {
	cell  *wiop.Cell
	lanes []*wiop.Cell
}

// Check implements [wiop.VerifierAction].
func (c *publicE6Check) Check(rt *wiop.Runtime) error {
	var cell Cell
	for l, lane := range c.lanes {
		v := rt.GetCellValue(lane)
		if !v.IsBase() {
			return fmt.Errorf("recursion: public input %q: lane %d is not a base field element", c.cell.Context.Path(), l)
		}
		cell[l] = v.AsBase()
	}
	want, got := CellE6(cell), rt.GetCellValue(c.cell).AsExt()
	if !want.Equal(&got) {
		return fmt.Errorf("recursion: public input %q is not the E6 value of its lanes", c.cell.Context.Path())
	}
	return nil
}
