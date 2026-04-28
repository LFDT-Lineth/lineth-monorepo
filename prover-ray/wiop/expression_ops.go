package wiop

// This file provides the package-level expression constructors. Each function
// is immutable: it allocates a new [ArithmeticOperation] node and returns it
// as an [Expression]. Callers compose these freely without any side-effects.

// Constructor is the visitor function used by [EditExpression]. It is called
// for every node in the expression tree in bottom-up order. curr is the
// original node; newChildren holds the already-transformed children (nil for
// leaf nodes). The returned expression replaces curr in the rebuilt tree.
type Constructor func(curr Expression, newChildren []Expression) Expression

// EditExpression walks the expression tree rooted at expr in bottom-up order
// and rebuilds it by calling constructor at each node. The input expression is
// not modified.
//
// For leaf nodes ([*Constant], [*ColumnView], [*ColumnPosition], [*Cell],
// [*CoinField], and any unknown [Expression] implementation) newChildren is
// nil. For [*ArithmeticOperation] nodes newChildren contains the
// already-transformed operands in the same order as [ArithmeticOperation.Operands].
//
// Use [DefaultConstruct] inside a constructor to delegate the default
// reconstruction for nodes you do not need to rewrite:
//
//	EditExpression(expr, func(curr Expression, newChildren []Expression) Expression {
//	    if cv, ok := curr.(*ColumnView); ok {
//	        return replacement(cv)
//	    }
//	    return DefaultConstruct(curr, newChildren)
//	})
func EditExpression(expr Expression, constructor Constructor) Expression {
	ao, ok := expr.(*ArithmeticOperation)
	if !ok {
		return constructor(expr, nil)
	}
	newChildren := make([]Expression, len(ao.Operands))
	for i, operand := range ao.Operands {
		newChildren[i] = EditExpression(operand, constructor)
	}
	return constructor(ao, newChildren)
}

// DefaultConstruct is the identity [Constructor]: it returns a new
// [ArithmeticOperation] with the same operator and newChildren as operands, or
// returns curr unchanged for leaf nodes.
func DefaultConstruct(curr Expression, newChildren []Expression) Expression {
	ao, ok := curr.(*ArithmeticOperation)
	if !ok {
		return curr
	}
	return NewArithmeticOperation(ao.Operator, newChildren...)
}

// Add returns an expression that computes a + b.
func Add(a, b Expression) Expression {
	return NewArithmeticOperation(ArithmeticOperatorAdd, a, b)
}

// Sub returns an expression that computes a - b.
func Sub(a, b Expression) Expression {
	return NewArithmeticOperation(ArithmeticOperatorSub, a, b)
}

// Mul returns an expression that computes a * b.
func Mul(a, b Expression) Expression {
	return NewArithmeticOperation(ArithmeticOperatorMul, a, b)
}

// Div returns an expression that computes a / b. Note that calling Degree()
// on the returned expression will panic because division is not a polynomial
// operation.
func Div(a, b Expression) Expression {
	return NewArithmeticOperation(ArithmeticOperatorDiv, a, b)
}

// Double returns an expression that computes 2 * a.
func Double(a Expression) Expression {
	return NewArithmeticOperation(ArithmeticOperatorDouble, a)
}

// Square returns an expression that computes a * a.
func Square(a Expression) Expression {
	return NewArithmeticOperation(ArithmeticOperatorSquare, a)
}

// Negate returns an expression that computes -a.
func Negate(a Expression) Expression {
	return NewArithmeticOperation(ArithmeticOperatorNegate, a)
}

// Inverse returns an expression that computes 1/a. Note that calling Degree()
// on the returned expression will panic because inversion is not a polynomial
// operation.
func Inverse(a Expression) Expression {
	return NewArithmeticOperation(ArithmeticOperatorInverse, a)
}

// Sum returns an expression that computes the sum of all terms. It folds the
// terms left-to-right into a binary Add tree: Sum(a, b, c) = Add(Add(a, b), c).
//
// Panics if terms is empty.
func Sum(terms ...Expression) Expression {
	if len(terms) == 0 {
		panic("wiop: Sum requires at least one term")
	}
	result := terms[0]
	for _, t := range terms[1:] {
		result = Add(result, t)
	}
	return result
}

// Product returns an expression that computes the product of all factors. It
// folds the factors left-to-right into a binary Mul tree:
// Product(a, b, c) = Mul(Mul(a, b), c).
//
// Panics if factors is empty.
func Product(factors ...Expression) Expression {
	if len(factors) == 0 {
		panic("wiop: Product requires at least one factor")
	}
	result := factors[0]
	for _, f := range factors[1:] {
		result = Mul(result, f)
	}
	return result
}
