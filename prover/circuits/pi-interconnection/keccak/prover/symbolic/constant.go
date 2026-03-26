package symbolic

import (
	"fmt"
	"math/big"
	"reflect"

	"github.com/consensys/gnark/frontend"
	sv "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
)

// Constant is an implementation of [Operator] which represents a constant value
type Constant struct {
	Val field.Element
}

// Degree implements the [Operator] interface
func (Constant) Degree([]int) int {
	panic("we never call it for a constant")
}

// Evaluates implements the [Operator] interface
func (c Constant) Evaluate([]sv.SmartVector) sv.SmartVector {
	panic("we never call it for a constant")
}

// GnarkEval implements the [Operator] interface.
func (c Constant) GnarkEval(api frontend.API, inputs []frontend.Variable) frontend.Variable {
	panic("we never call it for a constant")
}

// NewConstant creates a new [Constant]. The function admits any input types
// that is either: field.Element, int, uint or decimal string.
func NewConstant(val interface{}) *Expression {
	var x field.Element
	if _, err := x.SetInterface(val); err != nil {
		panic(err)
	}

	res := &Expression{
		Operator: Constant{Val: x},
		Children: []*Expression{},
		ESHash:   x,
	}

	// Pass the string and not the field.Element itself
	var sig big.Int
	res.ESHash.SetBigInt(x.BigInt(&sig))
	return res
}

/*
Validate implements the [Operator] interface.
*/
func (c Constant) Validate(expr *Expression) error {
	if !reflect.DeepEqual(c, expr.Operator) {
		panic("expr.operator != c")
	}

	if len(expr.Children) != 0 {
		return fmt.Errorf("mismatch in the size of the children and coefficients")
	}

	return nil
}
