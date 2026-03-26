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
	"github.com/consensys/linea-monorepo/prover/utils/signal"
	"github.com/consensys/linea-monorepo/prover/utils/test_utils"
	"github.com/consensys/linea-monorepo/prover/zkevm"
	"github.com/sirupsen/logrus"
)

// runConglomerationWizardTest runs a full conglomeration pipeline for a single
// DistributedTestCase: distribution, segment compilation, conglomeration, proving
// and proof aggregation.
//
// numRow must match the numRow embedded in tc; it is used to derive the
// discoverer target weight so that at least two segments are created (required
// for the conglomeration binary tree to have something to aggregate).
func runConglomerationWizardTest(t *testing.T, tc DistributedTestCase, numRow int) {
	t.Helper()

	var (
		comp = wizard.Compile(func(build *wizard.Builder) {
			tc.Define(build.CompiledIOP)
		})

		disc = &distributed.StandardModuleDiscoverer{
			TargetWeight: 3 * numRow / 2,
			Advices:      tc.Advices(),
		}

		distWizard = distributed.DistributeWizard(comp, disc).
				CompileSegments(testCompilationParams).
				Conglomerate(testCompilationParams)

		runtimeBoot             = wizard.RunProver(distWizard.Bootstrapper, tc.Assign, false)
		witnessGLs, witnessLPPs = distributed.SegmentRuntime(
			runtimeBoot,
			distWizard.Disc,
			distWizard.BlueprintGLs,
			distWizard.BlueprintLPPs,
			distWizard.VerificationKeyMerkleTree.GetRoot(),
		)

		glProofs         = runProverGLs(t, distWizard, witnessGLs)
		sharedRandomness = distributed.GetSharedRandomnessFromSegmentProofs(glProofs)
		lppProofs        = runProverLPPs(t, distWizard, sharedRandomness, witnessLPPs)
	)

	runConglomerationProver(
		&distWizard.VerificationKeyMerkleTree,
		distWizard.CompiledConglomeration,
		glProofs,
		lppProofs,
	)
}

// TestConglomerationBasic generates a conglomeration proof for each of the
// standard DistributedTestCase types and checks that the proof is valid.
func TestConglomerationBasic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping heavy test in short mode")
	}
	signal.RegisterStackTraceDumpHandler()

	const numRow = 1 << 5

	testCases := []DistributedTestCase{
		&LookupTestCase{numRow: numRow},
		&ProjectionTestCase{numRow: numRow},
		&PermutationTestCase{numRow: numRow},
	}

	for _, tc := range testCases {
		t.Run(tc.Name(), func(t *testing.T) {
			runConglomerationWizardTest(t, tc, numRow)
		})
	}
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
		runtimeBoot = wizard.RunProver(distWizard.Bootstrapper, z.GetMainProverStep(witness), false)
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

func TestConglomerationProverSmallField(t *testing.T) {

	t.Skipf("the test is a development/debug/integration test. It is not needed for CI")

	cfgFilePath := "../../config/config-mainnet-limitless.toml"
	cfg, cfgErr := config.NewConfigFromFileUnchecked(cfgFilePath)

	if cfgErr != nil {
		t.Fatalf("could not read the config file: err=%v", cfgErr)
	}

	t.Logf("loaded config: %++v", cfg)

	limitlessZkEVM := zkevm.NewLimitlessZkEVM(cfg)
	logrus.Printf("ZkEVM loaded: %++v", limitlessZkEVM)
}
