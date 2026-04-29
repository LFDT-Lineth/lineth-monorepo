package wioptest

import (
	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/linea-monorepo/prover-ray/wiop"
)

// NewLocalOpeningScenario returns a scenario for LocalOpening.
//
//   - Column: [7, 7, 7, 7]; opening at position 2 → claimed value 7.
//   - Invalid: column honest, Result cell set to 0 (≠ 7).
func NewLocalOpeningScenario() *Scenario {
	sys := wiop.NewSystemf("lo-sc")
	r0 := sys.NewRound()
	sys.NewRound() // r1 needed so LagrangeEval / runtime construction is valid
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	lo := col.At(2).Open(sys.Context.Childf("lo"))

	return &Scenario{
		Name:  "LocalOpening",
		Sys:   sys,
		Query: lo,
		RunHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, baseVec(4, 7))
			lo.SelfAssign(*rt)
		},
		RunInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, baseVec(4, 7))
			// Correct value is 7; assign 0 instead.
			rt.AssignCell(lo.Result, field.ElemZero())
		},
	}
}

// NewLagrangeEvalScenario returns a scenario for LagrangeEval.
//
//   - Column: [3, 3, 3, 3] (constant 3); evaluation point: verifier coin.
//   - The Lagrange evaluation of a constant-3 polynomial is 3 everywhere.
//   - Invalid: column honest, claim cell set to 0 (≠ 3).
func NewLagrangeEvalScenario() *Scenario {
	sys := wiop.NewSystemf("le-sc")
	r0 := sys.NewRound()
	r1 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	coin := r1.NewCoinField(sys.Context.Childf("coin"))
	le := sys.NewLagrangeEval(sys.Context.Childf("le"), []*wiop.ColumnView{col.View()}, coin)

	return &Scenario{
		Name:  "LagrangeEval",
		Sys:   sys,
		Query: le,
		RunHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, baseVec(4, 3))
			rt.AdvanceRound() // sample coin
			le.SelfAssign(*rt)
		},
		RunInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, baseVec(4, 3))
			rt.AdvanceRound() // sample coin
			// Real evaluation is 3; claim 0 instead.
			rt.AssignCell(le.EvaluationClaims[0], field.ElemZero())
		},
	}
}

// NewLogDerivativeSumScenario returns a scenario for LogDerivativeSum.
//
//   - Column: [2, 2, 2, 2]; denominator: constant 1; sum = 4 × 2 = 8.
//   - Invalid: column honest, Result cell set to 0 (≠ 8).
func NewLogDerivativeSumScenario() *Scenario {
	sys := wiop.NewSystemf("rr-sc")
	r0 := sys.NewRound()
	sys.NewRound() // result cell goes here
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)

	den := wiop.NewConstantVector(mod, field.NewFromString("1"))
	rr := sys.NewLogDerivativeSum(
		sys.Context.Childf("rr"),
		[]wiop.Fraction{{Numerator: col.View(), Denominator: den}},
	)

	return &Scenario{
		Name:  "LogDerivativeSum",
		Sys:   sys,
		Query: rr,
		RunHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, baseVec(4, 2))
			rt.AdvanceRound()
			rr.SelfAssign(*rt)
		},
		RunInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, baseVec(4, 2))
			rt.AdvanceRound()
			// Real result is 8; claim 0 instead.
			rt.AssignCell(rr.Result, field.ElemZero())
		},
	}
}

// NewPermutationScenario returns a scenario for a Permutation TableRelation.
//
//   - Valid: A = [1, 2, 3, 4], B = [2, 1, 4, 3] (same multiset, rows shuffled).
//   - Invalid: A = [1, 2, 3, 4], B = [1, 2, 3, 5] (5 ≠ 4; different multisets).
func NewPermutationScenario() *Scenario {
	sys := wiop.NewSystemf("perm-sc")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	colA := mod.NewColumn(sys.Context.Childf("colA"), wiop.VisibilityOracle, r0)
	colB := mod.NewColumn(sys.Context.Childf("colB"), wiop.VisibilityOracle, r0)
	perm := sys.NewPermutation(
		sys.Context.Childf("perm"),
		[]wiop.Table{wiop.NewTable(colA.View())},
		[]wiop.Table{wiop.NewTable(colB.View())},
	)

	return &Scenario{
		Name:  "Permutation",
		Sys:   sys,
		Query: perm,
		RunHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(colA, makeVec(1, 2, 3, 4))
			rt.AssignColumn(colB, makeVec(2, 1, 4, 3)) // same multiset, different order
		},
		RunInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(colA, makeVec(1, 2, 3, 4))
			rt.AssignColumn(colB, makeVec(1, 2, 3, 5)) // 5 replaces 4 → different multisets
		},
	}
}

// NewRangeCheckScenario returns a scenario for a RangeCheck query.
//
//   - Valid: column [0, 1, 2, 3]; bound B = 8.
//   - Invalid: column [0, 1, 2, 9]; 9 ≥ 8.
func NewRangeCheckScenario() *Scenario {
	sys := wiop.NewSystemf("rc-sc")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	rc := mod.NewRangeCheck(sys.Context.Childf("rc"), col, 8)

	return &Scenario{
		Name:  "RangeCheck",
		Sys:   sys,
		Query: rc,
		RunHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(0, 1, 2, 3))
		},
		RunInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(0, 1, 2, 9)) // 9 ≥ B=8
		},
	}
}

// NewInclusionScenario returns a scenario for an Inclusion TableRelation.
//
//   - Valid: A = [1, 1, 2, 2], B = [1, 2, 3, 4] (every A value is in B).
//   - Invalid: A = [1, 1, 2, 5], B = [1, 2, 3, 4] (5 ∉ B).
func NewInclusionScenario() *Scenario {
	sys := wiop.NewSystemf("inc-sc")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	colA := mod.NewColumn(sys.Context.Childf("colA"), wiop.VisibilityOracle, r0)
	colB := mod.NewColumn(sys.Context.Childf("colB"), wiop.VisibilityOracle, r0)
	inc := sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTable(colA.View())},
		[]wiop.Table{wiop.NewTable(colB.View())},
	)

	return &Scenario{
		Name:  "Inclusion",
		Sys:   sys,
		Query: inc,
		RunHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(colA, makeVec(1, 1, 2, 2))
			rt.AssignColumn(colB, makeVec(1, 2, 3, 4))
		},
		RunInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(colA, makeVec(1, 1, 2, 5)) // 5 not in B
			rt.AssignColumn(colB, makeVec(1, 2, 3, 4))
		},
	}
}
