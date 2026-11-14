package wiop_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/global"
	"github.com/stretchr/testify/require"
)

// newFixSizeSystem builds a system with one static module (index 0) and two
// dynamic modules (indices 1 and 2). Module 2 is opened at shift 3, so its
// minimum runtime size is 8.
func newFixSizeSystem() *wiop.System {
	sys := wiop.NewSystemf("fix-size")
	r0 := sys.NewRound()
	r1 := sys.NewRound()
	sys.NewSizedModule(sys.Context.Childf("static"), 4, wiop.PaddingDirectionNone)
	sys.NewDynamicModule(sys.Context.Childf("dyn-a"), wiop.PaddingDirectionRight)
	shifted := sys.NewDynamicModule(sys.Context.Childf("dyn-b"), wiop.PaddingDirectionLeft)
	col := shifted.NewColumn(sys.Context.Childf("col"), r0)
	zeta := r1.NewCoinField(sys.Context.Childf("zeta"))
	sys.NewLagrangeEval(sys.Context.Childf("le"), []*wiop.ColumnView{col.View().Shift(3)}, zeta)
	return sys
}

func TestFixDynamicSizes(t *testing.T) {
	sys := newFixSizeSystem()
	require.Equal(t, 8, sys.Modules[2].MinRuntimeSize(), "fixture: shift 3 needs size 8")

	require.NoError(t, sys.FixDynamicSizes(map[int]int{1: 16, 2: 8}))
	for k, want := range map[int]int{0: 4, 1: 16, 2: 8} {
		require.False(t, sys.Modules[k].IsDynamic(), "module %d must be static", k)
		require.Equal(t, want, sys.Modules[k].Size(), "module %d size", k)
	}
}

func TestFixDynamicSizes_Rejects(t *testing.T) {
	for _, tc := range []struct {
		name    string
		sizes   map[int]int
		wantErr string
	}{
		{"missing module", map[int]int{1: 16}, `dynamic module "fix-size/dyn-b" has no size`},
		{"static module", map[int]int{0: 4, 1: 16, 2: 8}, `module "fix-size/static" is not dynamic`},
		{"out of range", map[int]int{1: 16, 2: 8, 3: 8}, "module index 3 out of range [0, 3)"},
		{"negative index", map[int]int{-1: 8, 1: 16, 2: 8}, "module index -1 out of range [0, 3)"},
		{"not a power of two", map[int]int{1: 12, 2: 8}, "size 12 is not a power of two"},
		{"zero", map[int]int{1: 0, 2: 8}, "size 0 is not a power of two"},
		{"below minimum", map[int]int{1: 16, 2: 4}, "size 4 is below its minimum 8"},
		{"above maximum", map[int]int{1: 2 * wiop.ColumnSizeMaxSupported, 2: 8}, "exceeds the maximum supported"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sys := newFixSizeSystem()
			require.ErrorContains(t, sys.FixDynamicSizes(tc.sizes), tc.wantErr)
			// All or nothing: a rejected call leaves every module as it was.
			require.True(t, sys.Modules[1].IsDynamic(), "module 1 must stay dynamic")
			require.True(t, sys.Modules[2].IsDynamic(), "module 2 must stay dynamic")
		})
	}
}

func TestFixDynamicSizes_RejectsCompiledSystem(t *testing.T) {
	sys := wiop.NewSystemf("fix-size-compiled")
	r0 := sys.NewRound()
	sys.NewRound()
	mod := sys.NewDynamicModule(sys.Context.Childf("mod"), wiop.PaddingDirectionRight)
	col := mod.NewColumn(sys.Context.Childf("col"), r0)
	mod.NewVanishing(sys.Context.Childf("v"), wiop.Mul(col.View(), col.View()))
	global.Compile(sys)

	require.ErrorContains(t, sys.FixDynamicSizes(map[int]int{0: 8}), "is already compiled")
	require.True(t, sys.Modules[0].IsDynamic())
}
