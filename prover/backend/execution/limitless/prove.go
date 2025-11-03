package limitless

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"
	"strconv"

	"github.com/consensys/linea-monorepo/prover/backend/execution"
	"github.com/consensys/linea-monorepo/prover/circuits"
	execCirc "github.com/consensys/linea-monorepo/prover/circuits/execution"
	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed"
	"github.com/consensys/linea-monorepo/prover/protocol/serialization"
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
)

// Prove function for the Assest struct
func Prove(cfg *config.Config, req *execution.Request) (*execution.Response, error) {

	// Set MonitorParams before any proving happens
	profiling.SetMonitorParams(cfg)

	// Setting the issue handler to exit on unsatisfied constraint but not limit overflow.
	exit.SetIssueHandlingMode(exit.ExitOnUnsatisfiedConstraint)

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

	mt, err := zkevm.LoadVerificationKeyMerkleTree(cfg)
	if err != nil {
		return nil, fmt.Errorf("could not load verification key merkle tree: %w", err)
	}

	var (
		numGL, numLPP = RunBootstrapper(cfg, witness.ZkEVM, mt.GetRoot())
	)
	logrus.Infof("Finished running the bootstrapper, generated %d GL modules and %d LPP modules", numGL, numLPP)

	// Use a parent context for the whole proving flow
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Final Conglomeration outcome
	type congResult struct {
		proof *distributed.SegmentProof
		err   error
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
		var err error
		cong, err = zkevm.LoadCompiledConglomeration(cfg)
		if err != nil || cong == nil {
			panic(fmt.Errorf("could not load compiled conglomeration: %w", err))
		}
		logrus.Infoln("Succesfully loaded the compiled conglomeration and starting to run hierarchical conglomeration")
		proof, err := RunConglomerationHierarchical(ctx, mt, cong, proofStream, totalProofs)
		resultCh <- congResult{proof: proof, err: err}
	}()

	// -- 3. Launch GL proof jobs, sending each result to proofStream
	glErrGroup.SetLimit(numConcurrentSubProverJobs)

	for i := 0; i < numGL; i++ {
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
				proofGL, err = RunGL(cfg, i)
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
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
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

	var (
		//--4. Compute shared randomness after GL proofs succeeded
		sharedRandomness = distributed.GetSharedRandomnessFromSegmentProofs(proofGLs)
	)

	// -- 5. Launch LPP proof jobs, streaming each proof to pipeline
	lppErrGroup.SetLimit(numConcurrentSubProverJobs)
	for i := 0; i < numLPP; i++ {
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

	// All producers finished successfully: close the proofStream so aggregator can finish
	close(proofStream)

	// Wait for final conglomeration proof
	res := <-resultCh
	if res.err != nil {
		return nil, fmt.Errorf("conglomeration failed: %w", res.err)
	}

	logrus.Infof("HIERARCHICAL CONGLOMERATION SUCCESSFUL!!!")

	congFinalproof := res.proof

	// -- 5. Load setup (in background started earlier maybe)
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

	// Wait for setup to finish loading
	<-chSetupDone
	if errSetup != nil {
		utils.Panic("could not load setup: %v", errSetup)
	}

	out.Proof = execCirc.MakeProof(
		&cfg.TracesLimits,
		setup,
		cong.RecursionComp,
		congFinalproof.GetOuterProofInput(),
		*witness.FuncInp,
	)

	out.VerifyingKeyShaSum = setup.VerifyingKeyDigest()

	return &out, nil
}

// RunBootstrapper loads the assets required to run the bootstrapper and runs it,
// the function then performs the module segmentation and saves each module
// witness in the /tmp directory.
func RunBootstrapper(cfg *config.Config, zkevmWitness *zkevm.Witness, merkleTreeRoot field.Element) (int, int) {

	logrus.Infof("Loading bootstrapper and zkevm")
	assets := &zkevm.LimitlessZkEVM{}
	if err := assets.LoadBootstrapper(cfg); err != nil || assets.DistWizard.Bootstrapper == nil {
		utils.Panic("could not load bootstrapper: %v", err)
	}

	if err := assets.LoadZkEVM(cfg); err != nil || assets.Zkevm == nil {
		utils.Panic("could not load zkevm: %v", err)
	}

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
				)
				return
			}

			scaledUpBootstrapper, scaledUpZkEVM := zkevm.GetScaledUpBootstrapper(
				cfg, assets.DistWizard.Disc, scalingFactor,
			)

			runtimeBoot = wizard.RunProver(
				scaledUpBootstrapper,
				scaledUpZkEVM.GetMainProverStep(zkevmWitness),
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
			if err := serialization.StoreToDisk(filePath, *witnessGLs[i], true); err != nil {
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
			if err := serialization.StoreToDisk(filePath, *witnessLPPs[i], true); err != nil {
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

// RunGL runs the GL prover for the provided witness index
func RunGL(cfg *config.Config, witnessIndex int) (proofGL *distributed.SegmentProof, err error) {

	logrus.Infof("Running the GL-prover for witness index=%v", witnessIndex)

	witness := &distributed.ModuleWitnessGL{}
	witnessFilePath := witnessDir + "/witness-GL-" + strconv.Itoa(witnessIndex)
	if err := serialization.LoadFromDisk(witnessFilePath, witness, true); err != nil {
		return nil, err
	}

	logrus.Infof("Loaded the witness for witness index=%v, module=%v", witnessIndex, witness.ModuleName)

	compiledGL, err := zkevm.LoadCompiledGL(cfg, witness.ModuleName)
	if err != nil {
		return nil, fmt.Errorf("could not load compiled GL: %w", err)
	}

	logrus.Infof("Loaded the compiled GL for witness index=%v, module=%v", witnessIndex, witness.ModuleName)

	_proofGL := compiledGL.ProveSegment(witness).ClearRuntime()

	logrus.Infof("Finished running the GL-prover for witness index=%v, module=%v", witnessIndex, witness.ModuleName)

	return _proofGL, nil
}

// RunLPP runs the LPP prover for the provided witness index
func RunLPP(cfg *config.Config, witnessIndex int, sharedRandomness field.Element) (proofLPP *distributed.SegmentProof, err error) {

	logrus.Infof("Running the LPP-prover for witness index=%v", witnessIndex)

	witness := &distributed.ModuleWitnessLPP{}
	witnessFilePath := witnessDir + "/witness-LPP-" + strconv.Itoa(witnessIndex)
	if err := serialization.LoadFromDisk(witnessFilePath, witness, true); err != nil {
		return nil, err
	}

	witness.InitialFiatShamirState = sharedRandomness

	logrus.Infof("Loaded the witness for witness index=%v, module=%v", witnessIndex, witness.ModuleName)

	compiledLPP, err := zkevm.LoadCompiledLPP(cfg, witness.ModuleName)
	if err != nil {
		return nil, fmt.Errorf("could not load compiled LPP: %w", err)
	}

	logrus.Infof("Loaded the compiled LPP for witness index=%v, module=%v", witnessIndex, witness.ModuleName)

	_proofLPP := compiledLPP.ProveSegment(witness).ClearRuntime()

	logrus.Infof("Finished running the LPP-prover for witness index=%v, module=%v", witnessIndex, witness.ModuleName)

	return _proofLPP, nil
}

// RunConglomerationHierarchical aggregates segment proofs into a single proof.
// It returns the final proof or an error. It respects the passed context for cancellation.
func RunConglomerationHierarchical(ctx context.Context,
	mt *distributed.VerificationKeyMerkleTree,
	cong *distributed.RecursedSegmentCompilation,
	proofStream <-chan *distributed.SegmentProof, totalProofs int,
) (*distributed.SegmentProof, error) {

	// Stack is a slice for channel-based pairing logic
	var stack []*distributed.SegmentProof
	proofsReceived := 0

	// Main loop: block on either new proof, or cancellation.
	for {
		// First, aggregate while we have at least 2 items
		for len(stack) >= 2 {

			// _proof2 normally has already its runtime cleared
			_proof1 := stack[len(stack)-1].ClearRuntime()
			_proof2 := stack[len(stack)-2]
			stack = stack[:len(stack)-2]

			logrus.Infof("Conglomerating sub-proofs for (proofType, moduleIdx, segmentIdx) = (%d, %d, %d) and (%d, %d, %d)",
				_proof1.ProofType, _proof1.ModuleIndex, _proof1.SegmentIndex,
				_proof2.ProofType, _proof2.ModuleIndex, _proof2.SegmentIndex)
			aggregated := cong.ProveSegment(&distributed.ModuleWitnessConglo{
				SegmentProofs:             []distributed.SegmentProof{*_proof1, *_proof2},
				VerificationKeyMerkleTree: *mt,
			})
			stack = append(stack, aggregated)
		}

		// If we've received all proofs and have exactly one on the stack -> done
		if proofsReceived >= totalProofs {
			// Last item is the final proof
			if len(stack) == 1 {
				logrus.Infoln("Successfully finished running conglomeration prover.")
				finalProof := stack[0]
				// Clear stack
				stack = nil
				return finalProof, nil
			}
			return nil, fmt.Errorf("conglomeration finished but stack size=%d (expected 1)", len(stack))
		}

		// Wait for next proof or cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()

		// Receive new proof from main proof stream
		case p, ok := <-proofStream:
			if !ok {
				// sender closed channel prematurely
				if len(stack) == 1 {
					return stack[0], nil
				}
				return nil, fmt.Errorf("proof stream closed prematurely; stack size=%d, proofsReceived=%d, totalProofs=%d", len(stack), proofsReceived, totalProofs)
			}
			logrus.Infof("Received proof (proofType, moduleIdx, segmentIdx) = (%d, %d, %d) for conglomeration", p.ProofType, p.ModuleIndex, p.SegmentIndex)
			stack = append(stack, p)
			proofsReceived++
		}
	}
}
