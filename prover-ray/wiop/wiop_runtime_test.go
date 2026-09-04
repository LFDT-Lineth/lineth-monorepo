package wiop_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/fiatshamir"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
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
	col := mod.NewColumn(sys.Context.Childf("col"), r0)
	assert.Equal(t, r0, col.Round())
}

func TestColumn_Degree(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("col"), r0)
	// module is sized to 4 → degree == 3
	assert.Equal(t, 3, col.Degree())
}

func TestColumn_Degree_UnsizedPanic(t *testing.T) {
	sys, r0, _, _ := newTestSystem(t)
	unsized := sys.NewModule(sys.Context.Childf("unsized"), wiop.PaddingDirectionNone)
	col := unsized.NewColumn(sys.Context.Childf("col2"), r0)
	assert.Panics(t, func() { col.Degree() })
}

func TestModule_NewColumn_NilRoundPanic(t *testing.T) {
	sys, _, _, mod := newTestSystem(t)
	assert.Panics(t, func() { mod.NewColumn(sys.Context.Childf("c"), nil) })
}

func TestModule_NewColumn_NilCtxPanic(t *testing.T) {
	_, r0, _, mod := newTestSystem(t)
	assert.Panics(t, func() { mod.NewColumn(nil, r0) })
}

func TestModule_NewColumn_ReusedCtxPanic(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	ctx := sys.Context.Childf("col")
	mod.NewColumn(ctx, r0) // first use is fine
	assert.Panics(t, func() {
		mod.NewColumn(ctx, r0) // re-using same ctx must panic
	})
}

func TestModule_NewExtensionColumn(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewExtensionColumn(sys.Context.Childf("ext"), r0)
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
	col := mod.NewColumn(sys.Context.Childf("col"), r0)
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
	col := mod.NewColumn(sys.Context.Childf("col"), r1)
	rt := wiop.NewRuntime(sys)
	assert.Panics(t, func() { rt.AssignColumn(col, baseVec(4, 0)) })
}

func TestRuntime_AssignColumn_DoublePanic(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("col"), r0)
	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(col, baseVec(4, 1))
	assert.Panics(t, func() { rt.AssignColumn(col, baseVec(4, 2)) })
}

func TestRuntime_GetColumnAssignment_UnassignedPanic(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("col"), r0)
	rt := wiop.NewRuntime(sys)
	assert.Panics(t, func() { rt.GetColumnAssignment(col) })
}

func TestRuntime_OverrideColumn(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("col"), r0)
	rt := wiop.NewRuntime(sys)

	rt.AssignColumn(col, baseVec(4, 7))
	replacement := baseVec(4, 9)
	rt.OverrideColumn(col, replacement)

	assert.Equal(t, replacement, rt.GetColumnAssignment(col),
		"override must replace the prior assignment")
}

func TestRuntime_OverrideColumn_UnassignedPanic(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("col"), r0)
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
	col := mod.NewColumn(sys.Context.Childf("col"), r0)
	rt := wiop.NewRuntime(sys)
	assert.Equal(t, r0, rt.CurrentRound())

	rt.AssignColumn(col, baseVec(4, 3))
	rt.AdvanceRound()
	assert.Equal(t, sys.Rounds[1], rt.CurrentRound())
}

func TestRuntime_AdvanceRound_WithCoinSampling(t *testing.T) {
	sys, r0, r1, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("col"), r0)
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

func (h *fixedSeedHook) Run(rt *wiop.Runtime) {
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
	col := mod.NewColumn(sys.Context.Childf("col"), r0)
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

// TestRound_PreSamplingHook_SecondRegistrationPanics covers the
// one-hook-per-round rule. A round holds a single Fiat-Shamir state, so a second
// hook could only discard the first's work — there is no seed-per-coin to be had
// from stacking. Registering one therefore panics rather than silently letting
// the last one win.
func TestRound_PreSamplingHook_SecondRegistrationPanics(t *testing.T) {
	_, _, r1, _ := newTestSystem(t)

	seed := field.NewOctupletFromStrings([8]string{"1", "1", "1", "1", "1", "1", "1", "1"})

	r1.RegisterPreSamplingHook(&fixedSeedHook{seed: seed})
	assert.Panics(t, func() { r1.RegisterPreSamplingHook(&fixedSeedHook{seed: seed}) },
		"a round accepts one PreSamplingHook; the second must be refused")
}

// TestRound_MarkSeeded_ScopesSeedToMarkedCoins is the core property: a seeded
// coin is invisible to the transcript. On a round carrying a marked and an
// unmarked coin, the hook's seed reaches the marked one only, and the unmarked
// one comes out exactly as it would have if the marked coin never existed.
func TestRound_MarkSeeded_ScopesSeedToMarkedCoins(t *testing.T) {
	seed := field.NewOctupletFromStrings([8]string{
		"1", "2", "3", "5", "8", "13", "21", "34",
	})

	drive := func(sys *wiop.System, col *wiop.Column) *wiop.Runtime {
		rt := wiop.NewRuntime(sys)
		rt.AssignColumn(col, baseVec(4, 7)) // influences the natural transcript
		rt.AdvanceRound()
		return rt
	}

	// The system under test: a marked coin and an unmarked one on one round,
	// behind a seeding hook.
	sys, r0, r1, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("col"), r0)
	seeded := r1.NewCoinField(sys.Context.Childf("seeded"))
	plain := r1.NewCoinField(sys.Context.Childf("plain"))
	seeded.MarkSeeded()
	r1.RegisterPreSamplingHook(&fixedSeedHook{seed: seed})
	rt := drive(sys, col)

	// The twin: the plain coin alone, no hook, driven identically.
	twinSys, twinR0, twinR1, twinMod := newTestSystem(t)
	twinCol := twinMod.NewColumn(twinSys.Context.Childf("col"), twinR0)
	twinPlain := twinR1.NewCoinField(twinSys.Context.Childf("plain"))
	twinRt := drive(twinSys, twinCol)

	// The marked coin is the seed's first draw, reproduced independently.
	fs := fiatshamir.NewFiatShamir()
	fs.SetState(seed)
	assert.Equal(t, field.ElemFromExt(fs.RandomFext()), rt.GetCoinValue(seeded),
		"a coin marked seeded must be drawn from the state the hook installed")

	// The unmarked coin is untouched by the seeded draw that preceded it.
	assert.Equal(t, twinRt.GetCoinValue(twinPlain), rt.GetCoinValue(plain),
		"an unmarked coin sharing the round must derive from the local transcript, "+
			"exactly as it would if the seeded coin did not exist")

	// Control: the two coins really are drawn from different states, so the
	// assertions above are not comparing a system against itself.
	assert.NotEqual(t, rt.GetCoinValue(plain), rt.GetCoinValue(seeded),
		"setup is degenerate: the seeded and unmarked coins must differ")

	// The transcript itself survives the seeded round.
	assert.Equal(t, twinRt.GetFS().State(), rt.GetFS().State(),
		"the Fiat-Shamir state after a seeded round must match the twin's, so every "+
			"later challenge stays bound to what preceded the round")
}

// TestRound_MarkSeeded_WithoutHookPanics covers the wiring guard: a coin asking
// to be seeded on a round where nothing installs a seed would quietly derive
// from the ordinary transcript, which is a compile-time mistake rather than a
// proof failure — so AdvanceRound refuses it.
func TestRound_MarkSeeded_WithoutHookPanics(t *testing.T) {
	sys, r0, r1, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("col"), r0)
	r1.NewCoinField(sys.Context.Childf("coin")).MarkSeeded()

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(col, baseVec(4, 1))
	assert.Panics(t, func() { rt.AdvanceRound() },
		"a seeded coin on a round with no PreSamplingHook must panic")
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
	col := mod.NewColumn(sys.Context.Childf("col"), r0)
	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(col, baseVec(4, 0))
	rt.AdvanceRound() // now at r1 (last round)
	assert.Panics(t, func() { rt.AdvanceRound() })
}

func TestRuntime_AdvanceRound_UnassignedCellPanic(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("col"), r0)
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
