package symbolic

import (
	"fmt"
	"reflect"

	"github.com/consensys/gnark/frontend"
	sv "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
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

	coeffs, items = expandTerms(&LinComb{}, coeffs, items)
	coeffs, items, constCoeffs, constVal := regroupTerms(coeffs, items)

	// This regroups all the constants into a global constant with a coefficient
	// of 1.
	var c, t field.Element
	for i := range constCoeffs {
		t.SetInt64(int64(constCoeffs[i]))
		t.Mul(&constVal[i], &t)
		c.Add(&c, &t)
	}

	if !c.IsZero() {
		coeffs = append(coeffs, 1)
		items = append(items, NewConstant(c))
	}

	coeffs, items = removeZeroCoeffs(coeffs, items)

	if len(items) == 0 {
		return NewConstant(0)
	}

	// The LinComb is just a single-term: more efficient to unwrap it
	if len(items) == 1 && coeffs[0] == 1 {
		return items[0]
	}

	e := &Expression{
		Operator: LinComb{Coeffs: coeffs},
		Children: items,
	}

	// Now we need to assign the ESH
	eshashes := make([]sv.SmartVector, len(e.Children))
	for i := range e.Children {
		eshashes[i] = sv.NewConstant(e.Children[i].ESHash, 1)
	}

	if len(items) > 0 {
		// The cast back to sv.Constant is not functionally important but is an easy
		// sanity check.
		e.ESHash = e.Operator.Evaluate(eshashes).(*sv.Constant).Get(0)
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
func (lc LinComb) GnarkEval(api frontend.API, inputs []frontend.Variable) frontend.Variable {

	res := frontend.Variable(0)

	// There should be as many inputs as there are coeffs
	if len(inputs) != len(lc.Coeffs) {
		utils.Panic("%v inputs but %v coeffs", len(inputs), len(lc.Coeffs))
	}

	/*
		Accumulate the scalars
	*/
	for i, input := range inputs {
		coeff := frontend.Variable(lc.Coeffs[i])
		res = api.Add(res, api.Mul(coeff, input))
	}

	return res
}
