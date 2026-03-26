package mpts

import (
	"fmt"
	"sync"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/fft/fastpoly"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/parallel"
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
		qr       = queryParams.Ys[len(va.NewQuery.Pols)-1]
		polysAtR = va.cptEvaluationMap(run)
		r        = queryParams.X
		rCoin    = run.GetRandomCoinField(va.EvaluationPoint.Name)

		// zetasOfR stores the values zetas[i] = lambda^i / (r - xi).
		// These values are precomputed for efficiency.
		zetasOfR = make([]field.Element, len(va.Queries))

		lambda = run.GetRandomCoinField(va.LinCombCoeffLambda.Name)
		rho    = run.GetRandomCoinField(va.LinCombCoeffRho.Name)
	)

	if r != rCoin {
		return fmt.Errorf("(verifier of %v) : Evaluation point of %v is incorrect (%v, expected %v)",
			va.NewQuery.QueryID, va.NewQuery.QueryID, r.String(), rCoin.String())
	}

	var (
		lambdaPowI = field.One()
		rhoK       = field.One()
		// res stores the right-hand of the equality check. Namely,
		// sum_{i,k \in claim} [\lambda^i \rho^k (Pk(r) - y_{ik})] / (r - xi).
		res = field.Zero()
	)

	for i, q := range va.Queries {

		xi := run.GetUnivariateParams(q.Name()).X
		zetasOfR[i].Sub(&r, &xi)
		// NB: this is very sub-optimal. We should use a batch-inverse instead
		// but the native verifier time is not very important in this context.
		zetasOfR[i].Inverse(&zetasOfR[i])
		zetasOfR[i].Mul(&zetasOfR[i], &lambdaPowI)
		lambdaPowI.Mul(&lambdaPowI, &lambda)
	}

	// This loop computes the value of [res]
	for k, p := range va.Polys {

		pr := polysAtR[p.GetColID()]

		for _, i := range va.EvalPointOfPolys[k] {
			// This sets tmp with the value of yik
			posOfYik := getPositionOfPolyInQueryYs(va.Queries[i], va.Polys[k])
			tmp := run.GetUnivariateParams(va.Queries[i].Name()).Ys[posOfYik]
			tmp.Sub(&pr, &tmp) // Pk(r) - y_{ik}
			tmp.Mul(&tmp, &zetasOfR[i])
			tmp.Mul(&tmp, &rhoK)
			res.Add(&res, &tmp)
		}

		rhoK.Mul(&rhoK, &rho)
	}

	if !res.Equal(&qr) {
		return fmt.Errorf("(verifier of %v) : Q(r) is incorrect (%v, expected %v)",
			va.NewQuery.QueryID, qr.String(), res.String())
	}

	return nil
}

func (va VerifierAction) RunGnark(api frontend.API, run wizard.GnarkRuntime) {

	var (
		queryParams = run.GetUnivariateParams(va.NewQuery.QueryID)
		// polyOfRs stores the values of P_k(r) as returned in the query.
		// The last value of the slice is the value of Q(r) where q
		// is the quotient polynomial.
		qr       = queryParams.Ys[len(va.NewQuery.Pols)-1]
		polysAtR = va.cptEvaluationMapGnark(api, run)
		r        = queryParams.X
		rCoin    = run.GetRandomCoinField(va.EvaluationPoint.Name)

		// zetasOfR stores the values zetas[i] = lambda^i / (r - xi).
		// These values are precomputed for efficiency.
		zetasOfR = make([]frontend.Variable, len(va.Queries))

		lambda = run.GetRandomCoinField(va.LinCombCoeffLambda.Name)
		rho    = run.GetRandomCoinField(va.LinCombCoeffRho.Name)
	)

	api.AssertIsEqual(r, rCoin)

	var (
		lambdaPowI = frontend.Variable(1)
		rhoK       = frontend.Variable(1)
		// res stores the right-hand of the equality check. Namely,
		// sum_{i,k \in claim} [\lambda^i \rho^k (Pk(r) - y_{ik})] / (r - xi).
		res = frontend.Variable(0)
	)

	for i, q := range va.Queries {

		xi := run.GetUnivariateParams(q.Name()).X
		zetasOfR[i] = api.Sub(r, xi)
		// NB: this is very sub-optimal. We should use a batch-inverse instead
		// but the native verifier time is not very important in this context.
		zetasOfR[i] = api.Inverse(zetasOfR[i])
		zetasOfR[i] = api.Mul(zetasOfR[i], lambdaPowI)
		lambdaPowI = api.Mul(lambdaPowI, lambda)
	}

	// This loop computes the value of [res]
	for k, p := range va.Polys {
		pr := polysAtR[p.GetColID()]
		for _, i := range va.EvalPointOfPolys[k] {
			// This sets tmp with the value of yik
			posOfYik := getPositionOfPolyInQueryYs(va.Queries[i], va.Polys[k])
			tmp := run.GetUnivariateParams(va.Queries[i].Name()).Ys[posOfYik]
			tmp = api.Sub(pr, tmp) // Pk(r) - y_{ik}
			tmp = api.Mul(tmp, zetasOfR[i])
			tmp = api.Mul(tmp, rhoK)
			res = api.Add(res, tmp)
		}

		rhoK = api.Mul(rhoK, rho)
	}

	api.AssertIsEqual(res, qr)
}

// cptEvaluationMap returns an evaluation map [Column] -> [Y] for all the
// polynomials handled by [ctx]. This includes the columns of the new query
// but also the explictly evaluated columns.
func (ctx *MultipointToSinglepointCompilation) cptEvaluationMap(run wizard.Runtime) map[ifaces.ColID]field.Element {

	var (
		evaluationMap = make(map[ifaces.ColID]field.Element, len(ctx.NewQuery.Pols)+len(ctx.ExplicitlyEvaluated))
		univParams    = run.GetParams(ctx.NewQuery.QueryID).(query.UnivariateEvalParams)
		x             = univParams.X
		lock          = sync.Mutex{}
	)

	for i := range ctx.NewQuery.Pols {
		colID := ctx.NewQuery.Pols[i].GetColID()
		evaluationMap[colID] = univParams.Ys[i]
	}

	parallel.Execute(len(ctx.ExplicitlyEvaluated), func(start, stop int) {
		for i := start; i < stop; i++ {
			colID := ctx.ExplicitlyEvaluated[i].GetColID()
			poly := ctx.ExplicitlyEvaluated[i].GetColAssignment(run)
			val := smartvectors.Interpolate(poly, x)

			lock.Lock()
			evaluationMap[colID] = val
			lock.Unlock()
		}
	})

	return evaluationMap
}

// cptEvaluationMapGnark is the same as [cptEvaluationMap] but for a gnark circuit.
func (ctx *MultipointToSinglepointCompilation) cptEvaluationMapGnark(api frontend.API, run wizard.GnarkRuntime) map[ifaces.ColID]frontend.Variable {

	var (
		evaluationMap = make(map[ifaces.ColID]frontend.Variable)
		univParams    = run.GetUnivariateParams(ctx.NewQuery.QueryID)
		x             = univParams.X
		polys         = make([][]frontend.Variable, 0)
	)

	for i := range ctx.NewQuery.Pols {
		colID := ctx.NewQuery.Pols[i].GetColID()
		evaluationMap[colID] = univParams.Ys[i]
	}

	for _, c := range ctx.ExplicitlyEvaluated {

		// When encountering a verifiercol.ConstCol, the optimization is to
		// represent the column not to its full length but as a length 1 column
		// which will yield the same result in the end.
		if constCol, isConstCol := c.(verifiercol.ConstCol); isConstCol {
			polys = append(polys, []frontend.Variable{constCol.F})
			continue
		}

		// When encountering a verifiercol.RepeatCol, the optimization is to
		// represent the column not to its full length but as a length 1 column
		// which will yield the same result in the end.
		if repeatCol, isRepeatCol := c.(verifiercol.RepeatedAccessor); isRepeatCol {
			y := repeatCol.Accessor.GetFrontendVariable(api, run)
			polys = append(polys, []frontend.Variable{y})
			continue
		}

		poly := c.GetColAssignmentGnark(run)
		polys = append(polys, poly)
	}

	ys := fastpoly.BatchInterpolateGnark(api, polys, x)

	for i := range ctx.ExplicitlyEvaluated {
		colID := ctx.ExplicitlyEvaluated[i].GetColID()
		evaluationMap[colID] = ys[i]
	}

	return evaluationMap
}
