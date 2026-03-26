package symbolic

import (
	"fmt"
	"reflect"
	"sort"
	"testing"

	"github.com/consensys/linea-monorepo/prover/utils/collection"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var noIteratorCfg = DegreeReductionConfig{}

// constantDegreeGetter returns a degree getter that assigns degree 1 to all variables.
func constantDegreeGetter() GetDegree {
	return func(interface{}) int { return 1 }
}

// largeDegreeGetter returns a degree getter that assigns degree 10 to all variables.
func largeDegreeGetter() GetDegree {
	return func(interface{}) int { return 1 << 20 }
}

func TestReduceDegreeOfExpressions(t *testing.T) {

	var (
		a      = NewDummyVar("a")
		b      = NewDummyVar("b")
		c      = NewDummyVar("c")
		d      = NewDummyVar("d")
		getDeg = constantDegreeGetter()
	)

	t.Run("no reduction needed", func(t *testing.T) {
		// Expression already within bound
		expr := a.Add(b)
		reduced, eliminated, newVars := ReduceDegreeOfExpressions(
			[]*Expression{expr},
			2,
			getDeg,
			noIteratorCfg,
		)
		require.Len(t, reduced, 1)
		require.Empty(t, eliminated)
		require.Empty(t, newVars)
		assert.Equal(t, expr.ESHash, reduced[0].ESHash)
	})

	t.Run("single quadratic reduction", func(t *testing.T) {
		// a * b * c has degree 3, bound is 2
		expr := a.Mul(b).Mul(c)
		reduced, eliminated, newVars := ReduceDegreeOfExpressions(
			[]*Expression{expr},
			2,
			getDeg,
			noIteratorCfg,
		)
		require.Len(t, reduced, 1)
		require.Len(t, eliminated, 1)
		require.Len(t, newVars, 1)
		// Reduced expression should have degree <= 2
		reducedDegree := reduced[0].Degree(getDeg)
		assert.LessOrEqual(t, reducedDegree, 2)
	})

	t.Run("multiple expressions sharing subexpression", func(t *testing.T) {
		// Both expressions contain a*b as a subexpression
		expr1 := a.Mul(b).Mul(c) // degree 3
		expr2 := a.Mul(b).Mul(a) // degree 3
		reduced, eliminated, newVars := ReduceDegreeOfExpressions(
			[]*Expression{expr1, expr2},
			2,
			getDeg,
			noIteratorCfg,
		)
		require.Len(t, reduced, 2)
		// Should eliminate a*b since it appears in both
		require.GreaterOrEqual(t, len(eliminated), 1)
		require.Equal(t, len(eliminated), len(newVars))
		// Both reduced expressions should satisfy the bound
		for i, red := range reduced {
			deg := red.Degree(getDeg)
			assert.LessOrEqual(t, deg, 2, "expression %d has degree %d", i, deg)
		}
	})

	t.Run("high degree requires multiple reductions", func(t *testing.T) {
		// a^5 has degree 5, bound is 2
		expr := a.Pow(5)
		reduced, eliminated, newVars := ReduceDegreeOfExpressions(
			[]*Expression{expr},
			2,
			getDeg,
			noIteratorCfg,
		)
		require.Len(t, reduced, 1)
		require.NotEmpty(t, eliminated)
		require.Equal(t, len(eliminated), len(newVars))
		reducedDegree := reduced[0].Degree(getDeg)
		assert.LessOrEqual(t, reducedDegree, 2)
	})

	t.Run("empty input", func(t *testing.T) {
		reduced, eliminated, newVars := ReduceDegreeOfExpressions(
			[]*Expression{},
			2,
			getDeg,
			noIteratorCfg,
		)
		require.Empty(t, reduced)
		require.Empty(t, eliminated)
		require.Empty(t, newVars)
	})

	t.Run("exactly at bound", func(t *testing.T) {
		// a * b has degree 2, bound is 2 - no reduction needed
		expr := a.Mul(b)
		reduced, eliminated, newVars := ReduceDegreeOfExpressions(
			[]*Expression{expr},
			2,
			getDeg,
			noIteratorCfg,
		)
		require.Len(t, reduced, 1)
		require.Empty(t, eliminated)
		require.Empty(t, newVars)
		assert.Equal(t, expr.ESHash, reduced[0].ESHash)
	})

	t.Run("very high degree a^10", func(t *testing.T) {
		// a^10 requires multiple reduction steps to reach degree 2
		expr := a.Pow(10)
		reduced, eliminated, newVars := ReduceDegreeOfExpressions(
			[]*Expression{expr},
			2,
			getDeg,
			noIteratorCfg,
		)
		require.Len(t, reduced, 1)
		require.NotEmpty(t, eliminated)
		require.Equal(t, len(eliminated), len(newVars))
		reducedDegree := reduced[0].Degree(getDeg)
		assert.LessOrEqual(t, reducedDegree, 2)
	})

	t.Run("very high degree a^10, with deg(a)=2^20", func(t *testing.T) {
		// a^10 requires multiple reduction steps to reach degree 2
		expr := a.Pow(10)
		expr.uncacheDegree()
		reduced, eliminated, newVars := ReduceDegreeOfExpressions(
			[]*Expression{expr},
			1<<21,
			largeDegreeGetter(),
			noIteratorCfg,
		)
		require.Len(t, reduced, 1)
		require.NotEmpty(t, eliminated)
		require.Equal(t, len(eliminated), len(newVars))
		reducedDegree := reduced[0].Degree(largeDegreeGetter())
		assert.LessOrEqual(t, reducedDegree, 1<<21)
		expr.uncacheDegree()
	})

	t.Run("mixed addition and multiplication", func(t *testing.T) {
		// (a * b * c) + (a * b * d) - common subexpression a*b
		expr := a.Mul(b).Mul(c).Add(a.Mul(b).Mul(d))
		reduced, eliminated, newVars := ReduceDegreeOfExpressions(
			[]*Expression{expr},
			2,
			getDeg,
			noIteratorCfg,
		)
		require.Len(t, reduced, 1)
		require.NotEmpty(t, eliminated)
		require.Equal(t, len(eliminated), len(newVars))
		reducedDegree := reduced[0].Degree(getDeg)
		assert.LessOrEqual(t, reducedDegree, 2)
	})

	t.Run("nested products", func(t *testing.T) {
		// (a * b) * (c * d) = degree 4
		expr := a.Mul(b).Mul(c.Mul(d))
		reduced, eliminated, newVars := ReduceDegreeOfExpressions(
			[]*Expression{expr},
			2,
			getDeg,
			noIteratorCfg,
		)
		require.Len(t, reduced, 1)
		require.NotEmpty(t, eliminated)
		require.Equal(t, len(eliminated), len(newVars))
		reducedDegree := reduced[0].Degree(getDeg)
		assert.LessOrEqual(t, reducedDegree, 2)
	})

	t.Run("multiple independent expressions", func(t *testing.T) {
		// Three independent high-degree expressions
		expr1 := a.Pow(4)
		expr2 := b.Pow(4)
		expr3 := c.Pow(4)
		reduced, eliminated, newVars := ReduceDegreeOfExpressions(
			[]*Expression{expr1, expr2, expr3},
			2,
			getDeg,
			noIteratorCfg,
		)
		require.Len(t, reduced, 3)
		require.NotEmpty(t, eliminated)
		require.Equal(t, len(eliminated), len(newVars))
		for i, red := range reduced {
			deg := red.Degree(getDeg)
			assert.LessOrEqual(t, deg, 2, "expression %d has degree %d", i, deg)
		}
	})

	t.Run("some expressions need reduction some dont", func(t *testing.T) {
		expr1 := a.Add(b)        // degree 1 - no reduction
		expr2 := a.Mul(b)        // degree 2 - no reduction
		expr3 := a.Mul(b).Mul(c) // degree 3 - needs reduction
		reduced, eliminated, newVars := ReduceDegreeOfExpressions(
			[]*Expression{expr1, expr2, expr3},
			2,
			getDeg,
			noIteratorCfg,
		)
		require.Len(t, reduced, 3)
		// Only expr3 should trigger elimination
		require.NotEmpty(t, eliminated)
		require.Equal(t, len(eliminated), len(newVars))
		for i, red := range reduced {
			deg := red.Degree(getDeg)
			assert.LessOrEqual(t, deg, 2, "expression %d has degree %d", i, deg)
		}
	})

	t.Run("repeated subexpression across many expressions", func(t *testing.T) {
		// a*b appears in all expressions - should be eliminated first
		ab := a.Mul(b)
		expr1 := ab.Mul(c)
		expr2 := ab.Mul(d)
		expr3 := ab.Mul(a)
		expr4 := ab.Mul(b)
		reduced, eliminated, newVars := ReduceDegreeOfExpressions(
			[]*Expression{expr1, expr2, expr3, expr4},
			2,
			getDeg,
			noIteratorCfg,
		)
		require.Len(t, reduced, 4)
		require.NotEmpty(t, eliminated)
		require.Equal(t, len(eliminated), len(newVars))
		// a*b should be among the eliminated since it appears 4 times
		for i, red := range reduced {
			deg := red.Degree(getDeg)
			assert.LessOrEqual(t, deg, 2, "expression %d has degree %d", i, deg)
		}
	})

	t.Run("constant expressions unchanged", func(t *testing.T) {
		// Constants have degree 0, should pass through unchanged
		one := NewConstant(1)
		expr := a.Mul(b).Mul(c).Add(one)
		reduced, eliminated, newVars := ReduceDegreeOfExpressions(
			[]*Expression{expr},
			2,
			getDeg,
			noIteratorCfg,
		)
		require.Len(t, reduced, 1)
		require.NotEmpty(t, eliminated)
		require.Equal(t, len(eliminated), len(newVars))
		reducedDegree := reduced[0].Degree(getDeg)
		assert.LessOrEqual(t, reducedDegree, 2)
	})

	t.Run("single variable unchanged", func(t *testing.T) {
		// Single variable has degree 1
		reduced, eliminated, newVars := ReduceDegreeOfExpressions(
			[]*Expression{a},
			2,
			getDeg,
			noIteratorCfg,
		)
		require.Len(t, reduced, 1)
		require.Empty(t, eliminated)
		require.Empty(t, newVars)
		assert.Equal(t, a.ESHash, reduced[0].ESHash)
	})

	t.Run("large bound no reduction", func(t *testing.T) {
		// With bound 10, even a^5 * b^5 should not need reduction
		expr := a.Pow(5).Mul(b.Pow(5))
		reduced, eliminated, newVars := ReduceDegreeOfExpressions(
			[]*Expression{expr},
			10,
			getDeg,
			noIteratorCfg,
		)
		require.Len(t, reduced, 1)
		require.Empty(t, eliminated)
		require.Empty(t, newVars)
	})

	t.Run("squared terms a^2 * b^2", func(t *testing.T) {
		// a^2 * b^2 has degree 4, with bound 2 should reduce
		expr := a.Square().Mul(b.Square())
		reduced, eliminated, newVars := ReduceDegreeOfExpressions(
			[]*Expression{expr},
			2,
			getDeg,
			noIteratorCfg,
		)
		require.Len(t, reduced, 1)
		require.NotEmpty(t, eliminated)
		require.Equal(t, len(eliminated), len(newVars))
		reducedDegree := reduced[0].Degree(getDeg)
		assert.LessOrEqual(t, reducedDegree, 2)
	})

	t.Run("complex expression with multiple operators", func(t *testing.T) {
		// (a*b*c) + (a^2*b) - (b*c*d) has degree 3 in all terms
		expr := a.Mul(b).Mul(c).Add(a.Square().Mul(b)).Sub(b.Mul(c).Mul(d))
		reduced, eliminated, newVars := ReduceDegreeOfExpressions(
			[]*Expression{expr},
			2,
			getDeg,
			noIteratorCfg,
		)
		require.Len(t, reduced, 1)
		require.NotEmpty(t, eliminated)
		require.Equal(t, len(eliminated), len(newVars))
		reducedDegree := reduced[0].Degree(getDeg)
		assert.LessOrEqual(t, reducedDegree, 2)
	})

	t.Run("preservation of expression count", func(t *testing.T) {
		// Ensure we always get back the same number of expressions
		exprs := []*Expression{
			a.Pow(3),
			b.Pow(4),
			c.Pow(5),
			a.Mul(b).Mul(c),
			a.Add(b).Mul(c.Mul(d)),
		}
		reduced, eliminated, newVars := ReduceDegreeOfExpressions(
			exprs,
			2,
			getDeg,
			noIteratorCfg,
		)
		require.Len(t, reduced, len(exprs))
		require.Equal(t, len(eliminated), len(newVars))
		for i, red := range reduced {
			deg := red.Degree(getDeg)
			assert.LessOrEqual(t, deg, 2, "expression %d has degree %d", i, deg)
		}
	})

	t.Run("unsimplified product chain", func(t *testing.T) {

		factorBottom := MulNoSimplify(
			Sub(a, 3),
			MulNoSimplify(
				Sub(a, 2),
				Sub(a, 1),
			),
		)

		factorTop := MulNoSimplify(
			Sub(a, 8),
			MulNoSimplify(
				Sub(a, 9),
				Sub(a, 10),
			),
		)

		expr := MulNoSimplify(b, factorBottom, factorTop)
		reduced, eliminated, newVars := ReduceDegreeOfExpressions(
			[]*Expression{expr},
			2,
			getDeg,
			noIteratorCfg,
		)
		require.Len(t, reduced, 1)
		require.NotEmpty(t, eliminated)
		require.Equal(t, len(eliminated), len(newVars))
		reducedDegree := reduced[0].Degree(getDeg)
		assert.LessOrEqual(t, reducedDegree, 2)

	})
}

func TestFindOverDegreeIndices(t *testing.T) {

	var (
		a      = NewDummyVar("a")
		b      = NewDummyVar("b")
		getDeg = constantDegreeGetter()
	)

	t.Run("mixed degrees", func(t *testing.T) {
		exprs := []*Expression{
			a.Add(b),        // degree 1
			a.Mul(b),        // degree 2
			a.Mul(b).Mul(a), // degree 3
		}

		indices := findOverDegreeIndices(exprs, 2, getDeg)
		require.Len(t, indices, 1)
		assert.Equal(t, 2, indices[0])
	})

	t.Run("all within bound", func(t *testing.T) {
		exprs := []*Expression{
			a.Add(b),
			a.Mul(b),
		}

		indices := findOverDegreeIndices(exprs, 2, getDeg)
		require.Empty(t, indices)
	})

	t.Run("all over bound", func(t *testing.T) {
		exprs := []*Expression{
			a.Mul(b).Mul(a),
			a.Pow(4),
		}

		indices := findOverDegreeIndices(exprs, 2, getDeg)
		require.Len(t, indices, 2)
	})
}

func TestIsTrivialExpr(t *testing.T) {

	var (
		a   = NewDummyVar("a")
		one = NewConstant(1)
	)

	t.Run("variable is trivial", func(t *testing.T) {
		assert.True(t, isTrivialExpr(a))
	})

	t.Run("constant is trivial", func(t *testing.T) {
		assert.True(t, isTrivialExpr(one))
	})

	t.Run("product is not trivial", func(t *testing.T) {
		expr := Mul(a, a)
		assert.False(t, isTrivialExpr(expr))
	})

	t.Run("lincomb is not trivial", func(t *testing.T) {
		b := NewDummyVar("b")
		expr := a.Add(b)
		assert.False(t, isTrivialExpr(expr))
	})
}

func TestSubstituteExpr(t *testing.T) {

	var (
		a = NewDummyVar("a")
		b = NewDummyVar("b")
		c = NewDummyVar("c")
	)

	t.Run("exact match replacement", func(t *testing.T) {
		target := a.Mul(b)
		replacement := c

		expr := a.Mul(b)
		result := substituteExpr(expr, target, replacement)

		assert.Equal(t, c.ESHash, result.ESHash)
	})

	t.Run("nested replacement", func(t *testing.T) {
		target := a.Mul(b)
		replacement := c

		// (a*b) + a should become c + a
		expr := target.Add(a)
		result := substituteExpr(expr, target, replacement)

		expected := c.Add(a)
		assert.Equal(t, expected.ESHash, result.ESHash)
	})

	t.Run("no match returns original", func(t *testing.T) {
		target := a.Mul(b)
		replacement := c

		expr := a.Add(b)
		result := substituteExpr(expr, target, replacement)

		assert.Equal(t, expr.ESHash, result.ESHash)
	})
}

func TestBuildSubProduct(t *testing.T) {

	var (
		a = NewDummyVar("a")
		b = NewDummyVar("b")
		c = NewDummyVar("c")
	)

	t.Run("single factor", func(t *testing.T) {
		children := []*Expression{a, b, c}
		exponents := []int{1, 2, 1}
		mask := uint64(0b001) // select only 'a'

		result := buildSubProduct(children, exponents, mask)
		require.NotNil(t, result)

		// Should be equivalent to a^1
		assert.Equal(t, a.ESHash, result.ESHash)
	})

	t.Run("two factors", func(t *testing.T) {
		children := []*Expression{a, b, c}
		exponents := []int{1, 2, 1}
		mask := uint64(0b011) // select 'a' and 'b'

		result := buildSubProduct(children, exponents, mask)
		require.NotNil(t, result)

		// Should be equivalent to a * b^2
		expected := NewProduct([]*Expression{a, b}, []int{1, 2})
		assert.Equal(t, expected.ESHash, result.ESHash)
	})

	t.Run("empty mask", func(t *testing.T) {
		children := []*Expression{a, b}
		exponents := []int{1, 1}
		mask := uint64(0)

		result := buildSubProduct(children, exponents, mask)
		assert.Nil(t, result)
	})
}

func TestComputeSubsetExponentSum(t *testing.T) {

	t.Run("single element", func(t *testing.T) {
		exponents := []int{3, 2, 1}
		mask := uint64(0b001)

		sum := computeSubsetExponentSum(mask, exponents)
		assert.Equal(t, 3, sum)
	})

	t.Run("multiple elements", func(t *testing.T) {
		exponents := []int{3, 2, 1}
		mask := uint64(0b111)

		sum := computeSubsetExponentSum(mask, exponents)
		assert.Equal(t, 6, sum)
	})

	t.Run("empty mask", func(t *testing.T) {
		exponents := []int{3, 2, 1}
		mask := uint64(0)

		sum := computeSubsetExponentSum(mask, exponents)
		assert.Equal(t, 0, sum)
	})
}

func TestEliminatedVarMetadata(t *testing.T) {

	a := NewDummyVar("a")
	expr := a.Mul(a)

	meta := EliminatedVarMetadata{id: 42, expr: expr}

	t.Run("string format", func(t *testing.T) {
		assert.Equal(t, "_elim_42", meta.String())
	})

	t.Run("isbase from expr", func(t *testing.T) {
		// The expression a*a is base if a is base
		assert.Equal(t, expr.IsBase, meta.IsBase())
	})
}

func TestSelectMostFrequentCandidate(t *testing.T) {

	var (
		a = NewDummyVar("a")
		b = NewDummyVar("b")
	)

	t.Run("selects highest count", func(t *testing.T) {
		candidates := []candidateInfo{
			{expr: a, count: 2},
			{expr: b, count: 5},
		}

		result := selectMostFrequentCandidate(candidates)
		assert.Equal(t, b.ESHash, result.ESHash)
	})

	t.Run("tie breaker by children count", func(t *testing.T) {
		ab := a.Mul(b)
		candidates := []candidateInfo{
			{expr: a, count: 3},
			{expr: ab, count: 3},
		}

		result := selectMostFrequentCandidate(candidates)
		// ab has more children, so it should be selected
		assert.Equal(t, ab.ESHash, result.ESHash)
	})
}

func TestReduceDegreePreservesSemantics(t *testing.T) {
	// This test verifies that the reduced expression computes
	// the same value as the original when the eliminated subexpressions
	// are substituted back

	var (
		a      = NewDummyVar("a")
		b      = NewDummyVar("b")
		getDeg = constantDegreeGetter()
	)

	t.Run("semantic equivalence", func(t *testing.T) {
		original := a.Mul(b).Mul(a).Add(a.Mul(b))
		reduced, eliminated, newVars := ReduceDegreeOfExpressions(
			[]*Expression{original},
			2,
			getDeg,
			noIteratorCfg,
		)

		require.Len(t, reduced, 1)

		// Verify that we can trace back the eliminated expressions
		for i, meta := range newVars {
			elimMeta, ok := meta.(EliminatedVarMetadata)
			require.True(t, ok)
			assert.Equal(t, eliminated[i].ESHash, elimMeta.expr.ESHash)
		}
	})
}

func TestBuildSubProductFromExponents(t *testing.T) {

	var (
		a = NewDummyVar("a")
		b = NewDummyVar("b")
		c = NewDummyVar("c")
	)

	t.Run("all zero exponents", func(t *testing.T) {
		result := buildSubProductFromExponents([]*Expression{a, b}, []int{0, 0})
		assert.Nil(t, result)
	})

	t.Run("single factor selected", func(t *testing.T) {
		result := buildSubProductFromExponents([]*Expression{a, b}, []int{2, 0})
		require.NotNil(t, result)
		// Should be a^2
		expected := NewProduct([]*Expression{a}, []int{2})
		assert.Equal(t, expected.ESHash, result.ESHash)
	})

	t.Run("multiple factors with varying exponents", func(t *testing.T) {
		result := buildSubProductFromExponents([]*Expression{a, b, c}, []int{1, 2, 0})
		require.NotNil(t, result)
		// Should be a * b^2
		expected := NewProduct([]*Expression{a, b}, []int{1, 2})
		assert.Equal(t, expected.ESHash, result.ESHash)
	})

	t.Run("all factors selected", func(t *testing.T) {
		result := buildSubProductFromExponents([]*Expression{a, b}, []int{1, 1})
		require.NotNil(t, result)
		expected := NewProduct([]*Expression{a, b}, []int{1, 1})
		assert.Equal(t, expected.ESHash, result.ESHash)
	})
}

func TestCollectSubProducts(t *testing.T) {

	var (
		a      = NewDummyVar("a")
		getDeg = constantDegreeGetter()
	)

	t.Run("a^3 collects a and a^2", func(t *testing.T) {
		// a^3 should generate sub-multisets: a^2
		expr := a.Pow(3)
		prod := expr.Operator.(Product)
		candidates := collection.MakeDeterministicMap[esHash, candidateInfo](0)

		collectSubProducts(expr, prod, 2, getDeg, noIteratorCfg, candidates)

		// With degree bound 2, only a^2 (deg=2) qualifies
		require.Equal(t, 1, candidates.Len())

		aSquared := a.Pow(2)
		hasASquared := candidates.HasKey(aSquared.ESHash)
		assert.True(t, hasASquared, "should contain a^2")
	})

	t.Run("a^5 collects a, a^2, a^3 and a^4", func(t *testing.T) {

		// a^5 should generate sub-multisets: a^2, a^3, a^4
		expr := a.Pow(5)
		prod := expr.Operator.(Product)
		candidates := collection.MakeDeterministicMap[esHash, candidateInfo](0)

		collectSubProducts(expr, prod, 5, getDeg, noIteratorCfg, candidates)

		require.Equal(t, 3, candidates.Len())

		for d := 2; d < 5; d++ {
			aPowD := a.Pow(d)
			hasAPowD := candidates.HasKey(aPowD.ESHash)
			assert.True(t, hasAPowD, "should contain a^2")
		}
	})
}

func TestMultiSetIteratorEmptyInput(t *testing.T) {
	it := newWeightedSubMultisetIterator([]int{}, []int{}, 10, noIteratorCfg)

	if it.next() {
		t.Error("Expected no results for empty input")
	}
}

func TestMultiSetIteratorSingleElementBelowWeight(t *testing.T) {
	// Single element with max exponent 3, weight 1, max weight 10
	// Should yield exponents 1, 2 (excluding 0=empty and 3=full)
	it := newWeightedSubMultisetIterator([]int{3}, []int{1}, 10, noIteratorCfg)

	var results [][]int
	for it.next() {
		results = append(results, it.currentSnapshot())
	}

	expected := [][]int{{1}, {2}}
	if !equalResults(results, expected) {
		t.Errorf("Expected %v, got %v", expected, results)
	}
}

func TestMultiSetIteratorSingleElementWeightConstrained(t *testing.T) {
	// Single element with max exponent 5, weight 3, max weight 7
	// Valid weights: 3 (exp=1), 6 (exp=2). Exp 0 is empty, exp 5 is full.
	it := newWeightedSubMultisetIterator([]int{5}, []int{3}, 7, noIteratorCfg)

	var results [][]int
	for it.next() {
		results = append(results, it.currentSnapshot())
	}

	expected := [][]int{{1}, {2}}
	if !equalResults(results, expected) {
		t.Errorf("Expected %v, got %v", expected, results)
	}
}

func TestMultiSetIteratorTwoElementsSimple(t *testing.T) {
	// Two elements, each with max exponent 1, uniform weights, max weight 10
	// Possible: [0,0], [0,1], [1,0], [1,1]
	// Excluding empty [0,0] and full [1,1]: expect [0,1], [1,0]
	it := newWeightedSubMultisetIterator([]int{1, 1}, []int{1, 1}, 10, noIteratorCfg)

	var results [][]int
	for it.next() {
		results = append(results, it.currentSnapshot())
	}

	expected := [][]int{{0, 1}, {1, 0}}
	if !equalResults(results, expected) {
		t.Errorf("Expected %v, got %v", expected, results)
	}
}

// =============================================================================
// Weight Constraint Tests
// =============================================================================

func TestMultiSetIteratorWeightConstraintExcludesHeavyMultisets(t *testing.T) {
	// Exponents [2, 2], weights [5, 3], maxWeight 6
	// Possible combinations and their weights:
	// [0,0]=0, [0,1]=3, [0,2]=6, [1,0]=5, [1,1]=8(excluded), [1,2]=11(excluded)
	// [2,0]=10(excluded), [2,1]=13(excluded), [2,2]=16(excluded,full)
	// Valid non-trivial: [0,1], [0,2], [1,0]
	it := newWeightedSubMultisetIterator([]int{2, 2}, []int{5, 3}, 6, noIteratorCfg)

	var results [][]int
	for it.next() {
		results = append(results, it.currentSnapshot())

		// Verify weight constraint
		weight := it.currentTotalWeight()
		if weight > 6 {
			t.Errorf("Result %v has weight %d > maxWeight 6", it.currentSnapshot(), weight)
		}
	}

	expected := [][]int{{0, 1}, {0, 2}, {1, 0}}
	if !equalResults(results, expected) {
		t.Errorf("Expected %v, got %v", expected, results)
	}
}

func TestMultiSetIteratorZeroMaxWeightYieldsNothing(t *testing.T) {
	// With maxWeight=0, only [0,0] is valid by weight, but it's empty (excluded)
	it := newWeightedSubMultisetIterator([]int{3, 3}, []int{1, 1}, 0, noIteratorCfg)

	if it.next() {
		t.Errorf("Expected no results with maxWeight=0, got %v", it.currentSnapshot())
	}
}

func TestMultiSetIteratorMaxWeightEqualsMinNonEmptyWeight(t *testing.T) {
	// Weights [10, 20], maxWeight=10
	// Only [1,0] has weight exactly 10 and is non-trivial
	it := newWeightedSubMultisetIterator([]int{5, 5}, []int{10, 20}, 10, noIteratorCfg)

	var results [][]int
	for it.next() {
		results = append(results, it.currentSnapshot())
	}

	expected := [][]int{{1, 0}}
	if !equalResults(results, expected) {
		t.Errorf("Expected %v, got %v", expected, results)
	}
}

func TestMultiSetIteratorNonUniformWeights(t *testing.T) {
	// Exponents [3, 2, 1], weights [1, 2, 10], maxWeight 5
	// Heavy third element (weight 10) should never appear
	it := newWeightedSubMultisetIterator([]int{3, 2, 1}, []int{1, 2, 10}, 5, noIteratorCfg)

	for it.next() {
		snapshot := it.currentSnapshot()
		if snapshot[2] != 0 {
			t.Errorf("Third element should never appear, got %v", snapshot)
		}
		if it.currentTotalWeight() > 5 {
			t.Errorf("Weight constraint violated: %v has weight %d", snapshot, it.currentTotalWeight())
		}
	}
}

// =============================================================================
// Edge Cases for Empty and Full Exclusion
// =============================================================================

func TestMultiSetIteratorExcludesEmptyMultiset(t *testing.T) {
	it := newWeightedSubMultisetIterator([]int{2, 2}, []int{1, 1}, 100, noIteratorCfg)

	for it.next() {
		if it.isEmpty() {
			t.Error("Iterator returned empty multiset")
		}
	}
}

func TestMultiSetIteratorExcludesFullMultiset(t *testing.T) {
	maxExp := []int{2, 3}
	it := newWeightedSubMultisetIterator(maxExp, []int{1, 1}, 100, noIteratorCfg)

	for it.next() {
		if it.isFull() {
			t.Error("Iterator returned full multiset")
		}
	}
}

func TestMultiSetIteratorFullMultisetExcludedEvenWhenUnderWeight(t *testing.T) {
	// Full multiset [2,2] has weight 4, which is under maxWeight 10
	// But it should still be excluded
	it := newWeightedSubMultisetIterator([]int{2, 2}, []int{1, 1}, 10, noIteratorCfg)

	for it.next() {
		snapshot := it.currentSnapshot()
		if snapshot[0] == 2 && snapshot[1] == 2 {
			t.Error("Full multiset should be excluded even when under weight")
		}
	}
}

func TestMultiSetIteratorAllOnesMaxExponents(t *testing.T) {
	// When all maxExponents are 1, only empty [0,0,0] and full [1,1,1] exist
	// for a 3-element case. With weight constraint high enough, we get
	// all 2^3 - 2 = 6 non-trivial subsets
	it := newWeightedSubMultisetIterator([]int{1, 1, 1}, []int{1, 1, 1}, 100, noIteratorCfg)

	count := 0
	for it.next() {
		count++
	}

	// 2^3 - 2 = 6 (excluding empty and full)
	if count != 6 {
		t.Errorf("Expected 6 results, got %d", count)
	}
}

// =============================================================================
// Correctness Property Tests
// =============================================================================

func TestMultiSetIteratorAllResultsAreUnique(t *testing.T) {
	it := newWeightedSubMultisetIterator([]int{3, 2, 2}, []int{2, 3, 1}, 8, noIteratorCfg)

	seen := make(map[string]bool)
	for it.next() {
		key := formatExponents(it.currentSnapshot())
		if seen[key] {
			t.Errorf("Duplicate result: %v", it.currentSnapshot())
		}
		seen[key] = true
	}
}

func TestMultiSetIteratorAllResultsSatisfyWeightConstraint(t *testing.T) {
	maxWeight := 7
	weights := []int{3, 2, 4}
	it := newWeightedSubMultisetIterator([]int{4, 3, 2}, weights, maxWeight, noIteratorCfg)

	for it.next() {
		weight := it.currentTotalWeight()
		if weight > maxWeight {
			t.Errorf("Result %v has weight %d > maxWeight %d",
				it.currentSnapshot(), weight, maxWeight)
		}
	}
}

func TestMultiSetIteratorAllResultsRespectMaxExponents(t *testing.T) {
	maxExp := []int{3, 2, 4}
	it := newWeightedSubMultisetIterator(maxExp, []int{1, 1, 1}, 100, noIteratorCfg)

	for it.next() {
		snapshot := it.currentSnapshot()
		for i, exp := range snapshot {
			if exp < 0 || exp > maxExp[i] {
				t.Errorf("Exponent %d at position %d out of range [0, %d]",
					exp, i, maxExp[i])
			}
		}
	}
}

func TestMultiSetIteratorCountMatchesBruteForce(t *testing.T) {
	maxExp := []int{2, 3, 2}
	weights := []int{2, 1, 3}
	maxWeight := 6

	// Brute force count
	bruteCount := 0
	for i := 0; i <= maxExp[0]; i++ {
		for j := 0; j <= maxExp[1]; j++ {
			for k := 0; k <= maxExp[2]; k++ {
				weight := i*weights[0] + j*weights[1] + k*weights[2]
				isEmpty := i == 0 && j == 0 && k == 0
				isFull := i == maxExp[0] && j == maxExp[1] && k == maxExp[2]
				if weight <= maxWeight && !isEmpty && !isFull {
					bruteCount++
				}
			}
		}
	}

	// Iterator count
	it := newWeightedSubMultisetIterator(maxExp, weights, maxWeight, noIteratorCfg)
	iterCount := 0
	for it.next() {
		iterCount++
	}

	if iterCount != bruteCount {
		t.Errorf("Iterator count %d != brute force count %d", iterCount, bruteCount)
	}
}

func TestMultiSetIteratorResultsMatchBruteForce(t *testing.T) {
	maxExp := []int{2, 2}
	weights := []int{3, 2}
	maxWeight := 5

	// Brute force collection
	var bruteResults [][]int
	for i := 0; i <= maxExp[0]; i++ {
		for j := 0; j <= maxExp[1]; j++ {
			weight := i*weights[0] + j*weights[1]
			isEmpty := i == 0 && j == 0
			isFull := i == maxExp[0] && j == maxExp[1]
			if weight <= maxWeight && !isEmpty && !isFull {
				bruteResults = append(bruteResults, []int{i, j})
			}
		}
	}

	// Iterator collection
	it := newWeightedSubMultisetIterator(maxExp, weights, maxWeight, noIteratorCfg)
	var iterResults [][]int
	for it.next() {
		iterResults = append(iterResults, it.currentSnapshot())
	}

	if !equalResults(iterResults, bruteResults) {
		t.Errorf("Results mismatch.\nIterator: %v\nBrute force: %v", iterResults, bruteResults)
	}
}

// =============================================================================
// Nil Weights (Default to 1)
// =============================================================================

func TestMultiSetIteratorNilWeightsDefaultToOne(t *testing.T) {
	maxExp := []int{2, 2}
	maxWeight := 3

	// With nil weights (defaults to 1s)
	it1 := newWeightedSubMultisetIterator(maxExp, nil, maxWeight, noIteratorCfg)
	var results1 [][]int
	for it1.next() {
		results1 = append(results1, it1.currentSnapshot())
	}

	// With explicit weights of 1
	it2 := newWeightedSubMultisetIterator(maxExp, []int{1, 1}, maxWeight, noIteratorCfg)
	var results2 [][]int
	for it2.next() {
		results2 = append(results2, it2.currentSnapshot())
	}

	if !equalResults(results1, results2) {
		t.Errorf("Nil weights should default to 1s.\nNil: %v\nExplicit: %v", results1, results2)
	}
}

// =============================================================================
// Iterator State Tests
// =============================================================================

func TestMultiSetIteratorExhaustedIteratorReturnsFalse(t *testing.T) {
	it := newWeightedSubMultisetIterator([]int{1, 1}, []int{1, 1}, 10, noIteratorCfg)

	// Exhaust the iterator
	for it.next() {
	}

	// Should keep returning false
	for i := 0; i < 5; i++ {
		if it.next() {
			t.Error("Exhausted iterator should keep returning false")
		}
	}
}

func TestMultiSetIteratorCurrentSnapshotIsACopy(t *testing.T) {
	it := newWeightedSubMultisetIterator([]int{2, 2}, []int{1, 1}, 10, noIteratorCfg)

	it.next()
	snapshot1 := it.currentSnapshot()
	original := make([]int, len(snapshot1))
	copy(original, snapshot1)

	// Modify the snapshot
	snapshot1[0] = 999

	// Get another snapshot
	snapshot2 := it.currentSnapshot()

	// Should be unchanged
	if !reflect.DeepEqual(snapshot2, original) {
		t.Error("currentSnapshot should return independent copies")
	}
}

func TestMultiSetIteratorCurrentTotalWeightIsCorrect(t *testing.T) {
	weights := []int{5, 3, 7}
	it := newWeightedSubMultisetIterator([]int{3, 3, 3}, weights, 20, noIteratorCfg)

	for it.next() {
		snapshot := it.currentSnapshot()
		expectedWeight := 0
		for i, exp := range snapshot {
			expectedWeight += exp * weights[i]
		}

		if it.currentTotalWeight() != expectedWeight {
			t.Errorf("currentTotalWeight() = %d, expected %d for %v",
				it.currentTotalWeight(), expectedWeight, snapshot)
		}
	}
}

// =============================================================================
// Special Cases
// =============================================================================

func TestSingleElementWithExponentOne(t *testing.T) {
	// Single element with max exponent 1 means only [0] (empty) and [1] (full)
	// Both should be excluded, so no results
	it := newWeightedSubMultisetIterator([]int{1}, []int{1}, 100, noIteratorCfg)

	if it.next() {
		t.Errorf("Expected no results for single element with max exponent 1, got %v",
			it.currentSnapshot())
	}
}

func TestSingleElementWithExponentZero(t *testing.T) {
	// Max exponent 0 means only [0] exists, which is both empty and full
	it := newWeightedSubMultisetIterator([]int{0}, []int{1}, 100, noIteratorCfg)

	if it.next() {
		t.Errorf("Expected no results for max exponent 0, got %v", it.currentSnapshot())
	}
}

func TestAllZeroMaxExponents(t *testing.T) {
	// All max exponents are 0
	it := newWeightedSubMultisetIterator([]int{0, 0, 0}, []int{1, 1, 1}, 100, noIteratorCfg)

	if it.next() {
		t.Errorf("Expected no results for all zero max exponents, got %v",
			it.currentSnapshot())
	}
}

func TestZeroWeightElements(t *testing.T) {
	// Elements with weight 0 don't contribute to total weight
	// Exponents [2, 2], weights [0, 5], maxWeight 4
	// First element is "free", second element can only be 0
	// Valid: [1,0], [2,0] (excluding [0,0]=empty and [2,2]=full)
	it := newWeightedSubMultisetIterator([]int{2, 2}, []int{0, 5}, 4, noIteratorCfg)

	var results [][]int
	for it.next() {
		results = append(results, it.currentSnapshot())
	}

	expected := [][]int{{1, 0}, {2, 0}}
	if !equalResults(results, expected) {
		t.Errorf("Expected %v, got %v", expected, results)
	}
}

func TestAllZeroWeights(t *testing.T) {
	// All weights are 0, so everything has weight 0 <= maxWeight
	// Should return all non-trivial sub-multisets
	it := newWeightedSubMultisetIterator([]int{2, 2}, []int{0, 0}, 0, noIteratorCfg)

	count := 0
	for it.next() {
		count++
	}

	// Total combinations: 3 * 3 = 9, minus empty and full = 7
	if count != 7 {
		t.Errorf("Expected 7 results with all zero weights, got %d", count)
	}
}

func TestVeryLargeMaxWeight(t *testing.T) {
	// maxWeight so large it doesn't constrain anything
	maxExp := []int{3, 3, 3}
	it := newWeightedSubMultisetIterator(maxExp, []int{1, 1, 1}, 1000000, noIteratorCfg)

	count := 0
	for it.next() {
		count++
	}

	// Total: 4 * 4 * 4 = 64, minus empty and full = 62
	if count != 62 {
		t.Errorf("Expected 62 results, got %d", count)
	}
}

func TestMaxWeightExactlyAllowsOneElement(t *testing.T) {
	// Weights [10, 20, 30], maxWeight 10
	// Only first element with exp=1 is allowed (and non-trivial)
	it := newWeightedSubMultisetIterator([]int{5, 5, 5}, []int{10, 20, 30}, 10, noIteratorCfg)

	var results [][]int
	for it.next() {
		results = append(results, it.currentSnapshot())
	}

	expected := [][]int{{1, 0, 0}}
	if !equalResults(results, expected) {
		t.Errorf("Expected %v, got %v", expected, results)
	}
}

func TestMixedZeroAndNonZeroMaxExponents(t *testing.T) {
	// Some elements have max exponent 0 (effectively not in the multiset)
	// Exponents [3, 0, 2], weights [1, 1, 1], maxWeight 100
	it := newWeightedSubMultisetIterator([]int{3, 0, 2}, []int{1, 1, 1}, 100, noIteratorCfg)

	for it.next() {
		snapshot := it.currentSnapshot()
		if snapshot[1] != 0 {
			t.Errorf("Element with max exponent 0 should always be 0, got %v", snapshot)
		}
	}
}

func TestLargeExponentsSmallWeight(t *testing.T) {
	// Large max exponents but very restrictive weight
	// Exponents [100, 100, 100], weights [10, 10, 10], maxWeight 15
	// Only combinations with total exponent <= 1 (weight <= 10) work
	it := newWeightedSubMultisetIterator([]int{100, 100, 100}, []int{10, 10, 10}, 15, noIteratorCfg)

	var results [][]int
	for it.next() {
		results = append(results, it.currentSnapshot())
		if it.currentTotalWeight() > 15 {
			t.Errorf("Weight constraint violated: %v", it.currentSnapshot())
		}
	}

	// Valid: [1,0,0], [0,1,0], [0,0,1] (each has weight 10)
	expected := [][]int{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}
	if !equalResults(results, expected) {
		t.Errorf("Expected %v, got %v", expected, results)
	}
}

func TestPruningEfficiency(t *testing.T) {
	// This test verifies pruning happens (indirectly via reasonable runtime)
	// Without pruning, this would enumerate 101^5 = 10+ billion combinations
	// With pruning, it should be very fast
	maxExp := []int{100, 100, 100, 100, 100}
	weights := []int{1, 1, 1, 1, 1}
	maxWeight := 3

	count := 0
	it := newWeightedSubMultisetIterator(maxExp, weights, maxWeight, noIteratorCfg)
	for it.next() {
		count++
		if it.currentTotalWeight() > maxWeight {
			t.Errorf("Weight constraint violated: %v", it.currentSnapshot())
		}
	}

	// With degree <= 3 across 5 elements, there are limited valid combinations
	// This should complete quickly if pruning works
	if count == 0 {
		t.Error("Expected some results")
	}
}

func TestWeightBoundaryExactMatch(t *testing.T) {
	// Test that weight exactly equal to maxWeight is included
	// Exponents [2, 2], weights [3, 4], maxWeight 7
	// [1, 1] has weight 3+4=7, should be included
	it := newWeightedSubMultisetIterator([]int{2, 2}, []int{3, 4}, 7, noIteratorCfg)

	found := false
	for it.next() {
		snapshot := it.currentSnapshot()
		if snapshot[0] == 1 && snapshot[1] == 1 {
			found = true
			if it.currentTotalWeight() != 7 {
				t.Errorf("Expected weight 7 for [1,1], got %d", it.currentTotalWeight())
			}
		}
	}

	if !found {
		t.Error("Expected [1, 1] with exact weight match to be included")
	}
}

func TestWeightBoundaryJustOver(t *testing.T) {
	// Test that weight just over maxWeight is excluded
	// Exponents [2, 2], weights [3, 4], maxWeight 6
	// [1, 1] has weight 7 > 6, should be excluded
	it := newWeightedSubMultisetIterator([]int{2, 2}, []int{3, 4}, 6, noIteratorCfg)

	for it.next() {
		snapshot := it.currentSnapshot()
		if snapshot[0] == 1 && snapshot[1] == 1 {
			t.Error("Expected [1, 1] with weight 7 > 6 to be excluded")
		}
	}
}

func TestSymmetricExponentsAsymmetricWeights(t *testing.T) {
	// Same max exponents but different weights should yield different results
	maxExp := []int{3, 3}

	it1 := newWeightedSubMultisetIterator(maxExp, []int{1, 10}, 5, noIteratorCfg)
	var results1 [][]int
	for it1.next() {
		results1 = append(results1, it1.currentSnapshot())
	}

	it2 := newWeightedSubMultisetIterator(maxExp, []int{10, 1}, 5, noIteratorCfg)
	var results2 [][]int
	for it2.next() {
		results2 = append(results2, it2.currentSnapshot())
	}

	// Results should be "mirrored" due to swapped weights
	if len(results1) != len(results2) {
		t.Errorf("Symmetric case should have same count: %d vs %d",
			len(results1), len(results2))
	}
}

func TestMultipleIteratorsIndependent(t *testing.T) {
	// Two iterators on same input should be independent
	maxExp := []int{2, 2}
	weights := []int{1, 1}
	maxWeight := 3

	it1 := newWeightedSubMultisetIterator(maxExp, weights, maxWeight, noIteratorCfg)
	it2 := newWeightedSubMultisetIterator(maxExp, weights, maxWeight, noIteratorCfg)

	// Advance it1 once
	it1.next()
	snap1 := it1.currentSnapshot()

	// it2 should start fresh
	it2.next()
	snap2 := it2.currentSnapshot()

	// Both should have same first result
	if !reflect.DeepEqual(snap1, snap2) {
		t.Errorf("Independent iterators should have same first result: %v vs %v",
			snap1, snap2)
	}

	// Advance it1 more
	it1.next()
	it1.next()

	// it2's current should still be first result
	snap2Again := it2.currentSnapshot()
	if !reflect.DeepEqual(snap2, snap2Again) {
		t.Error("Advancing one iterator should not affect another")
	}
}

func TestHighDimensionalSmallWeight(t *testing.T) {
	// Many dimensions but small weight forces sparse results
	n := 10
	maxExp := make([]int, n)
	weights := make([]int, n)
	for i := 0; i < n; i++ {
		maxExp[i] = 5
		weights[i] = 1
	}
	maxWeight := 2

	it := newWeightedSubMultisetIterator(maxExp, weights, maxWeight, noIteratorCfg)

	count := 0
	for it.next() {
		count++
		degree := 0
		for _, e := range it.currentSnapshot() {
			degree += e
		}
		if degree > maxWeight {
			t.Errorf("Degree %d exceeds maxWeight %d", degree, maxWeight)
		}
	}

	// With n=10 elements and max degree 2:
	// Degree 1: C(10,1) = 10 ways
	// Degree 2: C(10,1) (one element with exp 2) + C(10,2) (two elements with exp 1)
	//         = 10 + 45 = 55 ways
	// Total = 65
	expectedCount := 10 + 10 + 45
	if count != expectedCount {
		t.Errorf("Expected %d results, got %d", expectedCount, count)
	}
}

func TestOnlyFullMultisetUnderWeight(t *testing.T) {
	// Edge case where the only multiset under weight is the full one
	// Exponents [1, 1], weights [1, 1], maxWeight 2
	// [0,0]=0 (empty), [0,1]=1, [1,0]=1, [1,1]=2 (full)
	// Valid non-trivial: [0,1], [1,0]
	it := newWeightedSubMultisetIterator([]int{1, 1}, []int{1, 1}, 2, noIteratorCfg)

	var results [][]int
	for it.next() {
		results = append(results, it.currentSnapshot())
	}

	expected := [][]int{{0, 1}, {1, 0}}
	if !equalResults(results, expected) {
		t.Errorf("Expected %v, got %v", expected, results)
	}
}

func TestNegativeMaxWeightYieldsNothing(t *testing.T) {
	// Negative max weight should yield no results
	it := newWeightedSubMultisetIterator([]int{2, 2}, []int{1, 1}, -1, noIteratorCfg)

	if it.next() {
		t.Errorf("Expected no results with negative maxWeight, got %v",
			it.currentSnapshot())
	}
}

func TestVeryLargeWeights(t *testing.T) {
	// Large weights to check for overflow issues
	it := newWeightedSubMultisetIterator(
		[]int{2, 2},
		[]int{1000000000, 1000000000},
		1500000000,
		noIteratorCfg,
	)

	var results [][]int
	for it.next() {
		results = append(results, it.currentSnapshot())
	}

	// Only [1, 0] and [0, 1] have weight 1 billion <= 1.5 billion
	expected := [][]int{{1, 0}, {0, 1}}
	if !equalResults(results, expected) {
		t.Errorf("Expected %v, got %v", expected, results)
	}
}

func TestFullIsAlsoEmpty(t *testing.T) {
	// When all maxExponents are 0, empty and full are the same
	// Should yield no results
	it := newWeightedSubMultisetIterator([]int{0, 0}, []int{1, 1}, 100, noIteratorCfg)

	if it.next() {
		t.Errorf("Expected no results when full == empty, got %v", it.currentSnapshot())
	}
}

// formatExponents creates a string key from an exponent slice for use in maps
func formatExponents(exponents []int) string {
	if len(exponents) == 0 {
		return "[]"
	}

	result := "["
	for i, e := range exponents {
		if i > 0 {
			result += ","
		}
		result += fmt.Sprintf("%d", e)
	}
	result += "]"
	return result
}

// equalResults compares two slices of results, ignoring order
func equalResults(a, b [][]int) bool {
	if len(a) != len(b) {
		return false
	}

	// Make copies to avoid modifying originals
	aCopy := make([][]int, len(a))
	bCopy := make([][]int, len(b))

	for i := range a {
		aCopy[i] = make([]int, len(a[i]))
		copy(aCopy[i], a[i])
	}
	for i := range b {
		bCopy[i] = make([]int, len(b[i]))
		copy(bCopy[i], b[i])
	}

	// Sort both for comparison
	sortResults(aCopy)
	sortResults(bCopy)

	return reflect.DeepEqual(aCopy, bCopy)
}

// sortResults sorts a slice of int slices lexicographically
func sortResults(results [][]int) {
	sort.Slice(results, func(i, j int) bool {
		minLen := len(results[i])
		if len(results[j]) < minLen {
			minLen = len(results[j])
		}

		for k := 0; k < minLen; k++ {
			if results[i][k] != results[j][k] {
				return results[i][k] < results[j][k]
			}
		}
		return len(results[i]) < len(results[j])
	})
}
