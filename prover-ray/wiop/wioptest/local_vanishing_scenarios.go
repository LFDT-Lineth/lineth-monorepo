package wioptest

import (
	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/linea-monorepo/prover-ray/wiop"
)

// NewLocalSingleColumnFirstRowZeroScenario pins col at row 0 to zero.
//
//   - Valid: col = [0, 9, 9, 9]; row 0 satisfies col[0] = 0.
//   - Invalid: col = [7, 9, 9, 9]; row 0 violates.
func NewLocalSingleColumnFirstRowZeroScenario() *LocalVanishingScenario {
	sys := wiop.NewSystemf("lv-row0")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	mod.NewLocalConstraint(sys.Context.Childf("lc"), col.View(), 0)

	return &LocalVanishingScenario{
		Name: "SingleColumnFirstRowZero",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(0, 9, 9, 9))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(7, 9, 9, 9))
		},
	}
}

// NewLocalSingleColumnLastRowZeroScenario pins col at row n-1 to zero.
//
//   - Valid: col = [9, 9, 9, 0]; row 3 satisfies col[3] = 0.
//   - Invalid: col = [9, 9, 9, 7]; row 3 violates.
func NewLocalSingleColumnLastRowZeroScenario() *LocalVanishingScenario {
	sys := wiop.NewSystemf("lv-rowN")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	mod.NewLocalConstraint(sys.Context.Childf("lc"), col.View(), -1)

	return &LocalVanishingScenario{
		Name: "SingleColumnLastRowZero",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(9, 9, 9, 0))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(9, 9, 9, 7))
		},
	}
}

// NewLocalShiftedColumnFirstRowZeroScenario pins col at row 1 to zero by
// reading col through a +1 shift from a constraint anchored at row 0.
//
//   - Valid: col = [9, 0, 9, 9]; col[0 + 1] = col[1] = 0.
//   - Invalid: col = [9, 7, 9, 9]; col[1] != 0.
func NewLocalShiftedColumnFirstRowZeroScenario() *LocalVanishingScenario {
	sys := wiop.NewSystemf("lv-shift")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	mod.NewLocalConstraint(sys.Context.Childf("lc"), col.View().Shift(1), 0)

	return &LocalVanishingScenario{
		Name: "ShiftedColumnFirstRowZero",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(9, 0, 9, 9))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(9, 7, 9, 9))
		},
	}
}

// NewLocalTwoColumnsEqualAtFirstRowScenario asserts a[0] = b[0] via a
// two-column subtraction pinned at row 0.
//
//   - Valid: a[0] = b[0] = 5.
//   - Invalid: a[0] = 5, b[0] = 6.
func NewLocalTwoColumnsEqualAtFirstRowScenario() *LocalVanishingScenario {
	sys := wiop.NewSystemf("lv-pair")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	a := mod.NewColumn(sys.Context.Childf("a"), wiop.VisibilityOracle, r0)
	b := mod.NewColumn(sys.Context.Childf("b"), wiop.VisibilityOracle, r0)
	mod.NewLocalConstraint(sys.Context.Childf("lc"), wiop.Sub(a.View(), b.View()), 0)

	return &LocalVanishingScenario{
		Name: "TwoColumnsEqualAtFirstRow",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(a, makeVec(5, 9, 9, 9))
			rt.AssignColumn(b, makeVec(5, 9, 9, 9))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(a, makeVec(5, 9, 9, 9))
			rt.AssignColumn(b, makeVec(6, 9, 9, 9))
		},
	}
}

// NewLocalMultipleConstraintsSameModuleScenario registers two scalar
// constraints on the same column: col[0] = 0 and col[n-1] = 0.
//
//   - Valid: col = [0, 9, 9, 0].
//   - Invalid: col = [0, 9, 9, 7]; row n-1 violates the second constraint.
func NewLocalMultipleConstraintsSameModuleScenario() *LocalVanishingScenario {
	sys := wiop.NewSystemf("lv-multi")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	mod.NewLocalConstraint(sys.Context.Childf("lc-first"), col.View(), 0)
	mod.NewLocalConstraint(sys.Context.Childf("lc-last"), col.View(), -1)

	return &LocalVanishingScenario{
		Name: "MultipleConstraintsSameModule",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(0, 9, 9, 0))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(0, 9, 9, 7))
		},
	}
}

// NewLocalSecondRowConstraintScenario pins col at row 1 to a constant via
// the position-1 anchor of [Module.NewLocalConstraint]. Exercises the
// position == 1 branch.
//
//   - Valid: col[1] = 42.
//   - Invalid: col[1] = 41.
func NewLocalSecondRowConstraintScenario() *LocalVanishingScenario {
	sys := wiop.NewSystemf("lv-row1")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	fortyTwo := wiop.NewConstantField(field.NewFromString("42"))
	mod.NewLocalConstraint(sys.Context.Childf("lc"), wiop.Sub(col.View(), fortyTwo), 1)

	return &LocalVanishingScenario{
		Name: "SecondRowConstraint",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(99, 42, 99, 99))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(99, 41, 99, 99))
		},
	}
}

// NewLocalCellEqualityScenario asserts col[0] = cellValue via a base-field
// [wiop.Cell] inside the local constraint expression. Exercises the
// cell-leaf branch of the localvanishing compiler.
//
//   - Valid: col[0] = 5, cell = 5.
//   - Invalid: col[0] = 5, cell = 7.
func NewLocalCellEqualityScenario() *LocalVanishingScenario {
	sys := wiop.NewSystemf("lv-cell")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	cell := r0.NewCell(sys.Context.Childf("c"), false)
	mod.NewLocalConstraint(sys.Context.Childf("lc"), wiop.Sub(col.View(), cell), 0)

	return &LocalVanishingScenario{
		Name: "CellEquality",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(5, 9, 9, 9))
			var five field.Element
			five.SetUint64(5)
			rt.AssignCell(cell, field.ElemFromBase(five))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(5, 9, 9, 9))
			var seven field.Element
			seven.SetUint64(7)
			rt.AssignCell(cell, field.ElemFromBase(seven))
		},
	}
}

// NewLocalCoinScaledScenario asserts coin · (col[0] − 5) = 0 with an
// extension-field [wiop.CoinField] inside the local constraint expression.
// At honest col[0] = 5 the predicate trivially holds; at col[0] != 5 it
// fails with overwhelming probability over the coin value.
//
//   - Valid: col[0] = 5.
//   - Invalid: col[0] = 7.
func NewLocalCoinScaledScenario() *LocalVanishingScenario {
	sys := wiop.NewSystemf("lv-coin")
	r0 := sys.NewRound()
	r1 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	coin := r1.NewCoinField(sys.Context.Childf("coin"))
	five := wiop.NewConstantField(field.NewFromString("5"))
	mod.NewLocalConstraint(
		sys.Context.Childf("lc"),
		wiop.Mul(coin, wiop.Sub(col.View(), five)),
		0,
	)

	return &LocalVanishingScenario{
		Name: "CoinScaled",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(5, 9, 9, 9))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(7, 9, 9, 9))
		},
	}
}

// NewLocalMultipleAnchorsSharedColumnScenario registers three independent
// scalar constraints anchored at distinct rows (0, 1, and n-1) of the same
// column. The localvanishing compiler must allocate three separate Lagrange
// indicator columns — one per anchor — and lift each constraint independently.
//
//   - Valid: col = [10, 20, 99, 30].
//   - Invalid: col[1] perturbed to 21.
func NewLocalMultipleAnchorsSharedColumnScenario() *LocalVanishingScenario {
	sys := wiop.NewSystemf("lv-anchors")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	ten := wiop.NewConstantField(field.NewFromString("10"))
	twenty := wiop.NewConstantField(field.NewFromString("20"))
	thirty := wiop.NewConstantField(field.NewFromString("30"))
	mod.NewLocalConstraint(sys.Context.Childf("lc0"), wiop.Sub(col.View(), ten), 0)
	mod.NewLocalConstraint(sys.Context.Childf("lc1"), wiop.Sub(col.View(), twenty), 1)
	mod.NewLocalConstraint(sys.Context.Childf("lcN"), wiop.Sub(col.View(), thirty), -1)

	return &LocalVanishingScenario{
		Name: "MultipleAnchorsSharedColumn",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(10, 20, 99, 30))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(10, 21, 99, 30))
		},
	}
}

// NewLocalConstantSubtractionScenario asserts col[0] − 7 = 0 using a scalar
// vector constant (collapsed to scalar by the local-vanishing lowering).
//
//   - Valid: col[0] = 7.
//   - Invalid: col[0] = 8.
func NewLocalConstantSubtractionScenario() *LocalVanishingScenario {
	sys := wiop.NewSystemf("lv-const")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	seven := wiop.NewConstantVector(mod, field.NewFromString("7"))
	mod.NewLocalConstraint(sys.Context.Childf("lc"), wiop.Sub(col.View(), seven), 0)

	return &LocalVanishingScenario{
		Name: "ConstantSubtraction",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(7, 0, 0, 0))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(8, 0, 0, 0))
		},
	}
}

// NewLocalWrapAroundShiftScenario uses a negative shift at the first row
// (position 0). col.View().Shift(-1) at row 0 wraps to row n-1, exercising
// the modulo-n normalisation of [Module.NewLocalConstraint].
//
//   - Valid: col = [99, 99, 99, 0]; col[n-1] = 0.
//   - Invalid: col = [99, 99, 99, 7]; col[n-1] != 0.
func NewLocalWrapAroundShiftScenario() *LocalVanishingScenario {
	sys := wiop.NewSystemf("lv-wrap")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	mod.NewLocalConstraint(sys.Context.Childf("lc"), col.View().Shift(-1), 0)

	return &LocalVanishingScenario{
		Name: "WrapAroundShift",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(99, 99, 99, 0))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(99, 99, 99, 7))
		},
	}
}

// NewLocalCellAndCoinScenario asserts coin · (col[0] − cellExt) = 0 with
// both an extension cell and an extension coin in the same local
// predicate. Exercises the combined cell-+-coin leaf path of the
// local-vanishing lift.
//
//   - Valid: col[0] = 7, cell = 7.
//   - Invalid: col[0] = 7, cell = 8.
func NewLocalCellAndCoinScenario() *LocalVanishingScenario {
	sys := wiop.NewSystemf("lv-cell-coin")
	r0 := sys.NewRound()
	r1 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	cell := r0.NewCell(sys.Context.Childf("c"), true) // extension cell
	coin := r1.NewCoinField(sys.Context.Childf("coin"))
	mod.NewLocalConstraint(
		sys.Context.Childf("lc"),
		wiop.Mul(coin, wiop.Sub(col.View(), cell)),
		0,
	)

	mkExt := func(v uint64) field.Gen {
		var e field.Element
		e.SetUint64(v)
		return field.ElemFromExt(field.Lift(e))
	}

	return &LocalVanishingScenario{
		Name: "CellAndCoin",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(7, 9, 9, 9))
			rt.AssignCell(cell, mkExt(7))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(7, 9, 9, 9))
			rt.AssignCell(cell, mkExt(8))
		},
	}
}

// NewLocalThreeColumnLinearScenario asserts a[0] + b[0] − c[0] = 0 — a
// three-way linear constraint at row 0. Exercises the lift on a wider
// expression tree than the two-column variants.
//
//   - Valid: a[0]=3, b[0]=5, c[0]=8.
//   - Invalid: a[0]=3, b[0]=5, c[0]=9.
func NewLocalThreeColumnLinearScenario() *LocalVanishingScenario {
	sys := wiop.NewSystemf("lv-3col")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	a := mod.NewColumn(sys.Context.Childf("a"), wiop.VisibilityOracle, r0)
	b := mod.NewColumn(sys.Context.Childf("b"), wiop.VisibilityOracle, r0)
	c := mod.NewColumn(sys.Context.Childf("c"), wiop.VisibilityOracle, r0)
	mod.NewLocalConstraint(
		sys.Context.Childf("lc"),
		wiop.Sub(wiop.Add(a.View(), b.View()), c.View()),
		0,
	)

	return &LocalVanishingScenario{
		Name: "ThreeColumnLinear",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(a, makeVec(3, 9, 9, 9))
			rt.AssignColumn(b, makeVec(5, 9, 9, 9))
			rt.AssignColumn(c, makeVec(8, 9, 9, 9))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(a, makeVec(3, 9, 9, 9))
			rt.AssignColumn(b, makeVec(5, 9, 9, 9))
			rt.AssignColumn(c, makeVec(9, 9, 9, 9))
		},
	}
}

// NewLocalMultiAnchorMultiColumnScenario combines distinct anchors with a
// multi-column predicate: a[0] = 0 AND a[-1] − b[-1] = 0. Two scalar
// vanishings on different anchors, the second referencing two columns.
//
//   - Valid: a = [0, 9, 9, 5], b = [99, 99, 99, 5] (a[0]=0; a[3]=b[3]=5).
//   - Invalid: a[3]=5, b[3]=6.
func NewLocalMultiAnchorMultiColumnScenario() *LocalVanishingScenario {
	sys := wiop.NewSystemf("lv-multi-anchor-multi-col")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	a := mod.NewColumn(sys.Context.Childf("a"), wiop.VisibilityOracle, r0)
	b := mod.NewColumn(sys.Context.Childf("b"), wiop.VisibilityOracle, r0)
	mod.NewLocalConstraint(sys.Context.Childf("first"), a.View(), 0)
	mod.NewLocalConstraint(sys.Context.Childf("last-eq"), wiop.Sub(a.View(), b.View()), -1)

	return &LocalVanishingScenario{
		Name: "MultiAnchorMultiColumn",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(a, makeVec(0, 9, 9, 5))
			rt.AssignColumn(b, makeVec(99, 99, 99, 5))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(a, makeVec(0, 9, 9, 5))
			rt.AssignColumn(b, makeVec(99, 99, 99, 6))
		},
	}
}

// NewLocalCubeAtFirstRowScenario: a cubic predicate at a fixed row.
// col[0]² · col[0] − col[0] = 0 (col[0] ∈ {-1, 0, 1}). After lifting via
// the Lagrange indicator, the global vanishing has DegreeFactor 4 — this
// fires the ratio-4 quotient path through the local-vanishing pipeline.
//
//   - Valid: col[0] = 1.
//   - Invalid: col[0] = 2 → 8 − 2 = 6 ≠ 0.
func NewLocalCubeAtFirstRowScenario() *LocalVanishingScenario {
	sys := wiop.NewSystemf("lv-cube")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	cube := wiop.Mul(wiop.Mul(col.View(), col.View()), col.View())
	mod.NewLocalConstraint(
		sys.Context.Childf("lc"),
		wiop.Sub(cube, col.View()),
		0,
	)

	return &LocalVanishingScenario{
		Name: "CubeAtFirstRow",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(1, 9, 9, 9))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(2, 9, 9, 9))
		},
	}
}

// NewLocalMultiModuleScenario registers one local constraint on each of
// two separate modules. The local-vanishing compiler must allocate
// Lagrange columns on the correct modules and the downstream global
// compiler must process each module's lifted vanishing independently.
func NewLocalMultiModuleScenario() *LocalVanishingScenario {
	sys := wiop.NewSystemf("lv-multi-mod")
	r0 := sys.NewRound()
	modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionNone)
	modB := sys.NewSizedModule(sys.Context.Childf("modB"), 4, wiop.PaddingDirectionNone)
	colA := modA.NewColumn(sys.Context.Childf("a"), wiop.VisibilityOracle, r0)
	colB := modB.NewColumn(sys.Context.Childf("b"), wiop.VisibilityOracle, r0)
	modA.NewLocalConstraint(sys.Context.Childf("a-zero"), colA.View(), 0)
	modB.NewLocalConstraint(sys.Context.Childf("b-zero"), colB.View(), -1)

	return &LocalVanishingScenario{
		Name: "MultiModule",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(colA, makeVec(0, 9, 9, 9))
			rt.AssignColumn(colB, makeVec(9, 9, 9, 0))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(colA, makeVec(0, 9, 9, 9))
			rt.AssignColumn(colB, makeVec(9, 9, 9, 7)) // last row of modB violates
		},
	}
}

// NewLocalProductIsZeroScenario asserts (a · b)[0] = 0 — i.e. at least one of
// a[0] and b[0] is zero. The product is a quadratic expression; the
// local-vanishing compiler reduces this to a scalar predicate (no quadratic
// blow-up of the lifted vanishing's degree).
//
//   - Valid: a[0] = 0, b[0] = 5; product 0.
//   - Invalid: a[0] = 3, b[0] = 5; product 15.
func NewLocalProductIsZeroScenario() *LocalVanishingScenario {
	sys := wiop.NewSystemf("lv-prod")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	a := mod.NewColumn(sys.Context.Childf("a"), wiop.VisibilityOracle, r0)
	b := mod.NewColumn(sys.Context.Childf("b"), wiop.VisibilityOracle, r0)
	mod.NewLocalConstraint(sys.Context.Childf("lc"), wiop.Mul(a.View(), b.View()), 0)

	return &LocalVanishingScenario{
		Name: "ProductIsZero",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(a, makeVec(0, 9, 9, 9))
			rt.AssignColumn(b, makeVec(5, 9, 9, 9))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(a, makeVec(3, 9, 9, 9))
			rt.AssignColumn(b, makeVec(5, 9, 9, 9))
		},
	}
}

// NewLocalDynamicFirstRowZeroScenario pins col at row 0 to zero on a
// dynamic-size module. The module declares no size; its domain size is fixed by
// the column assignment at runtime. This exercises the localvanishing →
// global pipeline against a module whose size is unknown at compile time —
// only possible because the lift uses a [wiop.LagrangeSelector] (computed from
// the runtime size) rather than a static precomputed column.
//
//   - Valid: col = [0, 9, …]; row 0 satisfies col[0] = 0.
//   - Invalid: col = [7, 9, …]; row 0 violates.
func NewLocalDynamicFirstRowZeroScenario() *LocalVanishingScenario {
	sys := wiop.NewSystemf("lv-dyn-row0")
	r0 := sys.NewRound()
	mod := sys.NewDynamicModule(sys.Context.Childf("dynmod"), wiop.PaddingDirectionRight)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	mod.NewLocalConstraint(sys.Context.Childf("lc"), col.View(), 0)

	return &LocalVanishingScenario{
		Name: "DynamicFirstRowZero",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(0, 9, 9, 9, 9, 9, 9, 9)) // size 8, row 0 = 0
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(7, 9, 9, 9, 9, 9, 9, 9))
		},
	}
}

// NewLocalDynamicShiftedScenario pins col at row 1 to zero (read via a +1 shift
// from a row-0 anchor) on a dynamic-size module.
//
//   - Valid: col = [9, 0, …]; col[0 + 1] = 0.
//   - Invalid: col = [9, 7, …]; col[1] != 0.
func NewLocalDynamicShiftedScenario() *LocalVanishingScenario {
	sys := wiop.NewSystemf("lv-dyn-shift")
	r0 := sys.NewRound()
	mod := sys.NewDynamicModule(sys.Context.Childf("dynmod"), wiop.PaddingDirectionRight)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	mod.NewLocalConstraint(sys.Context.Childf("lc"), col.View().Shift(1), 0)

	return &LocalVanishingScenario{
		Name: "DynamicShifted",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(9, 0, 9, 9, 9, 9, 9, 9))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(9, 7, 9, 9, 9, 9, 9, 9))
		},
	}
}

// NewLocalDynamicProductIsZeroScenario asserts (a · b)[0] = 0 on a dynamic-size
// module. The quadratic predicate lifts to a degree-(2·(n-1)+1) vanishing whose
// quotient ratio exceeds 1, so it exercises the selector's coset evaluation on
// the *large* coset (N = n·ratio) for a module sized only at runtime.
//
//   - Valid: a[0] = 0, b[0] = 5; product 0.
//   - Invalid: a[0] = 3, b[0] = 5; product 15.
func NewLocalDynamicProductIsZeroScenario() *LocalVanishingScenario {
	sys := wiop.NewSystemf("lv-dyn-prod")
	r0 := sys.NewRound()
	mod := sys.NewDynamicModule(sys.Context.Childf("dynmod"), wiop.PaddingDirectionRight)
	a := mod.NewColumn(sys.Context.Childf("a"), wiop.VisibilityOracle, r0)
	b := mod.NewColumn(sys.Context.Childf("b"), wiop.VisibilityOracle, r0)
	mod.NewLocalConstraint(sys.Context.Childf("lc"), wiop.Mul(a.View(), b.View()), 0)

	return &LocalVanishingScenario{
		Name: "DynamicProductIsZero",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(a, makeVec(0, 9, 9, 9, 9, 9, 9, 9))
			rt.AssignColumn(b, makeVec(5, 9, 9, 9, 9, 9, 9, 9))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(a, makeVec(3, 9, 9, 9, 9, 9, 9, 9))
			rt.AssignColumn(b, makeVec(5, 9, 9, 9, 9, 9, 9, 9))
		},
	}
}
