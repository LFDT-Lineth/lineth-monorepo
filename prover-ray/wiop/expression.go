package wiop

import (
	"fmt"
	"sync"
	"sync/atomic"

	field "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
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
	// DegreeFactor returns the degree of the expression as a multiple of the
	// column degree. For a single column, DegreeFactor() returns 1. For a
	// product of two columns, DegreeFactor() returns 2. For constants and
	// scalars, DegreeFactor() returns 0.
	//
	// This allows computing the quotient ratio without knowing the module size,
	// which is required for dynamic-size modules. The actual degree is
	// DegreeFactor() * (n - 1) where n is the module size.
	//
	// Panics for non-polynomial operators (Div, Inverse).
	DegreeFactor() int
	// Size returns the length of the vector produced by this expression.
	// Precondition: IsMultiValued() must be true; panics otherwise.
	Size() int
	// IsSized reports whether the vector size of this expression is known.
	// Precondition: IsMultiValued() must be true; panics otherwise.
	IsSized() bool
	// EvaluateVector evaluates this expression against the given runtime and
	// returns the resulting vector.
	// Precondition: IsMultiValued() must be true; panics otherwise.
	EvaluateVector(*Runtime) ConcreteVector
	// EvaluateSingle evaluates this expression against the given runtime and
	// returns the resulting scalar.
	// Precondition: IsMultiValued() must be false; panics otherwise.
	EvaluateSingle(*Runtime) ConcreteField
	// Module returns the Module whose columns appear in this expression, or
	// nil if the expression contains no column reference. An expression may
	// reference columns from at most one module; mixing columns from different
	// modules is undefined behaviour and callers are expected to validate this
	// before constructing composite expressions.
	Module() *Module
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

// combineDegreeFactor returns the degree factor of the expression formed by
// applying op to operands whose degree factors are given by operandFactors.
// The degree factor is the degree expressed as a multiple of (n-1), where n is
// the module size. This allows computing the quotient ratio without knowing n.
//
// Panics for non-polynomial operators (Div, Inverse).
func (op ArithmeticOperator) combineDegreeFactor(operandFactors []int) int {
	switch op {
	case ArithmeticOperatorAdd, ArithmeticOperatorSub:
		return max(operandFactors[0], operandFactors[1])
	case ArithmeticOperatorDouble, ArithmeticOperatorNegate:
		return operandFactors[0]
	case ArithmeticOperatorMul:
		return operandFactors[0] + operandFactors[1]
	case ArithmeticOperatorSquare:
		return 2 * operandFactors[0]
	case ArithmeticOperatorDiv, ArithmeticOperatorInverse:
		panic(fmt.Sprintf("wiop: DegreeFactor() called on non-polynomial expression (%v)", op))
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
	// isExtensionCache and degreeFactorCache memoise IsExtension and
	// DegreeFactor, which are otherwise recomputed by a walk that re-descends
	// into every shared subexpression. Zero means "not computed yet";
	// otherwise isExtensionCache is 1 (false) or 2 (true) and
	// degreeFactorCache holds the factor plus one. They are written only
	// after a successful computation, so a panicking call (DegreeFactor on a
	// Div) caches nothing. Degree is deliberately not memoised: it depends on
	// the module size, which may not be known yet.
	isExtensionCache  atomic.Int32
	degreeFactorCache atomic.Int64
	// singleOnce guards singleProg, the compiled form used by EvaluateSingle.
	singleOnce sync.Once
	singleProg *scalarProgram
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
	if c := a.isExtensionCache.Load(); c != 0 {
		return c == 2
	}
	res := false
	for _, o := range a.Operands {
		if o.IsExtension() {
			res = true
			break
		}
	}
	if res {
		a.isExtensionCache.Store(2)
	} else {
		a.isExtensionCache.Store(1)
	}
	return res
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

// DegreeFactor implements [Expression]. Combines the degree factors of the
// operands using the operator's own degree-combination rule.
func (a *ArithmeticOperation) DegreeFactor() int {
	if c := a.degreeFactorCache.Load(); c != 0 {
		return int(c - 1)
	}
	factors := make([]int, len(a.Operands))
	for i, o := range a.Operands {
		factors[i] = o.DegreeFactor()
	}
	res := a.Operator.combineDegreeFactor(factors)
	a.degreeFactorCache.Store(int64(res) + 1)
	return res
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
func (a *ArithmeticOperation) EvaluateVector(rt *Runtime) ConcreteVector {
	if !a.IsMultiValued() {
		panic("wiop: EvaluateVector() called on a scalar ArithmeticOperation; check IsMultiValued() first")
	}
	a.once.Do(func() { a.prog = compileExpr(a) })
	result := a.prog.evaluateVector(rt)
	return ConcreteVector{Plain: result, promise: a}
}

// EvaluateSingle implements [Expression].
// Panics if IsMultiValued() is true.
//
// On the first call the subtree is compiled into a [scalarProgram] listing its
// distinct nodes, so that each shared subexpression is evaluated once per call.
func (a *ArithmeticOperation) EvaluateSingle(rt *Runtime) ConcreteField {
	if a.IsMultiValued() {
		panic("wiop: EvaluateSingle() called on a vector ArithmeticOperation; check IsMultiValued() first")
	}
	a.singleOnce.Do(func() { a.singleProg = compileScalar(a) })
	v := a.singleProg.evaluate(rt)
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
// A Constant is never extension-field.
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

// DegreeFactor implements [Expression]. Returns 0 for scalar constants, 1 for
// vector constants (degree is 1 * (n-1) = n-1).
func (c *Constant) DegreeFactor() int {
	if c.module == nil {
		return 0
	}
	return 1
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
// scalar (check [Constant.IsMultiValued] first) or if the bound module is
// dynamic or unsized (check [Constant.IsSized] first).
func (c *Constant) Size() int {
	if c.module == nil {
		panic("wiop: Size() cannot be called on a scalar Constant; check IsMultiValued() first")
	}
	if !c.module.IsSized() {
		panic("wiop: Size() called on a vector Constant with an unsized or dynamic module; check IsSized() first")
	}
	return c.module.Size()
}

// EvaluateVector implements [Expression]. Returns a [ConcreteVector] whose
// Plain slice contains a single [field.Vec] with Value repeated
// Module.Size() times, and whose Padding is Value. Panics if scalar; check
// [Constant.IsMultiValued] first.
func (c *Constant) EvaluateVector(rt *Runtime) ConcreteVector {
	if c.module == nil {
		panic("wiop: EvaluateVector() cannot be called on a scalar Constant; check IsMultiValued() first")
	}
	n := c.module.RuntimeSize(rt)
	elems := make([]field.Element, n)
	for i := range elems {
		elems[i] = c.Value
	}
	return ConcreteVector{
		Plain:   field.VecFromBase(elems),
		Padding: c.Value,
		promise: c,
	}
}

// EvaluateSingle implements [Expression]. Returns a [ConcreteField] wrapping
// Value. Panics if vector; check [Constant.IsMultiValued] first.
func (c *Constant) EvaluateSingle(_ *Runtime) ConcreteField {
	if c.module != nil {
		panic("wiop: EvaluateSingle() cannot be called on a vector Constant; check IsMultiValued() first")
	}
	return ConcreteField{
		Value:   field.ElemFromBase(c.Value),
		promise: c,
	}
}

// EvaluateAsExtVec evaluates expr against the runtime and returns a length-n
// extension-field slice. Scalar expressions are broadcast to every position;
// vector results shorter than n are extended with the padding value.
func EvaluateAsExtVec(rt *Runtime, expr Expression, n int) []field.Ext {
	out := make([]field.Ext, n)

	if !expr.IsMultiValued() {
		ext := expr.EvaluateSingle(rt).Value.AsExt()
		for i := range out {
			out[i] = ext
		}
		return out
	}

	cv := expr.EvaluateVector(rt)
	plain := cv.Plain
	if plain.IsBase() {
		base := plain.AsBase()
		copyLen := min(len(base), n)
		for i := 0; i < copyLen; i++ {
			out[i] = field.Lift(base[i])
		}
		pad := field.Lift(cv.Padding)
		for i := copyLen; i < n; i++ {
			out[i] = pad
		}
		return out
	}

	ext := plain.AsExt()
	copyLen := min(len(ext), n)
	copy(out[:copyLen], ext[:copyLen])
	pad := field.Lift(cv.Padding)
	for i := copyLen; i < n; i++ {
		out[i] = pad
	}
	return out
}

// scalarProgram is the compiled form of a scalar [ArithmeticOperation]
// subtree used by EvaluateSingle: its distinct nodes (by pointer) in
// post-order, so that operands always precede the nodes reading them.
type scalarProgram struct {
	nodes []scalarNode
}

// scalarNode is a leaf, evaluated through its own EvaluateSingle, or an
// operator over earlier nodes.
type scalarNode struct {
	leaf     Expression // nil for an operator node
	operator ArithmeticOperator
	operands [2]int // indices into scalarProgram.nodes; -1 when absent
}

func compileScalar(root *ArithmeticOperation) *scalarProgram {
	p := &scalarProgram{}
	index := map[Expression]int{}
	var visit func(e Expression) int
	visit = func(e Expression) int {
		if i, ok := index[e]; ok {
			return i
		}
		n := scalarNode{operands: [2]int{-1, -1}}
		if op, ok := e.(*ArithmeticOperation); ok {
			n.operator = op.Operator
			for k, o := range op.Operands {
				n.operands[k] = visit(o)
			}
		} else {
			n.leaf = e
		}
		p.nodes = append(p.nodes, n)
		index[e] = len(p.nodes) - 1
		return len(p.nodes) - 1
	}
	visit(root)
	return p
}

func (p *scalarProgram) evaluate(rt *Runtime) field.Gen {
	vals := make([]field.Gen, len(p.nodes))
	for i, n := range p.nodes {
		if n.leaf != nil {
			vals[i] = n.leaf.EvaluateSingle(rt).Value
			continue
		}
		a0 := vals[n.operands[0]]
		switch n.operator {
		case ArithmeticOperatorAdd:
			vals[i] = a0.Add(vals[n.operands[1]])
		case ArithmeticOperatorSub:
			vals[i] = a0.Sub(vals[n.operands[1]])
		case ArithmeticOperatorMul:
			vals[i] = a0.Mul(vals[n.operands[1]])
		case ArithmeticOperatorDiv:
			vals[i] = a0.Div(vals[n.operands[1]])
		case ArithmeticOperatorDouble:
			vals[i] = a0.Add(a0)
		case ArithmeticOperatorSquare:
			vals[i] = a0.Square()
		case ArithmeticOperatorNegate:
			vals[i] = a0.Neg()
		case ArithmeticOperatorInverse:
			vals[i] = a0.Inverse()
		default:
			panic(fmt.Sprintf("wiop: ArithmeticOperation.EvaluateSingle: unknown operator %v", n.operator))
		}
	}
	return vals[len(vals)-1]
}
