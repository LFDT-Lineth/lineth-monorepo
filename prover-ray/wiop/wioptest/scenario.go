// Package wioptest provides reusable test scenarios for the wiop protocol.
//
// Each scenario bundles a System, the query under test, and two runtime
// population callbacks:
//   - RunHonest: builds a valid witness; Query.Check must pass.
//   - RunInvalid: builds an invalid witness; Query.Check must return an error.
//
// VanishingScenarios returns analogous fixtures for compiler integration tests.
// Because each factory function closes over its own freshly-allocated System,
// calling the factory twice yields two completely independent scenarios.
package wioptest

import "github.com/consensys/linea-monorepo/prover-ray/wiop"

// Scenario is a self-contained test fixture for one query type.
//
// Call the factory function to obtain an instance; each call returns a new
// Scenario backed by its own System so tests never share mutable state.
type Scenario struct {
	// Name identifies the scenario in test output.
	Name string
	// Sys is the protocol system. Create fresh Runtimes from it; never mutate
	// it directly (e.g. do not compile it — use VanishingScenario for that).
	Sys *wiop.System
	// Query is the registered query whose Check method is under test.
	Query wiop.Query
	// RunHonest populates rt with a fully valid witness: oracle column
	// assignments, round advancement, and SelfAssign where applicable.
	// After this returns, Query.Check(rt) must return nil.
	RunHonest func(rt *wiop.Runtime)
	// RunInvalid populates rt with an invalid witness that Query.Check must
	// reject. For claim-based queries the column assignment is honest but the
	// result cell holds a wrong value; for relation queries the column
	// assignments directly violate the predicate.
	RunInvalid func(rt *wiop.Runtime)
}

// All returns factory functions for every built-in query scenario.
// Call each factory once per test to obtain an independent Scenario.
func All() []func() *Scenario {
	return []func() *Scenario{
		NewLocalOpeningScenario,
		NewLagrangeEvalScenario,
		NewLogDerivativeSumScenario,
		NewPermutationScenario,
		NewInclusionScenario,
		NewRangeCheckScenario,
	}
}
