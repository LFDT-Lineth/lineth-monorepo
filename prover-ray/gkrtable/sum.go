package gkrtable

import (
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/gkrverifier"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// ConstrainSum adds to the sized module the constraints of R = Σ_c λᶜ y_c, where λ, R and the y_c are
// elements of E₆, each given as the six base cells of its coordinates.
//
// By Horner's rule, with a_{23} = y_{23} and a_c = λ a_{c+1} + y_c, R = a₀. An expression of the whole
// sum has a size exponential in its depth, so the intermediate values a₁, …, a₂₂ are cells, in the round
// after the latest of the inputs', which a prover action of that round assigns. Each step is six local
// constraints, on the first row of the module and multiplied by a precomputed column that is 1 there,
// which the compilation of a local constraint needs as it lifts the column references. The expressions
// are otherwise over cells, so they add no quotient degree.
func ConstrainSum(module *wiop.Module, lambda, result [6]*wiop.Cell, y [nbColumns][6]*wiop.Cell) {
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
	var acc [nbColumns][6]*wiop.Cell
	acc[0], acc[nbColumns-1] = result, y[nbColumns-1]
	for c := 1; c < nbColumns-1; c++ {
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
	for c := range nbColumns - 1 {
		step := expr(acc[c+1]).Mul(l).Add(expr(y[c]))
		for k, diff := range expr(acc[c]).Equal(step) {
			module.NewLocalConstraint(module.Context.Childf("sum-step%d-%d", c, k), wiop.Mul(selector, diff), 0)
		}
	}
}

// sumAction is the prover action that assigns the intermediate values of the Horner evaluation of
// [ConstrainSum].
type sumAction struct {
	lambda [6]*wiop.Cell
	y      [nbColumns][6]*wiop.Cell
	acc    [nbColumns][6]*wiop.Cell
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

	lambda := value(a.lambda)
	sum := value(a.y[nbColumns-1])
	for c := nbColumns - 2; c >= 1; c-- {
		y := value(a.y[c])
		sum.Mul(&sum, &lambda)
		sum.Add(&sum, &y)
		for k, coordinate := range sum.AppendBase(nil) {
			rt.AssignCell(a.acc[c][k], field.ElemFromBase(coordinate))
		}
	}
}
