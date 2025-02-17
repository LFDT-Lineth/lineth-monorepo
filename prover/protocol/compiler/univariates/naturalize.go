package univariates

import (
	"fmt"
	"reflect"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/collection"
	"github.com/sirupsen/logrus"
)

const (
	NATURALIZE string = "NATURALIZE"
)

/*
This compiler ensures that all univariate queries relates to
`Natural` commitment. In a nutshell, it removes all the offset
, repeats etc..

Offset:

	P(xw) = a for x = t
		=>
	P(x) = a for x = wt

Repeat:

	P(x^2) = a for x = t
		=>
	P(x) = a for x = t^2

Interleaving:

	I(X) = 1/2*P(X)(X^n - 1) - 1/2*P(-X)(X^n + 1)
*/
func Naturalize(comp *wizard.CompiledIOP) {

	logrus.Trace("started naturalization compiler")
	defer logrus.Trace("finished naturalization compiler")

	// The compilation process is applied separately for each query
	for roundID := 0; roundID < comp.NumRounds(); roundID++ {
		for _, qName := range comp.QueriesParams.AllKeysAt(roundID) {

			if comp.QueriesParams.IsIgnored(qName) {
				continue
			}

			q_ := comp.QueriesParams.Data(qName)
			if _, ok := q_.(query.UnivariateEval); !ok {
				/*
					Every other type of parametrizable queries (inner-product, local opening)
					should have been compiled at this point.
				*/
				utils.Panic("query %v has type %v expected only univariate", qName, reflect.TypeOf(q_))
			}

			q := q_.(query.UnivariateEval)

			/*
				We skip the queries that are ineligible for compilation : the
				one that are related to only natural commitment already.
			*/
			isEligible := false
			for _, pol := range q.Pols {
				if pol.IsComposite() {
					isEligible = true
				}
			}

			if !isEligible {
				continue
			}

			// Skip if it was already compiled, else insert.
			if comp.QueriesParams.MarkAsIgnored(qName) {
				continue
			}

			/*
				Create the context
			*/
			ctx := naturalizationCtx{
				q:                q,
				roundID:          roundID,
				subQueriesNames:  []ifaces.QueryID{},
				polsPerSubQuery:  [][]ifaces.Column{},
				reprToSubQueryID: make(map[string]int),
			}

			ctx.registersTheNewQueries(comp)

			/*
				And assigns them
			*/
			comp.SubProvers.AppendToInner(roundID, ctx.prove)

			comp.InsertVerifier(roundID, ctx.Verify, ctx.GnarkVerify)
		}
	}
}

/*
Code-factorization utility struct. It holds the compilation context for a
single univariate query
*/
type naturalizationCtx struct {
	q       query.UnivariateEval
	roundID int
	/*
		Make the list of the roots and of the prefixes, the list is deduplicated.
		IE: we guarantee that a sub-query cannot reference several time the same
		poly.
	*/
	deduplicatedReprs []string
	subQueriesNames   []ifaces.QueryID
	polsPerSubQuery   [][]ifaces.Column
	reprToSubQueryID  map[string]int
}

/*
Registers the new query
*/
func (ctx *naturalizationCtx) registersTheNewQueries(comp *wizard.CompiledIOP) {

	/*
		Prevents including the same polynomial with the same repr in the
		query. Otherwise, there would be an issue.
	*/
	alreadySeen := make(map[string]struct{})
	for _, pol := range ctx.q.Pols {
		repr := column.AllDownStreamBranches(pol)
		rootsAll := column.RootParents(pol)

		for i, r := range repr {
			// Filter out (handle, repr) pairs that we already saw
			rootName := string(rootsAll[i].GetColID())
			if _, ok := alreadySeen[r+rootName]; ok {
				continue
			}

			// Initialization routine to add a new sub-query if necesary
			if _, ok := ctx.reprToSubQueryID[r]; !ok {
				queryID := len(ctx.subQueriesNames)
				ctx.reprToSubQueryID[r] = queryID
				newQueryName := deriveName[ifaces.QueryID](comp, NATURALIZE, ctx.q.QueryID, r)
				ctx.deduplicatedReprs = append(ctx.deduplicatedReprs, r)
				ctx.subQueriesNames = append(ctx.subQueriesNames, newQueryName)
				ctx.polsPerSubQuery = append(ctx.polsPerSubQuery, []ifaces.Column{})
			}

			alreadySeen[r+rootName] = struct{}{}
			queryID := ctx.reprToSubQueryID[r]
			// Add the current derived root-handle to the proper derived query
			ctx.polsPerSubQuery[queryID] = append(ctx.polsPerSubQuery[queryID], rootsAll[i])
		}
	}

	/*
		Registers the queries
	*/
	for queryID, qName := range ctx.subQueriesNames {
		comp.InsertUnivariate(ctx.roundID, qName, ctx.polsPerSubQuery[queryID])
	}

	/*
		sanity-check post-conditions
	*/
	if len(ctx.subQueriesNames) != len(ctx.polsPerSubQuery) {
		panic("mismatch in the sizes in the context")
	}

	if len(ctx.subQueriesNames) == 0 {
		panic("registered no subqueries")
	}
}

/*
Generates assignment for the new query
*/
func (ctx *naturalizationCtx) prove(run *wizard.ProverRuntime) {

	// At this time, the originalQuery query should be assigned already
	originalQuery := run.GetUnivariateParams(ctx.q.QueryID)

	if len(ctx.subQueriesNames) == 0 {
		panic("subqueries forgotten somehow forgotten\n")
	}

	/*
		List the derived X for each query. `alreadySeen` helps us account for
		the fact that `ctx.SubQueriesName` is deduplicated. We iterate in the
		same order as we did when collecting `ctx.SubQueriesName` and
		`ctx.polsPerSubQuery`. But we only collect the X of "the first
		derivation".

		newXs[i] contains the new X value for subquery #i
		newYs[i] contains then alleged evaluations
	*/

	newXs := []field.Element{}
	newYs := [][]field.Element{}

	alreadySeenPolyX := make(map[string]struct{})
	alreadySeenX := make(map[string]struct{})

	for parentID, pol := range ctx.q.Pols {
		repr := column.AllDownStreamBranches(pol)
		rootsAll := column.RootParents(pol)

		cachedXs := collection.NewMapping[string, field.Element]()
		cachedXs.InsertNew("", originalQuery.X)
		derivedXs := column.DeriveEvaluationPoint(pol, "", cachedXs, originalQuery.X)

		for rootID, r := range repr {

			// Filter out (handle, repr) pairs that we already saw
			rootName := string(rootsAll[rootID].GetColID())
			if _, ok := alreadySeenPolyX[r+rootName]; ok {
				continue
			}

			// If useful register a new query
			if _, ok := alreadySeenX[r]; !ok {
				if ctx.reprToSubQueryID[r] != len(newXs) {
					utils.Panic(
						"(while compiling %v) expected the subQueryID %v to be equal to len(newXs) but got %v",
						ctx.q.QueryID, ctx.reprToSubQueryID[r], len(newXs),
					)
				}
				newXs = append(newXs, derivedXs[rootID])
				newYs = append(newYs, []field.Element{})
				alreadySeenX[r] = struct{}{}
			}

			/*
				Two cases here

					- `pol` "contains" an interleaving in its definition. This
					is detected when the root parents size has size larger than
					1. In that case, we need to recompute the ys, all together.

					- `pol` contains no interleaving in its definition. There is
					only a single root parent and we can just retake the same y
					as from the parent query.
			*/

			subQueryID := ctx.reprToSubQueryID[r]

			if len(rootsAll) > 1 {
				rootWitness := run.GetColumn(rootsAll[rootID].GetColID())
				newY := smartvectors.Interpolate(rootWitness, derivedXs[rootID])
				newYs[subQueryID] = append(newYs[subQueryID], newY)
			} else {
				newYs[subQueryID] = append(newYs[subQueryID], originalQuery.Ys[parentID])
			}

			alreadySeenPolyX[r] = struct{}{}
		}
	}

	/*
		Assign the new univariate queries
	*/
	for queryID, qName := range ctx.subQueriesNames {
		run.AssignUnivariate(qName, newXs[queryID], newYs[queryID]...)
	}
}

func (ctx naturalizationCtx) Verify(run *wizard.VerifierRuntime) error {

	// Get the original query
	originalQuery := run.GetUnivariateEval(ctx.q.QueryID)
	originalQueryParams := run.GetUnivariateParams(ctx.q.QueryID)

	// Collect the subqueries and the collection in finalYs evaluations
	subQueries := []query.UnivariateEval{}
	subQueriesParams := []query.UnivariateEvalParams{}
	finalYs := collection.NewMapping[string, field.Element]()

	for qID, qName := range ctx.subQueriesNames {
		subQueries = append(subQueries, run.GetUnivariateEval(qName))
		subQueriesParams = append(subQueriesParams, run.GetUnivariateParams(qName))
		repr := ctx.deduplicatedReprs[qID]
		for j, derivedY := range subQueriesParams[qID].Ys {
			finalYs.InsertNew(column.DerivedYRepr(repr, subQueries[qID].Pols[j]), derivedY)
		}
	}

	// For each subqueries verifies the values for xs
	cachedXs := collection.NewMapping[string, field.Element]()
	cachedXs.InsertNew("", originalQueryParams.X)
	alreadyCheckedReprs := collection.NewSet[string]()

	/*
		Consistency check, for all poly in the original query
			- We recover the derived Xs for the derived  queries
				- This fills a cache of "cachedXs"
			- We make sure that they equal what whas alleged in the sub queries
			- We reuse the updated cache to check that the alleged evaluation Y is consistent with
				what was found in the sub queries.
	*/

	for originPolID, originH := range originalQuery.Pols {
		subReprs := column.AllDownStreamBranches(originH)
		recoveredXs := column.DeriveEvaluationPoint(originH, "", cachedXs, originalQueryParams.X)
		if len(subReprs) != len(recoveredXs) {
			panic("mismatch in size")
		}

		/*
			For each recovered X, check that it is consistent with what is
			already available in the sub queries parameters
		*/
		for i, subrepr := range subReprs {
			if alreadyCheckedReprs.Exists(subrepr) {
				continue
			}
			recoveredX := recoveredXs[i]
			qID := ctx.reprToSubQueryID[subrepr]
			submittedX := subQueriesParams[qID].X

			if recoveredX != submittedX {
				return fmt.Errorf("mismatch between the original query's evaluation point and the derived queries'")
			}
		}

		/*
			Recovers the Y values
		*/
		recoveredY := column.VerifyYConsistency(originH, "", cachedXs, finalYs)
		if recoveredY != originalQueryParams.Ys[originPolID] {
			return fmt.Errorf("mismatch between the origin query's alleged values")
		}
	}

	return nil

}

func (ctx naturalizationCtx) GnarkVerify(api frontend.API, c *wizard.WizardVerifierCircuit) {

	// Get the original query
	originalQuery := c.GetUnivariateEval(ctx.q.QueryID)
	originalQueryParams := c.GetUnivariateParams(ctx.q.QueryID)

	// Collect the subqueries and the collection in finalYs evaluations
	subQueries := []query.UnivariateEval{}
	subQueriesParams := []query.GnarkUnivariateEvalParams{}
	finalYs := collection.NewMapping[string, frontend.Variable]()

	for qID, qName := range ctx.subQueriesNames {
		subQueries = append(subQueries, c.GetUnivariateEval(qName))
		subQueriesParams = append(subQueriesParams, c.GetUnivariateParams(qName))
		repr := ctx.deduplicatedReprs[qID]
		for j, derivedY := range subQueriesParams[qID].Ys {
			finalYs.InsertNew(column.DerivedYRepr(repr, subQueries[qID].Pols[j]), derivedY)
		}
	}

	// For each subqueries verifies the values for xs
	cachedXs := collection.NewMapping[string, frontend.Variable]()
	cachedXs.InsertNew("", originalQueryParams.X)
	alreadyCheckedReprs := collection.NewSet[string]()

	/*
		Consistency check, for all poly in the original query
			- We recover the derived Xs for the derived  queries
				- This fills a cache of "cachedXs"
			- We make sure that they equal what whas alleged in the sub queries
			- We reuse the updated cache to check that the alleged evaluation Y is consistent with
				what was found in the sub queries.
	*/

	for originPolID, originH := range originalQuery.Pols {
		subReprs := column.AllDownStreamBranches(originH)
		recoveredXs := column.GnarkDeriveEvaluationPoint(api, originH, "", cachedXs, originalQueryParams.X)
		if len(subReprs) != len(recoveredXs) {
			panic("mismatch in size")
		}

		/*
			For each recovered X, check that it is consistent with what is
			already available in the sub queries parameters
		*/
		for i, subrepr := range subReprs {
			if alreadyCheckedReprs.Exists(subrepr) {
				continue
			}
			recoveredX := recoveredXs[i]
			qID := ctx.reprToSubQueryID[subrepr]
			submittedX := subQueriesParams[qID].X

			api.AssertIsEqual(recoveredX, submittedX)
		}

		/*
			Recovers the Y values
		*/
		recoveredY := column.GnarkVerifyYConsistency(api, originH, "", cachedXs, finalYs)
		api.AssertIsEqual(recoveredY, originalQueryParams.Ys[originPolID])
	}
}
