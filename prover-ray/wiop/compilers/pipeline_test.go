package compilers_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover-ray/wiop"
	"github.com/consensys/linea-monorepo/prover-ray/wiop/compilers/global"
	"github.com/consensys/linea-monorepo/prover-ray/wiop/compilers/localvanishing"
	"github.com/consensys/linea-monorepo/prover-ray/wiop/compilers/logderivativesum"
	"github.com/consensys/linea-monorepo/prover-ray/wiop/compilers/lookuptologderivsum"
	"github.com/consensys/linea-monorepo/prover-ray/wiop/compilers/rangecheck"
	"github.com/consensys/linea-monorepo/prover-ray/wiop/wioptest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// compileFullPipeline runs every wiop compilation pass in the canonical
// order so that each pass can consume the previous one's output:
//
//  1. rangecheck:           RangeCheck → Inclusion TableRelation
//  2. lookuptologderivsum:  Inclusion → LogDerivativeSum
//  3. logderivativesum:     LogDerivativeSum → recurrence Vanishings + endpoint openings
//  4. localvanishing:       scalar Vanishings → multi-valued Vanishings via the Lagrange lift
//  5. global:               multi-valued Vanishings → quotient shares + LagrangeEval claims
//
// Each pass is a no-op when its input queries are absent, so this ordering
// is safe to apply uniformly to every wioptest scenario regardless of which
// pass the scenario is primarily exercising.
func compileFullPipeline(sys *wiop.System) {
	rangecheck.Compile(sys)
	lookuptologderivsum.Compile(sys)
	logderivativesum.Compile(sys)
	localvanishing.Compile(sys)
	global.Compile(sys)
}

// TestFullPipeline_VanishingScenarios runs the full
// range → lookup → logderivative → local → global pipeline on every
// [wioptest.VanishingScenarios] fixture. These scenarios start with
// multi-valued [wiop.Vanishing] constraints; the local-vanishing pass is a
// no-op and the global pass discharges them through the quotient argument.
func TestFullPipeline_VanishingScenarios(t *testing.T) {
	for _, build := range wioptest.VanishingScenarios() {
		sc := build()
		t.Run(sc.Name, func(t *testing.T) {
			compileFullPipeline(sc.Sys)
			rt := wiop.NewRuntime(sc.Sys)
			sc.AssignHonest(&rt)
			require.NoError(t, wioptest.RunAndVerify(&rt),
				"full pipeline must accept an honest witness")
		})

		// Each soundness case rebuilds a fresh scenario so it doesn't share
		// compilation state with the completeness case above.
		t.Run(sc.Name+"/Soundness", func(t *testing.T) {
			sc := build()
			compileFullPipeline(sc.Sys)
			rt := wiop.NewRuntime(sc.Sys)
			sc.AssignInvalid(&rt)
			assert.Error(t, wioptest.RunAndVerify(&rt),
				"full pipeline must reject an invalid witness")
		})
	}
}

// TestFullPipeline_LocalVanishingScenarios runs the full pipeline on every
// [wioptest.LocalVanishingScenarios] fixture. The local-vanishing pass
// lifts each scalar [wiop.Vanishing] into a multi-valued one; the global
// pass then discharges it.
func TestFullPipeline_LocalVanishingScenarios(t *testing.T) {
	for _, build := range wioptest.LocalVanishingScenarios() {
		sc := build()
		t.Run(sc.Name, func(t *testing.T) {
			compileFullPipeline(sc.Sys)
			rt := wiop.NewRuntime(sc.Sys)
			sc.AssignHonest(&rt)
			require.NoError(t, wioptest.RunAndVerify(&rt),
				"full pipeline must accept an honest witness")
		})

		t.Run(sc.Name+"/Soundness", func(t *testing.T) {
			sc := build()
			compileFullPipeline(sc.Sys)
			rt := wiop.NewRuntime(sc.Sys)
			sc.AssignInvalid(&rt)
			assert.Error(t, wioptest.RunAndVerify(&rt),
				"full pipeline must reject an invalid witness")
		})
	}
}

// TestFullPipeline_LogDerivativeSumScenarios runs the full pipeline on
// every [wioptest.LogDerivativeSumCompilerScenarios] fixture. The
// log-derivative pass emits one recurrence Vanishing per Z column (plus
// LocalOpenings for the endpoints), and the global pass then discharges
// the recurrence.
func TestFullPipeline_LogDerivativeSumScenarios(t *testing.T) {
	for _, build := range wioptest.LogDerivativeSumCompilerScenarios() {
		sc := build()
		t.Run(sc.Name, func(t *testing.T) {
			compileFullPipeline(sc.Sys)
			rt := wiop.NewRuntime(sc.Sys)
			sc.AssignWitness(&rt)
			require.NoError(t, wioptest.RunAndVerify(&rt),
				"full pipeline must accept an honest witness")
		})
	}
}

// TestFullPipeline_LookupScenarios runs the full pipeline on every
// [wioptest.LookupScenarios] fixture. The pipeline reduces each Inclusion
// through the log-derivative + recurrence chain into quotient queries that
// the global pass discharges.
func TestFullPipeline_LookupScenarios(t *testing.T) {
	for _, build := range wioptest.LookupScenarios() {
		sc := build()
		t.Run(sc.Name, func(t *testing.T) {
			compileFullPipeline(sc.Sys)
			rt := wiop.NewRuntime(sc.Sys)
			sc.AssignWitness(&rt)
			require.NoError(t, wioptest.RunAndVerify(&rt),
				"full pipeline must accept an honest witness")
		})
	}
}

// TestFullPipeline_RangeCheckScenarios runs the full pipeline on every
// [wioptest.RangeCheckCompilerScenarios] fixture. Every step contributes:
// rangecheck → lookup → log-derivative → recurrence vanishings → global
// quotient.
func TestFullPipeline_RangeCheckScenarios(t *testing.T) {
	for _, build := range wioptest.RangeCheckCompilerScenarios() {
		sc := build()
		t.Run(sc.Name, func(t *testing.T) {
			compileFullPipeline(sc.Sys)
			rt := wiop.NewRuntime(sc.Sys)
			sc.AssignWitness(&rt)
			require.NoError(t, wioptest.RunAndVerify(&rt),
				"full pipeline must accept an honest witness")
		})
	}
}
