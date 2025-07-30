package distributed_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/consensys/linea-monorepo/prover/backend/execution"
	"github.com/consensys/linea-monorepo/prover/backend/files"
	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/recursion"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils/test_utils"
	"github.com/consensys/linea-monorepo/prover/zkevm"
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
				CompileSegments().
				Conglomerate(20)

		runtimeBoot             = wizard.RunProver(distWizard.Bootstrapper, tc.Assign)
		witnessGLs, witnessLPPs = distributed.SegmentRuntime(
			runtimeBoot,
			distWizard.Disc,
			distWizard.BlueprintGLs,
			distWizard.BlueprintLPPs,
		)
		runGLs = runProverGLs(t, distWizard, witnessGLs)
	)

	for i := range runGLs {
		t.Logf("sanity-checking runGLs[%d]\n", i)
		sanityCheckConglomeration(t, distWizard.CompiledConglomeration, runGLs[i])
	}

	var (
		sharedRandomness = getSharedRandomness(runGLs)
		runLPPs          = runProverLPPs(t, distWizard, sharedRandomness, witnessLPPs)
	)

	for i := range runLPPs {
		t.Logf("sanity-checking runLPPs[%d]\n", i)
		sanityCheckConglomeration(t, distWizard.CompiledConglomeration, runLPPs[i])
	}

	runConglomerationProver(t, distWizard.CompiledConglomeration, runGLs, runLPPs)
}

// TestConglomeration generates a conglomeration proof and checks if it is valid
func TestConglomeration(t *testing.T) {

	t.Skipf("the test is a development/debug/integration test. It is not needed for CI")

	var (
		z    = zkevm.GetTestZkEVM()
		disc = &distributed.StandardModuleDiscoverer{
			TargetWeight: 1 << 28,
			Affinities:   zkevm.GetAffinities(z),
			Predivision:  1,
		}

		// This tests the compilation of the compiled-IOP
		distWizard = distributed.DistributeWizard(z.WizardIOP, disc).
				CompileSegments().
				Conglomerate(20)
	)

	var (
		reqFile      = files.MustRead("/home/ubuntu/beta-v2-rc11/10556002-10556002-etv0.2.0-stv2.2.2-getZkProof.json")
		cfgFilePath  = "/home/ubuntu/zkevm-monorepo/prover/config/config-sepolia-full.toml"
		req          = &execution.Request{}
		reqDecodeErr = json.NewDecoder(reqFile).Decode(req)
		cfg, cfgErr  = config.NewConfigFromFileUnchecked(cfgFilePath)
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
		)
		runGLs = runProverGLs(t, distWizard, witnessGLs)
	)

	for i := range runGLs {
		t.Logf("sanity-checking runGLs[%d]\n", i)
		sanityCheckConglomeration(t, distWizard.CompiledConglomeration, runGLs[i])
	}

	var (
		sharedRandomness = getSharedRandomness(runGLs)
		runLPPs          = runProverLPPs(t, distWizard, sharedRandomness, witnessLPPs)
	)

	for i := range runLPPs {
		t.Logf("sanity-checking runLPPs[%d]\n", i)
		sanityCheckConglomeration(t, distWizard.CompiledConglomeration, runLPPs[i])
	}

	runConglomerationProver(t, distWizard.CompiledConglomeration, runGLs, runLPPs)
}

// getSharedRandomness computes the shared randomnesses from the runtime
func getSharedRandomness(runs []*wizard.ProverRuntime) field.Element {
	witnesses := make([]recursion.Witness, len(runs))
	for i := range runs {
		witnesses[i] = recursion.ExtractWitness(runs[i])
	}

	comps := make([]*wizard.CompiledIOP, len(runs))
	for i := range runs {
		comps[i] = runs[i].Spec
	}

	return distributed.GetSharedRandomnessFromWitnesses(comps, witnesses)
}

// Sanity-check for conglomeration compilation.
func sanityCheckConglomeration(t *testing.T, cong *distributed.ConglomeratorCompilation, run *wizard.ProverRuntime) {

	t.Logf("sanity-check for conglomeration")
	stopRound := recursion.VortexQueryRound(cong.ModuleGLIops[0])
	err := wizard.VerifyUntilRound(cong.ModuleGLIops[0], run.ExtractProof(), stopRound+1)

	if err != nil {
		t.Fatalf("could not verify proof: %v", err)
	}
}

// This function runs a prover for a conglomerator compilation. It takes in a ConglomeratorCompilation
// object and two slices of ProverRuntime objects, runGLs and runLPPs. It extracts witnesses from
// these runtimes, then uses the ConglomeratorCompilation object to prove the conglomerator,
// logging the start and end times of the proof process.
func runConglomerationProver(t *testing.T, cong *distributed.ConglomeratorCompilation, runGLs, runLPPs []*wizard.ProverRuntime) {

	var (
		witLPPs = make([]recursion.Witness, len(runLPPs))
		witGLs  = make([]recursion.Witness, len(runGLs))
	)

	for i := range runLPPs {
		witLPPs[i] = recursion.ExtractWitness(runLPPs[i])
	}

	for i := range runGLs {
		witGLs[i] = recursion.ExtractWitness(runGLs[i])
	}

	t.Logf("[%v] Starting to prove conglomerator\n", time.Now())
	proof := cong.Prove(witGLs, witLPPs)
	t.Logf("[%v] Finished proving conglomerator\n", time.Now())

	t.Logf("[%v] start sanity-checking proof\n", time.Now())

	err := wizard.Verify(cong.Wiop, proof)
	if err != nil {
		t.Fatalf("could not verify proof: %v", err)
	}

	t.Logf("[%v] done sanity-checking proof\n", time.Now())
}
