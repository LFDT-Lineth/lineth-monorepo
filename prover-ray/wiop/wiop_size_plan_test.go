package wiop_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/global"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/wioptest"
	"github.com/stretchr/testify/require"
)

// newFixSizeSystem's dynamic modules: index 1 and 2.
const (
	pathDynA = "fix-size/dyn-a"
	pathDynB = "fix-size/dyn-b"
)

func TestMeasureDynamicSizes(t *testing.T) {
	sys := newFixSizeSystem()
	proofs := []wiop.Proof{
		{DynamicSizes: map[int]int{1: 16, 2: 8}},
		{DynamicSizes: map[int]int{1: 4, 2: 32}},
	}
	got, err := sys.MeasureDynamicSizes(proofs...)
	require.NoError(t, err)
	require.Equal(t, wiop.ModuleSizes{pathDynA: 16, pathDynB: 32}, got, "the largest size per module")
}

func TestMeasureDynamicSizes_Rejects(t *testing.T) {
	for _, tc := range []struct {
		name    string
		proofs  []wiop.Proof
		wantErr string
	}{
		{"no proof", nil, "no proof given"},
		{"missing module", []wiop.Proof{{DynamicSizes: map[int]int{1: 16}}}, `proof 0 has no size for dynamic module "fix-size/dyn-b"`},
		{"static module", []wiop.Proof{{DynamicSizes: map[int]int{0: 4, 1: 16, 2: 8}}}, "module index 0, which is not a dynamic module"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := newFixSizeSystem().MeasureDynamicSizes(tc.proofs...)
			require.ErrorContains(t, err, tc.wantErr)
		})
	}

	static := wiop.NewSystemf("static-only")
	static.NewRound()
	static.NewSizedModule(static.Context.Childf("m"), 4, wiop.PaddingDirectionNone)
	_, err := static.MeasureDynamicSizes(wiop.Proof{})
	require.ErrorContains(t, err, "has no dynamic module")
}

func TestModuleSizesScaled(t *testing.T) {
	s := wiop.ModuleSizes{pathDynA: 8, pathDynB: 16}
	require.Equal(t, wiop.ModuleSizes{pathDynA: 16, pathDynB: 32}, s.Scaled(2))
	require.Equal(t, wiop.ModuleSizes{pathDynA: 8, pathDynB: 16}, s, "Scaled must not modify its receiver")
	require.Panics(t, func() { s.Scaled(3) })
	require.Panics(t, func() { s.Scaled(0) })
}

func TestCheckSizesCover(t *testing.T) {
	sys := newFixSizeSystem()
	measured := wiop.ModuleSizes{pathDynA: 16, pathDynB: 8}
	require.NoError(t, sys.CheckSizesCover(wiop.ModuleSizes{pathDynA: 16, pathDynB: 32}, measured))

	err := sys.CheckSizesCover(wiop.ModuleSizes{pathDynA: 8, "fix-size/gone": 8}, measured)
	require.ErrorContains(t, err, "3 problem(s)")
	require.ErrorContains(t, err, `dynamic module "fix-size/dyn-a" needs size 16, pinned to 8`)
	require.ErrorContains(t, err, `dynamic module "fix-size/dyn-b" has no pinned size`)
	require.ErrorContains(t, err, `pinned size for "fix-size/gone", which is not a dynamic module`)

	err = sys.CheckSizesCover(wiop.ModuleSizes{pathDynA: 16, pathDynB: 8}, wiop.ModuleSizes{pathDynA: 16})
	require.ErrorContains(t, err, `dynamic module "fix-size/dyn-b" has no measured size`)
}

func TestFixDynamicSizesByPath(t *testing.T) {
	sys := newFixSizeSystem()
	require.NoError(t, sys.FixDynamicSizesByPath(wiop.ModuleSizes{pathDynA: 16, pathDynB: 8}))
	require.Equal(t, 16, sys.Modules[1].Size())
	require.Equal(t, 8, sys.Modules[2].Size())

	sys = newFixSizeSystem()
	require.ErrorContains(t,
		sys.FixDynamicSizesByPath(wiop.ModuleSizes{pathDynA: 16, pathDynB: 8, "fix-size/gone": 8}),
		`no module has the path "fix-size/gone"`)
	require.True(t, sys.Modules[1].IsDynamic(), "a rejected plan must change nothing")

	// Errors of FixDynamicSizes come through, named by path.
	require.ErrorContains(t, sys.FixDynamicSizesByPath(wiop.ModuleSizes{pathDynA: 16}),
		`dynamic module "fix-size/dyn-b" has no size`)
}

// TestSizePlan_EndToEnd runs the terminal-layer recipe: prove the dynamic
// system, measure, check the plan covers the measurement, pin the sizes by path
// and prove the fixed system. A stale plan fails the coverage check, and data
// overflowing a pinned size fails the prover naming the plan.
func TestSizePlan_EndToEnd(t *testing.T) {
	probe := wioptest.NewDynamicQuadraticVanishingScenario()
	global.Compile(probe.Sys)
	probeProof, _ := probe.Sys.Prove(probe.AssignHonest)

	measured, err := wioptest.NewDynamicQuadraticVanishingScenario().Sys.MeasureDynamicSizes(probeProof)
	require.NoError(t, err)
	require.Equal(t, wiop.ModuleSizes{"dyn-quad/mod": 8}, measured)

	plan := measured.Scaled(2)
	fresh := wioptest.NewDynamicQuadraticVanishingScenario()
	require.NoError(t, fresh.Sys.CheckSizesCover(plan, measured))
	require.ErrorContains(t, fresh.Sys.CheckSizesCover(wiop.ModuleSizes{"dyn-quad/mod": 4}, measured),
		`needs size 8, pinned to 4`)

	require.NoError(t, fresh.Sys.FixDynamicSizesByPath(plan))
	global.Compile(fresh.Sys)
	proof, pub := fresh.Sys.Prove(fresh.AssignHonest)
	require.NoError(t, fresh.Sys.Verify(proof, pub))

	stale := wioptest.NewDynamicQuadraticVanishingScenario()
	require.NoError(t, stale.Sys.FixDynamicSizesByPath(wiop.ModuleSizes{"dyn-quad/mod": 4}))
	global.Compile(stale.Sys)
	require.PanicsWithValue(t,
		`wiop: AssignColumn: column "dyn-quad/col" has data length 8 which overflows module "dyn-quad/mod", pinned to size 4 `+
			`by System.FixDynamicSizes: the size plan is stale, re-measure it (see System.MeasureDynamicSizes)`,
		func() { stale.Sys.Prove(stale.AssignHonest) })
}
