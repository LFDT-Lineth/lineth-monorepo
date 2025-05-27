package controller

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/consensys/linea-monorepo/prover/cmd/controller/controller/metrics"
	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/sirupsen/logrus"
)

// function to run the controller
func runController(ctx context.Context, cfg *config.Config) {
	var (
		cLog          = cfg.Logger().WithField("component", "main-loop")
		fsWatcher     = NewFsWatcher(cfg)
		executor      = NewExecutor(cfg)
		numRetrySoFar int
	)

	// Start the metric server
	if cfg.Controller.Prometheus.Enabled {
		metrics.StartServer(
			cfg.Controller.LocalID,
			cfg.Controller.Prometheus.Route,
			cfg.Controller.Prometheus.Port,
		)
	}

	ctx, stop := signal.NotifyContext(ctx, syscall.SIGTERM)
	defer stop()

	// cmdContext is the context we provide for the command execution. In
	// spot-instance mode, the context is subordinated to the ctx.
	cmdContext := context.Background()
	if cfg.Controller.SpotInstanceMode {
		cmdContext = ctx
	}

	go func() {
		// This goroutine's raison d'etre is to log a message immediately when a
		// cancellation request (e.g., ctx expiration/cancellation, SIGTERM, etc.)
		// is received. It ensures timely logging of the request's reception,
		// which may be important for diagnostics. Without this
		// goroutine, if the prover is busy with a proof when, for example, a
		// SIGTERM is received, there would be no log entry about the signal
		// until the proof completes.
		<-ctx.Done()

		if cfg.Controller.SpotInstanceMode {
			cLog.Infoln("Received cancellation request. Killing the ongoing process and exiting immediately after.")
		} else {
			cLog.Infoln("Received cancellation request, will exit as soon as possible or once current proof task is complete.")
		}
	}()

	for {
		select {
		case <-ctx.Done():
			// Graceful shutdown.
			// This case captures both cancellations initiated by the caller
			// through ctx and SIGTERM signals. Even if the cancellation
			// request is first intercepted by the goroutine at line 34, Go
			// allows the ctx.Done channel to be read multiple times, which, in
			// our scenario, ensures cancellation requests are effectively
			// detected and handled.
			cLog.Infoln("Context canceled by caller or SIGTERM. Exiting")
			metrics.ShutdownServer(ctx)
			return

		// Processing a new job
		case <-retryDelay(cfg.Controller.RetryDelays, numRetrySoFar):
			// Fetch the best block we can fetch
			job := fsWatcher.GetBest()

			// No jobs, waiting a little before we retry
			if job == nil {
				numRetrySoFar++
				noJobFoundMsg := "found no jobs in the queue"
				if numRetrySoFar > 5 {
					cLog.Debug(noJobFoundMsg)
				} else {
					cLog.Info(noJobFoundMsg)
				}
				continue
			}

			// Else, reset the retry counter
			numRetrySoFar = 0

			// Run the command (potentially retrying in large mode)
			status := executor.Run(cmdContext, job)

			// createColumns the job according to the status we got
			switch {

			// Success
			case status.ExitCode == CodeSuccess:
				// NB: we already check that the response filename can be
				// generated prior to running the command. So this actually
				// will not panic.
				respFile, err := job.ResponseFile()
				tmpRespFile := job.TmpResponseFile(cfg)
				if err != nil {
					formatStr := "Could not generate the response file: %v (original request file: %v)"
					utils.Panic(formatStr, err, job.OriginalFile)
				}

				logrus.Infof(
					"Moving the response file from the tmp response file `%v`, to the final response file: `%v`",
					tmpRespFile, respFile,
				)

				if err := os.Rename(tmpRespFile, respFile); err != nil {
					// @Alex: it is unclear how the rename operation could fail
					// here. If this happens, we prefer removing the tmp file.
					// Note that the operation is an `rm -f`.
					os.Remove(tmpRespFile)

					cLog.Errorf(
						"Error renaming %v to %v: %v, removed the tmp file",
						tmpRespFile, respFile, err,
					)
				}

				// Move the inprogress to the done directory
				cLog.Infof(
					"Moving %v to %v with the success prefix",
					job.OriginalFile, job.Def.dirDone(),
				)

				jobDone := job.DoneFile(status)
				if err := os.Rename(job.InProgressPath(), jobDone); err != nil {
					// When that happens, the only thing left to do is to log
					// the error and let the inprogress file where it is. It
					// will likely require a human intervention.
					//
					// Note: this is assumedly an unreachable code path.
					cLog.Errorf(
						"Error renaming %v to %v: %v",
						job.InProgressPath(), jobDone, err,
					)
				}

			// Defer to the large prover
			case job.Def.Name == jobNameExecution && isIn(status.ExitCode, cfg.Controller.DeferToOtherLargeCodes):
				cLog.Infof("Renaming %v for the large prover", job.OriginalFile)
				// Move the inprogress file back in the from directory with
				// the new suffix
				toLargePath, err := job.DeferToLargeFile(status)
				if err != nil {
					// There are two possibilities of errors. (1), the status
					// we success but the above cases prevents that. The other
					// case is that the suffix was not provided. But, during
					// the config validation, we check already that the suffix
					// must be provided if the size of the list of
					// deferToOtherLargeCodes is non-zero. If the size of the
					// list was zero, then there would be no way to reach this
					// portion of the code given that the current exit code
					// cannot be part of the empty list. Thus, this section is
					// unreachable.
					cLog.Errorf(
						"error deriving the to-large-name of %v: %v",
						job.InProgressPath(), err,
					)
				}

				if err := os.Rename(job.InProgressPath(), toLargePath); err != nil {
					// When that happens, the only thing left to do is to log
					// the error and let the inprogress file where it is. It
					// will likely require a human intervention.
					cLog.Errorf(
						"error renaming %v to %v: %v",
						job.InProgressPath(), toLargePath, err,
					)
				}

			case status.ExitCode == CodeKilledByUs:
				// When receiving the killed-by-us code, the prover will put
				// back the file in the queue. We do not immediately exit the
				// loop as there is already a clause to exit the loop in case
				// of sigterm at the top of the loop. So it will immediately
				// exit as soon as we enter the next iteration.
				cLog.Infof("Job %v was killed by us. Reputting it in the request folder", job.OriginalFile)

				if err := os.Rename(job.InProgressPath(), job.OriginalPath()); err != nil {
					// When that happens, the only thing left to do is to log
					// the error and let the inprogress file where it is. It
					// will likely require a human intervention.
					//
					// Note: this is assumedly an unreachable code path.
					cLog.Errorf(
						"Error renaming %v to %v: %v",
						job.InProgressPath(), job.OriginalPath(), err,
					)
				}

				// As an edge-case, it's possible (in theory) that the process
				// completes exactly when we receive the kill signal. So we
				// could end up in a situation where the tmp-response file
				// exists. In that case, we simply delete it before exiting to
				// keep the FS clean.
				os.Remove(job.TmpResponseFile(cfg))

			// Failure case
			default:
				// Move the inprogress to the done directory
				cLog.Infof(
					"Moving %v with in %v with a failure suffix for code %v",
					job.OriginalFile, job.Def.dirDone(), status.ExitCode,
				)

				jobFailed := job.DoneFile(status)
				if err := os.Rename(job.InProgressPath(), jobFailed); err != nil {
					// When that happens, the only thing left to do is to log
					// the error and let the inprogress file where it is. It
					// will likely require a human intervention.
					//
					// Note: this is assumedly an unreachable code path.
					cLog.Errorf(
						"Error renaming %v to %v: %v",
						job.InProgressPath(), jobFailed, err,
					)
				}
			}
		}
	}
}

// Returns the duration to wait before retrying to find a job in the queue. This
// is to avoid spamming the FS with LS queries.
func retryDelay(retryDelaysSec []int, numRetrySoFar int) <-chan time.Time {

	// Hard coded values
	retryDelaysDur := make([]time.Duration, len(retryDelaysSec))
	for i := range retryDelaysSec {
		retryDelaysDur[i] = time.Duration(retryDelaysSec[i]) * time.Second
	}

	// By default, take the last value
	ttw := retryDelaysDur[len(retryDelaysDur)-1]

	// If it does not overflows, take the last value at `numSoFar`
	if numRetrySoFar < len(retryDelaysDur) {
		ttw = retryDelaysDur[numRetrySoFar]
	}

	return time.After(ttw)
}
