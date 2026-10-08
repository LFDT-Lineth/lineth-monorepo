package wiop

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/internal/vecprog"
)

// LowerOperator maps o to its [vecprog] operator.
func LowerOperator(o ArithmeticOperator) vecprog.Op {
	switch o {
	case ArithmeticOperatorAdd:
		return vecprog.Add
	case ArithmeticOperatorSub:
		return vecprog.Sub
	case ArithmeticOperatorMul:
		return vecprog.Mul
	case ArithmeticOperatorDiv:
		return vecprog.Div
	case ArithmeticOperatorDouble:
		return vecprog.Double
	case ArithmeticOperatorSquare:
		return vecprog.Square
	case ArithmeticOperatorNegate:
		return vecprog.Negate
	case ArithmeticOperatorInverse:
		return vecprog.Inverse
	}
	panic(fmt.Sprintf("wiop: unknown ArithmeticOperator %v", o))
}

// RowLowering lowers expressions evaluated over n rows into one [vecprog]
// program, as [EvaluateAsExtVec] evaluates them: a column view reads its
// module's padded assignment at its shift, each column being written down
// once for all its views. An expression reading a column of a module of
// another size is evaluated by [EvaluateAsExtVec] instead, so that its
// padding to n rows is unchanged, and enters the program as a table.
type RowLowering struct {
	B *vecprog.Builder

	rt       *Runtime
	n        int
	baseCols map[*Column][]field.Element
	extCols  map[*Column][]field.Ext
}

// NewRowLowering returns a lowering over the first n rows, building into a
// fresh builder.
func NewRowLowering(rt *Runtime, n int) *RowLowering {
	return &RowLowering{
		B:        vecprog.NewBuilder(),
		rt:       rt,
		n:        n,
		baseCols: make(map[*Column][]field.Element),
		extCols:  make(map[*Column][]field.Ext),
	}
}

// Lower adds expr to the builder and returns its node.
func (l *RowLowering) Lower(expr Expression) int {
	if l.sizedTo(expr) {
		return l.lower(expr)
	}
	return l.B.Ext(EvaluateAsExtVec(l.rt, expr, l.n), 0)
}

// sizedTo reports whether every vector leaf of expr belongs to a module of
// l.n rows. Vector leaves of other kinds are not lowered.
func (l *RowLowering) sizedTo(expr Expression) bool {
	switch e := expr.(type) {
	case *ColumnView:
		return e.Column.Module.RuntimeSize(l.rt) == l.n
	case *Constant:
		return e.module == nil || e.module.RuntimeSize(l.rt) == l.n
	case *LagrangeSelector:
		return e.module.RuntimeSize(l.rt) == l.n
	case *Cell, *CoinField:
		return true
	case *ArithmeticOperation:
		for _, op := range e.Operands {
			if !l.sizedTo(op) {
				return false
			}
		}
		return true
	}
	return !expr.IsMultiValued()
}

func (l *RowLowering) lower(expr Expression) int {
	switch e := expr.(type) {
	case *ColumnView:
		offset := ((e.ShiftingOffset % l.n) + l.n) % l.n
		if e.Column.IsExtension {
			return l.B.Ext(l.extColumn(e.Column), offset)
		}
		return l.B.Base(l.baseColumn(e.Column), offset)
	case *Constant:
		return l.B.Scalar(field.Lift(e.Value), true)
	case *Cell:
		v := l.rt.GetCellValue(e)
		if e.IsExtension() {
			return l.B.Scalar(v.AsExt(), false)
		}
		if !v.IsBase() {
			panic(fmt.Sprintf("wiop: cell %q declared as base but holds an extension-field value", e.Context.Path()))
		}
		return l.B.Scalar(field.Lift(v.AsBase()), true)
	case *CoinField:
		return l.B.Scalar(l.rt.GetCoinValue(e).AsExt(), false)
	case *ArithmeticOperation:
		o := LowerOperator(e.Operator)
		args := make([]int, o.Arity())
		for i := range args {
			args[i] = l.lower(e.Operands[i])
		}
		return l.B.Op(o, args...)
	}
	if !expr.IsMultiValued() {
		v := expr.EvaluateSingle(l.rt).Value
		if v.IsBase() {
			return l.B.Scalar(field.Lift(v.AsBase()), true)
		}
		return l.B.Scalar(v.AsExt(), false)
	}
	// A Lagrange selector is written down as EvaluateAsExtVec does: its
	// plain values, then its padding.
	cv := expr.EvaluateVector(l.rt)
	if cv.Plain.IsBase() {
		table := make([]field.Element, l.n)
		k := copy(table, cv.Plain.AsBase())
		field.VecFillBase(table[k:], cv.Padding)
		return l.B.Base(table, 0)
	}
	table := make([]field.Ext, l.n)
	k := copy(table, cv.Plain.AsExt())
	field.VecFillExt(table[k:], field.Lift(cv.Padding))
	return l.B.Ext(table, 0)
}

// baseColumn returns col's padded assignment over n rows, written once.
func (l *RowLowering) baseColumn(col *Column) []field.Element {
	if t, ok := l.baseCols[col]; ok {
		return t
	}
	t := make([]field.Element, l.n)
	materializeBase(t, l.rt.GetColumnAssignment(col), col.Module.Padding, l.n)
	l.baseCols[col] = t
	return t
}

// extColumn returns col's padded assignment over n rows, written once.
func (l *RowLowering) extColumn(col *Column) []field.Ext {
	if t, ok := l.extCols[col]; ok {
		return t
	}
	t := make([]field.Ext, l.n)
	materializeExt(t, l.rt.GetColumnAssignment(col), col.Module.Padding, l.n)
	l.extCols[col] = t
	return t
}
