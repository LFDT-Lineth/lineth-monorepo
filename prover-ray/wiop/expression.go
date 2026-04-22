package wiop

import (
	"fmt"
	"sync"

	field "github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
)

// Expression is the interface satisfied by all symbolic arithmetic expressions
// in the framework. An expression is a node in an expression AST that
// evaluates to either a single field element or a vector of field elements at
// runtime.
//
// Not all methods are valid on every implementation. Size, IsSized, and
// EvaluateVector may only be called when IsMultiValued() is true.
// EvaluateSingle may only be called when IsMultiValued() is false.
// Implementations signal violations of these preconditions with a panic.
//
// Degree panics on non-polynomial expressions (those containing division or
// inversion).
type Expression interface {
	// IsExtension reports whether this expression involves any column or cell
	// that is evaluated over an extended domain.
	IsExtension() bool
	// IsMultiValued reports whether this expression evaluates to a vector.
	// If false, the expression is scalar.
	IsMultiValued() bool
	// Degree returns the polynomial degree of the expression.
	// Panics if the expression contains a non-polynomial operation (Div,
	// Inverse).
	Degree() int
	// Size returns the length of the vector produced by this expression.
	// Precondition: IsMultiValued() must be true; panics otherwise.
	Size() int
	// IsSized reports whether the vector size of this expression is known.
	// Precondition: IsMultiValued() must be true; panics otherwise.
	IsSized() bool
	// EvaluateVector evaluates this expression against the given runtime and
	// returns the resulting vector.
	// Precondition: IsMultiValued() must be true; panics otherwise.
	EvaluateVector(Runtime) ConcreteVector
	// EvaluateSingle evaluates this expression against the given runtime and
	// returns the resulting scalar.
	// Precondition: IsMultiValued() must be false; panics otherwise.
	EvaluateSingle(Runtime) ConcreteField
	// Module returns the Module whose columns appear in this expression, or
	// nil if the expression contains no column reference. An expression may
	// reference columns from at most one module; mixing columns from different
	// modules is undefined behaviour and callers are expected to validate this
	// before constructing composite expressions.
	Module() *Module
	// Visibility returns the most restrictive [Visibility] level of any leaf
	// in this expression. The ordering from least to most restrictive is:
	// Public < Oracle < Internal. A result of VisibilityInternal means the
	// expression cannot appear in any active query.
	Visibility() Visibility
}

// VectorPromise is the sub-interface of [Expression] satisfied by all
// vector-valued symbolic objects (e.g. [ColumnView]). It carries the same
// method set as [Expression]; the distinct type allows function signatures to
// declare that they require a vector operand, and enables type assertions to
// distinguish vector from scalar expressions.
type VectorPromise interface {
	Expression
}

// FieldPromise is the sub-interface of [Expression] satisfied by all
// scalar-valued symbolic objects (e.g. [Cell]). It carries the same method
// set as [Expression]; the distinct type allows function signatures to declare
// that they require a scalar operand.
type FieldPromise interface {
	Expression
}

// ArithmeticOperator identifies the arithmetic operation performed by an
// [ArithmeticOperation] node.
type ArithmeticOperator int

const (
	ArithmeticOperatorAdd     ArithmeticOperator = iota // a + b  (binary, linear)
	ArithmeticOperatorMul                               // a * b  (binary, product)
	ArithmeticOperatorSub                               // a - b  (binary, linear)
	ArithmeticOperatorDiv                               // a / b  (binary, non-polynomial)
	ArithmeticOperatorDouble                            // 2 * a  (unary, linear)
	ArithmeticOperatorSquare                            // a * a  (unary, product)
	ArithmeticOperatorNegate                            // -a     (unary, linear)
	ArithmeticOperatorInverse                           // 1 / a  (unary, non-polynomial)
)

// arity returns the required number of operands for op. Panics on an unknown
// operator.
func (op ArithmeticOperator) arity() int {
	switch op {
	case ArithmeticOperatorAdd, ArithmeticOperatorMul,
		ArithmeticOperatorSub, ArithmeticOperatorDiv:
		return 2
	case ArithmeticOperatorDouble, ArithmeticOperatorSquare,
		ArithmeticOperatorNegate, ArithmeticOperatorInverse:
		return 1
	default:
		panic(fmt.Sprintf("wiop: unknown ArithmeticOperator %d", int(op)))
	}
}

// combineDegree returns the degree of the expression formed by applying op to
// operands whose degrees are given by operandDegrees.
//
// Panics for non-polynomial operators (Div, Inverse), since degree is
// undefined for non-polynomial expressions.
func (op ArithmeticOperator) combineDegree(operandDegrees []int) int {
	switch op {
	case ArithmeticOperatorAdd, ArithmeticOperatorSub:
		return max(operandDegrees[0], operandDegrees[1])
	case ArithmeticOperatorDouble, ArithmeticOperatorNegate:
		return operandDegrees[0]
	case ArithmeticOperatorMul:
		return operandDegrees[0] + operandDegrees[1]
	case ArithmeticOperatorSquare:
		return 2 * operandDegrees[0]
	case ArithmeticOperatorDiv, ArithmeticOperatorInverse:
		panic(fmt.Sprintf("wiop: Degree() called on non-polynomial expression (%v)", op))
	default:
		panic(fmt.Sprintf("wiop: unknown ArithmeticOperator %d", int(op)))
	}
}

// String implements [fmt.Stringer].
func (op ArithmeticOperator) String() string {
	switch op {
	case ArithmeticOperatorAdd:
		return "Add"
	case ArithmeticOperatorMul:
		return "Mul"
	case ArithmeticOperatorSub:
		return "Sub"
	case ArithmeticOperatorDiv:
		return "Div"
	case ArithmeticOperatorDouble:
		return "Double"
	case ArithmeticOperatorSquare:
		return "Square"
	case ArithmeticOperatorNegate:
		return "Negate"
	case ArithmeticOperatorInverse:
		return "Inverse"
	default:
		return fmt.Sprintf("ArithmeticOperator(%d)", int(op))
	}
}

// ArithmeticOperation is an [Expression] node that applies an
// [ArithmeticOperator] to one or two sub-expressions.
//
// Size, IsSized, and EvaluateVector may only be called when IsMultiValued()
// is true. EvaluateSingle may only be called when IsMultiValued() is false.
type ArithmeticOperation struct {
	Operator ArithmeticOperator
	Operands []Expression
	// isMultiValuedOnce ensures the IsMultiValued traversal runs exactly once,
	// making concurrent calls safe without a mutex.
	isMultiValuedOnce sync.Once
	// isMultiValuedResult holds the cached result after the first traversal.
	isMultiValuedResult bool
	// once ensures the expression is compiled exactly once, even under
	// concurrent calls to EvaluateVector.
	once sync.Once
	// prog is the compiled bytecode representation of this subtree. It is
	// nil until the first call to EvaluateVector.
	prog *compiledProgram
}

// NewArithmeticOperation constructs an ArithmeticOperation, enforcing the
// arity contract of the given operator. Panics if the operand count is wrong
// or any operand is nil.
func NewArithmeticOperation(op ArithmeticOperator, operands ...Expression) *ArithmeticOperation {
	want := op.arity()
	if len(operands) != want {
		panic(fmt.Sprintf("wiop: %v requires %d operand(s), got %d", op, want, len(operands)))
	}
	for i, o := range operands {
		if o == nil {
			panic(fmt.Sprintf("wiop: operand %d of %v is nil", i, op))
		}
	}
	return &ArithmeticOperation{Operator: op, Operands: operands}
}

// IsExtension implements [Expression]. Returns true if any operand involves
// an extended-domain column or cell.
func (a *ArithmeticOperation) IsExtension() bool {
	for _, o := range a.Operands {
		if o.IsExtension() {
			return true
		}
	}
	return false
}

// IsMultiValued implements [Expression]. Returns true if any operand is
// vector-valued. The result is computed once and cached via a [sync.Once],
// making concurrent calls safe.
func (a *ArithmeticOperation) IsMultiValued() bool {
	a.isMultiValuedOnce.Do(func() {
		for _, o := range a.Operands {
			if o.IsMultiValued() {
				a.isMultiValuedResult = true
				return
			}
		}
	})
	return a.isMultiValuedResult
}

// Degree implements [Expression]. Combines the degrees of the operands using
// the operator's own degree-combination rule. Panics for non-polynomial
// operators (Div, Inverse).
func (a *ArithmeticOperation) Degree() int {
	degrees := make([]int, len(a.Operands))
	for i, o := range a.Operands {
		degrees[i] = o.Degree()
	}
	return a.Operator.combineDegree(degrees)
}

// Size implements [Expression]. Returns the size of the first vector-valued
// operand. Panics if IsMultiValued() is false.
func (a *ArithmeticOperation) Size() int {
	if !a.IsMultiValued() {
		panic("wiop: Size() called on a scalar ArithmeticOperation; check IsMultiValued() first")
	}
	for _, o := range a.Operands {
		if o.IsMultiValued() {
			return o.Size()
		}
	}
	panic("unreachable")
}

// IsSized implements [Expression]. Returns true if all vector-valued operands
// are sized. Panics if IsMultiValued() is false.
func (a *ArithmeticOperation) IsSized() bool {
	if !a.IsMultiValued() {
		panic("wiop: IsSized() called on a scalar ArithmeticOperation; check IsMultiValued() first")
	}
	for _, o := range a.Operands {
		if o.IsMultiValued() && !o.IsSized() {
			return false
		}
	}
	return true
}

// EvaluateVector implements [Expression].
// Panics if IsMultiValued() is false.
//
// On the first call the expression subtree is compiled into a [compiledProgram]
// and cached. Subsequent calls reuse the compiled program directly.
func (a *ArithmeticOperation) EvaluateVector(rt Runtime) ConcreteVector {
	if !a.IsMultiValued() {
		panic("wiop: EvaluateVector() called on a scalar ArithmeticOperation; check IsMultiValued() first")
	}
	a.once.Do(func() { a.prog = compileExpr(a) })
	result := a.prog.evaluateVector(rt)
	return ConcreteVector{Plain: []field.Vec{result}, promise: a}
}

// EvaluateSingle implements [Expression].
// Panics if IsMultiValued() is true.
func (a *ArithmeticOperation) EvaluateSingle(rt Runtime) ConcreteField {
	if a.IsMultiValued() {
		panic("wiop: EvaluateSingle() called on a vector ArithmeticOperation; check IsMultiValued() first")
	}
	eval := func(i int) field.Gen { return a.Operands[i].EvaluateSingle(rt).Value }
	var v field.Gen
	switch a.Operator {
	case ArithmeticOperatorAdd:
		v = eval(0).Add(eval(1))
	case ArithmeticOperatorSub:
		v = eval(0).Sub(eval(1))
	case ArithmeticOperatorMul:
		v = eval(0).Mul(eval(1))
	case ArithmeticOperatorDiv:
		v = eval(0).Div(eval(1))
	case ArithmeticOperatorDouble:
		e := eval(0)
		v = e.Add(e)
	case ArithmeticOperatorSquare:
		v = eval(0).Square()
	case ArithmeticOperatorNegate:
		v = eval(0).Neg()
	case ArithmeticOperatorInverse:
		v = eval(0).Inverse()
	default:
		panic(fmt.Sprintf("wiop: ArithmeticOperation.EvaluateSingle: unknown operator %v", a.Operator))
	}
	return ConcreteField{Value: v, promise: a}
}

// Module implements [Expression]. Returns the module of the first
// vector-valued operand, or nil if all operands are scalar. All vector-valued
// operands are expected to share the same module; this invariant is the
// caller's responsibility when constructing the expression.
func (a *ArithmeticOperation) Module() *Module {
	for _, o := range a.Operands {
		if o.IsMultiValued() {
			return o.Module()
		}
	}
	return nil
}

// Visibility implements [Expression]. Returns the most restrictive visibility
// level among all operands: the minimum of their Visibility values (Internal
// < Oracle < Public). This reflects that an expression involving any internal
// leaf cannot appear in an active query.
func (a *ArithmeticOperation) Visibility() Visibility {
	best := VisibilityPublic
	for _, o := range a.Operands {
		if v := o.Visibility(); v < best {
			best = v
		}
	}
	return best
}

// Constant is a fixed-value expression. Its behaviour is determined by whether
// module is nil:
//
//   - module == nil → scalar constant ([FieldPromise] semantics):
//     IsMultiValued() == false; EvaluateSingle returns Value;
//     IsSized, Size, and EvaluateVector panic.
//
//   - module != nil → vector constant ([VectorPromise] semantics):
//     IsMultiValued() == true; EvaluateVector returns Value repeated
//     Module.Size() times; EvaluateSingle panics.
//
// A Constant is never extension-field and is always [VisibilityPublic].
//
// Constructors:
//
//	NewConstantField(v field.Element) *Constant          (module = nil)
//	NewConstantVector(m *Module, v field.Element) *Constant
type Constant struct {
	// Value is the fixed field element this constant represents.
	Value field.Element
	// module is nil for scalar constants; non-nil binds the constant to a
	// module domain, making the constant vector-valued.
	module *Module
}

// NewConstantField constructs a scalar [Constant] with [FieldPromise] semantics.
func NewConstantField(v field.Element) *Constant {
	return &Constant{Value: v}
}

// NewConstantVector constructs a vector [Constant] with [VectorPromise]
// semantics bound to the given module.
//
// Panics if m is nil.
func NewConstantVector(m *Module, v field.Element) *Constant {
	if m == nil {
		panic("wiop: NewConstantVector requires a non-nil Module")
	}
	return &Constant{Value: v, module: m}
}

// IsExtension implements [Expression]. Always returns false: constants are
// always base-field values.
func (c *Constant) IsExtension() bool { return false }

// IsMultiValued implements [Expression]. Returns true iff the constant is
// bound to a module (vector semantics).
func (c *Constant) IsMultiValued() bool { return c.module != nil }

// Visibility implements [Expression]. Always returns [VisibilityPublic].
func (c *Constant) Visibility() Visibility { return VisibilityPublic }

// Degree implements [Expression]. Returns 0 for scalar constants. For vector
// constants returns Module.Size()-1; panics if the module is unsized.
func (c *Constant) Degree() int {
	if c.module == nil {
		return 0
	}
	if !c.module.IsSized() {
		panic("wiop: Constant.Degree() called on a vector constant with an unsized module")
	}
	return c.module.Size() - 1
}

// Module implements [Expression]. Returns the bound module, or nil for scalar
// constants.
func (c *Constant) Module() *Module { return c.module }

// IsSized implements [Expression]. Delegates to the bound module. Panics if
// scalar; check [Constant.IsMultiValued] first.
func (c *Constant) IsSized() bool {
	if c.module == nil {
		panic("wiop: IsSized() cannot be called on a scalar Constant; check IsMultiValued() first")
	}
	return c.module.IsSized()
}

// Size implements [Expression]. Delegates to the bound module. Panics if
// scalar; check [Constant.IsMultiValued] first.
func (c *Constant) Size() int {
	if c.module == nil {
		panic("wiop: Size() cannot be called on a scalar Constant; check IsMultiValued() first")
	}
	return c.module.Size()
}

// EvaluateVector implements [Expression]. Returns a [ConcreteVector] whose
// Plain slice contains a single [field.Vec] with Value repeated
// Module.Size() times, and whose Padding is Value. Panics if scalar; check
// [Constant.IsMultiValued] first.
func (c *Constant) EvaluateVector(_ Runtime) ConcreteVector {
	if c.module == nil {
		panic("wiop: EvaluateVector() cannot be called on a scalar Constant; check IsMultiValued() first")
	}
	n := c.module.Size()
	elems := make([]field.Element, n)
	for i := range elems {
		elems[i] = c.Value
	}
	return ConcreteVector{
		Plain:   []field.Vec{field.VecFromBase(elems)},
		Padding: c.Value,
		promise: c,
	}
}

// EvaluateSingle implements [Expression]. Returns a [ConcreteField] wrapping
// Value. Panics if vector; check [Constant.IsMultiValued] first.
func (c *Constant) EvaluateSingle(_ Runtime) ConcreteField {
	if c.module != nil {
		panic("wiop: EvaluateSingle() cannot be called on a vector Constant; check IsMultiValued() first")
	}
	return ConcreteField{
		Value:   field.ElemFromBase(c.Value),
		promise: c,
	}
}
