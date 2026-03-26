package symbolic

import (
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
)

/*
IsMul returns true if an expression is a mul at the top-level
*/
func (e *Expression) IsMul() bool {
	_, ok := e.Operator.(Product)
	return ok
}

/*
IsConstant returns true if it is a constant and return its value if so
*/
func (e *Expression) IsConstant() (bool, fext.GenericFieldElem) {
	if cons, ok := e.Operator.(Constant); ok {
		return ok, cons.Val
	}
	return false, fext.GenericFieldZero()
}

/*
IsLC returns true if the expression is a LC
*/
func (e *Expression) IsLC() bool {
	_, ok := e.Operator.(LinComb)
	return ok
}

/*
IsVariable returns true if the expression is a variable
*/
func (e *Expression) IsVariable() bool {
	_, ok := e.Operator.(Variable)
	return ok
}
