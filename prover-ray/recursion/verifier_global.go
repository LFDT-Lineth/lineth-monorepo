package recursion

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/global"
)

// viewKey identifies a column view: its column and shift.
type viewKey struct {
	col   wiop.ObjectID
	shift int
}

// checkGlobal checks a module's quotient identity at the evaluation coin r,
// for each bucket, as global.Verifier.Check does:
//
//	Σ_i coin^i·P_i(r)·C_i(r) = (r^n − 1)·Σ_k r^(kn)·Q_k(r)
//
// with P_i the vanishings over the claimed column evaluations, C_i their
// cancellation polynomials and Q_k the quotient shares. Both sums are Horner
// chains.
func (v *Verifier) checkGlobal(gv *global.Verifier) error {
	n := gv.Module.Size()
	if len(gv.WitnessViews) != len(gv.WitnessClaims) {
		return fmt.Errorf("global verifier of %q: %d views, %d claims",
			gv.Module.Context.Path(), len(gv.WitnessViews), len(gv.WitnessClaims))
	}
	ev := &evaluator{
		v: v, r: v.coin(gv.EvalCoin),
		views: map[viewKey]int{}, memo: map[wiop.Expression]int{}, pows: map[int]int{}, anns: map[int]int{},
	}
	for i, cv := range gv.WitnessViews {
		ev.views[viewKey{cv.Column.Context.ID, cv.ShiftingOffset}] = v.cell(gv.WitnessClaims[i])
	}
	rn := ev.pow(n)
	annihilator := ev.annihilator(n)
	coin := v.coin(gv.MergeCoin)
	for _, bkt := range gv.Buckets {
		if len(bkt.QuotientClaims) == 0 || len(bkt.Vanishings) == 0 {
			return fmt.Errorf("global verifier of %q: a bucket with %d quotient shares and %d vanishings",
				gv.Module.Context.Path(), len(bkt.QuotientClaims), len(bkt.Vanishings))
		}
		// Horner takes the highest power's coefficient first.
		qs := make([]int, len(bkt.QuotientClaims))
		for k, c := range bkt.QuotientClaims {
			qs[len(qs)-1-k] = v.cell(c)
		}
		terms := make([]int, len(bkt.Vanishings))
		for i, van := range bkt.Vanishings {
			p, err := ev.eval(van.Expression)
			if err != nil {
				return fmt.Errorf("global verifier of %q: %w", gv.Module.Context.Path(), err)
			}
			if c := ev.cancellation(van.CancelledPositions, n); c >= 0 {
				p = v.m.Mul(p, c)
			}
			terms[len(terms)-1-i] = p
		}
		v.assertEq(v.m.Horner(coin, terms), v.m.Mul(annihilator, v.m.Horner(rn, qs)))
	}
	return nil
}

// evaluator evaluates a module's expressions at the point r, in the machine:
// column views are their claimed evaluations, Lagrange selectors their closed
// form, cells and coins their cells. A node shared between expressions is
// computed once.
type evaluator struct {
	v     *Verifier
	r     int
	views map[viewKey]int
	memo  map[wiop.Expression]int
	pows  map[int]int // n → r^n
	anns  map[int]int // n → r^n − 1
}

// pow returns r^n, n a power of two, by squarings.
func (ev *evaluator) pow(n int) int {
	if n <= 0 || n&(n-1) != 0 {
		panic(fmt.Sprintf("recursion: domain size %d", n))
	}
	if p, ok := ev.pows[n]; ok {
		return p
	}
	p := ev.r
	if n > 1 {
		h := ev.pow(n / 2)
		p = ev.v.m.Mul(h, h)
	}
	ev.pows[n] = p
	return p
}

// annihilator returns r^n − 1, computed once per domain size: the global check
// and every Lagrange selector of the domain share it.
func (ev *evaluator) annihilator(n int) int {
	if a, ok := ev.anns[n]; ok {
		return a
	}
	a := ev.v.m.Sub(ev.pow(n), ev.v.scalar(field.One()))
	ev.anns[n] = a
	return a
}

// root returns ω_n^k, k resolved modulo n (negative positions index from the
// end).
func root(n, k int) field.Element {
	if k < 0 {
		k += n
	}
	var w field.Element
	w.ExpInt64(field.RootOfUnityBy(n), int64(k))
	return w
}

// cancellation returns the cell of Π_k (r − ω_n^k) over the cancelled
// positions, or -1 if there are none.
func (ev *evaluator) cancellation(cancelled []int, n int) int {
	res := -1
	for _, k := range cancelled {
		f := ev.v.m.Sub(ev.r, ev.v.scalar(root(n, k)))
		if res < 0 {
			res = f
		} else {
			res = ev.v.m.Mul(res, f)
		}
	}
	return res
}

// lagrange returns the cell of the Lagrange selector L_pos of a domain of size
// n at r:
//
//	L_pos(r) = ω^pos·(r^n − 1) / (n·(r − ω^pos))
//
// The division binds its result only if r ≠ ω^pos, which holds but with
// probability n/|E6| for a Fiat–Shamir coin r. (Native evaluation panics
// there.)
func (ev *evaluator) lagrange(ls *wiop.LagrangeSelector) int {
	n := ls.Size()
	if ls.Position >= n || ls.Position < -n {
		panic(fmt.Sprintf("recursion: Lagrange selector at %d in a domain of size %d", ls.Position, n))
	}
	w := root(n, ls.Position)
	var c, nInv field.Element
	nInv.SetUint64(uint64(n))
	nInv.Inverse(&nInv)
	c.Mul(&w, &nInv)
	m := ev.v.m
	num := m.Mul(ev.annihilator(n), ev.v.scalar(c))
	return m.Div(num, m.Sub(ev.r, ev.v.scalar(w)))
}

// eval returns the cell of an expression's value at r.
//
// Divisions and inverses are rejected: the machine's division does not bind
// its result when the divisor is zero, where native evaluation returns zero,
// and a vanishing's divisor need not be random.
func (ev *evaluator) eval(e wiop.Expression) (int, error) {
	if a, ok := ev.memo[e]; ok {
		return a, nil
	}
	var a int
	switch e := e.(type) {
	case *wiop.ColumnView:
		var ok bool
		if a, ok = ev.views[viewKey{e.Column.Context.ID, e.ShiftingOffset}]; !ok {
			return 0, fmt.Errorf("column view (%q, shift %d) has no claim", e.Column.Context.Path(), e.ShiftingOffset)
		}
	case *wiop.LagrangeSelector:
		a = ev.lagrange(e)
	case *wiop.Constant:
		// A vector constant is a constant polynomial: its value at r is its
		// value.
		a = ev.v.scalar(e.Value)
	case *wiop.Cell:
		a = ev.v.cell(e)
	case *wiop.CoinField:
		a = ev.v.coin(e)
	case *wiop.ArithmeticOperation:
		ops := make([]int, len(e.Operands))
		for i, o := range e.Operands {
			var err error
			if ops[i], err = ev.eval(o); err != nil {
				return 0, err
			}
		}
		m := ev.v.m
		switch e.Operator {
		case wiop.ArithmeticOperatorAdd:
			a = m.Add(ops[0], ops[1])
		case wiop.ArithmeticOperatorSub:
			a = m.Sub(ops[0], ops[1])
		case wiop.ArithmeticOperatorMul:
			a = m.Mul(ops[0], ops[1])
		case wiop.ArithmeticOperatorDouble:
			a = m.Add(ops[0], ops[0])
		case wiop.ArithmeticOperatorSquare:
			a = m.Mul(ops[0], ops[0])
		case wiop.ArithmeticOperatorNegate:
			a = m.Sub(ev.v.scalar(field.Zero()), ops[0])
		default:
			return 0, fmt.Errorf("operator %v is not supported", e.Operator)
		}
	default:
		return 0, fmt.Errorf("expression %T is not supported", e)
	}
	ev.memo[e] = a
	return a, nil
}
