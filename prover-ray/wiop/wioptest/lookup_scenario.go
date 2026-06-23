package wioptest

import "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"

// LookupScenario is a fixture for testing the
// lookuptologderivsum → logderivativesum compiler pipeline end-to-end.
//
// Round structure used by every fixture: r0 hosts the witness columns
// (plus the multiplicity column M added by the lookup compiler); the lookup
// compiler also allocates two more rounds — one for the α/γ coins and one
// for the LogDerivativeSum result. A test typically calls
//
//	lookuptologderivsum.Compile(sc.Sys)
//	logderivativesum.Compile(sc.Sys)
//	rt := wiop.NewRuntime(sc.Sys)
//	sc.AssignWitness(&rt)
//	// Then drives round 0 → 1 → 2 (see driveProtocol in lookup tests).
type LookupScenario struct {
	// Name identifies the scenario in test output.
	Name string
	// Sys is the pre-compilation System; each factory call returns an
	// independent Sys.
	Sys *wiop.System
	// AssignWitness assigns the user-witness columns in round 0. The lookup
	// compiler's M-assignment prover action runs in the same round and
	// computes M from these columns.
	AssignWitness func(rt *wiop.Runtime)
}

// LookupScenarios returns factory functions for honest-witness lookup
// scenarios accepted by the lookuptologderivsum → logderivativesum
// pipeline.
func LookupScenarios() []func() *LookupScenario {
	return []func() *LookupScenario{
		NewLookupSingleColumnNoFiltersScenario,
		NewLookupFilterOnIncludedScenario,
		NewLookupFilterOnIncludingScenario,
		NewLookupDoubleConditionalScenario,
		NewLookupMultiColumnScenario,
		NewLookupSharedTableScenario,
		NewLookupDistinctTablesScenario,
		NewLookupMultiColumnFilterOnIncludingScenario,
		NewLookupRepeatedValueInTableScenario,
		// Additional shift / width / fragment-count coverage.
		NewLookupShiftedAColumnScenario,
		NewLookupShiftedBColumnScenario,
		NewLookupMultipleAFragmentsScenario,
		NewLookupWidthThreeScenario,
		NewLookupSizeOneScenario,
		NewLookupPrecomputedTableScenario,
		NewLookupRepeatedSValuesScenario,
		NewLookupEmptySelectedScenario,
	}
}

// LookupSoundnessScenario describes a prover-time failure of the
// lookup-to-logderivative pipeline: the witness column assignments cause
// the multiplicity-assignment prover action to panic (typically because an
// active A row has no matching B row, or because a non-binary filter is
// supplied on the A side).
//
// Tests assert that the panic is reached when round 0's prover actions are
// invoked. The wioptest scenarios do NOT cover the alternative
// "Z-tampering" soundness path (manipulating M after compile); those
// scenarios are kept as direct tests in the lookuptologderivsum package
// because they depend on the compiler's internal column ordering.
type LookupSoundnessScenario struct {
	// Name identifies the scenario in test output.
	Name string
	// Sys is the pre-compilation System; each factory call returns an
	// independent Sys.
	Sys *wiop.System
	// AssignWitness assigns columns that violate the lookup; the
	// M-assignment prover action must panic when run.
	AssignWitness func(rt *wiop.Runtime)
}

// LookupSoundnessScenarios returns factory functions for soundness lookup
// scenarios where the M-assignment prover task is expected to panic.
func LookupSoundnessScenarios() []func() *LookupSoundnessScenario {
	return []func() *LookupSoundnessScenario{
		NewLookupSingleColumnUnmatchedScenario,
		NewLookupMultiColumnPartialMatchScenario,
		NewLookupMaskedRowMatchScenario,
		NewLookupNonBinaryFilterScenario,
	}
}
