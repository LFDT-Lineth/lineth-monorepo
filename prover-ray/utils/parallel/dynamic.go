package parallel

import (
	"runtime"
	"sync/atomic"
)

// ExecuteDynamic calls work(i) once for every i in [0, nbItems), on up to
// GOMAXPROCS workers (or maxWorkers when given) that pull items from a shared
// counter in index order. Unlike [Execute], which hands each worker a fixed
// contiguous range, this balances items of very different costs: callers
// should order items largest first, so that the tail is made of small items.
//
// As with [Execute], a panic in work is re-raised in the calling goroutine
// once every worker has stopped.
func ExecuteDynamic(nbItems int, work func(i int), maxWorkers ...int) {
	workers := runtime.GOMAXPROCS(0)
	if len(maxWorkers) == 1 && maxWorkers[0] > 0 {
		workers = maxWorkers[0]
	}
	workers = max(1, min(workers, nbItems))

	var next atomic.Int64
	Execute(workers, func(start, end int) {
		for range end - start {
			for {
				i := int(next.Add(1) - 1)
				if i >= nbItems {
					break
				}
				work(i)
			}
		}
	}, workers)
}
