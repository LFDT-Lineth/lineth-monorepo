package wiop_test

import (
	"bytes"
	"testing"

	"github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/fiatshamir"
	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/linea-monorepo/prover-ray/wiop"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestSystem is a helper that builds a minimal two-round system with one
// sized module. It returns the system, both rounds, and the module.
func newTestSystem(t *testing.T) (*wiop.System, *wiop.Round, *wiop.Round, *wiop.Module) {
	t.Helper()
	sys := wiop.NewSystemf("test")
	r0 := sys.NewRound()
	r1 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	return sys, r0, r1, mod
}

// baseVec builds a PaddingDirectionNone ConcreteVector of length n where each
// element equals the provided uint64 value.
func baseVec(n int, val uint64) *wiop.ConcreteVector {
	elems := make([]field.Element, n)
	v := field.NewFromString(string(rune('0' + val)))
	_ = v // zero for val==0 is the zero element
	var e field.Element
	e.SetUint64(val)
	for i := range elems {
		elems[i] = e
	}
	return &wiop.ConcreteVector{Plain: field.VecFromBase(elems)}
}

// ---- Column/Module methods ----

func TestColumn_Round(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	assert.Equal(t, r0, col.Round())
}

func TestColumn_Degree(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	// module is sized to 4 → degree == 3
	assert.Equal(t, 3, col.Degree())
}

func TestColumn_Degree_UnsizedPanic(t *testing.T) {
	sys, r0, _, _ := newTestSystem(t)
	unsized := sys.NewModule(sys.Context.Childf("unsized"), wiop.PaddingDirectionNone)
	col := unsized.NewColumn(sys.Context.Childf("col2"), wiop.VisibilityOracle, r0)
	assert.Panics(t, func() { col.Degree() })
}

func TestModule_NewColumn_NilRoundPanic(t *testing.T) {
	sys, _, _, mod := newTestSystem(t)
	assert.Panics(t, func() { mod.NewColumn(sys.Context.Childf("c"), wiop.VisibilityOracle, nil) })
}

func TestModule_NewColumn_NilCtxPanic(t *testing.T) {
	_, r0, _, mod := newTestSystem(t)
	assert.Panics(t, func() { mod.NewColumn(nil, wiop.VisibilityOracle, r0) })
}

func TestModule_NewColumn_ReusedCtxPanic(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	ctx := sys.Context.Childf("col")
	mod.NewColumn(ctx, wiop.VisibilityOracle, r0) // first use is fine
	assert.Panics(t, func() {
		mod.NewColumn(ctx, wiop.VisibilityOracle, r0) // re-using same ctx must panic
	})
}

func TestModule_NewExtensionColumn(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewExtensionColumn(sys.Context.Childf("ext"), wiop.VisibilityOracle, r0)
	assert.True(t, col.IsExtension)
}

// ---- Cell / CoinField methods ----

func TestCell_Properties(t *testing.T) {
	sys, r0, _, _ := newTestSystem(t)
	cell := r0.NewCell(sys.Context.Childf("cell"), false)
	extCell := r0.NewCell(sys.Context.Childf("extcell"), true)

	assert.Equal(t, r0, cell.Round())
	assert.False(t, cell.IsExtension())
	assert.True(t, extCell.IsExtension())
	assert.False(t, cell.IsMultiValued())
	assert.Equal(t, 0, cell.Degree())
	assert.Nil(t, cell.Module())
	assert.Equal(t, wiop.VisibilityPublic, cell.Visibility())

	// scalar-only panics
	assert.Panics(t, func() { cell.Size() })
	assert.Panics(t, func() { cell.IsSized() })
}

func TestCoinField_Properties(t *testing.T) {
	sys, r0, _, _ := newTestSystem(t)
	coin := r0.NewCoinField(sys.Context.Childf("coin"))

	assert.Equal(t, r0, coin.Round())
	assert.True(t, coin.IsExtension())
	assert.False(t, coin.IsMultiValued())
	assert.Equal(t, 0, coin.Degree())
	assert.Nil(t, coin.Module())
	assert.Equal(t, wiop.VisibilityPublic, coin.Visibility())

	assert.Panics(t, func() { coin.Size() })
	assert.Panics(t, func() { coin.IsSized() })
}

func TestRound_NewCell_NilCtxPanic(t *testing.T) {
	_, r0, _, _ := newTestSystem(t)
	assert.Panics(t, func() { r0.NewCell(nil, false) })
}

func TestRound_NewCoinField_NilCtxPanic(t *testing.T) {
	_, r0, _, _ := newTestSystem(t)
	assert.Panics(t, func() { r0.NewCoinField(nil) })
}

// ---- Runtime: column assignment ----

func TestRuntime_AssignAndGetColumn(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	rt := wiop.NewRuntime(sys)

	assert.False(t, rt.HasColumnAssignment(col))

	v := baseVec(4, 7)
	rt.AssignColumn(col, v)

	assert.True(t, rt.HasColumnAssignment(col))
	got := rt.GetColumnAssignment(col)
	assert.Equal(t, v, got)
}

func TestRuntime_AssignColumn_WrongRoundPanic(t *testing.T) {
	sys, _, r1, mod := newTestSystem(t)
	// col belongs to r1 but runtime starts at r0
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r1)
	rt := wiop.NewRuntime(sys)
	assert.Panics(t, func() { rt.AssignColumn(col, baseVec(4, 0)) })
}

func TestRuntime_AssignColumn_DoublePanic(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(col, baseVec(4, 1))
	assert.Panics(t, func() { rt.AssignColumn(col, baseVec(4, 2)) })
}

func TestRuntime_GetColumnAssignment_UnassignedPanic(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	rt := wiop.NewRuntime(sys)
	assert.Panics(t, func() { rt.GetColumnAssignment(col) })
}

func TestRuntime_OverrideColumn(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	rt := wiop.NewRuntime(sys)

	rt.AssignColumn(col, baseVec(4, 7))
	replacement := baseVec(4, 9)
	rt.OverrideColumn(col, replacement)

	assert.Equal(t, replacement, rt.GetColumnAssignment(col),
		"override must replace the prior assignment")
}

func TestRuntime_OverrideColumn_UnassignedPanic(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	rt := wiop.NewRuntime(sys)
	assert.Panics(t, func() { rt.OverrideColumn(col, baseVec(4, 1)) },
		"overriding an unassigned column must panic")
}

// ---- Runtime: cell assignment ----

func TestRuntime_AssignAndGetCell(t *testing.T) {
	sys, r0, _, _ := newTestSystem(t)
	cell := r0.NewCell(sys.Context.Childf("cell"), false)
	rt := wiop.NewRuntime(sys)

	assert.False(t, rt.HasCellValue(cell))

	v := field.ElemFromBase(field.NewFromString("5"))
	rt.AssignCell(cell, v)

	assert.True(t, rt.HasCellValue(cell))
	got := rt.GetCellValue(cell)
	assert.Equal(t, v, got)
}

func TestRuntime_AssignCell_WrongRoundPanic(t *testing.T) {
	sys, _, r1, _ := newTestSystem(t)
	cell := r1.NewCell(sys.Context.Childf("cell"), false)
	rt := wiop.NewRuntime(sys)
	assert.Panics(t, func() { rt.AssignCell(cell, field.ElemZero()) })
}

func TestRuntime_AssignCell_DoublePanic(t *testing.T) {
	sys, r0, _, _ := newTestSystem(t)
	cell := r0.NewCell(sys.Context.Childf("cell"), false)
	rt := wiop.NewRuntime(sys)
	rt.AssignCell(cell, field.ElemZero())
	assert.Panics(t, func() { rt.AssignCell(cell, field.ElemZero()) })
}

func TestRuntime_GetCellValue_UnassignedPanic(t *testing.T) {
	sys, r0, _, _ := newTestSystem(t)
	cell := r0.NewCell(sys.Context.Childf("cell"), false)
	rt := wiop.NewRuntime(sys)
	assert.Panics(t, func() { rt.GetCellValue(cell) })
}

func TestRuntime_OverrideCell(t *testing.T) {
	sys, r0, _, _ := newTestSystem(t)
	cell := r0.NewCell(sys.Context.Childf("cell"), false)
	rt := wiop.NewRuntime(sys)

	rt.AssignCell(cell, field.ElemFromBase(field.NewFromString("5")))
	replacement := field.ElemFromBase(field.NewFromString("8"))
	rt.OverrideCell(cell, replacement)

	assert.Equal(t, replacement, rt.GetCellValue(cell),
		"override must replace the prior value")
}

func TestRuntime_OverrideCell_UnassignedPanic(t *testing.T) {
	sys, r0, _, _ := newTestSystem(t)
	cell := r0.NewCell(sys.Context.Childf("cell"), false)
	rt := wiop.NewRuntime(sys)
	assert.Panics(t, func() { rt.OverrideCell(cell, field.ElemZero()) },
		"overriding an unassigned cell must panic")
}

// ---- Runtime: state bag ----

func TestRuntime_State(t *testing.T) {
	sys, _, _, _ := newTestSystem(t)
	rt := wiop.NewRuntime(sys)

	_, ok := rt.GetState("k")
	assert.False(t, ok)

	rt.SetState("k", "hello")
	v, ok := rt.GetState("k")
	require.True(t, ok)
	assert.Equal(t, "hello", v)
}

// ---- Runtime: AdvanceRound and coins ----

func TestRuntime_AdvanceRound_Basic(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	rt := wiop.NewRuntime(sys)
	assert.Equal(t, r0, rt.CurrentRound())

	rt.AssignColumn(col, baseVec(4, 3))
	rt.AdvanceRound()
	assert.Equal(t, sys.Rounds[1], rt.CurrentRound())
}

func TestRuntime_AdvanceRound_WithCoinSampling(t *testing.T) {
	sys, r0, r1, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	coin := r1.NewCoinField(sys.Context.Childf("coin"))
	rt := wiop.NewRuntime(sys)

	rt.AssignColumn(col, baseVec(4, 1))
	rt.AdvanceRound()

	// coin must be available after advancing into r1
	v := rt.GetCoinValue(coin)
	// just assert it's deterministic (call twice with same state → same coin)
	assert.Equal(t, v, rt.GetCoinValue(coin))
}

// fixedSeedHook is a [wiop.ProverAction] that overrides the runtime's
// Fiat–Shamir state with a precomputed seed before any coin on the
// containing round is sampled. It is the test analogue of the original
// prover's SetInitialFSHash, used to verify that PreSamplingHooks fire at
// the right moment in AdvanceRound and that Runtime.SetFSState propagates
// into the coin derivation.
type fixedSeedHook struct {
	seed field.Octuplet
}

func (h *fixedSeedHook) Run(rt wiop.Runtime) {
	rt.SetFSState(h.seed)
}

// TestRound_PreSamplingHook_SeedsCoin verifies the wiring added for
// shared-randomness seeding:
//
//  1. A PreSamplingHook registered on round N fires during AdvanceRound
//     into N, *after* round (N-1)'s commitments have been absorbed and
//     *before* round N's coins are sampled.
//  2. Runtime.SetFSState invoked inside such a hook propagates into the
//     subsequent coin derivation, so the sampled coin is uniquely determined
//     by the seed (not by the natural FS transcript).
//
// The test reproduces the expected coin from an independent
// fiatshamir.FiatShamir instance seeded with the same value; the natural
// FS transcript would land on that exact extension-field element with
// negligible probability, so a match proves the hook ran and SetFSState
// took effect.
func TestRound_PreSamplingHook_SeedsCoin(t *testing.T) {
	sys, r0, r1, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	coin := r1.NewCoinField(sys.Context.Childf("coin"))

	seed := field.NewOctupletFromStrings([8]string{
		"1", "2", "3", "5", "8", "13", "21", "34",
	})
	r1.RegisterPreSamplingHook(&fixedSeedHook{seed: seed})

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(col, baseVec(4, 7)) // arbitrary, would influence natural FS
	rt.AdvanceRound()

	// Reproduce the post-seed coin from an independent FS instance.
	fs := fiatshamir.NewFiatShamir()
	fs.SetState(seed)
	expected := field.ElemFromExt(fs.RandomFext())

	assert.Equal(t, expected, rt.GetCoinValue(coin),
		"PreSamplingHook must seed the FS state before the coin loop fires; "+
			"sampled coin must equal the value produced by an independent FS instance with the same seed")
}

// TestRound_PreSamplingHook_RunsInOrder verifies two coupled contracts of
// the multi-hook path:
//
//  1. Registering a second PreSamplingHook on a round emits a logrus
//     warning. The wiring is informational, not blocking — see the
//     RegisterPreSamplingHook docstring — but the warning must fire so the
//     misuse is surfaced early.
//  2. Despite being discouraged, the stacked-hooks behaviour is still
//     well-defined: hooks run in registration order and the last hook's
//     SetFSState is the state from which the coins are derived.
func TestRound_PreSamplingHook_RunsInOrder(t *testing.T) {
	sys, r0, r1, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	coin := r1.NewCoinField(sys.Context.Childf("coin"))

	seedFirst := field.NewOctupletFromStrings([8]string{"1", "1", "1", "1", "1", "1", "1", "1"})
	seedLast := field.NewOctupletFromStrings([8]string{"9", "9", "9", "9", "9", "9", "9", "9"})

	// Capture logrus output so we can verify the multi-hook warning fires
	// on the second registration. Restore the standard logger's output on
	// exit so neighbouring tests are unaffected.
	var buf bytes.Buffer
	stdLogger := logrus.StandardLogger()
	origOut := stdLogger.Out
	stdLogger.SetOutput(&buf)
	defer stdLogger.SetOutput(origOut)

	r1.RegisterPreSamplingHook(&fixedSeedHook{seed: seedFirst})
	assert.Empty(t, buf.String(),
		"first PreSamplingHook registration must be silent")

	r1.RegisterPreSamplingHook(&fixedSeedHook{seed: seedLast})
	assert.Contains(t, buf.String(), "already has",
		"second PreSamplingHook registration must emit a logrus warning")

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(col, baseVec(4, 0))
	rt.AdvanceRound()

	// Expected coin derives from seedLast, since it is registered second
	// and overwrites the first hook's SetFSState.
	fs := fiatshamir.NewFiatShamir()
	fs.SetState(seedLast)
	expected := field.ElemFromExt(fs.RandomFext())

	assert.Equal(t, expected, rt.GetCoinValue(coin),
		"PreSamplingHooks must fire in registration order; the last SetFSState wins")
}

func TestRuntime_GetCoinValue_NotSampledPanic(t *testing.T) {
	sys, r0, r1, _ := newTestSystem(t)
	_ = r0
	coin := r1.NewCoinField(sys.Context.Childf("coin"))
	rt := wiop.NewRuntime(sys)
	// we have not advanced yet — coin is from r1
	assert.Panics(t, func() { rt.GetCoinValue(coin) })
}

func TestRuntime_AdvanceRound_LastRoundPanic(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(col, baseVec(4, 0))
	rt.AdvanceRound() // now at r1 (last round)
	assert.Panics(t, func() { rt.AdvanceRound() })
}

func TestRuntime_AdvanceRound_UnassignedOraclePanic(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	_ = mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0) // not assigned
	rt := wiop.NewRuntime(sys)
	assert.Panics(t, func() { rt.AdvanceRound() })
}

func TestRuntime_AdvanceRound_UnassignedCellPanic(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	_ = r0.NewCell(sys.Context.Childf("cell"), false) // not assigned
	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(col, baseVec(4, 0))
	assert.Panics(t, func() { rt.AdvanceRound() })
}

func TestNewRuntime_NoRoundsPanic(t *testing.T) {
	sys := wiop.NewSystemf("empty")
	assert.Panics(t, func() { wiop.NewRuntime(sys) })
}

// ---- HasCellAssignment ----

func TestRuntime_HasCellAssignment(t *testing.T) {
	sys, r0, _, _ := newTestSystem(t)
	cell := r0.NewCell(sys.Context.Childf("cell"), false)
	rt := wiop.NewRuntime(sys)
	assert.False(t, rt.HasCellAssignment(cell))
	rt.AssignCell(cell, field.ElemZero())
	assert.True(t, rt.HasCellAssignment(cell))
}
