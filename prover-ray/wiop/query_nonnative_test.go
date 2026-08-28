package wiop_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/wioptest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// NB! The tests here are only runtime tests, they do not establish
// cryptographic soundness of the NonNative query. Soundness is established by
// compiling the system using the nonnative and global compilers, which
// transform the NonNative query into a combination of range checks and quotient
// arguments that are soundly enforced by the proof system.

// ---- Soundness ----

func TestNonNative_Soundness_Completeness(t *testing.T) {
	sc := wioptest.NewNonNativeScenario()
	rt := wiop.NewRuntime(sc.Sys)
	sc.RunHonest(rt)
	require.NoError(t, sc.Query.Check(rt), "honest witness must pass Check")
}

func TestNonNative_Soundness_InvalidWitness(t *testing.T) {
	sc := wioptest.NewNonNativeScenario()
	rt := wiop.NewRuntime(sc.Sys)
	sc.RunInvalid(rt)
	assert.Error(t, sc.Query.Check(rt), "invalid witness must be rejected by Check")
}

// ---- Round ----

func TestNonNative_Round(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	left := singleLimb(sys, mod, r0, "left")
	right := singleLimb(sys, mod, r0, "right")
	modulus := singleLimb(sys, mod, r0, "modulus")
	result := singleLimb(sys, mod, r0, "result")
	quotient := singleLimb(sys, mod, r0, "quotient")

	q := mod.NewNonNative(sys.Context.Childf("nn"), 16, left, right, modulus, result, quotient)
	assert.Equal(t, r0, q.Round())
}

// ---- Check: boundary values ----

func TestNonNative_Check_BoundaryValues(t *testing.T) {
	cases := []struct {
		name             string
		left, right      uint64
		modulus, quo     uint64
		result           uint64
		wantErr          bool
		wantErrSubstring string
	}{
		{
			name: "valid multiplication", left: 3, right: 4, modulus: 5, quo: 2, result: 2,
		},
		{
			name: "invalid: product mismatch", left: 3, right: 4, modulus: 5, quo: 2, result: 3,
			wantErr: true, wantErrSubstring: "left*right",
		},
		{
			// left*right=15, quo*modulus+result=2*5+5=15: the algebraic
			// identity holds, but result (5) is not < modulus (5).
			name: "invalid: result not reduced", left: 3, right: 5, modulus: 5, quo: 2, result: 5,
			wantErr: true, wantErrSubstring: "not reduced",
		},
		{
			// modulus=0 makes quotient*modulus always zero regardless of
			// quotient, so the relation degenerates to left*right=result.
			name: "modulus zero: honest", left: 3, right: 4, modulus: 0, quo: 999, result: 12,
		},
		{
			name: "modulus zero: invalid", left: 3, right: 4, modulus: 0, quo: 0, result: 13,
			wantErr: true, wantErrSubstring: "left*right",
		},
		{
			name: "zero operands", left: 0, right: 0, modulus: 5, quo: 0, result: 0,
		},
		{
			// result=0 is always reduced, regardless of how large modulus is.
			name: "result exactly zero", left: 10, right: 10, modulus: 20, quo: 5, result: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sys := wiop.NewSystemf("nn-bnd")
			r0 := sys.NewRound()
			mod := sys.NewSizedModule(sys.Context.Childf("mod"), 1, wiop.PaddingDirectionNone)

			left := singleLimb(sys, mod, r0, "left")
			right := singleLimb(sys, mod, r0, "right")
			modulus := singleLimb(sys, mod, r0, "modulus")
			result := singleLimb(sys, mod, r0, "result")
			quotient := singleLimb(sys, mod, r0, "quotient")
			q := mod.NewNonNative(sys.Context.Childf("nn"), 16, left, right, modulus, result, quotient)

			rt := wiop.NewRuntime(sys)
			rt.AssignColumn(left[0], makeVecU64(tc.left))
			rt.AssignColumn(right[0], makeVecU64(tc.right))
			rt.AssignColumn(modulus[0], makeVecU64(tc.modulus))
			rt.AssignColumn(result[0], makeVecU64(tc.result))
			rt.AssignColumn(quotient[0], makeVecU64(tc.quo))

			err := q.Check(rt)
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErrSubstring)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestNonNative_Check_MultiLimb exercises Check with operands that span more
// than one limb, so the little-endian recomposition in composeLimbsRow is
// actually exercised across a limb boundary.
//
// 99991*99989 = 99980*100000 + 99, and every operand except Result requires
// 2 limbs of 16 bits (>= 2^16).
func TestNonNative_Check_MultiLimb(t *testing.T) {
	build := func(resultLimb0 uint64) (*wiop.NonNative, *wiop.Runtime) {
		sys := wiop.NewSystemf("nn-multi")
		r0 := sys.NewRound()
		mod := sys.NewSizedModule(sys.Context.Childf("mod"), 1, wiop.PaddingDirectionNone)

		newLimbs := func(label string) []*wiop.Column {
			return []*wiop.Column{
				mod.NewColumn(sys.Context.Childf("%s-limb-0", label), r0),
				mod.NewColumn(sys.Context.Childf("%s-limb-1", label), r0),
			}
		}
		left := newLimbs("left")
		right := newLimbs("right")
		modulus := newLimbs("modulus")
		result := newLimbs("result")
		quotient := newLimbs("quotient")
		q := mod.NewNonNative(sys.Context.Childf("nn"), 16, left, right, modulus, result, quotient)

		rt := wiop.NewRuntime(sys)
		rt.AssignColumn(left[0], makeVecU64(34455))
		rt.AssignColumn(left[1], makeVecU64(1))
		rt.AssignColumn(right[0], makeVecU64(34453))
		rt.AssignColumn(right[1], makeVecU64(1))
		rt.AssignColumn(modulus[0], makeVecU64(34464))
		rt.AssignColumn(modulus[1], makeVecU64(1))
		rt.AssignColumn(quotient[0], makeVecU64(34444))
		rt.AssignColumn(quotient[1], makeVecU64(1))
		rt.AssignColumn(result[0], makeVecU64(resultLimb0))
		rt.AssignColumn(result[1], makeVecU64(0))
		return q, rt
	}

	honestQ, honestRt := build(99)
	require.NoError(t, honestQ.Check(honestRt))

	// Perturb result[0] so the relation no longer holds.
	invalidQ, invalidRt := build(100)
	assert.Error(t, invalidQ.Check(invalidRt))
}

// TestNonNative_Check_UnassignedColumnsReturnsNil documents that Check is a
// no-op (returns nil) when any referenced limb column has no runtime
// assignment, which happens when checking a query against a proof that only
// carries public columns rather than the full trace.
func TestNonNative_Check_UnassignedColumnsReturnsNil(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	left := singleLimb(sys, mod, r0, "left")
	right := singleLimb(sys, mod, r0, "right")
	modulus := singleLimb(sys, mod, r0, "modulus")
	result := singleLimb(sys, mod, r0, "result")
	quotient := singleLimb(sys, mod, r0, "quotient")
	q := mod.NewNonNative(sys.Context.Childf("nn"), 16, left, right, modulus, result, quotient)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(left[0], baseVec(4, 3))
	rt.AssignColumn(right[0], baseVec(4, 4))
	rt.AssignColumn(modulus[0], baseVec(4, 5))
	rt.AssignColumn(quotient[0], baseVec(4, 2))
	// Result is intentionally left unassigned.

	assert.NoError(t, q.Check(rt), "Check must be a no-op when a referenced column is unassigned")
}

// ---- IsReduced ----

func TestNonNative_IsReduced(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	left := singleLimb(sys, mod, r0, "left")
	right := singleLimb(sys, mod, r0, "right")
	modulus := singleLimb(sys, mod, r0, "modulus")
	result := singleLimb(sys, mod, r0, "result")
	quotient := singleLimb(sys, mod, r0, "quotient")
	q := mod.NewNonNative(sys.Context.Childf("nn"), 16, left, right, modulus, result, quotient)

	assert.False(t, q.IsReduced())
	q.MarkAsReduced()
	assert.True(t, q.IsReduced())
}

// ---- NewNonNative: panics ----

func TestNonNative_NewNonNative_NilModulePanic(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	left := singleLimb(sys, mod, r0, "left")
	right := singleLimb(sys, mod, r0, "right")
	modulus := singleLimb(sys, mod, r0, "modulus")
	result := singleLimb(sys, mod, r0, "result")
	quotient := singleLimb(sys, mod, r0, "quotient")

	var nilMod *wiop.Module
	assert.Panics(t, func() {
		nilMod.NewNonNative(sys.Context.Childf("nn"), 16, left, right, modulus, result, quotient)
	})
}

func TestNonNative_NewNonNative_NilCtxPanic(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	left := singleLimb(sys, mod, r0, "left")
	right := singleLimb(sys, mod, r0, "right")
	modulus := singleLimb(sys, mod, r0, "modulus")
	result := singleLimb(sys, mod, r0, "result")
	quotient := singleLimb(sys, mod, r0, "quotient")
	assert.Panics(t, func() {
		mod.NewNonNative(nil, 16, left, right, modulus, result, quotient)
	})
}

func TestNonNative_NewNonNative_ZeroBitsPerLimbPanic(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	left := singleLimb(sys, mod, r0, "left")
	right := singleLimb(sys, mod, r0, "right")
	modulus := singleLimb(sys, mod, r0, "modulus")
	result := singleLimb(sys, mod, r0, "result")
	quotient := singleLimb(sys, mod, r0, "quotient")
	assert.Panics(t, func() {
		mod.NewNonNative(sys.Context.Childf("nn"), 0, left, right, modulus, result, quotient)
	})
}

func TestNonNative_NewNonNative_EmptyLimbSlicePanic(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	right := singleLimb(sys, mod, r0, "right")
	modulus := singleLimb(sys, mod, r0, "modulus")
	result := singleLimb(sys, mod, r0, "result")
	quotient := singleLimb(sys, mod, r0, "quotient")
	assert.Panics(t, func() {
		mod.NewNonNative(sys.Context.Childf("nn"), 16, nil, right, modulus, result, quotient)
	})
}

func TestNonNative_NewNonNative_MismatchedLimbCountPanic(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	left := []*wiop.Column{
		mod.NewColumn(sys.Context.Childf("left-0"), r0),
		mod.NewColumn(sys.Context.Childf("left-1"), r0),
	}
	right := singleLimb(sys, mod, r0, "right")
	modulus := singleLimb(sys, mod, r0, "modulus")
	result := singleLimb(sys, mod, r0, "result")
	quotient := singleLimb(sys, mod, r0, "quotient")
	assert.Panics(t, func() {
		mod.NewNonNative(sys.Context.Childf("nn"), 16, left, right, modulus, result, quotient)
	})
}

func TestNonNative_NewNonNative_NilColumnPanic(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	left := []*wiop.Column{nil}
	right := singleLimb(sys, mod, r0, "right")
	modulus := singleLimb(sys, mod, r0, "modulus")
	result := singleLimb(sys, mod, r0, "result")
	quotient := singleLimb(sys, mod, r0, "quotient")
	assert.Panics(t, func() {
		mod.NewNonNative(sys.Context.Childf("nn"), 16, left, right, modulus, result, quotient)
	})
}

func TestNonNative_NewNonNative_ColumnFromDifferentModulePanic(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	otherMod := sys.NewSizedModule(sys.Context.Childf("other-mod"), 4, wiop.PaddingDirectionNone)
	left := []*wiop.Column{otherMod.NewColumn(sys.Context.Childf("left"), r0)}
	right := singleLimb(sys, mod, r0, "right")
	modulus := singleLimb(sys, mod, r0, "modulus")
	result := singleLimb(sys, mod, r0, "result")
	quotient := singleLimb(sys, mod, r0, "quotient")
	assert.Panics(t, func() {
		mod.NewNonNative(sys.Context.Childf("nn"), 16, left, right, modulus, result, quotient)
	})
}

func TestNonNative_NewNonNative_ColumnFromDifferentRoundPanic(t *testing.T) {
	sys, r0, r1, mod := newTestSystem(t)
	left := singleLimb(sys, mod, r0, "left")
	right := []*wiop.Column{mod.NewColumn(sys.Context.Childf("right"), r1)}
	modulus := singleLimb(sys, mod, r0, "modulus")
	result := singleLimb(sys, mod, r0, "result")
	quotient := singleLimb(sys, mod, r0, "quotient")
	assert.Panics(t, func() {
		mod.NewNonNative(sys.Context.Childf("nn"), 16, left, right, modulus, result, quotient)
	})
}

// singleLimb declares a single-column limb slice for label in mod/round r.
func singleLimb(sys *wiop.System, mod *wiop.Module, r *wiop.Round, label string) []*wiop.Column {
	return []*wiop.Column{mod.NewColumn(sys.Context.Childf("%s", label), r)}
}
