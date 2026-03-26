package globalcs

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
)

const (
	GLOBAL_REDUCTION                string = "GLOBAL_REDUCTION"
	OFFSET_RANDOMNESS               string = "OFFSET_RANDOMNESS"
	DEGREE_RANDOMNESS               string = "DEGREE_RANDOMNESS"
	QUOTIENT_POLY_TMPL              string = "QUOTIENT_DEG_%v_SHARE_%v"
	EVALUATION_RANDOMESS            string = "EVALUATION_RANDOMNESS"
	UNIVARIATE_EVAL_ALL_HANDLES     string = "UNIV_EVAL_ALL_HANDLES"
	UNIVARIATE_EVAL_QUOTIENT_SHARES string = "UNIV_EVAL_QUOTIENT_%v_OVER_%v"
)

// Compile takes all the uncompiled global constraint found in comp and compile
// them using Plonk's quotient technique. The compiler also applies symbolic
// expression optimization and runtime memory optimizations for the prover.
func Compile(comp *wizard.CompiledIOP) {

	degreeRed := degreeReduce(comp, 5)

	if len(degreeRed.DegreeReducedExpression) == 0 {
		return
	}

	comp.RegisterProverAction(degreeRed.MaxRound, degreeRed)

	merging, anyCs := accumulateFromDegreeReducer(degreeRed)
	if !anyCs {
		return
	}

	var (
		aggregateExprs = merging.aggregateConstraints(comp)
		// factoredExprs   = factorExpressionList(comp, aggregateExprs)
		// TODO @gbotrel restore the factor logic.
		quotientCtx     = createQuotientCtx(comp, merging.Ratios, aggregateExprs)
		evaluationCtx   = declareUnivariateQueries(comp, quotientCtx)
		quotientRound   = quotientCtx.QuotientShares[0][0].Round()
		evaluationRound = quotientRound + 1
	)

	comp.RegisterProverAction(quotientRound, &quotientCtx)
	comp.RegisterProverAction(evaluationRound, EvaluationProver(evaluationCtx))
	comp.RegisterVerifierAction(evaluationRound, &EvaluationVerifier{EvaluationCtx: evaluationCtx})

}

func deriveName(comp *wizard.CompiledIOP, s string, args ...any) string {
	fmts := fmt.Sprintf(s, args...)
	return fmt.Sprintf("%v_%v_%v", GLOBAL_REDUCTION, comp.SelfRecursionCount, fmts)
}
