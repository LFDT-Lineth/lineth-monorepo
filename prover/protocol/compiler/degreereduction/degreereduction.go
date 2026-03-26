package degreereduction

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
	// MaxRound is the maximum round of the global constraints
	MaxRound int
}

// DegreeReduce reduces the degree of the global constraints.
func DegreeReduce(degreeBound int) func(comp *wizard.CompiledIOP) {

	return func(comp *wizard.CompiledIOP) {

		constraints, exprs, maxRound := listExpressions(comp)
		if len(exprs) == 0 {
			return
		}

		var (
			degreeReducedExpr, elimExpr, newVariables = sym.ReduceDegreeOfExpressions(
				exprs,
				degreeBound,
				GetDegree,
				sym.DegreeReductionConfig{
					// The min degree **must** larger than domainSize and small than
					// the degree bound.
					MinDegreeForCandidate: 2,
					// Degree-0 terms (accessors, coins) must not be counted against
					// the NLast budget: they cannot increase the degree of a
					// sub-product and, without this, a product whose degree-0
					// children outnumber NLast would exhaust the budget before
					// reaching any degree-1 factor, producing zero candidates.
					MinWeightForTerm: 1,
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
				MaxRound:              maxRound,
			}

			exprCompilationWG = &sync.WaitGroup{}
			sem               = make(chan struct{}, 8)
			newAll            []*sym.Expression
		)

		defer close(sem)

		for i := range newVariables {

			board := elimExpr[i].Board()
			metadatas := board.ListVariableMetadata()
			domainSize := 0

			// Using the function column.ExprIsOnSameLengthHandles to get the
			// the domain size would not be possible because elimExpr might
			// contain only references to other eliminated variables.
		metadatasLoop:
			for metadata := range metadatas {

				switch metadata := metadatas[metadata].(type) {
				case ifaces.Column:
					domainSize = metadata.Size()
					break metadatasLoop
				case sym.EliminatedVarMetadata:
					id := metadata.ID()
					domainSize = degRedStep.NewColumns[id].Size()
					if domainSize == 0 {
						panic("an eliminated expression may only depends on the previous ones")
					}
					break metadatasLoop
				}
			}

			degRedStep.NewColumns[i] = comp.InsertCommit(
				maxRound,
				ifaces.ColIDf("COMP_%v_ELIM_%v", comp.SelfRecursionCount, i),
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
			switch cs := constraints[i].(type) {
			case query.LocalConstraint:
				comp.InsertLocal(maxRound, cs.ID+"_DEGREEREDUCED", newExpr)
			case query.GlobalConstraint:
				if cs.NoBoundCancel {
					comp.InsertGlobal(maxRound, cs.ID+"_DEGREEREDUCED", newExpr, true)
				} else {
					offsetRange := query.MinMaxOffset(&cs)
					comp.InsertGlobalOverrideOffset(maxRound, cs.ID+"_DEGREEREDUCED", newExpr, offsetRange)
				}
			}

			newAll = append(newAll, newExpr)
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

			comp.InsertGlobal(
				maxRound,
				ifaces.QueryIDf("DEGREEREDUCTION_COMP_%v_NEW_%v", comp.SelfRecursionCount, i),
				sym.Sub(degRedStep.NewColumns[i], elimExpr),
			)

			newAll = append(newAll, elimExpr)
		}

		sym.AssertHasNoElimVarMetadata(newAll)

		comp.RegisterProverAction(maxRound, degRedStep)
		exprCompilationWG.Wait()
	}
}

// Run implements the [wizard.ProverAction] interface. Namely, it assigns the
// NewColumns by evaluating the NewColumnsExpressions.
func (d *DegreeReductionStep) Run(run *wizard.ProverRuntime) {

	for i, expr := range d.NewColumnsExpressions {

		var (
			metadatas  = expr.ListVariableMetadata()
			evalInputs = make([]sv.SmartVector, len(metadatas))
			domainSize = 0
		)

		// The first pass is to get the domain size. The second pass assigns
		// everything that needs to know what the domain size is.
		for k, metadata := range metadatas {

			switch metadata := metadata.(type) {
			case ifaces.Column:
				evalInputs[k] = metadata.GetColAssignment(run)
				domainSize = metadata.Size()
			}
		}

		for k, metadata := range metadatas {

			switch metadata := metadata.(type) {
			case ifaces.Column:
				// pass as we already added a column there
			case coin.Info:
				if metadata.Type != coin.FieldExt && metadata.Type != coin.FieldFromSeed {
					utils.Panic("unsupported, coins are always over field extensions")
				}
				evalInputs[k] = sv.NewConstantExt(run.GetRandomCoinFieldExt(metadata.Name), domainSize)
			case variables.X:
				evalInputs[k] = metadata.EvalCoset(domainSize, 0, 1, false)
			case variables.PeriodicSample:
				evalInputs[k] = metadata.EvalCoset(domainSize, 0, 1, false)
			case ifaces.Accessor:
				if metadata.IsBase() {
					evalInputs[k] = sv.NewConstant(metadata.GetVal(run), domainSize)
				} else {
					evalInputs[k] = sv.NewConstantExt(metadata.GetValExt(run), domainSize)
				}
			default:
				utils.Panic("Not a variable type %v", reflect.TypeOf(metadata))
			}
		}

		run.AssignColumn(d.NewColumns[i].GetColID(), expr.Evaluate(evalInputs))
	}

}

// GetDegree is a generator returning a DegreeGetter that can be passed to
// [symbolic.ExpressionBoard.Degree]. The generator takes the domain size as
// input.s
func GetDegree(iface any) int {
	switch iface.(type) {
	case ifaces.Column, symbolic.EliminatedVarMetadata, variables.PeriodicSample:
		return 1
	case coin.Info, ifaces.Accessor:
		// Coins are treated
		return 0
	default:
		utils.Panic("unexpected type: %T", iface)
		return -1 // unreachable
	}
}

// listExpressions lists all the constraints arising from local constraints and
// local constraints and list separately their expressions. The constraints are
// provided in the same order as they are declared. Meaning that constraints
// sharing subexpressions are more likely to be close in the list.
func listExpressions(comp *wizard.CompiledIOP) (
	constraints []ifaces.Query,
	expressions []*sym.Expression,
	maxRound int,
) {

	for _, qName := range comp.QueriesNoParams.AllUnignoredKeys() {

		var (
			cs   = comp.QueriesNoParams.Data(qName)
			expr *sym.Expression
		)

		switch cs := cs.(type) {
		case query.LocalConstraint:
			expr = cs.Expression
		case query.GlobalConstraint:
			expr = cs.Expression
		default:
			continue
		}

		// Mark the constraint as ignored, so that it does not get compiled a
		// second time by a sub-sequent round of compilation.
		comp.QueriesNoParams.MarkAsIgnored(qName)
		constraints = append(constraints, cs)
		expressions = append(expressions, expr)
		maxRound = max(maxRound, comp.QueriesNoParams.Round(qName))
	}

	return constraints, expressions, maxRound
}
