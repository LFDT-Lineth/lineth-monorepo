package globalcs

import (
	"math/big"

	"github.com/consensys/gnark-crypto/field/koalabear/fft"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/variables"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// mergingCtx collects all the compilation input, output and artefacts pertaining
// the merging of the global constraints
type mergingCtx struct {

	// DomainSize corresponds to the shared domain size of all the uncompiled global
	// constraints. The fact that all the global constraints share the same domain
	// size is a pre-condition for the compiler and is enforced by Split/Stick compilers.
	DomainSize int

	// RatioBuckets stores arranged by "ratio".
	//
	// The "ratio" denotes the ratio
	// between the quotient size for that particular constraint and the domain
	// size of the constraint.
	//
	//
	// All the constraints happening here have the same domain size even when they
	// are in different bucket. The only difference is then the degree and the
	// and the offset of the constraint.
	//
	// The constraints that are stored here are modified w.r.t. the one initially
	// present in the compiledIOP when the compiler starts because they are multiplied
	// by X-1, X-\omega, X-\omega^2 to account for the bound cancellation.
	RatioBuckets map[int][]*symbolic.Expression

	// Ratios stores the list of the "ratios" in the order in which they are
	// encountered in the compiled IOP.
	//
	// The "ratio" denotes the ratio
	// between the quotient size for that particular constraint and the domain
	// size of the constraint.
	Ratios []int
}

// accumulateFromDegreeReducer accumulates the constraints from the degree reducer
func accumulateFromDegreeReducer(degreeReducer *DegreeReductionStep) (mergingCtx, bool) {

	if len(degreeReducer.DegreeReducedExpression) == 0 {
		return mergingCtx{}, false
	}

	ctx := mergingCtx{
		RatioBuckets: make(map[int][]*symbolic.Expression),
		DomainSize:   degreeReducer.DomainSize,
	}

	for i := range degreeReducer.DegreeReducedExpression {
		ctx.registerExpression(degreeReducer.DegreeReducedExpression[i])
	}

	return ctx, true
}

// aggregateConstraints returns the list of the aggregated constraints
func (ctx *mergingCtx) aggregateConstraints(comp *wizard.CompiledIOP) []*symbolic.Expression {

	var (
		aggregateExpressions = make([]*symbolic.Expression, len(ctx.Ratios))
		initialRound         = comp.NumRounds()
		mergingCoin          = comp.InsertCoin(initialRound, coin.Name(deriveName(comp, DEGREE_RANDOMNESS)), coin.FieldExt)
	)

	for i, ratio := range ctx.Ratios {
		aggregateExpressions[i] = symbolic.NewPolyEval(mergingCoin.AsVariable(), ctx.RatioBuckets[ratio])
	}

	return aggregateExpressions
}

// registerExpression registers an (already bound-constrained) expression in the
// contexte.
func (ctx *mergingCtx) registerExpression(expr *symbolic.Expression) {

	ratio := getExprRatio(expr)

	// Initialize the outer-maps / slices if the entries are not already allocated
	if _, ok := ctx.RatioBuckets[ratio]; !ok {
		ctx.RatioBuckets[ratio] = []*symbolic.Expression{}
		ctx.Ratios = append(ctx.Ratios, ratio)
	}

	ctx.RatioBuckets[ratio] = append(ctx.RatioBuckets[ratio], expr)
}

// getBoundCancelledExpression computes the "bound cancelled expression" for the
// constraint cs. Namely, the constraints expression is multiplied by terms of the
// form X-\omega^k to cancel the expression at position "k" if required. If the
// constraint uses the "noBoundCancel" feature, then the constraint expression is
// directly returned.
func getBoundCancelledExpression(cs query.GlobalConstraint) *symbolic.Expression {

	if cs.NoBoundCancel {
		return cs.Expression
	}

	var (
		cancelRange = query.MinMaxOffset(&cs)
		res         = cs.Expression
		domainSize  = cs.DomainSize
		x           = variables.NewXVar()
		omega, _    = fft.Generator(uint64(domainSize))
		// factorTop and factorBottom are used to store the terms of the form
		// X-\omega^k. They are constructed, disabling simplifications and in
		// a way that helps the evaluator to regroup shared subexpressions.
		factorTop    *symbolic.Expression
		factorBottom *symbolic.Expression
		factorCount  = 0
	)

	if cancelRange.Min < 0 {
		// Cancels the expression on the range [0, -cancelRange.Min)
		for i := 0; i < -cancelRange.Min; i++ {

			factorCount++
			var root field.Element
			root.Exp(omega, big.NewInt(int64(i)))
			term := symbolic.Sub(x, root)

			if factorBottom == nil {
				factorBottom = term
			} else {
				factorBottom = symbolic.MulNoSimplify(factorBottom, term)
			}
		}
	}

	if cancelRange.Max > 0 {
		// Cancels the expression on the range (N-cancelRange.Max-1, N-1]
		for i := 0; i < cancelRange.Max; i++ {

			factorCount++
			var root field.Element
			point := domainSize - i - 1
			root.Exp(omega, big.NewInt(int64(point)))
			term := symbolic.Sub(x, root)

			if factorTop == nil {
				factorTop = term
			} else {
				factorTop = symbolic.MulNoSimplify(factorTop, term)
			}
		}
	}

	if factorCount > 10000 {
		utils.Panic("too many terms to cancel: %v + %v = %v", -cancelRange.Min, cancelRange.Max, factorCount)
	}

	switch {
	case factorTop != nil && factorBottom != nil:
		return symbolic.MulNoSimplify(factorTop, factorBottom, res)
	case factorTop != nil:
		return symbolic.MulNoSimplify(factorTop, res)
	case factorBottom != nil:
		return symbolic.MulNoSimplify(factorBottom, res)
	default:
		return res
	}
}

// getExprRatio computes the ratio of the expression and ceil to the next power
// of two. The input expression should be pre-bound-cancelled. The domainSize
func getExprRatio(expr *symbolic.Expression) int {
	var (
		board        = expr.Board()
		domainSize   = column.ExprIsOnSameLengthHandles(&board)
		exprDegree   = board.Degree(GetDegree(domainSize))
		quotientSize = exprDegree - domainSize + 1
		ratio        = utils.DivCeil(quotientSize, domainSize)
	)
	return utils.NextPowerOfTwo(max(1, ratio))
}

// GetDegree is a generator returning a DegreeGetter that can be passed to
// [symbolic.ExpressionBoard.Degree]. The generator takes the domain size as
// input.
func GetDegree(size int) func(iface interface{}) int {
	return func(iface interface{}) int {
		switch v := iface.(type) {
		case ifaces.Column:
			// Univariate polynomials is X. We pad them with zeroes so it is safe
			// to return the domainSize directly.
			if size != v.Size() {
				panic("unconsistent sizes for the commitments")
			}
			// The size gives the number of coefficients , but we return the degree
			// hence the - 1
			return v.Size() - 1
		case coin.Info, ifaces.Accessor:
			// Coins are treated
			return 0
		case variables.X:
			return 1
		case variables.PeriodicSample:
			return size - size/v.T
		default:
			// Otherwise, it might be an internal replacement for column in
			// in the case intermediate columns.
			return size - 1
		}
	}
}
