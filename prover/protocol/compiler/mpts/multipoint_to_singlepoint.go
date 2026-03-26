package mpts

import (
	"errors"
	"fmt"
	"slices"

	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/splitextension"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/sirupsen/logrus"
)

// MultiPointToSinglePointCompilation holds the compilation context of the
// multipoint to singlepoint step. This step ensures that all the PIOP
// evaluations are done at the same points for all the polynomials. This
// ensures that Vortex can be applied on top.
type MultipointToSinglepointCompilation struct {
	// Queries lists all the compiled evaluation queries that are relevant
	// to the compilation step.
	Queries []query.UnivariateEval

	// Polys lists all the polynomials involved in the compilation. Meaning
	// all the polynomials that are affetcted by one of the queries. The
	// pols appear in the following order: first by round, then by column
	// name (alphabetical order). Finally, each "round" sequence is punctuated
	// by a sequence of "shadow columns", these are columns that are added
	// by the compiler to match a prescribed profile.
	//
	// The first round is a little special as the precomputed polynomials are
	// considered in their own round, placed at the beginning.
	Polys []ifaces.Column

	// ExplictlyEvaluated columns are columns that are explicitly evaluated
	// in the compilation. This includes the non-committed or non-precomputed
	// columns and all the verifier columns. They are not part of the generated
	// query.
	ExplicitlyEvaluated []ifaces.Column

	// EvalPointOfPolys lists, for each entry in [Polys], the entries of
	// [Queries] corresponding to evaluation points affected to the
	// polynomial. The list can be empty for shadow columns.
	EvalPointOfPolys [][]int

	// PolysOfEvalPoint lists, for each entry in [Queries], the entries of
	// [Polys] corresponding to polynomials evaluated at the same point.
	PolysOfEvalPoint [][]int

	// PolyPositionInQuery lists, for each entry in [Queries], the position
	// of the polynomial in the query. This is used to speed up the evaluation
	// of the quotient.
	PolyPositionInQuery [][]int

	// NumColumnProfileOpt is an optional compilation parameter that can be
	// set to control the number of column in every round of the compilation.
	// This can be used to obtain uniform wizards when doing conglomeration.
	NumColumnProfileOpt []int

	// NumColumnProfilePrecomputed is as [NumColumnProfileOpt] but for
	// precomputed polynomials. The value of this field is only read if the
	// value of [NumColumnProfileOpt] is not nil. It serves the same purpose
	// as [NumColumnProfileOpt] as well.
	NumColumnProfilePrecomputed int

	// AddUnconstrainedColumnsOpt is an optional compilation parameter that
	// can be set to control whether unconstrained columns are added to the
	// newQuery.
	AddUnconstrainedColumnsOpt bool

	// NumRow is the number of rows in the columns that are compiled. The
	// value is lazily evaluated and the evaluation procedure sanity-checks
	// that all the columns has the same number of rows. The value of this
	// field should not be accessed directly and the caller should instead
	// use getNumRow().
	NumRow int

	// NewQuery is the query that is produced by the compilation, also named
	// the "Grail" query in the paper. The evaluation spans overall the
	// polynomials found in polys and the quotient.
	NewQuery query.UnivariateEval

	// Quotient is the column constructed as the quotient of all the
	// polynomials found in polys by their respective evaluation claims.
	Quotient ifaces.Column

	// LinCombCoeffLambda is the linear combination coefficient that is used to
	// accumulate the quotient.
	LinCombCoeffLambda, LinCombCoeffRho coin.Info

	// EvaluationPoint is the random evaluation point that is used to evaluate
	// the quotient and all the evaluation claims.
	EvaluationPoint coin.Info
}

// Compile applies the multipoint to singlepoint compilation pass over `comp`.
func Compile(options ...Option) func(*wizard.CompiledIOP) {
	return func(comp *wizard.CompiledIOP) {
		if ctx := compileMultipointToSinglepoint(comp, options); ctx == nil {
			return
		}
		// adding the split-extension pass right after MPTS
		// and before Vortex
		splitextension.CompileSplitExtToBase(comp)
	}
}

// compileMultipointToSinglepoint takes all the uncompiled multipoint to
// singlepoint queries and compile them using a quotient accumulation technique.
func compileMultipointToSinglepoint(comp *wizard.CompiledIOP, options []Option) *MultipointToSinglepointCompilation {
	ctx := &MultipointToSinglepointCompilation{
		Queries: getAndMarkAsCompiledQueries(comp),
	}

	if len(ctx.Queries) == 0 {
		return nil
	}

	for _, op := range options {
		op(ctx)
	}

	polysByRound, polyPrecomputed, direct := sortPolynomialsByRoundAndName(comp, ctx.Queries, ctx.AddUnconstrainedColumnsOpt)

	ctx.setMaxNumberOfRowsOf(slices.Concat(
		append(polysByRound, polyPrecomputed, direct)...),
	)

	var err, errLocal error

	if ctx.NumColumnProfileOpt != nil {

		var (
			profile            = []int{}
			profilePrecomputed = len(polyPrecomputed)
		)

		// startingRound is the first round that is not empty in polyRound
		// ignoring precomputed columns.
		startingRound := getStartingRound(comp, polysByRound)

		polyPrecomputed, errLocal = extendPWithShadowColumns(comp, 0,
			ctx.NumRow, polyPrecomputed, ctx.NumColumnProfilePrecomputed, true)

		err = errors.Join(err, errLocal)

		for round := startingRound; round < len(polysByRound); round++ {

			profile = append(profile, len(polysByRound[round]))

			// This panic routine is useful as it gives all the informations for
			// the dev to fix the value of the mpts profile in use if needs be.
			if round-startingRound >= len(ctx.NumColumnProfileOpt) {
				numPolPerRounds := []int{}
				for round := startingRound; round < len(polysByRound); round++ {
					numPolPerRounds = append(numPolPerRounds, len(polysByRound[round]))
				}
				utils.Panic("the number of polynomials in every round is not set. Got %v", numPolPerRounds)
			}

			polysByRound[round], errLocal = extendPWithShadowColumns(comp, round,
				ctx.NumRow, polysByRound[round], ctx.NumColumnProfileOpt[round-startingRound], false)

			err = errors.Join(err, errLocal)
		}

		logrus.Infof(
			"[MPTS] extending number of rows, precomputed: %v -> %v, profile: %v -> %v",
			profilePrecomputed, ctx.NumColumnProfilePrecomputed,
			profile, ctx.NumColumnProfileOpt,
		)
	}

	if err != nil {
		panic(err)
	}

	ctx.Polys = slices.Concat(append([][]ifaces.Column{polyPrecomputed, direct}, polysByRound...)...)
	ctx.ExplicitlyEvaluated = direct

	// This is used to disambiguate column names in such a way that the same
	// the column names are compatible with conglomeration. This counts the
	// total number of elements in the polys slice, each extension element
	// counts for 4.
	numMPTSColumns := len(ctx.Polys)
	for i := range ctx.Polys {
		if !ctx.Polys[i].IsBase() {
			numMPTSColumns += 3
		}
	}

	ctx.LinCombCoeffLambda = comp.InsertCoin(
		ctx.getNumRound(comp),
		coin.Namef("MPTS_LINCOMB_COEFF_LAMBDA_%v_%v", comp.SelfRecursionCount, numMPTSColumns),
		coin.FieldExt,
	)

	ctx.LinCombCoeffRho = comp.InsertCoin(
		ctx.getNumRound(comp),
		coin.Namef("MPTS_LINCOMB_COEFF_RHO_%v_%v", comp.SelfRecursionCount, numMPTSColumns),
		coin.FieldExt,
	)

	ctx.Quotient = comp.InsertCommit(
		ctx.getNumRound(comp),
		ifaces.ColIDf("MPTS_QUOTIENT_%v_%v", comp.SelfRecursionCount, numMPTSColumns),
		ctx.NumRow,
		false,
	)

	ctx.EvaluationPoint = comp.InsertCoin(
		ctx.getNumRound(comp)+1,
		coin.Namef("MPTS_EVALUATION_POINT_%v_%v", comp.SelfRecursionCount, numMPTSColumns),
		coin.FieldExt,
	)

	ctx.NewQuery = comp.InsertUnivariate(
		ctx.getNumRound(comp)+1,
		ifaces.QueryIDf("MPTS_NEW_QUERY_%v_%v", comp.SelfRecursionCount, numMPTSColumns),
		append(slices.Concat(append([][]ifaces.Column{polyPrecomputed}, polysByRound...)...), ctx.Quotient),
	)

	ctx.EvalPointOfPolys, ctx.PolysOfEvalPoint, ctx.PolyPositionInQuery = indexPolysAndPoints(ctx.Polys, ctx.Queries)

	comp.RegisterProverAction(ctx.getNumRound(comp), QuotientAccumulation{ctx})
	comp.RegisterProverAction(ctx.getNumRound(comp)+1, RandomPointEvaluation{ctx})
	comp.RegisterVerifierAction(ctx.getNumRound(comp)+1, VerifierAction{ctx})

	return ctx
}

// setMaxNumberOfRowsOf scans the list of columns and returns the largest size.
// and set it in the context. The function returns an error if the list is empty.
func (ctx *MultipointToSinglepointCompilation) setMaxNumberOfRowsOf(columns []ifaces.Column) int {
	if len(columns) == 0 {
		return 0
	}

	numRow := columns[0].Size()
	for i := 1; i < len(columns); i++ {
		numRow = max(numRow, columns[i].Size())
	}
	ctx.NumRow = numRow
	return numRow
}

// getNumRow returns the number of rows and panics if the field is not set
// in the context.
func (ctx *MultipointToSinglepointCompilation) getNumRow() int {
	if ctx.NumRow == 0 {
		utils.Panic("the number of rows is not set")
	}
	return ctx.NumRow
}

// getNumRound returns the number of rounds that are compiled. This is also
// the defintion round of the [LinCombCoeff] and [EvaluationPoint] in the
// context.
//
// The function assumes that the number of queries to compile is not 0.
func (ctx *MultipointToSinglepointCompilation) getNumRound(comp *wizard.CompiledIOP) int {
	maxRound := -1
	for i := range ctx.Queries {
		round := comp.QueriesParams.Round(ctx.Queries[i].Name())
		if round > maxRound {
			maxRound = round
		}
	}

	if maxRound < 0 {
		panic("no queries")
	}

	// This relies on the fact that the polynomials are sorted by round. So
	// the last one has the highest round number.
	return maxRound + 1
}

// getAndMarkAsCompiledQueries returns all the queries that are relevant
// to the multipoint to singlepoint compilation and mark them as ignored.
func getAndMarkAsCompiledQueries(comp *wizard.CompiledIOP) []query.UnivariateEval {
	var (
		selectedQueries = []query.UnivariateEval{}
		allQueries      = comp.QueriesParams.AllUnignoredKeys()
	)

	for _, qName := range allQueries {

		q, isUnivariate := comp.QueriesParams.Data(qName).(query.UnivariateEval)
		if !isUnivariate {
			continue
		}

		comp.QueriesParams.MarkAsIgnored(qName)
		selectedQueries = append(selectedQueries, q)
	}

	return selectedQueries
}

// sortPolynomialsByRoundAndName returns the polynomials sorted by round
// and then by name. The result is a double list of polynomials. Each entry
// corresponds to a round, and the sublists are sorted by name.
//
// Precomputed polynomials can be considered at round -1 and are placed at
// the beginning. addUnconstrainedColumn is a flag indicating whether to add
// the unconstrained columns, it will only add the columns with the
// [column.Committed] status.
func sortPolynomialsByRoundAndName(comp *wizard.CompiledIOP, queries []query.UnivariateEval, addUnconstrainedColumn bool) (compiledByRound [][]ifaces.Column, precomputed []ifaces.Column, direct []ifaces.Column) {
	compiledByRound = make([][]ifaces.Column, 0)
	precomputed = make([]ifaces.Column, 0)
	direct = make([]ifaces.Column, 0)

	for _, q := range queries {
		for _, poly := range q.Pols {

			if _, isShf := poly.(column.Shifted); isShf {
				utils.Panic("shifted polys are not supported. Please, run the naturalization pass prior to calling the MPTS pass")
			}

			if _, isV := poly.(verifiercol.VerifierCol); isV {
				direct = append(direct, poly)
				continue
			}

			if poly.(column.Natural).Status().IsPublic() {
				direct = append(direct, poly)
				continue
			}

			// This works assuming the input polys are [colum.Natural] columns.
			// Otherwise, the check would fail even if the column were a shifted
			// version of a precomputed column.
			if comp.Precomputed.Exists(poly.GetColID()) {
				precomputed = append(precomputed, poly)
				continue
			}

			round := poly.Round()
			compiledByRound = utils.GrowSliceSize(compiledByRound, round+1)
			compiledByRound[round] = append(compiledByRound[round], poly)
		}
	}

	if addUnconstrainedColumn {

		allColumns := comp.Columns.AllKeysCommitted()

		for _, c := range allColumns {
			col := comp.Columns.GetHandle(c)
			round := col.Round()
			compiledByRound = utils.GrowSliceSize(compiledByRound, round+1)
			compiledByRound[round] = append(compiledByRound[round], col)
		}
	}

	// cmpNames is a helper function returning a comparison between two columns
	// by name in alphabetical order.
	cmpNames := func(a, b ifaces.Column) int {
		if a.GetColID() < b.GetColID() {
			return -1
		}
		if a.GetColID() > b.GetColID() {
			return 1
		}
		return 0
	}

	// cleanSubList sorts and removes duplicates and empty entries from its
	// input list. The input slice is mutated and should not be used anymore.
	cleanSubList := func(s []ifaces.Column) []ifaces.Column {
		slices.SortFunc(s, cmpNames)
		s = slices.CompactFunc(s, func(a, b ifaces.Column) bool { return a.GetColID() == b.GetColID() })
		s = slices.Clip(s)
		return s
	}

	precomputed = cleanSubList(precomputed)
	for round := range compiledByRound {
		compiledByRound[round] = cleanSubList(compiledByRound[round])
	}

	return compiledByRound, precomputed, direct
}

// extendPWithShadowColumns adds shadow columns to the given list of polynomials
// to match a given profile. The profile corresponds to a target number of columns
// to meet in "p". The function will ignore the verifiercol from the count.
func extendPWithShadowColumns(comp *wizard.CompiledIOP, round int, numRow int, p []ifaces.Column, profile int, precomputed bool) ([]ifaces.Column, error) {
	if len(p) > profile {
		return nil, fmt.Errorf("the profile is too small for the given polynomials list, round=%v len(p)=%v profile=%v", round, len(p), profile)
	}

	numP := len(p)

	// This loop effective remove the verifiercol from consideration when evaluating
	// how many shadow columns are needed.
	for i := range p {
		_, isVcol := p[i].(verifiercol.VerifierCol)
		if isVcol {
			numP--
		}
		// if p[i] is an extension column, then it will be
		// split into 4 base columns, so we count it as 3 extra columns.
		if !p[i].IsBase() {
			numP = numP + 3
		}
	}

	// We check the same sanity as above just to be safe.
	if numP > profile {
		return nil, fmt.Errorf("the profile is too small for the given polynomials list, round=%v len(p)=%v profile=%v", round, numP, profile)
	}

	for i := numP; i < profile; i++ {

		var newShadowCol ifaces.Column
		if precomputed {
			newShadowCol = precomputedShadowRow(comp, numRow, i)
		} else {
			newShadowCol = autoAssignedShadowRow(comp, numRow, round, i-numP)
		}

		p = append(p, newShadowCol)
	}

	return p, nil
}

// getStartingRound returns the first position in s with a non-empty
// sublist. The function panics if all the sublists are empty of if s is an
// empty of nil list itself.
//
// The function will ignore the first round if it only contains precomputed
// columns. The function also ignores [VerifierCol].
func getStartingRound(comp *wizard.CompiledIOP, s [][]ifaces.Column) int {
	if len(s) == 0 {
		panic("empty list")
	}

	for j := range s[0] {

		_, isVcol := s[0][j].(verifiercol.VerifierCol)
		if isVcol {
			continue
		}

		if !comp.Precomputed.Exists(s[0][j].GetColID()) {
			fmt.Printf("found non precomputed column %s\n", s[0][j].GetColID())
			return 0
		}
	}

	for i := 1; i < len(s); i++ {
		if len(s[i]) > 0 {
			return i
		}
	}

	panic("all sublists are empty")
}

// indexPolysAndPoints indexes the polynomials and points in two lists.
// The first one indicates the queries affecting each polynomial and the
// second one indicates which polynomial are affected by each point.
//
// The returns are two lists of lists of integers referencing the positions
// in the inputs [polys] and [points].
func indexPolysAndPoints(polys []ifaces.Column, points []query.UnivariateEval) (evalPointOfPolys, polysOfEvalPoint, polyPositionInQuery [][]int) {
	evalPointOfPolys = make([][]int, len(polys))
	polysOfEvalPoint = make([][]int, len(points))
	polyPositionInQuery = make([][]int, len(points))
	polyNameToIndex := make(map[ifaces.ColID]int)

	for i := range polys {
		polyNameToIndex[polys[i].GetColID()] = i
	}

	type polyInfo struct {
		polyID int
		pos    int
	}
	tempPolysOfEvalPoint := make([][]polyInfo, len(points))

	for queryID, q := range points {
		for pos, poly := range q.Pols {
			polyID := polyNameToIndex[poly.GetColID()]
			evalPointOfPolys[polyID] = append(evalPointOfPolys[polyID], queryID)
			tempPolysOfEvalPoint[queryID] = append(tempPolysOfEvalPoint[queryID], polyInfo{polyID, pos})
		}
	}

	// evalPointOfPolys is built by iterating in-order over the queryIDs
	// by appending the queryIDs to each entry of evalPointOfPolys, we
	// can assume that the entries of evalPointOfPolys are sorted in
	// order. But this is not the case for polysOfEvalPoint as nothing
	// indicate they are sorted in [query.Pols] field.
	for i := range tempPolysOfEvalPoint {
		slices.SortFunc(tempPolysOfEvalPoint[i], func(a, b polyInfo) int {
			return a.polyID - b.polyID
		})
		for _, info := range tempPolysOfEvalPoint[i] {
			polysOfEvalPoint[i] = append(polysOfEvalPoint[i], info.polyID)
			polyPositionInQuery[i] = append(polyPositionInQuery[i], info.pos)
		}
	}

	return evalPointOfPolys, polysOfEvalPoint, polyPositionInQuery
}
