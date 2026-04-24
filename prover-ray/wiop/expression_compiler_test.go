package wiop_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/linea-monorepo/prover-ray/wiop"
	"github.com/stretchr/testify/assert"
)

// vecEval evaluates an expression over two columns assigned to all-2 and all-3.
func makeVecEvalSystem(t *testing.T) (*wiop.System, *wiop.Column, *wiop.Column, wiop.Runtime) {
	t.Helper()
	sys, r0, _, mod := newTestSystem(t)
	c1 := mod.NewColumn(sys.Context.Childf("c1"), wiop.VisibilityOracle, r0)
	c2 := mod.NewColumn(sys.Context.Childf("c2"), wiop.VisibilityOracle, r0)
	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(c1, baseVec(4, 2)) // all 2
	rt.AssignColumn(c2, baseVec(4, 3)) // all 3
	return sys, c1, c2, rt
}

func vecAt(cv wiop.ConcreteVector, i int) field.Element {
	return cv.Plain[0].AsBase()[i]
}

func TestCompiler_VecSub(t *testing.T) {
	_, c1, c2, rt := makeVecEvalSystem(t)
	expr := wiop.Sub(c1.View(), c2.View())
	cv := expr.(interface {
		EvaluateVector(wiop.Runtime) wiop.ConcreteVector
	}).EvaluateVector(rt)
	// 2 - 3 = -1 in field
	var want field.Element
	want.SetUint64(2)
	var three field.Element
	three.SetUint64(3)
	want.Sub(&want, &three)
	assert.Equal(t, want, vecAt(cv, 0))
}

func TestCompiler_VecMul(t *testing.T) {
	_, c1, c2, rt := makeVecEvalSystem(t)
	expr := wiop.Mul(c1.View(), c2.View())
	cv := expr.(interface {
		EvaluateVector(wiop.Runtime) wiop.ConcreteVector
	}).EvaluateVector(rt)
	// 2*3 = 6
	var want field.Element
	want.SetUint64(6)
	assert.Equal(t, want, vecAt(cv, 0))
}

func TestCompiler_VecDiv(t *testing.T) {
	_, c1, c2, rt := makeVecEvalSystem(t)
	expr := wiop.Div(c1.View(), c2.View())
	cv := expr.(interface {
		EvaluateVector(wiop.Runtime) wiop.ConcreteVector
	}).EvaluateVector(rt)
	assert.Equal(t, 4, cv.Plain[0].Len())
}

func TestCompiler_VecDouble(t *testing.T) {
	_, c1, _, rt := makeVecEvalSystem(t)
	expr := wiop.Double(c1.View())
	cv := expr.(interface {
		EvaluateVector(wiop.Runtime) wiop.ConcreteVector
	}).EvaluateVector(rt)
	// 2+2 = 4
	var want field.Element
	want.SetUint64(4)
	assert.Equal(t, want, vecAt(cv, 0))
}

func TestCompiler_VecSquare(t *testing.T) {
	_, c1, _, rt := makeVecEvalSystem(t)
	expr := wiop.Square(c1.View())
	cv := expr.(interface {
		EvaluateVector(wiop.Runtime) wiop.ConcreteVector
	}).EvaluateVector(rt)
	// 2*2 = 4
	var want field.Element
	want.SetUint64(4)
	assert.Equal(t, want, vecAt(cv, 0))
}

func TestCompiler_VecNegate(t *testing.T) {
	_, c1, _, rt := makeVecEvalSystem(t)
	expr := wiop.Negate(c1.View())
	cv := expr.(interface {
		EvaluateVector(wiop.Runtime) wiop.ConcreteVector
	}).EvaluateVector(rt)
	// -2 in the field
	var want field.Element
	want.SetUint64(2)
	want.Neg(&want)
	assert.Equal(t, want, vecAt(cv, 0))
}

func TestCompiler_VecInverse(t *testing.T) {
	_, c1, _, rt := makeVecEvalSystem(t)
	expr := wiop.Inverse(c1.View())
	cv := expr.(interface {
		EvaluateVector(wiop.Runtime) wiop.ConcreteVector
	}).EvaluateVector(rt)
	assert.Equal(t, 4, cv.Plain[0].Len())
}

// Composite expression reusing a subexpression — exercises slot reuse in the compiler.
func TestCompiler_VecComposite(t *testing.T) {
	_, c1, c2, rt := makeVecEvalSystem(t)
	// (c1+c2) * (c1-c2)  = (2+3)*(2-3) = 5*(-1) = -5
	sum := wiop.Add(c1.View(), c2.View())
	diff := wiop.Sub(c1.View(), c2.View())
	expr := wiop.Mul(sum, diff)
	cv := expr.(interface {
		EvaluateVector(wiop.Runtime) wiop.ConcreteVector
	}).EvaluateVector(rt)
	// 5 * (-1)
	var five, neg1, want field.Element
	five.SetUint64(5)
	neg1.SetUint64(1)
	neg1.Neg(&neg1)
	want.Mul(&five, &neg1)
	assert.Equal(t, want, vecAt(cv, 0))
}
