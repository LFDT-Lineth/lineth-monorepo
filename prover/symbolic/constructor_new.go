package symbolic

import (
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// Add constructs a symbolic expression representing the sum of its inputs.
// The caller can provide either a pointer to an [Expression] or a
// [symbolic.Metadata]. If a [Metadata] is provided, the function automatically
// converts it into a symbolic expression by instantiating the corresponding
// variable.
//
// If one of the provided inputs is `nil`, then the function will return `nil`.
// The function panics if no inputs are provided or if an unexpected type is
// provided. @alex: it could make sense to return zero when no inputs are
// provided actually.
func Add(inputs ...any) *Expression {

	if len(inputs) == 0 {
		panic("No inputs were provided")
	}

	if doesContainNil(inputs...) {
		return nil
	}

	exprInputs := intoExprSlice(inputs...)

	magnitudes := make([]int, len(exprInputs))
	for i := range exprInputs {
		magnitudes[i] = 1
	}

	return NewLinComb(exprInputs, magnitudes)
}

// Mul constructs a symbolic expression representing the product of its inputs.
// The caller can provide either a pointer to an [Expression] or a
// [symbolic.Metadata]. If a [Metadata] is provided, the function automatically
// converts it into a symbolic expression by instantiating the corresponding
// variable.
//
// If one of the provided inputs is `nil`, then the function will return `nil`.
// The function panics if no inputs are provided or if an unexpected type is
// provided. @alex: it could make sense to return 1 when no inputs are
// provided actually.
func Mul(inputs ...any) *Expression {

	if len(inputs) == 0 {
		panic("No inputs were provided")
	}

	if doesContainNil(inputs...) {
		return nil
	}

	exprInputs := intoExprSlice(inputs...)

	magnitudes := make([]int, len(exprInputs))
	for i := range exprInputs {
		magnitudes[i] = 1
	}

	return NewProduct(exprInputs, magnitudes)
}

// MulNoSimplify is as [Mul] but does not do any simplification.
func MulNoSimplify(inputs ...any) *Expression {

	exprInputs := intoExprSlice(inputs...)

	magnitudes := make([]int, len(exprInputs))
	for i := range exprInputs {
		magnitudes[i] = 1
	}

	return newProductNoSimplify(exprInputs, magnitudes)
}

// Sub returns a symbolic expression representing the subtraction of `a` by all
// the entries in the `bs` : a - bs[0] - bs[1] - bs[2] - ...
// The caller can provide either a pointer to an [Expression] or a
// [symbolic.Metadata]. If a [Metadata] is provided, the function automatically
// converts it into a symbolic expression by instantiating the corresponding
// variable.
//
// If one of the provided inputs is `nil`, then the function will return `nil`.
// The function panics if no inputs are provided or if an unexpected type is
// provided. @alex: it could make sense to return 'a' when `bs` is empty.
func Sub(a any, bs ...any) *Expression {

	if a == nil || doesContainNil(bs) {
		return nil
	}

	var (
		aExpr      = intoExpr(a)
		bExpr      = intoExprSlice(bs...)
		exprInputs = append([]*Expression{aExpr}, bExpr...)
		magnitudes = make([]int, len(exprInputs))
	)

	for i := range exprInputs {
		magnitudes[i] = -1
	}
	magnitudes[0] = 1

	return NewLinComb(exprInputs, magnitudes)
}

// Neg returns an expression representing the negation of an expression or of
// a [Variable] or of a [Constant]. The caller can provide a [Metadata] which
// will be converted into a [Variable] or a "field-like" object that will be
// converted into a [Constant]. If the caller provides `nil`, the function also
// returns nil. And any other case results in a panic.
func Neg(x any) *Expression {

	if x == nil {
		return nil
	}

	return intoExpr(x).Neg()
}

// Square returns an expression representing the squaring of an expression or of
// a [Variable] or of a [Constant]. The caller can provide a [Metadata] which
// will be converted into a [Variable] or a "field-like" object that will be
// converted into a [Constant]. If the caller provides `nil`, the function also
// returns nil. And any other case results in a panic.
func Square(x any) *Expression {

	if x == nil {
		return nil
	}

	return NewProduct([]*Expression{intoExpr(x)}, []int{2})
}

// Pow returns an expression representing the raising to the power "n" of an
// expression or of
// a [Variable] or of a [Constant]. The caller can provide a [Metadata] which
// will be converted into a [Variable] or a "field-like" object that will be
// converted into a [Constant]. If the caller provides `nil`, the function also
// returns nil. And any other case results in a panic.
//
// Additionally, the function requires that the provided power to be non-negative.
// If the user, provides `0`, then the function returns 1 and if 1 is provided,
// the function the input converted into an Expression.
func Pow(x any, n int) *Expression {

	if n < 0 {
		utils.Panic("Pow cannot accept negative exponent")
	}

	if x == nil {
		return nil
	}

	return intoExpr(x).Pow(n)

}

// This function takes an array of entries that can be of type either
//   - a *Expression
//   - a Metadata
//   - a field convertible into a field.Element
//
// and converts it into an array of *Expression, by instantiating all the
// [Variable] from the metadatas or a [Constant] if the input is field like.
// It will panic if another type is found or if the conversion fails.
func intoExprSlice(inputs ...any) []*Expression {
	exprInputs := make([]*Expression, len(inputs))
	for i := range inputs {
		exprInputs[i] = intoExpr(inputs[i])
	}
	return exprInputs
}

// This function takes either a *Expression or a Metadata or a field-like input.
// If it is an expression the original expression is returned, if it is a
// Metadata, the function instantiates a [Variable] from the metadata or a
// [Constant] if the input is field like. It will panic if another type is found
// or if the conversion fails.
func intoExpr(input any) *Expression {
	switch inp := input.(type) {
	case *Expression:
		return inp
	case Metadata:
		return NewVariable(inp)
	case int, uint, int64, uint64, int32, uint32, string, field.Element, fext.Element:
		return NewConstant(inp)
	default:
		utils.Panic("expected either a *Expression or a Metadata, but got %T", inp)
	}

	panic("unreachable")
}

// This function returns whether the array contains nil
func doesContainNil(inputs ...any) bool {
	for i := range inputs {
		if inputs[i] == nil {
			return true
		}
	}
	return false
}
