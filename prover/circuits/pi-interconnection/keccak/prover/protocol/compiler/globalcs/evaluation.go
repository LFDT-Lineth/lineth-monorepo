package globalcs

import (
	"fmt"
	"math/big"
	"reflect"
	"sync"

	"github.com/consensys/gnark/frontend"
	sv "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/fft"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/variables"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/gnarkutil"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/parallel"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/profiling"
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
				coin.Field,
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

	// TODO @gbotrel this use a lot of memory, investigate why.

	var (
		stoptimer = profiling.LogTimer("Evaluate the queries for the global constraints")
		r         = run.GetRandomCoinField(pa.EvalCoin.Name)
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

	ys := sv.BatchInterpolate(witnesses, r)
	run.AssignUnivariate(pa.WitnessEval.QueryID, r, ys...)

	/*
		For the quotient evaluate it on `x = r / g`, where g is the coset
		shift. The generation of the domain is memoized.
	*/

	var (
		maxRatio          = utils.Max(pa.Ratios...)
		mulGenInv         = fft.NewDomain(maxRatio * pa.DomainSize).FrMultiplicativeGenInv
		rootInv           = fft.GetOmega(maxRatio * pa.DomainSize)
		quotientEvalPoint field.Element
		wg                = &sync.WaitGroup{}
	)

	rootInv.Inverse(&rootInv)
	quotientEvalPoint.Mul(&mulGenInv, &r)

	for i := range pa.QuotientEvals {
		wg.Add(1)
		go func(i int, evalPoint field.Element) {
			var (
				q  = pa.QuotientEvals[i]
				ys = make([]field.Element, len(q.Pols))
			)

			parallel.Execute(len(q.Pols), func(start, stop int) {
				for i := start; i < stop; i++ {
					c := q.Pols[i].GetColAssignment(run)
					ys[i] = sv.Interpolate(c, evalPoint)
				}
			})

			run.AssignUnivariate(q.Name(), evalPoint, ys...)
			wg.Done()
		}(i, quotientEvalPoint)
		quotientEvalPoint.Mul(&quotientEvalPoint, &rootInv)
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
		r = run.GetRandomCoinField(ctx.EvalCoin.Name)
		// Map all the evaluations and checks the evaluations points
		mapYs = make(map[ifaces.ColID]field.Element)
		// Get the parameters
		params           = run.GetUnivariateParams(ctx.WitnessEval.QueryID)
		univQuery        = run.GetUnivariateEval(ctx.WitnessEval.QueryID)
		quotientYs, errQ = ctx.recombineQuotientSharesEvaluation(run, r)
	)

	if errQ != nil {
		return fmt.Errorf("invalid evaluation point for the quotients: %v", errQ.Error())
	}

	// Check the evaluation point is consistent with r
	if params.X != r {
		return fmt.Errorf("(verifier of global queries) : Evaluation point of %v is incorrect (%v, expected %v)",
			ctx.WitnessEval.QueryID, params.X.String(), r.String())
	}

	// Collect the evaluation points
	for j, handle := range univQuery.Pols {
		mapYs[handle.GetColID()] = params.Ys[j]
	}

	// Annulator = X^n - 1, common for all ratios
	one := field.One()
	annulator := r
	annulator.Exp(annulator, big.NewInt(int64(ctx.DomainSize)))
	annulator.Sub(&annulator, &one)

	for i, ratio := range ctx.Ratios {

		board := ctx.AggregateExpressions[i].Board()
		metadatas := board.ListVariableMetadata()

		evalInputs := make([]sv.SmartVector, len(metadatas))

		for k, metadataInterface := range metadatas {
			switch metadata := metadataInterface.(type) {
			case ifaces.Column:
				evalInputs[k] = sv.NewConstant(mapYs[metadata.GetColID()], 1)
			case coin.Info:
				evalInputs[k] = sv.NewConstant(run.GetRandomCoinField(metadata.Name), 1)
			case variables.X:
				evalInputs[k] = sv.NewConstant(r, 1)
			case variables.PeriodicSample:
				evalInputs[k] = sv.NewConstant(metadata.EvalAtOutOfDomain(ctx.DomainSize, r), 1)
			case ifaces.Accessor:
				evalInputs[k] = sv.NewConstant(metadata.GetVal(run), 1)
			default:
				utils.Panic("Not a variable type %v in global query (ratio %v)", reflect.TypeOf(metadataInterface), ratio)
			}
		}

		left := board.Evaluate(evalInputs).Get(0)

		// right : r^{n}-1 Q(r)
		qr := quotientYs[i]
		var right field.Element
		right.Mul(&annulator, &qr)

		if left != right {
			return fmt.Errorf("global constraint - ratio %v - mismatch at random point - %v != %v", ratio, left.String(), right.String())
		}
	}

	return nil
}

// Verifier step, evaluate the constraint and checks that
func (ctx *EvaluationVerifier) RunGnark(api frontend.API, c wizard.GnarkRuntime) {

	// Will be assigned to "X", the random point at which we check the constraint.
	r := c.GetRandomCoinField(ctx.EvalCoin.Name)
	annulator := gnarkutil.Exp(api, r, ctx.DomainSize)
	quotientYs := ctx.recombineQuotientSharesEvaluationGnark(api, c, r)
	params := c.GetUnivariateParams(ctx.WitnessEval.QueryID)
	univQuery := c.GetUnivariateEval(ctx.WitnessEval.QueryID)

	annulator = api.Sub(annulator, frontend.Variable(1))

	// Get the parameters
	api.AssertIsEqual(r, params.X) // check the evaluation is consistent with the other stuffs

	// Map all the evaluations and checks the evaluations points
	mapYs := make(map[ifaces.ColID]frontend.Variable)

	// Collect the evaluation points
	for j, handle := range univQuery.Pols {
		mapYs[handle.GetColID()] = params.Ys[j]
	}

	for i, ratio := range ctx.Ratios {

		board := ctx.AggregateExpressions[i].Board()
		metadatas := board.ListVariableMetadata()

		evalInputs := make([]frontend.Variable, len(metadatas))

		for k, metadataInterface := range metadatas {
			switch metadata := metadataInterface.(type) {
			case ifaces.Column:
				evalInputs[k] = mapYs[metadata.GetColID()]
			case coin.Info:
				evalInputs[k] = c.GetRandomCoinField(metadata.Name)
			case variables.X:
				evalInputs[k] = r
			case variables.PeriodicSample:
				evalInputs[k] = metadata.GnarkEvalAtOutOfDomain(api, ctx.DomainSize, r)
			case ifaces.Accessor:
				evalInputs[k] = metadata.GetFrontendVariable(api, c)
			default:
				utils.Panic("Not a variable type %v in global query (ratio %v)", reflect.TypeOf(metadataInterface), ratio)
			}
		}

		left := board.GnarkEval(api, evalInputs)

		// right : r^{n}-1 Q(r)
		qr := quotientYs[i]
		right := api.Mul(annulator, qr)

		api.AssertIsEqual(left, right)
		logrus.Debugf("verifying global constraint : DONE")

	}
}

// recombineQuotientSharesEvaluation returns the evaluations of the quotients
// on point r
func (ctx EvaluationVerifier) recombineQuotientSharesEvaluation(run wizard.Runtime, r field.Element) ([]field.Element, error) {

	var (
		// res stores the list of the recombined quotient evaluations for each
		// combination.
		recombinedYs = make([]field.Element, len(ctx.Ratios))
		// ys stores the values of the quotient shares ordered by ratio
		qYs      = make([][]field.Element, utils.Max(ctx.Ratios...))
		maxRatio = utils.Max(ctx.Ratios...)
		// shiftedR = r / g where g is the generator of the multiplicative group
		shiftedR field.Element
		// mulGen is the generator of the multiplicative group
		mulGenInv = fft.NewDomain(maxRatio * ctx.DomainSize).FrMultiplicativeGenInv
		// omegaN is a root of unity generating the domain of size `domainSize
		// * maxRatio`
		omegaN = fft.GetOmega(ctx.DomainSize * maxRatio)
	)

	shiftedR.Mul(&r, &mulGenInv)

	for i, q := range ctx.QuotientEvals {
		params := run.GetUnivariateParams(q.Name())
		qYs[i] = params.Ys

		// Check that the provided value for x is the right one
		providedX := params.X
		var expectedX field.Element
		expectedX.Inverse(&omegaN)
		expectedX.Exp(expectedX, big.NewInt(int64(i)))
		expectedX.Mul(&expectedX, &shiftedR)
		if providedX != expectedX {
			return nil, fmt.Errorf("bad X value")
		}
	}

	for i, ratio := range ctx.Ratios {
		var (
			jumpBy = maxRatio / ratio
			ys     = make([]field.Element, ratio)
		)

		for j := range ctx.QuotientShares[i] {
			ys[j] = qYs[j*jumpBy][0]
			qYs[j*jumpBy] = qYs[j*jumpBy][1:]
		}

		var (
			m          = ctx.DomainSize
			n          = ctx.DomainSize * ratio
			omegaRatio = fft.GetOmega(ratio)
			rPowM      field.Element
			// outerFactor stores m/n*(r^n - 1)
			outerFactor   = shiftedR
			one           = field.One()
			omegaRatioInv field.Element
			res           field.Element
			ratioInvField = field.NewElement(uint64(ratio))
		)

		rPowM.Exp(shiftedR, big.NewInt(int64(m)))
		ratioInvField.Inverse(&ratioInvField)
		omegaRatioInv.Inverse(&omegaRatio)

		for k := range ys {

			// tmp stores ys[k] / ((r^m / omegaRatio^k) - 1)
			var tmp field.Element
			tmp.Exp(omegaRatioInv, big.NewInt(int64(k)))
			tmp.Mul(&tmp, &rPowM)
			tmp.Sub(&tmp, &one)
			tmp.Div(&ys[k], &tmp)

			res.Add(&res, &tmp)
		}

		outerFactor.Exp(shiftedR, big.NewInt(int64(n)))
		outerFactor.Sub(&outerFactor, &one)
		outerFactor.Mul(&outerFactor, &ratioInvField)
		res.Mul(&res, &outerFactor)
		recombinedYs[i] = res
	}

	return recombinedYs, nil
}

// recombineQuotientSharesEvaluation returns the evaluations of the quotients
// on point r
func (ctx EvaluationVerifier) recombineQuotientSharesEvaluationGnark(api frontend.API, run wizard.GnarkRuntime, r frontend.Variable) []frontend.Variable {

	var (
		// res stores the list of the recombined quotient evaluations for each
		// combination.
		recombinedYs = make([]frontend.Variable, len(ctx.Ratios))
		// ys stores the values of the quotient shares ordered by ratio
		qYs      = make([][]frontend.Variable, utils.Max(ctx.Ratios...))
		maxRatio = utils.Max(ctx.Ratios...)
		// shiftedR = r / g where g is the generator of the multiplicative group
		shiftedR frontend.Variable
		// mulGen is the generator of the multiplicative group
		mulGenInv = fft.NewDomain(maxRatio * ctx.DomainSize).FrMultiplicativeGenInv
		// omegaN is a root of unity generating the domain of size `domainSize
		// * maxRatio`
		omegaN = fft.GetOmega(ctx.DomainSize * maxRatio)
	)

	shiftedR = api.Mul(r, mulGenInv)

	for i, q := range ctx.QuotientEvals {
		params := run.GetUnivariateParams(q.Name())
		qYs[i] = params.Ys

		// Check that the provided value for x is the right one
		providedX := params.X
		var expectedX frontend.Variable
		expectedX = api.Inverse(omegaN)
		expectedX = gnarkutil.Exp(api, expectedX, i)
		expectedX = api.Mul(expectedX, shiftedR)
		api.AssertIsEqual(providedX, expectedX)
	}

	for i, ratio := range ctx.Ratios {
		var (
			jumpBy = maxRatio / ratio
			ys     = make([]frontend.Variable, ratio)
		)

		for j := range ctx.QuotientShares[i] {
			ys[j] = qYs[j*jumpBy][0]
			qYs[j*jumpBy] = qYs[j*jumpBy][1:]
		}

		var (
			m          = ctx.DomainSize
			n          = ctx.DomainSize * ratio
			omegaRatio = fft.GetOmega(ratio)
			// outerFactor stores m/n*(r^n - 1)
			one           = field.One()
			omegaRatioInv field.Element
			res           = frontend.Variable(0)
			ratioInvField = field.NewElement(uint64(ratio))
		)

		rPowM := gnarkutil.Exp(api, shiftedR, m)
		ratioInvField.Inverse(&ratioInvField)
		omegaRatioInv.Inverse(&omegaRatio)

		for k := range ys {

			// tmp stores ys[k] / ((r^m / omegaRatio^k) - 1)
			var omegaInvPowK field.Element
			omegaInvPowK.Exp(omegaRatioInv, big.NewInt(int64(k)))
			tmp := api.Mul(omegaInvPowK, rPowM)
			tmp = api.Sub(tmp, one)
			tmp = api.Div(ys[k], tmp)

			res = api.Add(res, tmp)
		}

		outerFactor := gnarkutil.Exp(api, shiftedR, n)
		outerFactor = api.Sub(outerFactor, one)
		outerFactor = api.Mul(outerFactor, ratioInvField)
		res = api.Mul(res, outerFactor)
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
