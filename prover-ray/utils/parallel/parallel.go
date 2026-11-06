package parallel

import (
	"runtime"
	"runtime/debug"
	"sync"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
)

// Execute calls work on contiguous ranges [start, end) that cover
// [0, nbIterations), on GOMAXPROCS goroutines. A panic in work is re-raised in
// the calling goroutine once every goroutine has stopped.
func Execute(nbIterations int, work func(start, end int)) {
	ExecuteWithNbTasks(nbIterations, runtime.GOMAXPROCS(0), work)
}

// ExecuteWithNbTasks is [Execute] on nbTasks goroutines (at least one, and at
// most one per iteration).
func ExecuteWithNbTasks(nbIterations, nbTasks int, work func(start, end int)) {
	nbTasks = max(1, nbTasks)

	if nbTasks == 1 {
		work(0, nbIterations)
		return
	}

	nbIterationsPerCpus := nbIterations / nbTasks

	// more CPUs than tasks: a CPU will work on exactly one iteration
	if nbIterationsPerCpus < 1 {
		nbIterationsPerCpus = 1
		nbTasks = nbIterations
	}

	var wg sync.WaitGroup

	extraTasks := nbIterations - (nbTasks * nbIterationsPerCpus)
	extraTasksOffset := 0

	var (
		panicTrace []byte
		panicMsg   any
		panicOnce  = &sync.Once{}
	)

	for i := 0; i < nbTasks; i++ {
		wg.Add(1)
		_start := i*nbIterationsPerCpus + extraTasksOffset
		_end := _start + nbIterationsPerCpus
		if extraTasks > 0 {
			_end++
			extraTasks--
			extraTasksOffset++
		}

		go func() {
			// In case the subtask panics, we recover so that we can repanic in
			// the main goroutine. Simplifying the process of tracing back the
			// error and allowing to test the panics.
			defer func() {
				if r := recover(); r != nil {
					panicOnce.Do(func() {
						panicMsg = r
						panicTrace = debug.Stack()
					})
				}

				wg.Done()
			}()

			work(_start, _end)
		}()
	}

	wg.Wait()

	if len(panicTrace) > 0 {
		utils.Panic("Had a panic: %v\nStack: %v\n", panicMsg, string(panicTrace))
	}
}

// ExecuteAligned is [Execute] with every range starting at a multiple of
// align, and every range but the last a whole number of align units: the
// tail of nbIterations goes to the last range. Vector kernels then run their
// unrolled loop on every range, with at most one scalar remainder.
func ExecuteAligned(nbIterations, align int, work func(start, end int)) {
	align = max(1, align)
	units := (nbIterations + align - 1) / align
	Execute(units, func(start, end int) {
		work(start*align, min(end*align, nbIterations))
	})
}
