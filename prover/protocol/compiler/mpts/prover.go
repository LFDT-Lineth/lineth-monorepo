package mpts

import (
	"sync"

	"github.com/consensys/linea-monorepo/prover/maths/common/mempool"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/fft"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/parallel"
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
		quotient = make([]field.Element, qa.getNumRow())

		// powersOfRho lists all the powers of rho and are precomputed to help
		// parallelization.
		powersOfRho = vector.PowerVec(rho, len(qa.Polys))

		// mempool is a memory pool that is used to allocate and reuse memory
		// for the partial results.
		memPool = mempool.CreateFromSyncPool(qa.getNumRow())

		// quotientLock protects [quotientOfSizes]
		quotientLock = &sync.Mutex{}

		// foundNonConstantPoly indicates whether any of the assignments of
		// [Polys] is not constant. It is evaluated on the fly during the
		// first loop.
		foundNonConstantPoly = false
	)

	// The first part of the algorithm is to compute the terms of the form:
	// \sum_{k, i \in claims} \rho^k \lambda^i P(X) / (X - xi)
	parallel.ExecuteChunky(len(qa.Polys), func(start, stop int) {

		// This creates a thread-local memory pool that does not rely on sync
		// and is a little faster.
		memPool := mempool.WrapsWithMemCache(memPool)

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

			foundNonConstantPoly = true

			var (
				poly                    = polySV.IntoRegVecSaveAlloc()
				polyPtr                 *[]field.Element
				pointsOfPoly            = qa.EvalPointOfPolys[polyID]
				localPartialQuotientPtr = memPool.Alloc()
				localPartialQuotient    = *localPartialQuotientPtr
			)

			if len(poly) < qa.getNumRow() {
				polyPtr = ldeOf(poly, memPool)
				poly = *polyPtr
			}

			for j, queryID := range pointsOfPoly {
				zeta := zetas[queryID]

				// For the first term, we do not need to add anything to accumulate
				// but this is not just an optimization: since partialQuotient is
				// allocated from the pool, we cannot assume it was already zeroed.
				if j == 0 {
					copy(localPartialQuotient, zeta)
					continue
				}

				for k := 0; k < len(poly); k++ {
					localPartialQuotient[k].Add(&localPartialQuotient[k], &zeta[k])
				}
			}

			for k := range localPartialQuotient {
				localPartialQuotient[k].Mul(&localPartialQuotient[k], &poly[k])
				localPartialQuotient[k].Mul(&localPartialQuotient[k], &powersOfRho[polyID])
			}

			// This part of the algorithm cannot be parallelized or there
			// would be race condition. Expectedly, this amounts to a very
			// small part of the computation.
			{
				quotientLock.Lock()
				vector.Add(quotient, quotient, localPartialQuotient)
				quotientLock.Unlock()
			}

			// Since the pool is "manual", we need to free the memory allocated
			// manually.
			memPool.Free(localPartialQuotientPtr)

			if polyPtr != nil {
				memPool.Free(polyPtr)
			}
		}
	})

	// This clause addresses the edge-case where all the [Polys] are
	// constant. In that case, the quotient is always the constant zero
	// and we can early return with a default assignment to zero.
	if !foundNonConstantPoly {
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

		var (
			localResultPtr = memPool.Alloc()
			localResult    = *localResultPtr
		)

		for i := start; i < stop; i++ {

			// The first step is to compute the \sum_k \rho^k y_{i,k}. This is
			// pure scalar operation.
			var (
				sumRhoKYik = field.Zero()
				zetaI      = zetas[i]
			)

			for _, k := range qa.PolysOfEvalPoint[i] {

				// Constant polys do not contribute to the quotient as their
				// respective quotient cancels out.
				polySV := qa.Polys[k].GetColAssignment(run)
				if _, ok := polySV.(*smartvectors.Constant); ok {
					continue
				}

				var (
					paramsI  = run.GetUnivariateParams(qa.Queries[i].Name())
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
			vector.ScalarMul(zetaI, zetaI, sumRhoKYik)

			if len(localResult) != len(zetaI) {
				utils.Panic("len(localResult) = %v len(zetaI) = %v", len(localResult), len(zetaI))
			}

			vector.Add(localResult, localResult, zetaI)
		}

		quotientLock.Lock()
		vector.Sub(quotient, quotient, localResult)
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
	for i := range ys {
		ys[i] = smartvectors.Interpolate(polyVals[i], r)
	}

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
		omega = fft.GetOmega(n)
		res   = make([]field.Element, n)
	)

	res[0] = field.One()

	for i := 1; i < n; i++ {
		res[i].Mul(&res[i-1], &omega)
	}

	return res
}

// ldeOf computes the low-degree extension of a vector and allocates the result
// in the pool. The size of the result is the same as the size of the pool.
func ldeOf(v []field.Element, pool mempool.MemPool) *[]field.Element {

	var (
		sizeLarge   = pool.Size()
		domainSmall = fft.NewDomain(len(v))
		domainLarge = fft.NewDomain(sizeLarge)
		resPtr      = pool.Alloc()
		res         = *resPtr
	)

	vector.Fill(res, field.Zero())
	copy(res[:len(v)], v)

	// Note: this implementation is very suboptimal as it should be possible
	// reduce the overheads of bit-reversal with a smarter implementation.
	// To be digged in the future, if this comes up as a bottleneck.
	domainSmall.FFTInverse(res[:len(v)], fft.DIF)
	fft.BitReverse(res[:len(v)])
	domainLarge.FFT(res, fft.DIF)
	fft.BitReverse(res)

	return resPtr
}

func getPositionOfPolyInQueryYs(q query.UnivariateEval, poly ifaces.Column) int {

	for i, p := range q.Pols {
		if p.GetColID() == poly.GetColID() {
			return i
		}
	}

	utils.Panic("not found, poly=%v in query=%v", poly.GetColID(), q.Name())
	return 0
}
