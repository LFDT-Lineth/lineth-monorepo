package recursion

import "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"

// Setup columns of the witness chip.
const (
	witnessAddr = "addr"
	witnessMult = "mult"
	witnessZ1   = "z1" // lanes 1..5 are zero (a scalar)
	witnessZ6   = "z6" // lanes 6, 7 are zero (a scalar or an E6 value)
)

// Witness columns of the witness chip.
const (
	witnessV = "v" // lanes
)

// witnessChip: one row per cell the prover writes freely: the inputs, and the
// hints, cells computed during execution whose readers check them.
//
// Setup: addr, mult, z1, z6 (from the cell's kind). Witness: v0..7. Bus:
// writes [addr ‖ v] with multiplicity mult (the cell's read count). The
// readers check the values; the chip only types them, zeroing the lanes
// beyond the cell's kind:
//
//	z1·v_k = 0  for k = 1..5,    z6·v_k = 0  for k = 6, 7.
type witnessChip struct{ m *Machine }

func (c witnessChip) name() string { return witnessMod }

func (c witnessChip) cells() []typedCell {
	return append(append([]typedCell(nil), c.m.inputs...), c.m.hints...)
}

func (c witnessChip) rows() int { return len(c.cells()) }

func (c witnessChip) setup(cs *cols) {
	cs.declare(witnessAddr, witnessMult, witnessZ1, witnessZ6)
	for row, tc := range c.cells() {
		cs.set(witnessAddr, row, uint64(tc.addr))
		cs.set(witnessMult, row, uint64(c.m.reads[tc.addr]))
		if tc.kind == KindScalar {
			cs.set(witnessZ1, row, 1)
		}
		if tc.kind <= KindE6 {
			cs.set(witnessZ6, row, 1)
		}
	}
}

func (c witnessChip) define(m *module, bus *Bus) error {
	v := m.lanes(witnessV, CellWidth)
	for k := KindScalar; k < CellWidth; k++ {
		z := m.s(witnessZ1)
		if k >= KindE6 {
			z = m.s(witnessZ6)
		}
		m.assertZero(wiop.Mul(z, v[k]))
	}
	bus.Write(m, m.s(witnessAddr), cellLanes(v), m.s(witnessMult))
	return nil
}

func (c witnessChip) trace(cs *cols, r *Run) error {
	cs.declareLanes(witnessV, CellWidth)
	for row, tc := range c.cells() {
		cs.setCell(witnessV, row, r.values[tc.addr])
	}
	return nil
}
