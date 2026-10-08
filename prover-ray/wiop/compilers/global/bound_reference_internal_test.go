package global

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// The point-by-point tree evaluator the quotient program replaced. It is kept
// as the reference the program is checked against. Points are indexed in the
// coset-major layout of [cosetShifts], n points per small coset.

// shiftedIndex returns the index of point j shifted by offset within its small
// coset of n points.
func shiftedIndex(j, offset, n int) int {
	return j/n*n + (j%n+offset)%n
}

// evalBase evaluates a base-field bound expression at coset point j. The
// caller must guarantee isBase; extension leaves cannot appear below a base
// node by construction.
func (e *boundExpr) evalBase(j, n int) field.Element {
	switch e.kind {
	case boundVecBase:
		return e.vecBase[shiftedIndex(j, e.offset, n)]
	case boundScalarBase:
		return e.scalarBase
	}
	a0 := e.operands[0].evalBase(j, n)
	var res field.Element
	switch e.operator {
	case wiop.ArithmeticOperatorAdd:
		a1 := e.operands[1].evalBase(j, n)
		res.Add(&a0, &a1)
	case wiop.ArithmeticOperatorSub:
		a1 := e.operands[1].evalBase(j, n)
		res.Sub(&a0, &a1)
	case wiop.ArithmeticOperatorMul:
		a1 := e.operands[1].evalBase(j, n)
		res.Mul(&a0, &a1)
	case wiop.ArithmeticOperatorDiv:
		a1 := e.operands[1].evalBase(j, n)
		var invA1 field.Element
		invA1.Inverse(&a1)
		res.Mul(&a0, &invA1)
	case wiop.ArithmeticOperatorDouble:
		res.Add(&a0, &a0)
	case wiop.ArithmeticOperatorSquare:
		res.Square(&a0)
	case wiop.ArithmeticOperatorNegate:
		res.Neg(&a0)
	case wiop.ArithmeticOperatorInverse:
		res.Inverse(&a0)
	default:
		panic(fmt.Sprintf("wiop/compilers: unknown ArithmeticOperator %v", e.operator))
	}
	return res
}

// evalExt evaluates a bound expression at coset point j in the extension
// field. Base subtrees are evaluated by [boundExpr.evalBase] and lifted at
// the boundary; a Mul with one base operand folds it in via MulByElement
// instead of paying a full extension-field multiplication.
func (e *boundExpr) evalExt(j, n int) field.Ext {
	if e.isBase {
		return field.Lift(e.evalBase(j, n))
	}
	switch e.kind {
	case boundVecExt:
		return e.vecExt[shiftedIndex(j, e.offset, n)]
	case boundScalarExt:
		return e.scalarExt
	}
	var res field.Ext
	switch e.operator {
	case wiop.ArithmeticOperatorAdd:
		a0 := e.operands[0].evalExt(j, n)
		a1 := e.operands[1].evalExt(j, n)
		res.Add(&a0, &a1)
	case wiop.ArithmeticOperatorSub:
		a0 := e.operands[0].evalExt(j, n)
		a1 := e.operands[1].evalExt(j, n)
		res.Sub(&a0, &a1)
	case wiop.ArithmeticOperatorMul:
		if e.operands[0].isBase {
			b := e.operands[0].evalBase(j, n)
			a1 := e.operands[1].evalExt(j, n)
			res.MulByElement(&a1, &b)
		} else if e.operands[1].isBase {
			b := e.operands[1].evalBase(j, n)
			a0 := e.operands[0].evalExt(j, n)
			res.MulByElement(&a0, &b)
		} else {
			a0 := e.operands[0].evalExt(j, n)
			a1 := e.operands[1].evalExt(j, n)
			res.Mul(&a0, &a1)
		}
	case wiop.ArithmeticOperatorDiv:
		a0 := e.operands[0].evalExt(j, n)
		a1 := e.operands[1].evalExt(j, n)
		var inv field.Ext
		inv.Inverse(&a1)
		res.Mul(&a0, &inv)
	case wiop.ArithmeticOperatorDouble:
		a0 := e.operands[0].evalExt(j, n)
		res.Double(&a0)
	case wiop.ArithmeticOperatorSquare:
		a0 := e.operands[0].evalExt(j, n)
		res.Square(&a0)
	case wiop.ArithmeticOperatorNegate:
		a0 := e.operands[0].evalExt(j, n)
		res.Neg(&a0)
	case wiop.ArithmeticOperatorInverse:
		a0 := e.operands[0].evalExt(j, n)
		res.Inverse(&a0)
	default:
		panic(fmt.Sprintf("wiop/compilers: unknown ArithmeticOperator %v", e.operator))
	}
	return res
}

// accumulate adds coinPow · P(coset_j) · C(coset_j) into aggregate[j] for
// every j in [start, end). It dispatches once on the expression's field so
// the inner loop stays entirely in base or extension arithmetic:
//
//   - base expression: pVal·cancellation multiplies in base, then promotes
//     once into Ext via [field.Ext.MulByElement];
//   - extension expression: the cancellation is base, so MulByElement folds
//     it in, then [field.Ext.Mul] applies the coin power.
func (be *boundEntry) accumulate(aggregate []field.Ext, start, end, n int) {
	if be.expr.isBase {
		for j := start; j < end; j++ {
			pVal := be.expr.evalBase(j, n)
			if be.cancellation != nil {
				pVal.Mul(&pVal, &be.cancellation[j])
			}
			var term field.Ext
			term.MulByElement(&be.coinPow, &pVal)
			aggregate[j].Add(&aggregate[j], &term)
		}
		return
	}
	for j := start; j < end; j++ {
		pVal := be.expr.evalExt(j, n)
		if be.cancellation != nil {
			pVal.MulByElement(&pVal, &be.cancellation[j])
		}
		var term field.Ext
		term.Mul(&be.coinPow, &pVal)
		aggregate[j].Add(&aggregate[j], &term)
	}
}
