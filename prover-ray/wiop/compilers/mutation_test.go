package compilers_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover-ray/wiop"
	"github.com/consensys/linea-monorepo/prover-ray/wiop/wioptest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// assertSweepCatches compiles the full pipeline on sys, then sweeps every
// committed transcript position and asserts the verifier rejects at least one
// corruption.
//
// The sweep tampers a committed value rather than a fresh input: prepare runs
// the honest prover to completion, so every dependent value (quotient shares,
// Z columns, opening cells, the LogDerivativeSum Result, ...) is committed from
// the honest trace before a single position is flipped. The verifier-only step
// then runs against that almost-honest transcript. This catches values that an
// "input then re-prove" sweep would miss — most notably a column whose claimed
// aggregate (a cell) was already pinned: flipping the column now contradicts
// the committed cell instead of being silently re-derived.
func assertSweepCatches(t *testing.T, sys *wiop.System, assign func(*wiop.Runtime)) {
	t.Helper()
	compileFullPipeline(sys)

	prepare := func(rt *wiop.Runtime) {
		assign(rt)
		wioptest.RunProver(rt)
	}
	verify := func(rt wiop.Runtime) error { return wioptest.RunVerifier(rt) }

	report := wioptest.SweepMutations(sys, prepare, verify, nil, 0)
	require.NotEmpty(t, report,
		"scenario must expose at least one committed transcript position")
	assert.True(t, report.AnyCaught(),
		"a single-value mutation of the committed transcript must be rejected by the verifier")
}

// TestMutationSoundness_VanishingScenarios is the basis of the mutation-driven
// soundness tester: it flips each committed transcript value one at a time and
// asserts the verifier rejects at least one corrupted run.
func TestMutationSoundness_VanishingScenarios(t *testing.T) {
	for _, build := range wioptest.VanishingScenarios() {
		sc := build()
		t.Run(sc.Name, func(t *testing.T) {
			assertSweepCatches(t, sc.Sys, sc.AssignHonest)
		})
	}
}

// TestMutationSoundness_LocalVanishingScenarios sweeps the scalar/local
// vanishing fixtures, whose constraints are lifted to global ones before the
// quotient argument discharges them.
func TestMutationSoundness_LocalVanishingScenarios(t *testing.T) {
	for _, build := range wioptest.LocalVanishingScenarios() {
		sc := build()
		t.Run(sc.Name, func(t *testing.T) {
			assertSweepCatches(t, sc.Sys, sc.AssignHonest)
		})
	}
}

// TestMutationSoundness_LogDerivativeSumScenarios sweeps the logderivativesum
// fixtures. Because prepare commits the claimed Result before tampering, both a
// flipped Result cell and a flipped witness column (which then contradicts the
// committed Result and Z recurrence) are caught.
func TestMutationSoundness_LogDerivativeSumScenarios(t *testing.T) {
	for _, build := range wioptest.LogDerivativeSumCompilerScenarios() {
		sc := build()
		t.Run(sc.Name, func(t *testing.T) {
			assertSweepCatches(t, sc.Sys, sc.AssignWitness)
		})
	}
}

// TestMutationSoundness_LookupScenarios sweeps the lookup fixtures: a flipped
// witness entry contradicts the committed multiplicity column and Z recurrence.
func TestMutationSoundness_LookupScenarios(t *testing.T) {
	for _, build := range wioptest.LookupScenarios() {
		sc := build()
		t.Run(sc.Name, func(t *testing.T) {
			assertSweepCatches(t, sc.Sys, sc.AssignWitness)
		})
	}
}

// TestMutationSoundness_RangeCheckScenarios sweeps the range-check fixtures,
// which reduce to a lookup against a precomputed range column.
func TestMutationSoundness_RangeCheckScenarios(t *testing.T) {
	for _, build := range wioptest.RangeCheckCompilerScenarios() {
		sc := build()
		t.Run(sc.Name, func(t *testing.T) {
			assertSweepCatches(t, sc.Sys, sc.AssignWitness)
		})
	}
}
