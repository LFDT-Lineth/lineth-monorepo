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

// SetMaxRevealLenForTest overrides [maxRevealLen], the largest column length the
// PCS reveals in the clear rather than committing via FRI. Setting it to 0
// disables revealing so every committable column takes the FRI path (the
// behaviour before small-column reveal existed). Intended solely for tests;
// production code must never call it.
func SetMaxRevealLenForTest(n int) {
	maxRevealLen = n
}
