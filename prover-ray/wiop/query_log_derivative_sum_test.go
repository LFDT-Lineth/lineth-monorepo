package wiop_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/linea-monorepo/prover-ray/wiop"
	"github.com/consensys/linea-monorepo/prover-ray/wiop/wioptest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- Soundness ----

func TestLogDerivativeSum_Soundness_Completeness(t *testing.T) {
	sc := wioptest.NewLogDerivativeSumScenario()
	rt := wiop.NewRuntime(sc.Sys)
	sc.RunHonest(&rt)
	require.NoError(t, sc.Query.Check(rt), "honest witness must pass Check")
}

func TestLogDerivativeSum_Soundness_InvalidWitness(t *testing.T) {
	sc := wioptest.NewLogDerivativeSumScenario()
	rt := wiop.NewRuntime(sc.Sys)
	sc.RunInvalid(&rt)
	assert.Error(t, sc.Query.Check(rt), "invalid witness must be rejected by Check")
}

func TestLogDerivativeSum_Sum(t *testing.T) {
	// 4-row column of all-2; denominator = constant 1; sum = 4*2 = 8
	sys := wiop.NewSystemf("rrSys")
	r0 := sys.NewRound()
	r1 := sys.NewRound() // result cell goes here
	_ = r1
	mod := sys.NewSizedModule(sys.Context.Childf("rrMod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("rrCol"), wiop.VisibilityOracle, r0)

	one := wiop.NewConstantVector(mod, field.NewFromString("1"))
	frac := wiop.Fraction{Numerator: col.View(), Denominator: one}
	rr := sys.NewLogDerivativeSum(sys.Context.Childf("rrQ"), []wiop.Fraction{frac})
	require.NotNil(t, rr)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(col, baseVec(4, 2))
	rt.AdvanceRound()

	assert.False(t, rr.IsAlreadyAssigned(rt))
	rr.SelfAssign(rt)
	assert.True(t, rr.IsAlreadyAssigned(rt))

	require.NoError(t, rr.Check(rt))
	assert.Equal(t, r1, rr.Round())
}

func TestLogDerivativeSum_Check_Mismatch(t *testing.T) {
	sys := wiop.NewSystemf("rrMisSys")
	r0 := sys.NewRound()
	sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("rrMisMod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("rrMisCol"), wiop.VisibilityOracle, r0)

	one := wiop.NewConstantVector(mod, field.NewFromString("1"))
	frac := wiop.Fraction{Numerator: col.View(), Denominator: one}
	rr := sys.NewLogDerivativeSum(sys.Context.Childf("rrMisQ"), []wiop.Fraction{frac})

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(col, baseVec(4, 1))
	rt.AdvanceRound()
	// assign wrong value
	rt.AssignCell(rr.Result, field.ElemFromBase(field.NewFromString("99")))

	err := rr.Check(rt)
	assert.Error(t, err)
}

func TestNewLogDerivativeSum_NilCtxPanic(t *testing.T) {
	sys := wiop.NewSystemf("s")
	sys.NewRound()
	sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("m"), 4, wiop.PaddingDirectionNone)
	r := sys.Rounds[0]
	col := mod.NewColumn(sys.Context.Childf("c"), wiop.VisibilityOracle, r)
	one := wiop.NewConstantVector(mod, field.NewFromString("1"))
	frac := wiop.Fraction{Numerator: col.View(), Denominator: one}
	assert.Panics(t, func() { sys.NewLogDerivativeSum(nil, []wiop.Fraction{frac}) })
}

func TestNewLogDerivativeSum_EmptyFractionsPanic(t *testing.T) {
	sys := wiop.NewSystemf("s")
	sys.NewRound()
	assert.Panics(t, func() {
		sys.NewLogDerivativeSum(sys.Context.Childf("q"), nil)
	})
}

func TestNewLogDerivativeSum_NilNumeratorPanic(t *testing.T) {
	sys := wiop.NewSystemf("s")
	r0 := sys.NewRound()
	sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("m"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("c"), wiop.VisibilityOracle, r0)
	frac := wiop.Fraction{Numerator: nil, Denominator: col.View()}
	assert.Panics(t, func() {
		sys.NewLogDerivativeSum(sys.Context.Childf("q"), []wiop.Fraction{frac})
	})
}

func TestNewLogDerivativeSum_BothScalarPanic(t *testing.T) {
	sys := wiop.NewSystemf("s")
	sys.NewRound()
	sys.NewRound()
	k := wiop.NewConstantField(field.NewFromString("1"))
	frac := wiop.Fraction{Numerator: k, Denominator: k}
	assert.Panics(t, func() {
		sys.NewLogDerivativeSum(sys.Context.Childf("q"), []wiop.Fraction{frac})
	})
}

func TestNewLogDerivativeSum_NoNextRoundPanic(t *testing.T) {
	// Only one round — no next round for result cell
	sys := wiop.NewSystemf("s")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("m"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("c"), wiop.VisibilityOracle, r0)
	one := wiop.NewConstantVector(mod, field.NewFromString("1"))
	frac := wiop.Fraction{Numerator: col.View(), Denominator: one}
	assert.Panics(t, func() {
		sys.NewLogDerivativeSum(sys.Context.Childf("q"), []wiop.Fraction{frac})
	})
}

func TestLogDerivativeSum_ScalarNumVecDen(t *testing.T) {
	// num = scalar 1, den = vector all-2; sum = 4*(1/2)
	sys := wiop.NewSystemf("rrScalNum")
	r0 := sys.NewRound()
	sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("m"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("c"), wiop.VisibilityOracle, r0)

	one := wiop.NewConstantField(field.NewFromString("1"))
	frac := wiop.Fraction{Numerator: one, Denominator: col.View()}
	rr := sys.NewLogDerivativeSum(sys.Context.Childf("rr"), []wiop.Fraction{frac})

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(col, baseVec(4, 2))
	rt.AdvanceRound()
	rr.SelfAssign(rt)
	require.NoError(t, rr.Check(rt))
}

func TestLogDerivativeSum_VecNumScalarDen(t *testing.T) {
	// num = vector all-3, den = scalar 1; sum = 4*3 = 12
	sys := wiop.NewSystemf("rrScalDen")
	r0 := sys.NewRound()
	sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("m"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("c"), wiop.VisibilityOracle, r0)

	one := wiop.NewConstantField(field.NewFromString("1"))
	frac := wiop.Fraction{Numerator: col.View(), Denominator: one}
	rr := sys.NewLogDerivativeSum(sys.Context.Childf("rr"), []wiop.Fraction{frac})

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(col, baseVec(4, 3))
	rt.AdvanceRound()
	rr.SelfAssign(rt)
	require.NoError(t, rr.Check(rt))
}

func TestNewLogDerivativeSum_NilDenominatorPanic(t *testing.T) {
	sys := wiop.NewSystemf("s")
	r0 := sys.NewRound()
	sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("m"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("c"), wiop.VisibilityOracle, r0)
	frac := wiop.Fraction{Numerator: col.View(), Denominator: nil}
	assert.Panics(t, func() {
		sys.NewLogDerivativeSum(sys.Context.Childf("q"), []wiop.Fraction{frac})
	})
}

// TestNewLogDerivativeSum_NoRoundBearingExprPanic covers the guard that fires
// when every expression in every fraction is a constant vector (has a module,
// so neither expression is scalar) but none carries a round-bearing
// column/cell/coin. maxFracRound stays nil and the constructor panics.
func TestNewLogDerivativeSum_NoRoundBearingExprPanic(t *testing.T) {
	sys := wiop.NewSystemf("rrNoRound")
	sys.NewRound()
	sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("m"), 4, wiop.PaddingDirectionNone)
	num := wiop.NewConstantVector(mod, field.NewFromString("2"))
	den := wiop.NewConstantVector(mod, field.NewFromString("1"))
	frac := wiop.Fraction{Numerator: num, Denominator: den}
	assert.Panics(t, func() {
		sys.NewLogDerivativeSum(sys.Context.Childf("q"), []wiop.Fraction{frac})
	})
}

func TestNewLogDerivativeSum_DifferentModulePanic(t *testing.T) {
	sys := wiop.NewSystemf("rrDiffMod")
	r0 := sys.NewRound()
	sys.NewRound()
	mod1 := sys.NewSizedModule(sys.Context.Childf("m1"), 4, wiop.PaddingDirectionNone)
	mod2 := sys.NewSizedModule(sys.Context.Childf("m2"), 4, wiop.PaddingDirectionNone)
	c1 := mod1.NewColumn(sys.Context.Childf("c1"), wiop.VisibilityOracle, r0)
	c2 := mod2.NewColumn(sys.Context.Childf("c2"), wiop.VisibilityOracle, r0)
	frac := wiop.Fraction{Numerator: c1.View(), Denominator: c2.View()}
	assert.Panics(t, func() {
		sys.NewLogDerivativeSum(sys.Context.Childf("q"), []wiop.Fraction{frac})
	})
}
