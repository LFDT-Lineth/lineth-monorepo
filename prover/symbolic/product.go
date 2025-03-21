package symbolic

import (
	"errors"
	"fmt"
	"math/big"
	"reflect"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/maths/common/mempool"
	sv "github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/gnarkutil"
)

var (
	ErrTermIsProduct    error = errors.New("term is product")
	ErrTermIsNotProduct error = errors.New("term is not product")
)

// Product is an implementation of the [Operator] interface and represents a
// product of terms with exponents.
//
// For expression building it is advised to use the constructors instead of this
// directly. The exposition of this Operator is meant to allow implementing
// symbolic expression optimizations.
//
// Note that the library does not support exponents with negative values and
// such expressions would be treated as invalid and yield panic errors if the
// package sees it.
type Product struct {
	// Exponents for each term in the multiplication
	Exponents []int
}

// NewProduct returns an expression representing a product of items applying
// exponents to them. The newly constructed expression is subjected to basic
// optimizations routines: detection of zero factors, expansion by
// associativity, regroupment of terms and removal or terms with a coefficient
// of zero.
//
// Thus, the returned expression is not guaranteed to be of type [Product]. To
// actually multiply or exponentiate [Expression] objects, the user is advised
// to use [Mul] [Square] or [Pow] instead.
//
// If provided an empty list of items/exponents the function returns 1 as a
// default value and if the lengths of the two parameters do not match, the
// function panics.
func NewProduct(items []*Expression, exponents []int) *Expression {

	if len(items) != len(exponents) {
		panic("unmatching lengths")
	}

	for i := range exponents {
		if exponents[i] < 0 {
			panic("negative exponents are not allowed")
		}
	}

	for i := range items {
		if items[i].ESHash.IsZero() && exponents[i] != 0 {
			return NewConstant(0)
		}
	}

	exponents, items = expandTerms(&Product{}, exponents, items)
	exponents, items, constExponents, constVal := regroupTerms(exponents, items)

	// This regroups all the constants into a global constant with a coefficient
	// of 1.
	var c, t field.Element
	c.SetOne()
	for i := range constExponents {
		t.Exp(constVal[i], big.NewInt(int64(constExponents[i])))
		c.Mul(&c, &t)
	}

	if !c.IsOne() {
		exponents = append(exponents, 1)
		items = append(items, NewConstant(c))
	}

	exponents, items = removeZeroCoeffs(exponents, items)

	if len(items) == 0 {
		return NewConstant(1)
	}

	if len(items) == 1 && exponents[0] == 1 {
		return items[0]
	}

	e := &Expression{
		Operator: Product{Exponents: exponents},
		Children: items,
		ESHash:   field.One(),
	}

	for i := range e.Children {
		var tmp field.Element
		switch {
		case exponents[i] == 1:
			e.ESHash.Mul(&e.ESHash, &e.Children[i].ESHash)
		case exponents[i] == 2:
			tmp.Square(&e.Children[i].ESHash)
			e.ESHash.Mul(&e.ESHash, &tmp)
		case exponents[i] == 3:
			tmp.Square(&e.Children[i].ESHash)
			tmp.Mul(&tmp, &e.Children[i].ESHash)
			e.ESHash.Mul(&e.ESHash, &tmp)
		case exponents[i] == 4:
			tmp.Square(&e.Children[i].ESHash)
			tmp.Square(&tmp)
			e.ESHash.Mul(&e.ESHash, &tmp)
		default:
			exponent := big.NewInt(int64(exponents[i]))
			tmp.Exp(e.Children[i].ESHash, exponent)
			e.ESHash.Mul(&e.ESHash, &tmp)
		}
	}

	return e
}

// Degree implements the [Operator] interface and returns the sum of the degree
// of all the operands weighted by the exponents.
func (prod Product) Degree(inputDegrees []int) int {
	res := 0
	// Just the sum of all the degrees
	for i, exp := range prod.Exponents {
		res += exp * inputDegrees[i]
	}
	return res
}

// Evaluate implements the [Operator] interface.
func (prod Product) Evaluate(inputs []sv.SmartVector, p ...mempool.MemPool) sv.SmartVector {
	return sv.Product(prod.Exponents, inputs, p...)
}

// Validate implements the [Operator] interface.
func (prod Product) Validate(expr *Expression) error {
	if !reflect.DeepEqual(prod, expr.Operator) {
		panic("expr.operator != prod")
	}

	if len(prod.Exponents) != len(expr.Children) {
		return fmt.Errorf("mismatch in the size of the children and coefficients")
	}

	for _, e := range prod.Exponents {
		if e < 0 {
			panic("found a negative exponent")
		}
	}

	return nil
}

// GnarkEval implements the [Operator] interface.
func (prod Product) GnarkEval(api frontend.API, inputs []frontend.Variable) frontend.Variable {

	res := frontend.Variable(1)

	// There should be as many inputs as there are coeffs
	if len(inputs) != len(prod.Exponents) {
		utils.Panic("%v inputs but %v coeffs", len(inputs), len(prod.Exponents))
	}

	/*
		Accumulate the scalars
	*/
	for i, input := range inputs {
		term := gnarkutil.Exp(api, input, prod.Exponents[i])
		res = api.Mul(res, term)
	}

	return res
}
