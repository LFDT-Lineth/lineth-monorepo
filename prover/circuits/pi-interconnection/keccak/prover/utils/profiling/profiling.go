package profiling

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/pkg/profile"
	"github.com/sirupsen/logrus"
)

var SKIP_PROFILING = true

// ProfileTrace run the benchmark function with optionally, benchmarking and tracing
// The path should neither start nor end with a "/".
func ProfileTrace(name string, profiled, traced bool, fn func()) {

	if SKIP_PROFILING {
		logrus.Warnf("Skipping the profiling globally")
		fn()
		return
	}

	var pprof interface{ Stop() }

	/*
		Some validation on the inputs
	*/
	if strings.HasPrefix(name, "/") {
		utils.Panic("Forbidden, name starts with /")
	}

	if strings.HasPrefix(name, "./") {
		utils.Panic("Forbidden, name starts with ./")
	}

	if strings.HasSuffix(name, "/") {
		utils.Panic("Forbidden, name starts with /")
	}

	dir := fmt.Sprintf("profiling/%v", name)

	// Attempt to create the directory
	err := os.MkdirAll(dir, 0775)
	if err != nil {
		panic(err)
	}

	if traced {
		ctx, cancel := context.WithCancel(
			context.Background(),
		)
		Trace(ctx, dir)
		defer cancel()
	}

	if profiled {
		pprof = profile.Start(
			profile.ProfilePath(dir),
			profile.Quiet,
			// profile.MemProfile,
		)
		defer pprof.Stop()
	}

	fn()
}
