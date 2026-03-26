package globalcs

import (
	"reflect"
	"sync"

	sv "github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/variables"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/symbolic"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/sirupsen/logrus"
)

// DegreeReductionStep is the first step of the global constraint compilation
// process. It creates intermediate columns aimed at reducing the degree of the
// global constraints. The structure additionally implements the
// [wizard.ProverAction] package.
type DegreeReductionStep struct {
	// NewColumns are the intermediate columns returned by the step
	NewColumns []column.Natural
	// NewColumnsExpressions are the symbolic expressions corresponding to the
	// intermediate columns.
	NewColumnsExpressions []*sym.ExpressionBoard
	// DegreeReducedExpression is the list of expressions after degree reduction
	DegreeReducedExpression []*sym.Expression
	// DomainSize is the domain size of the global constraints
	DomainSize int
	// MaxRound is the maximum round of the global constraints
	MaxRound int
}

// degreeReduce reduces the degree of the global constraints.
func degreeReduce(comp *wizard.CompiledIOP, degreeBound int) *DegreeReductionStep {

	var (
		constraints, domainSize, maxRound = listAllGlobalConstraints(comp)
		exprs                             = make([]*sym.Expression, len(constraints))
	)

	for i, cs := range constraints {
		exprs[i] = getBoundCancelledExpression(cs)
		degree := exprs[i].Degree(GetDegree(domainSize))
		if degree >= 16*domainSize {
			logrus.
				WithField("constraint", cs.Name()).
				WithField("degree", degree).
				WithField("where", "globalcs.DegreeReducer").
				Debug("constraint with large degree")
			continue
		}
	}

	if len(exprs) == 0 {

		return &DegreeReductionStep{
			DomainSize: 0,
			MaxRound:   0,
			NewColumns: []column.Natural{},
			NewColumnsExpressions: []*sym.ExpressionBoard{
				{},
			},
			DegreeReducedExpression: exprs,
		}
	}

	var (
		degreeReducedExpr, elimExpr, newVariables = sym.ReduceDegreeOfExpressions(
			exprs,
			degreeBound*domainSize,
			GetDegree(domainSize),
			sym.DegreeReductionConfig{
				// The min degree **must** larger than domainSize and small than
				// the degree bound.
				MinDegreeForCandidate: 3 * domainSize / 2,
				// ignore degrees smaller than 100, which will correspond to product
				// of the form (X-a)(X-b)(..)
				MinWeightForTerm: 100,
				// Note: don't increase it too much because it can have dramatic
				// effects on the efficiency of the algorithm.
				NLast: 15,
				// Note: shouldn't be increased too much because it can have
				// dramatic effects on the efficiency of the algorithm.
				MaxCandidatePerRound: 1 << 13,
			},
		)

		degRedStep = &DegreeReductionStep{
			NewColumnsExpressions: make([]*sym.ExpressionBoard, len(newVariables)),
			NewColumns:            make([]column.Natural, len(newVariables)),
			DomainSize:            domainSize,
			MaxRound:              maxRound,
		}

		exprCompilationWG           = &sync.WaitGroup{}
		sem                         = make(chan struct{}, 8)
		newDegreeReducedExpressions = make([]*sym.Expression, 0, len(degreeReducedExpr)+len(newVariables))
	)

	defer close(sem)

	for i := range newVariables {
		degRedStep.NewColumns[i] = comp.InsertCommit(
			maxRound,
			ifaces.ColIDf("COMP_%v_ELIM_%v_GLOBALCS", comp.SelfRecursionCount, i),
			domainSize,
			elimExpr[i].IsBase,
		).(column.Natural)
	}

	// replaceElimVariable replace occurences of [EliminatedVarMetadata] into
	// the corresponding new column.
	replaceElimVariable := func(e *sym.Expression, children []*sym.Expression) (new *sym.Expression) {

		switch op := e.Operator.(type) {
		case sym.LinComb, sym.Product, sym.PolyEval, sym.Constant:
			return e.SameWithNewChildrenNoSimplify(children)
		case sym.Variable:
			elim, isElim := op.Metadata.(sym.EliminatedVarMetadata)
			if !isElim {
				return e
			}
			return symbolic.NewVariable(degRedStep.NewColumns[elim.ID()])
		default:
			utils.Panic("unexpected operator: %T", e.Operator)
			return nil
		}
	}

	for i := range degreeReducedExpr {
		newExpr := degreeReducedExpr[i].ReconstructBottomUpSingleThreaded(replaceElimVariable)
		newDegreeReducedExpressions = append(newDegreeReducedExpressions, newExpr)
	}

	for i := range newVariables {

		elimExpr := elimExpr[i].ReconstructBottomUpSingleThreaded(replaceElimVariable)
		board := elimExpr.Board()
		degRedStep.NewColumnsExpressions[i] = &board

		sem <- struct{}{}
		exprCompilationWG.Add(1)
		go func() {
			degRedStep.NewColumnsExpressions[i].Compile()
			exprCompilationWG.Done()
			<-sem
		}()

		newExpr := sym.Sub(degRedStep.NewColumns[i], elimExpr)
		newDegreeReducedExpressions = append(newDegreeReducedExpressions, newExpr)
	}

	sym.AssertHasNoElimVarMetadata(newDegreeReducedExpressions)

	degRedStep.DegreeReducedExpression = newDegreeReducedExpressions

	comp.RegisterProverAction(maxRound, degRedStep)
	exprCompilationWG.Wait()

	return degRedStep
}

// Run implements the [wizard.ProverAction] interface. Namely, it assigns the
// NewColumns by evaluating the NewColumnsExpressions.
func (d *DegreeReductionStep) Run(run *wizard.ProverRuntime) {

	for i, expr := range d.NewColumnsExpressions {

		var (
			metadatas  = expr.ListVariableMetadata()
			evalInputs = make([]sv.SmartVector, len(metadatas))
		)

		for k, metadata := range metadatas {

			switch metadata := metadata.(type) {
			case ifaces.Column:
				evalInputs[k] = metadata.GetColAssignment(run)
			case coin.Info:
				if metadata.IsBase() {
					utils.Panic("unsupported, coins are always over field extensions")

				} else {
					evalInputs[k] = sv.NewConstantExt(run.GetRandomCoinFieldExt(metadata.Name), d.DomainSize)
				}
			case variables.X:
				evalInputs[k] = metadata.EvalCoset(d.DomainSize, 0, 1, false)
			case variables.PeriodicSample:
				evalInputs[k] = metadata.EvalCoset(d.DomainSize, 0, 1, false)
			case ifaces.Accessor:
				if metadata.IsBase() {
					evalInputs[k] = sv.NewConstant(metadata.GetVal(run), d.DomainSize)
				} else {
					evalInputs[k] = sv.NewConstantExt(metadata.GetValExt(run), d.DomainSize)
				}
			default:
				utils.Panic("Not a variable type %v", reflect.TypeOf(metadata))
			}
		}

		run.AssignColumn(d.NewColumns[i].GetColID(), expr.Evaluate(evalInputs))
	}

}

// listAllGlobalConstraints lists all the global constraints in the
// [wizard.CompiledIOP] that can be compiled by the current compilation pass.
//
// The function also initializes the value of ctx.DomainSize.
// And it marks all the returned global constraints as ignored to signify they
// are being compiled (and won't need to be compiled again).
func listAllGlobalConstraints(
	comp *wizard.CompiledIOP,
) (
	constraints []query.GlobalConstraint,
	domainSize int,
	maxRound int,
) {

	for _, qName := range comp.QueriesNoParams.AllUnignoredKeys() {

		// Filter only the global constraints, excluding all other type of queries
		cs, ok := comp.QueriesNoParams.Data(qName).(query.GlobalConstraint)
		if !ok {
			// Not a global constraint
			continue
		}

		// For the first iteration, the domain size is unset so we need to initialize
		// it. This works because the domain size of a constraint cannot legally
		// be 0.
		if domainSize == 0 {
			domainSize = cs.DomainSize
		}

		// This enforces the precondition that all the global constraint must
		// share the same domain.
		if cs.DomainSize != domainSize {
			utils.Panic("At this point in the compilation process, we expect all constraints to have the same domain")
		}

		// Mark the constraint as ignored, so that it does not get compiled a
		// second time by a sub-sequent round of compilation.
		comp.QueriesNoParams.MarkAsIgnored(qName)
		constraints = append(constraints, cs)
		maxRound = max(maxRound, comp.QueriesNoParams.Round(qName))
	}

	return constraints, domainSize, maxRound
}
