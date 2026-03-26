package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/consensys/linea-monorepo/prover/backend/execution/statemanager"
	"github.com/consensys/linea-monorepo/prover/utils/parallel"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/time/rate"
)

const (
	// Max number of retries we allow ourselves before giving up and returning
	// an error.
	maxRetries = 10
	// Bucket size of the throttler
	throttlerBucketSize = 50
	// Num file processed tick time
	tickTime = 10 * time.Second
)

// Run runs the CLI command
func fetchAndInspect(cmd *cobra.Command, args []string) error {

	runtime.GOMAXPROCS(numThreads)

	shomei := &shomeiClient{
		hostport: url,
		client:   http.DefaultClient,
		throttler: rate.NewLimiter(
			rate.Every(maxRps),
			throttlerBucketSize,
		),
		maxRetries: maxRetries,
	}

	reportFile, err := os.OpenFile("./shomei.report", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)

	if err != nil {
		logrus.Errorf("failed to open the report file: %v", err)
		os.Exit(1)
	}

	// Defer only after we made sure the files was successfully open, otherwise
	// it will panic in the defer.
	defer func() {
		if err := reportFile.Close(); err != nil {
			logrus.Errorf("failed to close report file: %v", err)
		}
	}()

	var (
		numBlocksArg               = stopArg - startArg
		numRangeArgs               = (numBlocksArg + blockRange - 1) / blockRange
		processedRangeCount uint64 = 0
	)

	// The function is exited when the main goroutine terminates
	go func() {
		for {
			<-time.Tick(tickTime)
			processedRangeCount := atomic.LoadUint64(&processedRangeCount)
			if blockRange < 0 || processedRangeCount > uint64(math.MaxInt/blockRange) { // #nosec G115 -- Checked for overflow
				panic("overflow")
			}
			processedBlockCount := blockRange * int(processedRangeCount) // #nosec G115 -- Checked for overflow
			totalBlockToProcess := numRangeArgs * blockRange

			logrus.Infof("processed %v blocks of %v to process", processedBlockCount, totalBlockToProcess)
		}
	}()

	parallel.ExecuteChunky(numRangeArgs, func(rangeID, _ int) {

		var (
			blockOffset         = rangeID * blockRange
			start               = startArg + blockOffset
			stop                = start + blockRange - 1 // NB: shomei takes inclusive ranges
			emptyParentRootHash = types.KoalaOctuplet{}
		)

		shomeiResp, err := shomei.fetchStateTransitionProofs(
			context.Background(),
			&zkEVMStateMerkleProofV0Req{
				StartBlockNumber: start,
				EndBlockNumber:   stop,
				ShomeiVersion:    shomeiVersion,
			},
		)

		if err != nil {
			logrus.Fatalf("Failed to fetch block from shomei: %v", err)
		}

		_, ferr := inspectTrace(emptyParentRootHash, shomeiResp, true)

		if len(ferr) > 0 {
			fmt.Fprintf(reportFile, "\n\n================================================\n\n")
			fmt.Fprintf(reportFile, "errors when inspecting range of bloc %v-%v : %s", start, stop, errors.Join(ferr...).Error())
		}

		atomic.AddUint64(&processedRangeCount, 1)
	}, numThreads)

	return nil

}

// Inspect the traces. Prev root is the root over which we are expecting to check
// the traces. For the first block, setting the flag as true skips the check that
// the previous parent claimed in the state manager response should match the one
// we have. In that case, the prev root input is ignored.
func inspectTrace(
	prevRoot types.KoalaOctuplet,
	serialized []byte,
	ignoreParent bool,
) (newRoot types.KoalaOctuplet, errs []error) {

	// The current code can panic. If it does, then we append the panic message
	// wrapped in an error and return as it was a soft failure (which it is).
	defer func() {
		if p := recover(); p != nil {
			errs = append(errs, fmt.Errorf("got the panic message: %v", p))
		}
	}()

	shomeiOut := &statemanager.ShomeiOutput{}
	if err := json.Unmarshal(serialized, shomeiOut); err != nil {
		// At this point, if we have an error, there is no point moving forward.
		// Because it means we failed to read the alleged traces.
		return types.KoalaOctuplet{}, append(errs, fmt.Errorf("could not unmarshal shomei output: %w", err))
	}

	var (
		traces         = shomeiOut.Result.ZkStateMerkleProof
		parentRootHash = shomeiOut.Result.ZkParentStateRootHash
	)

	// Since we process several traces of shomei in sequence, we need to ensure
	// that the claimed parent root hashes are consistent with what we had seen
	// so far.
	if !ignoreParent && prevRoot.Hex() != parentRootHash.Hex() {
		errs = append(errs, fmt.Errorf("mismatch between the expected parent root hash %s and the parent root hash we got from shomei %s", prevRoot.Hex(), parentRootHash.Hex()))
	}

	if len(traces) == 0 {
		errs = append(errs, errors.New("trace parser returned nil or found no traces"))
		return types.KoalaOctuplet{}, errs
	}

	// We keep track of the oldest root hash
	parent := parentRootHash
	for i := range traces {
		// empty blocks don't have traces. Don't panic if that happens
		if len(traces[i]) == 0 {
			continue
		}

		old, new, err := statemanager.CheckTraces(traces[i])
		if err != nil {
			errs = append(errs, fmt.Errorf("trace inspection found an error : %w", err))
		}
		if parent.Hex() != old.Hex() {
			errs = append(errs, fmt.Errorf("mismatch between the expected parent root hash %s and the parent root hash we got from shomei %s", prevRoot.Hex(), parentRootHash.Hex()))
		}
		parent = new
	}

	// @parent will be containing the new root hash of the sequence
	return parent, errs
}
