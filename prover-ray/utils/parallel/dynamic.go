package parallel

import (
	"runtime"
	"sort"
	"sync/atomic"
)

// ExecuteDynamic calls work(i) once for every i in [0, nbItems), on up to
// GOMAXPROCS workers that pull items from a shared counter in index order.
// Unlike [Execute], which hands each worker a fixed contiguous range, this
// balances items of very different costs: callers should order items largest
// first, so that the tail is made of small items.
//
// As with [Execute], a panic in work is re-raised in the calling goroutine
// once every worker has stopped.
func ExecuteDynamic(nbItems int, work func(i int)) {
	workers := max(1, min(runtime.GOMAXPROCS(0), nbItems))
	var next atomic.Int64
	Execute(workers, func(_, _ int) {
		for i := int(next.Add(1) - 1); i < nbItems; i = int(next.Add(1) - 1) {
			work(i)
		}
	}, workers)
}

// OrderDesc returns the indices of costs sorted by decreasing cost, ties in
// index order.
func OrderDesc(costs []int) []int {
	order := make([]int, len(costs))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return costs[order[a]] > costs[order[b]] })
	return order
}

// ExecuteLargestFirst calls work(i, workers) once for every item i of costs,
// through [ExecuteDynamic] in [OrderDesc] order. workers is the item's share
// of the GOMAXPROCS CPUs, proportional to its cost and at least one, for the
// item's own inner parallelism: the shares add up to about GOMAXPROCS, so the
// largest items are themselves parallel without nesting a full fan-out per
// item.
func ExecuteLargestFirst(costs []int, work func(i, workers int)) {
	order := OrderDesc(costs)
	total := 0
	for _, c := range costs {
		total += c
	}
	cpus := runtime.GOMAXPROCS(0)
	ExecuteDynamic(len(order), func(k int) {
		i := order[k]
		workers := 1
		if total > 0 {
			workers = max(1, cpus*costs[i]/total)
		}
		work(i, workers)
	})
}
