package recursion

import (
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// Setup columns of the fold chip.
const (
	foldActive = "active"
	foldFirst  = "first"
	foldLast   = "last"
	foldAddrP  = "addr_p"
	foldAddrQ  = "addr_q"
	foldAddrAl = "addr_al"
	foldAddrT  = "addr_t"
	foldAddrXi = "addr_xi"
)

// Witness columns of the fold chip.
const (
	foldP  = "p"  // lanes 0..5: the pair folded, at x
	foldQ  = "q"  // lanes 0..5: and at −x
	foldAl = "al" // lanes 0..5: the round's fold challenge
	foldT  = "t"  // lanes 0..5: what the fold must equal
	foldXi = "xi" // x⁻¹ at the round
)

// FoldRound is one round of a FRI query in ray's PCS (fri.checkFolds): the
// pair folded, P at the query's point x of the round and Q at −x, its fold
// challenge Alpha, and Target, the value the fold must equal: the next
// round's running-layer value at the query (its leaf), or the final
// polynomial's value after the last round. All are E6 cells.
//
// The pair is the running layer's (its leaf and the bottom sibling of its
// path), or, at the round a level (a module size) joins FRI, the level's
// pair, a^R·running + its DEEP terms (see fri.deepPowers), which the caller
// computes: the chip does not combine levels.
type FoldRound struct {
	P, Q, Alpha, Target int
}

type foldInst struct {
	xInv   int
	rounds []FoldRound
}

// FoldChain checks a FRI query's folds: for every round j,
//
//	(P + Q)/2 + α·(P − Q)·x_j⁻¹/2 = Target,
//
// with x_j⁻¹ = xInv^(2^j): xInv is the scalar cell of x⁻¹ at the first round,
// and the point squares from one round to the next. It writes no cell, and a
// chain that does not fold executes, its proof does not verify.
func (m *Machine) FoldChain(xInv int, rounds []FoldRound) {
	if len(rounds) == 0 {
		panic("FoldChain: no round")
	}
	m.read(xInv)
	for _, r := range rounds {
		for _, a := range []int{r.P, r.Q, r.Alpha, r.Target} {
			m.read(a)
		}
	}
	m.folds = append(m.folds, foldInst{xInv: xInv, rounds: append([]FoldRound(nil), rounds...)})
}

// foldChip: one row per FRI round of a query, a query's rounds contiguous.
//
// Witness: p0..5, q0..5, al0..5, t0..5, xi (see their names). Setup: active,
// first, last (a chain's first and last rows), addr_p, addr_q, addr_al,
// addr_t, addr_xi (first row).
//
//	active·((p + q) + al·(p − q)·xi − 2·t) = 0
//	(active − last)·(xi[+1] − xi²) = 0          the point squares
//
// Bus: reads p, q, al, t on every active row, and xi on a chain's first row.
// Every access has a setup multiplicity, so padding rows are inert. The
// constraints have degree 3.
type foldChip struct{ m *Machine }

func (c foldChip) name() string { return foldMod }

func (c foldChip) rows() int {
	n := 0
	for _, f := range c.m.folds {
		n += len(f.rounds)
	}
	return n
}

func (c foldChip) setup(cs *cols) {
	cs.declare(foldActive, foldFirst, foldLast, foldAddrP, foldAddrQ, foldAddrAl, foldAddrT, foldAddrXi)
	row := 0
	for _, f := range c.m.folds {
		for k, r := range f.rounds {
			cs.set(foldActive, row, 1)
			if k == 0 {
				cs.set(foldFirst, row, 1)
				cs.set(foldAddrXi, row, uint64(f.xInv))
			}
			if k == len(f.rounds)-1 {
				cs.set(foldLast, row, 1)
			}
			cs.set(foldAddrP, row, uint64(r.P))
			cs.set(foldAddrQ, row, uint64(r.Q))
			cs.set(foldAddrAl, row, uint64(r.Alpha))
			cs.set(foldAddrT, row, uint64(r.Target))
			row++
		}
	}
}

func (c foldChip) define(m *module, bus *Bus) error {
	active, first, last := m.s(foldActive), m.s(foldFirst), m.s(foldLast)
	p, q, al, t := m.lanes(foldP, KindE6), m.lanes(foldQ, KindE6), m.lanes(foldAl, KindE6), m.lanes(foldT, KindE6)
	xi := m.w(foldXi)

	diff := make([]wiop.Expression, KindE6)
	for i := range diff {
		diff[i] = wiop.Mul(wiop.Sub(p[i], q[i]), xi)
	}
	odd := e6MulExprs(al, diff)
	for i := range KindE6 {
		fold := sum(p[i], q[i], odd[i])
		m.assertZero(wiop.Mul(active, wiop.Sub(fold, wiop.Mul(constant(2), t[i]))))
	}
	m.assertZero(wiop.Mul(wiop.Sub(active, last), wiop.Sub(m.wShift(foldXi, 1), wiop.Mul(xi, xi))))

	for _, x := range []struct {
		addr string
		v    []wiop.Expression
	}{{foldAddrP, p}, {foldAddrQ, q}, {foldAddrAl, al}, {foldAddrT, t}} {
		bus.Read(m, m.s(x.addr), cellOf(x.v...), active)
	}
	bus.Read(m, m.s(foldAddrXi), cellOf(xi), first)
	return nil
}

func (c foldChip) trace(cs *cols, r *Run) error {
	cs.declare(foldXi)
	for _, x := range []string{foldP, foldQ, foldAl, foldT} {
		cs.declareLanes(x, KindE6)
	}
	row := 0
	for _, f := range c.m.folds {
		xi := r.values[f.xInv][0]
		for _, fr := range f.rounds {
			cs.setE6(foldP, row, CellE6(r.values[fr.P]))
			cs.setE6(foldQ, row, CellE6(r.values[fr.Q]))
			cs.setE6(foldAl, row, CellE6(r.values[fr.Alpha]))
			cs.setE6(foldT, row, CellE6(r.values[fr.Target]))
			cs.setElem(foldXi, row, xi)
			xi.Square(&xi)
			row++
		}
	}
	return nil
}
