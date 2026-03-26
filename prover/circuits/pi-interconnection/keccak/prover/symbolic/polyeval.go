package symbolic

import (
	"fmt"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/poly"
	sv "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

/*
	Dedicated type of expression for evaluating a polynomial
	whose coefficients are sub-expressions. It is use for batching
	multiple evaluation constraints into a single check using the
	Schwartz-Zippel lemma
*/

type PolyEval struct{}

func NewPolyEval(x *Expression, coeffs []*Expression) *Expression {

	/*
		No coeff is unexpected. If this panics on a legit use-case,
		feel free to return the constant zero instead.
	*/
	if len(coeffs) == 0 {
		utils.Panic("Polynomial with no coeffs")
	}

	/*
		Only one coeff. In this case no need to use `x`. We just return
		the original expression
	*/
	if len(coeffs) == 1 {
		return coeffs[0]
	}

	eshashes := []field.Element{}
	for i := range coeffs {
		eshashes = append(eshashes, coeffs[i].ESHash)
	}

	esh := poly.EvalUnivariate(eshashes, x.ESHash)

	return &Expression{
		Operator: PolyEval{},
		Children: append([]*Expression{x}, coeffs...),
		ESHash:   esh,
	}
}

/*
Returns the degree of the operation given, as input, the degree of the children
*/
func (PolyEval) Degree(inputDegrees []int) int {
	return utils.Max(inputDegrees...)
}

/*
Evaluates a polynomial evaluation
*/
func (PolyEval) Evaluate(inputs []sv.SmartVector) sv.SmartVector {
	// We assume that the first element is always a scalar
	// Get the constant value. We use Get(0) to get the value, but any integer would
	// also work provided it is also in range. 0 ensures that.
	x := inputs[0].(*sv.Constant).Get(0)
	return sv.PolyEval(inputs[1:], x)
}

/*
Validates that the LC is well-formed
*/
func (PolyEval) Validate(expr *Expression) error {
	if len(expr.Children) < 2 {
		return fmt.Errorf("poly eval of degree 0")
	}
	return nil
}

/*
Evaluate the expression in a gnark circuit
Does not support vector evaluation
*/
func (PolyEval) GnarkEval(api frontend.API, inputs []frontend.Variable) frontend.Variable {
	/*
		We use the Horner method
	*/
	x := inputs[0]
	res := inputs[len(inputs)-1]

	for i := len(inputs) - 2; i >= 1; i-- {
		res = api.Mul(res, x)
		c := inputs[i]
		res = api.Add(res, c)
	}

	return res
}
