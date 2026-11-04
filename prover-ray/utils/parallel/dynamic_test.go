package parallel_test

import (
	"runtime"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/parallel"
)

func TestExecuteDynamic_RunsEveryItemOnce(t *testing.T) {
	for _, items := range []int{0, 1, 3, 1000} {
		hits := make([]atomic.Int32, items)
		parallel.ExecuteDynamic(items, func(i int) { hits[i].Add(1) })
		for i := range hits {
			assert.EqualValuesf(t, 1, hits[i].Load(), "items=%d item %d", items, i)
		}
	}
}

func TestExecuteDynamic_RepanicsInCaller(t *testing.T) {
	assert.Panics(t, func() {
		parallel.ExecuteDynamic(100, func(i int) {
			if i == 42 {
				panic("boom")
			}
		})
	})
}

func TestOrderDesc(t *testing.T) {
	assert.Equal(t, []int{1, 3, 0, 2}, parallel.OrderDesc([]int{2, 5, 1, 5}))
	assert.Empty(t, parallel.OrderDesc(nil))
}

// The CPU shares of ExecuteLargestFirst: every item runs once, with at least
// one worker; the shares add up to at most GOMAXPROCS plus one per item (the
// rounding up of the small ones); and a costlier item never gets fewer
// workers than a cheaper one.
func TestExecuteLargestFirst_SharesCPUsByCost(t *testing.T) {
	cpus := runtime.GOMAXPROCS(0)
	for _, costs := range [][]int{
		nil,
		{0, 0, 0},
		{0, 1, 3 * cpus, 1000 * cpus},
		{5, 5, 5, 5, 5, 5, 5, 5},
		{1 << 22, 1 << 10, 7, 1 << 20, 1 << 22, 3, 1 << 16},
	} {
		workers := make([]atomic.Int64, len(costs))
		calls := make([]atomic.Int32, len(costs))
		parallel.ExecuteLargestFirst(costs, func(i, w int) {
			calls[i].Add(1)
			workers[i].Store(int64(w))
		})
		sum := 0
		for i := range costs {
			assert.EqualValuesf(t, 1, calls[i].Load(), "costs=%v item %d", costs, i)
			w := int(workers[i].Load())
			assert.GreaterOrEqualf(t, w, 1, "costs=%v item %d", costs, i)
			sum += w
			for j := range costs {
				if costs[i] < costs[j] {
					assert.LessOrEqualf(t, w, int(workers[j].Load()), "costs=%v items %d < %d", costs, i, j)
				}
			}
		}
		assert.LessOrEqualf(t, sum, cpus+len(costs), "costs=%v", costs)
	}
}
