package recursion

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/wioptest"
	"github.com/stretchr/testify/require"
)

// testPCSOpts keeps FRI's query count tiny so the tests stay fast: the full
// pipeline runs the PCS. The reduced count must not be used in production.
var testPCSOpts = wioptest.TestingCompileOptions()

// copyMod is the module name of copyChip.
const copyMod = "copy"

// copyChip is a chip for these tests: Copy(src) returns a new cell holding
// src's value, one row per copy, reading src and writing the copy on the bus.
//
// Setup: active, src_addr, dst_addr, dst_mult. Witness: v0..7.
type copyChip struct {
	m      *Machine
	copies *[][2]slot
}

// Copy returns a new cell holding the value of src.
func (c copyChip) Copy(src int) int {
	s, d := c.m.rd(src), c.m.wr()
	*c.copies = append(*c.copies, [2]slot{s, d})
	c.m.recordFunc(func(r *Run) error {
		r.values[d.addr] = r.values[s.addr]
		return nil
	})
	return d.addr
}

func (c copyChip) name() string { return copyMod }
func (c copyChip) rows() int    { return len(*c.copies) }

func (c copyChip) setup(cs *cols) {
	cs.declare("active", "src_addr", "dst_addr", "dst_mult")
	for row, sd := range *c.copies {
		cs.set("active", row, 1)
		cs.set("src_addr", row, uint64(sd[0].addr))
		cs.set("dst_addr", row, uint64(sd[1].addr))
		cs.setInt("dst_mult", row, c.m.mult(sd[1]))
	}
}

func (c copyChip) define(m *module, bus *Bus) error {
	v := cellLanes(m.lanes("v", CellWidth))
	bus.Read(m, m.s("src_addr"), v, m.s("active"))
	bus.Write(m, m.s("dst_addr"), v, m.s("dst_mult"))
	return nil
}

func (c copyChip) trace(cs *cols, r *Run) error {
	cs.declareLanes("v", CellWidth)
	for row, sd := range *c.copies {
		cs.setCell("v", row, r.values[sd[0].addr])
	}
	return nil
}

func withCopy(m *Machine) copyChip {
	c := copyChip{m: m, copies: &[][2]slot{}}
	m.extra = append(m.extra, c)
	return c
}

// cell returns a cell of 8 distinct lanes from a seed.
func cell(seed uint64) Cell {
	var c Cell
	for i := range c {
		c[i].SetUint64(seed*100 + uint64(i))
	}
	return c
}

// busProgram: two inputs and a constant, read through copies — a chain (a
// copy of a copy), a cell read twice (a), a constant read, an input never
// read (u).
func busProgram() (*Machine, []int) {
	m := &Machine{}
	cp := withCopy(m)
	a, b, u := m.InputDigest(), m.InputDigest(), m.InputDigest()
	k := m.Const(cell(9))
	outs := []int{cp.Copy(a), cp.Copy(cp.Copy(b)), cp.Copy(k), cp.Copy(a)}
	_ = u
	return m, outs
}

func proveAndVerify(p *Program, r *Run) error {
	proof, pub := p.Prove(r)
	return p.Verify(proof, pub)
}

// TestMachineBus: one compiled program, executed on two input sets, proves
// and verifies; the copies hold their sources' values.
func TestMachineBus(t *testing.T) {
	m, outs := busProgram()
	p, err := m.Compile(testPCSOpts...)
	require.NoError(t, err)
	require.NotNil(t, p.Bus, "the program accesses the memory")
	for _, seed := range []uint64{1, 5} {
		in := []Cell{cell(seed), cell(seed + 1), cell(seed + 2)}
		r, err := p.Execute(in)
		require.NoError(t, err)
		require.Equal(t, in[0], r.Value(outs[0]), "copy of a")
		require.Equal(t, in[1], r.Value(outs[1]), "copy of a copy of b")
		require.Equal(t, cell(9), r.Value(outs[2]), "copy of the constant")
		require.NoError(t, proveAndVerify(p, r), "honest execution (seed %d)", seed)
	}

	_, err = p.Execute([]Cell{cell(1)})
	require.Error(t, err, "wrong number of inputs")
}

// TestMachineBusRejects: a cell value changed on one side of the bus — the
// writer's (an input, all lanes or the last one) or the reader's — no longer
// balances, and the proof is rejected.
func TestMachineBusRejects(t *testing.T) {
	for name, tamper := range map[string]struct {
		chip, col string
		row       int
	}{
		"input value":           {witnessMod, "v0", 0},
		"input value, lane 7":   {witnessMod, "v7", 1},
		"copied value":          {copyMod, "v3", 2},
		"input read twice (a)":  {witnessMod, "v2", 0},
		"copy of a copy (read)": {copyMod, "v0", 1},
	} {
		t.Run(name, func(t *testing.T) {
			m, _ := busProgram()
			p, err := m.Compile(testPCSOpts...)
			require.NoError(t, err)
			r, err := p.Execute([]Cell{cell(1), cell(2), cell(3)})
			require.NoError(t, err)
			col := r.Witness(p, tamper.chip, tamper.col)
			require.NotNil(t, col, "column %s.%s", tamper.chip, tamper.col)
			var one field.Element
			one.SetOne()
			col[tamper.row].Add(&col[tamper.row], &one)
			require.Error(t, proveAndVerify(p, r), "tampered %s.%s accepted", tamper.chip, tamper.col)
		})
	}
}

// counterChip is a chip for these tests with a constraint on consecutive
// rows: c = 0 on the first row, c[+1] = c + 1 on the others, through a cyclic
// shift — the first row stays constrained (no row is cancelled).
type counterChip struct{ n int }

func (c counterChip) name() string { return "counter" }
func (c counterChip) rows() int    { return c.n }
func (c counterChip) setup(cs *cols) {
	cs.declare("first", "last")
	cs.set("first", 0, 1)
	cs.set("last", cs.n-1, 1)
}

func (c counterChip) define(m *module, _ *Bus) error {
	cnt, first, last := m.w("c"), m.s("first"), m.s("last")
	m.assertZero(wiop.Mul(first, cnt))
	// (1 − last)·(c[+1] − c − 1): at the last row, c[+1] wraps to row 0.
	m.assertZero(wiop.Mul(wiop.Sub(constant(1), last), wiop.Sub(wiop.Sub(m.wShift("c", 1), cnt), constant(1))))
	return nil
}

func (c counterChip) trace(cs *cols, _ *Run) error {
	for i := range cs.n {
		cs.set("c", i, uint64(i))
	}
	return nil
}

// TestMachineConstraints: a chip's constraints, with shifts, hold on every
// row of an honest trace and reject a tampered one, the first row included.
func TestMachineConstraints(t *testing.T) {
	for name, tamperRow := range map[string]int{"honest": -1, "first row": 0, "middle row": 5} {
		t.Run(name, func(t *testing.T) {
			m := &Machine{}
			m.extra = append(m.extra, counterChip{n: 8})
			p, err := m.Compile(testPCSOpts...)
			require.NoError(t, err)
			require.Nil(t, p.Bus, "no memory access")
			r, err := p.Execute(nil)
			require.NoError(t, err)
			if tamperRow < 0 {
				require.NoError(t, proveAndVerify(p, r))
				return
			}
			c := r.Witness(p, "counter", "c")
			c[tamperRow].SetUint64(77)
			require.Error(t, proveAndVerify(p, r), "tampered counter accepted")
		})
	}
}

// TestCompileDeterministic: compiling the same program twice declares the
// same columns, in the same order (their IDs are their positions).
func TestCompileDeterministic(t *testing.T) {
	columns := func() []string {
		m, _ := busProgram()
		p, err := m.Compile(testPCSOpts...)
		require.NoError(t, err)
		var res []string
		for _, mod := range p.modules {
			for _, c := range mod.mod.Columns {
				res = append(res, c.Context.Path())
			}
		}
		return res
	}
	want := columns()
	for range 5 {
		require.Equal(t, want, columns())
	}
}

// kindsProgram: one input of each kind, and an E6 hint (the E6 input's
// lanes, computed during execution), each copied (the copy chip reads all 8
// lanes, so that nothing but the witness chip types them). It returns the
// inputs and the hint's address.
func kindsProgram() (*Machine, []Cell, int) {
	m := &Machine{}
	cp := withCopy(m)
	ins := []int{m.InputScalar(), m.InputE6(), m.InputDigest()}
	for _, a := range ins {
		cp.Copy(a)
	}
	h := m.hint(KindE6)
	m.recordFunc(func(r *Run) error {
		r.values[h] = r.values[ins[1]]
		return nil
	})
	cp.Copy(h)
	c := cell(1)
	var e6 Cell
	copy(e6[:KindE6], c[:KindE6])
	return m, []Cell{ScalarCell(c[0]), e6, c}, h
}

// TestWitnessKinds: typed inputs and hints prove; an input with a nonzero
// lane beyond its kind does not execute, and a prover writing one anyway (or a
// mistyped hint), consistently on the bus, is rejected.
func TestWitnessKinds(t *testing.T) {
	m, in, _ := kindsProgram()
	p, err := m.Compile(testPCSOpts...)
	require.NoError(t, err)
	r, err := p.Execute(in)
	require.NoError(t, err)
	require.NoError(t, proveAndVerify(p, r))

	for name, tamper := range map[string]struct{ input, lane int }{
		"scalar, lane 1": {0, 1},
		"scalar, lane 5": {0, 5},
		"scalar, lane 7": {0, 7},
		"E6, lane 6":     {1, 6},
		"E6, lane 7":     {1, 7},
	} {
		t.Run(name, func(t *testing.T) {
			m, in, _ := kindsProgram()
			p, err := m.Compile(testPCSOpts...)
			require.NoError(t, err)
			bad := append([]Cell(nil), in...)
			bad[tamper.input][tamper.lane].SetUint64(5)
			_, err = p.Execute(bad)
			require.Error(t, err, "mistyped input executed")

			r, err := p.Execute(in)
			require.NoError(t, err)
			r.values[m.inputs[tamper.input].addr] = bad[tamper.input]
			require.NoError(t, p.trace(r))
			require.Error(t, proveAndVerify(p, r), "mistyped input accepted")
		})
	}

	t.Run("E6 hint, lane 6", func(t *testing.T) {
		m, in, h := kindsProgram()
		p, err := m.Compile(testPCSOpts...)
		require.NoError(t, err)
		r, err := p.Execute(in)
		require.NoError(t, err)
		r.values[h][6].SetUint64(5)
		require.NoError(t, p.trace(r))
		require.Error(t, proveAndVerify(p, r), "mistyped hint accepted")
	})
}
