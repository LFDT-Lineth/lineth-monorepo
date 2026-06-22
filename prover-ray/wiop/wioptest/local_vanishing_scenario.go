package wioptest

import "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"

// LocalVanishingScenario is a fixture for testing the localvanishing → global
// compiler pipeline.
//
// The localvanishing pass lifts scalar (row-pinned) vanishings into
// multi-valued vanishings via the Lagrange-indicator trick, which the global
// pass then discharges through the quotient argument. A test typically calls
//
//	localvanishing.Compile(sc.Sys)
//	global.Compile(sc.Sys)
//	rt := wiop.NewRuntime(sc.Sys)
//	sc.AssignHonest(&rt)       // (or AssignInvalid)
//	err := wioptest.RunAndVerify(&rt)
//
// before asserting nil-error vs. non-nil-error.
type LocalVanishingScenario struct {
	// Name identifies the scenario in test output.
	Name string
	// Sys is the pre-compilation System; each factory call returns an
	// independent Sys.
	Sys *wiop.System
	// AssignHonest assigns columns (and cells, when used) with a witness that
	// satisfies every scalar [wiop.Vanishing] registered via
	// [wiop.Module.NewLocalConstraint] on Sys.
	AssignHonest func(rt *wiop.Runtime)
	// AssignInvalid assigns columns/cells with a witness that violates at
	// least one constraint, so the compiled verifier must reject.
	AssignInvalid func(rt *wiop.Runtime)
}

// LocalVanishingScenarios returns factory functions for the built-in scalar
// vanishing fixtures. Each call to a factory yields an independent
// [*LocalVanishingScenario].
func LocalVanishingScenarios() []func() *LocalVanishingScenario {
	return []func() *LocalVanishingScenario{
		NewLocalSingleColumnFirstRowZeroScenario,
		NewLocalSingleColumnLastRowZeroScenario,
		NewLocalShiftedColumnFirstRowZeroScenario,
		NewLocalTwoColumnsEqualAtFirstRowScenario,
		NewLocalMultipleConstraintsSameModuleScenario,
		NewLocalSecondRowConstraintScenario,
		NewLocalCellEqualityScenario,
		NewLocalCoinScaledScenario,
		NewLocalMultipleAnchorsSharedColumnScenario,
		NewLocalConstantSubtractionScenario,
		NewLocalWrapAroundShiftScenario,
		NewLocalProductIsZeroScenario,
		NewLocalCellAndCoinScenario,
		NewLocalThreeColumnLinearScenario,
		NewLocalMultiAnchorMultiColumnScenario,
		NewLocalCubeAtFirstRowScenario,
		NewLocalMultiModuleScenario,
		NewLocalDynamicFirstRowZeroScenario,
		NewLocalDynamicShiftedScenario,
		NewLocalDynamicProductIsZeroScenario,
	}
}
