package mpts

import (
	"fmt"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/maths/common/fastpolyext"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
	"github.com/consensys/linea-monorepo/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
)

// VerifierAction implements [wizard.VerifierAction]. It is tasked with
//   - (1) checking that the X value of the univariate query is correct.
//   - (2) checking that q(r) = sum_{i,k \in claim} [\lambda^i \rho^k (Pk(r) - y_{ik})] / (r - xi).
type VerifierAction struct {
	*MultipointToSinglepointCompilation
}

func (va VerifierAction) Run(run wizard.Runtime) error {

	var (
		queryParams = run.GetUnivariateParams(va.NewQuery.QueryID)
		// _polyOfRs stores the values of P_k(r) as returned in the query.
		// The last value of the slice is the value of Q(r) where q
		// is the quotient polynomial.
		//
		// However, the other values cannot be directly used by the verifier
		// and should instead use the ysMap.
		qr       = queryParams.ExtYs[len(va.NewQuery.Pols)-1]
		polysAtR = va.cptEvaluationMapExt(run)
		r        = queryParams.ExtX
		rCoin    = run.GetRandomCoinFieldExt(va.EvaluationPoint.Name)

		// zetasOfR stores the values zetas[i] = lambda^i / (r - xi).
		// These values are precomputed for efficiency.
		zetasOfR = make([]fext.Element, len(va.Queries))

		lambda = run.GetRandomCoinFieldExt(va.LinCombCoeffLambda.Name)
		rho    = run.GetRandomCoinFieldExt(va.LinCombCoeffRho.Name)
	)

	if r != rCoin {
		return fmt.Errorf("(verifier of %v) : Evaluation point of %v is incorrect (%v, expected %v)",
			va.NewQuery.QueryID, va.NewQuery.QueryID, r.String(), rCoin.String())
	}

	var (
		lambdaPowI = fext.One()
		// res stores the right-hand of the equality check. Namely,
		// sum_{i,k \in claim} [\lambda^i \rho^k (Pk(r) - y_{ik})] / (r - xi).
		res        = fext.Zero()
		numQueries = len(va.Queries)
	)

	// Step 1: Compute zeta[i] = λⁱ / (r - xᵢ) for each evaluation point
	for i, q := range va.Queries {
		xi := run.GetUnivariateParams(q.Name()).ExtX
		zetasOfR[i].Sub(&r, &xi)
		// NB: this is very sub-optimal. We should use a batch-inverse instead
		// but the native verifier time is not very important in this context.
		zetasOfR[i].Inverse(&zetasOfR[i])
		zetasOfR[i].Mul(&zetasOfR[i], &lambdaPowI)
		lambdaPowI.Mul(&lambdaPowI, &lambda)
	}

	// Precompute powers of rho
	const maxCachedGap = 8
	rhoCache := make([]fext.Element, maxCachedGap+1)
	rhoCache[0] = fext.One()
	rhoCache[1] = rho
	for g := 2; g <= maxCachedGap; g++ {
		rhoCache[g].Mul(&rhoCache[g-1], &rho)
	}

	// Step 2: For each evaluation point, compute contribution using Horner's method
	for i := 0; i < numQueries; i++ {
		polyIndices := va.PolysOfEvalPoint[i]
		if len(polyIndices) == 0 {
			continue
		}

		n := len(polyIndices)

		// Start from last polynomial (highest index)
		lastIdx := n - 1
		lastK := polyIndices[lastIdx]
		lastP := va.Polys[lastK]
		lastPr := polysAtR[lastP.GetColID()]
		posOfY := va.PolyPositionInQuery[i][lastIdx]
		lastY := run.GetUnivariateParams(va.Queries[i].Name()).ExtYs[posOfY]
		var pointSum fext.Element
		pointSum.Sub(&lastPr, &lastY)

		// Process remaining polynomials in reverse order using Horner's method
		for j := n - 2; j >= 0; j-- {
			k := polyIndices[j]
			nextK := polyIndices[j+1]
			gap := nextK - k

			// pointSum = ρ^gap * pointSum + (Pₖ(r) - yₖᵢ)
			var rhoGap fext.Element
			if gap <= maxCachedGap {
				rhoGap = rhoCache[gap]
			} else {
				rhoGap = computeRhoPowerNative(rho, gap)
			}
			pointSum.Mul(&pointSum, &rhoGap)

			p := va.Polys[k]
			pr := polysAtR[p.GetColID()]
			posOfY = va.PolyPositionInQuery[i][j]
			yik := run.GetUnivariateParams(va.Queries[i].Name()).ExtYs[posOfY]
			var diff fext.Element
			diff.Sub(&pr, &yik)
			pointSum.Add(&pointSum, &diff)
		}

		// Multiply by ρ^(first_k) to get correct weighting
		firstK := polyIndices[0]
		if firstK > 0 {
			var rhoFirst fext.Element
			if firstK <= maxCachedGap {
				rhoFirst = rhoCache[firstK]
			} else {
				rhoFirst = computeRhoPowerNative(rho, firstK)
			}
			pointSum.Mul(&pointSum, &rhoFirst)
		}

		// Multiply by zeta[i] and add to result
		var contribution fext.Element
		contribution.Mul(&pointSum, &zetasOfR[i])
		res.Add(&res, &contribution)
	}

	if !res.Equal(&qr) {
		return fmt.Errorf("(verifier of %v) : Q(r) is incorrect (%v, expected %v)",
			va.NewQuery.QueryID, qr.String(), res.String())
	}

	return nil
}

func (va VerifierAction) RunGnark(api frontend.API, run wizard.GnarkRuntime) {
	// Formula: qr = Σᵢ ζᵢ * [Σₖ∈polys(i) ρᵏ * (Pₖ(r) - yₖᵢ)]
	// where ζᵢ = λⁱ / (r - xᵢ)  (Point-specific weight)
	//
	// 	Optimizations:
	//  1. Factorization: We group evaluations by point (i) rather than polynomial (k) to
	//  	precompute the complex barycentric weights (ζᵢ) once per query point.
	//  2. Recursive Horner's Method: Given N polynomials, instead of a naive sum which requires N MulExts
	//  to compute powers of ρ and another N MulExts to scale the differences, we use
	//  Horner's method in reverse.
	//
	//  Logical Transformation:
	//  - Naive:  ρ⁰d₀ + ρ¹d₁ + ρ²d₂ + ... + ρⁿdₙ  (~2N MulExt)
	//  - Horner: d₀ + ρ(d₁ + ρ(d₂ + ... ρ(dₙ₋₁ + ρ dₙ)...)) (~N MulExt)
	//

	var (
		queryParams = run.GetUnivariateParams(va.NewQuery.QueryID)
		qr          = queryParams.ExtYs[len(va.NewQuery.Pols)-1]
		polysAtR    = va.cptEvaluationMapGnarkExt(api, run)
		r           = queryParams.ExtX
		rCoin       = run.GetRandomCoinFieldExt(va.EvaluationPoint.Name)
		lambda      = run.GetRandomCoinFieldExt(va.LinCombCoeffLambda.Name)
		rho         = run.GetRandomCoinFieldExt(va.LinCombCoeffRho.Name)
		numQueries  = len(va.Queries)
	)

	koalaAPI := koalagnark.NewAPI(api)
	koalaAPI.AssertIsEqualExt(r, rCoin)

	// Step 1: Compute zeta[i] = λⁱ / (r - xᵢ) for each evaluation point
	// Cost: n DivExt + (n-1) MulExt
	lambdaPowers := make([]koalagnark.Ext, numQueries)
	lambdaPowers[0] = koalaAPI.OneExt()
	for i := 1; i < numQueries; i++ {
		lambdaPowers[i] = koalaAPI.MulExt(lambdaPowers[i-1], lambda)
	}

	zetasOfR := make([]koalagnark.Ext, numQueries)
	for i, q := range va.Queries {
		xi := run.GetUnivariateParams(q.Name()).ExtX
		rMinusXi := koalaAPI.SubExt(r, xi)
		zetasOfR[i] = koalaAPI.DivExt(lambdaPowers[i], rMinusXi)
	}

	// Step 2: For each evaluation point, compute contribution using Horner's method
	// This avoids precomputing all ρ^k powers!
	//
	// For point i with polys [k₀, k₁, k₂, ...] (sorted ascending):
	// Σₖ ρᵏ(Pₖ(r) - yₖᵢ) = ρ^k₀ * (d₀ + ρ^(k₁-k₀)(d₁ + ρ^(k₂-k₁)(d₂ + ...)))
	//
	// Using Horner in reverse: start from last poly, accumulate backwards

	res := koalaAPI.ZeroExt()

	// Precompute small powers of rho for common gaps (1, 2, 3, ...)
	// This avoids repeated square-and-multiply for common cases
	const maxCachedGap = 8
	rhoCache := make([]koalagnark.Ext, maxCachedGap+1)
	rhoCache[0] = koalaAPI.OneExt()
	rhoCache[1] = rho
	for g := 2; g <= maxCachedGap; g++ {
		rhoCache[g] = koalaAPI.MulExt(rhoCache[g-1], rho)
	}

	for i := 0; i < numQueries; i++ {
		polyIndices := va.PolysOfEvalPoint[i]
		if len(polyIndices) == 0 {
			continue
		}

		n := len(polyIndices)

		// Start from last polynomial (highest index)
		lastIdx := n - 1
		lastK := polyIndices[lastIdx]
		lastP := va.Polys[lastK]
		lastPr := polysAtR[lastP.GetColID()]
		posOfY := va.PolyPositionInQuery[i][lastIdx]
		lastY := run.GetUnivariateParams(va.Queries[i].Name()).ExtYs[posOfY]
		pointSum := koalaAPI.SubExt(*lastPr, lastY)

		// Process remaining polynomials in reverse order using Horner's method
		for j := n - 2; j >= 0; j-- {
			k := polyIndices[j]
			nextK := polyIndices[j+1]
			gap := nextK - k

			// pointSum = ρ^gap * pointSum + (Pₖ(r) - yₖᵢ)
			var rhoGap koalagnark.Ext
			if gap <= maxCachedGap {
				rhoGap = rhoCache[gap]
			} else {
				rhoGap = computeRhoPower(koalaAPI, rho, gap, rhoCache)
			}
			pointSum = koalaAPI.MulExt(pointSum, rhoGap)

			p := va.Polys[k]
			pr := polysAtR[p.GetColID()]
			posOfY = va.PolyPositionInQuery[i][j]
			yik := run.GetUnivariateParams(va.Queries[i].Name()).ExtYs[posOfY]
			diff := koalaAPI.SubExt(*pr, yik)
			pointSum = koalaAPI.AddExt(pointSum, diff)
		}

		// Multiply by ρ^(first_k) to get correct weighting
		firstK := polyIndices[0]
		if firstK > 0 {
			var rhoFirst koalagnark.Ext
			if firstK <= maxCachedGap {
				rhoFirst = rhoCache[firstK]
			} else {
				rhoFirst = computeRhoPower(koalaAPI, rho, firstK, rhoCache)
			}
			pointSum = koalaAPI.MulExt(pointSum, rhoFirst)
		}

		// Multiply by zeta[i] and add to result
		contribution := koalaAPI.MulExt(pointSum, zetasOfR[i])
		res = koalaAPI.AddExt(res, contribution)
	}

	koalaAPI.AssertIsEqualExt(res, qr)
}

// computeRhoPowerNative computes rho^n using square-and-multiply for native field
func computeRhoPowerNative(rho fext.Element, n int) fext.Element {
	result := fext.One()
	base := rho

	for n > 0 {
		if n&1 == 1 {
			result.Mul(&result, &base)
		}
		n >>= 1
		if n > 0 {
			base.Mul(&base, &base)
		}
	}
	return result
}

// computeRhoPower computes rho^n using square-and-multiply with cache
func computeRhoPower(koalaAPI *koalagnark.API, rho koalagnark.Ext, n int, cache []koalagnark.Ext) koalagnark.Ext {
	if n < len(cache) {
		return cache[n]
	}

	// Square-and-multiply
	result := koalaAPI.OneExt()
	base := rho

	for n > 0 {
		if n&1 == 1 {
			result = koalaAPI.MulExt(result, base)
		}
		n >>= 1
		if n > 0 {
			base = koalaAPI.MulExt(base, base)
		}
	}
	return result
}

// cptEvaluationMap returns an evaluation map [Column] -> [Y] for all the
// polynomials handled by [ctx]. This includes the columns of the new query
// but also the explictly evaluated columns.
func (ctx *MultipointToSinglepointCompilation) cptEvaluationMapExt(run wizard.Runtime) map[ifaces.ColID]fext.Element {

	var (
		evaluationMap = make(map[ifaces.ColID]fext.Element)
		univParams    = run.GetParams(ctx.NewQuery.QueryID).(query.UnivariateEvalParams)
		x             = univParams.ExtX
	)

	for i := range ctx.NewQuery.Pols {
		colID := ctx.NewQuery.Pols[i].GetColID()
		evaluationMap[colID] = univParams.ExtYs[i]
	}
	for i, c := range ctx.ExplicitlyEvaluated {
		colID := ctx.ExplicitlyEvaluated[i].GetColID()
		poly := c.GetColAssignment(run)

		evaluationMap[colID] = smartvectors.EvaluateFextPolyLagrange(poly, x)
	}

	return evaluationMap
}

// cptEvaluationMapGnark is the same as [cptEvaluationMap] but for a gnark circuit.
func (ctx *MultipointToSinglepointCompilation) cptEvaluationMapGnarkExt(api frontend.API, run wizard.GnarkRuntime) map[ifaces.ColID]*koalagnark.Ext {

	var (
		evaluationMap = make(map[ifaces.ColID]*koalagnark.Ext)
		univParams    = run.GetUnivariateParams(ctx.NewQuery.QueryID)
		x             = univParams.ExtX
		polys         = make([][]*koalagnark.Ext, 0)
		koalaAPI      = koalagnark.NewAPI(api)
	)

	for i := range ctx.NewQuery.Pols {
		colID := ctx.NewQuery.Pols[i].GetColID()
		evaluationMap[colID] = &univParams.ExtYs[i]
	}

	for _, c := range ctx.ExplicitlyEvaluated {

		if constCol, isConstCol := c.(verifiercol.ConstCol); isConstCol {
			// Use ConstExt instead of NewExt to properly create circuit constants.
			// NewExt uses emulated.ValueOf which doesn't work during circuit definition.
			v := koalaAPI.ConstExt(constCol.F.GetExt())
			poly := []*koalagnark.Ext{&v}
			polys = append(polys, poly)
			continue
		}

		if repeated, isRepeated := c.(verifiercol.RepeatedAccessor); isRepeated {
			// Same optimization as for const columns. We can skip the Lagrange
			// evaluation for this one as the column is made of X times the
			// same value.
			fExt := repeated.Accessor.GetFrontendVariableExt(api, run)
			poly := []*koalagnark.Ext{&fExt}
			polys = append(polys, poly)
			continue
		}

		type mayProvidePtrDirectly interface {
			GetColAssignmentGnarkExtAsPtr(run ifaces.GnarkRuntime) []*koalagnark.Ext
		}

		if mayPtr, ok := c.(mayProvidePtrDirectly); ok {
			polys = append(polys, mayPtr.GetColAssignmentGnarkExtAsPtr(run))
			continue
		}

		poly := c.GetColAssignmentGnarkExt(run)
		polyPtr := make([]*koalagnark.Ext, len(poly))
		for i := range poly {
			polyPtr[i] = &poly[i]
		}
		polys = append(polys, polyPtr)
	}

	ys := fastpolyext.BatchEvaluateLagrangeGnark(api, polys, x)

	for i := range ctx.ExplicitlyEvaluated {
		colID := ctx.ExplicitlyEvaluated[i].GetColID()
		evaluationMap[colID] = &ys[i]
	}

	return evaluationMap
}
