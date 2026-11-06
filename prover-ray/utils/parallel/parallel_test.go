package parallel_test

import (
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/parallel"
)

// covers runs exec over n iterations and checks that every iteration is
// visited exactly once.
func covers(t *testing.T, n int, exec func(n int, work func(start, end int))) {
	t.Helper()
	hits := make([]atomic.Int32, n)
	exec(n, func(start, end int) {
		for i := start; i < end; i++ {
			hits[i].Add(1)
		}
	})
	for i := range hits {
		require.EqualValuesf(t, 1, hits[i].Load(), "n=%d iteration %d", n, i)
	}
}

func TestExecuteWithNbTasks_CoversEveryIterationOnce(t *testing.T) {
	for _, n := range []int{0, 1, 7, 1000} {
		for _, tasks := range []int{-1, 0, 1, 3, 64, 2000} {
			covers(t, n, func(n int, work func(int, int)) { parallel.ExecuteWithNbTasks(n, tasks, work) })
		}
	}
}

func TestExecuteAligned_RangesAreAligned(t *testing.T) {
	for _, n := range []int{0, 1, 15, 16, 17, 1000, 4096} {
		covers(t, n, func(n int, work func(int, int)) { parallel.ExecuteAligned(n, 16, work) })
		parallel.ExecuteAligned(n, 16, func(start, end int) {
			assert.Zerof(t, start%16, "n=%d: range starts at %d", n, start)
			if end != n {
				assert.Zerof(t, end%16, "n=%d: range [%d, %d) is not the last one", n, start, end)
			}
		})
	}
}

func TestExecute_ReraisesPanics(t *testing.T) {
	require.Panics(t, func() {
		parallel.ExecuteWithNbTasks(100, 4, func(start, _ int) {
			if start == 0 {
				panic("boom")
			}
		})
	})
}
