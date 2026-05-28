package wioptest

import (
	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/linea-monorepo/prover-ray/wiop"
)

// tamperResult returns a TamperResult callback that overwrites ld.Result with
// a fixed wrong value. The base-field literal "123456" is far from any honest
// running-sum value used in the surrounding scenarios.
func tamperResult(ld *wiop.LogDerivativeSum) func(rt *wiop.Runtime) {
	return func(rt *wiop.Runtime) {
		rt.AssignCell(ld.Result, field.ElemFromBase(field.NewFromString("123456")))
	}
}

// NewLDSSingleFractionAllOnesScenario: a single fraction with no filter,
// numerator = vector, denominator = constant 1. Honest sum = column sum.
//
//   - Witness: num = [3, 5, 7, 9] → honest result = 24.
//   - Tamper: Result cell pinned to a wrong value (verifier rejects).
func NewLDSSingleFractionAllOnesScenario() *LogDerivativeSumCompilerScenario {
	sys := wiop.NewSystemf("lds-ones")
	r0 := sys.NewRound()
	sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	num := mod.NewColumn(sys.Context.Childf("num"), wiop.VisibilityOracle, r0)
	one := wiop.NewConstantVector(mod, field.NewFromString("1"))
	ld := sys.NewLogDerivativeSum(
		sys.Context.Childf("ld"),
		[]wiop.Fraction{{Numerator: num.View(), Denominator: one}},
	)

	return &LogDerivativeSumCompilerScenario{
		Name: "SingleFractionAllOnes",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			rt.AssignColumn(num, makeVec(3, 5, 7, 9))
		},
		TamperResult: tamperResult(ld),
	}
}

// NewLDSPartialFilterScenario: a filter masks individual rows.
//
//   - Witness: num = [3, 5, 7, 9], filter = [1, 0, 1, 0] → honest result = 10.
//   - Tamper: Result cell pinned to a wrong value.
func NewLDSPartialFilterScenario() *LogDerivativeSumCompilerScenario {
	sys := wiop.NewSystemf("lds-partial")
	r0 := sys.NewRound()
	sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	num := mod.NewColumn(sys.Context.Childf("num"), wiop.VisibilityOracle, r0)
	flt := mod.NewColumn(sys.Context.Childf("flt"), wiop.VisibilityOracle, r0)
	one := wiop.NewConstantVector(mod, field.NewFromString("1"))
	ld := sys.NewLogDerivativeSum(
		sys.Context.Childf("ld"),
		[]wiop.Fraction{{Filter: flt.View(), Numerator: num.View(), Denominator: one}},
	)

	return &LogDerivativeSumCompilerScenario{
		Name: "PartialFilter",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			rt.AssignColumn(num, makeVec(3, 5, 7, 9))
			rt.AssignColumn(flt, makeVec(1, 0, 1, 0))
		},
		TamperResult: tamperResult(ld),
	}
}

// NewLDSAllZeroFilterScenario: an all-zero filter must produce a zero
// running sum without panicking, even with non-trivial numerator and
// denominator on the masked rows.
//
//   - Witness: num arbitrary, filter = [0, 0, 0, 0] → honest result = 0.
//   - Tamper: Result cell pinned to a non-zero value.
func NewLDSAllZeroFilterScenario() *LogDerivativeSumCompilerScenario {
	sys := wiop.NewSystemf("lds-zeros")
	r0 := sys.NewRound()
	sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	num := mod.NewColumn(sys.Context.Childf("num"), wiop.VisibilityOracle, r0)
	flt := mod.NewColumn(sys.Context.Childf("flt"), wiop.VisibilityOracle, r0)
	one := wiop.NewConstantVector(mod, field.NewFromString("1"))
	ld := sys.NewLogDerivativeSum(
		sys.Context.Childf("ld"),
		[]wiop.Fraction{{Filter: flt.View(), Numerator: num.View(), Denominator: one}},
	)

	return &LogDerivativeSumCompilerScenario{
		Name: "AllZeroFilter",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			rt.AssignColumn(num, makeVec(3, 5, 7, 9))
			rt.AssignColumn(flt, makeVec(0, 0, 0, 0))
		},
		TamperResult: tamperResult(ld),
	}
}

// NewLDSFilterMasksZeroDenominatorScenario covers the headline feature:
// rows with zero denominator are OK when the filter masks them.
//
//   - Witness: num = [4, 99, 8, 99], den = [2, 0, 4, 0], filter = [1, 0, 1, 0]
//     → honest result = 4/2 + 8/4 = 4.
//   - Tamper: Result cell pinned to a wrong value.
func NewLDSFilterMasksZeroDenominatorScenario() *LogDerivativeSumCompilerScenario {
	sys := wiop.NewSystemf("lds-maskzero")
	r0 := sys.NewRound()
	sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	num := mod.NewColumn(sys.Context.Childf("num"), wiop.VisibilityOracle, r0)
	den := mod.NewColumn(sys.Context.Childf("den"), wiop.VisibilityOracle, r0)
	flt := mod.NewColumn(sys.Context.Childf("flt"), wiop.VisibilityOracle, r0)
	ld := sys.NewLogDerivativeSum(
		sys.Context.Childf("ld"),
		[]wiop.Fraction{{Filter: flt.View(), Numerator: num.View(), Denominator: den.View()}},
	)

	return &LogDerivativeSumCompilerScenario{
		Name: "FilterMasksZeroDenominator",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			rt.AssignColumn(num, makeVec(4, 99, 8, 99))
			rt.AssignColumn(den, makeVec(2, 0, 4, 0))
			rt.AssignColumn(flt, makeVec(1, 0, 1, 0))
		},
		TamperResult: tamperResult(ld),
	}
}

// NewLDSPackingScenario registers four fractions on the same module,
// exercising the packing arity = 3 split: the compiler allocates two Z
// columns (⌈4/3⌉ = 2).
//
//   - Witness: per-column row sums add up.
//   - Tamper: Result cell pinned to a wrong value.
func NewLDSPackingScenario() *LogDerivativeSumCompilerScenario {
	sys := wiop.NewSystemf("lds-pack")
	r0 := sys.NewRound()
	sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	one := wiop.NewConstantVector(mod, field.NewFromString("1"))
	cols := make([]*wiop.Column, 4)
	fractions := make([]wiop.Fraction, 4)
	for i := range cols {
		cols[i] = mod.NewColumn(sys.Context.Childf("c%d", i), wiop.VisibilityOracle, r0)
		fractions[i] = wiop.Fraction{Numerator: cols[i].View(), Denominator: one}
	}
	ld := sys.NewLogDerivativeSum(sys.Context.Childf("ld"), fractions)

	return &LogDerivativeSumCompilerScenario{
		Name: "Packing4Fractions",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			rt.AssignColumn(cols[0], makeVec(1, 2, 3, 4))
			rt.AssignColumn(cols[1], makeVec(5, 6, 7, 8))
			rt.AssignColumn(cols[2], makeVec(9, 10, 11, 12))
			rt.AssignColumn(cols[3], makeVec(13, 14, 15, 16))
		},
		TamperResult: tamperResult(ld),
	}
}

// NewLDSMultiModuleBucketingScenario places fractions on two different
// modules, exercising the per-module bucketing of the compiler. Each module
// gets its own Z column.
//
//   - Witness: sum_A = 10, sum_B (filtered) = 11 → total = 21.
//   - Tamper: Result cell pinned to a wrong value.
func NewLDSMultiModuleBucketingScenario() *LogDerivativeSumCompilerScenario {
	sys := wiop.NewSystemf("lds-multi-mod")
	r0 := sys.NewRound()
	sys.NewRound()
	mA := sys.NewSizedModule(sys.Context.Childf("mA"), 4, wiop.PaddingDirectionNone)
	mB := sys.NewSizedModule(sys.Context.Childf("mB"), 4, wiop.PaddingDirectionNone)
	cA := mA.NewColumn(sys.Context.Childf("cA"), wiop.VisibilityOracle, r0)
	cB := mB.NewColumn(sys.Context.Childf("cB"), wiop.VisibilityOracle, r0)
	fB := mB.NewColumn(sys.Context.Childf("fB"), wiop.VisibilityOracle, r0)
	oneA := wiop.NewConstantVector(mA, field.NewFromString("1"))
	oneB := wiop.NewConstantVector(mB, field.NewFromString("1"))
	ld := sys.NewLogDerivativeSum(sys.Context.Childf("ld"), []wiop.Fraction{
		{Numerator: cA.View(), Denominator: oneA},
		{Filter: fB.View(), Numerator: cB.View(), Denominator: oneB},
	})

	return &LogDerivativeSumCompilerScenario{
		Name: "MultiModuleBucketing",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			rt.AssignColumn(cA, makeVec(1, 2, 3, 4))
			rt.AssignColumn(cB, makeVec(5, 6, 7, 8))
			rt.AssignColumn(fB, makeVec(1, 1, 0, 0))
		},
		TamperResult: tamperResult(ld),
	}
}

// NewLDSSizeOneModuleScenario: a module of size 1 has no recurrence; only
// the initial-condition / endpoint identity ties Z[0] to the fraction's
// row-0 value. Exercises the size-1 branch of the compiler.
//
//   - Witness: single fraction with num/1 = 17 → honest result = 17.
//   - Tamper: Result cell pinned to a wrong value.
func NewLDSSizeOneModuleScenario() *LogDerivativeSumCompilerScenario {
	sys := wiop.NewSystemf("lds-size1")
	r0 := sys.NewRound()
	sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 1, wiop.PaddingDirectionNone)
	num := mod.NewColumn(sys.Context.Childf("num"), wiop.VisibilityOracle, r0)
	one := wiop.NewConstantVector(mod, field.NewFromString("1"))
	ld := sys.NewLogDerivativeSum(
		sys.Context.Childf("ld"),
		[]wiop.Fraction{{Numerator: num.View(), Denominator: one}},
	)

	return &LogDerivativeSumCompilerScenario{
		Name: "SizeOneModule",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			rt.AssignColumn(num, makeVec(17))
		},
		TamperResult: tamperResult(ld),
	}
}

// NewLDSManyFractionsScenario registers seven fractions on a single
// module. Packing arity is 3, so the compiler must allocate ⌈7/3⌉ = 3 Z
// columns — the first scenario that goes beyond two packed Z chunks.
//
//   - Witness: each column has row sums 1+2+3+4 = 10; total = 7·10 = 70.
//   - Tamper: Result cell pinned to a wrong value.
func NewLDSManyFractionsScenario() *LogDerivativeSumCompilerScenario {
	sys := wiop.NewSystemf("lds-many")
	r0 := sys.NewRound()
	sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	one := wiop.NewConstantVector(mod, field.NewFromString("1"))
	cols := make([]*wiop.Column, 7)
	fractions := make([]wiop.Fraction, 7)
	for i := range cols {
		cols[i] = mod.NewColumn(sys.Context.Childf("c%d", i), wiop.VisibilityOracle, r0)
		fractions[i] = wiop.Fraction{Numerator: cols[i].View(), Denominator: one}
	}
	ld := sys.NewLogDerivativeSum(sys.Context.Childf("ld"), fractions)

	return &LogDerivativeSumCompilerScenario{
		Name: "ManyFractions",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			for _, c := range cols {
				rt.AssignColumn(c, makeVec(1, 2, 3, 4))
			}
		},
		TamperResult: tamperResult(ld),
	}
}

// NewLDSSizeTwoModuleScenario: a size-2 module — the smallest non-trivial
// module. The Z column has exactly two rows so the recurrence has one
// active step.
func NewLDSSizeTwoModuleScenario() *LogDerivativeSumCompilerScenario {
	sys := wiop.NewSystemf("lds-size2")
	r0 := sys.NewRound()
	sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 2, wiop.PaddingDirectionNone)
	num := mod.NewColumn(sys.Context.Childf("num"), wiop.VisibilityOracle, r0)
	one := wiop.NewConstantVector(mod, field.NewFromString("1"))
	ld := sys.NewLogDerivativeSum(
		sys.Context.Childf("ld"),
		[]wiop.Fraction{{Numerator: num.View(), Denominator: one}},
	)

	return &LogDerivativeSumCompilerScenario{
		Name: "SizeTwoModule",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			rt.AssignColumn(num, makeVec(3, 5))
		},
		TamperResult: tamperResult(ld),
	}
}

// NewLDSMultipleQueriesScenario registers two distinct [wiop.LogDerivativeSum]
// queries on the same system. The compiler must process each independently;
// each gets its own Z column, recurrence, and verifier action.
//
// We tamper the FIRST query's Result cell only; the second's verifier check
// continues to accept, but the global verifier still rejects (a single
// per-query failure is enough).
func NewLDSMultipleQueriesScenario() *LogDerivativeSumCompilerScenario {
	sys := wiop.NewSystemf("lds-multi-q")
	r0 := sys.NewRound()
	sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	one := wiop.NewConstantVector(mod, field.NewFromString("1"))
	c1 := mod.NewColumn(sys.Context.Childf("c1"), wiop.VisibilityOracle, r0)
	c2 := mod.NewColumn(sys.Context.Childf("c2"), wiop.VisibilityOracle, r0)
	ld1 := sys.NewLogDerivativeSum(
		sys.Context.Childf("ld1"),
		[]wiop.Fraction{{Numerator: c1.View(), Denominator: one}},
	)
	sys.NewLogDerivativeSum(
		sys.Context.Childf("ld2"),
		[]wiop.Fraction{{Numerator: c2.View(), Denominator: one}},
	)

	return &LogDerivativeSumCompilerScenario{
		Name: "MultipleQueries",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			rt.AssignColumn(c1, makeVec(1, 2, 3, 4)) // ld1.Result = 10
			rt.AssignColumn(c2, makeVec(5, 6, 7, 8)) // ld2.Result = 26
		},
		TamperResult: tamperResult(ld1),
	}
}

// NewLDSVectorDenominatorScenario: the denominator is a non-trivial vector
// (not just the constant 1), so per-row denominator inversion is exercised
// in the prover task.
//
//   - Witness: num = [6, 10, 15, 21], den = [2, 5, 3, 7] → 3+2+5+3 = 13.
//   - Tamper: Result cell pinned to a wrong value.
func NewLDSVectorDenominatorScenario() *LogDerivativeSumCompilerScenario {
	sys := wiop.NewSystemf("lds-vec-den")
	r0 := sys.NewRound()
	sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	num := mod.NewColumn(sys.Context.Childf("num"), wiop.VisibilityOracle, r0)
	den := mod.NewColumn(sys.Context.Childf("den"), wiop.VisibilityOracle, r0)
	ld := sys.NewLogDerivativeSum(
		sys.Context.Childf("ld"),
		[]wiop.Fraction{{Numerator: num.View(), Denominator: den.View()}},
	)

	return &LogDerivativeSumCompilerScenario{
		Name: "VectorDenominator",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			rt.AssignColumn(num, makeVec(6, 10, 15, 21))
			rt.AssignColumn(den, makeVec(2, 5, 3, 7)) // ratios 3, 2, 5, 3 → sum 13
		},
		TamperResult: tamperResult(ld),
	}
}

// NewLDSAllFiltersOnesPackedScenario: three filtered fractions that all
// pack into one Z column (packing arity = 3), with every filter equal to 1.
// This pins down that the filter-aware prover task agrees with the
// non-filter path when filters are vacuous.
//
//   - Witness: num1 = [1,1,1,1], num2 = [2,2,2,2], num3 = [3,3,3,3] →
//     filtered sums 4, 8, 12 → total = 24.
//   - Tamper: Result cell pinned to a wrong value.
func NewLDSAllFiltersOnesPackedScenario() *LogDerivativeSumCompilerScenario {
	sys := wiop.NewSystemf("lds-ones-pack")
	r0 := sys.NewRound()
	sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	num1 := mod.NewColumn(sys.Context.Childf("n1"), wiop.VisibilityOracle, r0)
	num2 := mod.NewColumn(sys.Context.Childf("n2"), wiop.VisibilityOracle, r0)
	num3 := mod.NewColumn(sys.Context.Childf("n3"), wiop.VisibilityOracle, r0)
	flt := mod.NewColumn(sys.Context.Childf("flt"), wiop.VisibilityOracle, r0)
	one := wiop.NewConstantVector(mod, field.NewFromString("1"))
	ld := sys.NewLogDerivativeSum(
		sys.Context.Childf("ld"),
		[]wiop.Fraction{
			{Filter: flt.View(), Numerator: num1.View(), Denominator: one},
			{Filter: flt.View(), Numerator: num2.View(), Denominator: one},
			{Filter: flt.View(), Numerator: num3.View(), Denominator: one},
		},
	)

	return &LogDerivativeSumCompilerScenario{
		Name: "AllFiltersOnesPacked",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			rt.AssignColumn(num1, makeVec(1, 1, 1, 1))
			rt.AssignColumn(num2, makeVec(2, 2, 2, 2))
			rt.AssignColumn(num3, makeVec(3, 3, 3, 3))
			rt.AssignColumn(flt, makeVec(1, 1, 1, 1))
		},
		TamperResult: tamperResult(ld),
	}
}

// NewLDSConditionalLookupShapeScenario models a small conditional-lookup
// pattern that aggregates to zero:
//
//	Σ_S filterS / (γ + S) + Σ_T (−M) / (γ + T) = 0.
//
// γ is a fixed constant for testing convenience.
//
//   - Witness: honest M cancels every filtered-in S row in T → result = 0.
//   - Tamper: Result cell pinned to a non-zero value.
func NewLDSConditionalLookupShapeScenario() *LogDerivativeSumCompilerScenario {
	sys := wiop.NewSystemf("lds-cond")
	r0 := sys.NewRound()
	sys.NewRound()
	mS := sys.NewSizedModule(sys.Context.Childf("mS"), 4, wiop.PaddingDirectionNone)
	mT := sys.NewSizedModule(sys.Context.Childf("mT"), 2, wiop.PaddingDirectionNone)

	colS := mS.NewColumn(sys.Context.Childf("S"), wiop.VisibilityOracle, r0)
	filterS := mS.NewColumn(sys.Context.Childf("filterS"), wiop.VisibilityOracle, r0)
	colT := mT.NewColumn(sys.Context.Childf("T"), wiop.VisibilityOracle, r0)
	colM := mT.NewColumn(sys.Context.Childf("M"), wiop.VisibilityOracle, r0)

	gammaS := wiop.NewConstantVector(mS, field.NewFromString("7"))
	gammaT := wiop.NewConstantVector(mT, field.NewFromString("7"))
	denS := wiop.Add(gammaS, colS.View())
	denT := wiop.Add(gammaT, colT.View())
	negM := wiop.Negate(colM.View())
	oneS := wiop.NewConstantVector(mS, field.NewFromString("1"))

	ld := sys.NewLogDerivativeSum(sys.Context.Childf("ld"), []wiop.Fraction{
		{Filter: filterS.View(), Numerator: oneS, Denominator: denS},
		{Numerator: negM, Denominator: denT},
	})

	return &LogDerivativeSumCompilerScenario{
		Name: "ConditionalLookupShape",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			rt.AssignColumn(colS, makeVec(10, 10, 20, 99))
			rt.AssignColumn(filterS, makeVec(1, 1, 1, 0))
			rt.AssignColumn(colT, makeVec(10, 20))
			rt.AssignColumn(colM, makeVec(2, 1))
		},
		TamperResult: tamperResult(ld),
	}
}
