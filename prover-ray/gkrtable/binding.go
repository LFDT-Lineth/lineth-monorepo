package gkrtable

import (
	"errors"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// defineBinding binds T to the claims of the GKR proof, as the comment of the package describes: the
// bit columns b_j, the column E in the round after the GKR proof's with its constraints, the coin λ,
// and the log-derivative sum with the verifier action that checks it.
func (t *Table) defineBinding(sys *wiop.System, ctx *wiop.ContextFrame) {
	n := len(t.Point)
	round := t.Seed.Round().EnsureNext()
	t.eq = t.Module.NewExtensionColumn(ctx.Childf("eq"), round)
	eqView := t.eq.View()
	t.Lambda = round.NewCoinField(ctx.Childf("lambda"))
	round.EnsureNext() // the round of the result of the log-derivative sum

	// b_j(i) is bit n-j of i: b_1 is the most significant bit, as in gnark's points.
	bitColumns := make([]*wiop.Column, n)
	for j := range bitColumns {
		bit := make([]field.Element, t.Module.Size())
		for i := range bit {
			bit[i].SetUint64(uint64(i >> (n - 1 - j) & 1))
		}
		bitColumns[j] = t.Module.NewPrecomputedColumn(
			ctx.Childf("bit-%d", j+1),
			&wiop.ConcreteVector{Plain: field.VecFromBase(bit)},
		)
	}

	// E[0] = ∏ⱼ (1 - r*ⱼ)
	oneConst := wiop.NewConstantField(field.One())
	product := wiop.Expression(oneConst)
	for _, r := range t.Point {
		product = wiop.Mul(product, wiop.Sub(oneConst, r))
	}
	t.Module.NewLocalConstraint(ctx.Childf("eq-first-row"), wiop.Sub(eqView, product), 0)

	// b_j · ((1 - r*ⱼ) E[i] - r*ⱼ E[i - 2^(n-j)]) = 0 on every row
	for j, r := range t.Point {
		shifted := eqView.Shift(-(1 << (n - 1 - j)))
		t.Module.NewVanishingManual(
			ctx.Childf("eq-step-%d", j+1),
			wiop.Mul(bitColumns[j].View(), wiop.Sub(wiop.Mul(wiop.Sub(oneConst, r), eqView), wiop.Mul(r, shifted))),
		)
	}

	// Σᵢ (Σ_c λᶜ C_c[i]) E[i]
	batched := make([]wiop.Expression, NbColumns)
	for c, column := range t.Columns {
		batched[c] = column.View()
	}
	sum := sys.NewLogDerivativeSum(ctx.Childf("claims"), []wiop.Fraction{{
		Numerator:   wiop.Mul(wiop.RLCExpression(t.Lambda, batched), eqView),
		Denominator: oneConst,
	}})
	t.Result = sum.Result

	round.RegisterAction(&eqAction{Table: t})
	sum.Result.Round().RegisterVerifierAction(&sumVerifierAction{Table: t})
}

// eqAction is the prover action that assigns E[i] = eq(r*, i).
type eqAction struct{ *Table }

// Run implements [wiop.ProverAction].
func (a *eqAction) Run(rt *wiop.Runtime) {
	// The index of a row has the bits b_1, …, b_n from the most significant, so each coordinate
	// of r* doubles the rows: the row with the bit clear gets the factor 1 - r*ⱼ, the other r*ⱼ.
	one := field.OneExt()
	eq := []field.Ext{one}
	for _, cell := range a.Point {
		r := rt.GetCellValue(cell).AsExt()
		var oneMinusR field.Ext
		oneMinusR.Sub(&one, &r)
		next := make([]field.Ext, 2*len(eq))
		for i := range eq {
			next[2*i].Mul(&eq[i], &oneMinusR)
			next[2*i+1].Mul(&eq[i], &r)
		}
		eq = next
	}
	rt.AssignColumn(a.eq, &wiop.ConcreteVector{Plain: field.VecFromExt(eq)})
}

// sumVerifierAction is the verifier action that checks the result of the log-derivative sum
// Σᵢ (Σ_c λᶜ C_c[i]) E[i], which is Σ_c λᶜ C̃_c(r*), against Σ_c λᶜ y_c.
type sumVerifierAction struct{ *Table }

// Check implements [wiop.VerifierAction].
func (a *sumVerifierAction) Check(rt *wiop.Runtime) error {
	y := make([]field.Ext, NbColumns)
	for c, cell := range a.Values {
		y[c] = rt.GetCellValue(cell).AsExt()
	}
	expected := horner(rt.GetCoinValue(a.Lambda).AsExt(), y)[0]
	if got := rt.GetCellValue(a.Result).AsExt(); got != expected {
		return errors.New("gkrtable: the columns of T do not have the evaluations y at r*")
	}
	return nil
}
