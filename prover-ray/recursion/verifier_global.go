package recursion

import (
	"fmt"
	"math/bits"

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
// chains. For a dynamic module, r^n and the constants depending on n are read
// from tables, at the module's log size.
func (v *Verifier) checkGlobal(gv *global.Verifier) error {
	if len(gv.WitnessViews) != len(gv.WitnessClaims) {
		return fmt.Errorf("global verifier of %q: %d views, %d claims",
			gv.Module.Context.Path(), len(gv.WitnessViews), len(gv.WitnessClaims))
	}
	ev := &evaluator{
		v: v, r: v.coin(gv.EvalCoin),
		views: map[viewKey]int{}, memo: map[wiop.Expression]int{}, pows: map[int]int{},
		rns: map[*wiop.Module]int{}, anns: map[*wiop.Module]int{},
	}
	for i, cv := range gv.WitnessViews {
		ev.views[viewKey{cv.Column.Context.ID, cv.ShiftingOffset}] = v.cell(gv.WitnessClaims[i])
	}
	rn := ev.powSize(gv.Module)
	annihilator := ev.annihilator(gv.Module)
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
			if c := ev.cancellation(van.CancelledPositions, gv.Module); c >= 0 {
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
	pows  map[int]int          // n → r^n
	rns   map[*wiop.Module]int // r^n, n the module's size
	anns  map[*wiop.Module]int // r^n − 1
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

// powSize returns r^n, n the module's size, computed once per module.
func (ev *evaluator) powSize(mod *wiop.Module) int {
	if p, ok := ev.rns[mod]; ok {
		return p
	}
	p := ev.v.powSize(ev.r, mod, ev.pow)
	ev.rns[mod] = p
	return p
}

// annihilator returns r^n − 1, n the module's size, computed once per module:
// the global check and every Lagrange selector of the module share it.
func (ev *evaluator) annihilator(mod *wiop.Module) int {
	if a, ok := ev.anns[mod]; ok {
		return a
	}
	a := ev.v.m.Sub(ev.powSize(mod), ev.v.scalar(field.One()))
	ev.anns[mod] = a
	return a
}

// root returns ω_n^k: for a negative position k, ω_n^(n+k), the position from
// the end; ω_n^k all the same, as ω_n^n = 1.
func root(n, k int) field.Element {
	k %= n
	if k < 0 {
		k += n
	}
	var w field.Element
	w.ExpInt64(field.RootOfUnityBy(n), int64(k))
	return w
}

// rootKey names the function n ↦ ω_n^k, for the tables of a dynamic module.
func rootKey(k int) string { return fmt.Sprintf("root %d", k) }

// rootConst returns the cell of ω_n^k, n the module's size, of a size at
// least 2^lo: the constant 1 for k = 0, whatever n.
func (ev *evaluator) rootConst(mod *wiop.Module, lo, k int) int {
	if k == 0 {
		return ev.v.scalar(field.One())
	}
	return ev.v.sizeConstAtLeast(mod, lo, rootKey(k), func(n int) field.Element { return root(n, k) })
}

// cancellation returns the cell of Π_k (r − ω_n^k) over the cancelled
// positions, n the module's size, or -1 if there are none.
func (ev *evaluator) cancellation(cancelled []int, mod *wiop.Module) int {
	res := -1
	for _, k := range cancelled {
		w := ev.rootConst(mod, 0, k)
		f := ev.v.m.Sub(ev.r, w)
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
//
// The position must be in the domain: pos < n, or −pos ≤ n from the end.
// Native evaluation panics on a smaller dynamic size, and the machine's
// execution fails (the tables start at the smallest size that holds it).
func (ev *evaluator) lagrange(ls *wiop.LagrangeSelector) int {
	pos, mod := ls.Position, ls.Module()
	minN := pos + 1
	if pos < 0 {
		minN = -pos
	}
	lo := bits.Len(uint(minN - 1))  // the smallest i with 2^i ≥ minN
	w := ev.rootConst(mod, lo, pos) // pos = 0 has lo = 0: no bound to check
	c := ev.v.sizeConstAtLeast(mod, lo, fmt.Sprintf("lagrange %d", pos), func(n int) field.Element {
		var c, nInv field.Element
		w := root(n, pos)
		nInv.SetUint64(uint64(n))
		nInv.Inverse(&nInv)
		return *c.Mul(&w, &nInv)
	})
	m := ev.v.m
	num := m.Mul(ev.annihilator(mod), c)
	return m.Div(num, m.Sub(ev.r, w))
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
