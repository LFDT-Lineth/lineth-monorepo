package univariates

import (
	"errors"
	"fmt"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/sirupsen/logrus"
)

func CompileLocalOpening(comp *wizard.CompiledIOP) {

	logrus.Trace("started local opening compiler")
	defer logrus.Trace("finished local opening compiler")

	// The main idea is that we want to group the fixed point queries
	// that are on the same points. That way, we maintain the invariant
	// that all univariate queries are on different points.

	ctx := newLocalOpeningCtx(comp)
	ctx.populateWith(comp)

	if len(ctx.compiledQueries) == 0 {
		logrus.Infof("no fixed point query to compile, skipping")
		return
	}

	comp.InsertUnivariate(
		ctx.startRound,
		ctx.fixedToVariable(),
		ctx.handles,
	)

	comp.SubProvers.AppendToInner(ctx.startRound, ctx.prover)
	comp.InsertVerifier(ctx.startRound, ctx.verifier, ctx.gnarkVerifier)
}

type localOpeningCtx struct {
	handles          []ifaces.Column
	startRound       int
	compiledQueries  []query.LocalOpening
	selfRecursionCnt int
}

// returns an empty context with all subcollection initialized
func newLocalOpeningCtx(comp *wizard.CompiledIOP) localOpeningCtx {
	return localOpeningCtx{
		handles:          []ifaces.Column{},
		compiledQueries:  []query.LocalOpening{},
		selfRecursionCnt: comp.SelfRecursionCount,
	}
}

// populate the context with the registered fixed-point queries of a comp
func (ctx *localOpeningCtx) populateWith(comp *wizard.CompiledIOP) {
	// We sort the query by sizes
	handles := []ifaces.Column{}
	startRound := 0
	compiledQueries := []query.LocalOpening{}

	// The main idea is that we want to aggregate the fixed point evaluations
	// that are on the same points.

	for _, qName := range comp.QueriesParams.AllUnignoredKeys() {
		q, ok := comp.QueriesParams.Data(qName).(query.LocalOpening)
		if !ok {
			// not an inner-product query
			continue
		}

		comp.QueriesParams.MarkAsIgnored(qName)
		compiledQueries = append(compiledQueries, q)
		handles = append(handles, q.Pol)
		startRound = utils.Max(startRound, comp.QueriesParams.Round(qName))
	}

	ctx.compiledQueries = compiledQueries
	ctx.startRound = startRound
	ctx.handles = handles
}

func (ctx *localOpeningCtx) prover(assi *wizard.ProverRuntime) {
	ys := []field.Element{}

	// Collect the evaluation from the assigned compiled queries
	for _, q := range ctx.compiledQueries {
		params := assi.GetLocalPointEvalParams(q.ID)
		ys = append(ys, params.Y)
	}

	assi.AssignUnivariate(ctx.fixedToVariable(), field.One(), ys...)
}

func (ctx localOpeningCtx) verifier(assi *wizard.VerifierRuntime) error {
	ys := []field.Element{}

	// Collect the evaluation from the assigned compiled queries
	for _, q := range ctx.compiledQueries {
		params := assi.GetLocalPointEvalParams(q.ID)
		ys = append(ys, params.Y)
	}

	newParams := assi.GetUnivariateParams(ctx.fixedToVariable())

	if !newParams.X.IsOne() {
		return fmt.Errorf("the x of the evaluation was %v, should be one", newParams.X.String())
	}

	if len(ys) != len(newParams.Ys) {
		return fmt.Errorf("the ys do not have the same length")
	}

	errMsg := "fixed point compiler verifier failed\n"
	anyErr := false
	for i := range ys {
		if ys[i] != newParams.Ys[i] {
			anyErr = true
			errMsg += fmt.Sprintf("\ti=%v, ys[i]=%v, newYs[i]=%v\n", i, ys[i].String(), newParams.Ys[i].String())
		}
	}

	if anyErr {
		return errors.New(errMsg)
	}

	return nil
}

func (ctx localOpeningCtx) gnarkVerifier(api frontend.API, c *wizard.WizardVerifierCircuit) {
	ys := []frontend.Variable{}

	// Collect the evaluation from the assigned compiled queries
	for _, q := range ctx.compiledQueries {
		y := c.GetLocalPointEvalParams(q.ID)
		ys = append(ys, y.Y)
	}

	newParams := c.GetUnivariateParams(ctx.fixedToVariable())

	api.AssertIsEqual(newParams.X, 1)

	// Sanity-check to happen in the circuit definition time
	if len(ys) != len(newParams.Ys) {
		utils.Panic("the ys do not have the same length")
	}

	// Check that ys are the same
	for i := range ys {
		api.AssertIsEqual(ys[i], newParams.Ys[i])
	}
}

func (ctx *localOpeningCtx) fixedToVariable() ifaces.QueryID {
	return ifaces.QueryIDf("FIXED_TO_VARIABLE_%v", ctx.selfRecursionCnt)
}
