package recursion

import (
	"math/bits"
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/stretchr/testify/require"
)

// deepTerm is one value of a chain, in Horner order (decreasing index).
type deepTerm struct {
	x   field.Ext
	sel bool
}

// deepChainSpec is a chain of the test circuit: its rows' kinds, in order.
// Base rows are full cells, but for those listed in partial (lanes 5..7
// padding).
type deepChainSpec struct {
	ext     []bool
	partial map[int]bool
}

// deepChains covers: segments of both kinds alternating, partial base cells
// mid-chain, Ext segments of 1 (a partial Ext row) and 3 rows (a full and a
// partial one), chains of a single kind, and an Ext-first chain.
var deepChains = []deepChainSpec{
	{ext: []bool{false, false, true, false, false, true, false}, partial: map[int]bool{3: true}},
	{ext: []bool{true, true, true, false}},
	{ext: []bool{false, false}, partial: map[int]bool{1: true}},
	{ext: []bool{true, true, true, true}},
}

// deepCircuit: per chain, an α input, its rows' cells as inputs, and its
// result asserted equal to an input (the chain's expected value), so that a
// forged trace can claim the result it computes.
func deepCircuit(seed uint64) (m *Machine, in []Cell, want []field.Ext) {
	r := rand.New(rand.NewPCG(seed, 22))
	m = &Machine{}
	for _, spec := range deepChains {
		alphaV := randE6(r)
		alpha := m.InputE6()
		in = append(in, E6Cell(alphaV))
		var rows []DeepRow
		var terms []deepTerm
		nExt := 0
		for i, isExt := range spec.ext {
			if isExt {
				x := randE6(r)
				// every third Ext row of a chain is not selected
				var s uint8
				if nExt%3 != 2 {
					s = 1
				}
				nExt++
				rows = append(rows, DeepRow{Cell: m.InputE6(), Ext: true, Act: 1, Sel: s})
				in = append(in, E6Cell(x))
				terms = append(terms, deepTerm{x, s == 1})
				continue
			}
			act := uint8(0xff)
			if spec.partial[i] {
				act = 0b00011111
			}
			sel := uint8(r.Uint32()) & act
			var c Cell
			for j := CellWidth - 1; j >= 0; j-- {
				if act>>j&1 == 0 {
					continue // a padding lane: zero, as a group's last cell
				}
				c[j].SetUint64(r.Uint64())
				var x field.Ext
				x.B0.A0 = c[j]
				terms = append(terms, deepTerm{x, sel>>j&1 == 1})
			}
			rows = append(rows, DeepRow{Cell: m.InputDigest(), Act: act, Sel: sel})
			in = append(in, c)
		}
		// Σ_i α^i·sel_i·x_i, the terms by decreasing index
		var res, pow field.Ext
		pow.SetOne()
		for i := len(terms) - 1; i >= 0; i-- {
			if terms[i].sel {
				var t field.Ext
				t.Mul(&pow, &terms[i].x)
				res.Add(&res, &t)
			}
			pow.Mul(&pow, &alphaV)
		}
		want = append(want, res)
		w := m.InputE6()
		in = append(in, E6Cell(res))
		m.AssertEqE6(m.DeepHorner(alpha, rows), w)
	}
	return m, in, want
}

func TestDeepHorner(t *testing.T) {
	m, in, _ := deepCircuit(1)
	p, err := m.Compile(testPCSOpts...)
	require.NoError(t, err)
	r, err := p.Execute(in)
	require.NoError(t, err)
	require.NoError(t, proveAndVerify(p, r))

	// the rows cover what deepChains claims
	ext, partialBase, partialExt := 0, false, false
	for _, inst := range m.deeps {
		for _, sg := range inst.segs {
			for _, row := range sg.rows {
				if row.Ext {
					ext++
				} else if bits.OnesCount8(row.Act) < CellWidth {
					partialBase = true
				}
			}
			if sg.ext && len(sg.rows)%deepExtLanes != 0 {
				partialExt = true
			}
		}
	}
	require.True(t, ext > 0 && partialBase && partialExt)
}

// TestDeepHornerRejects: statements of chain 0 with one value changed, which
// execute. A selected value, α or the expected result changed is rejected;
// an unselected value, or a padding lane of a partial cell, is not part of
// the sum, and the proof still verifies.
func TestDeepHornerRejects(t *testing.T) {
	m, in, _ := deepCircuit(1)
	p, err := m.Compile(testPCSOpts...)
	require.NoError(t, err)
	one := field.One()
	rows := deepChains[0].ext
	// the input cells of chain 0: α, its rows, the expected result
	type value struct{ cell, lane int }
	var sel, unsel []value
	var selExt value
	k := 0
	for _, sg := range m.deeps[0].segs {
		for _, row := range sg.rows {
			k++
			for j := range CellWidth {
				if row.Act>>j&1 == 0 || (row.Ext && j > 0) {
					continue
				}
				v := value{k, j}
				switch {
				case row.Sel>>j&1 == 0:
					unsel = append(unsel, v)
				case row.Ext:
					selExt = v
				default:
					sel = append(sel, v)
				}
			}
		}
	}
	require.Equal(t, len(rows), k)
	require.True(t, len(sel) > 0 && len(unsel) > 0 && selExt.cell > 0)
	for _, tc := range []struct {
		name   string
		v      value
		reject bool
	}{
		{"α", value{0, 3}, true},
		{"a selected base value", sel[0], true},
		{"the last selected base value", sel[len(sel)-1], true},
		{"a selected Ext value", value{selExt.cell, 4}, true},
		{"expected result", value{len(rows) + 1, 0}, true},
		{"an unselected value", unsel[0], false},
		{"a padding lane", value{4, 7}, false}, // row 3 is partial
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := append([]Cell(nil), in...)
			bad[tc.v.cell][tc.v.lane].Add(&bad[tc.v.cell][tc.v.lane], &one)
			r, err := p.Execute(bad)
			require.NoError(t, err)
			if tc.reject {
				require.Error(t, proveAndVerify(p, r), "tampered chain accepted")
			} else {
				require.NoError(t, proveAndVerify(p, r))
			}
		})
	}
}

// deepForge tampers with one row of a deep instance while retracing.
type deepForge struct {
	mod  string
	row  int
	hin  func(h field.Ext) field.Ext // the h entering the row
	al   func(a field.Ext) field.Ext // the row's α
	x    func(x []field.Element)     // the row's cell lanes (all cells of an Ext row)
	skip int                         // a step not taken (h_j = h_(j+1)), −1 none
}

// deepRetrace recomputes the deep instances' h columns, in chain order, from
// the witness's cells and α, with f's tampering, as a prover would forge
// them; it returns the chains' results.
func deepRetrace(p *Program, r *Run, m *Machine, f deepForge) []field.Ext {
	type inst struct {
		c   deepChip
		s   *cols
		row int
	}
	insts := map[bool]*inst{}
	for _, c := range []deepChip{
		{m: m, mod: deepMod, lanes: CellWidth}, {m: m, mod: deepxMod, lanes: deepExtLanes, ext: true},
	} {
		s := newCols(len(r.Witness(p, c.mod, laneName(deepAl, 0))))
		c.setup(s)
		insts[c.ext] = &inst{c: c, s: s}
	}
	wE6 := func(mod, prefix string, row int) field.Ext {
		var c Cell
		for i := range KindE6 {
			c[i] = r.Witness(p, mod, laneName(prefix, i))[row]
		}
		return CellE6(c)
	}
	setE6 := func(mod, prefix string, row int, x field.Ext) {
		c := E6Cell(x)
		for i := range KindE6 {
			r.Witness(p, mod, laneName(prefix, i))[row] = c[i]
		}
	}
	res := make([]field.Ext, 0, len(m.deeps))
	for _, chain := range m.deeps {
		var h field.Ext
		for _, sg := range chain.segs {
			it := insts[sg.ext]
			c, mod := it.c, it.c.mod
			w := c.cellLanes()
			for range deepPhysRows(sg) {
				row := it.row
				forged := mod == f.mod && row == f.row
				if forged && f.hin != nil {
					h = f.hin(h)
				}
				al := wE6(mod, deepAl, row)
				if forged && f.al != nil {
					al = f.al(al)
				}
				x := make([]field.Element, c.cells()*w)
				for l := range x {
					x[l] = r.Witness(p, mod, laneName(deepX, l))[row]
				}
				if forged && f.x != nil {
					f.x(x)
				}
				setE6(mod, deepHIn, row, h)
				setE6(mod, deepAl, row, al)
				for l := range x {
					r.Witness(p, mod, laneName(deepX, l))[row] = x[l]
				}
				for j := c.lanes - 1; j >= 0; j-- {
					act := it.s.vals[laneName(deepAct, j)][row].IsOne()
					sel := it.s.vals[laneName(deepSel, j)][row].IsOne()
					if act && (!forged || f.skip != j) {
						var t field.Ext
						if sel {
							if c.ext {
								var cell Cell
								copy(cell[:KindE6], x[j*w:(j+1)*w])
								t = CellE6(cell)
							} else {
								t.B0.A0 = x[j]
							}
						}
						h.Mul(&h, &al)
						h.Add(&h, &t)
					}
					setE6(mod, deepHPrefix(j), row, h)
				}
				it.row++
			}
		}
		res = append(res, h)
	}
	return res
}

// deepForged forges a trace with f: it retraces the chains, re-executes with
// the forged results as the expected values (the inputs after each chain's
// rows), retraces again, and sets the asserting E6 rows (a = o = the result).
func deepForged(t *testing.T, f deepForge) error {
	t.Helper()
	m, in, want := deepCircuit(1)
	p, err := m.Compile(testPCSOpts...)
	require.NoError(t, err)
	r, err := p.Execute(in)
	require.NoError(t, err)
	got := deepRetrace(p, r, m, f)
	changed := false
	k := 0
	for c := range deepChains {
		k += 1 + len(deepChains[c].ext)
		in[k] = E6Cell(got[c])
		k++
		changed = changed || !got[c].Equal(&want[c])
	}
	require.True(t, changed, "the forgery changes no result")
	r, err = p.Execute(in)
	require.NoError(t, err)
	deepRetrace(p, r, m, f)
	for c, v := range got {
		cell := E6Cell(v)
		for i := range KindE6 {
			r.Witness(p, e6Mod, laneName(e6A, i))[c] = cell[i]
			r.Witness(p, e6Mod, laneName(e6O, i))[c] = cell[i]
		}
	}
	return proveAndVerify(p, r)
}

// TestDeepHornerRejectsForged: forged traces, each breaking a single check of
// the chips (each a check of both instances, forged on each):
//   - a step not taken: the step's constraint;
//   - α, or a cell's value, other than the cell's: its read;
//   - the h entering a row other than the previous row's: the chaining;
//   - a chain starting from a nonzero h: the start constraint;
//   - a continuing segment starting from another h than the handed-over cell:
//     the cell's read.
//
// Rows: base instance, chain 0 is rows 0, 1 | 2, 3 | 4 (segments), chain 1 is
// row 5, chain 2 rows 6, 7; Ext instance, chain 0 is rows 0 | 1, chain 1 rows
// 2, 3, chain 3 rows 4, 5.
func TestDeepHornerRejectsForged(t *testing.T) {
	one := field.One()
	bump := func(h field.Ext) field.Ext {
		h.B1.A0.Add(&h.B1.A0, &one)
		return h
	}
	none := -1
	bumpAll := func(x []field.Element) {
		for i := range x {
			x[i].Add(&x[i], &one)
		}
	}
	for name, f := range map[string]deepForge{
		"base step":        {mod: deepMod, row: 7, skip: 2},
		"ext step":         {mod: deepxMod, row: 4, skip: 0},
		"base α":           {mod: deepMod, row: 1, al: bump, skip: none},
		"ext α":            {mod: deepxMod, row: 3, al: bump, skip: none},
		"base value":       {mod: deepMod, row: 6, x: bumpAll, skip: none},
		"ext value":        {mod: deepxMod, row: 2, x: bumpAll, skip: none},
		"base chaining":    {mod: deepMod, row: 7, hin: bump, skip: none},
		"ext chaining":     {mod: deepxMod, row: 5, hin: bump, skip: none},
		"base chain start": {mod: deepMod, row: 6, hin: bump, skip: none},
		"ext chain start":  {mod: deepxMod, row: 4, hin: bump, skip: none},
		"base handover":    {mod: deepMod, row: 2, hin: bump, skip: none},
		"ext handover":     {mod: deepxMod, row: 1, hin: bump, skip: none},
	} {
		t.Run(name, func(t *testing.T) {
			require.Error(t, deepForged(t, f), "forged chain accepted")
		})
	}
	// the forgery machinery itself: an untampered retrace verifies
	t.Run("honest retrace", func(t *testing.T) {
		m, in, _ := deepCircuit(1)
		p, err := m.Compile(testPCSOpts...)
		require.NoError(t, err)
		r, err := p.Execute(in)
		require.NoError(t, err)
		deepRetrace(p, r, m, deepForge{skip: none})
		require.NoError(t, proveAndVerify(p, r))
	})
}
