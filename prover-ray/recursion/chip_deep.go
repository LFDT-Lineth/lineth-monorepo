package recursion

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// Setup columns of the deep chips.
const (
	deepActive = "active"
	deepFirst  = "first"   // a segment's first row, and padding
	deepCont   = "cont"    // the segment continues from addr_in
	deepAct    = "act"     // lanes: the step takes part
	deepSel    = "sel"     // lanes: the step adds its value
	deepAddrX  = "addr_x"  // lanes: the cells read (one on the base instance)
	deepAddrAl = "addr_al" // α
	deepAddrIn = "addr_in" // the handed-over h, on a continuing segment's first row
	deepAddrO  = "addr_o"  // the h written on a segment's last row
	deepMO     = "m_o"     // its read count
)

// Witness columns of the deep chips.
const (
	deepX   = "x"   // lanes: the cells read
	deepAl  = "al"  // lanes 0..5: α
	deepHIn = "hin" // lanes 0..5: the h entering the row
	deepH   = "h"   // h<j>_0..5: the h after step j (deepHPrefix)
)

// deepExtLanes is the number of E6 values one row of the Ext instance
// consumes. Each costs a cell read and a set of h columns, and halves the
// instance's rows.
const deepExtLanes = 2

// DeepRow is one row of a DEEP Horner chain: the cell it reads and, as bit
// masks over its lanes, the lanes that take part (Act) and those whose value
// is added (Sel). A base row reads up to CellWidth base values, one per lane;
// an Ext row reads an E6 value and has a single step (Act = 1).
type DeepRow struct {
	Cell     int
	Ext      bool
	Act, Sel uint8
}

// deepSeg is a maximal run of rows of one kind within a chain. Base and Ext
// rows live on different modules (see deepChip), so a chain is cut at every
// change of kind and the running h is handed over through a cell: in is the
// cell holding the h the segment starts from (−1 for a chain's first
// segment), out the cell it writes the h it ends with: the chain's result on
// the last segment, the next segment's in otherwise.
type deepSeg struct {
	ext  bool
	rows []DeepRow
	in   int
	out  slot
}

type deepInst struct {
	alpha int
	segs  []deepSeg
}

// DeepHorner returns h after the rows, from h = 0: a base row applies, for
// its active lanes j from CellWidth − 1 down to 0,
//
//	h ← h·α + sel_j·x_j,
//
// x_j the j-th lane of its cell (a base value); an Ext row applies
// h ← h·α + sel·x, x its cell's E6 value. Over values of decreasing index i,
// this is the Horner Σ_i α^i·sel_i·x_i, CellWidth base values per row.
//
// It computes the opened side of a DEEP quotient at a query: the level's
// value at x is a^R·running + Σ_z (Σ_c a^(p(c))·f_c(x) − V_z)/(x − z) (see
// fri.deepPowers), V_z = Σ_c a^(p(c))·y_(c,z) is independent of the query,
// and Σ_c a^(p(c))·f_c(x) over a group's columns opened at z is a chain whose
// Sel marks those columns (the group's powers are contiguous). Act skips a
// cell's lanes past the group's last value.
func (m *Machine) DeepHorner(alpha int, rows []DeepRow) int {
	if len(rows) == 0 {
		panic("DeepHorner: no rows")
	}
	for _, r := range rows {
		if r.Act == 0 || r.Sel&^r.Act != 0 || (r.Ext && r.Act != 1) {
			panic(fmt.Sprintf("DeepHorner: row act %08b sel %08b ext %v", r.Act, r.Sel, r.Ext))
		}
		m.read(r.Cell)
	}
	in := deepInst{alpha: alpha}
	for i := 0; i < len(rows); {
		j := i
		for j < len(rows) && rows[j].Ext == rows[i].Ext {
			j++
		}
		in.segs = append(in.segs, deepSeg{ext: rows[i].Ext, rows: append([]DeepRow(nil), rows[i:j]...), in: -1})
		i = j
	}
	for k := range in.segs[:len(in.segs)-1] {
		in.segs[k].out = m.wr()
		in.segs[k+1].in = m.rd(in.segs[k].out.addr).addr
	}
	last := &in.segs[len(in.segs)-1]
	last.out = m.wr()
	// α is read once per physical row
	for _, sg := range in.segs {
		for range deepPhysRows(sg) {
			m.read(alpha)
		}
	}
	m.deeps = append(m.deeps, in)
	m.record(instrDeep, len(m.deeps)-1)
	return last.out.addr
}

// deepPhysRows is the number of rows a segment takes on its instance: one per
// base row, deepExtLanes Ext rows per row.
func deepPhysRows(sg deepSeg) int {
	if sg.ext {
		return (len(sg.rows) + deepExtLanes - 1) / deepExtLanes
	}
	return len(sg.rows)
}

// deepRowSteps applies a row to h and returns h after each lane, hs[j] after
// lane j (lanes CellWidth − 1 down to 0; an inactive lane keeps h).
func deepRowSteps(h, alpha field.Ext, row DeepRow, x Cell) [CellWidth]field.Ext {
	var hs [CellWidth]field.Ext
	for j := CellWidth - 1; j >= 0; j-- {
		if row.Act>>j&1 == 1 {
			var t field.Ext
			if row.Sel>>j&1 == 1 {
				if row.Ext {
					t = CellE6(x)
				} else {
					t.B0.A0 = x[j]
				}
			}
			h.Mul(&h, &alpha)
			h.Add(&h, &t)
		}
		hs[j] = h
	}
	return hs
}

// execDeep runs the chain and writes every segment's h, the handed-over cells
// as well as the chain's result.
func (m *Machine) execDeep(in deepInst, r *Run) {
	alpha := CellE6(r.values[in.alpha])
	var h field.Ext
	for _, sg := range in.segs {
		for _, row := range sg.rows {
			h = deepRowSteps(h, alpha, row, r.values[row.Cell])[0]
		}
		r.values[sg.out.addr] = E6Cell(h)
	}
}

// deepChip: the DEEP Horner chains, one instance per kind of row,
//
//	h_j = h_(j+1) + act_j·(h_(j+1)·α + t_j − h_(j+1)),   j = lanes − 1 … 0,
//
// with h_lanes = hin, and t_j = sel_j·x_j: on the base instance x_j is lane j
// of the row's cell, added to h's first lane; on the Ext instance it is the
// E6 value of the row's cell j.
//
// There are two instances, on two modules. A base row steps through up to
// CellWidth base values, so it needs CellWidth sets of h columns; an Ext row
// consumes a single E6 value and steps once, so seven of those eight sets
// would be dead on it. Ext rows are most of the chip (74% on the prototype's
// two-proof RISC-V verifier at 128 queries), so the Ext instance has its own
// module, deepExtLanes steps (cells) per row.
//
// A chain's rows are cut into segments at every change of kind, and the
// running h crosses from one module to the other through a cell:
//
//	hin = h_0[−1]               on a segment's later rows
//	hin = the cell at addr_in   on a continuing segment's first row (cont)
//	hin = 0                     on a chain's first row, and on padding
//
// Witness: x (the cells: 8 lanes on the base instance, 6 per cell on the Ext
// one), al0..5, hin0..5, h<j>_0..5 for j < lanes. Setup: active, first, cont,
// act_j, sel_j, addr_x (per cell), addr_al, addr_in, and addr_o, m_o (h_0
// written on a segment's last row). Padding rows are first rows with no
// active lane and no cont, so their h is 0. Every bus access has a setup
// multiplicity, so padding rows are inert; the constraints have degree 3.
type deepChip struct {
	m     *Machine
	mod   string
	lanes int // steps per row
	ext   bool
}

func (c deepChip) name() string { return c.mod }

// cells is the number of cells a row reads: one for a base row, whose
// CellWidth steps are its lanes, and one per step for an Ext row.
func (c deepChip) cells() int {
	if c.ext {
		return c.lanes
	}
	return 1
}

// cellLanes is the number of lanes of each cell the chip commits: all of them
// for a base row, the six of an E6 value for an Ext row (an E6 cell has zero
// lanes 6, 7).
func (c deepChip) cellLanes() int {
	if c.ext {
		return KindE6
	}
	return CellWidth
}

func (c deepChip) rows() int {
	n := 0
	for _, in := range c.m.deeps {
		for _, sg := range in.segs {
			if sg.ext == c.ext {
				n += deepPhysRows(sg)
			}
		}
	}
	return n
}

// deepHPrefix is the prefix of the lanes of h after step j: h<j>_0..5.
func deepHPrefix(j int) string { return laneName(deepH, j) + "_" }

func (c deepChip) setup(cs *cols) {
	cs.declare(deepActive, deepFirst, deepCont, deepAddrAl, deepAddrIn, deepAddrO, deepMO)
	cs.declareLanes(deepAddrX, c.cells())
	cs.declareLanes(deepAct, c.lanes)
	cs.declareLanes(deepSel, c.lanes)
	per := c.cells()
	row := 0
	for _, in := range c.m.deeps {
		for _, sg := range in.segs {
			if sg.ext != c.ext {
				continue
			}
			// An Ext row takes c.lanes of the segment's rows, the earliest on
			// the highest lane so that the Horner's order is kept; a base row
			// takes one, on all its lanes.
			for i := 0; i < len(sg.rows); i += per {
				cs.set(deepActive, row, 1)
				if i == 0 {
					cs.set(deepFirst, row, 1)
					if sg.in >= 0 {
						cs.set(deepCont, row, 1)
						cs.set(deepAddrIn, row, uint64(sg.in))
					}
				}
				if c.ext {
					for k := 0; k < per && i+k < len(sg.rows); k++ {
						r, j := sg.rows[i+k], c.lanes-1-k
						cs.set(laneName(deepAct, j), row, 1)
						cs.set(laneName(deepSel, j), row, uint64(r.Sel&1))
						cs.set(laneName(deepAddrX, j), row, uint64(r.Cell))
					}
				} else {
					r := sg.rows[i]
					for j := range c.lanes {
						cs.set(laneName(deepAct, j), row, uint64(r.Act>>j&1))
						cs.set(laneName(deepSel, j), row, uint64(r.Sel>>j&1))
					}
					cs.set(laneName(deepAddrX, 0), row, uint64(r.Cell))
				}
				cs.set(deepAddrAl, row, uint64(in.alpha))
				if i+per >= len(sg.rows) {
					cs.set(deepAddrO, row, uint64(sg.out.addr))
					cs.setInt(deepMO, row, c.m.mult(sg.out))
				}
				row++
			}
		}
	}
	for ; row < cs.n; row++ {
		cs.set(deepFirst, row, 1)
	}
}

func (c deepChip) define(m *module, bus *Bus) error {
	active, first, cont := m.s(deepActive), m.s(deepFirst), m.s(deepCont)
	one := constant(1)
	al := m.lanes(deepAl, KindE6)
	w := c.cellLanes()
	x := m.lanes(deepX, c.cells()*w)

	// hin, the h the row starts from: the previous row's within a segment,
	// the handed-over cell on a continuing segment's first row (the bus read
	// binds it), 0 otherwise.
	hin := m.lanes(deepHIn, KindE6)
	for k := range KindE6 {
		prev := m.wShift(laneName(deepHPrefix(0), k), -1)
		m.assertZero(wiop.Mul(wiop.Sub(one, first), wiop.Sub(hin[k], prev)))
		m.assertZero(prod(first, wiop.Sub(one, cont), hin[k]))
	}
	bus.Read(m, m.s(deepAddrIn), cellOf(hin...), cont)

	prev := hin
	for j := c.lanes - 1; j >= 0; j-- {
		act, sel := m.s(laneName(deepAct, j)), m.s(laneName(deepSel, j))
		step := e6MulExprs(prev, al)
		if c.ext {
			// step j adds the E6 value of its own cell
			for k := range KindE6 {
				step[k] = wiop.Add(step[k], wiop.Mul(sel, x[j*w+k]))
			}
		} else {
			step[0] = wiop.Add(step[0], wiop.Mul(sel, x[j]))
		}
		h := m.lanes(deepHPrefix(j), KindE6)
		for k := range KindE6 {
			m.assertZero(wiop.Sub(wiop.Sub(h[k], prev[k]), wiop.Mul(act, wiop.Sub(step[k], prev[k]))))
		}
		prev = h
	}
	if c.ext {
		for j := range c.lanes {
			bus.Read(m, m.s(laneName(deepAddrX, j)), cellOf(x[j*w:(j+1)*w]...), m.s(laneName(deepAct, j)))
		}
	} else {
		bus.Read(m, m.s(laneName(deepAddrX, 0)), cellLanes(x), active)
	}
	bus.Read(m, m.s(deepAddrAl), cellOf(al...), active)
	bus.Write(m, m.s(deepAddrO), cellOf(m.lanes(deepHPrefix(0), KindE6)...), m.s(deepMO))
	return nil
}

func (c deepChip) trace(cs *cols, r *Run) error {
	w := c.cellLanes()
	cs.declareLanes(deepX, c.cells()*w)
	cs.declareLanes(deepAl, KindE6)
	cs.declareLanes(deepHIn, KindE6)
	for j := range c.lanes {
		cs.declareLanes(deepHPrefix(j), KindE6)
	}
	row := 0
	for _, in := range c.m.deeps {
		alpha := CellE6(r.values[in.alpha])
		for _, sg := range in.segs {
			if sg.ext != c.ext {
				continue
			}
			var h field.Ext
			if sg.in >= 0 {
				h = CellE6(r.values[sg.in])
			}
			if !c.ext {
				for _, dr := range sg.rows {
					x := r.values[dr.Cell]
					hs := deepRowSteps(h, alpha, dr, x)
					cs.setE6(deepAl, row, alpha)
					cs.setE6(deepHIn, row, h)
					cs.setCell(deepX, row, x)
					for j := range c.lanes {
						cs.setE6(deepHPrefix(j), row, hs[j])
					}
					h = hs[0]
					row++
				}
				continue
			}
			for i := 0; i < len(sg.rows); i += c.lanes {
				cs.setE6(deepAl, row, alpha)
				cs.setE6(deepHIn, row, h)
				// step j of the row is the segment's row i + lanes − 1 − j; an
				// unused step (a partial row) keeps h
				for j := c.lanes - 1; j >= 0; j-- {
					if k := i + c.lanes - 1 - j; k < len(sg.rows) {
						dr := sg.rows[k]
						x := r.values[dr.Cell]
						for l := range w {
							cs.setElem(laneName(deepX, j*w+l), row, x[l])
						}
						h = deepRowSteps(h, alpha, dr, x)[0]
					}
					cs.setE6(deepHPrefix(j), row, h)
				}
				row++
			}
		}
	}
	return nil
}
