package wioptest

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
