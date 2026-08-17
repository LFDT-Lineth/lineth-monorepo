package parallel

import (
	"runtime"
	"sync"
)

// NbTasksPerJob divides the available CPU budget evenly across nbJobs running
// in parallel: it returns max(1, NumCPU/nbJobs). Use it to set per-job
// parallelism (e.g. fft.WithNbTasks) so that outer × inner goroutines stays
// near NumCPU instead of multiplying.
func NbTasksPerJob(nbJobs int) int {
	if nbJobs <= 1 {
		return runtime.NumCPU()
	}
	n := runtime.NumCPU() / nbJobs
	if n < 1 {
		return 1
	}
	return n
}

// ExecuteWithThreshold runs work in parallel when nbIterations >= threshold,
// otherwise invokes it once in the current goroutine. Saves the scheduler
// round-trip when nbIterations is tiny.
func ExecuteWithThreshold(nbIterations, threshold int, work func(int, int)) {
	if nbIterations <= 0 {
		return
	}
	if nbIterations < threshold {
		work(0, nbIterations)
		return
	}
	Execute(nbIterations, work)
}

func Execute(nbIterations int, work func(int, int), maxCpus ...int) {

	nbTasks := runtime.NumCPU()
	if len(maxCpus) == 1 {
		nbTasks = maxCpus[0]
		if nbTasks < 1 {
			nbTasks = 1
		} else if nbTasks > 512 {
			nbTasks = 512
		}
	}

	if nbTasks == 1 {
		// no go routines
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

	for i := 0; i < nbTasks; i++ {
		_start := i*nbIterationsPerCpus + extraTasksOffset
		_end := _start + nbIterationsPerCpus
		if extraTasks > 0 {
			_end++
			extraTasks--
			extraTasksOffset++
		}
		wg.Go(func() {
			work(_start, _end)
		})
	}

	wg.Wait()
}
