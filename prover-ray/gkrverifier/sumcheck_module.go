package gkrverifier

import (
	"fmt"
	"slices"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// foldModule has one row per fold row.
type foldModule struct {
	module                   *wiop.Module
	v, phi, e, c, sum, final [6]*wiop.Column // witness
	// precomputed
	active, chain, last  *wiop.Column
	readE, readC         *wiop.Column
	idV, idPhi, idE, idC *wiop.Column
	idSum, idFinal       *wiop.Column
}

// eqModule has one row per eq row.
type eqModule struct {
	module *wiop.Module
	// witness: p and r of each slot, the product h of the factors of the first two slots, the product g
	// of the three, and the running product
	p, r          [eqSlots][6]*wiop.Column
	h, g, product [6]*wiop.Column
	// precomputed
	active, chain, last *wiop.Column
	read                [eqSlots]*wiop.Column // 1 where the slot's coordinate is read
	idP, idR            [eqSlots]*wiop.Column
	idResult            *wiop.Column
}

// roundModule has one row per round row, with room for the largest degree of the levels. It is nil if no
// level has rounds.
type roundModule struct {
	module *wiop.Module
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

// unit returns coordinate k of 1 in E₆, as a constant expression.
func unit(k int) wiop.Expression {
	if k == 0 {
		return baseConstant(1)
	}
	return baseConstant(0)
}

func (v *Verifier) defineFolds(sys *wiop.System, ctx *wiop.ContextFrame, round *wiop.Round) {
	l, f := v.layout, &v.fold
	nb := len(l.foldRows)
	f.module = sys.NewSizedModule(ctx, moduleSize(nb), wiop.PaddingDirectionNone)
	for k := range 6 {
		f.v[k] = f.module.NewColumn(ctx.Childf("v%d", k), round)
		f.phi[k] = f.module.NewColumn(ctx.Childf("phi%d", k), round)
		f.e[k] = f.module.NewColumn(ctx.Childf("e%d", k), round)
		f.c[k] = f.module.NewColumn(ctx.Childf("c%d", k), round)
		f.sum[k] = f.module.NewColumn(ctx.Childf("sum%d", k), round)
		f.final[k] = f.module.NewColumn(ctx.Childf("final%d", k), round)
	}
	chained, idV, idPhi, idFinal := make([]int, nb), make([]int, nb), make([]int, nb), make([]int, nb)
	idSum, idE, idC := make([]int, nb), make([]int, nb), make([]int, nb)
	for i, row := range l.foldRows {
		if !row.first {
			chained[i] = 1
		}
		idV[i], idPhi[i], idFinal[i] = row.value, row.phi, max(row.final, 0)
		idSum[i], idE[i], idC[i] = row.sum, row.eq, row.fold
	}
	f.active = precomputed(ctx.Childf("active"), f.module, slices.Repeat([]int{1}, nb))
	f.chain = precomputed(ctx.Childf("chain"), f.module, chained)
	f.last, f.idSum = idColumns(ctx.Childf("sum"), f.module, idSum)
	f.readE, f.idE = idColumns(ctx.Childf("e"), f.module, idE)
	f.readC, f.idC = idColumns(ctx.Childf("c"), f.module, idC)
	f.idV = precomputed(ctx.Childf("id-v"), f.module, idV)
	f.idPhi = precomputed(ctx.Childf("id-phi"), f.module, idPhi)
	f.idFinal = precomputed(ctx.Childf("id-final"), f.module, idFinal)

	// On every row, the sums extend those of the previous row, except on the first row of a level; e is
	// 1 where it is not read; c is 0 where it is not read.
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
			wiop.Mul(wiop.Sub(baseConstant(1), f.readC.View()), f.c[k].View()))
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

func (f *foldModule) assign(run *wiop.Runtime, w *foldWitness) {
	assignExt(run, f.v, w.v)
	assignExt(run, f.phi, w.phi)
	assignExt(run, f.e, w.e)
	assignExt(run, f.c, w.c)
	assignExt(run, f.sum, w.sum)
	assignExt(run, f.final, w.final)
}

func (v *Verifier) defineEqs(sys *wiop.System, ctx *wiop.ContextFrame, round *wiop.Round) {
	l, q := v.layout, &v.eq
	nb := len(l.eqRows)
	q.module = sys.NewSizedModule(ctx, moduleSize(nb), wiop.PaddingDirectionNone)
	for s := range eqSlots {
		for k := range 6 {
			q.p[s][k] = q.module.NewColumn(ctx.Childf("p%d-%d", s, k), round)
			q.r[s][k] = q.module.NewColumn(ctx.Childf("r%d-%d", s, k), round)
		}
	}
	for k := range 6 {
		q.h[k] = q.module.NewColumn(ctx.Childf("h%d", k), round)
		q.g[k] = q.module.NewColumn(ctx.Childf("g%d", k), round)
		q.product[k] = q.module.NewColumn(ctx.Childf("product%d", k), round)
	}

	chained, results := make([]int, nb), make([]int, nb)
	for i, row := range l.eqRows {
		if !row.first {
			chained[i] = 1
		}
		results[i] = row.result
	}
	q.active = precomputed(ctx.Childf("active"), q.module, slices.Repeat([]int{1}, nb))
	q.chain = precomputed(ctx.Childf("chain"), q.module, chained)
	q.last, q.idResult = idColumns(ctx.Childf("result"), q.module, results)
	for s := range eqSlots {
		idP, idR := make([]int, nb), make([]int, nb)
		for i, row := range l.eqRows {
			idP[i], idR[i] = row.p[s], max(row.r[s], 0)
		}
		q.read[s], q.idP[s] = idColumns(ctx.Childf("p%d", s), q.module, idP)
		q.idR[s] = precomputed(ctx.Childf("id-r%d", s), q.module, idR)
	}

	// On every row: a slot that is not read is zero, so that its factor is 1; the factor of a slot is
	// 1 − p − r + 2pr; h is the product of the first two factors, g that of the three; and the running
	// product is g on the first row of an eq, g times the previous running product on the others.
	var factors [eqSlots]E6
	for s := range eqSlots {
		p, r := columnViews(q.p[s]), columnViews(q.r[s])
		pr := p.Mul(r)
		for k := range factors[s] {
			factors[s][k] = wiop.Sub(wiop.Add(unit(k), wiop.Mul(baseConstant(2), pr[k])), wiop.Add(p[k], r[k]))
			q.module.NewVanishingManual(ctx.Childf("zero-p%d-%d", s, k),
				wiop.Mul(wiop.Sub(baseConstant(1), q.read[s].View()), p[k]))
			q.module.NewVanishingManual(ctx.Childf("zero-r%d-%d", s, k),
				wiop.Mul(wiop.Sub(baseConstant(1), q.read[s].View()), r[k]))
		}
	}
	h, g := columnViews(q.h), columnViews(q.g)
	for k, diff := range h.Equal(factors[0].Mul(factors[1])) {
		q.module.NewVanishingManual(ctx.Childf("h%d", k), diff)
	}
	for k, diff := range g.Equal(h.Mul(factors[2])) {
		q.module.NewVanishingManual(ctx.Childf("g%d", k), diff)
	}
	previous := shifted(q.product).MulByBase(q.chain.View())
	for k := range previous {
		previous[k] = wiop.Add(previous[k], wiop.Mul(wiop.Sub(q.active.View(), q.chain.View()), unit(k)))
	}
	for k, diff := range columnViews(q.product).Equal(previous.Mul(g)) {
		q.module.NewVanishingManual(ctx.Childf("product%d", k), diff)
	}
}

func (q *eqModule) producers() []wiop.Table {
	return []wiop.Table{busElement(q.module, q.last, q.idResult, q.product)}
}

func (q *eqModule) consumers() []wiop.Table {
	tables := make([]wiop.Table, 0, 2*eqSlots)
	for s := range eqSlots {
		tables = append(tables,
			busElement(q.module, q.read[s], q.idP[s], q.p[s]),
			busElement(q.module, q.read[s], q.idR[s], q.r[s]))
	}
	return tables
}

// assign assigns the witness. The rows past the eq rows are padding: nothing is read, so every factor is
// 1, and so are h and g.
func (q *eqModule) assign(run *wiop.Runtime, w *eqWitness) {
	for s := range eqSlots {
		assignExt(run, q.p[s], w.p[s])
		assignExt(run, q.r[s], w.r[s])
	}
	pad := func(x []field.Ext) []field.Ext {
		return slices.Concat(x, slices.Repeat([]field.Ext{field.OneExt()}, q.module.Size()-len(x)))
	}
	assignExt(run, q.h, pad(w.h))
	assignExt(run, q.g, pad(w.g))
	assignExt(run, q.product, w.product)
}

func (v *Verifier) defineRounds(sys *wiop.System, ctx *wiop.ContextFrame, round *wiop.Round) {
	l, m := v.layout, &v.rounds
	nb, d := len(l.roundRows), l.maxDegree
	if d == 0 {
		return
	}
	m.module = sys.NewSizedModule(ctx, moduleSize(nb), wiop.PaddingDirectionNone)
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

	chained, kappa, idR := make([]int, nb), make([]int, nb), make([]int, nb)
	idSum, idFinal, idMu := make([]int, nb), make([]int, nb), make([]int, nb)
	for i, row := range l.roundRows {
		if !row.first {
			chained[i] = 1
		}
		kappa[i], idR[i] = row.kappa, row.r
		idSum[i], idFinal[i], idMu[i] = row.sum, row.final, row.mu
	}
	m.active = precomputed(ctx.Childf("active"), m.module, slices.Repeat([]int{1}, nb))
	m.chain = precomputed(ctx.Childf("chain"), m.module, chained)
	m.kappa = precomputed(ctx.Childf("kappa"), m.module, kappa)
	m.idR = precomputed(ctx.Childf("id-r"), m.module, idR)
	m.readMu, m.idMu = idColumns(ctx.Childf("mu"), m.module, idMu)
	for k := range d {
		idC := make([]int, nb)
		for i, row := range l.roundRows {
			idC[i] = -1
			if k < len(row.coeffs) {
				idC[i] = row.coeffs[k]
			}
		}
		readC, idCol := idColumns(ctx.Childf("c%d", k+1), m.module, idC)
		m.readC, m.idC = append(m.readC, readC), append(m.idC, idCol)
	}
	m.first, m.idSum = idColumns(ctx.Childf("sum"), m.module, idSum)
	m.last, m.idFinal = idColumns(ctx.Childf("final"), m.module, idFinal)

	// the constraints, which hold on every row
	a, out, r, mu, c0 := columnViews(m.a), columnViews(m.out), columnViews(m.r), columnViews(m.mu), columnViews(m.c0)
	coefficients := make([]E6, d)
	for k := range d {
		coefficients[k] = columnViews(m.c[k])
	}
	total := coefficients[0]
	for _, c := range coefficients[1:] {
		total = total.Add(c)
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
	for k, diff := range a.Equal(shifted(m.out)) {
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
				wiop.Mul(wiop.Sub(baseConstant(1), m.readC[k].View()), m.c[k][i].View()))
		}
	}
}

func (m *roundModule) producers() []wiop.Table { return nil }

func (m *roundModule) consumers() []wiop.Table {
	if m.module == nil {
		return nil
	}
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

func (m *roundModule) assign(run *wiop.Runtime, w *roundWitness) {
	if m.module == nil {
		return
	}
	assignExt(run, m.a, w.a)
	assignExt(run, m.out, w.out)
	assignExt(run, m.r, w.r)
	assignExt(run, m.mu, w.mu)
	assignExt(run, m.c0, w.c0)
	for k := range m.c {
		assignExt(run, m.c[k], columnOf(w.c, k))
	}
	for k := range m.h {
		assignExt(run, m.h[k], columnOf(w.h, k))
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
