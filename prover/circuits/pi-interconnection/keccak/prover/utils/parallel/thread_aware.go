package parallel

import (
	"runtime"
	"sync"
)

type ThreadInit func(threadID int)
type Worker func(taskID, threadID int)

func ExecuteThreadAware(nbIterations int, init ThreadInit, worker Worker, numcpus ...int) {

	numcpu := runtime.GOMAXPROCS(0)
	if len(numcpus) > 0 && numcpus[0] > 0 {
		numcpu = numcpus[0]
	}

	// The wait group ensures that all the children goroutine have terminated
	// before we close the
	wg := sync.WaitGroup{}
	wg.Add(nbIterations)

	taskCounter := NewAtomicCounter(nbIterations)

	// Each goroutine consumes the jobChan to
	for p := 0; p < numcpu; p++ {
		threadID := p
		go func() {
			init(threadID)
			for {
				taskID, ok := taskCounter.Next()
				if !ok {
					break
				}

				worker(taskID, threadID)
				wg.Done()
			}
		}()
	}

	wg.Wait()
}
