package localvanishing_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/linea-monorepo/prover-ray/wiop"
	"github.com/consensys/linea-monorepo/prover-ray/wiop/compilers/global"
	"github.com/consensys/linea-monorepo/prover-ray/wiop/compilers/localvanishing"
	"github.com/consensys/linea-monorepo/prover-ray/wiop/wioptest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// proveVerify runs the explicit prover/verifier split on an already-compiled
// system: it assigns the witness via assign, produces a strict public-only
// Proof, and verifies it — returning the first failing check, or nil.
func proveVerify(sys *wiop.System, assign func(rt *wiop.Runtime)) error {
	proof := sys.Prove(assign)
	return sys.Verify(proof)
}

func makeVec(vals ...uint64) *wiop.ConcreteVector {
	elems := make([]field.Element, len(vals))
	for i, v := range vals {
		elems[i].SetUint64(v)
	}
	return &wiop.ConcreteVector{Plain: field.VecFromBase(elems)}
}

// scenario bundles a freshly built System with a pair of column assignments,
// one honest (constraint satisfied) and one invalid (constraint violated).
type scenario struct {
	name  string
	build func() (*wiop.System, func(rt *wiop.Runtime), func(rt *wiop.Runtime))
}

func scenarios() []scenario {
	return []scenario{
		{
			name: "SingleColumn_FirstPositionZero",
			build: func() (*wiop.System, func(*wiop.Runtime), func(*wiop.Runtime)) {
				sys := wiop.NewSystemf("lv-p0")
				r0 := sys.NewRound()
				mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
				col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
				mod.NewLocalConstraint(sys.Context.Childf("lc"), col.View(), 0)
				honest := func(rt *wiop.Runtime) { rt.AssignColumn(col, makeVec(0, 9, 9, 9)) }
				invalid := func(rt *wiop.Runtime) { rt.AssignColumn(col, makeVec(7, 9, 9, 9)) }
				return sys, honest, invalid
			},
		},
		{
			name: "SingleColumn_LastPositionZero",
			build: func() (*wiop.System, func(*wiop.Runtime), func(*wiop.Runtime)) {
				sys := wiop.NewSystemf("lv-pNeg1")
				r0 := sys.NewRound()
				mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
				col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
				mod.NewLocalConstraint(sys.Context.Childf("lc"), col.View(), -1)
				honest := func(rt *wiop.Runtime) { rt.AssignColumn(col, makeVec(9, 9, 9, 0)) }
				invalid := func(rt *wiop.Runtime) { rt.AssignColumn(col, makeVec(9, 9, 9, 7)) }
				return sys, honest, invalid
			},
		},
		{
			name: "ShiftedColumn_SecondPositionZero",
			build: func() (*wiop.System, func(*wiop.Runtime), func(*wiop.Runtime)) {
				sys := wiop.NewSystemf("lv-shift")
				r0 := sys.NewRound()
				mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
				col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
				// col[0+1] = col[1] must be zero (constraint pinned at position 0,
				// reading col shifted by +1).
				mod.NewLocalConstraint(sys.Context.Childf("lc"), col.View().Shift(1), 0)
				honest := func(rt *wiop.Runtime) { rt.AssignColumn(col, makeVec(9, 0, 9, 9)) }
				invalid := func(rt *wiop.Runtime) { rt.AssignColumn(col, makeVec(9, 7, 9, 9)) }
				return sys, honest, invalid
			},
		},
		{
			name: "TwoColumns_EqualAtFirstPosition",
			build: func() (*wiop.System, func(*wiop.Runtime), func(*wiop.Runtime)) {
				sys := wiop.NewSystemf("lv-pair")
				r0 := sys.NewRound()
				mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
				a := mod.NewColumn(sys.Context.Childf("a"), wiop.VisibilityOracle, r0)
				b := mod.NewColumn(sys.Context.Childf("b"), wiop.VisibilityOracle, r0)
				mod.NewLocalConstraint(
					sys.Context.Childf("lc"),
					wiop.Sub(a.View(), b.View()),
					0,
				)
				honest := func(rt *wiop.Runtime) {
					rt.AssignColumn(a, makeVec(5, 9, 9, 9))
					rt.AssignColumn(b, makeVec(5, 9, 9, 9))
				}
				invalid := func(rt *wiop.Runtime) {
					rt.AssignColumn(a, makeVec(5, 9, 9, 9))
					rt.AssignColumn(b, makeVec(6, 9, 9, 9))
				}
				return sys, honest, invalid
			},
		},
		{
			name: "MultipleConstraints_SameModule",
			build: func() (*wiop.System, func(*wiop.Runtime), func(*wiop.Runtime)) {
				sys := wiop.NewSystemf("lv-multi")
				r0 := sys.NewRound()
				mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
				col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
				mod.NewLocalConstraint(sys.Context.Childf("lc-first"), col.View(), 0)
				mod.NewLocalConstraint(sys.Context.Childf("lc-last"), col.View(), -1)
				honest := func(rt *wiop.Runtime) { rt.AssignColumn(col, makeVec(0, 9, 9, 0)) }
				invalid := func(rt *wiop.Runtime) { rt.AssignColumn(col, makeVec(0, 9, 9, 7)) }
				return sys, honest, invalid
			},
		},
	}
}

func TestCompile_Completeness(t *testing.T) {
	for _, sc := range scenarios() {
		t.Run(sc.name, func(t *testing.T) {
			sys, honest, _ := sc.build()
			localvanishing.Compile(sys)
			global.Compile(sys)
			require.NoError(t, proveVerify(sys, honest),
				"compiled verifier must accept an honest witness")
		})
	}
}

func TestCompile_Soundness(t *testing.T) {
	for _, sc := range scenarios() {
		t.Run(sc.name, func(t *testing.T) {
			sys, _, invalid := sc.build()
			localvanishing.Compile(sys)
			global.Compile(sys)
			assert.Error(t, proveVerify(sys, invalid),
				"compiled verifier must reject an invalid witness")
		})
	}
}

// TestCompile_WioptestCompleteness exercises [wioptest.LocalVanishingScenarios]
// — a shared corpus of scalar-vanishing fixtures consumed by every test in
// this package's pipeline. Each scenario is independent; the loop builds a
// fresh System for each case via the factory functions.
func TestCompile_WioptestCompleteness(t *testing.T) {
	for _, build := range wioptest.LocalVanishingScenarios() {
		sc := build()
		t.Run(sc.Name, func(t *testing.T) {
			localvanishing.Compile(sc.Sys)
			global.Compile(sc.Sys)
			require.NoError(t, proveVerify(sc.Sys, sc.AssignHonest),
				"compiled verifier must accept an honest witness")
		})
	}
}

// TestCompile_WioptestSoundness checks every wioptest scalar-vanishing
// fixture: an invalid witness must be rejected by the compiled verifier.
func TestCompile_WioptestSoundness(t *testing.T) {
	for _, build := range wioptest.LocalVanishingScenarios() {
		sc := build()
		t.Run(sc.Name, func(t *testing.T) {
			localvanishing.Compile(sc.Sys)
			global.Compile(sc.Sys)
			assert.Error(t, proveVerify(sc.Sys, sc.AssignInvalid),
				"compiled verifier must reject an invalid witness")
		})
	}
}

func TestCompile_MarksScalarVanishingAsReduced(t *testing.T) {
	sys := wiop.NewSystemf("lv-reduce")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	v := mod.NewLocalConstraint(sys.Context.Childf("lc"), col.View(), 0)

	assert.False(t, v.IsReduced(), "scalar vanishing should start un-reduced")
	localvanishing.Compile(sys)
	assert.True(t, v.IsReduced(),
		"after Compile, the scalar vanishing must be marked reduced")
}

func TestCompile_EmitsMultiValuedReplacement(t *testing.T) {
	sys := wiop.NewSystemf("lv-emit")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	mod.NewLocalConstraint(sys.Context.Childf("lc"), col.View(), 0)

	before := len(mod.Vanishings)
	localvanishing.Compile(sys)
	require.Len(t, mod.Vanishings, before+1,
		"Compile must register one lifted multi-valued vanishing per scalar one")
	lifted := mod.Vanishings[before]
	assert.True(t, lifted.Expression.IsMultiValued(),
		"the lifted vanishing must be multi-valued so the global compiler can pick it up")
	assert.False(t, lifted.IsReduced(),
		"the lifted vanishing must be left unreduced for the global compiler")
}

func TestCompile_SkipsMultiValuedVanishings(t *testing.T) {
	sys := wiop.NewSystemf("lv-mv")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	// Vector-valued vanishing; the global compiler — not this one — handles it.
	v := mod.NewVanishing(sys.Context.Childf("global-v"), col.View())

	before := len(mod.Vanishings)
	localvanishing.Compile(sys)
	assert.Len(t, mod.Vanishings, before,
		"Compile must not register new vanishings when only multi-valued ones exist")
	assert.False(t, v.IsReduced(),
		"Compile must leave multi-valued vanishings unreduced")
}

func TestCompile_NoWork_IsNoOp(t *testing.T) {
	sys := wiop.NewSystemf("lv-empty")
	sys.NewRound()
	before := len(sys.Rounds)
	localvanishing.Compile(sys)
	assert.Len(t, sys.Rounds, before,
		"Compile must not touch sys.Rounds when there is nothing to reduce")
}

func TestCompile_IsIdempotent(t *testing.T) {
	sys := wiop.NewSystemf("lv-idemp")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	mod.NewLocalConstraint(sys.Context.Childf("lc"), col.View(), 0)

	localvanishing.Compile(sys)
	afterFirst := len(mod.Vanishings)
	localvanishing.Compile(sys)
	assert.Len(t, mod.Vanishings, afterFirst,
		"second Compile must not register more vanishings; the scalar one is "+
			"already reduced and the lifted one is multi-valued")
}

// findSelector returns the first LagrangeSelector leaf in expr, or nil.
func findSelector(expr wiop.Expression) *wiop.LagrangeSelector {
	switch t := expr.(type) {
	case *wiop.LagrangeSelector:
		return t
	case *wiop.ArithmeticOperation:
		for _, op := range t.Operands {
			if s := findSelector(op); s != nil {
				return s
			}
		}
	}
	return nil
}

func TestCompile_SharesSelectorsByAnchor(t *testing.T) {
	sys := wiop.NewSystemf("lv-share")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	a := mod.NewColumn(sys.Context.Childf("a"), wiop.VisibilityOracle, r0)
	b := mod.NewColumn(sys.Context.Childf("b"), wiop.VisibilityOracle, r0)
	// Two distinct scalar vanishings, both anchored at position 0.
	mod.NewLocalConstraint(sys.Context.Childf("lc-a"), a.View(), 0)
	mod.NewLocalConstraint(sys.Context.Childf("lc-b"), b.View(), 0)

	colsBefore := len(mod.Columns)
	localvanishing.Compile(sys)

	// Selectors are not committed columns, so the lift creates none.
	assert.Len(t, mod.Columns, colsBefore,
		"LagrangeSelectors are not committed, so no column should be created")

	// Both lifted vanishings share anchor 0, so the per-(module, anchor) cache
	// must hand them the same selector instance.
	var selectors []*wiop.LagrangeSelector
	for _, v := range mod.Vanishings {
		if !v.IsReduced() && v.Expression.IsMultiValued() {
			if s := findSelector(v.Expression); s != nil {
				selectors = append(selectors, s)
			}
		}
	}
	require.Len(t, selectors, 2, "expected one lifted vanishing per local constraint")
	assert.Same(t, selectors[0], selectors[1],
		"both vanishings share anchor 0, so they must reuse one selector")
}

// TestCompile_PanicsOnOutOfRangePosition ensures an out-of-range anchor is
// rejected with a localised panic instead of an opaque "index out of range"
// crash deep inside the runtime. The expression hand-constructs a
// *ColumnPosition with a negative Position via Column.At, bypassing the
// normalisation that NewLocalConstraint would otherwise apply; the negative
// anchor then flows into NewLagrangeSelector, which validates it.
func TestCompile_PanicsOnOutOfRangePosition(t *testing.T) {
	sys := wiop.NewSystemf("lv-oob")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)

	// Hand-built ColumnPosition with Position = -1; Column.At does no
	// validation, so this slips past the lowerToRow normalisation that
	// NewLocalConstraint normally applies.
	badExpr := col.At(-1)
	mod.NewLocalConstraint(sys.Context.Childf("lc"), badExpr, 0)

	defer func() {
		r := recover()
		require.NotNil(t, r, "out-of-range anchor must trigger a panic")
		msg, _ := r.(string)
		assert.Contains(t, msg, "position must be non-negative")
	}()
	localvanishing.Compile(sys)
}

// elementFromUint64 converts a literal uint64 into a koalabear field.Element.
func elementFromUint64(v uint64) field.Element {
	var e field.Element
	e.SetUint64(v)
	return e
}

// extOf lifts a base-field uint64 into an extension-field [field.Gen] (with
// the isBase tag set to false so downstream arithmetic stays in the
// extension path).
func extOf(v uint64) field.Gen {
	ext := field.Lift(elementFromUint64(v))
	return field.ElemFromExt(ext)
}

// TestCompile_BaseCellLeaf_RoundTrip exercises base-cell support: a local
// constraint that asserts col[0] − c == 0 must accept matching values and
// reject mismatched ones.
func TestCompile_BaseCellLeaf_RoundTrip(t *testing.T) {
	build := func() (*wiop.System, *wiop.Column, *wiop.Cell) {
		sys := wiop.NewSystemf("lv-basecell")
		r0 := sys.NewRound()
		mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
		col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
		cell := r0.NewCell(sys.Context.Childf("c"), false)
		mod.NewLocalConstraint(
			sys.Context.Childf("lc"),
			wiop.Sub(col.View(), cell),
			0,
		)
		return sys, col, cell
	}

	t.Run("honest", func(t *testing.T) {
		sys, col, cell := build()
		localvanishing.Compile(sys)
		global.Compile(sys)
		require.NoError(t, proveVerify(sys, func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(5, 9, 9, 9))
			rt.AssignCell(cell, field.ElemFromBase(elementFromUint64(5)))
		}))
	})

	t.Run("invalid", func(t *testing.T) {
		sys, col, cell := build()
		localvanishing.Compile(sys)
		global.Compile(sys)
		assert.Error(t, proveVerify(sys, func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(5, 9, 9, 9))
			rt.AssignCell(cell, field.ElemFromBase(elementFromUint64(7)))
		}))
	})
}

// TestCompile_ExtensionCellLeaf_RoundTrip exercises the Gen-widened
// evalExprOnCoset: the cell is declared extension and assigned via
// ElemFromExt, forcing pTimesC into the extension branch of the prover-side
// multiplication chain.
func TestCompile_ExtensionCellLeaf_RoundTrip(t *testing.T) {
	build := func() (*wiop.System, *wiop.Column, *wiop.Cell) {
		sys := wiop.NewSystemf("lv-extcell")
		r0 := sys.NewRound()
		mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
		col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
		cell := r0.NewCell(sys.Context.Childf("c"), true) // extension cell
		mod.NewLocalConstraint(
			sys.Context.Childf("lc"),
			wiop.Sub(col.View(), cell),
			0,
		)
		return sys, col, cell
	}

	t.Run("honest", func(t *testing.T) {
		sys, col, cell := build()
		localvanishing.Compile(sys)
		global.Compile(sys)
		require.NoError(t, proveVerify(sys, func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(5, 9, 9, 9))
			rt.AssignCell(cell, extOf(5))
		}))
	})

	t.Run("invalid", func(t *testing.T) {
		sys, col, cell := build()
		localvanishing.Compile(sys)
		global.Compile(sys)
		assert.Error(t, proveVerify(sys, func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(5, 9, 9, 9))
			rt.AssignCell(cell, extOf(7))
		}))
	})
}

// TestCompile_CoinLeaf_RoundTrip exercises CoinField (always extension)
// support: the constraint coin · (col[0] − 5) = 0 is satisfied when col[0]
// equals 5 (regardless of the coin) and rejected otherwise (almost surely,
// since the coin is a random non-zero extension element).
func TestCompile_CoinLeaf_RoundTrip(t *testing.T) {
	build := func() (*wiop.System, *wiop.Column) {
		sys := wiop.NewSystemf("lv-coin")
		r0 := sys.NewRound()
		r1 := sys.NewRound()
		mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
		col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
		coin := r1.NewCoinField(sys.Context.Childf("coin"))
		five := wiop.NewConstantField(elementFromUint64(5))
		mod.NewLocalConstraint(
			sys.Context.Childf("lc"),
			wiop.Mul(coin, wiop.Sub(col.View(), five)),
			0,
		)
		return sys, col
	}

	t.Run("honest", func(t *testing.T) {
		sys, col := build()
		localvanishing.Compile(sys)
		global.Compile(sys)
		require.NoError(t, proveVerify(sys, func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(5, 9, 9, 9))
		}))
	})

	t.Run("invalid", func(t *testing.T) {
		sys, col := build()
		localvanishing.Compile(sys)
		global.Compile(sys)
		assert.Error(t, proveVerify(sys, func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(7, 9, 9, 9))
		}))
	})
}

// TestCompile_ExtensionCellAndCoin_RoundTrip is the headline test for the
// Gen widening: the constraint coin · (col[0] − cExt) = 0 simultaneously
// exercises an extension cell and an extension coin inside a single lifted
// vanishing expression.
func TestCompile_ExtensionCellAndCoin_RoundTrip(t *testing.T) {
	build := func() (*wiop.System, *wiop.Column, *wiop.Cell) {
		sys := wiop.NewSystemf("lv-extcell-coin")
		r0 := sys.NewRound()
		r1 := sys.NewRound()
		mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
		col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
		cell := r0.NewCell(sys.Context.Childf("cExt"), true)
		coin := r1.NewCoinField(sys.Context.Childf("coin"))
		mod.NewLocalConstraint(
			sys.Context.Childf("lc"),
			wiop.Mul(coin, wiop.Sub(col.View(), cell)),
			0,
		)
		return sys, col, cell
	}

	t.Run("honest", func(t *testing.T) {
		sys, col, cell := build()
		localvanishing.Compile(sys)
		global.Compile(sys)
		require.NoError(t, proveVerify(sys, func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(5, 9, 9, 9))
			rt.AssignCell(cell, extOf(5))
		}))
	})

	t.Run("invalid", func(t *testing.T) {
		sys, col, cell := build()
		localvanishing.Compile(sys)
		global.Compile(sys)
		assert.Error(t, proveVerify(sys, func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(5, 9, 9, 9))
			rt.AssignCell(cell, extOf(7))
		}))
	})
}
