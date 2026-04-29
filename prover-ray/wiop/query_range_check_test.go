package wiop_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/linea-monorepo/prover-ray/wiop"
	"github.com/consensys/linea-monorepo/prover-ray/wiop/wioptest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRangeCheck_Soundness_Completeness(t *testing.T) {
	sc := wioptest.NewRangeCheckScenario()
	rt := wiop.NewRuntime(sc.Sys)
	sc.RunHonest(&rt)
	require.NoError(t, sc.Query.Check(rt), "honest witness must pass Check")
}

func TestRangeCheck_Soundness_InvalidWitness(t *testing.T) {
	sc := wioptest.NewRangeCheckScenario()
	rt := wiop.NewRuntime(sc.Sys)
	sc.RunInvalid(&rt)
	assert.Error(t, sc.Query.Check(rt), "invalid witness must be rejected by Check")
}

func TestRangeCheck_Round(t *testing.T) {
	sys, r0, r1, mod := newTestSystem(t)
	col0 := mod.NewColumn(sys.Context.Childf("rc-r0"), wiop.VisibilityOracle, r0)
	col1 := mod.NewColumn(sys.Context.Childf("rc-r1"), wiop.VisibilityOracle, r1)
	rc0 := mod.NewRangeCheck(sys.Context.Childf("rc0"), col0, 4)
	rc1 := mod.NewRangeCheck(sys.Context.Childf("rc1"), col1, 4)
	assert.Equal(t, r0, rc0.Round())
	assert.Equal(t, r1, rc1.Round())
}

func TestRangeCheck_String(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("rc-col"), wiop.VisibilityOracle, r0)
	rc := mod.NewRangeCheck(sys.Context.Childf("rc"), col, 16)
	s := rc.String()
	assert.Contains(t, s, "RangeCheck")
	assert.Contains(t, s, "16")
}

func TestRangeCheck_Check_BoundaryValues(t *testing.T) {
	cases := []struct {
		name    string
		vals    []uint64
		b       int
		wantErr bool
	}{
		{"zero is valid", []uint64{0, 0, 0, 0}, 4, false},
		{"b-1 is valid", []uint64{3, 3, 3, 3}, 4, false},
		{"b is invalid", []uint64{0, 1, 2, 4}, 4, true},
		{"b+1 is invalid", []uint64{0, 1, 2, 5}, 4, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sys := wiop.NewSystemf("rc-bnd")
			r0 := sys.NewRound()
			mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
			col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
			rc := mod.NewRangeCheck(sys.Context.Childf("rc"), col, tc.b)
			rt := wiop.NewRuntime(sys)
			rt.AssignColumn(col, makeVecU64(tc.vals...))
			if tc.wantErr {
				assert.Error(t, rc.Check(rt))
			} else {
				assert.NoError(t, rc.Check(rt))
			}
		})
	}
}

func TestRangeCheck_NewRangeCheck_NilCtxPanic(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("rc-nil-col"), wiop.VisibilityOracle, r0)
	assert.Panics(t, func() { mod.NewRangeCheck(nil, col, 4) })
}

func TestRangeCheck_NewRangeCheck_NilColPanic(t *testing.T) {
	sys, _, _, mod := newTestSystem(t)
	assert.Panics(t, func() { mod.NewRangeCheck(sys.Context.Childf("rc"), nil, 4) })
}

func TestRangeCheck_NewRangeCheck_ZeroBoundPanic(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	assert.Panics(t, func() { mod.NewRangeCheck(sys.Context.Childf("rc"), col, 0) })
}

func TestRangeCheck_NewRangeCheck_NegativeBoundPanic(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	assert.Panics(t, func() { mod.NewRangeCheck(sys.Context.Childf("rc"), col, -1) })
}

func TestRangeCheck_IsReduced(t *testing.T) {
	sys, r0, _, mod := newTestSystem(t)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	rc := mod.NewRangeCheck(sys.Context.Childf("rc"), col, 4)
	assert.False(t, rc.IsReduced())
	rc.MarkAsReduced()
	assert.True(t, rc.IsReduced())
}

// makeVecU64 builds a PaddingDirectionNone ConcreteVector from explicit uint64 values.
func makeVecU64(vals ...uint64) *wiop.ConcreteVector {
	elems := make([]field.Element, len(vals))
	for i, v := range vals {
		elems[i].SetUint64(v)
	}
	return &wiop.ConcreteVector{Plain: []field.Vec{field.VecFromBase(elems)}}
}
