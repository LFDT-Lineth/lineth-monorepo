package gkrverifier

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// foldModule has one row per fold row.
type foldModule struct {
	module                   *wiop.Module
	size                     int
	v, phi, e, c, sum, final [6]*wiop.Column // witness
	// precomputed
	active, chain, last  *wiop.Column
	readE, readC         *wiop.Column
	idV, idPhi, idE, idC *wiop.Column
	idSum, idFinal       *wiop.Column
}

// eqModule has one row per eq row.
type eqModule struct {
	module                *wiop.Module
	size                  int
	p, r, factor, product [6]*wiop.Column // witness
	active, chain, last   *wiop.Column    // precomputed
	idP, idR, idResult    *wiop.Column
}

// roundModule has one row per round row, with room for the largest degree of the levels.
type roundModule struct {
	module *wiop.Module
	size   int
	degree int
	a, out [6]*wiop.Column
	r, mu  [6]*wiop.Column
	c0     [6]*wiop.Column
	c, h   [][6]*wiop.Column // c₁, …, c_D and h₁, …, h_{D−1}
	// precomputed
	active, chain, kappa *wiop.Column
	readMu, idMu         *wiop.Column
	idR                  *wiop.Column
	readC, idC           []*wiop.Column // per coefficient
	first, idSum         *wiop.Column
	last, idFinal        *wiop.Column
}

// shifted returns the E₆ element of the previous row of the columns.
func shifted(cols [6]*wiop.Column) E6 {
	var x E6
	for k, c := range cols {
		x[k] = c.View().Shift(-1)
	}
	return x
}

// unit returns the coordinates of 1 in E₆, as expressions.
func unit(k int) wiop.Expression {
	if k == 0 {
		return wiop.NewConstantField(field.One())
	}
	return wiop.NewConstantField(field.Zero())
}

func (v *Verifier) defineFolds(sys *wiop.System, ctx *wiop.ContextFrame, round *wiop.Round) {
	l, f := v.layout, &v.fold
	nb := len(l.foldRows)
	f.size = moduleSize(nb)
	f.module = sys.NewSizedModule(ctx, f.size, wiop.PaddingDirectionNone)
	for k := range 6 {
		f.v[k] = f.module.NewColumn(ctx.Childf("v%d", k), round)
		f.phi[k] = f.module.NewColumn(ctx.Childf("phi%d", k), round)
		f.e[k] = f.module.NewColumn(ctx.Childf("e%d", k), round)
		f.c[k] = f.module.NewColumn(ctx.Childf("c%d", k), round)
		f.sum[k] = f.module.NewColumn(ctx.Childf("sum%d", k), round)
		f.final[k] = f.module.NewColumn(ctx.Childf("final%d", k), round)
	}
	chained, idV, idPhi, idFinal := make([]int, nb), make([]int, nb), make([]int, nb), make([]int, nb)
	for i, row := range l.foldRows {
		if !row.first {
			chained[i] = 1
		}
		idV[i], idPhi[i], idFinal[i] = row.value, row.phi, max(row.final, 0)
	}
	f.active = precomputed(ctx.Childf("active"), f.module, f.size, ones(nb))
	f.chain = precomputed(ctx.Childf("chain"), f.module, f.size, chained)
	f.last, f.idSum = idColumns(ctx.Childf("sum"), f.module, f.size, nb, func(i int) int { return l.foldRows[i].sum })
	f.readE, f.idE = idColumns(ctx.Childf("e"), f.module, f.size, nb, func(i int) int { return l.foldRows[i].eq })
	f.readC, f.idC = idColumns(ctx.Childf("c"), f.module, f.size, nb, func(i int) int { return l.foldRows[i].fold })
	f.idV = precomputed(ctx.Childf("id-v"), f.module, f.size, idV)
	f.idPhi = precomputed(ctx.Childf("id-phi"), f.module, f.size, idPhi)
	f.idFinal = precomputed(ctx.Childf("id-final"), f.module, f.size, idFinal)

	// every row holds, on every row: the sums extend those of the row before, except on the first row
	// of a level, e is one where it is not read, and c is zero where it is not read.
	value, phi, e, c := columnViews(f.v), columnViews(f.phi), columnViews(f.e), columnViews(f.c)
	chain := f.chain.View()
	for k, diff := range columnViews(f.sum).Equal(value.Add(c.Mul(shifted(f.sum)).MulByBase(chain))) {
		f.module.NewVanishingManual(ctx.Childf("sum%d", k), diff)
	}
	for k, diff := range columnViews(f.final).Equal(e.Mul(phi).Add(c.Mul(shifted(f.final)).MulByBase(chain))) {
		f.module.NewVanishingManual(ctx.Childf("final%d", k), diff)
	}
	for k := range 6 {
		f.module.NewVanishingManual(ctx.Childf("e-is-one%d", k),
			wiop.Mul(wiop.Sub(f.active.View(), f.readE.View()), wiop.Sub(f.e[k].View(), unit(k))))
		f.module.NewVanishingManual(ctx.Childf("c-is-zero%d", k),
			wiop.Mul(wiop.Sub(wiop.NewConstantField(field.One()), f.readC.View()), f.c[k].View()))
	}
}

func (f *foldModule) producers() []wiop.Table {
	return []wiop.Table{
		busElement(f.module, f.last, f.idSum, f.sum),
		busElement(f.module, f.last, f.idFinal, f.final),
	}
}

func (f *foldModule) consumers() []wiop.Table {
	return []wiop.Table{
		busElement(f.module, f.active, f.idV, f.v),
		busElement(f.module, f.active, f.idPhi, f.phi),
		busElement(f.module, f.readE, f.idE, f.e),
		busElement(f.module, f.readC, f.idC, f.c),
	}
}

func (v *Verifier) defineEqs(sys *wiop.System, ctx *wiop.ContextFrame, round *wiop.Round) {
	l, q := v.layout, &v.eq
	nb := len(l.eqRows)
	q.size = moduleSize(nb)
	q.module = sys.NewSizedModule(ctx, q.size, wiop.PaddingDirectionNone)
	for k := range 6 {
		q.p[k] = q.module.NewColumn(ctx.Childf("p%d", k), round)
		q.r[k] = q.module.NewColumn(ctx.Childf("r%d", k), round)
		q.factor[k] = q.module.NewColumn(ctx.Childf("factor%d", k), round)
		q.product[k] = q.module.NewColumn(ctx.Childf("product%d", k), round)
	}
	chain, idP, idR := make([]int, nb), make([]int, nb), make([]int, nb)
	for i, row := range l.eqRows {
		if !row.first {
			chain[i] = 1
		}
		idP[i], idR[i] = row.p, row.r
	}
	q.active = precomputed(ctx.Childf("active"), q.module, q.size, ones(nb))
	q.chain = precomputed(ctx.Childf("chain"), q.module, q.size, chain)
	q.last, q.idResult = idColumns(ctx.Childf("result"), q.module, q.size, nb,
		func(i int) int { return l.eqRows[i].result })
	q.idP = precomputed(ctx.Childf("id-p"), q.module, q.size, idP)
	q.idR = precomputed(ctx.Childf("id-r"), q.module, q.size, idR)

	// factor = 1 − p − r + 2pr, and product = factor on the first row of an eq, factor times the
	// previous product on the others
	p, r, factor := columnViews(q.p), columnViews(q.r), columnViews(q.factor)
	pr := p.Mul(r)
	for k := range factor {
		rhs := wiop.Sub(wiop.Add(unit(k), wiop.Mul(baseConstant(2), pr[k])), wiop.Add(p[k], r[k]))
		q.module.NewVanishingManual(ctx.Childf("factor%d", k), wiop.Mul(q.active.View(), wiop.Sub(factor[k], rhs)))
	}
	previous := shifted(q.product).MulByBase(q.chain.View())
	for k := range previous {
		previous[k] = wiop.Add(previous[k], wiop.Mul(wiop.Sub(q.active.View(), q.chain.View()), unit(k)))
	}
	for k, diff := range columnViews(q.product).Equal(previous.Mul(factor)) {
		q.module.NewVanishingManual(ctx.Childf("product%d", k), diff)
	}
}

func (q *eqModule) producers() []wiop.Table {
	return []wiop.Table{busElement(q.module, q.last, q.idResult, q.product)}
}

func (q *eqModule) consumers() []wiop.Table {
	return []wiop.Table{
		busElement(q.module, q.active, q.idP, q.p),
		busElement(q.module, q.active, q.idR, q.r),
	}
}

func (v *Verifier) defineRounds(sys *wiop.System, ctx *wiop.ContextFrame, round *wiop.Round) {
	l, m := v.layout, &v.rounds
	nb, d := len(l.roundRows), l.maxDegree
	m.degree = d
	m.size = moduleSize(nb)
	m.module = sys.NewSizedModule(ctx, m.size, wiop.PaddingDirectionNone)
	column := func(name string) (cols [6]*wiop.Column) {
		for k := range cols {
			cols[k] = m.module.NewColumn(ctx.Childf("%s%d", name, k), round)
		}
		return cols
	}
	m.a, m.out, m.r, m.mu, m.c0 = column("a"), column("out"), column("r"), column("mu"), column("c0")
	for k := range d {
		m.c = append(m.c, column(fmt.Sprintf("c%d-", k+1)))
	}
	for k := 1; k < d; k++ {
		m.h = append(m.h, column(fmt.Sprintf("h%d-", k)))
	}

	chain, kappa, idR := make([]int, nb), make([]int, nb), make([]int, nb)
	for i, row := range l.roundRows {
		if !row.first {
			chain[i] = 1
		}
		kappa[i], idR[i] = row.kappa, row.r
	}
	m.active = precomputed(ctx.Childf("active"), m.module, m.size, ones(nb))
	m.chain = precomputed(ctx.Childf("chain"), m.module, m.size, chain)
	m.kappa = precomputed(ctx.Childf("kappa"), m.module, m.size, kappa)
	m.idR = precomputed(ctx.Childf("id-r"), m.module, m.size, idR)
	m.readMu, m.idMu = idColumns(ctx.Childf("mu"), m.module, m.size, nb, func(i int) int { return l.roundRows[i].mu })
	for k := range d {
		readC, idC := idColumns(ctx.Childf("c%d", k+1), m.module, m.size, nb, func(i int) int {
			if coeffs := l.roundRows[i].coeffs; k < len(coeffs) {
				return coeffs[k]
			}
			return -1
		})
		m.readC, m.idC = append(m.readC, readC), append(m.idC, idC)
	}
	m.first, m.idSum = idColumns(ctx.Childf("sum"), m.module, m.size, nb, func(i int) int { return l.roundRows[i].sum })
	m.last, m.idFinal = idColumns(ctx.Childf("final"), m.module, m.size, nb,
		func(i int) int { return l.roundRows[i].final })

	// the constraints, which hold on every row
	a, out, r, mu, c0 := columnViews(m.a), columnViews(m.out), columnViews(m.r), columnViews(m.mu), columnViews(m.c0)
	coefficients := make([]E6, d)
	var total E6
	for k := range d {
		coefficients[k] = columnViews(m.c[k])
		if k == 0 {
			total = coefficients[0]
		} else {
			total = total.Add(coefficients[k])
		}
	}
	if d == 0 {
		return
	}

	// κc₀ + μΣcₖ = a
	for k, diff := range a.Equal(c0.MulByBase(m.kappa.View()).Add(mu.Mul(total))) {
		m.module.NewVanishingManual(ctx.Childf("relation%d", k), diff)
	}
	// Horner's rule: hₖ = cₖ + r·hₖ₊₁ with h_D = c_D, and out = c₀ + r·h₁
	next := coefficients[d-1]
	for k := d - 2; k >= 0; k-- {
		h := columnViews(m.h[k])
		for i, diff := range h.Equal(coefficients[k].Add(r.Mul(next))) {
			m.module.NewVanishingManual(ctx.Childf("h%d-%d", k+1, i), diff)
		}
		next = h
	}
	for k, diff := range out.Equal(c0.Add(r.Mul(next))) {
		m.module.NewVanishingManual(ctx.Childf("out%d", k), diff)
	}
	// a is the previous row's out, except on the first row of a level, and μ is 1 where it is not read
	previous := shifted(m.out)
	for k, diff := range a.Equal(previous) {
		m.module.NewVanishingManual(ctx.Childf("chain%d", k), wiop.Mul(m.chain.View(), diff))
	}
	for k := range 6 {
		m.module.NewVanishingManual(ctx.Childf("mu-is-one%d", k),
			wiop.Mul(wiop.Sub(m.active.View(), m.readMu.View()), wiop.Sub(m.mu[k].View(), unit(k))))
	}
	// the coefficients that a level has no use for are zero
	for k := range d {
		for i := range 6 {
			m.module.NewVanishingManual(ctx.Childf("c%d-is-zero%d", k+1, i),
				wiop.Mul(wiop.Sub(wiop.NewConstantField(field.One()), m.readC[k].View()), m.c[k][i].View()))
		}
	}
}

func (m *roundModule) consumers() []wiop.Table {
	tables := make([]wiop.Table, 0, 4+len(m.c))
	tables = append(tables,
		busElement(m.module, m.first, m.idSum, m.a),
		busElement(m.module, m.last, m.idFinal, m.out),
		busElement(m.module, m.active, m.idR, m.r),
		busElement(m.module, m.readMu, m.idMu, m.mu))
	for k := range m.c {
		tables = append(tables, busElement(m.module, m.readC[k], m.idC[k], m.c[k]))
	}
	return tables
}

func (f *foldModule) assign(run *wiop.Runtime, w *foldWitness) {
	assignExt(run, f.v, f.size, w.v)
	assignExt(run, f.phi, f.size, w.phi)
	assignExt(run, f.e, f.size, w.e)
	assignExt(run, f.c, f.size, w.c)
	assignExt(run, f.sum, f.size, w.sum)
	assignExt(run, f.final, f.size, w.final)
}

func (q *eqModule) assign(run *wiop.Runtime, w *eqWitness) {
	assignExt(run, q.p, q.size, w.p)
	assignExt(run, q.r, q.size, w.r)
	assignExt(run, q.factor, q.size, w.factor)
	assignExt(run, q.product, q.size, w.product)
}

func (m *roundModule) assign(run *wiop.Runtime, w *roundWitness) {
	assignExt(run, m.a, m.size, w.a)
	assignExt(run, m.out, m.size, w.out)
	assignExt(run, m.r, m.size, w.r)
	assignExt(run, m.mu, m.size, w.mu)
	assignExt(run, m.c0, m.size, w.c0)
	for k := range m.c {
		assignExt(run, m.c[k], m.size, columnOf(w.c, k))
	}
	for k := range m.h {
		assignExt(run, m.h[k], m.size, columnOf(w.h, k))
	}
}

// columnOf returns the k-th entry of every row.
func columnOf(rows [][]field.Ext, k int) []field.Ext {
	r := make([]field.Ext, len(rows))
	for i := range rows {
		r[i] = rows[i][k]
	}
	return r
}
