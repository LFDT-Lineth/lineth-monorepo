package wioptest

import "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"

// VanishingScenario is a fixture for testing the global-quotient compiler.
//
// Sys holds the pre-compilation system; AssignHonest / AssignInvalid assign
// only the oracle columns from the initial round. The compiler's prover and
// verifier actions are registered on the rounds that Compile adds.
type VanishingScenario struct {
	// Name identifies the scenario in test output.
	Name string
	// Sys is the pre-compilation System. Call compilers.Compile(sc.Sys) before
	// creating a Runtime; each factory call returns an independent Sys.
	Sys *wiop.System
	// AssignHonest assigns oracle columns with a witness that satisfies all
	// vanishing constraints. After Compile and RunAndVerify, must return nil.
	AssignHonest func(rt *wiop.Runtime)
	// AssignInvalid assigns oracle columns that violate at least one vanishing
	// constraint. After Compile and RunAndVerify, must return an error.
	AssignInvalid func(rt *wiop.Runtime)
}

// VanishingScenarios returns factory functions for the built-in vanishing
// scenarios. The returned Sys is always pre-compilation; call
// compilers.Compile(sc.Sys) in the test before creating a Runtime.
func VanishingScenarios() []func() *VanishingScenario {
	return []func() *VanishingScenario{
		NewBooleanColumnVanishingScenario,
		NewFibonacciVanishingScenario,
		NewGeometricProgressionVanishingScenario,
		NewConditionalCounterVanishingScenario,
		NewPythagoreanTripletVanishingScenario,
		NewDynamicFibonacciVanishingScenario,
		// Additional scenarios that broaden compiler coverage.
		NewConstantColumnVanishingScenario,
		NewForwardShiftVanishingScenario,
		NewBooleanCubeVanishingScenario,
		NewLinearCombinationVanishingScenario,
		NewLargeFibonacciVanishingScenario,
		NewMultipleVanishingsSameRatioScenario,
		NewMixedRatioVanishingsScenario,
		NewMultiModuleVanishingScenario,
		NewManualCancellationVanishingScenario,
		NewPrecomputedSelectorVanishingScenario,
		NewCellLeafVanishingScenario,
		NewCoinScaledVanishingScenario,
		NewThreeStepRecurrenceVanishingScenario,
		NewQuarticVanishingScenario,
		NewLeftPadDynamicVanishingScenario,
		// Ratio > 1 corner cases (ratios stay within {2, 4}, the realistic
		// range produced by [computeRatio] for DegreeFactor up to 4).
		NewCubicWithBackShiftVanishingScenario,
		NewMixedHighRatioVanishingsScenario,
		NewMultiModuleHighRatioVanishingScenario,
		NewSizeThirtyTwoCubicVanishingScenario,
		NewLargeForwardShiftVanishingScenario,
		NewBackAndForwardShiftVanishingScenario,
		NewDynamicQuadraticVanishingScenario,
		NewQuarticWithBackShiftVanishingScenario,
	}
}

// RunProver drives rt through every interactive round, running each round's
// registered prover actions before advancing to the next. It leaves rt holding
// a complete honest transcript: every committed column, cell, and coin is
// populated. It does not run any verifier action.
//
// The caller must assign all r0 oracle columns before calling RunProver; any
// prover action registered on r0 (e.g. the multiplicity-column assignment that
// lookuptologderivsum installs) is then run here.
func RunProver(rt *wiop.Runtime) {
	sys := rt.System
	// Run any prover actions on the current (first) round before any
	// AdvanceRound. The lookup-to-log-derivative compiler installs its
	// multiplicity-assignment task on the group's witness round, which is
	// the runtime's starting round.
	for _, a := range rt.CurrentRound().ProverActions {
		a.Run(*rt)
	}
	for rt.CurrentRound().ID < len(sys.Rounds)-1 {
		rt.AdvanceRound()
		for _, a := range rt.CurrentRound().ProverActions {
			a.Run(*rt)
		}
	}
}

// RunVerifier runs every verifier action across all rounds against a fully
// populated rt and returns the first error encountered, or nil. Separating it
// from [RunProver] lets a test tamper with a committed value between proving
// and verifying — the basis of transcript-level mutation testing.
func RunVerifier(rt wiop.Runtime) error {
	for _, r := range rt.System.Rounds {
		for _, va := range r.VerifierActions {
			if err := va.Check(rt); err != nil {
				return err
			}
		}
	}
	return nil
}

// RunAndVerify runs the honest prover to completion and then the verifier,
// returning the first verification error or nil.
func RunAndVerify(rt *wiop.Runtime) error {
	RunProver(rt)
	return RunVerifier(*rt)
}
