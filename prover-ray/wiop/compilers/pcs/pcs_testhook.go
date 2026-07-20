package pcs

import "sync"

// SetFRINumQueriesForTest overrides [friNumQueries] and resets the process-wide
// static FRI parameters so the next call to [staticFRI] rebuilds them at the
// new query count. It is intended solely for tests that exercise the full
// compilation pipeline with a low query count; production code must never call
// it.
func SetFRINumQueriesForTest(n int) {
	friNumQueries = n
	staticFRIOnce = sync.Once{}
}
