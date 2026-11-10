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
// (KindE6) as one extension public input, bound to its lanes by an [E6Check].
// Public inputs are laid out in the order of the Public calls. kind is the
// caller's declaration, not checked against the cell: a scalar declared on an
// E6 cell exposes lane 0 only. A digest has no public-input shape: Public
// panics on KindDigest.
func (m *Machine) Public(addr, kind int, tag wiop.PublicInputTag, index int) {
	if kind != KindScalar && kind != KindE6 {
		panic(fmt.Sprintf("Public: kind %d, want a scalar or an E6 value", kind))
	}
	m.publics = append(m.publics, publicInst{s: m.rd(addr), kind: kind, tag: tag, index: index})
}

// publicChip: one row per Public call. Setup: addr, sel. Witness: v0..v5, the
// cell read on the bus at addr, whose first kind lanes are opened. Lanes 6 and
// 7 are constant zero on the bus, as no digest is exposed.
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
	v := m.lanes(publicV, KindE6)
	bus.Read(m, m.s(publicAddr), cellOf(v...), m.s(publicSel))
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
		ext := m.round.NewLazyCell(ctx.Childf("e6"), true, func(rt *wiop.Runtime) field.Gen {
			var cell Cell
			for l, lane := range lanes {
				cell[l] = rt.GetCellValue(lane).AsBase()
			}
			return field.ElemFromExt(CellE6(cell))
		})
		m.sys.RegisterPublicInputs(p.tag, ext, p.index)
		m.round.RegisterVerifierAction(&E6Check{Cell: ext, Lanes: lanes})
	}
	return nil
}

func (c publicChip) trace(cs *cols, r *Run) error {
	cs.declareLanes(publicV, KindE6)
	for row, p := range c.m.publics {
		cs.setE6(publicV, row, CellE6(r.values[p.s.addr]))
	}
	return nil
}

// E6Check is the verifier action binding an E6 public input to the six lane
// openings of its public chip row: Cell = CellE6(Lanes), the E6 value of lanes
// (B0.A0, B0.A1, B1.A0, B1.A1, B2.A0, B2.A1). It is the only check tying the
// public input to the proof: wiop has no extension constant, so the
// recomposition cannot be a constraint, and the lanes alone are bound by their
// openings. Every verifier of a program must run it: verifier-ray's codegen
// (which rejects an action it does not handle), the in-machine verifier, and
// gnark should a program ever be wrapped.
type E6Check struct {
	Cell  *wiop.Cell
	Lanes []*wiop.Cell
}

// Check implements [wiop.VerifierAction].
func (c *E6Check) Check(rt *wiop.Runtime) error {
	var cell Cell
	for l, lane := range c.Lanes {
		v := rt.GetCellValue(lane)
		if !v.IsBase() {
			return fmt.Errorf("recursion: public input %q: lane %d is not a base field element", c.Cell.Context.Path(), l)
		}
		cell[l] = v.AsBase()
	}
	want, got := CellE6(cell), rt.GetCellValue(c.Cell).AsExt()
	if !want.Equal(&got) {
		return fmt.Errorf("recursion: public input %q is not the E6 value of its lanes", c.Cell.Context.Path())
	}
	return nil
}
