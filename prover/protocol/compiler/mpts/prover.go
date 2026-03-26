package mpts

import (
	"sync"

	"github.com/consensys/gnark-crypto/field/koalabear/extensions"
	"github.com/consensys/gnark-crypto/field/koalabear/fft"
	gutils "github.com/consensys/gnark-crypto/utils"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors_mixed"
	"github.com/consensys/linea-monorepo/prover/maths/common/vectorext"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils/arena"
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
		rho = run.GetRandomCoinFieldExt(qa.LinCombCoeffRho.Name)

		// zetas stores the values zetas[i] = lambda^i / (X - xi)
		// where xi is the i-th evaluation point. Having these values precomputed
		// allows to greatly speed-up the computation. This comes with a trade-off
		// in space as these can be big if the number of queries is big.
		zetas = qa.computeZetasExt(run)

		// quotient stores the assignment of the quotient polynomial as it is
		// being computed.
		quotient = make(extensions.Vector, qa.getNumRow())

		// powersOfRho lists all the powers of rho and are precomputed to help
		// parallelization.
		powersOfRho = vectorext.PowerVec(rho, len(qa.Polys))

		// quotientLock protects [quotientOfSizes]
		quotientLock = &sync.Mutex{}

		// foundNonConstantPoly indicates whether any of the assignments of
		// [Polys] is not constant. It is evaluated on the fly during the
		// first loop.
		foundNonConstantPoly = false
	)

	// The first part of the algorithm is to compute the terms of the form:
	// \sum_{k, i \in claims} \rho^k \lambda^i P(X) / (X - xi)
	parallel.Execute(len(qa.Polys), func(start, stop int) {

		localRes := make(extensions.Vector, qa.getNumRow())

		// Buffers for reuse
		combinedPoly := make(extensions.Vector, qa.getNumRow())
		tmpPoly := make(extensions.Vector, qa.getNumRow())

		// Group polynomials by evaluation points
		type group struct {
			points  []int
			polyIDs []int
		}
		var groups []group

		hasWork := false

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

			hasWork = true
			pointsOfPoly := qa.EvalPointOfPolys[polyID]

			// Find group
			found := false
			for i := range groups {
				// Check equality of points
				if len(groups[i].points) != len(pointsOfPoly) {
					continue
				}
				match := true
				for k, p := range pointsOfPoly {
					if groups[i].points[k] != p {
						match = false
						break
					}
				}
				if match {
					groups[i].polyIDs = append(groups[i].polyIDs, polyID)
					found = true
					break
				}
			}
			if !found {
				groups = append(groups, group{points: pointsOfPoly, polyIDs: []int{polyID}})
			}
		}

		if hasWork {
			quotientLock.Lock()
			foundNonConstantPoly = true
			quotientLock.Unlock()
		}

		for _, g := range groups {
			polyIDs := g.polyIDs
			if len(polyIDs) == 0 {
				continue
			}

			// Compute combinedPoly = sum(rho^k * P_k)
			first := true
			for _, polyID := range polyIDs {
				polySV := qa.Polys[polyID].GetColAssignment(run)

				// Optimization for full-size regular vectors (base field)
				if reg, ok := polySV.(*smartvectors.Regular); ok && polySV.Len() == qa.getNumRow() {
					poly := field.Vector(*reg)
					rho := powersOfRho[polyID]

					if first {
						for i := 0; i < len(poly); i++ {
							combinedPoly[i] = rho
						}
						combinedPoly.MulByElement(combinedPoly, poly)
						// for i := 0; i < len(poly); i++ {
						// 	// combinedPoly[i] = rho * poly[i]
						// 	combinedPoly[i].B0.A0.Mul(&rho.B0.A0, &poly[i])
						// 	combinedPoly[i].B0.A1.Mul(&rho.B0.A1, &poly[i])
						// 	combinedPoly[i].B1.A0.Mul(&rho.B1.A0, &poly[i])
						// 	combinedPoly[i].B1.A1.Mul(&rho.B1.A1, &poly[i])
						// }
						first = false
					} else {
						combinedPoly.MulAccByElement(poly, &rho)
					}
					continue
				}

				if polySV.Len() < qa.getNumRow() {
					polySV.WriteInSliceExt(tmpPoly[:polySV.Len()])
					_ldeOfExt(tmpPoly, polySV.Len(), qa.getNumRow())
				} else {
					polySV.WriteInSliceExt(tmpPoly)
				}

				tmpPoly.ScalarMul(tmpPoly, &powersOfRho[polyID])

				if first {
					copy(combinedPoly, tmpPoly)
					first = false
				} else {
					combinedPoly.Add(combinedPoly, tmpPoly)
				}
			}

			// Reconstruct points from key or just take from first poly
			pointsOfPoly := g.points

			// Compute sumZeta using tmpPoly as buffer
			copy(tmpPoly, zetas[pointsOfPoly[0]])
			for j := 1; j < len(pointsOfPoly); j++ {
				tmpPoly.Add(tmpPoly, zetas[pointsOfPoly[j]])
			}

			// Multiply combinedPoly by sumZeta
			combinedPoly.Mul(combinedPoly, tmpPoly)

			// Add to localRes
			localRes.Add(localRes, combinedPoly)
		}

		quotientLock.Lock()
		quotient.Add(quotient, localRes)
		quotientLock.Unlock()
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

	// We first compute the scalars \sum_k \rho^k y_{i,k} for each query i.
	scalars := make(extensions.Vector, len(qa.Queries))
	parallel.Execute(len(qa.Queries), func(start, stop int) {
		for i := start; i < stop; i++ {
			var sumRhoKYik fext.Element
			for kIdx, polyID := range qa.PolysOfEvalPoint[i] {

				// Constant polys do not contribute to the quotient as their
				// respective quotient cancels out.
				polySV := qa.Polys[polyID].GetColAssignment(run)
				if _, ok := polySV.(*smartvectors.Constant); ok {
					continue
				}

				var (
					paramsI  = run.GetUnivariateParams(qa.Queries[i].Name())
					posOfYik = qa.PolyPositionInQuery[i][kIdx]
					yik      = paramsI.ExtYs[posOfYik]
				)

				// This reuses the memory slot of yik to compute the temporary
				// rho^k y_ik
				yik.Mul(&yik, &powersOfRho[polyID])
				sumRhoKYik.Add(&sumRhoKYik, &yik)
			}
			scalars[i] = sumRhoKYik
		}
	})

	// Then we accumulate the result into the quotient.
	// quotient -= \sum_i scalars[i] * zetas[i]
	// We parallelize over the rows of the quotient.
	parallel.Execute(qa.getNumRow(), func(start, stop int) {
		transposed := make(extensions.Vector, len(qa.Queries))
		for row := start; row < stop; row++ {
			// transpose zetas to access by row
			for i := 0; i < len(qa.Queries); i++ {
				transposed[i] = zetas[i][row]
			}
			sum := transposed.InnerProduct(scalars)
			quotient[row].Sub(&quotient[row], &sum)
		}
	})

	run.AssignColumn(qa.Quotient.GetColID(), smartvectors.NewRegularExt(quotient))
}

func (re RandomPointEvaluation) Run(run *wizard.ProverRuntime) {

	var (
		r        = run.GetRandomCoinFieldExt(re.EvaluationPoint.Name)
		polys    = re.NewQuery.Pols
		polyVals = make([]smartvectors.SmartVector, len(polys))
	)
	for i := range polyVals {
		polyVals[i] = polys[i].GetColAssignment(run)

	}

	ys := smartvectors_mixed.BatchEvaluateLagrange(polyVals, r)

	run.AssignUnivariateExt(re.NewQuery.QueryID, r, ys...)
}

func (qa QuotientAccumulation) computeZetasExt(run *wizard.ProverRuntime) []vectorext.Vector {

	var (
		// powersOfOmega is the list of the powers of omega starting from 0.
		powersOfOmega = getPowersOfOmegaExt(qa.getNumRow())
		zetaI         = make([]vectorext.Vector, len(qa.Queries))
		lambda        = run.GetRandomCoinFieldExt(qa.LinCombCoeffLambda.Name)
		// powersOfLambda are precomputed outside of the loop to allow for
		// parallization.
		powersOfLambda = vectorext.PowerVec(lambda, len(qa.Queries))
	)

	arenaExt := arena.NewVectorArena[fext.Element](len(powersOfOmega) * len(qa.Queries))

	parallel.Execute(len(qa.Queries), func(start, stop int) {
		for i := start; i < stop; i++ {

			q := qa.Queries[i]
			params := run.GetUnivariateParams(q.Name())
			xi := params.ExtX

			// l is the value of lambda^i / (X - xi). It is computed by:
			// 	1 - Deep copying the powers of omega
			//  2 - Substracting xi to each entry
			//  3 - Batch inverting the result
			//  4 - Multiplying the result by lambdaPowi
			l := arena.Get[fext.Element](arenaExt, len(powersOfOmega))
			vl := extensions.Vector(l)
			copy(vl, powersOfOmega)

			for j := range vl {
				vl[j].Sub(&vl[j], &xi)
			}
			vl = fext.ParBatchInvert(vl, 4)
			vl.ScalarMul(vl, &powersOfLambda[i])

			zetaI[i] = vl
		}
	})

	return zetaI
}

func getPowersOfOmegaExt(n int) []fext.Element {

	var (
		omega, _ = fft.Generator(uint64(n))
		res      = make([]fext.Element, n)
	)

	res[0] = fext.One()

	for i := 1; i < n; i++ {
		res[i].MulByElement(&res[i-1], &omega)
	}

	return res
}

func _ldeOfExt(v []fext.Element, sizeSmall, sizeLarge int) {
	domainSmall := fft.NewDomain(uint64(sizeSmall), fft.WithCache())
	domainLarge := fft.NewDomain(uint64(sizeLarge), fft.WithCache())

	domainSmall.FFTInverseExt(v[:sizeSmall], fft.DIF, fft.WithNbTasks(1))
	gutils.BitReverse(v[:sizeSmall])
	domainLarge.FFTExt(v, fft.DIF, fft.WithNbTasks(1))
	gutils.BitReverse(v)
}
