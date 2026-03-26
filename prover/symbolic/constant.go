package symbolic

import (
	"fmt"
	"reflect"

	"github.com/consensys/gnark/frontend"
	sv "github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
)

// Constant is an implementation of [Operator] which represents a constant value
type Constant struct {
	// TODO @gbotrel separate constant ext and no-ext
	Val fext.GenericFieldElem
}

// Degree implements the [Operator] interface
func (Constant) Degree([]int) int {
	panic("we never call it for a constant")
}

// Evaluates implements the [Operator] interface
func (c Constant) Evaluate([]sv.SmartVector) sv.SmartVector {
	panic("we never call it for a constant")
}

func (c Constant) EvaluateExt([]sv.SmartVector) sv.SmartVector {
	panic("we never call EvaluateExt for a constant")
}

func (c Constant) EvaluateMixed([]sv.SmartVector) sv.SmartVector {
	panic("we never call EvaluateMixed for a constant")
}

// GnarkEval implements the [Operator] interface.
func (c Constant) GnarkEval(api frontend.API, inputs []koalagnark.Element) koalagnark.Element {
	panic("we never call it for a constant")
}

// GnarkEvalExt implements the [Operator] interface.
func (c Constant) GnarkEvalExt(api frontend.API, inputs []any) koalagnark.Ext {
	panic("we never call it for a constant")
}

// NewConstant creates a new [Constant]. The function admits any input types
// that is either: field.Element, int, uint or decimal string.
func NewConstant(val interface{}) *Expression {
	var x fext.Element
	if _, err := fext.SetInterface(&x, val); err != nil {
		panic(err)
	}

	newHash := fext.NewMinimalESHashFromExt(&x)
	//Create the expression
	res := &Expression{
		Operator: Constant{Val: *newHash},
		Children: []*Expression{},
		ESHash:   x,
		IsBase:   fext.IsBase(&x),
	}
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
