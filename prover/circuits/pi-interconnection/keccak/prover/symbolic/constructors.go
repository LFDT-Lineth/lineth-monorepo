package symbolic

// Get the expression obtained by adding two expressions together. The function
// some elementary simplification routines such as merging the LinComb nodes or
// collapsing the constants.
//
// Deprecated: Use [Add] instead. In the future, we will make this function
// completely internal.
func (e *Expression) Add(other *Expression) *Expression {

	// This happens if the zkEVM returns a poisoned variable and
	// this variable gets used in an expression in either a global
	// or a local constraint. In this case, we reject when the global
	// constraint is instantiated (e.g., after the expression if built).
	// As a consequence, we need to build "poisoned expressions" (i.e.,
	// a nil pointer). This will be caught later on.
	if e == nil || other == nil {
		return nil
	}

	return NewLinComb([]*Expression{e, other}, []int{1, 1})

}

// Get the expression obtained by multiplying two expressions together
//
// Deprecated: Use [Mul] instead as this function will be made private.
func (e *Expression) Mul(other *Expression) *Expression {

	// This happens if the zkEVM returns a poisoned variable and
	// this variable gets used in an expression in either a global
	// or a local constraint. In this case, we reject when the global
	// constraint is instantiated (e.g., after the expression if built).
	// As a consequence, we need to build "poisoned expressions" (i.e.,
	// a nil pointer). This will be caught later on.
	if e == nil || other == nil {
		return nil
	}

	return NewProduct([]*Expression{e, other}, []int{1, 1})
}

// Returns a negation of expr
//
// Deprecated: Use [Neg] instead of this function as it will be made private.
func (e *Expression) Neg() *Expression {

	// This happens if the zkEVM returns a poisoned variable and
	// this variable gets used in an expression in either a global
	// or a local constraint. In this case, we reject when the global
	// constraint is instantiated (e.g., after the expression if built).
	// As a consequence, we need to build "poisoned expressions" (i.e.,
	// a nil pointer). This will be caught later on.
	if e == nil {
		return nil
	}

	// Otherwise, wrap it in a singleTerm LC
	return NewLinComb([]*Expression{e}, []int{-1})
}

/*
Substract two expressions

Deprecated: Use [Sub] instead of this function as it will be made private.
*/
func (e *Expression) Sub(other *Expression) *Expression {

	// This happens if the zkEVM returns a poisoned variable and
	// this variable gets used in an expression in either a global
	// or a local constraint. In this case, we reject when the global
	// constraint is instantiated (e.g., after the expression if built).
	// As a consequence, we need to build "poisoned expressions" (i.e.,
	// a nil pointer). This will be caught later on.
	if e == nil || other == nil {
		return nil
	}

	return NewLinComb([]*Expression{e, other}, []int{1, -1})
}

/*
Square an expression

Deprecated: Use [Square] instead of this function as it will be made private.
*/
func (e *Expression) Square() *Expression {

	// This happens if the zkEVM returns a poisoned variable and
	// this variable gets used in an expression in either a global
	// or a local constraint. In this case, we reject when the global
	// constraint is instantiated (e.g., after the expression if built).
	// As a consequence, we need to build "poisoned expressions" (i.e.,
	// a nil pointer). This will be caught later on.
	if e == nil {
		return nil
	}

	return NewProduct([]*Expression{e}, []int{2})
}

/*
Square an expression

// Deprecated: Use [Pow] instead of this function as it will be made private.
*/
func (e *Expression) Pow(n int) *Expression {

	if e == nil {
		return nil
	}

	e.AssertValid()

	return NewProduct([]*Expression{e}, []int{n})
}
