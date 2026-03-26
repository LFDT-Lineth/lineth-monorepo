package univariates

import (
	"fmt"
	"reflect"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/collection"
	"github.com/sirupsen/logrus"
)

const (
	NATURALIZE string = "NATURALIZE"
)

type NaturalizeProverAction struct {
	Ctx NaturalizationCtx
}

func (a *NaturalizeProverAction) Run(run *wizard.ProverRuntime) {
	a.Ctx.prove(run)
}

type NaturalizeVerifierAction struct {
	Ctx NaturalizationCtx
}

func (a *NaturalizeVerifierAction) Run(run wizard.Runtime) error {
	return a.Ctx.Verify(run)
}

func (a *NaturalizeVerifierAction) RunGnark(api frontend.API, c wizard.GnarkRuntime) {
	a.Ctx.GnarkVerify(api, c)
}

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
			ctx := NaturalizationCtx{
				Q:                q,
				RoundID:          roundID,
				SubQueriesNames:  []ifaces.QueryID{},
				PolsPerSubQuery:  [][]ifaces.Column{},
				ReprToSubQueryID: make(map[string]int),
			}

			ctx.registersTheNewQueries(comp)

			/*
				And assigns them
			*/
			// comp.SubProvers.AppendToInner(roundID, ctx.prove)
			comp.RegisterProverAction(roundID, &NaturalizeProverAction{
				Ctx: ctx,
			})

			// comp.InsertVerifier(roundID, ctx.Verify, ctx.GnarkVerify)
			comp.RegisterVerifierAction(roundID, &NaturalizeVerifierAction{
				Ctx: ctx,
			})
		}
	}
}

/*
Code-factorization utility struct. It holds the compilation context for a
single univariate query
*/
type NaturalizationCtx struct {
	Q       query.UnivariateEval
	RoundID int
	/*
		Make the list of the roots and of the prefixes, the list is deduplicated.
		IE: we guarantee that a sub-query cannot reference several time the same
		poly.
	*/
	DeduplicatedReprs []string
	SubQueriesNames   []ifaces.QueryID
	PolsPerSubQuery   [][]ifaces.Column
	ReprToSubQueryID  map[string]int
}

/*
Registers the new query
*/
func (ctx *NaturalizationCtx) registersTheNewQueries(comp *wizard.CompiledIOP) {

	/*
		Prevents including the same polynomial with the same repr in the
		query. Otherwise, there would be an issue.
	*/
	alreadySeen := make(map[string]struct{})
	for _, pol := range ctx.Q.Pols {

		repr := column.DownStreamBranch(pol)
		root := column.RootParents(pol)
		rootName := string(root.GetColID())

		if _, ok := alreadySeen[repr+rootName]; ok {
			continue
		}

		// Initialization routine to add a new sub-query if necesary
		if _, ok := ctx.ReprToSubQueryID[repr]; !ok {
			queryID := len(ctx.SubQueriesNames)
			ctx.ReprToSubQueryID[repr] = queryID
			newQueryName := deriveName[ifaces.QueryID](comp, NATURALIZE, ctx.Q.QueryID, repr)
			ctx.DeduplicatedReprs = append(ctx.DeduplicatedReprs, repr)
			ctx.SubQueriesNames = append(ctx.SubQueriesNames, newQueryName)
			ctx.PolsPerSubQuery = append(ctx.PolsPerSubQuery, []ifaces.Column{})
		}

		alreadySeen[repr+rootName] = struct{}{}
		queryID := ctx.ReprToSubQueryID[repr]
		// Add the current derived root-handle to the proper derived query
		ctx.PolsPerSubQuery[queryID] = append(ctx.PolsPerSubQuery[queryID], root)
	}

	/*
		Registers the queries
	*/
	for queryID, qName := range ctx.SubQueriesNames {
		comp.InsertUnivariate(ctx.RoundID, qName, ctx.PolsPerSubQuery[queryID])
		// The result of the query is ditched from the FS state because in
		// all scenarios. The result of the opening is (and is checked to be)
		// identical to the result of the original univariate query. Hence, it
		// does not contain informations that the verifier does not already have.
		comp.QueriesParams.MarkAsSkippedFromProverTranscript(qName)
	}

	/*
		sanity-check post-conditions
	*/
	if len(ctx.SubQueriesNames) != len(ctx.PolsPerSubQuery) {
		panic("mismatch in the sizes in the context")
	}

	if len(ctx.SubQueriesNames) == 0 {
		panic("registered no subqueries")
	}
}

/*
Generates assignment for the new query
*/
func (ctx *NaturalizationCtx) prove(run *wizard.ProverRuntime) {

	// At this time, the originalQuery query should be assigned already
	originalQuery := run.GetUnivariateParams(ctx.Q.QueryID)

	if len(ctx.SubQueriesNames) == 0 {
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

	for parentID, pol := range ctx.Q.Pols {
		repr := column.DownStreamBranch(pol)
		rootsAll := column.RootParents(pol)

		// Filter out (handle, repr) pairs that we already saw
		rootName := string(rootsAll.GetColID())
		if _, ok := alreadySeenPolyX[repr+rootName]; ok {
			continue
		}

		cachedXs := collection.NewMapping[string, field.Element]()
		cachedXs.InsertNew("", originalQuery.X)
		derivedXs := column.DeriveEvaluationPoint(pol, "", cachedXs, originalQuery.X)

		// If useful register a new query
		if _, ok := alreadySeenX[repr]; !ok {
			if ctx.ReprToSubQueryID[repr] != len(newXs) {
				utils.Panic(
					"(while compiling %v) expected the subQueryID %v to be equal to len(newXs) but got %v",
					ctx.Q.QueryID, ctx.ReprToSubQueryID[repr], len(newXs),
				)
			}
			newXs = append(newXs, derivedXs)
			newYs = append(newYs, []field.Element{})
			alreadySeenX[repr] = struct{}{}
		}

		/*
			Two cases here

				- `pol` "contains" an interleaving in i  ts definition. This
				is detected when the root parents size has size larger than
				1. In that case, we need to recompute the ys, all together.

				- `pol` contains no interleaving in its definition. There is
				only a single root parent and we can just retake the same y
				as from the parent query.
		*/

		subQueryID := ctx.ReprToSubQueryID[repr]
		newYs[subQueryID] = append(newYs[subQueryID], originalQuery.Ys[parentID])
		alreadySeenPolyX[repr] = struct{}{}
	}

	/*
		Assign the new univariate queries
	*/
	for queryID, qName := range ctx.SubQueriesNames {
		run.AssignUnivariate(qName, newXs[queryID], newYs[queryID]...)
	}
}

func (ctx NaturalizationCtx) Verify(run wizard.Runtime) error {

	// Get the original query
	originalQuery := run.GetUnivariateEval(ctx.Q.QueryID)
	originalQueryParams := run.GetUnivariateParams(ctx.Q.QueryID)

	// Collect the subqueries and the collection in finalYs evaluations
	subQueries := []query.UnivariateEval{}
	subQueriesParams := []query.UnivariateEvalParams{}
	finalYs := collection.NewMapping[string, field.Element]()

	for qID, qName := range ctx.SubQueriesNames {
		subQueries = append(subQueries, run.GetUnivariateEval(qName))
		subQueriesParams = append(subQueriesParams, run.GetUnivariateParams(qName))
		repr := ctx.DeduplicatedReprs[qID]
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
		subrepr := column.DownStreamBranch(originH)
		recoveredX := column.DeriveEvaluationPoint(originH, "", cachedXs, originalQueryParams.X)

		if alreadyCheckedReprs.Exists(subrepr) {
			continue
		}

		qID := ctx.ReprToSubQueryID[subrepr]
		submittedX := subQueriesParams[qID].X

		if recoveredX != submittedX {
			return fmt.Errorf("mismatch between the original query's evaluation point and the derived queries'")
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

func (ctx NaturalizationCtx) GnarkVerify(api frontend.API, c wizard.GnarkRuntime) {

	// Get the original query
	originalQuery := c.GetUnivariateEval(ctx.Q.QueryID)
	originalQueryParams := c.GetUnivariateParams(ctx.Q.QueryID)

	// Collect the subqueries and the collection in finalYs evaluations
	subQueries := []query.UnivariateEval{}
	subQueriesParams := []query.GnarkUnivariateEvalParams{}
	finalYs := collection.NewMapping[string, frontend.Variable]()

	for qID, qName := range ctx.SubQueriesNames {
		subQueries = append(subQueries, c.GetUnivariateEval(qName))
		subQueriesParams = append(subQueriesParams, c.GetUnivariateParams(qName))
		repr := ctx.DeduplicatedReprs[qID]
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
		subrepr := column.DownStreamBranch(originH)
		recoveredX := column.GnarkDeriveEvaluationPoint(api, originH, "", cachedXs, originalQueryParams.X)

		if alreadyCheckedReprs.Exists(subrepr) {
			continue
		}

		qID := ctx.ReprToSubQueryID[subrepr]
		submittedX := subQueriesParams[qID].X
		// Or it is a mismatch between the evaluation queries and the derived query
		api.AssertIsEqual(recoveredX[0], submittedX)

		/*
			Recovers the Y values
		*/
		recoveredY := column.GnarkVerifyYConsistency(api, originH, "", cachedXs, finalYs)
		api.AssertIsEqual(recoveredY, originalQueryParams.Ys[originPolID])
	}
}
