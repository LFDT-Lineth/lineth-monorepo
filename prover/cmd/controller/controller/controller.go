package controller

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/consensys/linea-monorepo/prover/backend/execution"
	"github.com/consensys/linea-monorepo/prover/cmd/controller/controller/metrics"
	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/sirupsen/logrus"
)

func runController(ctx context.Context, cfg *config.Config) {
	var (
		cLog      = cfg.Logger().WithField("component", "main-loop")
		fsWatcher = NewFsWatcher(cfg)
		executor  = NewExecutor(cfg)

		// Track currently active job for safe requeue
		activeJob      *Job
		activeJobMutex = &sync.Mutex{}

		// Track the number of retries so far
		numRetrySoFar int

		// Atomic coordination flags
		spotReclaimDetected       atomic.Bool
		gracefulShutdownRequested atomic.Bool

		// Channel to receive signals
		signalChan = make(chan os.Signal, 2)

		// Channel to notify job completion for SIGTERM handler
		jobDoneChan   chan struct{}
		jobDoneChanMu sync.Mutex
	)

	// Helper to signal job completion
	notifyJobDone := func() {
		jobDoneChanMu.Lock()
		if jobDoneChan != nil {
			close(jobDoneChan)
			jobDoneChan = nil
		}
		jobDoneChanMu.Unlock()
	}

	// Helper to requeue a job
	requeueJob := func(job *Job) {
		_ = os.Remove(job.TmpResponseFile(cfg))
		if err := os.Rename(job.InProgressPath(), job.OriginalPath()); err != nil {
			cLog.Errorf("Failed to requeue job %v: %v", job.InProgressPath(), err)
		}
		cLog.Infof("REQUEUED job: %v", job.OriginalFile)
	}

	// Start the metric server
	if cfg.Controller.Prometheus.Enabled {
		metrics.StartServer(
			cfg.Controller.LocalID,
			cfg.Controller.Prometheus.Route,
			cfg.Controller.Prometheus.Port,
		)
	}

	// Derive the command context with a cancel function
	cmdCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	signal.Notify(signalChan, syscall.SIGTERM, syscall.SIGUSR1)
	defer signal.Stop(signalChan)

	// Signal handler goroutine — race-safe with atomic flags
	// We never stop listening since it is possible to receive more than one signals
	// For ex: spot reclaim => SIGUSR1 -> SIGTERM
	go func() {
		for sig := range signalChan {
			switch sig {
			case syscall.SIGUSR1:
				spotReclaimDetected.Store(true)
				if !gracefulShutdownRequested.Load() {
					cLog.Infof("Received SIGUSR1: marking spot reclaim detected, cancelling context ASAP (max %ds)", cfg.Controller.SpotInstanceReclaimTime)
					cancel()
					if cfg.Controller.Prometheus.Enabled {
						metrics.IncSpotInterruption(cfg.Controller.LocalID)
					}
				} else {
					cLog.Info("Received SIGUSR1 after SIGTERM: marking spot reclaim detected (shutdown already in progress)")
				}

				// Requeue active job immediately for spot reclaim
				activeJobMutex.Lock()
				if activeJob != nil {
					cLog.Infof("Spot reclaim detected during job: %v", activeJob.OriginalFile)
					requeueJob(activeJob)
					activeJob = nil
					notifyJobDone()
				}
				activeJobMutex.Unlock()
			case syscall.SIGTERM:
				gracefulShutdownRequested.Store(true)
				if !spotReclaimDetected.Load() {
					cLog.Info("Received SIGTERM: graceful shutdown requested")
				}
				// NOTE: DO NOT call cancel() here to respect the termination grace period
				// If there's an active job, start a timer to enforce grace period
				activeJobMutex.Lock()
				if activeJob != nil {
					activeJob := *activeJob
					activeJobMutex.Unlock()
					jobDoneChanMu.Lock()
					if jobDoneChan == nil {
						jobDoneChan = make(chan struct{})
					}
					// Using a local copy prevents subtle bugs and race conditions that can happen if the global jobDoneChan
					// is modified from elsewhere
					localJobDoneChan := jobDoneChan
					jobDoneChanMu.Unlock()
					go func() {
						cLog.Infof("Allowing in-flight job %s to finish (max %ds)...", activeJob.OriginalFile, cfg.Controller.TerminationGracePeriod)
						select {
						case <-time.After(time.Duration(cfg.Controller.TerminationGracePeriod) * time.Second):
							cLog.Info("Termination grace period expired. Cancelling context to force shutdown...")
							cancel()

						// Closing the done channel immediately  succeeds in reading (returning zero value)
						// This case is triggered when the running job is done and the channel is closed—letting the
						// shutdown happen promptly instead of waiting out the whole grace period.
						case <-localJobDoneChan:
							cLog.Info("Job finished before grace period expired. Cancelling context to exit immediately...")
							cancel()
						}
					}()
				} else {
					activeJobMutex.Unlock()
					cLog.Info("No active job during SIGTERM, cancelling context to exit immediately")
					cancel()
				}
			}
		}
	}()

	// This goroutine's purpose is to log a message immediately when a
	// cancellation request (e.g., ctx expiration/cancellation, SIGTERM, etc.)
	// is received. It ensures timely logging of the request's reception,
	// which may be important for diagnostics. Without this
	// goroutine, if the prover is busy with a proof when, for example, a
	// SIGTERM is received, there would be no log entry about the signal
	// until the proof completes.
	go func() {
		<-cmdCtx.Done()
		if spotReclaimDetected.Load() {
			cLog.Infof("Context done due to spot reclaim. Aborting ASAP (max %ds)...", cfg.Controller.SpotInstanceReclaimTime)
		} else if gracefulShutdownRequested.Load() {
			activeJobMutex.Lock()
			if activeJob != nil {
				cLog.Infof("Context done: grace period expired, forcing shutdown")
			} else {
				cLog.Info("Context done: graceful shutdown complete")
			}
			activeJobMutex.Unlock()
		} else {
			cLog.Infoln("Context done due to cancellation request.")
		}
	}()

	for {
		select {
		case <-cmdCtx.Done():
			cLog.Infoln("Context cancelled by caller or externally triggered (SIGTERM/SIGUSR1). Exiting...")
			metrics.ShutdownServer(cmdCtx)

			if spotReclaimDetected.Load() {
				cLog.Infof("Waiting up to %ds for spot reclaim...", cfg.Controller.SpotInstanceReclaimTime)
				time.Sleep(time.Duration(cfg.Controller.SpotInstanceReclaimTime) * time.Second)
			}

			// For graceful shutdown, do not sleep here and requeue only if
			// the job is still active (timer already slept in signal handler) - See signal handler go routine
			activeJobMutex.Lock()
			if !spotReclaimDetected.Load() && gracefulShutdownRequested.Load() && activeJob != nil {
				cLog.Infof("Job %v did not finish before termination grace period. Requeuing...", activeJob.OriginalFile)
				requeueJob(activeJob)
				activeJob = nil
				activeJobMutex.Unlock()
				notifyJobDone()
			} else {
				activeJobMutex.Unlock()
			}

			return

			// Processing a new job
		case <-retryDelay(cfg.Controller.RetryDelays, numRetrySoFar):
			// Prevent starting new jobs if shutdown started
			if cmdCtx.Err() != nil {
				continue
			}

			//  Skip fetching new jobs if graceful shutdown or spot reclaim is in progress
			if gracefulShutdownRequested.Load() || spotReclaimDetected.Load() {
				numRetrySoFar++
				noJobFoundMsg := "Shutdown in progress, skipping new jobs"
				if numRetrySoFar > 5 {
					cLog.Debug(noJobFoundMsg)
				} else {
					cLog.Info(noJobFoundMsg)
				}
				continue
			}

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

			numRetrySoFar = 0

			// Important: Set the active job to the current job for safe requeue mechanism
			activeJobMutex.Lock()
			activeJob = job
			activeJobMutex.Unlock()

			// Run the command (potentially retrying in large mode)
			status := executor.Run(cmdCtx, job)

			// CreateColumns the job according to the status we got
			switch {
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
				logrus.Infof("Moving tmp response file %v to final response file: %v", tmpRespFile, respFile)
				if err := os.Rename(tmpRespFile, respFile); err != nil {
					// It is unclear how the rename operation could fail
					// here. If this happens, we prefer removing the tmp file.
					// Note that the operation is an `rm -f`.
					os.Remove(tmpRespFile)
					cLog.Errorf("Error renaming %v to %v: %v, removed tmp file", tmpRespFile, respFile, err)
				}

				// After the successful execution proof request, we delete the
				// associated trace file to save costs on EFS. This steps won't
				// alter the controller flow if it fail as it is meant as an
				// optimization.
				if job.Def.Name == jobNameExecution && job.End > cfg.Execution.KeepTracesUntilBlock {
					tryDeleteExecution(cLog, job, cfg)
				}

				// Move the inprogress to the done directory
				cLog.Infof("Moving %v to %v with success prefix", job.OriginalFile, job.Def.dirDone())
				jobDone := job.DoneFile(status)
				if err := os.Rename(job.InProgressPath(), jobDone); err != nil {
					// When that happens, the only thing left to do is to log
					// the error and let the inprogress file where it is. It
					// will likely require a human intervention.
					//
					// Note: this is assumedly an unreachable code path
					cLog.Errorf("Error renaming %v to %v: %v", job.InProgressPath(), jobDone, err)
				}

				// Set active job to nil once the job is successful
				activeJobMutex.Lock()
				activeJob = nil
				activeJobMutex.Unlock()
				notifyJobDone()

			// Defer to the large prover:
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
					cLog.Errorf("Error deriving large name for %v: %v", job.InProgressPath(), err)
				}
				if err := os.Rename(job.InProgressPath(), toLargePath); err != nil {
					cLog.Errorf("Error renaming %v to %v: %v", job.InProgressPath(), toLargePath, err)
				}

				// From the controller perspective, it has completed its job by renaming and
				// moving it to the large prover’s queue. Setting activeJob to nil prevents
				// incorrect requeuing during shutdown, avoiding filesystem errors
				activeJobMutex.Lock()
				activeJob = nil
				activeJobMutex.Unlock()
				notifyJobDone()

			// Controller killed the job via external signal handlers
			// Important not to set active job to nil so that it can re-queued again if necessary
			case status.ExitCode == CodeKilledByUs:
				// When receiving the killed-by-us code, the prover will put back the file in the request queue
				// automatically if spot reclaim is detected. For graceful shutdown (only SIGTERM), the proof will be
				// continue until terminationGracePeriod is reached and if the job is not finished it is automatically rqueued again.
				cLog.Infof("Active job %v killed by external signal handler: Job requeued for either spot-reclaim or if job is not finished within %v (graceful shutdown)", job.OriginalFile, cfg.Controller.TerminationGracePeriod)

				// As an edge-case, it's possible (in theory) that the process
				// completes exactly when we receive the kill signal. So we
				// could end up in a situation where the tmp-response file
				// exists. In that case, we simply delete it before exiting to
				// keep the FS clean.
				os.Remove(job.TmpResponseFile(cfg))

			// Failure case
			default:
				// Move the inprogress to the done directory
				cLog.Infof("Moving %v with failure suffix (code %v)", job.OriginalFile, status.ExitCode)
				jobFailed := job.DoneFile(status)
				if err := os.Rename(job.InProgressPath(), jobFailed); err != nil {
					// When that happens, the only thing left to do is to log and will require human intervention
					cLog.Errorf("Error renaming %v to %v: %v", job.InProgressPath(), jobFailed, err)
				}

				// Set active job to nil as there is nothing more to retry
				activeJobMutex.Lock()
				activeJob = nil
				activeJobMutex.Unlock()
				notifyJobDone()
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

// tryDeleteExecution attemps to delete the execution trace file to free up some
// space. It will log a warning if it fails to delete and keep going if it fails
// to do so.
//
// The function expects to be provided an execution job or it will panic.
func tryDeleteExecution(cLog *logrus.Entry, job *Job, cfg *config.Config) {

	if job.Def.Name != jobNameExecution {
		panic("expected an execution job")
	}

	req := &execution.Request{}
	f, err := os.Open(job.InProgressPath())
	if err != nil {
		// This is sort of unexpected. It could happen if the underlying prover binary
		// could delete or move the request folder. If it fails open, the file then
		// we cannot delete the trace file and we simply continue as if nothing happened
		// on the normal flow.
		cLog.Errorf("could not open file: %s", err.Error())
		return
	}

	defer f.Close()

	if err := json.NewDecoder(f).Decode(req); err != nil {
		cLog.Errorf("could not decode input file: %s", err.Error())
		return
	}

	// Get the trace file and delete it. It is gauranteed that trace file will exist
	// If otherwise, the prover would panic at the beginning.
	traceFile := req.ConflatedExecTraceFilepath(cfg.Execution.ConflatedTracesDir)
	if err := os.Remove(traceFile); err != nil {
		cLog.Errorf("could not remove trace file: %s", err.Error())
		return
	}

	cLog.Infof("Deleted trace file: %s after successful execution proof request", traceFile)
}
