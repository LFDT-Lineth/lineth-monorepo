package symbolic

import (
	"fmt"
	"slices"
	"sort"

	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/collection"
	"github.com/consensys/linea-monorepo/prover/utils/parallel"
	"github.com/sirupsen/logrus"
)

// EliminatedVarMetadata implements Metadata for variables created during
// degree reduction. Each instance represents a substituted subexpression.
type EliminatedVarMetadata struct {
	id   int
	expr *Expression
}

// String returns a unique identifier for the eliminated variable.
func (e EliminatedVarMetadata) String() string {
	return fmt.Sprintf("_elim_%d", e.id)
}

// IsBase returns true since eliminated variables are treated as base field elements.
func (e EliminatedVarMetadata) IsBase() bool {
	return e.expr.IsBase
}

// ID returns the index of the eliminated variable.
func (e EliminatedVarMetadata) ID() int {
	return e.id
}

// weightedSubMultisetIteratorConfig is a configuration for the
// weightedSubMultisetIterator. They allow for speeding up the iterator when
// we start from a large set to only look at a subset of it.
type DegreeReductionConfig struct {
	// MinWeight controls minimal weight a position should have to be accounted
	// for in the multiset iteration. Anything below, will be disregarded.
	MinWeightForTerm int
	// NLast controls the number of positions the iterator looks at. This goes
	// after eliminating positions with min-weight. Thus, the iterator will
	// always look at either NLast positions whose weight is >= MinWeight, or
	// all positions whose weight is >= MinWeight.
	NLast int
	// MinDegree is the minimal degree that a subexpression should have to be
	// candidate for elimination.
	MinDegreeForCandidate int
	// MaxCandidatePerStep lists the maximal number of candidate that can be
	// collected per step.
	MaxCandidatePerRound int
	// MaxNumElimination controls the maximum number of eliminated subexpressions.
	MaxNumElimination int
}

// ReduceDegreeOfExpressions performs a degree reduction algorithm by iteratively
// eliminating the most frequent common subexpressions whose degree is within
// the bound, from expressions exceeding that bound. The algorithm replaces
// each eliminated subexpression with a fresh variable and continues until
// all expressions satisfy the degree constraint.
//
// If the provided bound is one, the function will panic (as it would not be
// possible to degree-reduce it).
func ReduceDegreeOfExpressions(
	exprs []*Expression,
	degreeBound int,
	degreeGetter GetDegree,
	iteratorConfig DegreeReductionConfig,
) (degreeReduced []*Expression, eliminatedSubExpressions []*Expression, newVars []Metadata) {

	AssertHasNoElimVarMetadata(exprs)

	if degreeBound <= 1 {
		utils.Panic("cannot degree reduce with bound of %v", degreeBound)
	}

	// Working copies that get progressively transformed
	current := slices.Clone(exprs)

	for {

		if iteratorConfig.MaxNumElimination > 0 && len(newVars) >= iteratorConfig.MaxNumElimination {
			logrus.Infof("reached max number of eliminated subexpressions %v", iteratorConfig.MaxNumElimination)
			break
		}

		// Identify expressions that still exceed the degree bound
		overDegreeIndices := findOverDegreeIndices(current, degreeBound, degreeGetter)
		if len(overDegreeIndices) == 0 {
			break
		}

		// Collect all candidate subexpressions from over-degree expressions
		candidates := collectCandidateSubexprs(current, overDegreeIndices, degreeBound, degreeGetter, iteratorConfig)

		if len(candidates) == 0 {
			// No valid candidates found; cannot reduce further
			logrus.Infof("no more candidates found, degree bound is %v, nb-over-degree-indices %v", degreeBound, len(overDegreeIndices))
			break
		}

		// Select the most frequent candidate
		best := selectMostFrequentCandidate(candidates)

		// Create a new variable to replace this subexpression
		meta := EliminatedVarMetadata{id: len(newVars), expr: best}
		newVar := NewVariable(meta)

		// Substitute the candidate in all expressions
		current = substituteInAll(current, best, newVar)

		eliminatedSubExpressions = append(eliminatedSubExpressions, best)
		newVars = append(newVars, meta)

		if len(newVars)%100 == 0 {
			logrus.
				WithField("varCounter", len(newVars)).
				WithField("where", "ReduceDegreeOfExpressions").
				WithField("over-degree-bound", len(overDegreeIndices)).
				WithField("candidate-found", len(candidates)).
				Infof("successfully eliminated one expression")
		}
	}

	logrus.
		WithField("varCounter", len(newVars)).
		WithField("where", "ReduceDegreeOfExpressions").
		Infof("done reducing the degree of the expression")

	degreeReduced = current
	return
}

// AssertHasNoElimVarMetadata panics when one of exprs has a [ElimVarMetadata]
// as input. This is useful to sanity-check
func AssertHasNoElimVarMetadata(exprs []*Expression) {
	for _, expr := range exprs {
		board := expr.Board()
		metadatas := board.ListVariableMetadata()
		for _, metadata := range metadatas {
			if _, isElim := metadata.(EliminatedVarMetadata); isElim {
				panic("found an non-replaced ElimVarMetadata")
			}
		}
	}
}

// findOverDegreeIndices returns indices of expressions exceeding the degree bound.
func findOverDegreeIndices(exprs []*Expression, bound int, degreeGetter GetDegree) []int {
	var indices []int
	for i, expr := range exprs {
		deg := expr.degreeKeepCache(degreeGetter)
		if deg > bound {
			indices = append(indices, i)
		}
	}
	return indices
}

// candidateInfo tracks a candidate subexpression and its occurrence count.
type candidateInfo struct {
	expr  *Expression
	count int
}

// candidateMapType is a sugar-coat over collection.DeterministicMap to avoid
// using such a long name in all the code.
type candidateMapType = *collection.DeterministicMap[esHash, candidateInfo]

// collectCandidateSubexprs gathers all subexpressions from over-degree expressions
// that have degree within the bound. Includes both direct subexpressions and
// sub-products derived from Product nodes.
func collectCandidateSubexprs(
	exprs []*Expression,
	overDegreeIndices []int,
	degreeBound int,
	degreeGetter GetDegree,
	iteratorConfig DegreeReductionConfig,
) []candidateInfo {

	// Map from ESHash to candidate info for deduplication. The extra 1<<10
	// margin is to add some margin as [iteratorConfig.MaxCandidatePerRound]
	// is only loosely enforced.
	candidateMap := collection.MakeDeterministicMap[esHash, candidateInfo](
		iteratorConfig.MaxCandidatePerRound + 1<<10)

	for _, idx := range overDegreeIndices {

		nbCandidateAdded := collectFromExpr(exprs[idx], degreeBound, degreeGetter, candidateMap, iteratorConfig)

		if nbCandidateAdded == 0 {
			childrenDegree := make([]int, len(exprs[idx].Children))
			for i, child := range exprs[idx].Children {
				childrenDegree[i] = child.degreeKeepCache(degreeGetter)
			}
			logrus.
				WithField("childrenDegree", childrenDegree).
				Errorf("did not add a single candidate for %++v", exprs[idx].MarshalJSONString())
		}

		if candidateMap.Len() >= iteratorConfig.MaxCandidatePerRound {
			break
		}
	}

	return candidateMap.ValueSlice()
}

// collectFromExpr recursively collects candidate subexpressions from an expression.
func collectFromExpr(
	expr *Expression,
	degreeBound int,
	degreeGetter GetDegree,
	candidates candidateMapType,
	iteratorConfig DegreeReductionConfig,
) (nbCandidateAdded int) {
	if expr == nil {
		return
	}

	degree := expr.degreeKeepCache(degreeGetter)

	if degree < iteratorConfig.MinDegreeForCandidate {
		return
	}

	// If this expression is within bound and non-trivial, it's a candidate
	if degree > 0 && degree <= degreeBound && !isTrivialExpr(expr) {
		addCandidate(candidates, expr)
		nbCandidateAdded++
	}

	// Recurse into children
	for _, child := range expr.Children {
		nbCandidateAdded += collectFromExpr(child, degreeBound, degreeGetter, candidates, iteratorConfig)
	}

	// For Product nodes, enumerate sub-products as additional candidates
	if prod, ok := expr.Operator.(Product); ok {
		nbCandidateAdded += collectSubProducts(expr, prod, degreeBound, degreeGetter, iteratorConfig, candidates)
	}

	return nbCandidateAdded
}

// isTrivialExpr returns true for expressions that should not be substituted
// (constants, simple variables, or degree-zero expressions).
func isTrivialExpr(expr *Expression) bool {
	switch expr.Operator.(type) {
	case Constant, Variable:
		return true
	}
	return false
}

// addCandidate increments the count for a candidate or creates a new entry.
func addCandidate(candidates candidateMapType, expr *Expression) {
	if existing, ok := candidates.GetPtr(expr.ESHash); ok {
		existing.count++
	} else {
		candidates.InsertNew(expr.ESHash, candidateInfo{expr: expr, count: 1})
	}
}

// collectSubProducts enumerates all non-trivial sub-multisets of a Product
// expression and adds those within the degree bound as candidates.
func collectSubProducts(
	expr *Expression,
	prod Product,
	degreeBound int,
	degreeGetter GetDegree,
	iteratorConfig DegreeReductionConfig,
	candidates candidateMapType,
) (nbCandidateAdded int) {
	if len(expr.Children) == 0 {
		return
	}

	// We use the degree of the terms as weights for weighted enumeration
	// of sub-multisets.
	weights := make([]int, len(prod.Exponents))
	for i := range prod.Exponents {
		weights[i] = expr.Children[i].degreeKeepCache(degreeGetter)
	}

	iter := newWeightedSubMultisetIterator(prod.Exponents, weights, degreeBound, iteratorConfig)
	for iter.next() {

		// Quick filter: skip if the sub-multiset is just one element
		if iter.hasOnlyASingleOne() {
			continue
		}

		degree := iter.currentTotalWeight()
		if degree < 0 || degree > degreeBound {
			continue
		}

		subExpr := buildSubProductFromExponents(expr.Children, iter.currentSnapshot())
		if subExpr == nil {
			continue
		}

		addCandidate(candidates, subExpr)
		nbCandidateAdded++
	}

	return nbCandidateAdded
}

// buildSubProductFromExponents constructs a Product from children with given exponents.
// Factors with zero exponent are excluded from the result.
func buildSubProductFromExponents(children []*Expression, exponents []int) *Expression {
	var subChildren []*Expression
	var subExponents []int

	for i, exp := range exponents {
		if exp > 0 {
			subChildren = append(subChildren, children[i])
			subExponents = append(subExponents, exp)
		}
	}

	if len(subChildren) == 0 {
		return nil
	}

	return NewProduct(subChildren, subExponents)
}

// factorOutSubProduct attempts to factor out target's exponents from expr's exponents.
// Returns the remaining factors if target is a sub-multiset of expr.
func factorOutSubProduct(
	exprChildren []*Expression,
	exprExponents []int,
	targetChildren []*Expression,
	targetExponents []int,
) (remaining []factorPair, found bool) {

	// Build map from ESHash to (index, remaining exponent) for expr
	type exponentEntry struct {
		index    int
		exponent int
	}

	exprMap := make(map[esHash]exponentEntry, len(exprChildren))
	for i, child := range exprChildren {
		exprMap[child.ESHash] = exponentEntry{index: i, exponent: exprExponents[i]}
	}

	// Subtract target exponents from expr exponents
	for i, child := range targetChildren {
		entry, exists := exprMap[child.ESHash]
		if !exists || entry.exponent < targetExponents[i] {
			return nil, false
		}
		entry.exponent -= targetExponents[i]
		exprMap[child.ESHash] = entry
	}

	// Collect remaining factors with positive exponents
	for _, child := range exprChildren {
		entry := exprMap[child.ESHash]
		if entry.exponent > 0 {
			remaining = append(remaining, factorPair{expr: child, exp: entry.exponent})
			// Mark as collected
			entry.exponent = 0
			exprMap[child.ESHash] = entry
		}
	}

	return remaining, true
}

// computeSubsetExponentSum calculates the sum of exponents for selected factors.
func computeSubsetExponentSum(mask uint64, exponents []int) int {
	sum := 0
	for i, exp := range exponents {
		if mask&(1<<i) != 0 {
			sum += exp
		}
	}
	return sum
}

// buildSubProduct constructs a Product expression from a subset of factors.
func buildSubProduct(children []*Expression, exponents []int, mask uint64) *Expression {
	var subChildren []*Expression
	var subExponents []int

	for i := range children {
		if mask&(1<<i) != 0 {
			subChildren = append(subChildren, children[i])
			subExponents = append(subExponents, exponents[i])
		}
	}

	if len(subChildren) == 0 {
		return nil
	}

	return NewProduct(subChildren, subExponents)
}

// selectMostFrequentCandidate returns the candidate with the highest occurrence count.
// Ties are broken by selecting the highest-degree candidate (more reduction impact).
func selectMostFrequentCandidate(candidates []candidateInfo) *Expression {
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].count != candidates[j].count {
			return candidates[i].count > candidates[j].count
		}
		// Tie-breaker: prefer higher degree for more impact
		return len(candidates[i].expr.Children) > len(candidates[j].expr.Children)
	})
	return candidates[0].expr
}

// substituteInAll replaces all occurrences of target with replacement in each expression.
func substituteInAll(exprs []*Expression, target, replacement *Expression) []*Expression {
	result := make([]*Expression, len(exprs))
	parallel.Execute(len(exprs), func(start, stop int) {
		for i := start; i < stop; i++ {
			expr := exprs[i]
			result[i] = substituteExpr(expr, target, replacement)
		}
	})
	return result
}

// substituteExpr recursively replaces occurrences of target with replacement.
// Returns a new expression tree with substitutions applied.
func substituteExpr(expr, target, replacement *Expression) *Expression {
	if expr == nil {
		return nil
	}

	// Check if current expression matches the target
	if expr.ESHash == target.ESHash {
		return replacement
	}

	// For Product nodes, check if target is a sub-product
	if prod, ok := expr.Operator.(Product); ok {
		if substituted := substituteSubProduct(expr, prod, target, replacement); substituted != nil {
			return substituted
		}
	}

	// Recurse into children
	if len(expr.Children) == 0 {
		return expr
	}

	newChildren := substituteInChildren(expr.Children, target, replacement)
	if childrenUnchanged(expr.Children, newChildren) {
		return expr
	}

	return rebuildExpression(expr, newChildren)
}

// substituteInChildren applies substitution to each child expression.
func substituteInChildren(children []*Expression, target, replacement *Expression) []*Expression {
	newChildren := make([]*Expression, len(children))
	for i, child := range children {
		newChildren[i] = substituteExpr(child, target, replacement)
	}
	return newChildren
}

// childrenUnchanged returns true if no child was modified during substitution.
func childrenUnchanged(old, new []*Expression) bool {
	for i := range old {
		if old[i] != new[i] {
			return false
		}
	}
	return true
}

// rebuildExpression constructs a new expression with the same operator but new children.
func rebuildExpression(original *Expression, newChildren []*Expression) *Expression {
	var new *Expression
	switch op := original.Operator.(type) {
	case LinComb:
		new = NewLinComb(newChildren, op.Coeffs)
	case Product:
		new = NewProduct(newChildren, op.Exponents)
	case PolyEval:
		new = NewPolyEval(newChildren[0], newChildren[1:])
	default:
		// For other operators, return original (should not happen for valid expressions)
		original.cachedDegree = nil
		return original
	}
	// No need to uncache: newly created expressions have cachedDegree=nil.
	return new
}

// substituteSubProduct checks if target is a sub-product of expr and performs
// the substitution by factoring out the target. Returns nil if not applicable.
func substituteSubProduct(
	expr *Expression,
	prod Product,
	target, replacement *Expression,
) *Expression {

	targetProd, ok := target.Operator.(Product)
	if !ok {
		return nil
	}

	// Check if target's factors are a subset of expr's factors
	remaining, found := factorOutSubProduct(expr.Children, prod.Exponents, target.Children, targetProd.Exponents)
	if !found {
		return nil
	}

	// Build the result: replacement * remaining factors
	return buildFactoredProduct(replacement, remaining)
}

// factorPair represents a factor and its exponent in a product.
type factorPair struct {
	expr *Expression
	exp  int
}

// buildFactoredProduct constructs a product of replacement and remaining factors.
func buildFactoredProduct(replacement *Expression, remaining []factorPair) *Expression {
	if len(remaining) == 0 {
		return replacement
	}

	children := []*Expression{replacement}
	exponents := []int{1}

	for _, f := range remaining {
		children = append(children, f.expr)
		exponents = append(exponents, f.exp)
	}

	// No need to uncache: NewProduct returns a fresh expression with cachedDegree=nil.
	return NewProduct(children, exponents)
}

// weightedSubMultisetIterator provides iteration over all non-trivial sub-multisets
// where the total weighted sum is <= maxWeight.
// Uses backtracking with pruning to avoid generating invalid multisets.
type weightedSubMultisetIterator struct {
	curr      []int
	maxWeight int
	// This tells the iterator to disregard positions whose weight is below
	// some threshold. This is useful to save time by pruning the iterator.
	maxExponents []int
	weights      []int
	exhausted    bool
	config       DegreeReductionConfig
}

// newWeightedSubMultisetIterator creates an iterator over sub-multisets
// with total weight <= maxWeight.
// If weights is nil, each element has weight 1 (equivalent to degree constraint).
// Excludes empty multiset and full multiset.
func newWeightedSubMultisetIterator(exponents []int, weights []int, maxWeight int, cfg DegreeReductionConfig) *weightedSubMultisetIterator {
	current := make([]int, len(exponents))

	// Default weights to 1 if not provided
	if weights == nil {
		weights = make([]int, len(exponents))
		for i := range weights {
			weights[i] = 1
		}
	}

	return &weightedSubMultisetIterator{
		maxExponents: exponents,
		config:       cfg,
		weights:      weights,
		curr:         current,
		maxWeight:    maxWeight,
		exhausted:    false,
	}
}

func (w *weightedSubMultisetIterator) next() bool {

	if w.exhausted {
		return false
	}

	var (
		totalWeight            = w.currentTotalWeight()
		cntNonIgnoredPositions = 0
	)

	for idx := len(w.curr) - 1; idx >= 0; idx-- {

		if w.weights[idx] < w.config.MinWeightForTerm {
			continue
		}

		cntNonIgnoredPositions++
		if w.config.NLast > 0 && cntNonIgnoredPositions > w.config.NLast {
			break
		}

		if w.curr[idx] < w.maxExponents[idx] && totalWeight+w.weights[idx] <= w.maxWeight {
			w.curr[idx]++

			if totalWeight+w.weights[idx] < w.config.MinDegreeForCandidate {
				return w.next()
			}

			if w.isFull() || w.isEmpty() {
				continue
			}

			return true
		}

		totalWeight -= w.curr[idx] * w.weights[idx]
		w.curr[idx] = 0
	}

	w.exhausted = true
	return false
}

// isEmpty returns true if current multiset has all zeros.
func (it *weightedSubMultisetIterator) isEmpty() bool {
	for _, e := range it.curr {
		if e != 0 {
			return false
		}
	}
	return true
}

// isFull returns true if current equals maxExponents.
func (it *weightedSubMultisetIterator) isFull() bool {
	for i, e := range it.curr {
		if e != it.maxExponents[i] {
			return false
		}
	}
	return true
}

// currentTotalWeight returns the weighted sum of current exponents.
func (it *weightedSubMultisetIterator) currentTotalWeight() int {
	sum := 0
	for i, e := range it.curr {
		sum += e * it.weights[i]
	}
	return sum
}

// HasOnlyASingleOne returns true if the current multiset has only a single 1 and
// the rest is zero.
func (it *weightedSubMultisetIterator) hasOnlyASingleOne() bool {
	sum := 0
	countNonZero := 0
	for _, e := range it.curr {
		if e != 0 {
			countNonZero++
			sum += e
		}
	}
	return countNonZero == 1 && sum == 1
}

// currentSnapshot returns a copy of the current exponents.
func (it *weightedSubMultisetIterator) currentSnapshot() []int {
	return slices.Clone(it.curr)
}
