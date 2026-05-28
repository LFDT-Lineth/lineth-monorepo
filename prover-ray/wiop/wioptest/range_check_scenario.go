package wioptest

import "github.com/consensys/linea-monorepo/prover-ray/wiop"

// RangeCheckCompilerScenario is a fixture for testing the rangecheck →
// lookuptologderivsum → logderivativesum pipeline end-to-end.
//
// The rangecheck pass reduces a [wiop.RangeCheck] into an [wiop.LookupQuery]
// inclusion against a precomputed range column, which downstream passes
// then reduce further. A test typically calls
//
//	rangecheck.Compile(sc.Sys)
//	lookuptologderivsum.Compile(sc.Sys)
//	logderivativesum.Compile(sc.Sys)
//	rt := wiop.NewRuntime(sc.Sys)
//	sc.AssignWitness(&rt)
//	// then drives rounds 0 → 1 → 2.
type RangeCheckCompilerScenario struct {
	// Name identifies the scenario in test output.
	Name string
	// Sys is the pre-compilation System; each factory call returns an
	// independent Sys.
	Sys *wiop.System
	// AssignWitness assigns the user-witness columns in round 0.
	AssignWitness func(rt *wiop.Runtime)
}

// RangeCheckCompilerScenarios returns factory functions for the built-in
// honest-witness rangecheck pipeline fixtures.
func RangeCheckCompilerScenarios() []func() *RangeCheckCompilerScenario {
	return []func() *RangeCheckCompilerScenario{
		NewRangeCheckBasicScenario,
		NewRangeCheckSharedBoundScenario,
		NewRangeCheckDistinctBoundsScenario,
		NewRangeCheckBoundIsPowerOfTwoScenario,
		NewRangeCheckBoundIsOneScenario,
		// Additional coverage: cross-module sharing, larger bounds.
		NewRangeCheckMultiModuleScenario,
		NewRangeCheckLargeBoundScenario,
		NewRangeCheckNonPowerOfTwoBoundScenario,
		NewRangeCheckAllZerosScenario,
	}
}

// RangeCheckSoundnessScenario describes a runtime failure of the rangecheck
// pipeline: the assigned column contains a value outside the declared bound,
// so the M-assignment prover action of the downstream lookuptologderivsum
// compiler panics (no matching range-column row).
type RangeCheckSoundnessScenario struct {
	// Name identifies the scenario in test output.
	Name string
	// Sys is the pre-compilation System; each factory call returns an
	// independent Sys.
	Sys *wiop.System
	// AssignWitness assigns columns whose values fall outside the declared
	// bound.
	AssignWitness func(rt *wiop.Runtime)
}

// RangeCheckSoundnessScenarios returns factory functions for the soundness
// rangecheck fixtures.
func RangeCheckSoundnessScenarios() []func() *RangeCheckSoundnessScenario {
	return []func() *RangeCheckSoundnessScenario{
		NewRangeCheckValueAtBoundScenario,
		NewRangeCheckValueAboveBoundScenario,
	}
}
