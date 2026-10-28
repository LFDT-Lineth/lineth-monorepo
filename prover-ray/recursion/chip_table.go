package recursion

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// Setup columns of the table chip.
const (
	tableActive  = "active"
	tableSrcAddr = "src_addr"
	tableTag     = "tag"
	tableIdx     = "idx"
)

// Witness columns of the table chip.
const (
	tableCnt = "cnt"
	tableV   = "v" // lanes
)

// Setup columns of the lookup chip.
const (
	lookupActive  = "active"
	lookupIdxAddr = "idx_addr"
	lookupTag     = "tag"
	lookupOutAddr = "out_addr"
	lookupOutMult = "out_mult"
)

// Witness columns of the lookup chip.
const (
	lookupIdx = "idx"
	lookupV   = "v" // lanes
)

// Dynamic reads. A table holds copies of cells, its entries; a lookup reads
// the entry at an index computed during execution, from the index cell idx.
// The entries are not memory cells: they live on the bus's tables sum, as the
// tuples (tag, i, v) of the table tag, written once with a witness
// multiplicity, their number of reads (it depends on the execution). A lookup
// reads (tag, idx, v): it balances only against the entry idx of its own
// table, with its value. An index out of the table, or another table's entry,
// matches no entry, whatever its value: callers need not bound the indices.

// Table is a table of cells readable at a computed index.
type Table struct {
	tag int // its index in Machine.tables
	n   int
}

// Len returns the number of entries.
func (t Table) Len() int { return t.n }

type tableInst struct {
	src []int // the copied cells
}

type lookupInst struct {
	tag, idx int
	out      slot
}

// Table returns a table whose entry i holds the value of cells[i] (of its
// kind).
func (m *Machine) Table(cells []int) Table {
	if len(cells) == 0 {
		panic("Table: no cell")
	}
	for _, c := range cells {
		m.read(c)
	}
	m.tables = append(m.tables, tableInst{src: append([]int(nil), cells...)})
	return Table{tag: len(m.tables) - 1, n: len(cells)}
}

// Lookup returns a cell holding the entry idx of the table t, for a scalar
// cell idx. Executing it fails if idx is not below the table's length (and a
// proof of such a read does not verify).
func (m *Machine) Lookup(t Table, idx int) int {
	if t.tag < 0 || t.tag >= len(m.tables) || len(m.tables[t.tag].src) != t.n {
		panic("Lookup: unknown table")
	}
	in := lookupInst{tag: t.tag, idx: m.rd(idx).addr, out: m.wr()}
	m.lookups = append(m.lookups, in)
	m.record(instrLookup, len(m.lookups)-1)
	return in.out.addr
}

// lookupEntry returns the index of a lookup in its table, or an error if it
// is out of the table.
func (m *Machine) lookupEntry(l lookupInst, r *Run) (int, error) {
	i, n := r.values[l.idx][0].Uint64(), len(m.tables[l.tag].src)
	if i >= uint64(n) {
		return 0, fmt.Errorf("Lookup: index %d out of a table of %d entries", i, n)
	}
	return int(i), nil
}

func (m *Machine) execLookup(l lookupInst, r *Run) error {
	i, err := m.lookupEntry(l, r)
	if err != nil {
		return err
	}
	r.values[l.out.addr] = r.values[m.tables[l.tag].src[i]]
	return nil
}

// tableChip: one row per table entry.
//
// Witness: v0..7 (the value), cnt (its number of reads). Setup: active,
// src_addr (the copied cell), tag, idx (the entry's table and index). Bus:
// reads v at src_addr; writes the entry (tag, idx, v) with multiplicity
// active·cnt. The gating matters: on the module's padding rows the setup is
// zero, so the source read is off and v is free, while cnt is a witness;
// ungated, a padding row would write any value as the entry (tag 0, idx 0),
// with any multiplicity.
type tableChip struct{ m *Machine }

func (c tableChip) name() string { return tableMod }

func (c tableChip) rows() int {
	n := 0
	for _, t := range c.m.tables {
		n += len(t.src)
	}
	return n
}

func (c tableChip) setup(cs *cols) {
	cs.declare(tableActive, tableSrcAddr, tableTag, tableIdx)
	row := 0
	for k, t := range c.m.tables {
		for i, src := range t.src {
			cs.set(tableActive, row, 1)
			cs.set(tableSrcAddr, row, uint64(src))
			cs.set(tableTag, row, uint64(k))
			cs.set(tableIdx, row, uint64(i))
			row++
		}
	}
}

func (c tableChip) define(m *module, bus *Bus) error {
	v := cellLanes(m.lanes(tableV, CellWidth))
	bus.Read(m, m.s(tableSrcAddr), v, m.s(tableActive))
	bus.WriteEntry(m, m.s(tableTag), m.s(tableIdx), v, wiop.Mul(m.s(tableActive), m.w(tableCnt)))
	return nil
}

// trace fills the values and counts the reads of every entry.
func (c tableChip) trace(cs *cols, r *Run) error {
	cs.declare(tableCnt)
	cs.declareLanes(tableV, CellWidth)
	counts := make([][]uint64, len(c.m.tables))
	for k, t := range c.m.tables {
		counts[k] = make([]uint64, len(t.src))
	}
	for _, l := range c.m.lookups {
		i, err := c.m.lookupEntry(l, r)
		if err != nil {
			return err
		}
		counts[l.tag][i]++
	}
	row := 0
	for k, t := range c.m.tables {
		for i, src := range t.src {
			cs.setCell(tableV, row, r.values[src])
			cs.set(tableCnt, row, counts[k][i])
			row++
		}
	}
	return nil
}

// lookupChip: one row per lookup.
//
// Witness: idx (the index), v0..7 (the entry read). Setup: active, idx_addr
// (the index cell), tag (the table), out_addr and out_mult (the result cell).
// Bus: reads idx (a scalar) at idx_addr; reads the entry (tag, idx, v); writes
// v at out_addr.
type lookupChip struct{ m *Machine }

func (c lookupChip) name() string { return lookupMod }

func (c lookupChip) rows() int { return len(c.m.lookups) }

func (c lookupChip) setup(cs *cols) {
	cs.declare(lookupActive, lookupIdxAddr, lookupTag, lookupOutAddr, lookupOutMult)
	for row, l := range c.m.lookups {
		cs.set(lookupActive, row, 1)
		cs.set(lookupIdxAddr, row, uint64(l.idx))
		cs.set(lookupTag, row, uint64(l.tag))
		cs.set(lookupOutAddr, row, uint64(l.out.addr))
		cs.setInt(lookupOutMult, row, c.m.mult(l.out))
	}
}

func (c lookupChip) define(m *module, bus *Bus) error {
	active, idx := m.s(lookupActive), m.w(lookupIdx)
	v := cellLanes(m.lanes(lookupV, CellWidth))
	bus.Read(m, m.s(lookupIdxAddr), cellOf(idx), active)
	bus.ReadEntry(m, m.s(lookupTag), idx, v, active)
	bus.Write(m, m.s(lookupOutAddr), v, m.s(lookupOutMult))
	return nil
}

func (c lookupChip) trace(cs *cols, r *Run) error {
	cs.declare(lookupIdx)
	cs.declareLanes(lookupV, CellWidth)
	for row, l := range c.m.lookups {
		cs.setElem(lookupIdx, row, r.values[l.idx][0])
		cs.setCell(lookupV, row, r.values[l.out.addr])
	}
	return nil
}
