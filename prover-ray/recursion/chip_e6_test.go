package recursion

import (
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/stretchr/testify/require"
)

func randE6(r *rand.Rand) field.Ext {
	var x field.Ext
	for _, c := range []*field.Element{&x.B0.A0, &x.B0.A1, &x.B1.A0, &x.B1.A1, &x.B2.A0, &x.B2.A1} {
		c.SetUint64(r.Uint64())
	}
	return x
}

// e6Ops are the cells e6Program computes, to tamper with.
type e6Ops struct {
	mul, div, horner int
	hornerRow        int // an intermediate row of the 3-coefficient Horner
}

// e6Program runs every E6 operation on inputs x, y, z and the scalar 7, and
// asserts the results equal constants computed in Go. It returns the program,
// its inputs and the computed cells.
func e6Program() (*Machine, []Cell, e6Ops) {
	r := rand.New(rand.NewPCG(1, 2))
	m := &Machine{}
	x, y, z := randE6(r), randE6(r), randE6(r)
	s := field.NewElement(7)
	in := []Cell{E6Cell(x), E6Cell(y), E6Cell(z), ScalarCell(s)}
	a, b, c, sc := m.InputE6(), m.InputE6(), m.InputE6(), m.InputScalar()
	var ops e6Ops

	var want field.Ext
	want.Mul(&x, &y)
	ops.mul = m.Mul(a, b)
	m.AssertEqE6(ops.mul, m.Const(E6Cell(want)))
	want.Add(&x, &y)
	m.AssertEqE6(m.Add(a, b), m.Const(E6Cell(want)))
	want.Sub(&x, &y)
	m.AssertEqE6(m.Sub(a, b), m.Const(E6Cell(want)))
	want.Div(&x, &y)
	ops.div = m.Div(a, b)
	m.AssertEqE6(ops.div, m.Const(E6Cell(want)))

	// x·z² + y·z + z, and a one-coefficient Horner
	want.Mul(&x, &z)
	want.Add(&want, &y)
	want.Mul(&want, &z)
	want.Add(&want, &z)
	ops.hornerRow = len(m.e6Rows()) + 1
	ops.horner = m.Horner(c, []int{a, b, c})
	m.AssertEqE6(ops.horner, m.Const(E6Cell(want)))
	m.AssertEqE6(m.Horner(c, []int{a}), m.Const(E6Cell(x)))

	// a base scalar is its E6 embedding
	want.MulByElement(&x, &s)
	m.AssertEqE6(m.Mul(a, sc), m.Const(E6Cell(want)))

	// x^(2^0 + 2^2) by bits 1, 0, 1 of g = x0 (a scalar)
	bits := []int{m.Const(ScalarCell(field.One())), m.Const(Cell{}), m.Const(ScalarCell(field.One()))}
	g := x.B0.A0
	var g2, g4, w field.Element
	g2.Square(&g)
	g4.Square(&g2)
	w.Mul(&g, &g4)
	m.AssertEqE6(m.PowBits(bits, []field.Element{g, g2, g4}), m.Const(ScalarCell(w)))
	return m, in, ops
}

// e6Rows returns the E6 chip's rows, in order.
func (m *Machine) e6Rows() []e6Row {
	var rows []e6Row
	for _, in := range m.e6Insts {
		rows = append(rows, in.rows...)
	}
	return rows
}

// cheatE6 makes the prover claim a wrong value for the cell addr (lane 2
// plus one), consistently: the cell's value, every E6 row reading it, and
// every E6 row's o, set to its cell's value. The bus balances; the E6
// constraints must reject.
func cheatE6(p *Program, r *Run, addr int) error {
	var one field.Element
	one.SetOne()
	r.values[addr][2].Add(&r.values[addr][2], &one)
	if err := p.trace(r); err != nil {
		return err
	}
	o := make([][]field.Element, KindE6)
	for i := range o {
		o[i] = r.Witness(p, e6Mod, laneName(e6O, i))
	}
	for row, er := range p.m.e6Rows() {
		if er.o.role == roleNone {
			continue
		}
		for i := range KindE6 {
			o[i][row] = r.values[er.o.addr][i]
		}
	}
	return nil
}

// TestE6Chip: every E6 operation agrees with Go, and proves.
func TestE6Chip(t *testing.T) {
	m, in, _ := e6Program()
	p, err := m.Compile(testPCSOpts...)
	require.NoError(t, err)
	r, err := p.Execute(in)
	require.NoError(t, err)
	require.NoError(t, proveAndVerify(p, r))
}

// TestE6ChipRejects: a wrong result, claimed consistently on the bus, or a
// false assertion, is rejected.
func TestE6ChipRejects(t *testing.T) {
	for name, tamper := range map[string]func(p *Program, r *Run, ops e6Ops) error{
		"product":  func(p *Program, r *Run, ops e6Ops) error { return cheatE6(p, r, ops.mul) },
		"quotient": func(p *Program, r *Run, ops e6Ops) error { return cheatE6(p, r, ops.div) },
		"horner":   func(p *Program, r *Run, ops e6Ops) error { return cheatE6(p, r, ops.horner) },
		// an intermediate Horner value, which no cell holds
		"horner step": func(p *Program, r *Run, ops e6Ops) error {
			r.Witness(p, e6Mod, laneName(e6O, 1))[ops.hornerRow].SetUint64(5)
			return nil
		},
	} {
		t.Run(name, func(t *testing.T) {
			m, in, ops := e6Program()
			p, err := m.Compile(testPCSOpts...)
			require.NoError(t, err)
			r, err := p.Execute(in)
			require.NoError(t, err)
			require.NoError(t, tamper(p, r, ops))
			require.Error(t, proveAndVerify(p, r), "tampered trace accepted")
		})
	}

	t.Run("wrong claim", func(t *testing.T) {
		m := &Machine{}
		m.AssertEqE6(m.InputE6(), m.InputE6())
		p, err := m.Compile(testPCSOpts...)
		require.NoError(t, err)
		rnd := rand.New(rand.NewPCG(3, 4))
		r, err := p.Execute([]Cell{E6Cell(randE6(rnd)), E6Cell(randE6(rnd))})
		require.NoError(t, err)
		require.Error(t, proveAndVerify(p, r), "x = y accepted")
	})
	t.Run("lane 6", func(t *testing.T) {
		// E6 operands have zero lanes 6 and 7: a digest with lane 6 set is
		// not an E6 value, and an E6 read of it is rejected
		m := &Machine{}
		a := m.InputDigest()
		m.AssertEqE6(a, a)
		p, err := m.Compile(testPCSOpts...)
		require.NoError(t, err)
		var c Cell
		c[6].SetOne()
		r, err := p.Execute([]Cell{c})
		require.NoError(t, err)
		require.Error(t, proveAndVerify(p, r), "lane 6 accepted")
	})
	t.Run("zero divisor", func(t *testing.T) {
		m := &Machine{}
		m.Div(m.InputE6(), m.InputE6())
		p, err := m.Compile(testPCSOpts...)
		require.NoError(t, err)
		_, err = p.Execute([]Cell{{}, {}})
		require.Error(t, err)
	})
}

// TestScalarLanesBound is the reviewer's attack on untyped scalars: a scalar
// s, bound to the transcript as a scalar (packed, so through lane 0 only),
// used in E6 arithmetic, where its lanes 1..5 count. The program claims
// v·e = w for w ≠ v·e; the prover picks s' = (v, t1..t5), whose lane 0 is
// still v, and w = s'·e. Every hash, every E6 row and the bus are consistent.
// Two checks reject it, each on its own: the witness chip's typing of s (a
// scalar input), and the strict form of s's window, which reads it zero
// beyond the packed kind. With both disabled, the false claim verifies.
func TestScalarLanesBound(t *testing.T) {
	rnd := rand.New(rand.NewPCG(7, 8))
	v, e := field.NewElement(11), randE6(rnd)
	var sp field.Ext // s' = (v, t1..t5)
	sp.B0.A0 = v
	sp.B0.A1.SetUint64(1)
	sp.B2.A1.SetUint64(2)
	var w field.Ext
	w.Mul(&sp, &e)

	m := &Machine{}
	s, ein := m.InputScalar(), m.InputE6()
	// s is the transcript's v: its single block, left-padded, holds it in
	// lane 7
	block := m.Pack([]int{s}, []int{KindScalar})[0]
	m.AssertEqE6(m.Lanes(block, CellWidth-1, KindScalar), m.Const(ScalarCell(v)))
	mul := m.Mul(s, ein)
	m.AssertEqE6(mul, m.Const(E6Cell(w)))
	p, err := m.Compile(testPCSOpts...)
	require.NoError(t, err)

	in := []Cell{ScalarCell(v), E6Cell(e)}
	r, err := p.Execute(in)
	require.NoError(t, err)
	require.Error(t, proveAndVerify(p, r), "v·e = w accepted, honestly")

	_, err = p.Execute([]Cell{E6Cell(sp), E6Cell(e)})
	require.Error(t, err, "a scalar input with lanes 1..5 executed")

	// the cheat: s' instead of s, its product, the stream unchanged (lane 0)
	r.values[s] = E6Cell(sp)
	r.values[mul] = E6Cell(w)
	require.NoError(t, p.trace(r))
	require.Error(t, proveAndVerify(p, r), "v·e = w accepted, with s' = (v, t1..t5)")
}

// chainProgram computes the partial sums of init + Σ as[i]·bs[i] (k = 5
// steps, E6 factors, one scalar) with MulAddChain, between two other E6
// operations, and asserts the last sum equals Go's. It returns the program,
// its inputs and the partial sums' cells.
func chainProgram() (*Machine, []Cell, []int) {
	r := rand.New(rand.NewPCG(5, 6))
	const k = 5
	m := &Machine{}
	vals := make([]field.Ext, 1+2*k)
	in := make([]Cell, len(vals))
	cells := make([]int, len(vals))
	for i := range vals {
		if i == k { // as[k−1]: a scalar
			vals[i].B0.A0.SetUint64(7)
			cells[i] = m.InputScalar()
		} else {
			vals[i] = randE6(r)
			cells[i] = m.InputE6()
		}
		in[i] = E6Cell(vals[i])
	}
	m.Mul(cells[0], cells[1]) // a row before the chain
	sums := m.MulAddChain(cells[0], cells[1:1+k], cells[1+k:])
	m.Mul(sums[2], cells[2]) // and one after, reading a partial sum
	want := vals[0]
	for i := range k {
		var t field.Ext
		t.Mul(&vals[1+i], &vals[1+k+i])
		want.Add(&want, &t)
	}
	m.AssertEqE6(sums[k-1], m.Const(E6Cell(want)))
	return m, in, sums
}

// TestMulAddChain: the partial sums agree with Go and prove, in k + 1 rows.
func TestMulAddChain(t *testing.T) {
	m, in, sums := chainProgram()
	require.Len(t, m.e6Rows(), 1+(len(sums)+1)+1+1, "Mul, the chain (k + 1 rows), Mul, AssertEqE6")
	p, err := m.Compile(testPCSOpts...)
	require.NoError(t, err)
	r, err := p.Execute(in)
	require.NoError(t, err)
	s := CellE6(in[0])
	for i := range sums {
		x, y := CellE6(in[1+i]), CellE6(in[1+len(sums)+i])
		x.Mul(&x, &y)
		s.Add(&s, &x)
		require.Equal(t, E6Cell(s), r.Value(sums[i]), "s_%d", i+1)
	}
	require.NoError(t, proveAndVerify(p, r))
}

// TestMulAddChainRejects: a wrong partial sum, claimed consistently on the
// bus, or a wrong copy of init (which no cell holds), is rejected.
func TestMulAddChainRejects(t *testing.T) {
	for name, tamper := range map[string]func(p *Program, r *Run, sums []int) error{
		"partial sum": func(p *Program, r *Run, sums []int) error { return cheatE6(p, r, sums[1]) },
		"last sum":    func(p *Program, r *Run, sums []int) error { return cheatE6(p, r, sums[len(sums)-1]) },
		// the chain's first row (the row after the first Mul), its copy of init
		"copy of init": func(p *Program, r *Run, _ []int) error {
			r.Witness(p, e6Mod, laneName(e6O, 4))[1].SetUint64(5)
			return nil
		},
	} {
		t.Run(name, func(t *testing.T) {
			m, in, sums := chainProgram()
			p, err := m.Compile(testPCSOpts...)
			require.NoError(t, err)
			r, err := p.Execute(in)
			require.NoError(t, err)
			require.NoError(t, tamper(p, r, sums))
			require.Error(t, proveAndVerify(p, r), "tampered chain accepted")
		})
	}
}

// TestE6Limits pins the documented limits of Div and AssertEqE6.
func TestE6Limits(t *testing.T) {
	// Div binds q only if d ≠ 0: with n = d = 0 (which Execute refuses, but a
	// prover can write), any q verifies
	t.Run("Div by zero", func(t *testing.T) {
		m := &Machine{}
		n, d := m.InputE6(), m.InputE6()
		q := m.Div(n, d)
		p, err := m.Compile(testPCSOpts...)
		require.NoError(t, err)
		_, err = p.Execute([]Cell{{}, {}})
		require.Error(t, err, "a zero divisor executed")

		rnd := rand.New(rand.NewPCG(9, 10))
		r, err := p.Execute([]Cell{E6Cell(randE6(rnd)), E6Cell(randE6(rnd))})
		require.NoError(t, err)
		r.values[n], r.values[d] = Cell{}, Cell{}
		r.values[q] = E6Cell(randE6(rnd))
		require.NoError(t, p.trace(r))
		require.NoError(t, proveAndVerify(p, r), "0/0 = q, for any q")
	})
	// AssertEqE6 compares E6 values: two equal digests with nonzero lanes 6,
	// 7 cannot be proven equal
	t.Run("AssertEqE6 of digests", func(t *testing.T) {
		m := &Machine{}
		m.AssertEqE6(m.InputDigest(), m.InputDigest())
		p, err := m.Compile(testPCSOpts...)
		require.NoError(t, err)
		r, err := p.Execute([]Cell{cell(1), cell(1)})
		require.NoError(t, err)
		require.Error(t, proveAndVerify(p, r), "equal digests, read as E6 values")
	})
}
