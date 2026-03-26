package symbolic

import (
	"reflect"
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"

	"github.com/stretchr/testify/assert"
)

func TestConstructorExt(t *testing.T) {

	var (
		a    = NewDummyVarExt("a")
		b    = NewDummyVarExt("b")
		c    = NewDummyVarExt("c")
		zero = NewConstant(fext.NewFromUint(0, 0, 0, 0))
		one  = NewConstant(fext.NewFromUint(1, 0, 0, 0))
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
			Actual:           Sub(a, b),
			ExpectedOperator: LinComb{Coeffs: []int{1, -1}},
			ExpectedParent:   []*Expression{a, b},
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
			assert.Equal(t, testCases[i].ExpectedParent, testCases[i].ExpectedParent, "the expected parents do not match")
		})
	}

}

func TestImmutableConstructorsExt(t *testing.T) {

	var (
		// Importantly, this implements Metadata but is not Variable. We will
		// be expecting that the tested functions perform the casting over them
		a    = StringVarExt("a")
		b    = StringVarExt("b")
		c    = StringVarExt("c")
		aVar = NewVariable(a)
		bVar = NewVariable(b)
		cVar = NewVariable(c)
		one  = NewConstant(fext.NewFromUint(1, 0, 0, 0))
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

func TestConstructorMixed(t *testing.T) {

	var (
		a = NewDummyVar("a")
		b = NewDummyVar("b")
		c = NewDummyVar("c")
		// extension variables
		extA = NewDummyVarExt("extA")
		extB = NewDummyVar("extB")
		extC = NewDummyVar("extC")
		// constants
		zero    = NewConstant(fext.NewFromUint(0, 0, 0, 0))
		one     = NewConstant(fext.NewFromUint(1, 0, 0, 0))
		oneBase = NewConstant(field.NewElement(1))
	)

	testCases := []struct {
		Actual           *Expression
		ExpectedOperator Operator
		ExpectedChildren []*Expression
		Explainer        string
	}{
		{
			Actual:           Add(a, b, extA, extB),
			ExpectedOperator: LinComb{Coeffs: []int{1, 1, 1, 1}},
			ExpectedChildren: []*Expression{a, b, extA, extB},
			Explainer:        "Normally adding two variables",
		},
		{
			Actual:           a.Add(zero),
			ExpectedOperator: a.Operator,
			ExpectedChildren: []*Expression{},
			Explainer:        "When adding zero, this should be a no-op",
		},
		{
			Actual:           a.Add(a),
			ExpectedOperator: LinComb{Coeffs: []int{2}},
			ExpectedChildren: []*Expression{a},
			Explainer:        "When adding twice the same variable, this should resolve in a lincomb with a single term and a coefficient of 2",
		},
		{
			Actual:           a.Add(one).Add(one),
			ExpectedOperator: LinComb{Coeffs: []int{1, 1}},
			ExpectedChildren: []*Expression{a, NewConstant(2)},
			Explainer:        "When adding twice a constant, the constants should be regrouped",
		},
		{
			Actual:           Add(a, b, c, extA, extB, extC),
			ExpectedOperator: LinComb{Coeffs: []int{1, 1, 1, 1, 1, 1}},
			ExpectedChildren: []*Expression{a, b, c, extA, extB, extC},
			Explainer:        "LinComb should be regrouped by associativity",
		},
		{
			Actual:           Sub(a, b, extA, extB),
			ExpectedOperator: LinComb{Coeffs: []int{1, -1, -1, -1}},
			ExpectedChildren: []*Expression{a, b, extA, extB},
			Explainer:        "Normally substracting two variables should create a LinComb object",
		},
		{
			Actual:           a.Sub(zero),
			ExpectedOperator: a.Operator,
			ExpectedChildren: []*Expression{},
			Explainer:        "When substracting zero, this should be simplified into a no-op",
		},
		{
			Actual:           a.Sub(a),
			ExpectedOperator: zero.Operator,
			ExpectedChildren: []*Expression{},
			Explainer:        "When substracting a with itself, this should be simplified into zero",
		},
		{
			Actual:           a.Sub(one).Sub(one),
			ExpectedOperator: LinComb{Coeffs: []int{1, 1}},
			ExpectedChildren: []*Expression{a, NewConstant(-2)},
			Explainer:        "When substracting twice by a constant, this should be simplified into LinComb with only `1` as coeffs",
		},
		{
			Actual:           Sub(a, b, c, extA, extB, extC),
			ExpectedOperator: LinComb{Coeffs: []int{1, -1, -1, -1, -1, -1}},
			ExpectedChildren: []*Expression{a, b, c, extA, extB, extC},
			Explainer:        "When substracting a with b then c, this should be regrouped into a single linear combination",
		},
		{
			Actual:           Mul(a, b, extA, extB),
			ExpectedOperator: Product{Exponents: []int{1, 1, 1, 1}},
			ExpectedChildren: []*Expression{a, b, extA, extB},
			Explainer:        "Normally multiplying a with b should produce a product term",
		},
		{
			Actual:           Mul(a, extA, zero),
			ExpectedOperator: zero.Operator,
			ExpectedChildren: []*Expression{},
			Explainer:        "Multiplying something by zero automatically returns the zero constant",
		},
		{
			Actual:           Mul(a, one, oneBase),
			ExpectedOperator: a.Operator,
			ExpectedChildren: []*Expression{},
			Explainer:        "Multiplying something by 1 is a no-op and returns directly the multiplied expression",
		},
		{
			Actual:           Mul(a, a),
			ExpectedOperator: Product{Exponents: []int{2}},
			ExpectedChildren: []*Expression{a},
			Explainer:        "Multiplying a with itself produces a product with a single term with exponent 2",
		},
	}

	for i := range testCases {

		t.Run(testCases[i].Explainer, func(t *testing.T) {
			// This checks that the expression is well-formed and that the GenericFieldELem
			// is correctly computed.
			assert.NoError(t, testCases[i].Actual.Validate(), "the expression is not well-formed")
			assert.Equal(t, testCases[i].ExpectedOperator, testCases[i].Actual.Operator, "the operator does not match")
			assert.Equal(t, true, reflect.DeepEqual(testCases[i].ExpectedChildren, testCases[i].Actual.Children), "the expected children do not match")
		})
	}

}

func TestImmutableConstructorsMixed(t *testing.T) {

	var (
		// Importantly, this implements Metadata but is not Variable. We will
		// be expecting that the tested functions perform the casting over them
		a = StringVar("a")
		b = StringVar("b")
		c = StringVar("c")
		// extension variables
		extA = StringVarExt("a")
		extB = StringVarExt("b")
		extC = StringVarExt("c")
		// immutable versions of the base variables
		aVar = NewVariable(a)
		bVar = NewVariable(b)
		cVar = NewVariable(c)
		// immutable versions of the extension variables
		aExtVar = NewVariable(extA)
		bExtVar = NewVariable(extB)
		cExtVar = NewVariable(extC)
		// constants
		one     = NewConstant(fext.NewFromUint(1, 0, 0, 0))
		oneBase = NewConstant(field.NewElement(1))
	)

	testCases := []struct {
		Actual    *Expression
		Expected  *Expression
		Explainer string
	}{
		{
			Actual:    Add(a, b, 1, extA, extB),
			Expected:  Add(aVar, bVar, one, aExtVar, bExtVar),
			Explainer: "Adding two metadata and a constant",
		},
		{
			Actual:    Add(aVar, bVar, c, aExtVar, bExtVar, extC),
			Expected:  Add(aVar, bVar, cVar, aExtVar, bExtVar, cExtVar),
			Explainer: "Adding two expressions and a metadata",
		},
		{
			Actual:    Sub(a, b, 1, extA, extB),
			Expected:  Sub(aVar, bVar, one, aExtVar, bExtVar),
			Explainer: "Substracting two metadata and a constant",
		},
		{
			Actual:    Sub(aVar, bVar, c, aExtVar, bExtVar, extC),
			Expected:  Sub(aVar, bVar, cVar, aExtVar, bExtVar, cExtVar),
			Explainer: "Substracting two expressions and a metadata",
		},
		{
			Actual:    Mul(a, b, 1, extA, extB),
			Expected:  Mul(aVar, bVar, one, aExtVar, bExtVar, oneBase),
			Explainer: "Multiplying two metadata and a constant",
		},
		{
			Actual:    Mul(aVar, bVar, c, aExtVar, bExtVar, extC),
			Expected:  Mul(aVar, bVar, cVar, aExtVar, bExtVar, cExtVar),
			Explainer: "Multiplying two expressions and a metadata",
		},
	}

	for i := range testCases {
		t.Run(testCases[i].Explainer, func(t *testing.T) {
			assert.Equal(t, testCases[i].Expected, testCases[i].Actual)
		})
	}

}
