package globalcs

import (
	"fmt"
	"math/big"
	"reflect"
	"sync"

	"github.com/consensys/gnark-crypto/field/koalabear/fft"
	"github.com/consensys/gnark/frontend"
	sv "github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors_mixed"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/variables"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/gnarkutil"
	"github.com/consensys/linea-monorepo/prover/utils/parallel"
	"github.com/consensys/linea-monorepo/prover/utils/profiling"
	"github.com/sirupsen/logrus"
)

// EvaluationCtx collects the compilation artefacts related to the evaluation
// part of the Plonk quotient technique.
type EvaluationCtx struct {
	QuotientCtx
	QuotientEvals []query.UnivariateEval
	WitnessEval   query.UnivariateEval
	EvalCoin      coin.Info
}

// EvaluationProver wraps [evaluationCtx] to implement the [wizard.ProverAction]
// interface.
type EvaluationProver EvaluationCtx

// EvaluationVerifier wraps [evaluationCtx] to implement the [wizard.VerifierAction]
// interface.
type EvaluationVerifier struct {
	EvaluationCtx
	Skipped bool
}

// declareUnivariateQueries declares the univariate queries over all the quotient
// shares, making sure that the shares needing to be evaluated over the same
// point are in the same query. This is a req from the upcoming naturalization
// compiler.
func declareUnivariateQueries(
	comp *wizard.CompiledIOP,
	qCtx QuotientCtx,
) EvaluationCtx {

	var (
		round       = qCtx.QuotientShares[0][0].Round()
		ratios      = qCtx.Ratios
		maxRatio    = utils.Max(ratios...)
		queriesPols = make([][]ifaces.Column, maxRatio)
		res         = EvaluationCtx{
			QuotientCtx: qCtx,
			EvalCoin: comp.InsertCoin(
				round+1,
				coin.Name(deriveName(comp, EVALUATION_RANDOMESS)),
				coin.FieldExt,
			),
			WitnessEval: comp.InsertUnivariate(
				round+1,
				ifaces.QueryID(deriveName(comp, UNIVARIATE_EVAL_ALL_HANDLES)),
				qCtx.AllInvolvedColumns,
			),
			QuotientEvals: make([]query.UnivariateEval, maxRatio),
		}
	)

	for i, ratio := range ratios {
		var (
			jumpBy = maxRatio / ratio
		)
		for j := range qCtx.QuotientShares[i] {
			queriesPols[j*jumpBy] = append(queriesPols[j*jumpBy], qCtx.QuotientShares[i][j])
		}
	}

	for i := range queriesPols {
		res.QuotientEvals[i] = comp.InsertUnivariate(
			round+1,
			ifaces.QueryID(deriveName(comp, UNIVARIATE_EVAL_QUOTIENT_SHARES, i, maxRatio)),
			queriesPols[i],
		)
	}

	return res
}

// Run computes the evaluation of the univariate queries and implements the
// [wizard.ProverAction] interface.
func (pa EvaluationProver) Run(run *wizard.ProverRuntime) {

	var (
		stoptimer = profiling.LogTimer("evaluate the queries for the global constraints")
		r         = run.GetRandomCoinFieldExt(pa.EvalCoin.Name)
		witnesses = make([]sv.SmartVector, len(pa.AllInvolvedColumns))
	)

	// Compute the evaluations
	parallel.Execute(len(pa.AllInvolvedColumns), func(start, stop int) {
		for i := start; i < stop; i++ {
			handle := pa.AllInvolvedColumns[i]
			witness := handle.GetColAssignment(run)
			witnesses[i] = witness

			if witness.Len() == 0 {
				logrus.Errorf("found a witness of size zero: %v", handle.GetColID())
			}
		}
	})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		ys := smartvectors_mixed.BatchEvaluateLagrange(witnesses, r)
		run.AssignUnivariateExt(pa.WitnessEval.QueryID, r, ys...)
		wg.Done()
	}()

	/*
		For the quotient evaluate it on `x = r / g`, where g is the coset
		shift. The generation of the domain is memoized.
	*/

	var (
		maxRatio          = utils.Max(pa.Ratios...)
		mulGenInv         = fft.NewDomain(uint64(maxRatio*pa.DomainSize), fft.WithCache()).FrMultiplicativeGenInv
		rootInv, _        = fft.Generator(uint64(maxRatio * pa.DomainSize))
		quotientEvalPoint fext.Element
	)
	rootInv.Inverse(&rootInv)
	quotientEvalPoint.MulByElement(&r, &mulGenInv)

	for i := range pa.QuotientEvals {
		wg.Add(1)
		go func(i int, evalPoint fext.Element) {
			var (
				q  = pa.QuotientEvals[i]
				cs = make([]sv.SmartVector, len(q.Pols))
			)

			parallel.Execute(len(q.Pols), func(start, stop int) {
				for i := start; i < stop; i++ {
					cs[i] = q.Pols[i].GetColAssignment(run)
				}
			})
			ys := smartvectors_mixed.BatchEvaluateLagrange(cs, evalPoint)

			run.AssignUnivariateExt(q.Name(), evalPoint, ys...)
			wg.Done()
		}(i, quotientEvalPoint)
		quotientEvalPoint.MulByElement(&quotientEvalPoint, &rootInv)
	}

	wg.Wait()

	/*
		as we shifted the evaluation point. No need to do do coset evaluation
		here
	*/
	stoptimer()
}

// Run evaluate the constraint and checks that
func (ctx *EvaluationVerifier) Run(run wizard.Runtime) error {

	var (
		// Will be assigned to "X", the random point at which we check the constraint.
		r = run.GetRandomCoinFieldExt(ctx.EvalCoin.Name)
		// Map all the evaluations and checks the evaluations points
		mapYs = make(map[ifaces.ColID]fext.GenericFieldElem)
		// Get the parameters
		params           = run.GetUnivariateParams(ctx.WitnessEval.QueryID)
		univQuery        = run.GetUnivariateEval(ctx.WitnessEval.QueryID)
		quotientYs, errQ = ctx.recombineQuotientSharesEvaluation(run, r)
	)

	if errQ != nil {
		return fmt.Errorf("invalid evaluation point for the quotients: %v", errQ.Error())
	}

	// Check the evaluation point is consistent with r
	if params.ExtX != r {
		return fmt.Errorf("(verifier of global queries) : Evaluation point of %v is incorrect (%v, expected %v)",
			ctx.WitnessEval.QueryID, params.ExtX.String(), r.String())
	}

	// Collect the evaluation points
	for j, handle := range univQuery.Pols {
		var genericElem fext.GenericFieldElem

		genericElem = fext.NewGenFieldFromExt(params.ExtYs[j])

		mapYs[handle.GetColID()] = genericElem
	}

	// Annulator = r^n - 1, common for all ratios
	one := fext.One()
	annulator := r
	annulator.Exp(annulator, big.NewInt(int64(ctx.DomainSize)))
	annulator.Sub(&annulator, &one)

	for i, ratio := range ctx.Ratios {

		board := ctx.AggregateExpressionsBoard[i]
		metadatas := board.ListVariableMetadata()

		evalInputs := make([]sv.SmartVector, len(metadatas))

		for k, metadataInterface := range metadatas {
			switch metadata := metadataInterface.(type) {
			case ifaces.Column:
				entry := mapYs[metadata.GetColID()]
				if entry.IsBase {
					elem, _ := entry.GetBase()
					evalInputs[k] = sv.NewConstant(elem, 1)
				} else {
					elem := entry.GetExt()
					evalInputs[k] = sv.NewConstantExt(elem, 1)
				}
			case coin.Info:
				evalInputs[k] = sv.NewConstantExt(run.GetRandomCoinFieldExt(metadata.Name), 1)
			case variables.X:
				evalInputs[k] = sv.NewConstantExt(r, 1)
			case variables.PeriodicSample:
				evalInputs[k] = sv.NewConstantExt(metadata.EvalAtOutOfDomainExt(ctx.DomainSize, r), 1)
			case ifaces.Accessor:
				evalInputs[k] = sv.NewConstantExt(metadata.GetValExt(run), 1)
			default:
				utils.Panic("Not a variable type %v in global query (ratio %v)", reflect.TypeOf(metadataInterface), ratio)
			}
		}

		left := board.Evaluate(evalInputs).GetExt(0)

		// right : r^{n}-1 Q(r)
		qr := quotientYs[i]
		var right fext.Element
		right.Mul(&annulator, &qr)

		if left != right {
			return fmt.Errorf("global constraint - ratio %v - mismatch at random point - %v != %v", ratio, left.String(), right.String())
		}
	}

	return nil
}

// Verifier step, evaluate the constraint and checks that
func (ctx *EvaluationVerifier) RunGnark(api frontend.API, c wizard.GnarkRuntime) {

	var (
		koalaAPI = koalagnark.NewAPI(api)

		// Will be assigned to "X", the random point at which we check the constraint.
		r          = c.GetRandomCoinFieldExt(ctx.EvalCoin.Name)
		annulator  = gnarkutil.ExpExt(api, r, ctx.DomainSize)
		quotientYs = ctx.recombineQuotientSharesEvaluationGnark(api, c, r)
		params     = c.GetUnivariateParams(ctx.WitnessEval.QueryID)
		univQuery  = c.GetUnivariateEval(ctx.WitnessEval.QueryID)
		wOneExt    = koalaAPI.OneExt()
	)

	annulator = koalaAPI.SubExt(annulator, wOneExt)
	koalaAPI.AssertIsEqualExt(r, params.ExtX)

	// Map all the evaluations and checks the evaluations points
	mapYs := make(map[ifaces.ColID]koalagnark.Ext)

	// Collect the evaluation points
	for j, handle := range univQuery.Pols {
		mapYs[handle.GetColID()] = params.ExtYs[j]
	}

	for i, ratio := range ctx.Ratios {

		board := ctx.AggregateExpressionsBoard[i]
		metadatas := board.ListVariableMetadata()

		evalInputs := make([]any, len(metadatas))

		for k, metadataInterface := range metadatas {
			switch metadata := metadataInterface.(type) {
			case ifaces.Column:
				v := mapYs[metadata.GetColID()]
				if w, ok := koalaAPI.BaseValueOfElement(v); ok {
					evalInputs[k] = *w
				} else {
					evalInputs[k] = v
				}
			case coin.Info:
				if metadata.IsBase() {
					utils.Panic("unsupported, coins are always over field extensions")
				} else {
					tmp := c.GetRandomCoinFieldExt(metadata.Name)
					evalInputs[k] = tmp
				}
			case variables.X:
				evalInputs[k] = r
			case variables.PeriodicSample:
				evalInputs[k] = metadata.GnarkEvalAtOutOfDomain(api, ctx.DomainSize, r)
			case ifaces.Accessor:
				evalInputs[k] = metadata.GetFrontendVariableExt(api, c)
			default:
				utils.Panic("Not a variable type %v in global query (ratio %v)", reflect.TypeOf(metadataInterface), ratio)
			}
		}

		left := board.GnarkEvalExt(api, evalInputs)

		// right : r^{n}-1 Q(r)
		qr := quotientYs[i]
		right := koalaAPI.MulExt(annulator, qr)

		koalaAPI.AssertIsEqualExt(left, right)
		logrus.Debugf("verifying global constraint : DONE")

	}
}

// recombineQuotientSharesEvaluation returns the evaluations of the quotients
// on point r
func (ctx EvaluationVerifier) recombineQuotientSharesEvaluation(run wizard.Runtime, r fext.Element) ([]fext.Element, error) {

	var (
		// res stores the list of the recombined quotient evaluations for each
		// combination.
		recombinedYs = make([]fext.Element, len(ctx.Ratios))
		// ys stores the values of the quotient shares ordered by ratio
		qYs      = make([][]fext.Element, utils.Max(ctx.Ratios...))
		maxRatio = utils.Max(ctx.Ratios...)
		// shiftedR = r / g where g is the generator of the multiplicative group
		shiftedR fext.Element
		// mulGen is the generator of the multiplicative group
		mulGenInv = fft.NewDomain(uint64(maxRatio*ctx.DomainSize), fft.WithCache()).FrMultiplicativeGenInv
		// omegaN is a root of unity generating the domain of size `domainSize
		// * maxRatio`
		omegaN, _ = fft.Generator(uint64(ctx.DomainSize * maxRatio))
	)

	shiftedR.MulByElement(&r, &mulGenInv)

	for i, q := range ctx.QuotientEvals {
		params := run.GetUnivariateParams(q.Name())
		qYs[i] = params.ExtYs

		// Check that the provided value for x is the right one
		providedX := params.ExtX
		var expectedXinit field.Element
		var expectedX fext.Element

		expectedXinit.Inverse(&omegaN)
		expectedXinit.Exp(expectedXinit, big.NewInt(int64(i)))
		expectedX.MulByElement(&shiftedR, &expectedXinit)

		if !providedX.Equal(&expectedX) {
			return nil, fmt.Errorf("bad X value")
		}
	}

	for i, ratio := range ctx.Ratios {
		var (
			jumpBy = maxRatio / ratio
			ys     = make([]fext.Element, ratio)
		)

		for j := range ctx.QuotientShares[i] {
			ys[j] = qYs[j*jumpBy][0]
			qYs[j*jumpBy] = qYs[j*jumpBy][1:]
		}

		var (
			n             = ctx.DomainSize * ratio
			omegaRatio, _ = fft.Generator(uint64(ratio))
			rPowM         fext.Element
			// outerFactor stores m/n*(r^n - 1)
			outerFactor   = shiftedR
			one           = fext.One()
			omegaRatioInv field.Element
			res           fext.Element
			ratioInvField = field.NewElement(uint64(ratio))
		)

		rPowM.Exp(shiftedR, big.NewInt(int64(ctx.DomainSize)))
		ratioInvField.Inverse(&ratioInvField)
		omegaRatioInv.Inverse(&omegaRatio)

		for k := range ys {
			// tmp stores ys[k] / ((r^m / omegaRatio^k) - 1)
			var tmpinit field.Element
			var tmp fext.Element

			tmpinit.Exp(omegaRatioInv, big.NewInt(int64(k)))
			tmp.MulByElement(&rPowM, &tmpinit)
			tmp.Sub(&tmp, &one)
			tmp.Div(&ys[k], &tmp)
			res.Add(&res, &tmp)
		}

		outerFactor.Exp(shiftedR, big.NewInt(int64(n)))
		outerFactor.Sub(&outerFactor, &one)
		outerFactor.MulByElement(&outerFactor, &ratioInvField)
		res.Mul(&res, &outerFactor)
		recombinedYs[i] = res
	}

	return recombinedYs, nil
}

// recombineQuotientSharesEvaluation returns the evaluations of the quotients
// on point r
func (ctx EvaluationVerifier) recombineQuotientSharesEvaluationGnark(api frontend.API, run wizard.GnarkRuntime, r koalagnark.Ext) []koalagnark.Ext {

	// res stores the list of the recombined quotient evaluations for each
	// combination.
	recombinedYs := make([]koalagnark.Ext, len(ctx.Ratios))
	// ys stores the values of the quotient shares ordered by ratio

	qYs := make([][]koalagnark.Ext, utils.Max(ctx.Ratios...))

	maxRatio := utils.Max(ctx.Ratios...)

	// shiftedR = r / g where g is the generator of the multiplicative group
	var shiftedR koalagnark.Ext

	// TODO @thomas kill fft domain generation
	mulGenInv := fft.NewDomain(uint64(maxRatio*ctx.DomainSize), fft.WithCache()).FrMultiplicativeGenInv

	// omegaN is a root of unity generating the domain of size `domainSize
	// * maxRatio`
	omegaN, _ := fft.Generator(uint64(ctx.DomainSize * maxRatio))

	koalaAPI := koalagnark.NewAPI(api)

	// shiftedR.MulByFp(api, r, mulGenInv)
	bMulGenInv := big.NewInt(0).SetUint64(mulGenInv.Uint64())
	shiftedR = koalaAPI.MulConstExt(r, bMulGenInv)

	var invOmegaN field.Element
	invOmegaN.Inverse(&omegaN)

	// Optimization: compute powers of invOmegaN incrementally instead of using ExpExt
	// expectedX[i] = shiftedR * invOmegaN^i
	// We compute invOmegaN^i by repeated multiplication
	var accInvOmegaN field.Element
	accInvOmegaN.SetOne() // invOmegaN^0 = 1

	for i, q := range ctx.QuotientEvals {
		params := run.GetUnivariateParams(q.Name())
		qYs[i] = params.ExtYs

		// Check that the provided value for x is the right one
		providedX := params.ExtX
		var expectedX koalagnark.Ext
		// expectedX = shiftedR * invOmegaN^i (computed incrementally)
		wrappedAccInvOmegaN := big.NewInt(0).SetUint64(accInvOmegaN.Uint64())
		expectedX = koalaAPI.MulConstExt(shiftedR, wrappedAccInvOmegaN)
		koalaAPI.AssertIsEqualExt(providedX, expectedX)

		// Update accumulator: accInvOmegaN *= invOmegaN
		accInvOmegaN.Mul(&accInvOmegaN, &invOmegaN)
	}

	// Precompute shiftedR^DomainSize once, reuse for all ratios
	shiftedRPowDomainSize := gnarkutil.ExpExt(api, shiftedR, ctx.DomainSize)

	for i, ratio := range ctx.Ratios {
		var (
			jumpBy = maxRatio / ratio
			ys     = make([]koalagnark.Ext, ratio)
		)

		for j := range ctx.QuotientShares[i] {
			ys[j] = qYs[j*jumpBy][0]
			qYs[j*jumpBy] = qYs[j*jumpBy][1:]
		}

		omegaRatio, _ := fft.Generator(uint64(ratio))
		ratioInvField := field.NewElement(uint64(ratio))
		var omegaRatioInv field.Element
		res := koalaAPI.ZeroExt()

		// Optimization: rPowM = shiftedR^m = shiftedR^DomainSize (precomputed)
		rPowM := shiftedRPowDomainSize
		ratioInvField.Inverse(&ratioInvField)
		omegaRatioInv.Inverse(&omegaRatio)

		wOne := koalaAPI.OneExt()

		// Optimization: compute omegaRatioInv^k incrementally
		var accOmegaRatioInv field.Element
		accOmegaRatioInv.SetOne() // omegaRatioInv^0 = 1

		for k := range ys {
			// tmp stores ys[k] / ((r^m * omegaRatioInv^k) - 1)
			wrappedAccOmegaRatioInv := big.NewInt(0).SetUint64(accOmegaRatioInv.Uint64())
			tmp := koalaAPI.MulConstExt(rPowM, wrappedAccOmegaRatioInv)
			tmp = koalaAPI.SubExt(tmp, wOne)
			tmp = koalaAPI.DivExt(ys[k], tmp)
			res = koalaAPI.AddExt(res, tmp)

			// Update accumulator: accOmegaRatioInv *= omegaRatioInv
			accOmegaRatioInv.Mul(&accOmegaRatioInv, &omegaRatioInv)
		}

		// Optimization: outerFactor = shiftedR^n = shiftedR^(m*ratio) = (shiftedR^m)^ratio
		// Reuse rPowM instead of computing ExpExt again
		wrappedRatioInvField := big.NewInt(0).SetUint64(ratioInvField.Uint64())
		outerFactor := gnarkutil.ExpExt(api, rPowM, ratio)
		outerFactor = koalaAPI.SubExt(outerFactor, wOne)
		outerFactor = koalaAPI.MulConstExt(outerFactor, wrappedRatioInvField)
		res = koalaAPI.MulExt(res, outerFactor)
		recombinedYs[i] = res
	}

	return recombinedYs
}

func (ctx *EvaluationVerifier) Skip() {
	ctx.Skipped = true
}

func (ctx *EvaluationVerifier) IsSkipped() bool {
	return ctx.Skipped
}
