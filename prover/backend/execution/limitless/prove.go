package limitless

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/consensys/linea-monorepo/prover/backend/execution"
	"github.com/consensys/linea-monorepo/prover/circuits"
	execCirc "github.com/consensys/linea-monorepo/prover/circuits/execution"
	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/recursion"
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

	// Setting the issue handler to exit on unsatisfied constraint but not limit
	// overflow.
	exit.SetIssueHandlingMode(exit.ExitOnUnsatisfiedConstraint)

	// Clean up witness directory to be sure it is empty when we start the
	// process. This helps addressing the situation where a previous process
	// have been interrupted.
	os.RemoveAll(witnessDir)
	defer os.RemoveAll(witnessDir)

	// Setup execution witness and output response
	var (
		out           = execution.CraftProverOutput(cfg, req)
		witness       = execution.NewWitness(cfg, req, &out)
		numGL, numLPP int
	)

	if cfg.Execution.LimitlessWithDebug {
		limitlessZkEVM := zkevm.NewLimitlessDebugZkEVM(cfg)
		limitlessZkEVM.RunDebug(cfg, witness.ZkEVM)
		// The return of "out" is to avoid panics later on in the process.
		return &out, nil
	}

	logrus.Info("Starting to run the bootstrapper")

	numGL, numLPP = RunBootstrapper(cfg, witness.ZkEVM)

	logrus.Infof("Finished running the bootstrapper, generated %d GL modules and %d LPP modules", numGL, numLPP)

	var (
		proofGLs            = make([]recursion.Witness, numGL)
		proofLPPs           = make([]recursion.Witness, numGL)
		lppCommitments      = make([]field.Element, numGL)
		errGroup            = &errgroup.Group{}
		contextGL, cancelGL = context.WithCancel(context.Background())
	)

	errGroup.SetLimit(numConcurrentSubProverJobs)

	for i := 0; i < numGL; i++ {

		errGroup.Go(func() error {

			if contextGL.Err() != nil {
				return nil
			}

			var (
				jobErr        error
				proofGL       *recursion.Witness
				lppCommitment field.Element
			)

			// RunGL may panic and therefore exit the goroutine without returning
			// an error. Therefore it has to be wrapped in a recoverable function
			// call so that the error handling mechanism work. If an error occur
			// it is assigned to jobErr so that we can check it once the wrapper
			// function returns.
			func() {

				defer func() {
					if err := recover(); err != nil {
						jobErr = fmt.Errorf("GL prover for witness index=%v panicked: %v", i, err)
					}
				}()

				var err error
				proofGL, lppCommitment, err = RunGL(cfg, i)
				if err != nil {
					jobErr = fmt.Errorf("could not run GL prover for witness index=%v: %w", i, err)
				}
			}()

			if jobErr != nil {
				cancelGL()
				return jobErr
			}

			proofGLs[i] = *proofGL
			lppCommitments[i] = lppCommitment
			return nil
		})
	}

	err := errGroup.Wait()
	// This context cancellation is here to ensure the context is wiped-out in
	// every branches.
	cancelGL()
	if err != nil {
		return nil, err
	}

	var (
		sharedRandomness      = distributed.GetSharedRandomness(lppCommitments)
		contextLPP, cancelLPP = context.WithCancel(context.Background())
	)

	for i := 0; i < numLPP; i++ {

		errGroup.Go(func() error {

			if contextLPP.Err() != nil {
				return nil
			}

			var (
				jobErr   error
				proofLPP *recursion.Witness
			)

			// RunLPP may panic and therefore exit the goroutine without returning
			// an error. Therefore it has to be wrapped in a recoverable function
			// call so that the error handling mechanism work. If an error occur
			// it is assigned to jobErr so that we can check it once the wrapper
			// function returns.
			func() {

				defer func() {
					if err := recover(); err != nil {
						jobErr = fmt.Errorf("LPP prover for witness index=%v panicked: %v", i, err)
					}
				}()

				var err error
				proofLPP, err = RunLPP(cfg, i, sharedRandomness)
				if err != nil {
					jobErr = fmt.Errorf("could not run LPP prover for witness index=%v: %w", i, err)
				}
			}()

			if jobErr != nil {
				cancelLPP()
				return fmt.Errorf("could not run LPP prover for witness index=%v: %w", i, jobErr)
			}

			proofLPPs[i] = *proofLPP
			return nil
		})
	}

	err = errGroup.Wait()
	// This context cancellation is here to ensure the context is wiped-out in
	// every branches.
	cancelLPP()
	if err != nil {
		return nil, err
	}

	var (
		setup       circuits.Setup
		errSetup    error
		chSetupDone = make(chan struct{})
	)

	// Start loading the setup before starting the conglomeration so that it is
	// ready when we need it.
	go func() {
		logrus.Infof("Loading setup - circuitID: %s", circuits.ExecutionLimitlessCircuitID)
		setup, errSetup = circuits.LoadSetup(cfg, circuits.ExecutionLimitlessCircuitID)
		close(chSetupDone)
	}()

	proofConglo, congloWIOP, err := RunConglomeration(cfg, proofGLs, proofLPPs)
	if err != nil {
		return nil, fmt.Errorf("could not run conglomeration prover: %w", err)
	}

	// wait for setup to be loaded. It should already be loaded normally so we
	// do not expect to actually wait here.
	<-chSetupDone
	if errSetup != nil {
		utils.Panic("could not load setup: %v", errSetup)
	}

	out.Proof = execCirc.MakeProof(
		&cfg.TracesLimits,
		setup,
		congloWIOP,
		proofConglo,
		*witness.FuncInp,
	)

	out.VerifyingKeyShaSum = setup.VerifyingKeyDigest()

	return &out, nil
}

// RunBootstrapper loads the assets required to run the bootstrapper and runs it,
// the function then performs the module segmentation and saves each module
// witness in the /tmp directory.
func RunBootstrapper(cfg *config.Config, zkevmWitness *zkevm.Witness,
) (numWitnessGL, numWitnessLPP int) {

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
func RunGL(cfg *config.Config, witnessIndex int) (proofGL *recursion.Witness, lppCommitments field.Element, err error) {

	logrus.Infof("Running the GL-prover for witness index=%v", witnessIndex)

	witness := &distributed.ModuleWitnessGL{}
	witnessFilePath := witnessDir + "/witness-GL-" + strconv.Itoa(witnessIndex)
	if err := serialization.LoadFromDisk(witnessFilePath, witness, true); err != nil {
		return nil, field.Element{}, err
	}

	logrus.Infof("Loaded the witness for witness index=%v, module=%v", witnessIndex, witness.ModuleName)

	compiledGL, err := zkevm.LoadCompiledGL(cfg, witness.ModuleName)
	if err != nil {
		return nil, field.Element{}, fmt.Errorf("could not load compiled GL: %w", err)
	}

	logrus.Infof("Loaded the compiled GL for witness index=%v, module=%v", witnessIndex, witness.ModuleName)

	run := compiledGL.ProveSegment(witness)

	logrus.Infof("Finished running the GL-prover for witness index=%v, module=%v", witnessIndex, witness.ModuleName)

	_proofGL := recursion.ExtractWitness(run)

	logrus.Infof("Extracted the witness for witness index=%v, module=%v", witnessIndex, witness.ModuleName)

	return &_proofGL, distributed.GetLppCommitmentFromRuntime(run), nil
}

// RunLPP runs the LPP prover for the provided witness index
func RunLPP(cfg *config.Config, witnessIndex int, sharedRandomness field.Element) (proofLPP *recursion.Witness, err error) {

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

	run := compiledLPP.ProveSegment(witness)

	logrus.Infof("Finished running the LPP-prover for witness index=%v, module=%v", witnessIndex, witness.ModuleName)

	_proofLPP := recursion.ExtractWitness(run)

	logrus.Infof("Extracted the witness for witness index=%v, module=%v", witnessIndex, witness.ModuleName)

	return &_proofLPP, nil
}

// RunConglomeration runs the conglomeration prover for the provided subproofs
func RunConglomeration(cfg *config.Config, proofGLs, proofLPPs []recursion.Witness) (proof wizard.Proof, congloWIOP *wizard.CompiledIOP, err error) {

	logrus.Infof("Running the conglomeration-prover")

	cong, err := zkevm.LoadConglomeration(cfg)
	if err != nil {
		return wizard.Proof{}, nil, fmt.Errorf("could not load compiled conglomeration: %w", err)
	}

	logrus.Infof("Loaded the compiled conglomeration")

	proof = cong.Prove(proofGLs, proofLPPs)

	logrus.Infof("Finished running the conglomeration-prover")
	run, err := wizard.VerifyWithRuntime(cong.Wiop, proof)
	if err != nil {
		zkevm.LogPublicInputs(run)
		exit.OnUnsatisfiedConstraints(err)
	}

	logrus.Infof("Successfully sanity-checked the conglomerator")

	return proof, cong.Wiop, nil
}
