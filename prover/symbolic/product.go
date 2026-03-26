package symbolic

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/consensys/gnark/frontend"
	sv "github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors_mixed"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
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

	items, exponents = simplifyProduct(items, exponents)

	if len(items) == 0 {
		return NewConstant(1)
	}

	if len(items) == 1 && exponents[0] == 1 {
		return items[0]
	}

	return newProductNoSimplify(items, exponents)
}

// newProductNoSimplify is the same as NewProduct but does not perform any
// optimization.
func newProductNoSimplify(items []*Expression, exponents []int) *Expression {
	e := &Expression{
		Operator: Product{Exponents: exponents},
		Children: items,
		ESHash:   fext.One(),
		IsBase:   computeIsBaseFromChildren(items),
	}

	var tmp fext.Element
	for i := range e.Children {
		tmp.ExpInt64(e.Children[i].ESHash, int64(exponents[i]))
		e.ESHash.Mul(&e.ESHash, &tmp)
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
func (prod Product) Evaluate(inputs []sv.SmartVector) sv.SmartVector {
	return sv.Product(prod.Exponents, inputs)
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
func (prod Product) GnarkEval(api frontend.API, inputs []koalagnark.Element) koalagnark.Element {

	res := koalagnark.NewElement(1)

	koalaAPI := koalagnark.NewAPI(api)

	if len(inputs) != len(prod.Exponents) {
		utils.Panic("%v inputs but %v coeffs", len(inputs), len(prod.Exponents))
	}

	for i, input := range inputs {
		exp := prod.Exponents[i]
		var term koalagnark.Element

		// Optimization: handle common exponents directly to avoid Exp overhead
		switch exp {
		case 0:
			// x^0 = 1, skip multiplication
			continue
		case 1:
			term = input
		case 2:
			term = koalaAPI.Mul(input, input)
		default:
			term = gnarkutil.Exp(api, input, exp)
		}
		res = koalaAPI.Mul(res, term)
	}

	return res
}

// GnarkEvalExt implements the [Operator] interface.
func (prod Product) GnarkEvalExt(api frontend.API, inputs []any) koalagnark.Ext {

	koalaAPI := koalagnark.NewAPI(api)

	var (
		res       = koalaAPI.OneExt()
		resBase   = koalaAPI.One()
		countBase = 0
	)

	if len(inputs) != len(prod.Exponents) {
		utils.Panic("%v inputs but %v coeffs", len(inputs), len(prod.Exponents))
	}

	for i, input := range inputs {

		switch input := input.(type) {
		case koalagnark.Ext:
			// nothing to do
			exp := prod.Exponents[i]
			var term koalagnark.Ext
			// Optimization: handle common exponents directly to avoid ExpExt overhead
			switch exp {
			case 0:
				continue // x^0 = 1, skip multiplication
			case 1:
				term = input // mostly this case
			case 2:
				term = koalaAPI.SquareExt(input)
			default:
				term = gnarkutil.ExpExt(api, input, exp)
			}
			res = koalaAPI.MulExt(res, term)

		case koalagnark.Element:
			// nothing to do
			countBase++
			exp := prod.Exponents[i]
			var term koalagnark.Element
			// Optimization: handle common exponents directly to avoid ExpExt overhead
			switch exp {
			case 0:
				continue // x^0 = 1, skip multiplication
			case 1:
				term = input // mostly this case
			case 2:
				term = koalaAPI.Mul(input, input)
			default:
				term = gnarkutil.Exp(api, input, exp)
			}
			resBase = koalaAPI.Mul(resBase, term)

		default:
			panic("unknown input type " + reflect.TypeOf(input).String())
		}
	}

	if countBase > 0 {
		res = koalaAPI.MulByFpExt(res, resBase)
	}

	return res
}

func (prod Product) EvaluateExt(inputs []sv.SmartVector) sv.SmartVector {
	return sv.ProductExt(prod.Exponents, inputs)
}

func (prod Product) EvaluateMixed(inputs []sv.SmartVector) sv.SmartVector {
	return smartvectors_mixed.ProductMixed(prod.Exponents, inputs)
}
