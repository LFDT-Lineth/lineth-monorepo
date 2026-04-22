package wiop

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
)

// Fraction is a value type pairing a Numerator and a Denominator expression.
// Both fields are always non-nil: use [Module.NewConstant] with value 1 to
// express a unit numerator or denominator.
//
// At least one of the two must be vector-valued (IsMultiValued() == true). If
// both are vector-valued they must reference the same module. These invariants
// are enforced by [System.NewLogDerivativeSum].
type Fraction struct {
	// Numerator is the top expression. Always non-nil.
	Numerator Expression
	// Denominator is the bottom expression. Always non-nil.
	Denominator Expression
}

// LogDerivativeSum is a [Query] that reduces a list of [Fraction] objects
// to a single field-element result stored in a [Cell]:
//
//	Result = ∑_k ∑_row Num_k[row] / Den_k[row]
//
// This unifies LogDerivativeSum and InnerProduct (InnerProduct is the special
// case where each denominator is the constant 1).
//
// Different fractions may span different modules; no cross-fraction module
// constraint is imposed. The Result cell is allocated automatically in the
// round immediately following the latest column round across all fractions.
//
// LogDerivativeSum implements [AssignableQuery] but not [GnarkCheckableQuery]:
// a compiler pass must reduce it before gnark verification.
//
// Use [System.NewLogDerivativeSum] to construct and register an instance.
type LogDerivativeSum struct {
	baseQuery
	// Fractions is the ordered list of rational expression pairs. Contains
	// at least one entry.
	Fractions []Fraction
	// Result is the cell holding the prover's claimed aggregated value.
	// Allocated automatically by the constructor.
	Result *Cell
}

// Round implements [Query]. Returns the round of the [Result] cell, which is
// the round immediately following the latest column round across all fractions.
func (rr *LogDerivativeSum) Round() *Round {
	return rr.Result.Round()
}

// IsAlreadyAssigned implements [AssignableQuery]. Reports whether the Result
// cell already holds a runtime assignment.
func (rr *LogDerivativeSum) IsAlreadyAssigned(rt Runtime) bool {
	return rt.HasCellAssignment(rr.Result)
}

// SelfAssign implements [AssignableQuery]. Computes the rational reduction
// from the runtime column assignments and writes the result into Result.
func (rr *LogDerivativeSum) SelfAssign(rt Runtime) {
	rt.AssignCell(rr.Result, rr.reduce(rt))
}

// Check implements [Query]. Verifies that the Result cell holds the correct
// aggregated value ∑_k ∑_row Num_k[row] / Den_k[row].
// Returns an error if the claimed Result cell does not match.
func (rr *LogDerivativeSum) Check(rt Runtime) error {
	acc := rr.reduce(rt)
	got := rt.GetCellValue(rr.Result)
	diff := acc.Sub(got)
	if !diff.IsZero() {
		return fmt.Errorf(
			"wiop: LogDerivativeSum.Check(%s): result mismatch",
			rr.context.Path(),
		)
	}
	return nil
}

// reduce computes ∑_k ∑_row Num_k[row] / Den_k[row] from the runtime
// assignments. It is the shared core of [SelfAssign] and [Check].
//
// Panics on a zero denominator.
func (rr *LogDerivativeSum) reduce(rt Runtime) field.Gen {
	acc := field.ElemZero()

	for _, f := range rr.Fractions {
		// Determine the row count from whichever expression is vector-valued.
		var n int
		if f.Numerator.IsMultiValued() {
			n = f.Numerator.Size()
		} else {
			n = f.Denominator.Size()
		}

		var (
			numIsVec  = f.Numerator.IsMultiValued()
			denIsVec  = f.Denominator.IsMultiValued()
			numVec    ConcreteVector
			denVec    ConcreteVector
			numScalar ConcreteField
			denScalar ConcreteField
		)

		if numIsVec {
			numVec = f.Numerator.EvaluateVector(rt)
		} else {
			numScalar = f.Numerator.EvaluateSingle(rt)
		}
		if denIsVec {
			denVec = f.Denominator.EvaluateVector(rt)
		} else {
			denScalar = f.Denominator.EvaluateSingle(rt)
		}

		for row := 0; row < n; row++ {
			var num, den field.Gen

			if numIsVec {
				fv := numVec.Plain[0]
				if fv.IsBase() {
					num = field.ElemFromBase(fv.AsBase()[row])
				} else {
					num = field.ElemFromExt(fv.AsExt()[row])
				}
			} else {
				num = numScalar.Value
			}

			if denIsVec {
				fv := denVec.Plain[0]
				if fv.IsBase() {
					den = field.ElemFromBase(fv.AsBase()[row])
				} else {
					den = field.ElemFromExt(fv.AsExt()[row])
				}
			} else {
				den = denScalar.Value
			}

			acc = acc.Add(num.Div(den))
		}
	}

	return acc
}

// NewLogDerivativeSum constructs and registers a [LogDerivativeSum] query on
// sys. A fresh [Cell] is allocated automatically for the result, placed in the
// round immediately following the latest column round across all fractions.
//
// Invariants enforced at construction:
//   - len(fractions) ≥ 1.
//   - For each Fraction: Numerator and Denominator are both non-nil.
//   - For each Fraction: at least one of Numerator or Denominator is
//     vector-valued (IsMultiValued() == true, i.e. Module() != nil).
//   - For each Fraction: if both are vector-valued, they must share the same module.
//
// Panics if ctx is nil, any invariant is violated, or no round follows the
// latest fraction column round (call [System.NewRound] first in that case).
func (sys *System) NewLogDerivativeSum(ctx *ContextFrame, fractions []Fraction) *LogDerivativeSum {
	if ctx == nil {
		panic("wiop: System.NewLogDerivativeSum requires a non-nil ContextFrame")
	}
	if len(fractions) == 0 {
		panic("wiop: System.NewLogDerivativeSum requires at least one Fraction")
	}

	var maxFracRound *Round
	isExt := false
	for i, f := range fractions {
		if f.Numerator == nil {
			panic(fmt.Sprintf("wiop: System.NewLogDerivativeSum: fraction %d has a nil Numerator", i))
		}
		if f.Denominator == nil {
			panic(fmt.Sprintf("wiop: System.NewLogDerivativeSum: fraction %d has a nil Denominator", i))
		}
		numM := f.Numerator.Module()
		denM := f.Denominator.Module()
		if numM == nil && denM == nil {
			panic(fmt.Sprintf(
				"wiop: System.NewLogDerivativeSum: fraction %d has no vector-valued expression; "+
					"at least one of Numerator or Denominator must be vector-valued (IsMultiValued() == true)",
				i,
			))
		}
		if numM != nil && denM != nil && numM != denM {
			panic(fmt.Sprintf(
				"wiop: System.NewLogDerivativeSum: fraction %d Numerator module %q and Denominator module %q differ; "+
					"both must share the same module when vector-valued",
				i, numM.Context.Path(), denM.Context.Path(),
			))
		}
		for _, expr := range [2]Expression{f.Numerator, f.Denominator} {
			if r := maxRoundInExpr(expr); r != nil && (maxFracRound == nil || r.ID > maxFracRound.ID) {
				maxFracRound = r
			}
			if expr.IsExtension() {
				isExt = true
			}
		}
	}

	if maxFracRound == nil {
		panic("wiop: System.NewLogDerivativeSum: no column, cell, or coin found in any fraction expression; " +
			"at least one expression must be round-bearing")
	}

	resultRound, ok := maxFracRound.Next()
	if !ok {
		panic(fmt.Sprintf(
			"wiop: System.NewLogDerivativeSum: no round follows round %d (the latest fraction column round); "+
				"call sys.NewRound() before registering this query",
			maxFracRound.ID,
		))
	}

	result := resultRound.NewCell(ctx.Childf("result"), isExt)

	rr := &LogDerivativeSum{
		baseQuery: baseQuery{
			context:     ctx,
			Annotations: make(Annotations),
		},
		Fractions: fractions,
		Result:    result,
	}
	sys.LogDerivativeSums = append(sys.LogDerivativeSums, rr)
	return rr
}
