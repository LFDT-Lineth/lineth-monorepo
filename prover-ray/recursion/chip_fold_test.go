package recursion

import (
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/stretchr/testify/require"
)

// foldRef is fri.checkFolds' fold: (p + q + α·(p − q)·x⁻¹)/2.
func foldRef(p, q, alpha field.Ext, xInv field.Element) field.Ext {
	var s, d field.Ext
	s.Add(&p, &q)
	d.Sub(&p, &q)
	d.MulByElement(&d, &xInv)
	d.Mul(&d, &alpha)
	s.Add(&s, &d)
	s.Halve()
	return s
}

// foldChainValues are a query's FRI values: x⁻¹ at the first round, and per
// round the pair, the challenge and the target.
type foldChainValues struct {
	xInv          field.Element
	p, q, alpha   []field.Ext
	target        []field.Ext
	targetIsNextP bool // the target of round j is the pair P of round j + 1 (one cell)
}

// foldCircuit: chains of the given lengths, all values inputs. A chain whose
// targetIsNextP is set reads its next round's P as the target, as a FRI query
// without levels does, the last round's target an input (the final value).
func foldCircuit(lens []int, nextP []bool) *Machine {
	m := &Machine{}
	for c, n := range lens {
		xInv := m.InputScalar()
		rounds := make([]FoldRound, n)
		for j := range rounds {
			rounds[j] = FoldRound{P: m.InputE6(), Q: m.InputE6(), Alpha: m.InputE6()}
		}
		for j := range rounds {
			if nextP[c] && j+1 < n {
				rounds[j].Target = rounds[j+1].P
			} else {
				rounds[j].Target = m.InputE6()
			}
		}
		m.FoldChain(xInv, rounds)
	}
	return m
}

// honestFold returns the values of a chain of n rounds that folds.
func honestFold(r *rand.Rand, n int, nextP bool) foldChainValues {
	v := foldChainValues{targetIsNextP: nextP}
	v.xInv.SetUint64(r.Uint64N(1<<30) + 1)
	for range n {
		v.p, v.q, v.alpha = append(v.p, randE6(r)), append(v.q, randE6(r)), append(v.alpha, randE6(r))
	}
	v.target = make([]field.Ext, n)
	xi := v.xInv
	for j := range n {
		v.target[j] = foldRef(v.p[j], v.q[j], v.alpha[j], xi)
		if nextP && j+1 < n {
			v.p[j+1] = v.target[j]
		}
		xi.Square(&xi)
	}
	return v
}

// inputs returns the chain's input cells, in foldCircuit's order.
func (v foldChainValues) inputs() []Cell {
	res := []Cell{ScalarCell(v.xInv)}
	for j := range v.p {
		res = append(res, E6Cell(v.p[j]), E6Cell(v.q[j]), E6Cell(v.alpha[j]))
	}
	for j := range v.target {
		if !v.targetIsNextP || j+1 == len(v.target) {
			res = append(res, E6Cell(v.target[j]))
		}
	}
	return res
}

func foldInputs(chains []foldChainValues) []Cell {
	var res []Cell
	for _, v := range chains {
		res = append(res, v.inputs()...)
	}
	return res
}

var (
	foldLens  = []int{3, 1, 5}
	foldNextP = []bool{false, false, true}
)

func honestFolds(seed uint64) []foldChainValues {
	r := rand.New(rand.NewPCG(seed, 5))
	res := make([]foldChainValues, len(foldLens))
	for c, n := range foldLens {
		res[c] = honestFold(r, n, foldNextP[c])
	}
	return res
}

func TestFoldChain(t *testing.T) {
	p, err := foldCircuit(foldLens, foldNextP).Compile(testPCSOpts...)
	require.NoError(t, err)
	for seed := range uint64(3) {
		r, err := p.Execute(foldInputs(honestFolds(seed)))
		require.NoError(t, err)
		require.NoError(t, proveAndVerify(p, r))
	}
}

// TestFoldChainRejects: statements with one value wrong, which execute.
func TestFoldChainRejects(t *testing.T) {
	p, err := foldCircuit(foldLens, foldNextP).Compile(testPCSOpts...)
	require.NoError(t, err)
	one := field.One()
	for name, edit := range map[string]func(v []foldChainValues){
		"target":       func(v []foldChainValues) { v[0].target[1].B1.A0.Add(&v[0].target[1].B1.A0, &one) },
		"final":        func(v []foldChainValues) { v[2].target[4].B0.A0.Add(&v[2].target[4].B0.A0, &one) },
		"x⁻¹":          func(v []foldChainValues) { v[0].xInv.Add(&v[0].xInv, &one) },
		"α":            func(v []foldChainValues) { v[1].alpha[0].B2.A1.Add(&v[1].alpha[0].B2.A1, &one) },
		"pair swapped": func(v []foldChainValues) { v[0].p[2], v[0].q[2] = v[0].q[2], v[0].p[2] },
		// round 3's P, which is round 2's target
		"chained P": func(v []foldChainValues) { v[2].p[3].B0.A1.Add(&v[2].p[3].B0.A1, &one) },
	} {
		t.Run(name, func(t *testing.T) {
			v := honestFolds(1)
			edit(v)
			r, err := p.Execute(foldInputs(v))
			require.NoError(t, err, "not executed")
			require.Error(t, proveAndVerify(p, r), "tampered fold accepted")
		})
	}
}

// TestFoldChainRejectsForged: forged traces, each breaking a single check of
// the chip, all of a statement that does not fold (round 1 of chain 0, the
// rows 0..2, targets a wrong value, or x⁻¹ is wrong).
//   - a forged pair, challenge or target: the witness value that makes the
//     row fold, which only that column's read rejects;
//   - x⁻¹ squared wrong (from round 1 on, the targets folded with it): the
//     squaring constraint;
//   - a wrong x⁻¹ (the targets folded with the witness's): the first row's
//     x⁻¹ read.
func TestFoldChainRejectsForged(t *testing.T) {
	p, err := foldCircuit(foldLens, foldNextP).Compile(testPCSOpts...)
	require.NoError(t, err)
	const row = 1
	var two field.Element
	two.SetUint64(2)
	one := field.One()

	// the statement: chain 0's target at round 1 is off by one
	wrongTarget := func() ([]foldChainValues, field.Element) {
		v := honestFolds(2)
		v[0].target[row].B0.A0.Add(&v[0].target[row].B0.A0, &one)
		xi := v[0].xInv
		xi.Square(&xi)
		return v, xi
	}
	setE6 := func(p *Program, r *Run, col string, x field.Ext) {
		c := E6Cell(x)
		for i := range KindE6 {
			r.Witness(p, foldMod, laneName(col, i))[row] = c[i]
		}
	}
	// (1 + α·xi)·P + (1 − α·xi)·Q = 2T
	coef := func(alpha field.Ext, xi field.Element) (plus, minus field.Ext) {
		var ax field.Ext
		ax.MulByElement(&alpha, &xi)
		plus.SetOne()
		minus.SetOne()
		plus.Add(&plus, &ax)
		minus.Sub(&minus, &ax)
		return plus, minus
	}
	twoT := func(t field.Ext) field.Ext {
		t.MulByElement(&t, &two)
		return t
	}

	for name, forge := range map[string]func(v []foldChainValues, xi field.Element, p *Program, r *Run){
		"p": func(v []foldChainValues, xi field.Element, p *Program, r *Run) {
			c := v[0]
			plus, minus := coef(c.alpha[row], xi)
			var x field.Ext
			x.Mul(&minus, &c.q[row])
			t2 := twoT(c.target[row])
			x.Sub(&t2, &x)
			x.Div(&x, &plus)
			setE6(p, r, foldP, x)
		},
		"q": func(v []foldChainValues, xi field.Element, p *Program, r *Run) {
			c := v[0]
			plus, minus := coef(c.alpha[row], xi)
			var x field.Ext
			x.Mul(&plus, &c.p[row])
			t2 := twoT(c.target[row])
			x.Sub(&t2, &x)
			x.Div(&x, &minus)
			setE6(p, r, foldQ, x)
		},
		"α": func(v []foldChainValues, xi field.Element, p *Program, r *Run) {
			// α = (2T − P − Q)/((P − Q)·xi)
			c := v[0]
			var num, den field.Ext
			num = twoT(c.target[row])
			num.Sub(&num, &c.p[row])
			num.Sub(&num, &c.q[row])
			den.Sub(&c.p[row], &c.q[row])
			den.MulByElement(&den, &xi)
			num.Div(&num, &den)
			setE6(p, r, foldAl, num)
		},
		"target": func(v []foldChainValues, xi field.Element, p *Program, r *Run) {
			c := v[0]
			setE6(p, r, foldT, foldRef(c.p[row], c.q[row], c.alpha[row], xi))
		},
	} {
		t.Run(name, func(t *testing.T) {
			v, xi := wrongTarget()
			r, err := p.Execute(foldInputs(v))
			require.NoError(t, err)
			forge(v, xi, p, r)
			require.Error(t, proveAndVerify(p, r), "forged fold accepted")
		})
	}

	// x⁻¹ not squared: from round 1 on, the chain folds with y, y², …
	t.Run("x⁻¹ squared", func(t *testing.T) {
		v := honestFolds(3)
		c := &v[0]
		var y field.Element
		y.SetUint64(12345)
		for j := 1; j < len(c.p); j++ {
			c.target[j] = foldRef(c.p[j], c.q[j], c.alpha[j], y)
			y.Square(&y)
		}
		r, err := p.Execute(foldInputs(v))
		require.NoError(t, err)
		y.SetUint64(12345)
		xis := r.Witness(p, foldMod, foldXi)
		for j := 1; j < len(c.p); j++ {
			xis[j] = y
			y.Square(&y)
		}
		require.Error(t, proveAndVerify(p, r), "forged x⁻¹ accepted")
	})

	// a wrong x⁻¹: the whole chain folds with y, y², …, the cell holding x⁻¹
	t.Run("x⁻¹ read", func(t *testing.T) {
		v := honestFolds(3)
		c := &v[0]
		var y field.Element
		y.SetUint64(12345)
		for j := range c.p {
			c.target[j] = foldRef(c.p[j], c.q[j], c.alpha[j], y)
			y.Square(&y)
		}
		r, err := p.Execute(foldInputs(v))
		require.NoError(t, err)
		y.SetUint64(12345)
		xis := r.Witness(p, foldMod, foldXi)
		for j := range c.p {
			xis[j] = y
			y.Square(&y)
		}
		require.Error(t, proveAndVerify(p, r), "forged x⁻¹ accepted")
	})
}
