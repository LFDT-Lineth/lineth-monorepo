package wioptest

// Each factory function returns a [*VanishingScenario] backed by a freshly
// allocated [*wiop.System]. Calling the same factory twice yields two
// independent fixtures; nothing is shared between successive calls.

import (
	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/linea-monorepo/prover-ray/wiop"
)

// NewFibonacciVanishingScenario returns a scenario for the Fibonacci recurrence
// A[i] − A[i−1] − A[i−2] = 0.
//
//   - Valid: A = [1, 1, 2, 3, 5, 8, 13, 21].
//   - Invalid: A = [1, 1, 2, 3, 5, 8, 13, 22] (last value wrong).
//
// Rows 0 and 1 are automatically cancelled (back-shifts of 1 and 2).
func NewFibonacciVanishingScenario() *VanishingScenario {
	sys := wiop.NewSystemf("fib")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 8, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	// A[i] − A[i−1] − A[i−2] = 0
	mod.NewVanishing(
		sys.Context.Childf("fib"),
		wiop.Sub(
			wiop.Sub(col.View(), col.View().Shift(-1)),
			col.View().Shift(-2),
		),
	)

	return &VanishingScenario{
		Name: "Fibonacci",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(1, 1, 2, 3, 5, 8, 13, 21))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(1, 1, 2, 3, 5, 8, 13, 22)) // last value off by one
		},
	}
}

// NewGeometricProgressionVanishingScenario returns a scenario for the
// doubling recurrence A[i] − 2·A[i−1] = 0.
//
//   - Valid: A = [1, 2, 4, 8, 16, 32, 64, 128].
//   - Invalid: A = [1, 3, 9, 27, 81, 243, 729, 2187] (factor 3 instead of 2).
//
// Row 0 is automatically cancelled (back-shift of 1).
func NewGeometricProgressionVanishingScenario() *VanishingScenario {
	sys := wiop.NewSystemf("geo")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 8, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	two := wiop.NewConstantField(field.NewFromString("2"))
	// A[i] − 2·A[i−1] = 0
	mod.NewVanishing(
		sys.Context.Childf("geo"),
		wiop.Sub(col.View(), wiop.Mul(two, col.View().Shift(-1))),
	)

	return &VanishingScenario{
		Name: "GeometricProgression",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(1, 2, 4, 8, 16, 32, 64, 128))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(1, 3, 9, 27, 81, 243, 729, 2187)) // factor 3
		},
	}
}

// NewConditionalCounterVanishingScenario returns a scenario for the conditional
// increment constraint A[i] − A[i−1] − B[i] = 0, where B is a selector that
// controls whether the counter A advances.
//
//   - Valid: A = [0,1,1,1,2,3,3,3], B = [0,1,0,0,1,1,0,0].
//   - Invalid: A = [0,1,1,1,1,3,3,3], B = [0,1,0,0,1,1,0,0] (row 4 skips an increment).
//
// Row 0 is automatically cancelled (back-shift of 1).
func NewConditionalCounterVanishingScenario() *VanishingScenario {
	sys := wiop.NewSystemf("ctr")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 8, wiop.PaddingDirectionNone)
	colA := mod.NewColumn(sys.Context.Childf("colA"), wiop.VisibilityOracle, r0)
	colB := mod.NewColumn(sys.Context.Childf("colB"), wiop.VisibilityOracle, r0)
	// A[i] − A[i−1] − B[i] = 0
	mod.NewVanishing(
		sys.Context.Childf("ctr"),
		wiop.Sub(wiop.Sub(colA.View(), colA.View().Shift(-1)), colB.View()),
	)

	return &VanishingScenario{
		Name: "ConditionalCounter",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(colA, makeVec(0, 1, 1, 1, 2, 3, 3, 3))
			rt.AssignColumn(colB, makeVec(0, 1, 0, 0, 1, 1, 0, 0))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(colA, makeVec(0, 1, 1, 1, 1, 3, 3, 3)) // row 4: 1 instead of 2
			rt.AssignColumn(colB, makeVec(0, 1, 0, 0, 1, 1, 0, 0))
		},
	}
}

// NewPythagoreanTripletVanishingScenario returns a scenario for the identity
// A² − B² − C² = 0, satisfied by Pythagorean triples.
//
//   - Valid: rows are (0,0,0) or a Pythagorean triple (5,3,4), (1,0,1), etc.
//   - Invalid: C in row 1 changed to 5 so 25−9−25 ≠ 0.
//
// No rows are cancelled (no shifts).
func NewPythagoreanTripletVanishingScenario() *VanishingScenario {
	sys := wiop.NewSystemf("pyth")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 8, wiop.PaddingDirectionNone)
	colA := mod.NewColumn(sys.Context.Childf("colA"), wiop.VisibilityOracle, r0)
	colB := mod.NewColumn(sys.Context.Childf("colB"), wiop.VisibilityOracle, r0)
	colC := mod.NewColumn(sys.Context.Childf("colC"), wiop.VisibilityOracle, r0)
	// A² − B² − C² = 0
	mod.NewVanishing(
		sys.Context.Childf("pyth"),
		wiop.Sub(
			wiop.Sub(wiop.Square(colA.View()), wiop.Square(colB.View())),
			wiop.Square(colC.View()),
		),
	)

	return &VanishingScenario{
		Name: "PythagoreanTriplet",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(colA, makeVec(0, 5, 1, 17, 5, 13, 0, 0))
			rt.AssignColumn(colB, makeVec(0, 3, 0, 15, 4, 5, 0, 0))
			rt.AssignColumn(colC, makeVec(0, 4, 1, 8, 3, 12, 0, 0))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(colA, makeVec(0, 5, 1, 17, 5, 13, 0, 0))
			rt.AssignColumn(colB, makeVec(0, 3, 0, 15, 4, 5, 0, 0))
			rt.AssignColumn(colC, makeVec(0, 5, 1, 8, 3, 12, 0, 0)) // row 1: C=5, 25−9−25≠0
		},
	}
}

// NewBooleanColumnVanishingScenario returns a scenario for the constraint
// col * col − col = 0 (boolean column).
//
//   - Valid: column = [0, 1, 0, 1] — all values are 0 or 1.
//   - Invalid: column = [0, 2, 0, 1] — row 1 is 2, violating col² = col.
func NewBooleanColumnVanishingScenario() *VanishingScenario {
	sys := wiop.NewSystemf("bool-col")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	// col² − col = 0  ⟺  col ∈ {0, 1} on every row.
	mod.NewVanishing(
		sys.Context.Childf("bool"),
		wiop.Sub(wiop.Mul(col.View(), col.View()), col.View()),
	)

	return &VanishingScenario{
		Name: "BooleanColumn",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(0, 1, 0, 1))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(0, 2, 0, 1)) // row 1 is 2: 2²−2 = 2 ≠ 0
		},
	}
}

// NewDynamicFibonacciVanishingScenario is like NewFibonacciVanishingScenario
// but uses a dynamic-size module. The module size is determined at runtime
// from the assigned column length rather than being fixed at compile time.
//
//   - Valid: A = [1, 1, 2, 3, 5, 8, 13, 21].
//   - Invalid: A = [1, 1, 2, 3, 5, 8, 13, 22] (last value wrong).
//
// Rows 0 and 1 are automatically cancelled (back-shifts of 1 and 2).
func NewDynamicFibonacciVanishingScenario() *VanishingScenario {
	sys := wiop.NewSystemf("dyn-fib")
	r0 := sys.NewRound()
	mod := sys.NewDynamicModule(sys.Context.Childf("mod"), wiop.PaddingDirectionRight)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	// A[i] − A[i−1] − A[i−2] = 0
	mod.NewVanishing(
		sys.Context.Childf("fib"),
		wiop.Sub(
			wiop.Sub(col.View(), col.View().Shift(-1)),
			col.View().Shift(-2),
		),
	)

	return &VanishingScenario{
		Name: "DynamicFibonacci",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(1, 1, 2, 3, 5, 8, 13, 21))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(1, 1, 2, 3, 5, 8, 13, 22)) // last value off by one
		},
	}
}

// NewConstantColumnVanishingScenario returns a scenario for the constraint
// col − 7 = 0 (column pinned to a constant). The expression mixes a vector
// view with a scalar constant, which the global compiler must lift to a
// per-row identity. Ratio is 1, no rows are cancelled.
//
//   - Valid: [7, 7, 7, 7].
//   - Invalid: [7, 7, 8, 7] (row 2 violates).
func NewConstantColumnVanishingScenario() *VanishingScenario {
	sys := wiop.NewSystemf("const-col")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	seven := wiop.NewConstantField(field.NewFromString("7"))
	mod.NewVanishing(sys.Context.Childf("const"), wiop.Sub(col.View(), seven))

	return &VanishingScenario{
		Name: "ConstantColumn",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(7, 7, 7, 7))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(7, 7, 8, 7)) // row 2 violates
		},
	}
}

// NewForwardShiftVanishingScenario returns a scenario for the identity
// A[i] − A[i+1] = 0, which forces A[i] = A[i+1] for i in 0..n−2 (the
// constraint at row n−1 is exempt via the positive-shift cancellation).
// This exercises the "shift > 0 ⇒ negative-position cancellation" branch
// of the global compiler.
//
//   - Valid: A constant 5 throughout.
//   - Invalid: A breaks the constant-column relation at row 3.
func NewForwardShiftVanishingScenario() *VanishingScenario {
	sys := wiop.NewSystemf("fwd-shift")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 8, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	// A[i] − A[i+1] = 0
	mod.NewVanishing(
		sys.Context.Childf("eq-next"),
		wiop.Sub(col.View(), col.View().Shift(1)),
	)

	return &VanishingScenario{
		Name: "ForwardShiftConstant",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(5, 5, 5, 5, 5, 5, 5, 5))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(5, 5, 5, 6, 5, 5, 5, 5))
		},
	}
}

// NewBooleanCubeVanishingScenario returns a scenario for the identity
// col*col*col − col = 0, which is equivalent to col ∈ {−1, 0, 1}.
// DegreeFactor = 3, so the compiler must allocate a ratio > 1 bucket — the
// first scenario in the set that exercises the multi-share quotient path.
//
//   - Valid: col = [0, 1, 0, 1] (only 0/1 values).
//   - Invalid: col = [0, 1, 0, 2] (row 3 = 2: 8 − 2 = 6 ≠ 0).
func NewBooleanCubeVanishingScenario() *VanishingScenario {
	sys := wiop.NewSystemf("bool-cube")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	// col*col*col − col = 0
	cube := wiop.Mul(wiop.Mul(col.View(), col.View()), col.View())
	mod.NewVanishing(sys.Context.Childf("cube"), wiop.Sub(cube, col.View()))

	return &VanishingScenario{
		Name: "BooleanCube",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(0, 1, 0, 1))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(0, 1, 0, 2))
		},
	}
}

// NewLinearCombinationVanishingScenario returns a scenario for the constraint
// A − 2·B − 3·C = 0 across three columns of the same module.
//
//   - Valid: A = 2B + 3C row-by-row, no shifts.
//   - Invalid: row 2's A breaks the linear relation.
func NewLinearCombinationVanishingScenario() *VanishingScenario {
	sys := wiop.NewSystemf("lin-comb")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	colA := mod.NewColumn(sys.Context.Childf("a"), wiop.VisibilityOracle, r0)
	colB := mod.NewColumn(sys.Context.Childf("b"), wiop.VisibilityOracle, r0)
	colC := mod.NewColumn(sys.Context.Childf("c"), wiop.VisibilityOracle, r0)
	two := wiop.NewConstantField(field.NewFromString("2"))
	three := wiop.NewConstantField(field.NewFromString("3"))
	expr := wiop.Sub(
		wiop.Sub(colA.View(), wiop.Mul(two, colB.View())),
		wiop.Mul(three, colC.View()),
	)
	mod.NewVanishing(sys.Context.Childf("lin"), expr)

	return &VanishingScenario{
		Name: "LinearCombination",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			// A = 2B + 3C: rows (0,0,0), (5,1,1), (16,2,4), (11,4,1).
			rt.AssignColumn(colA, makeVec(0, 5, 16, 11))
			rt.AssignColumn(colB, makeVec(0, 1, 2, 4))
			rt.AssignColumn(colC, makeVec(0, 1, 4, 1))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(colA, makeVec(0, 5, 17, 11)) // row 2: A=17 instead of 16
			rt.AssignColumn(colB, makeVec(0, 1, 2, 4))
			rt.AssignColumn(colC, makeVec(0, 1, 4, 1))
		},
	}
}

// NewLargeFibonacciVanishingScenario stresses the FFT path with a size-16
// module. The constraint is identical to [NewFibonacciVanishingScenario]; the
// witness is the first 16 Fibonacci numbers.
//
//   - Valid: A = [1, 1, 2, 3, 5, …, 987].
//   - Invalid: last value perturbed by +1.
func NewLargeFibonacciVanishingScenario() *VanishingScenario {
	sys := wiop.NewSystemf("fib-16")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 16, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	mod.NewVanishing(
		sys.Context.Childf("fib"),
		wiop.Sub(
			wiop.Sub(col.View(), col.View().Shift(-1)),
			col.View().Shift(-2),
		),
	)

	honest := []uint64{1, 1, 2, 3, 5, 8, 13, 21, 34, 55, 89, 144, 233, 377, 610, 987}
	bad := append([]uint64{}, honest...)
	bad[15]++ // last value off by one

	return &VanishingScenario{
		Name: "LargeFibonacci",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(honest...))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(bad...))
		},
	}
}

// NewMultipleVanishingsSameRatioScenario registers two independent linear
// vanishings on the same module. Both share ratio = 1, so the compiler
// merges them into one quotient bucket — exercising the merging-coin
// random-linear-combination path.
//
//   - Valid: A constant 4, B equals next-row of A → constant 4.
//   - Invalid: B's row 1 breaks the second constraint.
func NewMultipleVanishingsSameRatioScenario() *VanishingScenario {
	sys := wiop.NewSystemf("same-ratio")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	colA := mod.NewColumn(sys.Context.Childf("a"), wiop.VisibilityOracle, r0)
	colB := mod.NewColumn(sys.Context.Childf("b"), wiop.VisibilityOracle, r0)
	four := wiop.NewConstantField(field.NewFromString("4"))
	// A − 4 = 0 and B − A = 0
	mod.NewVanishing(sys.Context.Childf("v1"), wiop.Sub(colA.View(), four))
	mod.NewVanishing(sys.Context.Childf("v2"), wiop.Sub(colB.View(), colA.View()))

	return &VanishingScenario{
		Name: "MultipleVanishingsSameRatio",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(colA, makeVec(4, 4, 4, 4))
			rt.AssignColumn(colB, makeVec(4, 4, 4, 4))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(colA, makeVec(4, 4, 4, 4))
			rt.AssignColumn(colB, makeVec(4, 5, 4, 4)) // row 1 breaks B − A
		},
	}
}

// NewMixedRatioVanishingsScenario registers two vanishings on the same module
// with different DegreeFactors, producing two distinct ratio buckets.
//
//   - Constraint 1: A − A<<−1 = 0 (degree factor 1, ratio 1)
//   - Constraint 2: A·A − A = 0 (degree factor 2, ratio 2)
//
// Valid: A is constant 0 (satisfies both); Invalid: row 2 set to 7
// (constant-equality holds within the run, but 49 − 7 ≠ 0).
func NewMixedRatioVanishingsScenario() *VanishingScenario {
	sys := wiop.NewSystemf("mixed-ratio")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("a"), wiop.VisibilityOracle, r0)
	// A[i] − A[i−1] = 0
	mod.NewVanishing(
		sys.Context.Childf("lin"),
		wiop.Sub(col.View(), col.View().Shift(-1)),
	)
	// A[i]·A[i] − A[i] = 0
	mod.NewVanishing(
		sys.Context.Childf("bool"),
		wiop.Sub(wiop.Mul(col.View(), col.View()), col.View()),
	)

	return &VanishingScenario{
		Name: "MixedRatioVanishings",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(0, 0, 0, 0))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			// Constant column 7 satisfies the linear recurrence but breaks
			// the boolean square identity (49 − 7 = 42 ≠ 0).
			rt.AssignColumn(col, makeVec(7, 7, 7, 7))
		},
	}
}

// NewMultiModuleVanishingScenario registers two independent vanishings, each
// on its own module. The compiler must compile each module separately and
// the verifier action for each module must be installed independently.
//
//   - Module A: boolean constraint over size 4.
//   - Module B: constant-7 constraint over size 8.
//
// Invalid case violates module A only; module B's witness stays honest.
func NewMultiModuleVanishingScenario() *VanishingScenario {
	sys := wiop.NewSystemf("multi-mod")
	r0 := sys.NewRound()
	modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionNone)
	modB := sys.NewSizedModule(sys.Context.Childf("modB"), 8, wiop.PaddingDirectionNone)
	colA := modA.NewColumn(sys.Context.Childf("colA"), wiop.VisibilityOracle, r0)
	colB := modB.NewColumn(sys.Context.Childf("colB"), wiop.VisibilityOracle, r0)
	modA.NewVanishing(
		sys.Context.Childf("a-bool"),
		wiop.Sub(wiop.Mul(colA.View(), colA.View()), colA.View()),
	)
	seven := wiop.NewConstantField(field.NewFromString("7"))
	modB.NewVanishing(
		sys.Context.Childf("b-const"),
		wiop.Sub(colB.View(), seven),
	)

	return &VanishingScenario{
		Name: "MultiModule",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(colA, makeVec(0, 1, 0, 1))
			rt.AssignColumn(colB, makeVec(7, 7, 7, 7, 7, 7, 7, 7))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(colA, makeVec(0, 2, 0, 1)) // row 1 violates boolean
			rt.AssignColumn(colB, makeVec(7, 7, 7, 7, 7, 7, 7, 7))
		},
	}
}

// NewManualCancellationVanishingScenario uses [Module.NewVanishingManual] to
// register an explicit cancellation set. The constraint A[i] − A[i−1] − 1 = 0
// (increment by one) is only enforced on rows 1..n−1 by hand-listing row 0 as
// cancelled. This exercises the manual-cancellation branch independently of
// the automatic-shift-driven cancellation used elsewhere.
//
//   - Valid: A = [42, 43, 44, 45]; row 0 is exempt (so the recurrence does
//     not need to bridge from A[n−1] back to row 0); rows 1..3 each increment
//     by one from the previous row.
//   - Invalid: A = [42, 43, 44, 46]; row 3 jumps by two.
func NewManualCancellationVanishingScenario() *VanishingScenario {
	sys := wiop.NewSystemf("manual-cxl")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	one := wiop.NewConstantField(field.NewFromString("1"))
	// A[i] − A[i−1] − 1, but only on rows 1..n−1.
	expr := wiop.Sub(wiop.Sub(col.View(), col.View().Shift(-1)), one)
	mod.NewVanishingManual(sys.Context.Childf("incr"), expr, 0)

	return &VanishingScenario{
		Name: "ManualCancellation",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(42, 43, 44, 45))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(42, 43, 44, 46)) // jumps by 2 at row 3
		},
	}
}

// NewPrecomputedSelectorVanishingScenario registers a vanishing whose witness
// is masked by a precomputed selector. The constraint is sel·(col − 9) = 0:
// whenever sel = 1, col must be 9; whenever sel = 0, col is unconstrained.
// The selector is precomputed, exercising the PrecomputedRound path.
//
//   - Valid: sel = [1,0,1,0]; col = [9, 42, 9, 17].
//   - Invalid: sel = [1,0,1,0]; col = [9, 42, 8, 17] (row 2 fails sel·diff).
func NewPrecomputedSelectorVanishingScenario() *VanishingScenario {
	sys := wiop.NewSystemf("pre-sel")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	sel := mod.NewPrecomputedColumn(
		sys.Context.Childf("sel"),
		wiop.VisibilityOracle,
		makeVec(1, 0, 1, 0),
	)
	nine := wiop.NewConstantField(field.NewFromString("9"))
	// sel·(col − 9) = 0
	expr := wiop.Mul(sel.View(), wiop.Sub(col.View(), nine))
	mod.NewVanishing(sys.Context.Childf("masked"), expr)

	return &VanishingScenario{
		Name: "PrecomputedSelector",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(9, 42, 9, 17))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(9, 42, 8, 17)) // row 2 violates
		},
	}
}

// NewCellLeafVanishingScenario uses a [Cell] inside a multi-valued vanishing.
// The expression col − cell broadcasts the cell value to every row, so col
// must be constant equal to cell. Exercises base-field cell handling in the
// quotient computation.
//
//   - Valid: col = [11, 11, 11, 11]; cell = 11.
//   - Invalid: col = [11, 11, 12, 11]; cell = 11 (row 2 violates).
func NewCellLeafVanishingScenario() *VanishingScenario {
	sys := wiop.NewSystemf("cell-leaf")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	cell := r0.NewCell(sys.Context.Childf("c"), false)
	mod.NewVanishing(sys.Context.Childf("eq"), wiop.Sub(col.View(), cell))

	eleven := field.NewFromString("11")
	return &VanishingScenario{
		Name: "CellLeaf",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(11, 11, 11, 11))
			rt.AssignCell(cell, field.ElemFromBase(eleven))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(11, 11, 12, 11))
			rt.AssignCell(cell, field.ElemFromBase(eleven))
		},
	}
}

// NewCoinScaledVanishingScenario registers a vanishing of the form
// coin · (A − A<<−1) = 0. The coin is sampled in round 1; the column is
// committed in round 0. Because the coin is an extension element, the
// expression is extension-valued and the compiler must take the extension
// branch when accumulating on the large coset.
//
//   - Valid: A constant 8 → (A − A_prev) = 0 row-by-row regardless of coin.
//   - Invalid: A is non-constant; with overwhelming probability over the coin
//     value, the LHS at some row is non-zero.
func NewCoinScaledVanishingScenario() *VanishingScenario {
	sys := wiop.NewSystemf("coin-scaled")
	r0 := sys.NewRound()
	r1 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	coin := r1.NewCoinField(sys.Context.Childf("coin"))
	expr := wiop.Mul(coin, wiop.Sub(col.View(), col.View().Shift(-1)))
	mod.NewVanishing(sys.Context.Childf("scaled"), expr)

	return &VanishingScenario{
		Name: "CoinScaled",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(8, 8, 8, 8))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(8, 9, 8, 8)) // row 1 differs from row 0
		},
	}
}

// NewThreeStepRecurrenceVanishingScenario registers a 3-step linear
// recurrence A[i] = A[i−1] + A[i−3]. Three rows (0, 1, 2) are cancelled by
// the back-shift of 3 plus the back-shift of 1. The recurrence stresses the
// CancelledPositions handling for non-trivial cancellation sets.
//
//   - Valid: A = [0, 1, 1, 1, 2, 3, 4, 6].
//   - Invalid: A = [0, 1, 1, 1, 2, 3, 4, 7] (last value off by one).
func NewThreeStepRecurrenceVanishingScenario() *VanishingScenario {
	sys := wiop.NewSystemf("step3")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 8, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	// A[i] − A[i−1] − A[i−3] = 0
	mod.NewVanishing(
		sys.Context.Childf("rec"),
		wiop.Sub(
			wiop.Sub(col.View(), col.View().Shift(-1)),
			col.View().Shift(-3),
		),
	)

	return &VanishingScenario{
		Name: "ThreeStepRecurrence",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(0, 1, 1, 1, 2, 3, 4, 6))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(0, 1, 1, 1, 2, 3, 4, 7))
		},
	}
}

// NewQuarticVanishingScenario registers a quartic identity
// col² · col² − col = 0, which forces col ∈ {0, 1} but with DegreeFactor 4.
// The compiler must allocate a ratio-4 bucket for this single vanishing.
//
//   - Valid: col = [0, 1, 0, 1].
//   - Invalid: col = [0, 1, 0, 2] (row 3 = 2 → 16 − 2 = 14 ≠ 0).
func NewQuarticVanishingScenario() *VanishingScenario {
	sys := wiop.NewSystemf("quartic")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	sq := wiop.Mul(col.View(), col.View())
	quart := wiop.Mul(sq, sq)
	mod.NewVanishing(sys.Context.Childf("q"), wiop.Sub(quart, col.View()))

	return &VanishingScenario{
		Name: "Quartic",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(0, 1, 0, 1))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(0, 1, 0, 2))
		},
	}
}

// NewLeftPadDynamicVanishingScenario combines a dynamic module with
// [wiop.PaddingDirectionLeft]. The Fibonacci-like recurrence A[i] = A[i−1] is
// enforced (constant column); the runtime size is inferred from the
// assigned column length.
//
//   - Valid: A = [9, 9, 9, 9, 9, 9, 9, 9].
//   - Invalid: A = [9, 9, 9, 9, 9, 8, 9, 9] (row 5 breaks A[5] − A[4]).
func NewLeftPadDynamicVanishingScenario() *VanishingScenario {
	sys := wiop.NewSystemf("dyn-leftpad")
	r0 := sys.NewRound()
	mod := sys.NewDynamicModule(sys.Context.Childf("mod"), wiop.PaddingDirectionLeft)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	mod.NewVanishing(
		sys.Context.Childf("eq"),
		wiop.Sub(col.View(), col.View().Shift(-1)),
	)

	return &VanishingScenario{
		Name: "LeftPadDynamic",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(9, 9, 9, 9, 9, 9, 9, 9))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(9, 9, 9, 9, 9, 8, 9, 9))
		},
	}
}

// NewCubicWithBackShiftVanishingScenario combines DegreeFactor 3 with a
// back-shift, exercising the ratio = 2 quotient path together with a
// non-empty cancellation set. The constraint asserts col[i]² · col[i−1] =
// col[i], which (for col ∈ {0,1}) reduces to "if col[i] is 1 then col[i−1]
// must also be 1" — satisfied by the all-ones column on rows 1..n−1 (row 0
// is cancelled).
//
//   - Valid: col = [1, 1, 1, 1, 1, 1, 1, 1].
//   - Invalid: col[5] = 2; 2²·1 − 2 = 2 ≠ 0.
func NewCubicWithBackShiftVanishingScenario() *VanishingScenario {
	sys := wiop.NewSystemf("cube-shift")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 8, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	// col² · col<<-1 − col = 0
	sq := wiop.Mul(col.View(), col.View())
	mod.NewVanishing(
		sys.Context.Childf("cube-shift"),
		wiop.Sub(wiop.Mul(sq, col.View().Shift(-1)), col.View()),
	)

	return &VanishingScenario{
		Name: "CubicWithBackShift",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(1, 1, 1, 1, 1, 1, 1, 1))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(1, 1, 1, 1, 1, 2, 1, 1))
		},
	}
}

// NewMixedHighRatioVanishingsScenario registers two vanishings on the same
// module with distinct DegreeFactors 3 and 4. The compiler must allocate two
// independent quotient buckets, one with ratio = 2 and one with ratio = 4.
//
//   - Valid: col = [0, 1, 0, 1, 0, 1, 0, 1] satisfies both.
//   - Invalid: col[4] = 2 violates the cubic (8 − 2 ≠ 0) and the quartic
//     (16 − 2 ≠ 0).
func NewMixedHighRatioVanishingsScenario() *VanishingScenario {
	sys := wiop.NewSystemf("mixed-hi-ratio")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 8, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	// col^3 − col (factor 3, ratio 2)
	cube := wiop.Mul(wiop.Mul(col.View(), col.View()), col.View())
	mod.NewVanishing(sys.Context.Childf("cube"), wiop.Sub(cube, col.View()))
	// col^4 − col (factor 4, ratio 4)
	sq := wiop.Mul(col.View(), col.View())
	quart := wiop.Mul(sq, sq)
	mod.NewVanishing(sys.Context.Childf("quartic"), wiop.Sub(quart, col.View()))

	return &VanishingScenario{
		Name: "MixedHighRatio",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(0, 1, 0, 1, 0, 1, 0, 1))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(0, 1, 0, 1, 2, 1, 0, 1))
		},
	}
}

// NewMultiModuleHighRatioVanishingScenario places a cubic vanishing on each
// of two modules, exercising the multi-module path AND the ratio > 1
// quotient path simultaneously.
func NewMultiModuleHighRatioVanishingScenario() *VanishingScenario {
	sys := wiop.NewSystemf("multi-mod-hi-ratio")
	r0 := sys.NewRound()
	modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionNone)
	modB := sys.NewSizedModule(sys.Context.Childf("modB"), 8, wiop.PaddingDirectionNone)
	colA := modA.NewColumn(sys.Context.Childf("colA"), wiop.VisibilityOracle, r0)
	colB := modB.NewColumn(sys.Context.Childf("colB"), wiop.VisibilityOracle, r0)
	cubeA := wiop.Mul(wiop.Mul(colA.View(), colA.View()), colA.View())
	cubeB := wiop.Mul(wiop.Mul(colB.View(), colB.View()), colB.View())
	modA.NewVanishing(sys.Context.Childf("cubeA"), wiop.Sub(cubeA, colA.View()))
	modB.NewVanishing(sys.Context.Childf("cubeB"), wiop.Sub(cubeB, colB.View()))

	return &VanishingScenario{
		Name: "MultiModuleHighRatio",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(colA, makeVec(0, 1, 0, 1))
			rt.AssignColumn(colB, makeVec(0, 1, 0, 1, 0, 1, 0, 1))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(colA, makeVec(0, 1, 0, 1))
			rt.AssignColumn(colB, makeVec(0, 1, 0, 1, 0, 1, 0, 2)) // modB row 7
		},
	}
}

// NewSizeThirtyTwoCubicVanishingScenario stresses the ratio = 2 FFT pipeline
// with a medium-sized module (n = 32). Larger n exposes any aliasing or
// indexing edge cases that pass for tiny domains.
func NewSizeThirtyTwoCubicVanishingScenario() *VanishingScenario {
	sys := wiop.NewSystemf("cube-32")
	r0 := sys.NewRound()
	const n = 32
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), n, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	cube := wiop.Mul(wiop.Mul(col.View(), col.View()), col.View())
	mod.NewVanishing(sys.Context.Childf("cube"), wiop.Sub(cube, col.View()))

	honest := make([]uint64, n)
	for i := range honest {
		honest[i] = uint64(i % 2) // alternating 0, 1 satisfies col^3 = col
	}
	bad := append([]uint64{}, honest...)
	bad[7] = 2

	return &VanishingScenario{
		Name: "SizeThirtyTwoCubic",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(honest...))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(bad...))
		},
	}
}

// NewLargeForwardShiftVanishingScenario uses a forward shift of +3 so the
// last 3 rows are cancelled. Combined with a constant predicate, this
// exercises the "shift > 0 ⇒ trailing-row cancellation" branch when more
// than one row is cancelled.
//
//   - Valid: col fully constant 4.
//   - Invalid: row 2 breaks A[2] = A[5].
func NewLargeForwardShiftVanishingScenario() *VanishingScenario {
	sys := wiop.NewSystemf("fwd-shift-3")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 8, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	// A[i] − A[i+3] = 0 → cancels last 3 rows.
	mod.NewVanishing(
		sys.Context.Childf("eq-plus3"),
		wiop.Sub(col.View(), col.View().Shift(3)),
	)

	return &VanishingScenario{
		Name: "LargeForwardShift",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(4, 4, 4, 4, 4, 4, 4, 4))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(4, 4, 5, 4, 4, 4, 4, 4))
		},
	}
}

// NewBackAndForwardShiftVanishingScenario uses BOTH a positive and a
// negative shift in the same expression. Cancellation covers the first row
// (from −1 shift) and the last row (from +1 shift).
//
//   - Constraint: A[i+1] − A[i−1] − 2·A[i] = 0 on cyclic shift (a discrete
//     analogue of f' = 0 — but here we just want a witness that satisfies it).
//   - Valid: a constant column: 5,5,5,5,5,5,5,5.
//     5 − 5 − 10 = −10 ≠ 0… that doesn't work. Use a constant 0 instead.
//   - With col ≡ 0 → 0 − 0 − 0 = 0. ✓
//   - Invalid: any row non-zero.
func NewBackAndForwardShiftVanishingScenario() *VanishingScenario {
	sys := wiop.NewSystemf("two-shift")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 8, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	two := wiop.NewConstantField(field.NewFromString("2"))
	// A[i+1] − A[i−1] − 2·A[i] = 0, cancels first and last row.
	expr := wiop.Sub(
		wiop.Sub(col.View().Shift(1), col.View().Shift(-1)),
		wiop.Mul(two, col.View()),
	)
	mod.NewVanishing(sys.Context.Childf("twoShift"), expr)

	return &VanishingScenario{
		Name: "BackAndForwardShift",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(0, 0, 0, 0, 0, 0, 0, 0))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			// Single non-zero entry inside the active range breaks the
			// recurrence at multiple rows.
			rt.AssignColumn(col, makeVec(0, 0, 0, 1, 0, 0, 0, 0))
		},
	}
}

// NewDynamicQuadraticVanishingScenario exercises a quadratic constraint on
// a dynamic module. The runtime size is inferred from the witness.
//
//   - Constraint: col² − col = 0 (boolean).
//   - Valid: col = [0, 1, 0, 1, 0, 1, 0, 1].
//   - Invalid: col[3] = 2 → 4 − 2 = 2 ≠ 0.
func NewDynamicQuadraticVanishingScenario() *VanishingScenario {
	sys := wiop.NewSystemf("dyn-quad")
	r0 := sys.NewRound()
	mod := sys.NewDynamicModule(sys.Context.Childf("mod"), wiop.PaddingDirectionRight)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	mod.NewVanishing(
		sys.Context.Childf("bool"),
		wiop.Sub(wiop.Mul(col.View(), col.View()), col.View()),
	)

	return &VanishingScenario{
		Name: "DynamicQuadratic",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(0, 1, 0, 1, 0, 1, 0, 1))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(0, 1, 0, 2, 0, 1, 0, 1))
		},
	}
}

// NewQuarticWithBackShiftVanishingScenario combines DegreeFactor 4 with a
// back-shift. The compiler must allocate a ratio-4 bucket and the shift's
// cancellation must compose correctly with the quartic numerator.
//
//   - Constraint: col² · col<<-1² − col² = 0. Quartic in (col, col<<-1).
//   - Valid: col = [0, 1, 0, 1, 0, 1, 0, 1]. col² ∈ {0, 1}; col<<-1² ∈ {0, 1};
//     so col²·col<<-1² ∈ {0, 1}, and on rows where col=0 both sides are 0,
//     where col=1, prev=0 so 1·0 − 1 = −1 — that's not zero. Use a constant
//     column instead.
//   - For a constant column k: k² · k² − k² = k⁴ − k² = k²(k² − 1). For k ∈
//     {0, 1, −1} this is 0. So col fully 1 works. ✓
//   - Invalid: col[3] = 2 → 4·1 − 4 = 0? 4·col[2]² = 4·1 = 4, then 4 − 4 = 0.
//     Hmm. Try col[3] = 3 with constant prev=1: 9·1 − 9 = 0. Still 0.
//     The form k²·k² − k² = 0 only fails for k² != 0 and k² != 1, i.e. k ∉
//     {0, 1, −1}. So col[3] = 2 → 4·1 − 4 = 0 stays zero. Hmm.
//   - Adjust: use col² · col<<-1 − col instead (factor 3 with shift, ratio 2).
func NewQuarticWithBackShiftVanishingScenario() *VanishingScenario {
	sys := wiop.NewSystemf("quartic-shift")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 8, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	// col² · col<<-1² − col² = 0. Row 0 cancelled by back-shift.
	sq := wiop.Mul(col.View(), col.View())
	prev := col.View().Shift(-1)
	prevSq := wiop.Mul(prev, prev)
	mod.NewVanishing(
		sys.Context.Childf("quartic-shift"),
		wiop.Sub(wiop.Mul(sq, prevSq), sq),
	)

	return &VanishingScenario{
		Name: "QuarticWithBackShift",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			// All-ones satisfies col²(prev² − 1) = 1·(1 − 1) = 0 at every
			// non-cancelled row.
			rt.AssignColumn(col, makeVec(1, 1, 1, 1, 1, 1, 1, 1))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			// Setting row 5 to 7 makes col²·prev² − col² = 49·1 − 49 = 0
			// (still satisfied because prev is 1). Set row 5 to 7 AND row 4
			// to 7: at row 5 we get 49·49 − 49 = 2352 ≠ 0.
			rt.AssignColumn(col, makeVec(1, 1, 1, 1, 7, 7, 1, 1))
		},
	}
}
