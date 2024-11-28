package univariates

import (
	"fmt"
	ppool "github.com/consensys/linea-monorepo/prover/utils/parallel/pool"
	"math/big"
	"reflect"
	"runtime"
	"sync"

	"github.com/consensys/gnark/frontend"
	"github.com/sirupsen/logrus"

	"github.com/consensys/linea-monorepo/prover/maths/common/mempool"
	"github.com/consensys/linea-monorepo/prover/maths/common/poly"
	sv "github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/fft"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/gnarkutil"
	"github.com/consensys/linea-monorepo/prover/utils/parallel"
	"github.com/consensys/linea-monorepo/prover/utils/profiling"
)

/*
Reduce all the univariate queries into a unique single point evaluation

See : https://eprint.iacr.org/2020/081.pdf (Section 3)
*/
func MultiPointToSinglePoint(targetSize int) func(comp *wizard.CompiledIOP) {

	logrus.Trace("started multi-point to single-point compiler")
	defer logrus.Trace("finished multi-point to single-point compiler")

	return func(comp *wizard.CompiledIOP) {

		ctx := createMptsCtx(comp, targetSize)
		if len(ctx.hs) == 0 {
			// Nothing to do : fly away
			return
		}

		// If the quotient is too smalls, we still adjust it to have the
		// desired size by expanding it.
		if ctx.quotientSize < targetSize {
			targetSize = ctx.quotientSize
			ctx.targetSize = ctx.quotientSize
			return
		}

		// Random coefficient to combine the quotients
		comp.InsertCoin(ctx.numRound, ctx.LinCombCoeff, coin.Field)

		// The larger quotient is a random linear combination of the
		// quotients of all polynomials. Since it may be larger than our
		// target size we may need to split it in several quotients.
		numQuotients := ctx.quotientSize / targetSize
		ctx.Quotients = make([]ifaces.Column, numQuotients)
		for i := 0; i < numQuotients; i++ {
			ctx.Quotients[i] = comp.InsertCommit(ctx.numRound, ifaces.ColIDf("%v_SHARE_%v", ctx.QuotientName, i), targetSize)
		}

		// Specify how to compute the quotient
		comp.SubProvers.AppendToInner(ctx.numRound, ctx.accumulateQuotients)
		// Evaluation point for the new expression
		comp.InsertCoin(ctx.numRound+1, ctx.EvaluationPoint, coin.Field)
		// Common single evaluation
		comp.InsertUnivariate(ctx.numRound+1, ctx.EvaluationQuery, append(ctx.polys, ctx.Quotients...))
		// Computation of the alleged values
		comp.SubProvers.AppendToInner(ctx.numRound+1, ctx.claimEvaluation)
		// Consistency check
		comp.InsertVerifier(ctx.numRound+1, ctx.verifier, ctx.gnarkVerify)
	}
}

const (
	/*
		Prefix to indicate that an identifier relates to
		global constraint compilation. MPTS stands for
		Multi Point To Single
	*/
	MPTS string = "MPTS"
	// Suffixes for the commitment created throughout the
	// compilation process
	MTSP_QUOTIENT_SUFFIX string = "QUOTIENT"
	// Suffixes for the random coins
	MTSP_LIN_COMB  string = "LIN_COMB"
	MTSP_RAND_EVAL string = "RAND_EVAL"
	// Suffixes for the queries
	MTPS_EVAL_QUERY string = "EVAL_QUERY"
)

/*
Utility struct that can be used to pass information
to different stage of the compilation of multi-point
to single point evaluation.
*/
type mptsCtx struct {
	// List of all the evaluation queries
	hs []ifaces.QueryID
	/*
		List the opening points for each polynomials. The integers
		gives the positions in the vector xs
	*/
	xPoly map[ifaces.ColID][]int
	// List of all the polynomials
	polys []ifaces.Column
	// The largest degree found in the polynomials
	// maxSize      int
	quotientSize int
	targetSize   int
	// maxSize is the size of the largest column being processed by the compiler
	maxSize int
	// Number of rounds in the original protocol
	numRound int
	// Various indentifiers created in the protocol
	Quotients    []ifaces.Column
	QuotientName ifaces.ColID
	// Suffixes for the random coins
	LinCombCoeff    coin.Name
	EvaluationPoint coin.Name
	// Suffixes for the queries
	EvaluationQuery ifaces.QueryID
}

/*
Initialize the context for an instance of the compilatioon
*/
func createMptsCtx(comp *wizard.CompiledIOP, targetSize int) mptsCtx {

	xPoly := make(map[ifaces.ColID][]int)
	hs := []ifaces.QueryID{}
	polys := []ifaces.Column{}
	maxSize := 0

	/*
		Adding coins in the protocol can add extra rounds,
		changing the value of `NumRounds`. Thus, we
		So we fix the old value
	*/
	numRound := comp.NumRounds()

	/*
		Querynum counts the number of queries that are compiled by the present
		compiler. It is used mainly to attribute a correct "hPos" to each query.
	*/
	queryCount := 0

	// Scan the multivariate evaluatation
	for _, qName := range comp.QueriesParams.AllKeys() {

		if comp.QueriesParams.IsIgnored(qName) {
			continue
		}

		q_ := comp.QueriesParams.Data(qName)
		if _, ok := q_.(query.UnivariateEval); !ok {
			/*
				Every other type of parametrizable queries (inner-product, local opening)
				should have been compiled at this point.
			*/
			utils.Panic("query %v has type %v", qName, reflect.TypeOf(q_))
		}

		// Skip if it was already compiled, else insert
		if comp.QueriesParams.MarkAsIgnored(qName) {
			continue
		}

		q := q_.(query.UnivariateEval)
		hs = append(hs, qName)

		/*
			The number of queries to be compiled by the present compilation
			step (so-far)
		*/
		queryCount = len(hs)

		// Scan each univariate query to populate the data structures
		for _, poly := range q.Pols {
			// At this point, we only tolerate non-composite commitments
			ifaces.AssertNotComposite(poly)

			if _, ok := xPoly[poly.GetColID()]; !ok {
				polys = append(polys, poly)
				xPoly[poly.GetColID()] = []int{}
				maxSize = utils.Max(maxSize, poly.Size())
			}
			xPoly[poly.GetColID()] = append(xPoly[poly.GetColID()], queryCount-1)
		}
	}

	// Compute the size of the quotient
	quotientSize := 0
	for _, p := range polys {
		xs := xPoly[p.GetColID()]
		currQuotientSize := p.Size()                            // size of p
		currQuotientSize = utils.Max(currQuotientSize, len(xs)) // size of P(X) - \sum_y yLx,S(X)
		currQuotientSize = currQuotientSize - len(xs)
		quotientSize = utils.Max(quotientSize, currQuotientSize)
	}
	// And pad it to the next power of 2
	quotientSize = utils.NextPowerOfTwo(quotientSize)

	return mptsCtx{
		xPoly:           xPoly,
		hs:              hs,
		polys:           polys,
		quotientSize:    quotientSize,
		targetSize:      targetSize,
		numRound:        numRound,
		maxSize:         maxSize,
		QuotientName:    deriveName[ifaces.ColID](comp, MPTS, "", MTSP_QUOTIENT_SUFFIX),
		LinCombCoeff:    deriveName[coin.Name](comp, MPTS, "", MTSP_LIN_COMB),
		EvaluationPoint: deriveName[coin.Name](comp, MPTS, "", MTSP_RAND_EVAL),
		EvaluationQuery: deriveName[ifaces.QueryID](comp, MPTS, "", MTPS_EVAL_QUERY),
	}
}

// Prover step to calculate the value of the accumulating quotient
func (ctx mptsCtx) accumulateQuotients(run *wizard.ProverRuntime) {

	stoptimer := profiling.LogTimer("run the prover of the mpts : quotient accumulation")

	// Sanity-check : ensures all evaluations queries are for different points.
	// Track all the duplicates to have a nice error message.
	ctx.assertNoDuplicateXAssignment(run)

	r := run.GetRandomCoinField(ctx.LinCombCoeff)

	// Preallocate the value of the quotient
	var (
		quotient = sv.AllocateRegular(ctx.quotientSize)
		quoLock  = sync.Mutex{}
		ys, hs   = ctx.getYsHs(run.GetUnivariateParams, run.GetUnivariateEval)

		// precompute the lagrange polynomials
		lagranges = getLagrangesPolys(hs)
		pool      = mempool.CreateFromSyncPool(ctx.targetSize).Prewarm(runtime.NumCPU())
		mainWg    = &sync.WaitGroup{}
	)

	mainWg.Add(runtime.GOMAXPROCS(0))

	parallel.ExecuteFromChan(len(ctx.polys), func(wg *sync.WaitGroup, index *parallel.AtomicCounter) {

		var (
			pool = mempool.WrapsWithMemCache(pool)

			// Preallocate the value of the quotient
			subQuotientReg  = sv.AllocateRegular(ctx.maxSize)
			subQuotientCnst = field.Zero()
		)

		defer pool.TearDown()

		for {
			i, ok := index.Next()
			if !ok {
				break
			}

			var (
				polHandle  = ctx.polys[i]
				polWitness = polHandle.GetColAssignment(run)
				ri         field.Element
			)

			ri.Exp(r, big.NewInt(int64(i)))

			if cnst, isCnst := polWitness.(*sv.Constant); isCnst {
				polWitness = sv.NewRegular([]field.Element{cnst.Val()})
			} else if pool.Size() == polWitness.Len() {
				polWitness = sv.FFTInverse(polWitness, fft.DIF, true, 0, 0, pool)
			} else {
				polWitness = sv.FFTInverse(polWitness, fft.DIF, true, 0, 0, nil)
			}

			/*
				Substract by the lagrange interpolator and get the quotient
			*/
			for j, hpos := range ctx.xPoly[polHandle.GetColID()] {
				var lagrange sv.SmartVector = sv.NewRegular(lagranges[hpos])
				// Get the associated `y` value
				y := ys[polHandle.GetColID()][j]
				lagrange = sv.ScalarMul(lagrange, y) // this is a copy op
				polWitness = sv.PolySub(polWitness, lagrange)
			}

			/*
				Then gets the quotient by Z_{S_i}
			*/
			var rem field.Element
			for _, hpos := range ctx.xPoly[polHandle.GetColID()] {
				h := hs[hpos]
				polWitness, rem = sv.RuffiniQuoRem(polWitness, h)

				if rem != field.Zero() {
					/*
						Panic mode, try re-evaluating the witness polynomial from
						the original witness and see if there is a problem there

						TODO use structured logging for this
					*/

					panicMsg := "bug in during multi-point\n"
					panicMsg = fmt.Sprintf("%v\ton the polynomial %v from query %v (hpos = %v)\n", panicMsg, polHandle.GetColID(), ctx.hs[hpos], hpos)

					witness := polHandle.GetColAssignment(run)
					yEvaluated := sv.Interpolate(witness, h)
					panicMsg += fmt.Sprintf("\twhose witness evaluates to y = %v x = %v\n", yEvaluated.String(), h.String())

					q := run.GetUnivariateEval(ctx.hs[hpos])
					params := run.GetUnivariateParams(ctx.hs[hpos])

					panicMsg += fmt.Sprintf("\tlooking at the query, we have\n\t\tx=%v\n", params.X.String())
					for i := range q.Pols {
						panicMsg += fmt.Sprintf("\t\tfor %v, P(x) = %v\n", q.Pols[i].GetColID(), params.Ys[i].String())
					}

					utils.Panic("%vremainder was %v (while reducing %v from query %v) \n", panicMsg, rem.String(), polHandle.GetColID(), ctx.hs[hpos])
				}
			}

			// Should be very uncommon
			if subQuotientReg.Len() < polWitness.Len() {
				logrus.Warnf("Warning reallocation of the subquotient for MPTS. If there are too many it's an issue")
				// It's the only known use-case for concatenating smart-vectors
				newSubquotient := make([]field.Element, polWitness.Len())
				subQuotientReg.WriteInSlice(newSubquotient[:subQuotientReg.Len()])
				subQuotientReg = sv.NewRegular(newSubquotient)
			}

			tmp := sv.ScalarMul(polWitness, ri)
			subQuotientReg = sv.PolyAdd(tmp, subQuotientReg)

			if pooled, ok := polWitness.(*sv.Pooled); ok {
				pooled.Free(pool)
			}

			wg.Done()
		}

		subQuotientReg = sv.PolyAdd(subQuotientReg, sv.NewConstant(subQuotientCnst, 1))

		// This locking mechanism is completely subOptimal, but this should be good enough
		quoLock.Lock()
		quotient = sv.PolyAdd(quotient, subQuotientReg)
		quoLock.Unlock()

		mainWg.Done()
	})

	mainWg.Wait()

	if quotient.Len() < ctx.targetSize {
		quo := sv.IntoRegVec(quotient)
		quotient = sv.RightZeroPadded(quo, ctx.targetSize)
	}

	for i := range ctx.Quotients {
		// each subquotient is a slice of the original larger quotient
		subQuotient := quotient.SubVector(i*ctx.targetSize, (i+1)*ctx.targetSize)
		subQuotient = sv.FFT(subQuotient, fft.DIF, true, 0, 0, nil)
		run.AssignColumn(ctx.Quotients[i].GetColID(), subQuotient)
	}

	stoptimer()
}

// Prover step - Evaluates all polys
func (ctx mptsCtx) claimEvaluation(run *wizard.ProverRuntime) {

	stoptimer := profiling.LogTimer("run the prover of the mpts : claim evaluation")

	/*
		Get the evaluation point
	*/
	x := run.GetRandomCoinField(ctx.EvaluationPoint)
	polys := append(ctx.polys, ctx.Quotients...)

	ys := make([]field.Element, len(polys))

	ppool.ExecutePoolChunky(len(polys), func(i int) {
		witness := polys[i].GetColAssignment(run)
		ys[i] = sv.Interpolate(witness, x)
	})

	run.AssignUnivariate(ctx.EvaluationQuery, x, ys...)

	stoptimer()
}

// verifier of the evaluation
func (ctx mptsCtx) verifier(run *wizard.VerifierRuntime) error {

	ys, hs := ctx.getYsHs(
		run.GetUnivariateParams,
		func(qName ifaces.QueryID) query.UnivariateEval {
			return run.Spec.QueriesParams.Data(qName).(query.UnivariateEval)
		},
	)

	// `x` is the random evaluation point
	x := run.GetRandomCoinField(ctx.EvaluationPoint)
	// `r` is the linear combination factor to accumulate the quotient
	r := run.GetRandomCoinField(ctx.LinCombCoeff)

	// Compute the lagrange polynomials
	lagrange := poly.EvaluateLagrangesAnyDomain(hs, x)
	univQ := run.GetUnivariateParams(ctx.EvaluationQuery)

	if x != univQ.X {
		return fmt.Errorf("bad evaluation point for the queries")
	}

	if len(univQ.Ys) != len(ctx.polys)+(ctx.quotientSize/ctx.targetSize) {
		utils.Panic(
			"expected %v evaluations (%v for the polys, and %v for the quotient), but got %v",
			len(ctx.polys)+(ctx.quotientSize/ctx.targetSize),
			len(ctx.polys), (ctx.quotientSize / ctx.targetSize),
			len(univQ.Ys),
		)
	}

	// The alleged opening values for all polynomial (except the quotient) in x
	evalsPolys := univQ.Ys[:len(ctx.polys)]
	// Evaluation of Q(x), where q is the aggregating quotient. Since the quotient
	// is split in several slices. We need to restick the slices together
	qxs := univQ.Ys[len(ctx.polys):]
	var xN field.Element
	xN.Exp(x, big.NewInt(int64(ctx.targetSize)))
	qx := poly.EvalUnivariate(qxs, xN)

	// The left hand corresponds to Q(X) * Z(X)
	// Where Z(X) is the vanishing polynomial in all `xs`
	left := qx
	for i := range hs {

		// allocate in a dedicated variable to avoid memory aliasing in for loop
		// (gosec G601)
		h := hs[i]

		var tmp field.Element
		tmp.Sub(&x, &h)
		left.Mul(&left, &tmp)
	}

	/*
		The right hand \sum_i r^i P(x) - \sum X-
	*/
	right := field.Zero()
	ri := field.One() // r^i

	for i := range evalsPolys {

		tmp := field.Zero()

		// tmp <- \sum_{x \in S_i} P_i(h) * L_h(x)
		// tmp is (called ri(x)) in the paper
		si := make(map[int]struct{})
		pName := ctx.polys[i]

		for j, hpos := range ctx.xPoly[pName.GetColID()] {
			si[hpos] = struct{}{}
			// Value of P(h)
			ph := ys[pName.GetColID()][j]
			// Value of L_h(x)
			var tmph field.Element
			tmph.Mul(&ph, &lagrange[hpos])
			tmp.Add(&tmp, &tmph)
		}

		// tmp <- P(x) - ri(X) (with the paper's convention)
		px := evalsPolys[i]
		tmp.Sub(&px, &tmp)

		// tmp <- tmp * \prod_{h \in S\S_i} (X - h)}
		for j := range hs {
			// allocate outside of the loop to avoid memory aliasing in for loop
			// (gosec G601)
			h := hs[j]
			if _, ok := si[j]; ok {
				// The current polynomial was evaluated in h, reject
				continue
			}

			var zhj field.Element
			zhj.Sub(&x, &h)
			tmp.Mul(&tmp, &zhj)
		}

		// tmp <- tmp * r^i
		tmp.Mul(&tmp, &ri)
		right.Add(&right, &tmp)

		// And update the power of i
		ri.Mul(&ri, &r)
	}

	if left != right {
		return fmt.Errorf("mismatch between left and right %v != %v", left.String(), right.String())
	}

	return nil
}

/*
Gnark function generating constraints to mirror the verification
of the evaluation step.
*/
func (ctx mptsCtx) gnarkVerify(api frontend.API, c *wizard.WizardVerifierCircuit) {

	logrus.Infof("Start verifying MPTS reduction")

	ys, hs := ctx.getYsHsGnark(c)

	// `x` is the random evaluation point
	x := c.GetRandomCoinField(ctx.EvaluationPoint)
	// `r` is the linear combination factor to accumulate the quotient
	r := c.GetRandomCoinField(ctx.LinCombCoeff)

	// Compute the lagrange polynomials
	lagrange := poly.EvaluateLagrangeAnyDomainGnark(api, hs, x)
	univQ := c.GetUnivariateParams(ctx.EvaluationQuery)

	// bad evaluation point for the queries
	api.AssertIsEqual(x, univQ.X)

	// The alleged opening values for all polynomial (except the quotient) in x
	evalsPolys := univQ.Ys[:len(ctx.polys)]
	// Evaluation of Q(x), where q is the aggregating quotient
	// Evaluation of Q(x), where q is the aggregating quotient. Since the quotient
	// is split in several slices. We need to restick the slices together
	qxs := univQ.Ys[len(ctx.polys):]
	xN := gnarkutil.Exp(api, x, ctx.targetSize)
	qx := poly.EvaluateUnivariateGnark(api, qxs, xN)

	// The left hand corresponds to Q(X) * Z(X)
	// Where Z(X) is the vanishing polynomial in all `xs`
	left := qx
	for _, h := range hs {
		tmp := api.Sub(x, h)
		left = api.Mul(left, tmp)
	}

	/*
		The right hand \sum_i r^i P(x) - \sum X-
	*/
	right := frontend.Variable(field.Zero())
	ri := frontend.Variable(field.One()) // r^i

	for i := range evalsPolys {

		tmp := frontend.Variable(field.Zero())

		// tmp <- \sum_{x \in S_i} P_i(h) * L_h(x)
		// tmp is (called ri(x)) in the paper
		si := make(map[int]struct{})
		pName := ctx.polys[i]

		for j, hpos := range ctx.xPoly[pName.GetColID()] {
			si[hpos] = struct{}{}
			// Value of P(h)
			ph := ys[pName.GetColID()][j]
			// Value of L_h(x)
			tmph := api.Mul(ph, lagrange[hpos])
			tmp = api.Add(tmp, tmph)
		}

		// tmp <- P(x) - ri(X) (with the paper's convention)
		tmp = api.Sub(evalsPolys[i], tmp)

		// tmp <- tmp * \prod_{h \in S\S_i} (X - h)}
		for j, h := range hs {
			if _, ok := si[j]; ok {
				// The current polynomial was evaluated in h, reject
				continue
			}

			zhj := api.Sub(x, h)
			tmp = api.Mul(tmp, zhj)
		}

		// tmp <- tmp * r^i
		tmp = api.Mul(tmp, ri)
		right = api.Add(right, tmp)

		// And update the power of i
		ri = api.Mul(ri, r)
	}

	api.AssertIsEqual(left, right)

	logrus.Debugf("Done verifying MPTS reduction")

}

// collect all the alleged opening values in a map, so that we can utilize them later.
func (ctx mptsCtx) getYsHs(
	// func that can be used to return the parameters of a given query
	getParam func(ifaces.QueryID) query.UnivariateEvalParams,
	// func that can be used to return the query's metadata given its name
	getQuery func(ifaces.QueryID) query.UnivariateEval,

) (
	ys map[ifaces.ColID][]field.Element,
	hs []field.Element,
) {
	/*
		Collect the `y` values in a map for easy access
		And collect the values for hs
	*/
	ys = make(map[ifaces.ColID][]field.Element)
	hs = []field.Element{}

	/*
		If there is several time the same point on the same polynomial,
		this can lead to a bug that is hard to catch. Having `k` queries on
		the same point on the same poly would end up getting to the code
		to assume that P is divisible by (X - k)^k.

		The point of this map is to catch this case
	*/
	checkNoDuplicateMap := make(map[struct {
		ifaces.ColID
		field.Element
	}]ifaces.QueryID)

	for _, qName := range ctx.hs {
		q := getQuery(qName)
		param := getParam(qName)
		hs = append(hs, param.X)
		for i, polHandle := range q.Pols {
			// initialize the nil slice if necessary
			if _, ok := ys[polHandle.GetColID()]; !ok {
				ys[polHandle.GetColID()] = []field.Element{}
			}

			/*
				check for no duplicate. This will also panic if somehow there
				are several time the same poly in the same query.
			*/
			if other, ok := checkNoDuplicateMap[struct {
				ifaces.ColID
				field.Element
			}{polHandle.GetColID(), param.X}]; ok {
				utils.Panic("Two queries for poly %v on point %v, (%v %v)", polHandle.GetColID(), param.X.String(), qName, other)
			}

			y := param.Ys[i]
			ys[polHandle.GetColID()] = append(ys[polHandle.GetColID()], y)
		}
	}
	return ys, hs
}

// Evaluates the lagrange polynomials for a given domain in r
// The result is a map h \in domain, h -> L_h(x)
func getLagrangesPolys(domain []field.Element) (lagranges [][]field.Element) {

	for i := range domain {
		// allocate outside of the loop to avoid memory aliasing in for loop
		// (gosec G601)
		hi := domain[i]

		lhi := []field.Element{field.One()}
		den := field.One()

		for j := range domain {
			// allocate outside of the loop to avoid memory aliasing in for loop
			// (gosec G601)
			hj := domain[j]
			if hj == hi {
				// Skip it
				continue
			}
			// more convenient to store -h instead of h
			hj.Neg(&hj)
			lhi = poly.Mul(lhi, []field.Element{hj, field.One()})
			hj.Add(&hi, &hj) // so hi - hj
			den.Mul(&den, &hj)
		}

		den.Inverse(&den)
		lhi = poly.ScalarMul(lhi, den)
		lagranges = append(lagranges, lhi)
	}

	return lagranges
}

// Mirrrors `getYsHs` to build a gnark circuit
func (ctx mptsCtx) getYsHsGnark(
	c *wizard.WizardVerifierCircuit,
) (
	ys map[ifaces.ColID][]frontend.Variable,
	hs []frontend.Variable,
) {
	/*
		Collect the `y` values in a map for easy access
		And collect the values for hs
	*/
	ys = make(map[ifaces.ColID][]frontend.Variable)
	hs = []frontend.Variable{}

	for _, qName := range ctx.hs {

		q := c.GetUnivariateEval(qName)
		params := c.GetUnivariateParams(qName)
		hs = append(hs, params.X)
		for i, polHandle := range q.Pols {
			// initialize the nil slice if necessary
			if _, ok := ys[polHandle.GetColID()]; !ok {
				ys[polHandle.GetColID()] = []frontend.Variable{}
			}

			y := params.Ys[i]
			ys[polHandle.GetColID()] = append(ys[polHandle.GetColID()], y)
		}
	}
	return ys, hs
}

// Sanity-assertion, will panic if several queries are assigned the same X values.
// This is a problem because it involves dividing by zero in later stages of the
// protocol. If it panics, it will print a nice error message summarizing all the duplicate
// founds.
func (ctx mptsCtx) assertNoDuplicateXAssignment(run *wizard.ProverRuntime) {

	duplicateMap := map[field.Element][]ifaces.QueryID{}
	foundDuplicate := false
	for _, h := range ctx.hs {
		eval := run.GetUnivariateParams(h)
		// If no entries exists : create one
		if _, ok := duplicateMap[eval.X]; !ok {
			duplicateMap[eval.X] = []ifaces.QueryID{h}
			continue
		}
		// Else append to the list. Implictly, this is an error-case
		duplicateMap[eval.X] = append(duplicateMap[eval.X], h)
		foundDuplicate = true
	}

	// Print informative message in case of failures
	if foundDuplicate {
		for x, lists := range duplicateMap {
			if len(lists) > 1 {
				logrus.Errorf("found an X (%v) that is concurrently assigned to multiple queries %v", x, lists)
			}
		}
		utils.Panic("Found duplicate assignments (see loggs above)")
	}
}
