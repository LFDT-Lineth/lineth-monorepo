package distributed_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/consensys/linea-monorepo/prover/backend/execution"
	"github.com/consensys/linea-monorepo/prover/backend/files"
	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils/test_utils"
	"github.com/consensys/linea-monorepo/prover/zkevm"
	"github.com/sirupsen/logrus"
)

// TestConglomerationBasic generates a conglomeration proof and checks if it is valid
func TestConglomerationBasic(t *testing.T) {
	t.Skipf("the test is a development/debug/integration test. It is not needed for CI")
	var (
		numRow = 1 << 10
		tc     = DistributeTestCase{numRow: numRow}
		disc   = &distributed.StandardModuleDiscoverer{
			TargetWeight: 3 * numRow / 2,
			Predivision:  1,
		}
		comp = wizard.Compile(func(build *wizard.Builder) {
			tc.Define(build.CompiledIOP)
		})

		// This tests the compilation of the compiled-IOP
		distWizard = distributed.DistributeWizard(comp, disc).
				CompileSegments(zkevm.LimitlessCompilationParams).
				Conglomerate(zkevm.LimitlessCompilationParams)

		runtimeBoot             = wizard.RunProver(distWizard.Bootstrapper, tc.Assign)
		witnessGLs, witnessLPPs = distributed.SegmentRuntime(
			runtimeBoot,
			distWizard.Disc,
			distWizard.BlueprintGLs,
			distWizard.BlueprintLPPs,
			distWizard.VerificationKeyMerkleTree.GetRoot(),
		)
		glProofs         = runProverGLs(t, distWizard, witnessGLs)
		sharedRandomness = distributed.GetSharedRandomnessFromSegmentProofs(glProofs)
		runLPPs          = runProverLPPs(t, distWizard, sharedRandomness, witnessLPPs)
	)

	runConglomerationProver(
		&distWizard.VerificationKeyMerkleTree,
		distWizard.CompiledConglomeration,
		glProofs,
		runLPPs,
	)
}

// TestConglomerationProverDebug
func TestConglomerationProverDebug(t *testing.T) {
	t.Skipf("the test is a development/debug/integration test. It is not needed for CI")
	var (
		reqFile        = files.MustRead("/home/ubuntu/mainnet-beta-v2-5.1.3/prover-execution/requests/20106872-20106937-etv0.2.0-stv2.3.0-getZkProof.json")
		cfgFilePath    = "/home/ubuntu/zkevm-monorepo/prover/config/config-mainnet-limitless.toml"
		req            = &execution.Request{}
		reqDecodeErr   = json.NewDecoder(reqFile).Decode(req)
		cfg, cfgErr    = config.NewConfigFromFileUnchecked(cfgFilePath)
		limitlessZkEVM = zkevm.NewLimitlessDebugZkEVM(cfg)
		_, witness     = test_utils.GetZkevmWitness(req, cfg)
	)

	if reqDecodeErr != nil {
		t.Fatalf("could not read the request file: %v", reqDecodeErr)
	}

	if cfgErr != nil {
		t.Fatalf("could not read the config file: err=%v, cfg=%++v", cfgErr, cfg)
	}

	limitlessZkEVM.RunDebug(cfg, witness)

}

// TestConglomeration generates a conglomeration proof and checks if it is valid
func TestConglomerationProverFile(t *testing.T) {
	t.Skipf("the test is a development/debug/integration test. It is not needed for CI")
	var (
		reqFile        = files.MustRead("/home/ubuntu/mainnet-beta-v2-5.1.3/prover-execution/requests/20106872-20106937-etv0.2.0-stv2.3.0-getZkProof.json")
		cfgFilePath    = "/home/ubuntu/zkevm-monorepo/prover/config/config-mainnet-limitless.toml"
		req            = &execution.Request{}
		reqDecodeErr   = json.NewDecoder(reqFile).Decode(req)
		cfg, cfgErr    = config.NewConfigFromFileUnchecked(cfgFilePath)
		limitlessZkEVM = zkevm.NewLimitlessZkEVM(cfg)
		z              = limitlessZkEVM.Zkevm
		distWizard     = limitlessZkEVM.DistWizard
	)

	if reqDecodeErr != nil {
		t.Fatalf("could not read the request file: %v", reqDecodeErr)
	}

	if cfgErr != nil {
		t.Fatalf("could not read the config file: err=%v, cfg=%++v", cfgErr, cfg)
	}

	t.Logf("loaded config: %++v", cfg)

	t.Logf("[%v] running the bootstrapper\n", time.Now())

	var (
		_, witness  = test_utils.GetZkevmWitness(req, cfg)
		runtimeBoot = wizard.RunProver(distWizard.Bootstrapper, z.GetMainProverStep(witness))
	)

	t.Logf("[%v] done running the bootstrapper\n", time.Now())

	var (
		witnessGLs, witnessLPPs = distributed.SegmentRuntime(
			runtimeBoot,
			distWizard.Disc,
			distWizard.BlueprintGLs,
			distWizard.BlueprintLPPs,
			distWizard.VerificationKeyMerkleTree.GetRoot(),
		)
		glProofs         = runProverGLs(t, distWizard, witnessGLs)
		sharedRandomness = distributed.GetSharedRandomnessFromSegmentProofs(glProofs)
		runLPPs          = runProverLPPs(t, distWizard, sharedRandomness, witnessLPPs)
	)

	runConglomerationProver(
		&distWizard.VerificationKeyMerkleTree,
		distWizard.CompiledConglomeration,
		glProofs,
		runLPPs,
	)
}

// This function runs a prover for a conglomerator compilation. It takes in a
// ConglomeratorCompilation object and two slices of ProverRuntime objects,
// runGLs and runLPPs. It extracts witnesses from these runtimes, then uses the
// ConglomeratorCompilation object to prove the conglomerator, logging the start
// and end times of the proof process.
func runConglomerationProver(
	mt *distributed.VerificationKeyMerkleTree,
	cong *distributed.RecursedSegmentCompilation,
	runGLs, runLPPs []*distributed.SegmentProof,
) *distributed.SegmentProof {

	// The channel is used as a FIFO queue to store the remaining proofs to be
	// aggregated.
	var (
		remainingProofs = make(chan *distributed.SegmentProof, len(runGLs)+len(runLPPs))
	)

	// This populates the queue
	for i := range runGLs {
		remainingProofs <- runGLs[i]
	}

	for i := range runLPPs {
		remainingProofs <- runLPPs[i]
	}

	// TryPopQueue attempts to consume a proof from the queue or return false
	// if the queue is empty.
	tryPopQueue := func() (*distributed.SegmentProof, bool) {
		select {
		case proof := <-remainingProofs:
			return proof, true
		default:
			return nil, false
		}
	}

	// This is the actual proof aggregation loop.
	for {
		a, ok := tryPopQueue()
		if !ok {
			panic("the queue cannot be empty here")
		}

		// If b cannot be found, it means that the queue contained only a single
		// proof to aggregate which means it was the result of the function.
		b, ok := tryPopQueue()
		if !ok {
			return a
		}

		logrus.Infof("AGGREGATING PROOF, remaining %v\n", len(remainingProofs))

		new := cong.ProveSegment(&distributed.ModuleWitnessConglo{
			SegmentProofs:             []distributed.SegmentProof{*a, *b},
			VerificationKeyMerkleTree: *mt,
		})

		logrus.Infof("AGGREGATED PROOFS\n")

		remainingProofs <- new
	}
}
