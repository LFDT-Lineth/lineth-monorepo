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

func TestExecuteLargestFirst_SharesCPUsByCost(t *testing.T) {
	cpus := runtime.GOMAXPROCS(0)
	costs := []int{0, 1, 3 * cpus, 1000 * cpus}
	workers := make([]int, len(costs))
	parallel.ExecuteLargestFirst(costs, func(i, w int) { workers[i] = w })
	total := 0
	for _, c := range costs {
		total += c
	}
	for i, c := range costs {
		assert.Equalf(t, max(1, cpus*c/total), workers[i], "item %d", i)
	}
}
