package limitless

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"

	"github.com/consensys/linea-monorepo/prover/backend/execution"
	"github.com/consensys/linea-monorepo/prover/circuits"
	execCirc "github.com/consensys/linea-monorepo/prover/circuits/execution"
	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/exit"
	"github.com/consensys/linea-monorepo/prover/zkevm"
	"github.com/consensys/linea-monorepo/prover/zkevm/arithmetization"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

// ProveOnTheFly runs the limitless prover without pre-computed disk assets.
// It compiles everything in memory, keeps witnesses in memory, and pipelines
// compilation with bootstrapper proving to hide latency.
//
// Architecture:
//
//	Step 1: BuildZkEVM + DistributeWizard (~1.5 min)
//	Step 2 (parallel):
//	  Thread A: CompileSegments + Conglomerate (~6-12 min)
//	  Thread B: RunBootstrapper prover (~2-3 min with optimizations)
//	Step 3: Wait for both → SegmentRuntime → GL → LPP → Conglomeration → OuterProof
func ProveOnTheFly(cfg *config.Config, req *execution.Request) (*execution.Response, error) {

	exit.SetIssueHandlingMode(exit.ExitOnUnsatisfiedConstraint | exit.ExitOnMissingTraceFile)

	var (
		out     = execution.CraftProverOutput(cfg, req)
		witness = execution.NewWitness(cfg, req, &out)
	)

	// =====================================================================
	// Step 1: Build ZkEVM + Distribute (~1.5 min)
	// =====================================================================

	var (
		traceLimits  = cfg.TracesLimits
		targetWeight = getEnvPositiveInt("LIMITLESS_TARGET_WEIGHT_LOG2", 28)
	)

	zkevmObj := zkevm.FullZKEVMWithSuite(&traceLimits, cfg, zkevm.CompilationSuite{}, nil)
	disc := &distributed.StandardModuleDiscoverer{
		TargetWeight: 1 << targetWeight,
		Advices:      zkevm.DiscoveryAdvices(zkevmObj),
	}

	dw := distributed.DistributeWizardWithOptions(
		zkevmObj.InitialCompiledIOP,
		disc,
		distributed.DistributeWizardOptions{
			BuildDebugModules: false,
		},
	)
	logrus.Infof("Build+distribute done. %d modules discovered. TargetWeight=1<<%d", len(dw.ModuleNames), targetWeight)

	// Override compatibility check setting
	zkevmObj.Arithmetization.Settings.IgnoreCompatibilityCheck = &cfg.Execution.IgnoreCompatibilityCheck

	// Start setup loading early (overlaps with compilation + bootstrapper)
	var (
		setup       circuits.Setup
		errSetup    error
		chSetupDone = make(chan struct{})
	)
	go func() {
		setup, errSetup = circuits.LoadSetup(cfg, circuits.ExecutionLimitlessCircuitID)
		close(chSetupDone)
	}()

	// Pre-start trace file decompression and parsing in background.
	preReadCh := make(chan arithmetization.PreReadResult, 1)
	go func() {
		preReadCh <- arithmetization.PreReadTrace(witness.ZkEVM.ExecTracesFPath)
	}()

	// =====================================================================
	// Step 2: Overlap compilation with bootstrapper
	//
	// Thread A: CompileSegments + Conglomerate (~6-12 min)
	//   Produces: CompiledGLs, CompiledLPPs, CompiledConglomeration, VK Merkle Tree
	//
	// Thread B: RunBootstrapper prover (~2-3 min)
	//   Produces: ProverRuntime with all assigned columns
	//   Only needs: dw.Bootstrapper, zkevmObj, dw.Disc (NOT compiled segments)
	// =====================================================================

	compileCh := make(chan error, 1)
	// Thread A: compilation pipeline
	go func() {
		dw.CompileSegments(zkevm.LimitlessCompilationParams).Conglomerate(zkevm.LimitlessCompilationParams)

		// Copy PI metadata needed for outer circuit
		dw.CompiledConglomeration.RecursionCompBLS.
			ExtraData[publicInput.PublicInputExtractorMetadata] = zkevmObj.
			InitialCompiledIOP.ExtraData[publicInput.PublicInputExtractorMetadata]

		compileCh <- nil
	}()

	// Thread B: bootstrapper prover (runs concurrently with compilation)

	runtimeBoot := runBootstrapperInMemory(cfg, dw.Bootstrapper, zkevmObj, witness.ZkEVM, dw.Disc, preReadCh)

	// Wait for compilation to finish (may already be done if bootstrapper was slower)
	if compErr := <-compileCh; compErr != nil {
		return nil, fmt.Errorf("compilation failed: %w", compErr)
	}

	// =====================================================================
	// Step 3: Segment runtime (in-memory witnesses)
	// =====================================================================

	mt := &dw.VerificationKeyMerkleTree

	witnessGLs, witnessLPPs := distributed.SegmentRuntime(
		runtimeBoot,
		dw.Disc,
		dw.BlueprintGLs,
		dw.BlueprintLPPs,
		mt.GetRoot(),
	)

	numGL := len(witnessGLs)
	numLPP := len(witnessLPPs)

	glModuleNames := make([]string, numGL)
	for i, w := range witnessGLs {
		glModuleNames[i] = string(w.ModuleName)
	}

	// Release bootstrapper runtime and zkEVM (witnesses are deep-copied by SegmentRuntime)
	zkevmObj = nil

	// =====================================================================
	// Step 4+: GL/LPP proving + Conglomeration + Outer proof
	// =====================================================================
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	type congResult struct {
		proof *distributed.SegmentProof
		err   error
	}

	var (
		totalProofs = numGL + numLPP
		proofGLs    = make([]*distributed.SegmentProof, numGL)
		glErrGroup  = &errgroup.Group{}
		lppErrGroup = &errgroup.Group{}
		proofStream = make(chan *distributed.SegmentProof, totalProofs)
		resultCh    = make(chan congResult, 1)
		cong        = dw.CompiledConglomeration
	)

	// Launch conglomeration pipeline
	go func() {
		proof, err := RunConglomerationHierarchical(ctx, mt, cong, proofStream, totalProofs)
		resultCh <- congResult{proof: proof, err: err}
	}()

	// Track compiled circuit usage for early release
	glModIndices := make([]int, numGL)
	for i, w := range witnessGLs {
		glModIndices[i] = w.ModuleIndex
	}
	lppModIndices := make([]int, numLPP)
	for i, w := range witnessLPPs {
		lppModIndices[i] = w.ModuleIndex
	}
	compiledGLUsage := newCompiledUsageTracker(dw.CompiledGLs, glModIndices)
	compiledLPPUsage := newCompiledUsageTracker(dw.CompiledLPPs, lppModIndices)

	// GL proving — use round-robin ordering from main
	glErrGroup.SetLimit(numConcurrentSubProverJobs)

	glOrder := buildRoundRobinOrder(glModuleNames)

	var glCompleted atomic.Int64
	var glGCMu sync.Mutex

	for _, i := range glOrder {
		glErrGroup.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			var (
				jobErr  error
				proofGL *distributed.SegmentProof
				modName string
			)

			func() {
				defer func() {
					if r := recover(); r != nil {
						jobErr = fmt.Errorf("GL prover for witness index=%v panicked: %v", i, r)
						logrus.Error(jobErr)
						debug.PrintStack()
					}
				}()

				wit := witnessGLs[i]
				modName = string(wit.ModuleName)
				compiled := compiledGLUsage.get(i)
				if compiled == nil {
					jobErr = fmt.Errorf("no compiled GL circuit for witness index=%d module=%s", i, modName)
					return
				}
				defer compiledGLUsage.done(i)

				proofGL = compiled.ProveSegmentKoala(wit).ClearRuntime()
				witnessGLs[i] = nil
			}()

			if jobErr != nil {
				return jobErr
			}

			proofGLs[i] = proofGL
			select {
			case proofStream <- proofGL:
			case <-ctx.Done():
				return ctx.Err()
			}

			n := glCompleted.Add(1)
			// GC every 4 completed jobs to balance memory pressure vs throughput.
			// Previous experiments showed per-job GC is too aggressive.
			if int(n) < numGL && n%4 == 0 {
				if glGCMu.TryLock() {
					logrus.Infof("GL inter-job GC after %d/%d jobs", n, numGL)
					debug.FreeOSMemory()
					glGCMu.Unlock()
				}
			}

			return nil
		})
	}

	if err := glErrGroup.Wait(); err != nil {
		cancel()
		res := <-resultCh
		if res.err != nil {
			return nil, fmt.Errorf("GL error: %w (aggregator error: %v)", err, res.err)
		}
		return nil, fmt.Errorf("GL error: %w", err)
	}

	// Shared randomness
	sharedRandomness := distributed.GetSharedRandomnessFromSegmentProofs(proofGLs)
	for i := range proofGLs {
		proofGLs[i] = nil
	}
	proofGLs = nil

	// GC between phases
	debug.FreeOSMemory()

	// LPP proving
	lppErrGroup.SetLimit(numConcurrentSubProverJobs)

	lppOrder := make([]int, numLPP)
	for i := range numLPP {
		lppOrder[i] = numLPP - 1 - i
	}

	for _, i := range lppOrder {
		lppErrGroup.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			var (
				jobErr   error
				proofLPP *distributed.SegmentProof
				modName  string
			)

			func() {
				defer func() {
					if r := recover(); r != nil {
						jobErr = fmt.Errorf("LPP prover for witness index=%v panicked: %v", i, r)
						logrus.Error(jobErr)
						debug.PrintStack()
					}
				}()

				wit := witnessLPPs[i]
				modName = string(wit.ModuleName)
				wit.InitialFiatShamirState = sharedRandomness

				compiled := compiledLPPUsage.get(i)
				if compiled == nil {
					jobErr = fmt.Errorf("no compiled LPP circuit for witness index=%d module=%s", i, modName)
					return
				}
				defer compiledLPPUsage.done(i)

				proofLPP = compiled.ProveSegmentKoala(wit).ClearRuntime()
				witnessLPPs[i] = nil
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

	if err := lppErrGroup.Wait(); err != nil {
		cancel()
		res := <-resultCh
		if res.err != nil {
			return nil, fmt.Errorf("LPP error: %w (aggregator error: %v)", err, res.err)
		}
		return nil, fmt.Errorf("LPP error: %w", err)
	}

	// Post-LPP GC
	debug.FreeOSMemory()

	close(proofStream)

	// Wait for conglomeration
	res := <-resultCh
	if res.err != nil {
		return nil, fmt.Errorf("conglomeration failed: %w", res.err)
	}

	congFinalproof := res.proof

	// Wait for setup (started during build phase; should be done by now)
	<-chSetupDone
	if errSetup != nil {
		utils.Panic("could not load setup: %v", errSetup)
	}

	// Outer proof
	out.Proof = execCirc.MakeProof(
		&cfg.TracesLimits,
		setup,
		cong.RecursionCompBLS,
		congFinalproof.GetOuterProofInput(),
		*witness.FuncInp,
		witness.ZkEVM.ExecData,
	)

	out.VerifyingKeyShaSum = setup.VerifyingKeyDigest()

	return &out, nil
}

// runBootstrapperInMemory runs the bootstrapper with in-memory assets (no disk IO).
// It uses pre-read trace if available.
func runBootstrapperInMemory(cfg *config.Config, bootstrapper *wizard.CompiledIOP, zkevmObj *zkevm.ZkEvm, zkevmWitness *zkevm.Witness, disc *distributed.StandardModuleDiscoverer, preReadCh <-chan arithmetization.PreReadResult) *wizard.ProverRuntime {
	// Wait for the pre-read result
	preReadResult := <-preReadCh

	var (
		scalingFactor = 1
		runtimeBoot   *wizard.ProverRuntime
	)

	for runtimeBoot == nil {
		logrus.Infof("Trying to bootstrap with a scaling of %v", scalingFactor)

		replayCh := make(chan arithmetization.PreReadResult, 1)
		if scalingFactor == 1 {
			replayCh <- preReadResult
		} else {
			clone := preReadResult
			clone.RawTrace = preReadResult.RawTrace.Clone()
			replayCh <- clone
		}

		func() {
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
				runtimeBoot = wizard.RunProver(
					bootstrapper,
					zkevmObj.GetMainProverStepWithPreRead(zkevmWitness, replayCh),
					false,
				)
				return
			}

			scaledUpBootstrapper, scaledUpZkEVM := zkevm.GetScaledUpBootstrapper(
				cfg, disc, scalingFactor,
			)

			runtimeBoot = wizard.RunProver(
				scaledUpBootstrapper,
				scaledUpZkEVM.GetMainProverStepWithPreRead(zkevmWitness, replayCh),
				false,
			)
		}()
	}

	return runtimeBoot
}

// compiledUsageTracker tracks how many segments reference each compiled circuit
// and releases it when the last segment is done.
type compiledUsageTracker struct {
	mu        sync.Mutex
	compiled  []*distributed.RecursedSegmentCompilation
	remaining map[int]int // moduleIndex -> remaining segment count
	modIndex  []int       // witnessIndex -> moduleIndex
}

func newCompiledUsageTracker(compiled []*distributed.RecursedSegmentCompilation, moduleIndices []int) *compiledUsageTracker {
	tracker := &compiledUsageTracker{
		compiled:  compiled,
		remaining: make(map[int]int),
		modIndex:  moduleIndices,
	}
	for _, modIdx := range moduleIndices {
		tracker.remaining[modIdx]++
	}
	return tracker
}

func (t *compiledUsageTracker) get(witnessIndex int) *distributed.RecursedSegmentCompilation {
	t.mu.Lock()
	defer t.mu.Unlock()
	modIdx := t.modIndex[witnessIndex]
	return t.compiled[modIdx]
}

func (t *compiledUsageTracker) done(witnessIndex int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	modIdx := t.modIndex[witnessIndex]
	t.remaining[modIdx]--
	if t.remaining[modIdx] <= 0 {
		t.compiled[modIdx] = nil
	}
}
