package recursion

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// Setup columns of the bits chip.
const (
	bitsFirst  = "first"
	bitsLast   = "last"
	bitsActive = "active"
	bitsPow    = "pow"
	bitsInlow  = "inlow"
	bitsChk    = "chk"
	bitsAddrV  = "addr_v"
	bitsRAddr  = "r_addr"
	bitsRMult  = "r_mult"
	bitsBAddr  = "b_addr"
	bitsBMult  = "b_mult"
)

// Witness columns of the bits chip.
const (
	bitsB   = "b"
	bitsW   = "w"
	bitsR   = "r"
	bitsV   = "v"
	bitsInv = "inv"
)

// BitsWidth is the number of bits of a canonical KoalaBear element, and the
// number of rows of a decomposition.
const BitsWidth = 31

// Bits is the decomposition of a scalar v: for i < n, Shr[i] is the scalar
// cell (v mod 2^n) >> i and Bit[i] the scalar cell of bit i of v.
type Bits struct {
	Shr, Bit []int
}

type bitsInst struct {
	v, n int
	out  Bits
}

// Bits decomposes the scalar cell v canonically (the bits are those of the
// representative in [0, p)) and returns its low n bits, n < BitsWidth.
func (m *Machine) Bits(v int, n int) Bits {
	if n < 1 || n >= BitsWidth {
		panic(fmt.Sprintf("Bits: n = %d", n))
	}
	m.read(v)
	var out Bits
	for range n {
		out.Shr = append(out.Shr, m.alloc())
		out.Bit = append(out.Bit, m.alloc())
	}
	m.bits = append(m.bits, bitsInst{v: v, n: n, out: out})
	m.record(instrBits, len(m.bits)-1)
	return out
}

func (m *Machine) execBits(b bitsInst, r *Run) {
	x := r.values[b.v][0].Uint64()
	low := x & (1<<b.n - 1)
	for i := range b.n {
		r.values[b.out.Shr[i]] = ScalarCell(field.NewElement(low >> i))
		r.values[b.out.Bit[i]] = ScalarCell(field.NewElement(x >> i & 1))
	}
}

// koalaHigh is the value of the top 7 bits (24..30) of p − 1 = 127·2^24: a
// 31-bit integer is below p iff its top 7 bits are not all set, or its low 24
// bits are zero.
const koalaHigh = 127

// bitsChip: BitsWidth rows per decomposition, row i for bit i.
//
// Witness: b (bit i), w (the low bits, Σ_(k<i) b_k·2^k), r ((v mod 2^n) >> i,
// zero for i ≥ n), v (the decomposed value), inv (at row 24, the inverse of
// 127 − (v − w)/2^24 if nonzero). Setup: first, last, active, pow (2^i), inlow
// (i < n), chk (i = 24), addr_v (read on the first row), r_addr, r_mult,
// b_addr, b_mult (the writes of r and b).
//
//	b·(b − 1) = 0
//	w = 0                          on the first row
//	w[+1] = w + b·pow, v[+1] = v   on every row but the last
//	v = w + b·pow                  on the last row
//	r = inlow·(b + 2·r[+1])
//	chk·w·(1 − z·inv) = 0,  z = 127 − (v − w)/2^24   (canonical: v < p)
type bitsChip struct{ m *Machine }

func (c bitsChip) name() string { return bitsMod }

func (c bitsChip) rows() int { return BitsWidth * len(c.m.bits) }

func (c bitsChip) setup(cs *cols) {
	cs.declare(bitsFirst, bitsLast, bitsActive, bitsPow, bitsInlow, bitsChk,
		bitsAddrV, bitsRAddr, bitsRMult, bitsBAddr, bitsBMult)
	for k, inst := range c.m.bits {
		for i := range BitsWidth {
			row := k*BitsWidth + i
			cs.set(bitsActive, row, 1)
			cs.set(bitsPow, row, 1<<i)
			switch i {
			case 0:
				cs.set(bitsFirst, row, 1)
				cs.set(bitsAddrV, row, uint64(inst.v))
			case 24:
				cs.set(bitsChk, row, 1)
			case BitsWidth - 1:
				cs.set(bitsLast, row, 1)
			}
			if i < inst.n {
				cs.set(bitsInlow, row, 1)
				cs.set(bitsRAddr, row, uint64(inst.out.Shr[i]))
				cs.set(bitsRMult, row, uint64(c.m.reads[inst.out.Shr[i]]))
				cs.set(bitsBAddr, row, uint64(inst.out.Bit[i]))
				cs.set(bitsBMult, row, uint64(c.m.reads[inst.out.Bit[i]]))
			}
		}
	}
}

func (c bitsChip) define(m *module, bus *Bus) error {
	first, last, active := m.s(bitsFirst), m.s(bitsLast), m.s(bitsActive)
	bit, w, r, v, inv := m.w(bitsB), m.w(bitsW), m.w(bitsR), m.w(bitsV), m.w(bitsInv)
	pow := m.s(bitsPow)
	notLast := wiop.Sub(active, last)
	next := wiop.Add(w, wiop.Mul(bit, pow)) // w + b·pow

	m.assertZero(wiop.Mul(bit, wiop.Sub(bit, constant(1))))
	m.assertZero(wiop.Mul(first, w))
	m.assertZero(wiop.Mul(notLast, wiop.Sub(m.wShift(bitsW, 1), next)))
	m.assertZero(wiop.Mul(notLast, wiop.Sub(m.wShift(bitsV, 1), v)))
	m.assertZero(wiop.Mul(last, wiop.Sub(v, next)))
	m.assertZero(wiop.Sub(r, wiop.Mul(m.s(bitsInlow), wiop.Add(bit, wiop.Mul(constant(2), m.wShift(bitsR, 1))))))
	var inv24 field.Element
	inv24.SetUint64(1 << 24)
	inv24.Inverse(&inv24)
	z := wiop.Sub(constant(koalaHigh), wiop.Mul(wiop.Sub(v, w), elemConst(inv24)))
	m.assertZero(prod(m.s(bitsChk), w, wiop.Sub(constant(1), wiop.Mul(z, inv))))

	bus.Read(m, m.s(bitsAddrV), cellOf(v), first)
	bus.Write(m, m.s(bitsRAddr), cellOf(r), m.s(bitsRMult))
	bus.Write(m, m.s(bitsBAddr), cellOf(bit), m.s(bitsBMult))
	return nil
}

func (c bitsChip) trace(cs *cols, r *Run) error {
	cs.declare(bitsB, bitsW, bitsR, bitsV, bitsInv)
	for k, inst := range c.m.bits {
		x := r.values[inst.v][0].Uint64()
		low := x & (1<<inst.n - 1)
		for i := range BitsWidth {
			row := k*BitsWidth + i
			cs.set(bitsB, row, x>>i&1)
			cs.set(bitsW, row, x&(1<<i-1))
			cs.set(bitsV, row, x)
			if i == 24 {
				var zv field.Element
				zv.SetUint64(koalaHigh - x>>24)
				if !zv.IsZero() {
					zv.Inverse(&zv)
				}
				cs.setElem(bitsInv, row, zv)
			}
			if i < inst.n {
				cs.set(bitsR, row, low>>i)
			}
		}
	}
	return nil
}
