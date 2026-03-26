package symbolic

import (
	"fmt"
	"math/big"
	"reflect"

	"github.com/consensys/gnark/frontend"
	sv "github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors_mixed"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// LinComb is an [Operator] symbolizing linear combinations of expressions with
// constant coefficients. It is used under the hood to generically represents
// additions or expression of variables.
//
// For expression building it is advised to use the constructors instead of this
// directly. The exposition of this Operator is meant to allow implementing
// symbolic expression optimizations.
type LinComb struct {
	// The Coeffs are typically small integers (1, -1)
	Coeffs []int
}

// NewLinComb creates an expression representing the linear combination of a
// list of expressions and coefficients.
// the constructor operates a sequence of optimization routines: flattening,
// regroupment and removal of zeroes.
//
// If provided an empty list of items, the function returns the zero constant
// and if the number of items does not match the number of coeffs, the function
// panics.
//
// Note: the function is not guaranteed to return a LinComb object, since the
// optimization routine may detect that this simplifies into a single-term
// expression or a constant expression.
func NewLinComb(items []*Expression, coeffs []int) *Expression {

	if len(items) != len(coeffs) {
		panic("unmatching lengths")
	}

	items, coeffs = simplifyLinComb(items, coeffs)

	if len(items) == 0 {
		return NewConstant(0)
	}

	// The LinCombExt is just a single-term: more efficient to unwrap it
	if len(items) == 1 && coeffs[0] == 1 {
		return items[0]
	}

	return newLinCombNoSimplify(items, coeffs)
}

// newLinCombNoSimplify is the same as NewLinComb but does not perform any
// optimization.
func newLinCombNoSimplify(items []*Expression, coeffs []int) *Expression {
	e := &Expression{
		Operator: LinComb{Coeffs: coeffs},
		Children: items,
		IsBase:   computeIsBaseFromChildren(items),
	}

	// Now we need to assign the ESH
	var esh esHash
	var coeff field.Element

	for i := range e.Children {
		coeff.SetInt64(int64(coeffs[i]))
		esh.MulByElement(&e.Children[i].ESHash, &coeff)
		e.ESHash.Add(&e.ESHash, &esh)
	}

	return e
}

// Degree implements the [Operator] interface and returns the maximum degree of
// the underlying expression.
func (LinComb) Degree(inputDegrees []int) int {
	return utils.Max(inputDegrees...)
}

// Evaluate implements the [Operator] interface.
func (lc LinComb) Evaluate(inputs []sv.SmartVector) sv.SmartVector {
	return sv.LinComb(lc.Coeffs, inputs)
}

func (lc LinComb) EvaluateExt(inputs []sv.SmartVector) sv.SmartVector {
	return sv.LinCombExt(lc.Coeffs, inputs)
}

func (lc LinComb) EvaluateMixed(inputs []sv.SmartVector) sv.SmartVector {
	return smartvectors_mixed.LinCombMixed(lc.Coeffs, inputs)
}

// Validate implements the [Operator] interface
func (lc LinComb) Validate(expr *Expression) error {
	if !reflect.DeepEqual(lc, expr.Operator) {
		panic("expr.operator != lc")
	}

	if len(lc.Coeffs) != len(expr.Children) {
		return fmt.Errorf("mismatch in the size of the children and coefficients")
	}

	return nil
}

// GnarkEval implements the [GnarkEval] interface
func (lc LinComb) GnarkEval(api frontend.API, inputs []koalagnark.Element) koalagnark.Element {

	koalaAPI := koalagnark.NewAPI(api)

	res := koalagnark.NewElement(0)

	if len(inputs) != len(lc.Coeffs) {
		utils.Panic("%v inputs but %v coeffs", len(inputs), len(lc.Coeffs))
	}

	for i, input := range inputs {
		coeff := koalagnark.NewElement(lc.Coeffs[i])
		tmp := koalaAPI.Mul(coeff, input)
		res = koalaAPI.Add(res, tmp)
	}

	return res
}

// GnarkEval implements the [GnarkEvalExt] interface
func (lc LinComb) GnarkEvalExt(api frontend.API, inputs []any) koalagnark.Ext {

	koalaAPI := koalagnark.NewAPI(api)
	res := koalaAPI.ZeroExt()
	resBase := koalaAPI.Zero()
	countBase := 0

	if len(inputs) != len(lc.Coeffs) {
		utils.Panic("%v inputs but %v coeffs", len(inputs), len(lc.Coeffs))
	}

	// Optimization: use MulByFp instead of full E4 Mul since coeffs are base field elements
	c := big.NewInt(0)
	for i, input := range inputs {

		switch input := input.(type) {

		case koalagnark.Ext:
			switch coeff := lc.Coeffs[i]; coeff {
			case 0:
				// skip
			case 1:
				res = koalaAPI.AddExt(res, input)
			case -1:
				res = koalaAPI.SubExt(res, input)
			default:
				c.SetInt64(int64(coeff))
				tmp := koalaAPI.MulConstExt(input, c)
				res = koalaAPI.AddExt(tmp, res)
			}

		case koalagnark.Element:
			countBase++
			switch coeff := lc.Coeffs[i]; coeff {
			case 0:
				// skip
			case 1:
				resBase = koalaAPI.Add(resBase, input)
			case -1:
				resBase = koalaAPI.Sub(resBase, input)
			default:
				c.SetInt64(int64(coeff))
				tmp := koalaAPI.MulConst(input, c)
				resBase = koalaAPI.Add(tmp, resBase)
			}

		default:
			panic("unknown input type " + reflect.TypeOf(input).String())
		}
	}

	if countBase > 0 {
		res = koalaAPI.AddByBaseExt(res, resBase)
	}

	return res
}
