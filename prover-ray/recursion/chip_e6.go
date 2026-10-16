package recursion

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// Setup columns of the e6 chip.
const (
	e6Mul = "mul"
	e6Add = "add"
	e6Bs  = "bs"
	e6Hor = "hor"
	e6Acc = "acc"
	// Per operand x ∈ {a, b, o}: addr_x and m_x (e6Addr, e6Mult).
	e6AddrPrefix = "addr_"
	e6MultPrefix = "m_"
)

// e6Operands are the operands of an E6 chip row.
var e6Operands = []string{e6A, e6B, e6O}

// e6Addr and e6Mult name the address and multiplicity columns of operand x.
func e6Addr(x string) string { return e6AddrPrefix + x }
func e6Mult(x string) string { return e6MultPrefix + x }

// Witness columns of the e6 chip.
const (
	e6A = "a" // lanes 0..5
	e6B = "b" // lanes 0..5
	e6O = "o" // lanes 0..5
)

// e6Row is one row of the E6 chip:
//
//	o = mul·(a·b) + add·a + bs·b + hor·(o[−1]·a) + acc·o[−1]
//
// Unused operands are zero; o is computed by the formula, so on a row whose o
// is read the chip checks the read value. The rows using o[−1] (hor, acc)
// follow the row of their previous value: an instruction's rows are
// contiguous.
type e6Row struct {
	mul, add, bs, hor, acc int64
	a, b, o                slot
}

// e6Inst is one E6 instruction: its kind, operands, written cell and rows
// (one, or one per coefficient for Horner, or one per step plus one for
// MulAddChain, whose rows write their own cells).
type e6Inst struct {
	kind int
	args []int // operand addresses
	out  int   // the written cell, or -1
	rows []e6Row
}

const (
	opMul = iota
	opAdd
	opSub
	opDiv
	opEq
	opHorner      // args = y, coeffs...
	opMulAddChain // args = init, as..., bs...
)

func (m *Machine) addE6(kind int, args []int, out int, rows ...e6Row) int {
	m.e6Insts = append(m.e6Insts, e6Inst{kind: kind, args: args, out: out, rows: rows})
	m.record(instrE6, len(m.e6Insts)-1)
	return out
}

// Mul returns a·b.
func (m *Machine) Mul(a, b int) int {
	r := e6Row{mul: 1, a: m.rd(a), b: m.rd(b), o: m.wr()}
	return m.addE6(opMul, []int{a, b}, r.o.addr, r)
}

// Add returns a + b.
func (m *Machine) Add(a, b int) int {
	r := e6Row{add: 1, bs: 1, a: m.rd(a), b: m.rd(b), o: m.wr()}
	return m.addE6(opAdd, []int{a, b}, r.o.addr, r)
}

// Sub returns a − b.
func (m *Machine) Sub(a, b int) int {
	r := e6Row{add: 1, bs: -1, a: m.rd(a), b: m.rd(b), o: m.wr()}
	return m.addE6(opSub, []int{a, b}, r.o.addr, r)
}

// Div returns n/d, checked as d·q = n. Executing it fails if d is zero.
func (m *Machine) Div(n, d int) int {
	r := e6Row{mul: 1, a: m.rd(d), b: m.wr(), o: m.rd(n)}
	return m.addE6(opDiv, []int{n, d}, r.b.addr, r)
}

// AssertEq checks that the cells a and b hold the same E6 value.
func (m *Machine) AssertEq(a, b int) {
	m.addE6(opEq, []int{a, b}, -1, e6Row{add: 1, a: m.rd(a), o: m.rd(b)})
}

// Horner returns Σ_i coeffs[i]·y^(len−1−i), one row per coefficient: the
// first row is o = coeffs[0], the next ones o = o[−1]·y + coeffs[i].
func (m *Machine) Horner(y int, coeffs []int) int {
	if len(coeffs) == 0 {
		panic("Horner: no coefficients")
	}
	rows := make([]e6Row, len(coeffs))
	for i, c := range coeffs {
		rows[i] = e6Row{bs: 1, b: m.rd(c)}
		if i > 0 {
			rows[i].hor, rows[i].a = 1, m.rd(y)
		}
	}
	rows[len(rows)-1].o = m.wr()
	return m.addE6(opHorner, append([]int{y}, coeffs...), rows[len(rows)-1].o.addr, rows...)
}

// MulAddChain returns the partial sums s_1..s_k of s_0 = init,
// s_(i+1) = s_i + as[i]·bs[i]: a γ-batching, a weighted sum, or a random
// linear combination. It takes k + 1 rows (a copy of init, then one row per
// step, o = o[−1] + a·b), where Add(acc, Mul(γ, x)) takes 2 per step.
func (m *Machine) MulAddChain(init int, as, bs []int) []int {
	if len(as) != len(bs) || len(as) == 0 {
		panic(fmt.Sprintf("MulAddChain: %d and %d factors", len(as), len(bs)))
	}
	rows := make([]e6Row, 1+len(as))
	rows[0] = e6Row{add: 1, a: m.rd(init)}
	outs := make([]int, len(as))
	for i := range as {
		rows[1+i] = e6Row{mul: 1, acc: 1, a: m.rd(as[i]), b: m.rd(bs[i]), o: m.wr()}
		outs[i] = rows[1+i].o.addr
	}
	args := append(append([]int{init}, as...), bs...)
	m.addE6(opMulAddChain, args, -1, rows...)
	return outs
}

// PowBits returns Π_i (bits[i] ? gs[i] : 1), for scalar bit cells and
// constants gs: an exponentiation by bits, such as g^bitrev(index).
func (m *Machine) PowBits(bits []int, gs []field.Element) int {
	if len(bits) != len(gs) {
		panic(fmt.Sprintf("PowBits: %d bits, %d constants", len(bits), len(gs)))
	}
	one := field.One()
	acc := m.Const(ScalarCell(one))
	for i, b := range bits {
		var gm1 field.Element
		gm1.Sub(&gs[i], &one)
		f := m.Add(m.Mul(b, m.Const(ScalarCell(gm1))), m.Const(ScalarCell(one)))
		acc = m.Mul(acc, f)
	}
	return acc
}

func (m *Machine) execE6(op e6Inst, r *Run) error {
	v := func(i int) field.Ext { return CellE6(r.values[op.args[i]]) }
	var z field.Ext
	switch op.kind {
	case opMul:
		x, y := v(0), v(1)
		z.Mul(&x, &y)
	case opAdd:
		x, y := v(0), v(1)
		z.Add(&x, &y)
	case opSub:
		x, y := v(0), v(1)
		z.Sub(&x, &y)
	case opDiv:
		n, d := v(0), v(1)
		if d.IsZero() {
			return fmt.Errorf("Div: zero divisor (cell %d)", op.args[1])
		}
		z.Inverse(&d)
		z.Mul(&z, &n)
	case opEq:
		return nil
	case opMulAddChain:
		k := (len(op.args) - 1) / 2
		z = v(0)
		for i := range k {
			x, y := v(1+i), v(1+k+i)
			x.Mul(&x, &y)
			z.Add(&z, &x)
			r.values[op.rows[1+i].o.addr] = E6Cell(z)
		}
		return nil
	case opHorner:
		y := v(0)
		for i := 1; i < len(op.args); i++ {
			c := v(i)
			z.Mul(&z, &y)
			z.Add(&z, &c)
		}
	}
	r.values[op.out] = E6Cell(z)
	return nil
}

// e6Chip: one E6 operation per row,
//
//	o = mul·(a·b) + add·a + bs·b + hor·(o[−1]·a) + acc·o[−1],
//
// over the columns a0..5, b0..5, o0..5. Setup: mul, add, bs (−1, 0 or 1), hor,
// acc (accumulate: o[−1] + a·b, a chain step), and per operand x ∈ {a, b, o} addr_x and the signed multiplicity m_x (−1
// read, the read count for a write, 0 unused). Bus: one access per operand,
// with lanes 6 and 7 zero.
type e6Chip struct{ m *Machine }

func (c e6Chip) name() string { return e6Mod }

func (c e6Chip) rows() int {
	n := 0
	for _, in := range c.m.e6Insts {
		n += len(in.rows)
	}
	return n
}

func (c e6Chip) setup(cs *cols) {
	cs.declare(e6Mul, e6Add, e6Bs, e6Hor, e6Acc)
	for _, x := range e6Operands {
		cs.declare(e6Addr(x), e6Mult(x))
	}
	row := -1
	for _, in := range c.m.e6Insts {
		for _, r := range in.rows {
			row++
			c.setupRow(cs, row, r)
		}
	}
}

func (c e6Chip) setupRow(cs *cols, row int, r e6Row) {
	cs.setInt(e6Mul, row, r.mul)
	cs.setInt(e6Add, row, r.add)
	cs.setInt(e6Bs, row, r.bs)
	cs.setInt(e6Hor, row, r.hor)
	cs.setInt(e6Acc, row, r.acc)
	for _, op := range []struct {
		name string
		s    slot
	}{{e6A, r.a}, {e6B, r.b}, {e6O, r.o}} {
		if op.s.role != roleNone {
			cs.set(e6Addr(op.name), row, uint64(op.s.addr))
			cs.setInt(e6Mult(op.name), row, c.m.mult(op.s))
		}
	}
}

func (c e6Chip) define(m *module, bus *Bus) error {
	a, b, o := m.lanes(e6A, KindE6), m.lanes(e6B, KindE6), m.lanes(e6O, KindE6)
	prevO := make([]wiop.Expression, KindE6)
	for i := range prevO {
		prevO[i] = m.wShift(laneName(e6O, i), -1)
	}
	ab, prevA := e6MulExprs(a, b), e6MulExprs(prevO, a)
	mul, add, bs, hor, acc := m.s(e6Mul), m.s(e6Add), m.s(e6Bs), m.s(e6Hor), m.s(e6Acc)
	for i := range KindE6 {
		rhs := sum(wiop.Mul(mul, ab[i]), wiop.Mul(add, a[i]), wiop.Mul(bs, b[i]),
			wiop.Mul(hor, prevA[i]), wiop.Mul(acc, prevO[i]))
		m.assertZero(wiop.Sub(o[i], rhs))
	}
	for _, x := range e6Operands {
		bus.Write(m, m.s(e6Addr(x)), cellOf(m.lanes(x, KindE6)...), m.s(e6Mult(x)))
	}
	return nil
}

// trace reads a and b from their cells (zero when unused) and computes o by
// the row's formula: on a row whose o is read, the bus checks it.
func (c e6Chip) trace(cs *cols, r *Run) error {
	for _, x := range e6Operands {
		cs.declareLanes(x, KindE6)
	}
	var prev field.Ext
	row := -1
	for _, in := range c.m.e6Insts {
		for _, er := range in.rows {
			row++
			prev = traceE6Row(cs, row, er, prev, r)
		}
	}
	return nil
}

// traceE6Row fills one row and returns its o, the next row's o[−1].
func traceE6Row(cs *cols, row int, er e6Row, prev field.Ext, r *Run) field.Ext {
	var a, b, o field.Ext
	if er.a.role != roleNone {
		a = CellE6(r.values[er.a.addr])
	}
	if er.b.role != roleNone {
		b = CellE6(r.values[er.b.addr])
	}
	var t field.Ext
	if er.mul != 0 {
		t.Mul(&a, &b)
		o.Add(&o, &t)
	}
	if er.add != 0 {
		o.Add(&o, &a)
	}
	switch er.bs {
	case 1:
		o.Add(&o, &b)
	case -1:
		o.Sub(&o, &b)
	}
	if er.hor != 0 {
		t.Mul(&prev, &a)
		o.Add(&o, &t)
	}
	if er.acc != 0 {
		o.Add(&o, &prev)
	}
	cs.setE6(e6A, row, a)
	cs.setE6(e6B, row, b)
	cs.setE6(e6O, row, o)
	return o
}
