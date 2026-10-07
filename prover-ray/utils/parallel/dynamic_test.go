package parallel_test

import (
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/parallel"
)

func TestExecuteDynamic_RunsEveryItemOnce(t *testing.T) {
	for _, tc := range []struct{ items, workers int }{{0, 4}, {1, 4}, {3, 8}, {1000, 7}, {1000, 0}} {
		hits := make([]atomic.Int32, tc.items)
		parallel.ExecuteDynamic(tc.items, func(i int) { hits[i].Add(1) }, tc.workers)
		for i := range hits {
			assert.EqualValuesf(t, 1, hits[i].Load(), "items=%d workers=%d item %d", tc.items, tc.workers, i)
		}
	}
}

func TestExecuteDynamic_RepanicsInCaller(t *testing.T) {
	assert.Panics(t, func() {
		parallel.ExecuteDynamic(100, func(i int) {
			if i == 42 {
				panic("boom")
			}
		}, 4)
	})
}
