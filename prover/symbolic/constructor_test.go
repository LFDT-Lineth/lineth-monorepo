package symbolic

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConstructor(t *testing.T) {

	var (
		a    = NewDummyVar("a")
		b    = NewDummyVar("b")
		c    = NewDummyVar("c")
		zero = NewConstant(0)
		one  = NewConstant(1)
	)

	testCases := []struct {
		Actual           *Expression
		ExpectedOperator Operator
		ExpectedParent   []*Expression
		Explainer        string
	}{
		{
			Actual:           a.Add(b),
			ExpectedOperator: LinComb{Coeffs: []int{1, 1}},
			ExpectedParent:   []*Expression{a, b},
			Explainer:        "Normally adding two variables",
		},
		{
			Actual:           a.Add(zero),
			ExpectedOperator: a.Operator,
			ExpectedParent:   []*Expression{},
			Explainer:        "When adding zero, this should be a no-op",
		},
		{
			Actual:           a.Add(a),
			ExpectedOperator: LinComb{Coeffs: []int{2}},
			ExpectedParent:   []*Expression{a},
			Explainer:        "When adding twice the same variable, this should resolve in a lincomb with a single term and a coefficient of 2",
		},
		{
			Actual:           a.Add(one).Add(one),
			ExpectedOperator: LinComb{Coeffs: []int{1, 1}},
			ExpectedParent:   []*Expression{a, NewConstant(2)},
			Explainer:        "When adding twice a constant, the constants should be regrouped",
		},
		{
			Actual:           a.Add(b).Add(c),
			ExpectedOperator: LinComb{Coeffs: []int{1, 1, 1}},
			ExpectedParent:   []*Expression{a, b, c},
			Explainer:        "LinComb should be regrouped by associativity",
		},
		{
			Actual:           a.Sub(b),
			ExpectedOperator: LinComb{Coeffs: []int{1, -1}},
			ExpectedParent:   []*Expression{b, a},
			Explainer:        "Normally substracting two variables should create a LinComb object",
		},
		{
			Actual:           a.Sub(zero),
			ExpectedOperator: a.Operator,
			ExpectedParent:   []*Expression{},
			Explainer:        "When substracting zero, this should be simplified into a no-op",
		},
		{
			Actual:           a.Sub(a),
			ExpectedOperator: zero.Operator,
			ExpectedParent:   []*Expression{},
			Explainer:        "When substracting a with itself, this should be simplified into zero",
		},
		{
			Actual:           a.Sub(one).Sub(one),
			ExpectedOperator: LinComb{Coeffs: []int{1, 1}},
			ExpectedParent:   []*Expression{a, NewConstant(-2)},
			Explainer:        "When substracting twice by a constant, this should be simplified into LinComb with only `1` as coeffs",
		},
		{
			Actual:           a.Sub(b).Sub(c),
			ExpectedOperator: LinComb{Coeffs: []int{1, -1, -1}},
			ExpectedParent:   []*Expression{a, b, c},
			Explainer:        "When substracting a with b then c, this should be regrouped into a single linear combination",
		},
		{
			Actual:           a.Mul(b),
			ExpectedOperator: Product{Exponents: []int{1, 1}},
			ExpectedParent:   []*Expression{a, b},
			Explainer:        "Normally multiplying a with b should produce a product term",
		},
		{
			Actual:           a.Mul(zero),
			ExpectedOperator: zero.Operator,
			ExpectedParent:   []*Expression{},
			Explainer:        "Multiplying something by zero automatically returns the zero constant",
		},
		{
			Actual:           a.Mul(one),
			ExpectedOperator: a.Operator,
			ExpectedParent:   []*Expression{},
			Explainer:        "Multiplying something by 1 is a no-op and returns directly the multiplied expression",
		},
		{
			Actual:           a.Mul(a),
			ExpectedOperator: Product{Exponents: []int{2}},
			ExpectedParent:   []*Expression{a},
			Explainer:        "Multiplying a with itself produces a product with a single term with exponent 2",
		},
	}

	for i := range testCases {

		t.Run(testCases[i].Explainer, func(t *testing.T) {
			// This checks that the expression is well-formed and that the GenericFieldELem
			// is correctly computed.
			assert.NoError(t, testCases[i].Actual.Validate(), "the expression is not well-formed")
			assert.Equal(t, testCases[i].ExpectedOperator, testCases[i].Actual.Operator, "the operator does not match")
			//assert.Equal(t, testCases[i].ExpectedParent, testCases[i].ExpectedParent, "the expected parents do not match")
		})
	}

}

func TestImmutableConstructors(t *testing.T) {

	var (
		// Importantly, this implements Metadata but is not Variable. We will
		// be expecting that the tested functions perform the casting over them
		a    = StringVar("a")
		b    = StringVar("b")
		c    = StringVar("c")
		aVar = NewVariable(a)
		bVar = NewVariable(b)
		cVar = NewVariable(c)
		one  = NewConstant(1)
	)

	testCases := []struct {
		Actual    *Expression
		Expected  *Expression
		Explainer string
	}{
		{
			Actual:    Add(a, b, 1),
			Expected:  aVar.Add(bVar).Add(one),
			Explainer: "Adding two metadata and a constant",
		},
		{
			Actual:    Add(aVar, bVar, c),
			Expected:  aVar.Add(bVar).Add(cVar),
			Explainer: "Adding two expressions and a metadata",
		},
		{
			Actual:    Sub(a, b, 1),
			Expected:  aVar.Sub(bVar).Sub(one),
			Explainer: "Substracting two metadata and a constant",
		},
		{
			Actual:    Sub(aVar, bVar, c),
			Expected:  aVar.Sub(bVar).Sub(cVar),
			Explainer: "Substracting two expressions and a metadata",
		},
		{
			Actual:    Mul(a, b, 1),
			Expected:  aVar.Mul(bVar).Mul(one),
			Explainer: "Multiplying two metadata and a constant",
		},
		{
			Actual:    Mul(aVar, bVar, c),
			Expected:  aVar.Mul(bVar).Mul(cVar),
			Explainer: "Multiplying two expressions and a metadata",
		},
		{
			Actual:    Square(aVar),
			Expected:  aVar.Square(),
			Explainer: "Squaring an expression",
		},
		{
			Actual:    Square(a),
			Expected:  aVar.Square(),
			Explainer: "Squaring a metadata",
		},
		{
			Actual:    Square(1),
			Expected:  one.Square(),
			Explainer: "Squaring a constant",
		},
		{
			Actual:    Neg(aVar),
			Expected:  aVar.Neg(),
			Explainer: "Negating an expression",
		},
		{
			Actual:    Neg(a),
			Expected:  aVar.Neg(),
			Explainer: "Negating a metadata",
		},
		{
			Actual:    Neg(1),
			Expected:  one.Neg(),
			Explainer: "Negating a constant",
		},
		{
			Actual:    Pow(aVar, 4),
			Expected:  aVar.Pow(4),
			Explainer: "Exponentiating an expression",
		},
		{
			Actual:    Pow(a, 4),
			Expected:  aVar.Pow(4),
			Explainer: "Exponentiating a metadata",
		},
		{
			Actual:    Pow(1, 4),
			Expected:  one.Pow(4),
			Explainer: "Exponentiating a constant",
		},
	}

	for i := range testCases {
		t.Run(testCases[i].Explainer, func(t *testing.T) {
			assert.Equal(t, testCases[i].Expected, testCases[i].Actual)
		})
	}

}
