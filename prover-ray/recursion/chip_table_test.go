package recursion

import (
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/stretchr/testify/require"
)

var tableVals = []uint64{5, 13, 2, 7, 8}

// tableProg is a program with two tables, A (8 E6 entries) and B (4), and
// lookups into them.
type tableProg struct {
	m       *Machine
	in      []Cell
	a, b    []field.Ext
	lookups []int // the lookups' output cells
}

// tableInputs adds the inputs of the tables A and B, random E6 values, and
// the tables.
func tableInputs(m *Machine) (tp tableProg, tabA, tabB Table) {
	r := rand.New(rand.NewPCG(11, 12))
	tp.m = m
	cellsA, cellsB := make([]int, 0, 8), make([]int, 0, 4)
	for range 8 {
		x := randE6(r)
		tp.a = append(tp.a, x)
		tp.in = append(tp.in, E6Cell(x))
		cellsA = append(cellsA, m.InputE6())
	}
	for range 4 {
		x := randE6(r)
		tp.b = append(tp.b, x)
		tp.in = append(tp.in, E6Cell(x))
		cellsB = append(cellsB, m.InputE6())
	}
	return tp, m.Table(cellsA), m.Table(cellsB)
}

// tableProgram looks entries of A up at the indices tableVals mod 8, and the
// entry 2 of B, the indices produced by Bits. With check, it asserts the
// results equal the entries computed in Go; without, it only copies them, so
// that nothing but the tables check them.
func tableProgram(check bool) tableProg {
	m := &Machine{}
	cp := withCopy(m)
	tp, tabA, tabB := tableInputs(m)
	lookup := func(t Table, v uint64, n int, want field.Ext) {
		idx := m.Bits(m.InputScalar(), n).Shr[0]
		tp.in = append(tp.in, ScalarCell(field.NewElement(v)))
		out := m.Lookup(t, idx)
		tp.lookups = append(tp.lookups, out)
		if check {
			m.AssertEqE6(out, m.Const(E6Cell(want)))
		} else {
			cp.Copy(out)
		}
	}
	for _, v := range tableVals {
		lookup(tabA, v, 3, tp.a[v&7])
	}
	lookup(tabB, 6, 2, tp.b[2])
	return tp
}

// TestTableLookup: lookups at indices computed during execution return their
// entries, and prove.
func TestTableLookup(t *testing.T) {
	tp := tableProgram(true)
	p, err := tp.m.Compile(testPCSOpts...)
	require.NoError(t, err)
	require.NotNil(t, p.Tables, "the program has tables")
	r, err := p.Execute(tp.in)
	require.NoError(t, err)
	for i, v := range tableVals {
		require.Equal(t, E6Cell(tp.a[v&7]), r.Value(tp.lookups[i]), "A[%d]", v&7)
	}
	require.Equal(t, E6Cell(tp.b[2]), r.Value(tp.lookups[len(tableVals)]), "B[2]")
	require.NoError(t, proveAndVerify(p, r))

	t.Run("out of range at execution", func(t *testing.T) {
		m := &Machine{}
		tab := m.Table([]int{m.InputE6(), m.InputE6()})
		m.Lookup(tab, m.InputScalar())
		p, err := m.Compile(testPCSOpts...)
		require.NoError(t, err)
		_, err = p.Execute([]Cell{{}, {}, ScalarCell(field.NewElement(2))})
		require.Error(t, err, "index 2 into a table of 2 executed")
	})
}

// addElem adds d to e.
func addElem(e *field.Element, d int64) {
	var x field.Element
	x.SetInt64(d)
	e.Add(e, &x)
}

// TestTableLookupRejects: a lookup returning another value than its entry's
// (consistently with its readers), entries' read counts moved, and a lookup
// moved to another entry than its index cell's, are rejected. Table rows:
// A's entries 0..7, then B's 0..3; lookups in program order.
func TestTableLookupRejects(t *testing.T) {
	for name, tamper := range map[string]func(tp tableProg, p *Program, r *Run) error{
		// lane 3 of the first lookup (A[5]), changed in its cell and
		// re-traced: its copy reads it, the entry read does not match
		"lookup value": func(tp tableProg, p *Program, r *Run) error {
			r.values[tp.lookups[0]][3].SetUint64(5)
			return p.trace(r)
		},
		// A[5] read once, counted 0; A[6] counted instead
		"entry count": func(_ tableProg, p *Program, r *Run) error {
			cnt := r.Witness(p, tableMod, tableCnt)
			addElem(&cnt[5], -1)
			addElem(&cnt[6], 1)
			return nil
		},
		// the third lookup (A[2]) reads A[3] instead, value and counts
		// moved, its index cell unchanged
		"lookup index": func(_ tableProg, p *Program, r *Run) error {
			moveLookup(p, r, 2, 2, 3)
			return nil
		},
	} {
		t.Run(name, func(t *testing.T) {
			tp := tableProgram(false)
			p, err := tp.m.Compile(testPCSOpts...)
			require.NoError(t, err)
			r, err := p.Execute(tp.in)
			require.NoError(t, err)
			require.NoError(t, tamper(tp, p, r))
			require.Error(t, proveAndVerify(p, r), "tampered lookup accepted")
		})
	}
}

// moveLookup makes the lookup row read the table row to instead of from: its
// index, its value, and both entries' read counts.
func moveLookup(p *Program, r *Run, row, from, to int) {
	addElem(&r.Witness(p, lookupMod, lookupIdx)[row], int64(to-from))
	for j := range CellWidth {
		lane := laneName(lookupV, j)
		r.Witness(p, lookupMod, lane)[row] = r.Witness(p, tableMod, laneName(tableV, j))[to]
	}
	cnt := r.Witness(p, tableMod, tableCnt)
	addElem(&cnt[from], -1)
	addElem(&cnt[to], 1)
}

// TestLookupOutOfTable: a lookup into A at index 8, one past its end, with
// B's first entry as its value and B's count raised: the index cell holds 8
// (an input), so the memory balances, and with consecutive entry addresses
// (A's base + 8 is B's base) the read would balance against B[0]. Tagged by
// its table, it matches no entry.
func TestLookupOutOfTable(t *testing.T) {
	m := &Machine{}
	tp, tabA, _ := tableInputs(m)
	idx := m.InputScalar()
	tp.in = append(tp.in, ScalarCell(field.NewElement(1)))
	m.Lookup(tabA, idx)
	p, err := m.Compile(testPCSOpts...)
	require.NoError(t, err)
	r, err := p.Execute(tp.in)
	require.NoError(t, err)
	require.NoError(t, proveAndVerify(p, r), "honest lookup of A[1]")

	// the index input: the witness chip's last row
	row := len(m.inputs) - 1
	addElem(&r.Witness(p, witnessMod, laneName(witnessV, 0))[row], 7)
	moveLookup(p, r, 0, 1, 8)
	require.Error(t, proveAndVerify(p, r), "A[8] read as B[0]")
}

// TestTablePaddingEntry: a lookup of A[0] returning a forged value, A[0]'s
// read count moved to a padding row of the table module, which writes the
// forged value as the entry (tag 0, idx 0) — its setup is zero, so its source
// read is off. The entries' multiplicity is gated by active: rejected.
func TestTablePaddingEntry(t *testing.T) {
	tp := tableProgram(false)
	p, err := tp.m.Compile(testPCSOpts...)
	require.NoError(t, err)
	r, err := p.Execute(tp.in)
	require.NoError(t, err)

	out := tp.lookups[4] // A[8 & 7] = A[0]
	r.values[out][3].SetUint64(5)
	require.NoError(t, p.trace(r))
	const pad = 12 // A: rows 0..7, B: 8..11
	cnt := r.Witness(p, tableMod, tableCnt)
	addElem(&cnt[0], -1)
	addElem(&cnt[pad], 1)
	for j := range CellWidth {
		r.Witness(p, tableMod, laneName(tableV, j))[pad] = r.values[out][j]
	}
	require.Error(t, proveAndVerify(p, r), "forged entry on a padding row accepted")
}
