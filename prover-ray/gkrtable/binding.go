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
	var (
		logSize    = t.logSize
		size       = 1 << logSize
		round      = t.Sigma.Round().EnsureNext()
		one        = field.One()
		oneConst   = wiop.NewConstantField(one)
		batchedRow = make([]wiop.Expression, nbColumns)
	)
	t.eq = t.Module.NewExtensionColumn(ctx.Childf("eq"), round)
	eqView := t.eq.View()
	t.Lambda = round.NewCoinField(ctx.Childf("lambda"))
	round.EnsureNext() // the round of the result of the log-derivative sum

	// b_j(i) is bit n-j of i: b_1 is the most significant bit, as in gnark's points.
	bitColumns := make([]*wiop.Column, logSize)
	for j := range bitColumns {
		bit := make([]field.Element, size)
		for i := range bit {
			bit[i].SetUint64(uint64(i >> (logSize - 1 - j) & 1))
		}
		bitColumns[j] = t.Module.NewPrecomputedColumn(
			ctx.Childf("bit-%d", j+1),
			&wiop.ConcreteVector{Plain: field.VecFromBase(bit)},
		)
	}

	// E[0] = ∏ⱼ (1 - r*ⱼ)
	product := wiop.Expression(oneConst)
	for _, r := range t.Point {
		product = wiop.Mul(product, wiop.Sub(oneConst, r))
	}
	t.Module.NewLocalConstraint(ctx.Childf("eq-first-row"), wiop.Sub(eqView, product), 0)

	// b_j · ((1 - r*ⱼ) E[i] - r*ⱼ E[i - 2^(n-j)]) = 0 on every row
	for j, r := range t.Point {
		shifted := eqView.Shift(-(1 << (logSize - 1 - j)))
		t.Module.NewVanishingManual(
			ctx.Childf("eq-step-%d", j+1),
			wiop.Mul(bitColumns[j].View(), wiop.Sub(wiop.Mul(wiop.Sub(oneConst, r), eqView), wiop.Mul(r, shifted))),
		)
	}

	// Σᵢ (Σ_c λᶜ C_c[i]) E[i]
	for c, column := range t.Columns {
		batchedRow[c] = column.View()
	}
	sum := sys.NewLogDerivativeSum(ctx.Childf("claims"), []wiop.Fraction{{
		Numerator:   wiop.Mul(wiop.RLCExpression(t.Lambda, batchedRow), eqView),
		Denominator: oneConst,
	}})
	t.Result = sum.Result

	round.RegisterAction(&eqAction{t})
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
	lambda := rt.GetCoinValue(a.Lambda).AsExt()
	// Horner, from the last value
	var expected field.Ext
	for c := len(a.Values) - 1; c >= 0; c-- {
		y := rt.GetCellValue(a.Values[c]).AsExt()
		expected.Mul(&expected, &lambda)
		expected.Add(&expected, &y)
	}
	if got := rt.GetCellValue(a.Result).AsExt(); got != expected {
		return errors.New("gkrtable: the columns of T do not have the evaluations y at r*")
	}
	return nil
}
