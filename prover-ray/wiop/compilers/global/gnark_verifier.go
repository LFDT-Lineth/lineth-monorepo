package global

import (
	"fmt"
	"math/big"
	"math/bits"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/circuit"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/consensys/gnark/frontend"
)

// CheckGnark implements [wiop.GnarkVerifierAction]. It enforces the same
// quotient identity as [Verifier.Check], P_agg(r) = (r^n - 1) * Q(r), over the
// proof's claimed evaluations. All arithmetic runs in the extension field:
// the evaluation point is an extension coin, so every intermediate value is
// an extension element anyway.
//
// Only statically sized modules are supported: n fixes the annihilator, the
// share recombination and the cancellation roots.
func (gv *Verifier) CheckGnark(_ frontend.API, run *wiop.GnarkRuntime) {
	if gv.Module.IsDynamic() {
		panic(fmt.Sprintf("wiop/compilers: global.Verifier.CheckGnark: dynamic module %q is not supported",
			gv.Module.Context.Path()))
	}
	n := gv.Module.Size()
	api := run.API()
	r := run.GetCoinValue(gv.EvalCoin)
	coin := run.GetCoinValue(gv.MergeCoin)

	viewEvals := make(map[colViewKey]circuit.Ext, len(gv.WitnessViews))
	for i, cv := range gv.WitnessViews {
		key := colViewKey{id: cv.Column.Context.ID, shift: cv.ShiftingOffset}
		viewEvals[key] = run.GetCellValue(gv.WitnessClaims[i])
	}

	rPowN := r
	for i := 0; i < bits.TrailingZeros(uint(n)); i++ {
		rPowN = api.SquareExt(rPowN)
	}
	annihilator := api.SubByBaseExt(rPowN, api.One())

	for _, bkt := range gv.Buckets {
		// Q(r) = Σ_k r^{kn} · Q_k(r)
		qr := api.ZeroExt()
		rPowKN := api.OneExt()
		for _, claim := range bkt.QuotientClaims {
			qr = api.AddExt(qr, api.MulExt(rPowKN, run.GetCellValue(claim)))
			rPowKN = api.MulExt(rPowKN, rPowN)
		}

		// P_agg(r) = Σ_i coin^i · P_i(r) · C_i(r)
		pagg := api.ZeroExt()
		coinPow := api.OneExt()
		for _, v := range bkt.Vanishings {
			pr := evalExprAtPointGnark(run, v.Expression, viewEvals, r, rPowN, n)
			cr := evalCancellationAtPointGnark(api, r, v.CancelledPositions, n)
			pagg = api.AddExt(pagg, api.MulExt(coinPow, pr, &cr))
			coinPow = api.MulExt(coinPow, coin)
		}

		api.AssertIsEqualExt(pagg, api.MulExt(annihilator, qr))
	}
}

// evalCancellationAtPointGnark mirrors [evalCancellationAtPoint]:
// C(r) = Π_{k ∈ cancelled} (r − ω_n^{norm(k)}), the roots being constants.
func evalCancellationAtPointGnark(api *circuit.API, r circuit.Ext, cancelled []int, n int) circuit.Ext {
	roots := cancellationRoots(cancelled, n)
	if len(roots) == 0 {
		return api.OneExt()
	}
	// Start from the first factor rather than from 1: the accumulator's first
	// multiplication would otherwise be a full extension multiplication by one.
	result := api.SubByBaseExt(r, api.ConstBig(roots[0].BigInt(new(big.Int))))
	for _, root := range roots[1:] {
		result = api.MulExt(result, api.SubByBaseExt(r, api.ConstBig(root.BigInt(new(big.Int)))))
	}
	return result
}

// evalExprAtPointGnark mirrors [evalExprAtPoint]: column views resolve to
// their claimed evaluations, Lagrange selectors to their closed form at r,
// everything else through the runtime's scalar evaluator.
//
// rPowN is r^n for the module under verification; it is shared with every
// Lagrange selector of that module so the power chain is paid for once per
// CheckGnark rather than once per selector occurrence. A selector belonging to
// a differently sized module falls back to computing its own power.
func evalExprAtPointGnark(
	run *wiop.GnarkRuntime, expr wiop.Expression, viewEvals map[colViewKey]circuit.Ext, r, rPowN circuit.Ext, n int,
) circuit.Ext {
	return run.EvaluateSingle(expr, func(e wiop.Expression) (circuit.Ext, bool) {
		switch leaf := e.(type) {
		case *wiop.ColumnView:
			key := colViewKey{id: leaf.Column.Context.ID, shift: leaf.ShiftingOffset}
			v, ok := viewEvals[key]
			if !ok {
				panic(fmt.Sprintf("wiop/compilers: ColumnView (%v, shift=%d) not in witness eval map",
					leaf.Column.Context.ID, leaf.ShiftingOffset))
			}
			return v, true
		case *wiop.LagrangeSelector:
			if leaf.Size() != n {
				return leaf.EvaluateOutOfDomainGnark(run, r), true
			}
			return leaf.EvaluateOutOfDomainGnarkAt(run.API(), r, rPowN), true
		}
		return circuit.Ext{}, false
	})
}

// cancellationRoots returns the constants ω_n^{norm(k)} for the cancelled rows.
func cancellationRoots(cancelled []int, n int) []field.Element {
	omega := field.RootOfUnityBy(n)
	roots := make([]field.Element, len(cancelled))
	for i, pos := range cancelled {
		k := pos
		if k < 0 {
			k = n + pos
		}
		field.ExpToInt(&roots[i], omega, k)
	}
	return roots
}
