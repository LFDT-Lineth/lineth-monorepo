package gkrtable

import (
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/gkrverifier"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// ConstrainSum declares in the sized module the constraints of R = Σ_c λᶜ y_c, where λ, R and the y_c are
// elements of E₆, each given as the six base cells of its coordinates: the cells a₁, …, a₂₂ of Horner's
// rule, with a₂₃ = y₂₃, a_c = λ a_{c+1} + y_c and a₀ = R, in the round after the latest of the inputs',
// which a prover action of that round assigns; and, for each step, six constraints on the first row of
// the module.
func ConstrainSum(module *wiop.Module, lambda, result [6]*wiop.Cell, y [NbColumns][6]*wiop.Cell) {
	round := lambda[0].Round()
	for _, cells := range append([][6]*wiop.Cell{lambda, result}, y[:]...) {
		for _, cell := range cells {
			if cell.Round().ID > round.ID {
				round = cell.Round()
			}
		}
	}
	round = round.EnsureNext()

	// acc[c] holds a_c
	var acc [NbColumns][6]*wiop.Cell
	acc[0], acc[NbColumns-1] = result, y[NbColumns-1]
	for c := 1; c < NbColumns-1; c++ {
		for k := range acc[c] {
			acc[c][k] = round.NewCell(module.Context.Childf("sum-a%d-%d", c, k), false)
		}
	}
	round.RegisterAction(&sumAction{lambda: lambda, y: y, acc: acc})

	first := make([]field.Element, module.Size())
	first[0].SetOne()
	selector := module.NewPrecomputedColumn(
		module.Context.Childf("sum-first-row"),
		&wiop.ConcreteVector{Plain: field.VecFromBase(first)},
	).View()

	expr := func(cells [6]*wiop.Cell) gkrverifier.E6 {
		var e gkrverifier.E6
		for k, cell := range cells {
			e[k] = cell
		}
		return e
	}
	l := expr(lambda)
	for c := range NbColumns - 1 {
		step := expr(acc[c+1]).Mul(l).Add(expr(y[c]))
		for k, diff := range expr(acc[c]).Equal(step) {
			module.NewVanishingManual(module.Context.Childf("sum-step%d-%d", c, k), wiop.Mul(selector, diff))
		}
	}
}

// sumAction is the prover action that assigns the cells a₁, …, a₂₂ of [ConstrainSum].
type sumAction struct {
	lambda [6]*wiop.Cell
	y      [NbColumns][6]*wiop.Cell
	acc    [NbColumns][6]*wiop.Cell
}

// Run implements [wiop.ProverAction].
func (a *sumAction) Run(rt *wiop.Runtime) {
	value := func(cells [6]*wiop.Cell) (e field.Ext) {
		var coordinates [6]field.Element
		for k, cell := range cells {
			coordinates[k] = rt.GetCellValue(cell).AsBase()
		}
		e.SetBase(coordinates[:])
		return e
	}

	y := make([]field.Ext, NbColumns)
	for c := range y {
		y[c] = value(a.y[c])
	}
	sums := horner(value(a.lambda), y)
	for c := 1; c < NbColumns-1; c++ {
		for k, coordinate := range sums[c].AppendBase(nil) {
			rt.AssignCell(a.acc[c][k], field.ElemFromBase(coordinate))
		}
	}
}

// horner returns the values a_c of Horner's rule, a_{len(y)-1} = y_{len(y)-1} and a_c = λ a_{c+1} + y_c, so
// that a₀ = Σ_c λᶜ y_c.
func horner(lambda field.Ext, y []field.Ext) []field.Ext {
	a := make([]field.Ext, len(y))
	var next field.Ext
	for c := len(y) - 1; c >= 0; c-- {
		a[c].Mul(&next, &lambda)
		a[c].Add(&a[c], &y[c])
		next = a[c]
	}
	return a
}
