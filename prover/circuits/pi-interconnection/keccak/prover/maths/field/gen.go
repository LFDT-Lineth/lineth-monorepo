package field

import (
	"runtime"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/parallel"
)

// ParBatchInvert is as a parallel implementation of [BatchInvert]. The caller
// can supply the target number of cores to use to perform the paralellization.
// If `numCPU=0` is provided, the function defaults to using all the available
// cores exposed by the OS.
func ParBatchInvert(a []Element, numCPU int) []Element {

	if numCPU == 0 {
		numCPU = runtime.NumCPU()
	}

	res := make([]Element, len(a))

	parallel.Execute(len(a), func(start, stop int) {
		subRes := BatchInvert(a[start:stop])
		copy(res[start:stop], subRes)
	}, numCPU)

	return res
}
