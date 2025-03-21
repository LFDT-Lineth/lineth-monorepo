package simplify

import (
	"fmt"
	"math"
	"sort"
	"sync"

	"github.com/consensys/linea-monorepo/prover/maths/field"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/sirupsen/logrus"
)

// factorizeExpression attempt to simplify the expression by identifying common
// factors within sums and factor them into a single term.
func factorizeExpression(expr *sym.Expression, iteration int) *sym.Expression {
	res := expr
	initEsh := expr.ESHash
	alreadyWalked := sync.Map{}
	factorMemo := sync.Map{}

	logrus.Infof("factoring expression : init stats %v", evaluateCostStat(expr))

	for i := 0; i < iteration; i++ {

		scoreInit := evaluateCostStat(res)

		res = res.ReconstructBottomUp(func(lincomb *sym.Expression, newChildren []*sym.Expression) *sym.Expression {
			// Time save, we reuse the results we got for that particular node.
			if ret, ok := alreadyWalked.Load(lincomb.ESHash); ok {
				return ret.(*sym.Expression)
			}

			// Incorporate the new children inside of the expression to account
			// for them.
			new := lincomb.SameWithNewChildren(newChildren)
			// To ensure that it is not accessed anymore. Note that this does
			// not mutate the input argument but makes it inaccessible to the
			// rest of the function for safety.
			lincomb = nil
			prevSize := len(new.Children)

			// The function returns only once it has figured out all the
			// factoring possibilities. There is also a bound on the loop to
			// prevent infinite loops.
			//
			// The choice of 100 is purely heuristic and is not meant to be
			// actually met.
			for k := 0; k < 100; k++ {
				_, ok := new.Operator.(sym.LinComb)
				if !ok {
					return new
				}

				group := findGdChildrenGroup(new)

				if len(group) < 1 {
					return new
				}

				// Memoize the factorLinCompFromGroup result
				cacheKey := fmt.Sprintf("%v-%v", new.ESHash, group)

				if cachedResult, ok := factorMemo.Load(cacheKey); ok {
					new = cachedResult.(*sym.Expression)

				} else {
					new = factorLinCompFromGroup(new, group)
					factorMemo.Store(cacheKey, new)
				}

				if len(new.Children) >= prevSize {
					return new
				}

				prevSize = len(new.Children)
			}

			alreadyWalked.Store(new.ESHash, new)
			return new
		})

		if res.ESHash != initEsh {
			panic("altered esh")
		}

		newScore := evaluateCostStat(res)
		logrus.Infof("finished iteration : new stats %v", newScore)

		if newScore.NumMul >= scoreInit.NumMul {
			break
		}
	}
	return res
}

// rankChildren ranks the children nodes of a list of parents based on which
// node has the highest number of parents in the list.
//
// The childrenSet is used as an exclusion set, the function shall not return
// children that are already in the children set.
func rankChildren(
	parents []*sym.Expression,
	childrenSet map[uint64]*sym.Expression,
) []*sym.Expression {

	// List all the grand-children of the expression whose parents are
	// products and counts the number of occurences by summing the exponents.
	// As an optimization the map is addressed using the first uint64 repr
	// of the element. We consider this is good enough to avoid collisions.
	// The risk if it happens is that it gets caught by the validation checks
	// at the end of the factorization routine. The preallocation value is
	// purely heuristic to avoid successive allocations.
	var relevantGdChildrenCnt map[uint64]int
	var uniqueChildrenList []*sym.Expression

	for _, p := range parents {

		prod, ok := p.Operator.(sym.Product)
		if !ok {
			continue
		}

		for i, c := range p.Children {
			// If the exponent is zero, then the term does not actually
			// contribute in the expression.
			if prod.Exponents[i] == 0 {
				continue
			}

			// If it's in the group, it does not count. We can't add it a second
			// time.
			if _, ok := childrenSet[c.ESHash[0]]; ok {
				continue
			}

			if relevantGdChildrenCnt == nil {
				relevantGdChildrenCnt = make(map[uint64]int, len(parents)+2)
				uniqueChildrenList = make([]*sym.Expression, 0, len(parents)+2)
			}

			if _, ok := relevantGdChildrenCnt[c.ESHash[0]]; !ok {
				relevantGdChildrenCnt[c.ESHash[0]] = 0
				uniqueChildrenList = append(uniqueChildrenList, c)
			}

			relevantGdChildrenCnt[c.ESHash[0]]++
		}
	}

	sort.SliceStable(uniqueChildrenList, func(i, j int) bool {
		x := uniqueChildrenList[i].ESHash
		y := uniqueChildrenList[j].ESHash
		// We want to a decreasing order
		return relevantGdChildrenCnt[x[0]] > relevantGdChildrenCnt[y[0]]
	})

	return uniqueChildrenList
}

// findGdChildrenGroup finds a large set of grandchildren including c that are
// grandchildren of expr such that they are as big as possible and share more
// than one parent. The finding is based on a greedy algorithm. We iteratively
// add nodes in the group so that the number of common parents decreases as
// slowly as possible.
func findGdChildrenGroup(expr *sym.Expression) map[uint64]*sym.Expression {

	curParents := expr.Children
	childrenSet := map[uint64]*sym.Expression{}

	ranked := rankChildren(curParents, childrenSet)

	// Can happen when we have a lincomb of lincomb. Ideally they should be
	// merged during canonization.
	if len(ranked) == 0 {
		return childrenSet
	}

	for i := range ranked {

		best := ranked[i]
		childrenSet[best.ESHash[0]] = best
		curParents = filterParentsWithChildren(curParents, best.ESHash)

		// Can't grow the set anymore
		if len(curParents) <= 1 {
			delete(childrenSet, best.ESHash[0])
			return childrenSet
		}

		logrus.Tracef(
			"find groups, so far we have %v parents and %v siblings",
			len(curParents), len(childrenSet))
	}

	return childrenSet
}

// filterParentsWithChildren returns a filtered list of parents who have at
// least one child with the given ESHash. The function allocates a new list
// of parents and returns it without mutating he original list.
func filterParentsWithChildren(
	parents []*sym.Expression,
	childEsh field.Element,
) []*sym.Expression {

	res := make([]*sym.Expression, 0, len(parents))
	for _, p := range parents {
		for _, c := range p.Children {
			if c.ESHash == childEsh {
				res = append(res, p)
				break
			}
		}
	}

	return res
}

// factorLinCompFromGroup rebuilds lincomb by factoring it using `group` to
// determine the best common factor.
func factorLinCompFromGroup(
	lincom *sym.Expression,
	group map[uint64]*sym.Expression,
) *sym.Expression {

	var (

		// numTerms indicates the number of children in the linear-combination
		numTerms = len(lincom.Children)

		lcCoeffs = lincom.Operator.(sym.LinComb).Coeffs
		// Build the common term by taking the max of the exponents
		exponentsOfGroup, groupExpr = optimRegroupExponents(lincom.Children, group)

		// Separate the non-factored terms
		nonFactoredTerms  = make([]*sym.Expression, 0, numTerms)
		nonFactoredCoeffs = make([]int, 0, numTerms)

		// The factored terms of the linear combination divided by the common
		// group factor
		factoredTerms  = make([]*sym.Expression, 0, numTerms)
		factoredCoeffs = make([]int, 0, numTerms)
	)

	numFactors := 0
	for i, p := range lincom.Children {
		factored, ok := isFactored(p, exponentsOfGroup)
		if ok {
			numFactors++
			factoredTerms = append(factoredTerms, factored)
			factoredCoeffs = append(factoredCoeffs, lcCoeffs[i])
		} else {
			nonFactoredTerms = append(nonFactoredTerms, p)
			nonFactoredCoeffs = append(nonFactoredCoeffs, lcCoeffs[i])
		}
	}

	logrus.Tracef("found %v factors for the group of size %v", numFactors, len(group))

	// Could not factor anything
	if numFactors == 0 {
		return lincom
	}

	factoredExpr := sym.NewLinComb(factoredTerms, factoredCoeffs)
	res := sym.Mul(factoredExpr, groupExpr)

	// This is a conditional because it might be that the linear combination is
	// fully factorized by the found factor.
	if len(nonFactoredTerms) > 0 {
		nonFactoredExpr := sym.NewLinComb(nonFactoredTerms, nonFactoredCoeffs)
		res = sym.Add(res, nonFactoredExpr)
	}

	return res
}

// Returns true if the product is factored by the given group. The current
// expression must be canonical.
//
// Assumption that the expression is canonical and that the exponent is
// not contained more than once. If the expression contains duplicates
// this will not be found.
//
// Fortunately, this is guaranteed if the expression was constructed via
// [sym.NewLinComb] or [sym.NewProduct] which is almost mandatory.
func isFactored(e *sym.Expression, exponentsOfGroup map[uint64]int) (
	factored *sym.Expression,
	success bool,
) {

	op, isProduct := e.Operator.(sym.Product)
	if !isProduct {
		return nil, false
	}

	exponents := op.Exponents
	factoredExponents := append([]int{}, exponents...)

	numMatches := 0
	for i, c := range e.Children {
		eig, found := exponentsOfGroup[c.ESHash[0]]
		if !found {
			continue
		}

		if eig > exponents[i] {
			return nil, false
		}

		numMatches++
		factoredExponents[i] -= eig
	}

	if numMatches != len(exponentsOfGroup) {
		return nil, false
	}

	return sym.NewProduct(e.Children, factoredExponents), true
}

// optimRegroupExponents returns an expression maximizing the exponents of an
// other expression. Panics if one of the parent is not a product or does not
// have the whole group as children.
func optimRegroupExponents(
	parents []*sym.Expression,
	group map[uint64]*sym.Expression,
) (
	exponentMap map[uint64]int,
	groupedTerm *sym.Expression,
) {

	exponentMap = make(map[uint64]int, 16)
	canonTermList := make([]*sym.Expression, 0, 16) // built in deterministic order

	for _, p := range parents {

		op, isProd := p.Operator.(sym.Product)
		if !isProd {
			continue
		}
		exponents := op.Exponents

		// Used to sanity-check that all the nodes of the group have been
		// reached through this parent.
		matched := make(map[uint64]int, len(p.Children))

		for i, c := range p.Children {
			if _, ingroup := group[c.ESHash[0]]; !ingroup {
				continue
			}

			if exponents[i] == 0 {
				panic("The expression is not canonic")
			}

			_, initialized := exponentMap[c.ESHash[0]]
			if !initialized {
				// Max int is used as a placeholder. It will be replaced anytime
				// we wall utils.Min(exponentMap[h], n) where n is actually an
				// exponent.
				exponentMap[c.ESHash[0]] = math.MaxInt
				canonTermList = append(canonTermList, c)
			}

			matched[c.ESHash[0]] = exponents[i]
		}

		if len(matched) != len(group) {
			continue
		}

		for esh, ex := range matched {
			// Recall that the values of the exponent maps are initialized to
			// MaxInt. So this will always pass ex the first time this loc is
			// reached for esh.
			exponentMap[esh] = utils.Min(ex, exponentMap[esh])
		}
	}

	canonExponents := []int{}
	for _, e := range canonTermList {
		canonExponents = append(canonExponents, exponentMap[e.ESHash[0]])
	}

	return exponentMap, sym.NewProduct(canonTermList, canonExponents)
}
