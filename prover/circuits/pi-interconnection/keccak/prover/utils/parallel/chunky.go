package parallel

import (
	"runtime"
	"sync"

	"github.com/sirupsen/logrus"
)

// Execute in parallel. This implementation is optimized for the case where
// the workload is a sequence of unbalanced and long computation time jobs.
func ExecuteChunky(nbIterations int, work func(start, stop int), numcpus ...int) {

	numcpu := runtime.GOMAXPROCS(0)
	if len(numcpus) > 0 && numcpus[0] > 0 {
		numcpu = numcpus[0]
	}

	// Then just call the trivial function
	if nbIterations < numcpu {
		logrus.Debugf("Loss in parallelization time numpcpu = %v and nbIterator = %v", numcpu, nbIterations)
		Execute(nbIterations, work, numcpu)
		return
	}

	// The wait group ensures that all the children goroutine have terminated
	// before we close the
	wg := sync.WaitGroup{}
	wg.Add(nbIterations)

	taskCounter := NewAtomicCounter(nbIterations)

	// Each goroutine consumes the jobChan to
	for p := 0; p < numcpu; p++ {
		go func() {
			for {
				taskID, ok := taskCounter.Next()
				if !ok {
					break
				}

				work(taskID, taskID+1)
				wg.Done()
			}
		}()
	}

	wg.Wait()
}
