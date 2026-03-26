package mpts

import (
	"sync"
	"sync/atomic"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/parallel"

	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr/fft"
)

// QuotientAccumulation is a [wizard.ProverAction] that accumulates the
// quotient polynomial and assigns it.
type QuotientAccumulation struct {
	*MultipointToSinglepointCompilation
}

// RandomPointEvaluation is a [wizard.ProverAction] that evaluates the
// polynomial at a random point.
type RandomPointEvaluation struct {
	*MultipointToSinglepointCompilation
}

func (qa QuotientAccumulation) Run(run *wizard.ProverRuntime) {

	var (
		rho = run.GetRandomCoinField(qa.LinCombCoeffRho.Name)

		// zetas stores the values zetas[i] = lambda^i / (X - xi)
		// where xi is the i-th evaluation point. Having these values precomputed
		// allows to greatly speed-up the computation. This comes with a trade-off
		// in space as these can be big if the number of queries is big.
		zetas = qa.computeZetas(run)

		// quotient stores the assignment of the quotient polynomial as it is
		// being computed.
		quotient = make(field.Vector, qa.getNumRow())

		// powersOfRho lists all the powers of rho and are precomputed to help
		// parallelization.
		powersOfRho = vector.PowerVec(rho, len(qa.Polys))

		// quotientLock protects [quotientOfSizes]
		quotientLock = &sync.Mutex{}

		// foundNonConstantPoly indicates whether any of the assignments of
		// [Polys] is not constant. It is evaluated on the fly during the
		// first loop.
		foundNonConstantPoly = int64(0)
	)

	// The first part of the algorithm is to compute the terms of the form:
	// \sum_{k, i \in claims} \rho^k \lambda^i P(X) / (X - xi)
	parallel.Execute(len(qa.Polys), func(start, stop int) {

		localPartialQuotient := make(field.Vector, qa.getNumRow())
		localRes := make(field.Vector, qa.getNumRow())

		for polyID := start; polyID < stop; polyID++ {

			polySV := qa.Polys[polyID].GetColAssignment(run)

			// Constant terms do not contribute to the quotient as they get
			// cancelled out by "y".
			if _, ok := polySV.(*smartvectors.Constant); ok {
				continue
			}

			// If the poly has no corresponding evaluation point, this will
			// also not contribute to the quotient.
			if len(qa.EvalPointOfPolys[polyID]) == 0 {
				continue
			}

			atomic.StoreInt64(&foundNonConstantPoly, 1) // at least one non-constant poly
			pointsOfPoly := qa.EvalPointOfPolys[polyID]

			copy(localPartialQuotient, zetas[pointsOfPoly[0]])
			for j := 1; j < len(pointsOfPoly); j++ {
				zeta := zetas[pointsOfPoly[j]]
				localPartialQuotient.Add(localPartialQuotient, zeta)
			}

			if polySV.Len() < qa.getNumRow() {
				poly := make(field.Vector, qa.getNumRow())
				polySV.WriteInSlice(poly[:polySV.Len()])
				_ldeOf(poly, polySV.Len(), qa.getNumRow())
				localPartialQuotient.Mul(localPartialQuotient, poly)
			} else {
				poly := polySV.IntoRegVecSaveAlloc()
				localPartialQuotient.Mul(localPartialQuotient, poly)
			}

			localPartialQuotient.ScalarMul(localPartialQuotient, &powersOfRho[polyID])
			localRes.Add(localRes, localPartialQuotient)
		}

		quotientLock.Lock()
		quotient.Add(quotient, localRes)
		quotientLock.Unlock()
	})

	// This clause addresses the edge-case where all the [Polys] are
	// constant. In that case, the quotient is always the constant zero
	// and we can early return with a default assignment to zero.
	if foundNonConstantPoly == 0 {
		run.AssignColumn(
			qa.Quotient.GetColID(),
			smartvectors.NewConstant(field.Zero(), qa.getNumRow()),
		)
		return
	}

	// The second part of the algorithm computes  \sum_{k, i \in claims}
	// \rho^k \lambda^i y_i / (X - xi). All results are added to the last
	// entry of the quotient. This part is done separately from the first
	// because it is optimized differently.
	parallel.Execute(len(qa.Queries), func(start, stop int) {

		localResult := make(field.Vector, qa.getNumRow())

		for i := start; i < stop; i++ {

			// The first step is to compute the \sum_k \rho^k y_{i,k}. This is
			// pure scalar operation.
			var (
				sumRhoKYik = field.Zero()
				zetaI      = field.Vector(zetas[i])
			)

			for _, k := range qa.PolysOfEvalPoint[i] {

				// Constant polys do not contribute to the quotient as their
				// respective quotient cancels out.
				polySV := qa.Polys[k].GetColAssignment(run)
				if _, ok := polySV.(*smartvectors.Constant); ok {
					continue
				}

				var (
					paramsI = run.GetUnivariateParams(qa.Queries[i].Name())
					// TODO @gbotrel this slows thing down, build a smart lookup or improve search.
					posOfYik = getPositionOfPolyInQueryYs(qa.Queries[i], qa.Polys[k])
					yik      = paramsI.Ys[posOfYik]
				)

				// This reuses the memory slot of yik to compute the temporary
				// rho^k y_ik
				yik.Mul(&yik, &powersOfRho[k])
				sumRhoKYik.Add(&sumRhoKYik, &yik)
			}

			// The second step is to multiply and accumulate the result by zetaI
			// and sumRhoKYik. This part "comsumes" the value of zetaI.
			zetaI.ScalarMul(zetaI, &sumRhoKYik)

			if len(localResult) != len(zetaI) {
				utils.Panic("len(localResult) = %v len(zetaI) = %v", len(localResult), len(zetaI))
			}

			localResult.Add(localResult, zetaI)
		}

		quotientLock.Lock()
		quotient.Sub(quotient, localResult)
		quotientLock.Unlock()
	})

	run.AssignColumn(qa.Quotient.GetColID(), smartvectors.NewRegular(quotient))
}

func (re RandomPointEvaluation) Run(run *wizard.ProverRuntime) {

	var (
		r        = run.GetRandomCoinField(re.EvaluationPoint.Name)
		polys    = re.NewQuery.Pols
		polyVals = make([]smartvectors.SmartVector, len(polys))
	)

	for i := range polyVals {
		polyVals[i] = polys[i].GetColAssignment(run)
	}

	ys := make([]field.Element, len(polyVals))
	parallel.Execute(len(ys), func(start, stop int) {
		for i := start; i < stop; i++ {
			ys[i] = smartvectors.Interpolate(polyVals[i], r)
		}
	})

	run.AssignUnivariate(re.NewQuery.QueryID, r, ys...)
}

// computeZetas returns the values of zeta_i = lambda^i / (X - xi)
// for each query. And returns an evaluation vector for each query for all powers
// of omega.
func (qa QuotientAccumulation) computeZetas(run *wizard.ProverRuntime) [][]field.Element {

	var (
		// powersOfOmega is the list of the powers of omega starting from 0.
		powersOfOmega = getPowersOfOmega(qa.getNumRow())
		zetaI         = make([][]field.Element, len(qa.Queries))
		lambda        = run.GetRandomCoinField(qa.LinCombCoeffLambda.Name)
		// powersOfLambda are precomputed outside of the loop to allow for
		// parallization.
		powersOfLambda = vector.PowerVec(lambda, len(qa.Queries))
	)

	parallel.Execute(len(qa.Queries), func(start, stop int) {
		for i := start; i < stop; i++ {

			var (
				q      = qa.Queries[i]
				params = run.GetUnivariateParams(q.Name())
				xi     = params.X

				// l is the value of lambda^i / (X - xi). It is computed by:
				// 	1 - Deep copying the powers of omega
				//  2 - Substracting xi to each entry
				//  3 - Batch inverting the result
				//  4 - Multiplying the result by lambdaPowi
				l = append([]field.Element{}, powersOfOmega...)
			)

			for j := range l {
				l[j].Sub(&l[j], &xi)
			}

			l = field.BatchInvert(l)

			for j := range l {
				l[j].Mul(&l[j], &powersOfLambda[i])
			}

			zetaI[i] = l
		}
	})

	return zetaI
}

// getPowersOfOmega returns the list of the powers of omega, where omega is a root
// of unity of order n.
func getPowersOfOmega(n int) []field.Element {

	var (
		omega, _ = fft.Generator(uint64(n))
		res      = make([]field.Element, n)
	)

	res[0] = field.One()

	for i := 1; i < n; i++ {
		res[i].Mul(&res[i-1], &omega)
	}

	return res
}

func _ldeOf(v []field.Element, sizeSmall, sizeLarge int) {
	domainSmall := fft.NewDomain(uint64(sizeSmall), fft.WithCache())
	domainLarge := fft.NewDomain(uint64(sizeLarge), fft.WithCache())

	domainSmall.FFTInverse(v[:sizeSmall], fft.DIF, fft.WithNbTasks(1))
	fft.BitReverse(v[:sizeSmall])
	domainLarge.FFT(v, fft.DIF, fft.WithNbTasks(1))
	fft.BitReverse(v)
}

func getPositionOfPolyInQueryYs(q query.UnivariateEval, poly ifaces.Column) int {
	// TODO @gbotrel this appears on the traces quite a lot -- lot of string comparisons
	toFind := poly.GetColID()
	for i, p := range q.Pols {
		if p.GetColID() == toFind {
			return i
		}
	}

	utils.Panic("not found, poly=%v in query=%v", toFind, q.Name())
	return 0
}
