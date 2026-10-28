package wiop_test

import (
	"fmt"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/wioptest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMinRuntimeSize checks the minimum size of a dynamic module against its
// largest opening shift: the smallest power of two N > 2·max|s|.
func TestMinRuntimeSize(t *testing.T) {
	for _, tc := range []struct {
		shifts []int
		want   int
	}{
		{nil, 1},
		{[]int{0}, 1},
		{[]int{1}, 4},
		{[]int{-1}, 4},
		{[]int{0, 2}, 8},
		{[]int{1, -3}, 8},
		{[]int{4}, 16},
	} {
		t.Run(fmt.Sprint(tc.shifts), func(t *testing.T) {
			sys := wiop.NewSystemf("min-size")
			r0, r1 := sys.NewRound(), sys.NewRound()
			mod := sys.NewDynamicModule(sys.Context.Childf("dyn"), wiop.PaddingDirectionRight)
			col := mod.NewColumn(sys.Context.Childf("col"), r0)
			other := sys.NewDynamicModule(sys.Context.Childf("other"), wiop.PaddingDirectionRight)
			other.NewColumn(sys.Context.Childf("unopened"), r0)

			if tc.shifts != nil {
				views := make([]*wiop.ColumnView, len(tc.shifts))
				for i, s := range tc.shifts {
					views[i] = col.View().Shift(s)
				}
				sys.NewLagrangeEval(sys.Context.Childf("eval"), views, r1.NewCoinField(sys.Context.Childf("x")))
			}

			assert.Equal(t, tc.want, mod.MinRuntimeSize())
			assert.Equal(t, 1, other.MinRuntimeSize(), "a module without openings has minimum 1")
			assert.Equal(t, map[int]int{0: tc.want, 1: 1}, sys.MinRuntimeSizes())
		})
	}
}

// TestProveVerifyDynamicSizeMinimum: a dynamic module whose logup column is
// read at shift −1 has the minimum size 4. Proving refuses it below, an honest
// proof at the minimum verifies, and the verifier rejects that proof with its
// size claimed below the minimum.
func TestProveVerifyDynamicSizeMinimum(t *testing.T) {
	sys := wiop.NewSystemf("min-size-e2e")
	r0 := sys.NewRound()
	table := sys.NewSizedModule(sys.Context.Childf("table"), 2, wiop.PaddingDirectionRight)
	colT := table.NewColumn(sys.Context.Childf("T"), r0)
	dyn := sys.NewDynamicModule(sys.Context.Childf("dyn"), wiop.PaddingDirectionRight)
	colS := dyn.NewColumn(sys.Context.Childf("S"), r0)
	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTable(colS.View())},
		[]wiop.Table{wiop.NewTable(colT.View())},
	)
	dynIdx := -1
	for i, m := range sys.Modules {
		if m == dyn {
			dynIdx = i
		}
	}
	require.NoError(t, compilers.CompileFull(sys, wioptest.TestingCompileOptions()...))
	require.Equal(t, 4, dyn.MinRuntimeSize(), "the logup column of the dynamic module is read at shift -1")

	vec := func(n int) *wiop.ConcreteVector {
		elems := make([]field.Element, n)
		for i := range elems {
			elems[i].SetUint64(7)
		}
		return &wiop.ConcreteVector{Plain: field.VecFromBase(elems)}
	}
	assign := func(n int) func(rt *wiop.Runtime) {
		return func(rt *wiop.Runtime) {
			rt.AssignColumn(colT, vec(2))
			rt.AssignColumn(colS, vec(n))
		}
	}

	assert.PanicsWithError(t,
		`wiop: dynamic module "min-size-e2e/dyn" has size 2, below its minimum 4 (shifts up to ±1 need N > 2)`,
		func() { sys.Prove(assign(2)) },
		"proving must refuse a dynamic module below its minimum")

	proof, pub := sys.Prove(assign(4))
	require.NoError(t, sys.Verify(proof, pub), "an honest proof at the minimum must verify")

	proof.DynamicSizes[dynIdx] = 2
	assert.ErrorContains(t, sys.Verify(proof, pub), "below its minimum",
		"the verifier must reject a dynamic size below the minimum")
}
