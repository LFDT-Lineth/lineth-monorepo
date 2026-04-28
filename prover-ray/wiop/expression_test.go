package wiop_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/linea-monorepo/prover-ray/wiop"
	"github.com/stretchr/testify/assert"
)

// ---- ArithmeticOperator ----

func TestArithmeticOperator_String(t *testing.T) {
	cases := []struct {
		op   wiop.ArithmeticOperator
		want string
	}{
		{wiop.ArithmeticOperatorAdd, "Add"},
		{wiop.ArithmeticOperatorSub, "Sub"},
		{wiop.ArithmeticOperatorMul, "Mul"},
		{wiop.ArithmeticOperatorDiv, "Div"},
		{wiop.ArithmeticOperatorDouble, "Double"},
		{wiop.ArithmeticOperatorSquare, "Square"},
		{wiop.ArithmeticOperatorNegate, "Negate"},
		{wiop.ArithmeticOperatorInverse, "Inverse"},
		{wiop.ArithmeticOperator(99), "ArithmeticOperator(99)"},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, tc.op.String())
	}
}

// ---- NewArithmeticOperation panics ----

func TestNewArithmeticOperation_WrongArity(t *testing.T) {
	c := wiop.NewConstantField(field.NewFromString("1"))
	// Add requires 2 operands
	assert.Panics(t, func() { wiop.NewArithmeticOperation(wiop.ArithmeticOperatorAdd, c) })
	// Double requires 1 operand
	assert.Panics(t, func() { wiop.NewArithmeticOperation(wiop.ArithmeticOperatorDouble, c, c) })
}

func TestNewArithmeticOperation_NilOperand(t *testing.T) {
	c := wiop.NewConstantField(field.NewFromString("1"))
	assert.Panics(t, func() { wiop.NewArithmeticOperation(wiop.ArithmeticOperatorAdd, c, nil) })
}

// ---- Constant (scalar) ----

func TestConstant_Scalar(t *testing.T) {
	_, r0, _, _ := newTestSystem(t)
	rt := wiop.NewRuntime(r0.System())

	v := field.NewFromString("7")
	c := wiop.NewConstantField(v)

	assert.False(t, c.IsExtension())
	assert.False(t, c.IsMultiValued())
	assert.Equal(t, 0, c.Degree())
	assert.Nil(t, c.Module())
	assert.Equal(t, wiop.VisibilityPublic, c.Visibility())

	assert.Panics(t, func() { c.IsSized() })
	assert.Panics(t, func() { c.Size() })
	assert.Panics(t, func() { c.EvaluateVector(rt) })
}

func TestConstant_Scalar_EvaluateSingle(t *testing.T) {
	_, r0, _, _ := newTestSystem(t)
	rt := wiop.NewRuntime(r0.System())

	v := field.NewFromString("42")
	c := wiop.NewConstantField(v)
	result := c.EvaluateSingle(rt)
	assert.Equal(t, field.ElemFromBase(v), result.Value)
}

// ---- Constant (vector) ----

func TestConstant_Vector(t *testing.T) {
	sys := wiop.NewSystemf("sys")
	ctx := sys.Context.Childf("mod")
	mod := sys.NewSizedModule(ctx, 4, wiop.PaddingDirectionNone)

	v := field.NewFromString("3")
	c := wiop.NewConstantVector(mod, v)

	assert.False(t, c.IsExtension())
	assert.True(t, c.IsMultiValued())
	assert.Equal(t, 3, c.Degree()) // size-1
	assert.Equal(t, mod, c.Module())
	assert.True(t, c.IsSized())
	assert.Equal(t, 4, c.Size())
	assert.Equal(t, wiop.VisibilityPublic, c.Visibility())

	_, r0, _, _ := newTestSystem(t)
	rt2 := wiop.NewRuntime(r0.System())
	assert.Panics(t, func() { c.EvaluateSingle(rt2) })
}

func TestConstant_Vector_EvaluateVector(t *testing.T) {
	sys := wiop.NewSystemf("sys")
	sys.NewRound() // need at least one round for NewRuntime
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	rt := wiop.NewRuntime(sys)

	v := field.NewFromString("5")
	c := wiop.NewConstantVector(mod, v)

	cv := c.EvaluateVector(rt)
	vec := cv.Plain
	assert.Equal(t, 4, vec.Len())
	for i := range 4 {
		assert.Equal(t, field.ElemFromBase(v), field.ElemFromBase(vec.AsBase()[i]))
	}
}

func TestConstant_Vector_UnsizedModuleDegree(t *testing.T) {
	sys := wiop.NewSystemf("sys")
	mod := sys.NewModule(sys.Context.Childf("mod"), wiop.PaddingDirectionNone)
	c := wiop.NewConstantVector(mod, field.NewFromString("1"))
	assert.Panics(t, func() { c.Degree() })
}

func TestNewConstantVector_NilModulePanic(t *testing.T) {
	assert.Panics(t, func() { wiop.NewConstantVector(nil, field.NewFromString("1")) })
}

// ---- ArithmeticOperation scalar evaluation ----

func TestArithmeticOperation_ScalarEval(t *testing.T) {
	// Build a runtime with a cell assigned to 3.
	sys := wiop.NewSystemf("sys")
	r0 := sys.NewRound()
	cell := r0.NewCell(sys.Context.Childf("cell"), false)
	rt := wiop.NewRuntime(sys)

	three := field.NewFromString("3")
	e3 := three
	rt.AssignCell(cell, field.ElemFromBase(e3))

	// Constant 2.
	two := field.NewFromString("2")
	k2 := wiop.NewConstantField(two)

	cases := []struct {
		name string
		expr wiop.Expression
		want func() field.Gen
	}{
		{"add", wiop.Add(cell, k2), func() field.Gen {
			return field.ElemFromBase(three).Add(field.ElemFromBase(two))
		}},
		{"sub", wiop.Sub(cell, k2), func() field.Gen {
			return field.ElemFromBase(three).Sub(field.ElemFromBase(two))
		}},
		{"mul", wiop.Mul(cell, k2), func() field.Gen {
			return field.ElemFromBase(three).Mul(field.ElemFromBase(two))
		}},
		{"double", wiop.Double(cell), func() field.Gen {
			return field.ElemFromBase(three).Add(field.ElemFromBase(three))
		}},
		{"square", wiop.Square(k2), func() field.Gen {
			return field.ElemFromBase(two).Square()
		}},
		{"negate", wiop.Negate(k2), func() field.Gen {
			return field.ElemFromBase(two).Neg()
		}},
		{"inverse", wiop.Inverse(k2), func() field.Gen {
			return field.ElemFromBase(two).Inverse()
		}},
		{"div", wiop.Div(cell, k2), func() field.Gen {
			return field.ElemFromBase(three).Div(field.ElemFromBase(two))
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.False(t, tc.expr.IsMultiValued())
			result := tc.expr.EvaluateSingle(rt)
			assert.Equal(t, tc.want(), result.Value)
		})
	}
}

func TestArithmeticOperation_EvaluateSingle_WrongModePanic(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(col, baseVec(4, 1))

	// col.View() is vector → Add(col.View(), col.View()) is vector
	v := wiop.Add(col.View(), col.View())
	assert.True(t, v.IsMultiValued())
	assert.Panics(t, func() { v.EvaluateSingle(rt) })
}

func TestArithmeticOperation_EvaluateVector_WrongModePanic(t *testing.T) {
	_, r0, _, _ := newTestSystem(t)
	rt := wiop.NewRuntime(r0.System())
	k := wiop.NewConstantField(field.NewFromString("1"))
	// scalar Add → EvaluateVector must panic
	expr := wiop.Add(k, k)
	assert.False(t, expr.IsMultiValued())
	assert.Panics(t, func() { expr.EvaluateVector(rt) })
}

// ---- ArithmeticOperation properties ----

func TestArithmeticOperation_IsExtension(t *testing.T) {
	sys := wiop.NewSystemf("sys")
	r := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	base := mod.NewColumn(sys.Context.Childf("base"), wiop.VisibilityOracle, r)
	ext := mod.NewExtensionColumn(sys.Context.Childf("ext"), wiop.VisibilityOracle, r)

	assert.False(t, wiop.Add(base.View(), base.View()).IsExtension())
	assert.True(t, wiop.Add(base.View(), ext.View()).IsExtension())
}

func TestArithmeticOperation_Degree(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("colDeg"), wiop.VisibilityOracle, r0)

	// degree of col.View() == 3 (size 4)
	// add: max(3,3) = 3
	assert.Equal(t, 3, wiop.Add(col.View(), col.View()).Degree())
	// mul: 3+3 = 6
	assert.Equal(t, 6, wiop.Mul(col.View(), col.View()).Degree())
	// square: 2*3 = 6
	assert.Equal(t, 6, wiop.Square(col.View()).Degree())
}

func TestArithmeticOperation_Degree_NonPolynomialPanic(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("colDivDeg"), wiop.VisibilityOracle, r0)
	assert.Panics(t, func() { wiop.Div(col.View(), col.View()).Degree() })
	assert.Panics(t, func() { wiop.Inverse(col.View()).Degree() })
}

func TestArithmeticOperation_Size_IsSized(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("colSz"), wiop.VisibilityOracle, r0)
	expr := wiop.Add(col.View(), col.View())
	assert.True(t, expr.IsMultiValued())
	assert.Equal(t, 4, expr.(interface{ Size() int }).Size())
	assert.True(t, expr.(interface{ IsSized() bool }).IsSized())
}

func TestArithmeticOperation_Size_ScalarPanic(t *testing.T) {
	k := wiop.NewConstantField(field.NewFromString("1"))
	expr := wiop.Add(k, k)
	assert.Panics(t, func() { expr.(interface{ Size() int }).Size() })
	assert.Panics(t, func() { expr.(interface{ IsSized() bool }).IsSized() })
}

func TestArithmeticOperation_Module(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("colMod"), wiop.VisibilityOracle, r0)
	k := wiop.NewConstantField(field.NewFromString("1"))

	// vector op → module is the column's module
	assert.Equal(t, mod, wiop.Add(col.View(), col.View()).(interface{ Module() *wiop.Module }).Module())
	// scalar op → nil
	assert.Nil(t, wiop.Add(k, k).(interface{ Module() *wiop.Module }).Module())
}

func TestArithmeticOperation_Visibility(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	oracle := mod.NewColumn(sys.Context.Childf("oracle"), wiop.VisibilityOracle, r0)
	internal := mod.NewColumn(sys.Context.Childf("internal"), wiop.VisibilityInternal, r0)

	assert.Equal(t, wiop.VisibilityOracle, wiop.Add(oracle.View(), oracle.View()).(interface{ Visibility() wiop.Visibility }).Visibility())
	// min(Oracle, Internal) = Internal
	assert.Equal(t, wiop.VisibilityInternal, wiop.Add(oracle.View(), internal.View()).(interface{ Visibility() wiop.Visibility }).Visibility())
}

// ---- ArithmeticOperation vector evaluation ----

func TestArithmeticOperation_VectorEval_Add(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("colVAdd"), wiop.VisibilityOracle, r0)
	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(col, baseVec(4, 3))

	expr := wiop.Add(col.View(), col.View())
	cv := expr.(interface {
		EvaluateVector(wiop.Runtime) wiop.ConcreteVector
	}).EvaluateVector(rt)
	// each element should be 3+3 = 6
	six := field.NewFromString("6")
	for i := range 4 {
		assert.Equal(t, six, cv.Plain.AsBase()[i])
	}
}

// ---- Sum and Product helpers ----

func TestSum_SingleTerm(t *testing.T) {
	_, r0, _, _ := newTestSystem(t)
	rt := wiop.NewRuntime(r0.System())
	k := wiop.NewConstantField(field.NewFromString("7"))
	result := wiop.Sum(k)
	v := result.EvaluateSingle(rt)
	assert.Equal(t, field.ElemFromBase(field.NewFromString("7")), v.Value)
}

func TestSum_EmptyPanic(t *testing.T) {
	assert.Panics(t, func() { wiop.Sum() })
}

func TestProduct_EmptyPanic(t *testing.T) {
	assert.Panics(t, func() { wiop.Product() })
}

func TestProduct_MultipleTerms(t *testing.T) {
	_, r0, _, _ := newTestSystem(t)
	rt := wiop.NewRuntime(r0.System())
	k2 := wiop.NewConstantField(field.NewFromString("2"))
	k3 := wiop.NewConstantField(field.NewFromString("3"))
	// 2 * 3 = 6
	result := wiop.Product(k2, k3)
	v := result.EvaluateSingle(rt)
	assert.Equal(t, field.ElemFromBase(field.NewFromString("6")), v.Value)
}
