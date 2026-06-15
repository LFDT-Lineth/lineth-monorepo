package limitless

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"

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
	"github.com/consensys/linea-monorepo/prover/zkevm/arithmetization"
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
	numConcurrentSubProverJobs = 5
	// numConcurrentMergeJobs governs the number of concurrent conglomeration
	// merge operations during hierarchical reduction.
	numConcurrentMergeJobs = 4
)

// getEnvPositiveInt reads a positive integer from an environment variable,
// returning defaultVal if unset, empty, or invalid.
func getEnvPositiveInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return defaultVal
}

// PipelineResult holds the output of RunDistributedPipeline.
type PipelineResult struct {
	FinalProof *distributed.SegmentProof
	Cong       *distributed.RecursedSegmentCompilation
	CongBuf    *serde.MmapBackedBuffer
	// Setup is the outer-circuit setup, loaded in the background by the pipeline.
	Setup circuits.Setup
}

// Prove function for the Assest struct
func Prove(cfg *config.Config, req *execution.Request) (*execution.Response, error) {

	// Set MonitorParams before any proving happens
	profiling.SetMonitorParams(cfg)

	// Setting the issue handler to exit on unsatisfied constraint and missing trace file,
	// but not limit overflow.
	exit.SetIssueHandlingMode(exit.ExitOnUnsatisfiedConstraint | exit.ExitOnMissingTraceFile)

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

	// Dispatch to on-the-fly prover when serialization is disabled.
	if !cfg.Execution.Serialization {
		return ProveOnTheFly(cfg, req)
	}

	// Run the distributed pipeline: bootstrapper → GL/LPP segment
	// proofs → shared randomness → hierarchical conglomeration.
	pipeline, err := RunDistributedPipeline(cfg, witness.ZkEVM)
	if err != nil {
		return nil, fmt.Errorf("distributed pipeline failed: %w", err)
	}

	setup := pipeline.Setup

	out.Proof = execCirc.MakeProof(
		&cfg.TracesLimits,
		setup,
		pipeline.Cong.RecursionCompBLS,
		pipeline.FinalProof.GetOuterProofInput(),
		*witness.FuncInp,
		witness.ZkEVM.ExecData,
	)

	// Release the conglomeration circuit mmap buffer now that MakeProof is done.
	pipeline.Cong = nil
	pipeline.CongBuf.Release()

	out.VerifyingKeyShaSum = setup.VerifyingKeyDigest()

	return &out, nil
}

// RunDistributedPipeline runs the full limitless distributed proving pipeline:
// bootstrapper → GL segment proofs → shared randomness → LPP segment proofs →
// hierarchical conglomeration. It returns the final conglomeration proof and
// the compiled conglomeration (needed for RecursionCompBLS in the outer proof).
func RunDistributedPipeline(cfg *config.Config, zkevmWitness *zkevm.Witness) (*PipelineResult, error) {

	os.RemoveAll(witnessDir)
	defer os.RemoveAll(witnessDir)

	// -- 1. Launch bootstrapper
	logrus.Info("Starting to run the bootstrapper")

	mt, err := zkevm.LoadVerificationKeyMerkleTree(cfg)
	if err != nil {
		return nil, fmt.Errorf("could not load verification key merkle tree: %w", err)
	}

	var (
		numGL, numLPP, glModuleNames = RunBootstrapper(cfg, zkevmWitness, mt.GetRoot())
	)
	logrus.Infof("Finished running the bootstrapper, generated %d GL modules and %d LPP modules", numGL, numLPP)

	// Use a parent context for the whole proving flow
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	type congResult struct {
		proof   *distributed.SegmentProof
		err     error
		congBuf *serde.MmapBackedBuffer
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
		proof, err := RunConglomerationHierarchical(ctx, mt, cong, proofStream, totalProofs)
		resultCh <- congResult{proof: proof, err: err, congBuf: congBuf}
	}()

	// -- 3. Launch GL proof jobs, sending each result to proofStream
	glErrGroup.SetLimit(numConcurrentSubProverJobs)

	// Group segments by module so glCache loads each module's circuit once.
	glOrder := buildModuleGroupedOrder(glModuleNames)

	glCache := newCircuitCache(glModuleNames)

	var glCompleted atomic.Int64
	var glGCMu sync.Mutex

	for _, i := range glOrder {
		i := i // local copy for closure
		glErrGroup.Go(func() error {
			// If the overall context is done, exit early
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			var (
				jobErr  error
				proofGL *distributed.SegmentProof
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
				proofGL, err = RunGL(cfg, i, glCache)
				if err != nil {
					jobErr = fmt.Errorf("could not run GL prover for witness index=%v: %w", i, err)
				}
			}()

			if jobErr != nil {
				// Return error to errgroup; caller (main) will cancel ctx
				return jobErr
			}

			// Store local copy for shared randomness computation
			proofGLs[i] = proofGL

			// Safe send: if ctx cancelled, abort send
			select {
			case proofStream <- proofGL:
			case <-ctx.Done():
				return ctx.Err()
			}

			n := glCompleted.Add(1)
			if int(n) < numGL {
				if glGCMu.TryLock() {
					logrus.Infof("GL inter-job GC after %d/%d jobs", n, numGL)
					runtime.GC()
					debug.FreeOSMemory()
					glGCMu.Unlock()
				}
			}

			return nil
		})
	}

	// Wait for GLs
	if err := glErrGroup.Wait(); err != nil {
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

	// GL proofs are extracted; the cached GL circuits are no longer needed.
	glCache.release()

	// Load the outer-circuit setup in the background, overlapping it with the LPP
	// and conglomeration phases. Collected just before return.
	type setupResult struct {
		setup circuits.Setup
		err   error
	}
	setupCh := make(chan setupResult, 1)
	go func() {
		logrus.Infof("Loading setup (background) - circuitID: %s", circuits.ExecutionLimitlessCircuitID)
		s, err := circuits.LoadSetup(cfg, circuits.ExecutionLimitlessCircuitID)
		setupCh <- setupResult{setup: s, err: err}
	}()

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
	runtime.GC()
	debug.FreeOSMemory()

	// -- 5. Launch LPP proof jobs, streaming each proof to pipeline
	lppErrGroup.SetLimit(numConcurrentSubProverJobs)

	// Create ordered witness indices: descending order for longest-first scheduling
	lppOrder := make([]int, numLPP)
	for i := 0; i < numLPP; i++ {
		lppOrder[i] = numLPP - 1 - i // Reverse order: [13,12,11,...,0]
	}

	for _, i := range lppOrder {
		i := i
		lppErrGroup.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			var (
				jobErr   error
				proofLPP *distributed.SegmentProof
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
				proofLPP, err = RunLPP(cfg, i, sharedRandomness)
				if err != nil {
					jobErr = fmt.Errorf("could not run LPP prover for witness index=%v: %w", i, err)
				}
			}()

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
		cancel()
		res := <-resultCh
		if res.err != nil {
			return nil, fmt.Errorf("LPP error: %w (aggregator error: %v)", err, res.err)
		}
		return nil, fmt.Errorf("LPP error: %w", err)
	}

	// Post-LPP GC: all compiled circuits were Munmap'd inside RunLPP, but proving
	// transients remain on the Go heap. Reclaim before the conglo sequential tail
	// so the 15+ serial merges run with lower memory pressure.
	runtime.GC()
	debug.FreeOSMemory()

	// All producers finished successfully: close the proofStream so aggregator can finish
	close(proofStream)

	// Wait for final conglomeration proof
	res := <-resultCh
	if res.err != nil {
		return nil, fmt.Errorf("conglomeration failed: %w", res.err)
	}

	logrus.Infof("HIERARCHICAL CONGLOMERATION SUCCESSFUL!!!")

	sr := <-setupCh
	if sr.err != nil {
		return nil, fmt.Errorf("could not load setup: %w", sr.err)
	}

	return &PipelineResult{
		FinalProof: res.proof,
		Cong:       cong,
		CongBuf:    res.congBuf,
		Setup:      sr.setup,
	}, nil
}

// RunBootstrapper loads the assets required to run the bootstrapper and runs it,
// the function then performs the module segmentation and saves each module
// witness in the /tmp directory.
func RunBootstrapper(cfg *config.Config, zkevmWitness *zkevm.Witness, merkleTreeRoot field.Octuplet) (int, int, []string) {

	logrus.Infof("Loading bootstrapper and zkevm")
	assets := &zkevm.LimitlessZkEVM{}

	// Pre-start trace file decompression and parsing in background.
	preReadCh := make(chan arithmetization.PreReadResult, 1)
	go func() {
		logrus.Info("Pre-reading trace file in background")
		preReadCh <- arithmetization.PreReadTrace(zkevmWitness.ExecTracesFPath)
		logrus.Info("Pre-read trace file complete")
	}()

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

	// Wait for the pre-read result. Re-read traceFile from from disk
	preReadResult := <-preReadCh

	// The function initially attempt to run the bootstrapper directly and will
	// catch "limit-overflow" panic msgs. When they happen, we reattempt running
	// the bootstrapper with higher and higher limits until it works.
	var (
		scalingFactor = 1
		runtimeBoot   *wizard.ProverRuntime
	)

	for runtimeBoot == nil {

		logrus.Infof("Trying to bootstrap with a scaling of %v\n", scalingFactor)

		replayCh := make(chan arithmetization.PreReadResult, 1)
		if scalingFactor == 1 {
			replayCh <- preReadResult
		} else {
			// Clone the trace: Propagate mutates TraceFile internals in place.
			clone := preReadResult
			clone.RawTrace = preReadResult.RawTrace.Clone()
			replayCh <- clone
		}

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
					assets.Zkevm.GetMainProverStepWithPreRead(zkevmWitness, replayCh),
					false,
				)
				return
			}

			scaledUpBootstrapper, scaledUpZkEVM := zkevm.GetScaledUpBootstrapper(
				cfg, assets.DistWizard.Disc, scalingFactor,
			)

			runtimeBoot = wizard.RunProver(
				scaledUpBootstrapper,
				scaledUpZkEVM.GetMainProverStepWithPreRead(zkevmWitness, replayCh),
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

	glModuleNames := make([]string, len(witnessGLs))
	for i, w := range witnessGLs {
		glModuleNames[i] = string(w.ModuleName)
	}

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

	return len(witnessGLs), len(witnessLPPs), glModuleNames
}

// RunGL runs the GL prover for the provided witness index.
// The circuit mmap buffer is released immediately after proving because
// ExtractProof deep-copies all string map keys (column/query names), so
// the proof no longer references the circuit mmap region.
func RunGL(cfg *config.Config, witnessIndex int, circuits *circuitCache) (proofGL *distributed.SegmentProof, err error) {

	logrus.Infof("Running the GL-prover for witness index=%v", witnessIndex)

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

	// Fetch the compiled GL circuit from the cache. Key on the heap-copied
	// moduleName, not witness.ModuleName, which points into the soon-released mmap.
	compiledGL, err := circuits.getGL(cfg, distributed.ModuleName(moduleName))
	if err != nil {
		witnessBuf.Release()
		return nil, fmt.Errorf("could not load compiled GL: %w", err)
	}

	logrus.Infof("Loaded the compiled GL for witness index=%v, module=%v", witnessIndex, moduleName)

	_proofGL := compiledGL.ProveSegmentKoala(witness).ClearRuntime()

	// Release the witness mmap; the circuit stays cached, freed by done() after
	// this module's last segment.
	witness = nil
	witnessBuf.Release()
	circuits.done(distributed.ModuleName(moduleName))

	logrus.Infof("Finished GL-prover for witness index=%v, module=%v (witness released)", witnessIndex, moduleName)

	return _proofGL, nil
}

// RunLPP runs the LPP prover for the provided witness index.
// Same immediate-release semantics as RunGL — see RunGL doc comment.
func RunLPP(cfg *config.Config, witnessIndex int, sharedRandomness field.Octuplet) (proofLPP *distributed.SegmentProof, err error) {

	logrus.Infof("Running the LPP-prover for witness index=%v", witnessIndex)

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

	_proofLPP := compiledLPP.ProveSegmentKoala(witness).ClearRuntime()

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
) (*distributed.SegmentProof, error) {

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

// buildModuleGroupedOrder emits each module's segments contiguously, largest
// module first, so a cached circuit loads once and is released before the next
// module starts, bounding resident memory.
func buildModuleGroupedOrder(moduleNames []string) []int {
	type group struct {
		name    string
		indices []int
	}
	seen := map[string]int{}
	var groups []group
	for i, name := range moduleNames {
		if idx, ok := seen[name]; ok {
			groups[idx].indices = append(groups[idx].indices, i)
		} else {
			seen[name] = len(groups)
			groups = append(groups, group{name: name, indices: []int{i}})
		}
	}
	sort.Slice(groups, func(a, b int) bool {
		return len(groups[a].indices) > len(groups[b].indices)
	})
	order := make([]int, 0, len(moduleNames))
	for g := range groups {
		order = append(order, groups[g].indices...)
	}
	return order
}

// buildRoundRobinOrder interleaves GL jobs across module types so that
// segments of the same module are spread apart in the schedule.
func buildRoundRobinOrder(moduleNames []string) []int {
	type group struct {
		name    string
		indices []int
	}
	seen := map[string]int{}
	var groups []group
	for i, name := range moduleNames {
		if idx, ok := seen[name]; ok {
			groups[idx].indices = append(groups[idx].indices, i)
		} else {
			seen[name] = len(groups)
			groups = append(groups, group{name: name, indices: []int{i}})
		}
	}

	sort.Slice(groups, func(a, b int) bool {
		return len(groups[a].indices) > len(groups[b].indices)
	})

	order := make([]int, 0, len(moduleNames))
	for {
		added := false
		for g := range groups {
			if len(groups[g].indices) > 0 {
				order = append(order, groups[g].indices[0])
				groups[g].indices = groups[g].indices[1:]
				added = true
			}
		}
		if !added {
			break
		}
	}
	return order
}
