package limitless

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"sync"
	"time"

	"github.com/consensys/linea-monorepo/prover/backend/execution"
	"github.com/consensys/linea-monorepo/prover/circuits"
	execCirc "github.com/consensys/linea-monorepo/prover/circuits/execution"
	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed"
	"github.com/consensys/linea-monorepo/prover/protocol/serde"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/exit"
	"github.com/consensys/linea-monorepo/prover/utils/profiling"
	"github.com/consensys/linea-monorepo/prover/zkevm"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

var (
	witnessDir = "/tmp/witnesses"
	// numConcurrentWitnessWritingGoroutines governs the goroutine serializing,
	// compressing and writing the  witness. The writing part is also controlled
	// by a semaphore on top of this.
	numConcurrentWitnessWritingGoroutines = 20
	// numConcurrentSubProverJobs governs the number of concurrent sub-prover
	// jobs.
	numConcurrentSubProverJobs = 4
	// numConcurrentMergeJobs governs the number of concurrent conglomeration
	// merge operations during hierarchical reduction.
	numConcurrentMergeJobs = 4
)

// Prove function for the Assest struct
func Prove(cfg *config.Config, req *execution.Request) (*execution.Response, error) {

	// Set MonitorParams before any proving happens
	profiling.SetMonitorParams(cfg)

	// Initialize JSONL performance event logger
	plog := newPerfLogger()
	defer plog.close()

	// Setting the issue handler to exit on unsatisfied constraint and missing trace file,
	// but not limit overflow.
	exit.SetIssueHandlingMode(exit.ExitOnUnsatisfiedConstraint | exit.ExitOnMissingTraceFile)

	// Clean up witness directory to be sure it is empty when we start the
	// process. This helps addressing the situation where a previous process
	// have been interrupted.
	os.RemoveAll(witnessDir)
	defer os.RemoveAll(witnessDir)

	// Setup execution witness and output response
	var (
		out     = execution.CraftProverOutput(cfg, req)
		witness = execution.NewWitness(cfg, req, &out)
	)

	if cfg.Execution.LimitlessWithDebug {
		limitlessZkEVM := zkevm.NewLimitlessDebugZkEVM(cfg)
		limitlessZkEVM.RunDebug(cfg, witness.ZkEVM)
		// The return of "out" is to avoid panics later on in the process.
		return &out, nil
	}

	// -- 1. Launch bootstrapper
	logrus.Info("Starting to run the bootstrapper")
	bootStart := plog.phaseStart("bootstrapper")

	mt, err := zkevm.LoadVerificationKeyMerkleTree(cfg)
	if err != nil {
		return nil, fmt.Errorf("could not load verification key merkle tree: %w", err)
	}

	var (
		numGL, numLPP = RunBootstrapper(cfg, witness.ZkEVM, mt.GetRoot())
	)
	plog.phaseEnd("bootstrapper", bootStart)
	logrus.Infof("Finished running the bootstrapper, generated %d GL modules and %d LPP modules", numGL, numLPP)

	// Use a parent context for the whole proving flow
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Final Conglomeration outcome
	type congResult struct {
		proof   *distributed.SegmentProof
		err     error
		congBuf *serde.MmapBackedBuffer // mmap buffer holding the conglomeration circuit
	}

	var (
		totalProofs = numGL + numLPP

		proofGLs   = make([]*distributed.SegmentProof, numGL)
		glErrGroup = &errgroup.Group{}

		lppErrGroup = &errgroup.Group{}

		// Conglomeration proof pipeline setup: have a buffered channel proofStream
		// with capacity large enough so producers won't block
		proofStream = make(chan *distributed.SegmentProof, totalProofs)
		resultCh    = make(chan congResult, 1)

		cong *distributed.RecursedSegmentCompilation
	)

	// -- 2. Launch background hierarchical reduction pipeline to recursively conglomerate as 2 or more
	// proofs come in. It will exit when it collects `totalProofs` or when ctx is cancelled.
	go func() {
		var (
			err     error
			congBuf *serde.MmapBackedBuffer
		)
		cong, congBuf, err = zkevm.LoadCompiledConglomerationMmap(cfg)
		if err != nil || cong == nil {
			panic(fmt.Errorf("could not load compiled conglomeration: %w", err))
		}
		logrus.Infoln("Succesfully loaded the compiled conglomeration and starting to run hierarchical conglomeration")
		proof, err := RunConglomerationHierarchical(ctx, mt, cong, proofStream, totalProofs, plog)
		resultCh <- congResult{proof: proof, err: err, congBuf: congBuf}
	}()

	// -- 3. Launch GL proof jobs, sending each result to proofStream
	glPhaseStart := plog.phaseStart("GL")
	glErrGroup.SetLimit(numConcurrentSubProverJobs)

	// Create ordered witness indices: descending order for longest-first scheduling
	// This ensures larger jobs run first, improving pipeline utilization and reducing tail idle
	glOrder := make([]int, numGL)
	for i := 0; i < numGL; i++ {
		glOrder[i] = numGL - 1 - i // Reverse order: [13,12,11,...,0]
	}
	plog.jobOrder("GL", glOrder)

	for _, i := range glOrder {
		i := i // local copy for closure
		glErrGroup.Go(func() error {
			// If the overall context is done, exit early
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			glJobStart := plog.jobStart("GL", i)

			var (
				jobErr   error
				proofGL  *distributed.SegmentProof
				glModule string
			)

			// RunGL may panic and therefore exit the goroutine without returning
			// an error. Therefore it has to be wrapped in a recoverable function
			// call so that the error handling mechanism work. If an error occur
			// it is assigned to jobErr so that we can check it once the wrapper
			// function returns.
			func() {
				defer func() {
					if r := recover(); r != nil {
						jobErr = fmt.Errorf("GL prover for witness index=%v panicked: %v", i, r)
						logrus.Error(jobErr)
						debug.PrintStack()
						return
					}
				}()

				var err error
				proofGL, err = RunGL(cfg, i, plog)
				if err != nil {
					jobErr = fmt.Errorf("could not run GL prover for witness index=%v: %w", i, err)
				}
			}()

			if proofGL != nil {
				glModule = fmt.Sprintf("type%d_mod%d_seg%d", proofGL.ProofType, proofGL.ModuleIndex, proofGL.SegmentIndex)
			}
			plog.jobEnd("GL", i, glModule, glJobStart)

			if jobErr != nil {
				// Return error to errgroup; caller (main) will cancel ctx
				return jobErr
			}

			// Store local copy for shared randomness computation
			proofGLs[i] = proofGL

			// Safe send: if ctx cancelled, abort send
			select {
			case proofStream <- proofGL:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	}

	// Wait for GLs
	if err := glErrGroup.Wait(); err != nil {
		plog.phaseEnd("GL", glPhaseStart)
		// Cancel overall flow (aggregator will observe ctx.Done)
		cancel()
		// Wait for aggregator to finish (so resultCh gets something)
		res := <-resultCh
		if res.err != nil {
			// aggregator returned an error (likely due to cancel); return original error
			return nil, fmt.Errorf("GL error: %w (aggregator error: %v)", err, res.err)
		}
		return nil, fmt.Errorf("GL error: %w", err)
	}
	plog.phaseEnd("GL", glPhaseStart)
	plog.flush()

	// -- 4. Compute shared randomness FIRST (while proofGLs is still valid)
	sharedRandomness := distributed.GetSharedRandomnessFromSegmentProofs(proofGLs)

	// Release proofGLs references — proofs were already sent to proofStream
	// for conglomeration; this array is the only remaining reference from this goroutine.
	for i := range proofGLs {
		proofGLs[i] = nil
	}
	proofGLs = nil

	// GC between GL and LPP phases to reclaim GL transient heap allocations.
	// With mmap-backed circuit/witness loading, compiled circuits are already released
	// via Munmap in RunGL. This GC targets remaining proving transients.
	var memBefore runtime.MemStats
	runtime.ReadMemStats(&memBefore)
	heapBefore := float64(memBefore.HeapAlloc) / (1 << 30)
	gcStart := time.Now()

	runtime.GC()
	debug.FreeOSMemory()

	gcEnd := time.Now()
	var memAfter runtime.MemStats
	runtime.ReadMemStats(&memAfter)
	heapAfter := float64(memAfter.HeapAlloc) / (1 << 30)
	plog.gcStats(gcStart, gcEnd, heapBefore, heapAfter)

	// -- 5. Launch LPP proof jobs, streaming each proof to pipeline
	lppPhaseStart := plog.phaseStart("LPP")
	lppErrGroup.SetLimit(numConcurrentSubProverJobs)

	// Create ordered witness indices: descending order for longest-first scheduling
	lppOrder := make([]int, numLPP)
	for i := 0; i < numLPP; i++ {
		lppOrder[i] = numLPP - 1 - i // Reverse order: [13,12,11,...,0]
	}
	plog.jobOrder("LPP", lppOrder)

	for _, i := range lppOrder {
		i := i
		lppErrGroup.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			lppJobStart := plog.jobStart("LPP", i)

			var (
				jobErr    error
				proofLPP  *distributed.SegmentProof
				lppModule string
			)

			// RunLPP may panic and therefore exit the goroutine without returning
			// an error. Therefore it has to be wrapped in a recoverable function
			// call so that the error handling mechanism work. If an error occur
			// it is assigned to jobErr so that we can check it once the wrapper
			// function returns.
			func() {
				defer func() {
					if r := recover(); r != nil {
						jobErr = fmt.Errorf("LPP prover for witness index=%v panicked: %v", i, r)
						logrus.Error(jobErr)
						debug.PrintStack()
						return
					}
				}()

				var err error
				proofLPP, err = RunLPP(cfg, i, sharedRandomness, plog)
				if err != nil {
					jobErr = fmt.Errorf("could not run LPP prover for witness index=%v: %w", i, err)
				}
			}()

			if proofLPP != nil {
				lppModule = fmt.Sprintf("type%d_mod%d_seg%d", proofLPP.ProofType, proofLPP.ModuleIndex, proofLPP.SegmentIndex)
			}
			plog.jobEnd("LPP", i, lppModule, lppJobStart)

			if jobErr != nil {
				return jobErr
			}

			select {
			case proofStream <- proofLPP:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	}

	// Wait for LPPs
	if err := lppErrGroup.Wait(); err != nil {
		plog.phaseEnd("LPP", lppPhaseStart)
		cancel()
		res := <-resultCh
		if res.err != nil {
			return nil, fmt.Errorf("LPP error: %w (aggregator error: %v)", err, res.err)
		}
		return nil, fmt.Errorf("LPP error: %w", err)
	}
	plog.phaseEnd("LPP", lppPhaseStart)
	plog.flush()

	// Post-LPP GC: all compiled circuits were Munmap'd inside RunLPP, but proving
	// transients remain on the Go heap. Reclaim before the conglo sequential tail
	// so the 15+ serial merges run with lower memory pressure.
	runtime.GC()
	debug.FreeOSMemory()

	// All producers finished successfully: close the proofStream so aggregator can finish
	close(proofStream)

	// Start setup loading in background to overlap with conglomeration tail.
	// Setup takes ~40s (loads proving key, SRS, circuit) and is independent of conglomeration.
	setupStart := plog.phaseStart("setup_load")
	var (
		setup       circuits.Setup
		errSetup    error
		chSetupDone = make(chan struct{})
	)
	go func() {
		logrus.Infof("Loading setup - circuitID: %s", circuits.ExecutionLimitlessCircuitID)
		setup, errSetup = circuits.LoadSetup(cfg, circuits.ExecutionLimitlessCircuitID)
		close(chSetupDone)
	}()

	// Wait for final conglomeration proof (setup loads concurrently)
	congStart := plog.phaseStart("conglomeration_wait")
	res := <-resultCh
	plog.phaseEnd("conglomeration_wait", congStart)
	if res.err != nil {
		return nil, fmt.Errorf("conglomeration failed: %w", res.err)
	}

	logrus.Infof("HIERARCHICAL CONGLOMERATION SUCCESSFUL!!!")

	congFinalproof := res.proof

	// Wait for setup to finish (likely already done; conglo tail >> setup load time)
	<-chSetupDone
	plog.phaseEnd("setup_load", setupStart)
	if errSetup != nil {
		utils.Panic("could not load setup: %v", errSetup)
	}

	outerStart := plog.phaseStart("outer_proof")
	out.Proof = execCirc.MakeProof(
		&cfg.TracesLimits,
		setup,
		cong.RecursionCompBLS,
		congFinalproof.GetOuterProofInput(),
		*witness.FuncInp,
		witness.ZkEVM.ExecData,
	)

	plog.phaseEnd("outer_proof", outerStart)

	// Release the conglomeration circuit mmap buffer now that MakeProof is done.
	// cong.RecursionCompBLS pointed into this buffer.
	cong = nil
	res.congBuf.Release()

	out.VerifyingKeyShaSum = setup.VerifyingKeyDigest()

	return &out, nil
}

// RunBootstrapper loads the assets required to run the bootstrapper and runs it,
// the function then performs the module segmentation and saves each module
// witness in the /tmp directory.
func RunBootstrapper(cfg *config.Config, zkevmWitness *zkevm.Witness, merkleTreeRoot field.Octuplet) (int, int) {

	logrus.Infof("Loading bootstrapper and zkevm")
	assets := &zkevm.LimitlessZkEVM{}
	if err := assets.LoadBootstrapper(cfg); err != nil || assets.DistWizard.Bootstrapper == nil {
		utils.Panic("could not load bootstrapper: %v", err)
	}

	if err := assets.LoadZkEVM(cfg); err != nil || assets.Zkevm == nil {
		utils.Panic("could not load zkevm: %v", err)
	}

	// Override the compatibility check setting from the runtime config, since
	// the serialized asset may have been built with a different value.
	assets.Zkevm.Arithmetization.Settings.IgnoreCompatibilityCheck = &cfg.Execution.IgnoreCompatibilityCheck

	if err := assets.LoadDisc(cfg); err != nil || assets.DistWizard.Disc == nil {
		utils.Panic("could not load disc: %v", err)
	}

	// The GL and LPP modules are loaded in the background immediately but we
	// only need them for the [distributed.SegmentRuntime] call.
	distDone := make(chan error)
	go func() {
		err := assets.LoadBlueprints(cfg)
		distDone <- err
	}()

	// The function initially attempt to run the bootstrapper directly and will
	// catch "limit-overflow" panic msgs. When they happen, we reattempt running
	// the bootstrapper with higher and higher limits until it works.
	var (
		scalingFactor = 1
		runtimeBoot   *wizard.ProverRuntime
	)

	for runtimeBoot == nil {

		logrus.Infof("Trying to bootstrap with a scaling of %v\n", scalingFactor)

		func() {

			// Since the [exit] package is configured to only send panic messages
			// on overflow. The overflows are catchable.
			defer func() {
				if err := recover(); err != nil {
					oFReport, isOF := err.(exit.LimitOverflowReport)
					if isOF {
						extra := utils.DivCeil(oFReport.RequestedSize, oFReport.Limit)
						scalingFactor *= utils.NextPowerOfTwo(extra)
						return
					}

					panic(err)
				}
			}()

			if scalingFactor == 1 {
				logrus.Infof("Running bootstrapper")
				runtimeBoot = wizard.RunProver(
					assets.DistWizard.Bootstrapper,
					assets.Zkevm.GetMainProverStep(zkevmWitness),
					false,
				)
				return
			}

			scaledUpBootstrapper, scaledUpZkEVM := zkevm.GetScaledUpBootstrapper(
				cfg, assets.DistWizard.Disc, scalingFactor,
			)

			// The disc's segment sizes (NewSizes, ColumnsToSize) must stay at
			// x1 so that each segment still fits the x1-compiled GL/LPP
			// blueprints. The segment count naturally doubles because the
			// runtime columns are 2x but each segment remains x1-sized.

			runtimeBoot = wizard.RunProver(
				scaledUpBootstrapper,
				scaledUpZkEVM.GetMainProverStep(zkevmWitness),
				false,
			)
		}()
	}

	// This frees the memory from the assets that are no longer needed. We don't
	// need to do that for the module GLs and LPPs because they are thrown away
	// when the current function returns.
	assets.Zkevm = nil
	assets.DistWizard.Bootstrapper = nil

	if err := <-distDone; err != nil {
		utils.Panic("could not load GL and LPP modules: %v", err)
	}

	logrus.Infof("Segmenting the runtime")
	witnessGLs, witnessLPPs := distributed.SegmentRuntime(
		runtimeBoot,
		assets.DistWizard.Disc,
		assets.DistWizard.BlueprintGLs,
		assets.DistWizard.BlueprintLPPs,
		merkleTreeRoot,
	)

	logrus.Info("Saving the witnesses")

	eg := &errgroup.Group{}
	eg.SetLimit(numConcurrentWitnessWritingGoroutines)

	for i := range witnessGLs {

		// This saves the value of i in the closure to ensure that the right
		// value is passed. It should be obsolete with newer version of Go.
		i := i
		eg.Go(func() error {

			filePath := witnessDir + "/witness-GL-" + strconv.Itoa(i)
			if err := serde.StoreToDisk(filePath, *witnessGLs[i], true); err != nil {
				return fmt.Errorf("could not save witnessGL: %v", err)
			}

			// This frees the memory from the witness that is no longer needed.
			witnessGLs[i] = nil
			return nil
		})

	}

	for i := range witnessLPPs {

		// This saves the value of i in the closure to ensure that the right
		// value is passed. It should be obsolete with newer version of Go.
		i := i

		eg.Go(func() error {

			filePath := witnessDir + "/witness-LPP-" + strconv.Itoa(i)
			if err := serde.StoreToDisk(filePath, *witnessLPPs[i], true); err != nil {
				return fmt.Errorf("could not save witnessLPP: %v", err)
			}

			// This frees the memory from the witness that is no longer needed.
			witnessLPPs[i] = nil
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		utils.Panic("could not save witnesses: %v", err)
	}

	return len(witnessGLs), len(witnessLPPs)
}

// RunGL runs the GL prover for the provided witness index.
// The circuit mmap buffer is released immediately after proving because
// ExtractProof deep-copies all string map keys (column/query names), so
// the proof no longer references the circuit mmap region.
func RunGL(cfg *config.Config, witnessIndex int, plog *perfLogger) (proofGL *distributed.SegmentProof, err error) {

	logrus.Infof("Running the GL-prover for witness index=%v", witnessIndex)

	ioStart := plog.jobStart("GL_io", witnessIndex)

	// Load witness into mmap-backed buffer for explicit memory release
	witness := &distributed.ModuleWitnessGL{}
	witnessFilePath := witnessDir + "/witness-GL-" + strconv.Itoa(witnessIndex)
	witnessBuf, err := serde.LoadFromDiskMmapBacked(witnessFilePath, witness)
	if err != nil {
		return nil, err
	}

	// Capture module name as a heap-allocated copy before mmap release
	// (the original string bytes live in the mmap buffer)
	moduleName := string([]byte(witness.ModuleName))

	logrus.Infof("Loaded the witness for witness index=%v, module=%v", witnessIndex, moduleName)

	// Load compiled GL into mmap-backed buffer for explicit memory release
	compiledGL, circBuf, err := zkevm.LoadCompiledGLMmap(cfg, witness.ModuleName)
	if err != nil {
		witnessBuf.Release()
		return nil, fmt.Errorf("could not load compiled GL: %w", err)
	}

	logrus.Infof("Loaded the compiled GL for witness index=%v, module=%v", witnessIndex, moduleName)
	plog.jobEnd("GL_io", witnessIndex, moduleName, ioStart)

	proveStart := plog.jobStart("GL_prove", witnessIndex)
	_proofGL := compiledGL.ProveSegmentKoala(witness).ClearRuntime()
	plog.jobEnd("GL_prove", witnessIndex, moduleName, proveStart)

	// Release both mmap buffers immediately. The proof's RecursionWitness
	// map keys (column/query names) have been deep-copied by ExtractProof,
	// so nothing in the proof references the circuit mmap anymore.
	compiledGL = nil
	witness = nil
	witnessBuf.Release()
	circBuf.Release()

	logrus.Infof("Finished GL-prover for witness index=%v, module=%v (witness+circuit released)", witnessIndex, moduleName)

	return _proofGL, nil
}

// RunLPP runs the LPP prover for the provided witness index.
// Same immediate-release semantics as RunGL — see RunGL doc comment.
func RunLPP(cfg *config.Config, witnessIndex int, sharedRandomness field.Octuplet, plog *perfLogger) (proofLPP *distributed.SegmentProof, err error) {

	logrus.Infof("Running the LPP-prover for witness index=%v", witnessIndex)

	ioStart := plog.jobStart("LPP_io", witnessIndex)

	// Load witness into mmap-backed buffer for explicit memory release
	witness := &distributed.ModuleWitnessLPP{}
	witnessFilePath := witnessDir + "/witness-LPP-" + strconv.Itoa(witnessIndex)
	witnessBuf, err := serde.LoadFromDiskMmapBacked(witnessFilePath, witness)
	if err != nil {
		return nil, err
	}

	witness.InitialFiatShamirState = sharedRandomness

	// Capture module name as a heap-allocated copy before mmap release
	moduleName := string([]byte(witness.ModuleName))

	logrus.Infof("Loaded the witness for witness index=%v, module=%v", witnessIndex, moduleName)

	// Load compiled LPP into mmap-backed buffer for explicit memory release
	compiledLPP, circBuf, err := zkevm.LoadCompiledLPPMmap(cfg, witness.ModuleName)
	if err != nil {
		witnessBuf.Release()
		return nil, fmt.Errorf("could not load compiled LPP: %w", err)
	}

	logrus.Infof("Loaded the compiled LPP for witness index=%v, module=%v", witnessIndex, moduleName)
	plog.jobEnd("LPP_io", witnessIndex, moduleName, ioStart)

	proveStart := plog.jobStart("LPP_prove", witnessIndex)
	_proofLPP := compiledLPP.ProveSegmentKoala(witness).ClearRuntime()
	plog.jobEnd("LPP_prove", witnessIndex, moduleName, proveStart)

	// Release both mmap buffers immediately — see RunGL doc comment.
	compiledLPP = nil
	witness = nil
	witnessBuf.Release()
	circBuf.Release()

	logrus.Infof("Finished LPP-prover for witness index=%v, module=%v (witness+circuit released)", witnessIndex, moduleName)

	return _proofLPP, nil
}

// RunConglomerationHierarchical aggregates segment proofs into a single proof
// using a pool of concurrent merge workers. During GL+LPP production, merges
// overlap with proving. After production completes, up to numConcurrentMergeJobs
// workers reduce the remaining proofs in parallel — this eliminates the
// sequential tail that previously dominated wall-clock time.
//
// The function returns the final proof or an error, and respects ctx for cancellation.
func RunConglomerationHierarchical(ctx context.Context,
	mt *distributed.VerificationKeyMerkleTree,
	cong *distributed.RecursedSegmentCompilation,
	proofStream <-chan *distributed.SegmentProof, totalProofs int,
	plog *perfLogger,
) (*distributed.SegmentProof, error) {

	congPhaseStart := plog.phaseStart("conglomeration")
	defer plog.phaseEnd("conglomeration", congPhaseStart)

	// `remaining` tracks how many items are in the system (items slice + in-flight
	// merge results). Each merge takes 2 items and produces 1, so remaining--.
	// When remaining == 1 after decrement, the current merge is the final one (BLS).
	var (
		mu         sync.Mutex
		cond       = sync.NewCond(&mu)
		items      []*distributed.SegmentProof
		remaining  = totalProofs
		mergeCount int
		cancelled  bool
	)

	// Signal workers on context cancellation.
	go func() {
		<-ctx.Done()
		mu.Lock()
		cancelled = true
		cond.Broadcast()
		mu.Unlock()
	}()

	// Drain proofStream into items.
	go func() {
		for proof := range proofStream {
			mu.Lock()
			items = append(items, proof)
			logrus.Infof("Received proof (proofType, moduleIdx, segmentIdx) = (%d, %d, %d) for conglomeration",
				proof.ProofType, proof.ModuleIndex, proof.SegmentIndex)
			cond.Broadcast()
			mu.Unlock()
		}
	}()

	var (
		wg         sync.WaitGroup
		finalProof *distributed.SegmentProof
	)

	// Spawn merge workers. Each worker loops: wait for 2+ items, take a pair,
	// merge, push the result back. Workers exit when remaining <= 1 or cancelled.
	for w := 0; w < numConcurrentMergeJobs; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				mu.Lock()
				for len(items) < 2 && remaining > 1 && !cancelled {
					cond.Wait()
				}
				if cancelled || remaining <= 1 {
					mu.Unlock()
					return
				}

				// Take 2 items from the end (LIFO — matches prior behavior).
				n := len(items)
				p1 := items[n-1]
				p2 := items[n-2]
				items[n-1] = nil
				items[n-2] = nil
				items = items[:n-2]
				remaining--
				isLast := remaining == 1
				idx := mergeCount
				mergeCount++
				mu.Unlock()

				// Clear runtime on the first proof to free proving transients.
				p1 = p1.ClearRuntime()

				mergeType := "koala"
				if isLast {
					mergeType = "bls_final"
				}

				logrus.Infof("Conglomerating sub-proofs [merge %d, %s] for (proofType, moduleIdx, segmentIdx) = (%d, %d, %d) and (%d, %d, %d)",
					idx, mergeType,
					p1.ProofType, p1.ModuleIndex, p1.SegmentIndex,
					p2.ProofType, p2.ModuleIndex, p2.SegmentIndex)

				mergeStart := plog.jobStart("conglo_merge", idx)

				wit := &distributed.ModuleWitnessConglo{
					SegmentProofs:             []distributed.SegmentProof{*p1, *p2},
					VerificationKeyMerkleTree: *mt,
				}

				var aggregated *distributed.SegmentProof
				if isLast {
					aggregated = cong.ProveSegmentBLS(wit)
				} else {
					aggregated = cong.ProveSegmentKoala(wit)
				}

				plog.jobEnd("conglo_merge", idx, mergeType, mergeStart)

				mu.Lock()
				items = append(items, aggregated)
				if isLast {
					finalProof = aggregated
				}
				cond.Broadcast()
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	if cancelled {
		return nil, ctx.Err()
	}

	if finalProof != nil {
		logrus.Infoln("Successfully finished running conglomeration prover.")
		return finalProof, nil
	}

	return nil, fmt.Errorf("conglomeration finished without producing a final proof")
}
