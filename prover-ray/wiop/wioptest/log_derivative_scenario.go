package wioptest

import "github.com/consensys/linea-monorepo/prover-ray/wiop"

// LogDerivativeSumCompilerScenario is a fixture for testing the
// logderivativesum compiler pass end-to-end.
//
// The compiler reduces a [wiop.LogDerivativeSum] query into Z-column
// recurrences plus endpoint openings. A test typically calls
//
//	logderivativesum.Compile(sc.Sys)
//	rt := wiop.NewRuntime(sc.Sys)
//	sc.AssignWitness(&rt)        // honest witness columns
//	rt.AdvanceRound()
//	// For soundness paths, call sc.TamperResult(&rt) here.
//	for _, a := range rt.CurrentRound().ProverActions { a.Run(*rt) }
//	err := checkAllVerifierActions(&rt)
//
// then asserts the expected outcome.
//
// The split between AssignWitness and TamperResult exists because the
// [wiop.LogDerivativeSum] Result cell lives in the round AFTER the witness
// columns; AssignCell on that cell must therefore happen after AdvanceRound.
type LogDerivativeSumCompilerScenario struct {
	// Name identifies the scenario in test output.
	Name string
	// Sys is the pre-compilation System; each factory call returns an
	// independent Sys. Round structure is: r0 hosts the witness columns and
	// the LogDerivativeSum Result cell lives in r1 (i.e. the round
	// immediately after r0).
	Sys *wiop.System
	// AssignWitness assigns the witness columns honestly in round 0. Used by
	// both completeness and soundness paths.
	AssignWitness func(rt *wiop.Runtime)
	// TamperResult corrupts the [wiop.LogDerivativeSum] Result cell so the
	// verifier's claim-vs-running-sum identity rejects. Must be called after
	// AdvanceRound and before the prover action runs (the prover skips
	// re-assigning a cell that already holds a value).
	TamperResult func(rt *wiop.Runtime)
}

// LogDerivativeSumCompilerScenarios returns factory functions for the
// built-in logderivativesum compiler fixtures. Each call to a factory yields
// an independent [*LogDerivativeSumCompilerScenario].
//
// For these fixtures, AssignInvalid corrupts the [wiop.LogDerivativeSum]
// Result cell after AdvanceRound, so the prover-side recurrence stays honest
// but the verifier's "claimed result matches the running sum" check rejects.
// This focuses each invalid case on a single failure mode and avoids the
// prover-panic edge case (zero denominator on a non-filtered row).
func LogDerivativeSumCompilerScenarios() []func() *LogDerivativeSumCompilerScenario {
	return []func() *LogDerivativeSumCompilerScenario{
		NewLDSSingleFractionAllOnesScenario,
		NewLDSPartialFilterScenario,
		NewLDSAllZeroFilterScenario,
		NewLDSFilterMasksZeroDenominatorScenario,
		NewLDSPackingScenario,
		NewLDSMultiModuleBucketingScenario,
		NewLDSSizeOneModuleScenario,
		NewLDSConditionalLookupShapeScenario,
		// Additional coverage: large packing, multi-query, edge sizes.
		NewLDSManyFractionsScenario,
		NewLDSSizeTwoModuleScenario,
		NewLDSMultipleQueriesScenario,
		NewLDSVectorDenominatorScenario,
		NewLDSAllFiltersOnesPackedScenario,
	}
}
