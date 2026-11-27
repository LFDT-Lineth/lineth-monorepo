package parallel

import (
	"runtime"
	"runtime/debug"
	"sync"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
)

// Execute runs work over [0, nbIterations) split into GOMAXPROCS contiguous
// ranges, one goroutine each. See [ExecuteWithNbTasks].
func Execute(nbIterations int, work func(start, stop int)) {
	ExecuteWithNbTasks(nbIterations, runtime.GOMAXPROCS(0), work)
}

// ExecuteWithNbTasks runs work over [0, nbIterations) split into nbTasks
// contiguous ranges of near-equal size, one goroutine each (at least one
// task, and no more than nbIterations). With one task, work runs in the
// calling goroutine. A panic in work is recovered and re-raised in the
// calling goroutine, with the stack of the goroutine that panicked.
func ExecuteWithNbTasks(nbIterations, nbTasks int, work func(start, stop int)) {
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
			// runtime.LockOSThread()
			// defer runtime.UnlockOSThread()
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
