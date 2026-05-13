package wiop

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
)

// Fraction is a filter-aware fraction triple. The semantic contribution of a
// Fraction to its enclosing [LogDerivativeSum] is
//
//	Σ_row Filter[row] · Numerator[row] / Denominator[row]
//
// Filter is optional: a nil Filter is treated as the constant 1. When Filter
// is provided, the caller is expected to use a {0, 1}-valued column so the
// contribution at row i is either Numerator[i]/Denominator[i] (when
// Filter[i] = 1) or 0 (when Filter[i] = 0); other values yield a generalised
// weighted sum.
//
// At least one of Numerator or Denominator must be vector-valued
// (IsMultiValued() == true). If Filter is non-nil and vector-valued, it must
// share the same module as the vector-valued side. These invariants are
// enforced by [System.NewLogDerivativeSum].
//
// On rows where Filter[i] is zero, the prover side of the [logderivativesum]
// compiler skips the inversion of Denominator[i]; that row therefore does not
// require Denominator[i] to be invertible. The constraint system, however,
// inherits the same non-zero-denominator soundness assumption as
// [LogDerivativeSum]: callers should ensure denominators are non-zero on
// every row (typically by binding them to a randomness coin).
type Fraction struct {
	// Filter is an optional 0/1 selector. A nil Filter means "always active"
	// and is equivalent to a constant-1 expression.
	Filter Expression
	// Numerator is the top expression. Always non-nil.
	Numerator Expression
	// Denominator is the bottom expression. Always non-nil.
	Denominator Expression
}

// LogDerivativeSum is a filter-aware [Query] that reduces a list of
// [Fraction] objects to a single field-element result stored in a [Cell]:
//
//	Result = ∑_k ∑_row Filter_k[row] · Num_k[row] / Den_k[row]
//
// It is the natural target for conditional lookups, where rows with
// Filter_k[row] = 0 should not contribute to the running sum even if their
// numerator/denominator would otherwise be ill-defined on those rows.
//
// LogDerivativeSum implements [AssignableQuery] but not [GnarkCheckableQuery]:
// a compiler pass must reduce it before gnark verification.
//
// Use [System.NewLogDerivativeSum] to construct and register an instance.
type LogDerivativeSum struct {
	baseQuery
	// Fractions is the ordered list of filter-aware fraction triples. Contains
	// at least one entry.
	Fractions []Fraction
	// Result is the cell holding the prover's claimed aggregated value.
	// Allocated automatically by the constructor.
	Result *Cell
}

// Round implements [Query]. Returns the round of the [Result] cell, which is
// the round immediately following the latest column round across all
// fraction expressions (numerator, denominator, and filter).
func (rr *LogDerivativeSum) Round() *Round {
	return rr.Result.Round()
}

// IsAlreadyAssigned implements [AssignableQuery]. Reports whether the Result
// cell already holds a runtime assignment.
func (rr *LogDerivativeSum) IsAlreadyAssigned(rt Runtime) bool {
	return rt.HasCellAssignment(rr.Result)
}

// SelfAssign implements [AssignableQuery]. Computes the filter-aware rational
// reduction from the runtime column assignments and writes the result into
// Result.
func (rr *LogDerivativeSum) SelfAssign(rt Runtime) {
	rt.AssignCell(rr.Result, rr.reduce(rt))
}

// Check implements [Query]. Verifies that the Result cell holds the correct
// aggregated value ∑_k ∑_row Filter_k[row] · Num_k[row] / Den_k[row].
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

// reduce computes ∑_k ∑_row Filter_k[row] · Num_k[row] / Den_k[row] from the
// runtime assignments. It is the shared core of [SelfAssign] and [Check].
//
// Rows where Filter_k[row] is zero are skipped — neither the inversion of
// Den_k[row] nor the multiplication by Num_k[row] is performed. Panics on a
// zero denominator only when the corresponding filter value is non-zero.
func (rr *LogDerivativeSum) reduce(rt Runtime) field.Gen {
	acc := field.ElemZero()

	for _, f := range rr.Fractions {
		var (
			numIsVec    = f.Numerator.IsMultiValued()
			denIsVec    = f.Denominator.IsMultiValued()
			hasFilter   = f.Filter != nil
			filterIsVec = hasFilter && f.Filter.IsMultiValued()

			numVec       ConcreteVector
			denVec       ConcreteVector
			filterVec    ConcreteVector
			numScalar    ConcreteField
			denScalar    ConcreteField
			filterScalar ConcreteField
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
		if hasFilter {
			if filterIsVec {
				filterVec = f.Filter.EvaluateVector(rt)
			} else {
				filterScalar = f.Filter.EvaluateSingle(rt)
			}
		}

		if !numIsVec && !denIsVec {
			panic("wiop: LogDerivativeSum.reduce: fraction has no vector-valued side; at least one of numerator or denominator must be multi-valued")
		}

		var n int
		switch {
		case numIsVec:
			n = numVec.Plain.Len()
		case denIsVec:
			n = denVec.Plain.Len()
		}

		readGen := func(scalar ConcreteField, vec ConcreteVector, isVec bool, row int) field.Gen {
			if !isVec {
				return scalar.Value
			}
			fv := vec.Plain
			if fv.IsBase() {
				return field.ElemFromBase(fv.AsBase()[row])
			}
			return field.ElemFromExt(fv.AsExt()[row])
		}

		for row := 0; row < n; row++ {
			if hasFilter {
				flt := readGen(filterScalar, filterVec, filterIsVec, row)
				if flt.IsZero() {
					continue
				}
				num := readGen(numScalar, numVec, numIsVec, row)
				den := readGen(denScalar, denVec, denIsVec, row)
				acc = acc.Add(flt.Mul(num.Div(den)))
				continue
			}
			num := readGen(numScalar, numVec, numIsVec, row)
			den := readGen(denScalar, denVec, denIsVec, row)
			acc = acc.Add(num.Div(den))
		}
	}

	return acc
}

// NewLogDerivativeSum constructs and registers a [LogDerivativeSum] query on
// sys. A fresh [Cell] is allocated automatically for the result, placed in the
// round immediately following the latest column round across all fraction
// expressions (numerator, denominator, and filter).
//
// Invariants enforced at construction:
//   - len(fractions) ≥ 1.
//   - For each Fraction: Numerator and Denominator are both non-nil. Filter
//     is allowed to be nil (treated as constant 1).
//   - For each Fraction: at least one of Numerator or Denominator is
//     vector-valued (IsMultiValued() == true, i.e. Module() != nil).
//   - For each Fraction: any non-nil vector-valued side (Filter, Numerator,
//     Denominator) must share the same module.
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

		// The filter, if vector-valued, must share the module of the
		// vector-valued numerator / denominator side.
		if f.Filter != nil {
			if filtM := f.Filter.Module(); filtM != nil {
				other := numM
				if other == nil {
					other = denM
				}
				if other != nil && filtM != other {
					panic(fmt.Sprintf(
						"wiop: System.NewLogDerivativeSum: fraction %d Filter module %q differs from the fraction's module %q",
						i, filtM.Context.Path(), other.Context.Path(),
					))
				}
			}
		}

		exprs := [3]Expression{f.Numerator, f.Denominator, f.Filter}
		for _, expr := range exprs {
			if expr == nil {
				continue
			}
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
