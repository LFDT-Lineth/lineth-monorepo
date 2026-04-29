package rangecheck_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/linea-monorepo/prover-ray/wiop"
	"github.com/consensys/linea-monorepo/prover-ray/wiop/compilers/rangecheck"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newRC builds a minimal system with one sized module and one RangeCheck.
func newRC(t *testing.T, b int) (sys *wiop.System, col *wiop.Column, rc *wiop.RangeCheck) {
	t.Helper()
	sys = wiop.NewSystemf("rc-test")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 8, wiop.PaddingDirectionNone)
	col = mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	rc = mod.NewRangeCheck(sys.Context.Childf("rc"), col, b)
	return
}

func makeVec(vals ...uint64) *wiop.ConcreteVector {
	elems := make([]field.Element, len(vals))
	for i, v := range vals {
		elems[i].SetUint64(v)
	}
	return &wiop.ConcreteVector{Plain: []field.Vec{field.VecFromBase(elems)}}
}

// ---- Structural tests ----

func TestCompile_CreatesInclusion(t *testing.T) {
	sys, _, rc := newRC(t, 8)
	rangecheck.Compile(sys)

	assert.True(t, rc.IsReduced(), "RangeCheck must be marked reduced after Compile")
	require.Len(t, sys.TableRelations, 1)
	assert.Equal(t, wiop.TableRelationInclusion, sys.TableRelations[0].Kind)
}

func TestCompile_SharedRangeColumn(t *testing.T) {
	sys := wiop.NewSystemf("rc-shared")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 8, wiop.PaddingDirectionNone)
	colA := mod.NewColumn(sys.Context.Childf("colA"), wiop.VisibilityOracle, r0)
	colB := mod.NewColumn(sys.Context.Childf("colB"), wiop.VisibilityOracle, r0)
	mod.NewRangeCheck(sys.Context.Childf("rcA"), colA, 4)
	mod.NewRangeCheck(sys.Context.Childf("rcB"), colB, 4)
	modulesBeforeCompile := len(sys.Modules) // 1

	rangecheck.Compile(sys)

	// One range module for B=4 shared across both RangeChecks.
	assert.Len(t, sys.Modules, modulesBeforeCompile+1)
	require.Len(t, sys.TableRelations, 2)
}

func TestCompile_DistinctBoundsDistinctModules(t *testing.T) {
	sys := wiop.NewSystemf("rc-distinct")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 8, wiop.PaddingDirectionNone)
	colA := mod.NewColumn(sys.Context.Childf("colA"), wiop.VisibilityOracle, r0)
	colB := mod.NewColumn(sys.Context.Childf("colB"), wiop.VisibilityOracle, r0)
	mod.NewRangeCheck(sys.Context.Childf("rcA"), colA, 4)
	mod.NewRangeCheck(sys.Context.Childf("rcB"), colB, 8)
	modulesBeforeCompile := len(sys.Modules)

	rangecheck.Compile(sys)

	// Two distinct bounds → two distinct range modules.
	assert.Len(t, sys.Modules, modulesBeforeCompile+2)
	require.Len(t, sys.TableRelations, 2)
}

func TestCompile_Idempotent(t *testing.T) {
	sys, _, _ := newRC(t, 4)
	rangecheck.Compile(sys)
	relationsAfterFirst := len(sys.TableRelations)
	modulesAfterFirst := len(sys.Modules)

	rangecheck.Compile(sys)

	assert.Len(t, sys.TableRelations, relationsAfterFirst,
		"second Compile must not add new relations")
	assert.Len(t, sys.Modules, modulesAfterFirst,
		"second Compile must not add new modules")
}

func TestCompile_NoRangeChecks(t *testing.T) {
	sys := wiop.NewSystemf("rc-none")
	sys.NewRound()
	sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)

	rangecheck.Compile(sys) // must not panic

	assert.Empty(t, sys.TableRelations)
}

// ---- Soundness tests ----

func TestCompile_Completeness(t *testing.T) {
	sys, col, _ := newRC(t, 8)
	rangecheck.Compile(sys)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(col, makeVec(0, 1, 2, 3, 4, 5, 6, 7))

	require.Len(t, sys.TableRelations, 1)
	require.NoError(t, sys.TableRelations[0].Check(rt),
		"all values in [0,8) must be accepted by the compiled Inclusion")
}

func TestCompile_Soundness_ValueAtBound(t *testing.T) {
	sys, col, _ := newRC(t, 4)
	rangecheck.Compile(sys)

	rt := wiop.NewRuntime(sys)
	// Value 4 == B is out of the range [0, 4).
	rt.AssignColumn(col, makeVec(0, 1, 2, 4, 0, 0, 0, 0))

	require.Len(t, sys.TableRelations, 1)
	assert.Error(t, sys.TableRelations[0].Check(rt),
		"value at bound must be rejected by the compiled Inclusion")
}

func TestCompile_Soundness_ValueAboveBound(t *testing.T) {
	sys, col, _ := newRC(t, 4)
	rangecheck.Compile(sys)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(col, makeVec(0, 1, 2, 7, 0, 0, 0, 0)) // 7 >= 4

	require.Len(t, sys.TableRelations, 1)
	assert.Error(t, sys.TableRelations[0].Check(rt),
		"value above bound must be rejected by the compiled Inclusion")
}
