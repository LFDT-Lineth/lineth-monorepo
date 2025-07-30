package distributed_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/consensys/linea-monorepo/prover/backend/execution"
	"github.com/consensys/linea-monorepo/prover/backend/files"
	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/logdata"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils/test_utils"
	"github.com/consensys/linea-monorepo/prover/zkevm"
)

// TestDistributedWizard attempts to compiler the wizard distribution.
func TestDistributedWizard(t *testing.T) {

	t.SkipNow()

	var (
		z          = zkevm.GetTestZkEVM()
		affinities = zkevm.GetAffinities(z)
		discoverer = &distributed.StandardModuleDiscoverer{
			TargetWeight: 1 << 28,
			Affinities:   affinities,
			Predivision:  1,
		}
	)

	// This dumps a CSV with the list of all the columns in the original wizard
	logdata.GenCSV(
		files.MustOverwrite("./base-data/main.csv"),
		logdata.IncludeAllFilter,
	)(z.WizardIOP)

	distributed := distributed.DistributeWizard(z.WizardIOP, discoverer)

	for i, modGL := range distributed.GLs {

		logdata.GenCSV(
			files.MustOverwrite(fmt.Sprintf("./module-data/module-%v-gl.csv", i)),
			logdata.IncludeAllFilter,
		)(modGL.Wiop)
	}

	for i, modLPP := range distributed.LPPs {

		logdata.GenCSV(
			files.MustOverwrite(fmt.Sprintf("./module-data/module-%v-lpp.csv", i)),
			logdata.IncludeAllFilter,
		)(modLPP.Wiop)
	}
}

// TestDistributeWizard attempts to run and compile the distributed protocol in
// dummy mode. Meaning without actual compilation.
func TestDistributedWizardLogic(t *testing.T) {

	t.Skipf("the test is a development/debug/integration test. It is not needed for CI")

	var (
		// #nosec G404 --we don't need a cryptographic RNG for testing purpose
		// rng              = rand.New(utils.NewRandSource(0))
		// sharedRandomness = field.PseudoRand(rng)
		z    = zkevm.GetTestZkEVM()
		disc = &distributed.StandardModuleDiscoverer{
			TargetWeight: 1 << 28,
			Affinities:   zkevm.GetAffinities(z),
			Predivision:  1,
		}

		// This tests the compilation of the compiled-IOP
		distWizard = distributed.DistributeWizard(z.WizardIOP, disc)
	)

	// This applies the dummy.Compiler to all parts of the distributed wizard.
	for i := range distWizard.GLs {
		dummy.CompileAtProverLvl()(distWizard.GLs[i].Wiop)
	}

	for i := range distWizard.LPPs {
		dummy.CompileAtProverLvl()(distWizard.LPPs[i].Wiop)
	}

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

	t.Logf("Checking the initial bootstrapper - wizard")
	var (
		_, witness  = test_utils.GetZkevmWitness(req, cfg)
		runtimeBoot = wizard.RunProver(distWizard.Bootstrapper, z.GetMainProverStep(witness))
		proof       = runtimeBoot.ExtractProof()
		verBootErr  = wizard.Verify(distWizard.Bootstrapper, proof)
	)
	t.Logf("Checking the initial bootstrapper - wizard")

	if verBootErr != nil {
		t.Fatalf("Bootstrapper failed because: %v", verBootErr)
	}

	var (
		allGrandProduct     = field.NewElement(1)
		allLogDerivativeSum = field.Element{}
		allHornerSum        = field.Element{}
		prevGlobalSent      = field.Element{}
		prevHornerN1Hash    = field.Element{}
	)

	witnessGLs, witnessLPPs := distributed.SegmentRuntime(
		runtimeBoot,
		distWizard.Disc,
		distWizard.BlueprintGLs,
		distWizard.BlueprintLPPs,
	)

	for i := range witnessGLs {

		var (
			witnessGL   = witnessGLs[i]
			moduleIndex = witnessGLs[i].ModuleIndex
			moduleName  = witnessGLs[i].ModuleName
			moduleGL    *distributed.ModuleGL
		)

		t.Logf("segment(total)=%v module=%v segment.index=%v", i, witnessGL.ModuleName, witnessGL.ModuleIndex)

		for k := range distWizard.ModuleNames {
			if distWizard.ModuleNames[k] != witnessGLs[i].ModuleName {
				continue
			}

			moduleGL = distWizard.GLs[k]
			break
		}

		if moduleGL == nil {
			t.Fatalf("module does not exists")
		}

		var (
			proverRunGL        = wizard.RunProver(moduleGL.Wiop, moduleGL.GetMainProverStep(witnessGLs[i]))
			proofGL            = proverRunGL.ExtractProof()
			verGLRun, verGLErr = wizard.VerifyWithRuntime(moduleGL.Wiop, proofGL)
		)

		if verGLErr != nil {
			t.Errorf("verifier failed for segment %v, reason=%v", i, verGLErr)
		}

		var (
			errMsg         = fmt.Sprintf("segment=%v, moduleName=%v, segment-index=%v", i, moduleName, moduleIndex)
			globalReceived = verGLRun.GetPublicInput(distributed.GlobalReceiverPublicInput)
			globalSent     = verGLRun.GetPublicInput(distributed.GlobalSenderPublicInput)
			isFirst        = verGLRun.GetPublicInput(distributed.IsFirstPublicInput)
			isLast         = verGLRun.GetPublicInput(distributed.IsLastPublicInput)
			shouldBeFirst  = i == 0 || witnessGLs[i].ModuleName != witnessGLs[i-1].ModuleName
			shouldBeLast   = i == len(witnessGLs)-1 || witnessGLs[i].ModuleName != witnessGLs[i+1].ModuleName
		)

		if isFirst.IsOne() != shouldBeFirst {
			t.Error("isFirst has unexpected values: " + errMsg)
		}

		if isLast.IsOne() != shouldBeLast {
			t.Error("isLast has unexpected values: " + errMsg)
		}

		if !shouldBeFirst && globalReceived != prevGlobalSent {
			t.Error("global-received does not match: " + errMsg)
		}

		prevGlobalSent = globalSent
	}

	for i := range witnessLPPs {

		var (
			witnessLPP  = witnessLPPs[i]
			moduleIndex = witnessLPPs[i].ModuleIndex
			moduleNames = witnessLPPs[i].ModuleName
			moduleLPP   *distributed.ModuleLPP
		)

		witnessLPP.InitialFiatShamirState = field.NewFromString("6861409415040334196327676756394403519979367936044773323994693747743991500772")

		t.Logf("segment(total)=%v module=%v segment.index=%v", i, witnessLPP.ModuleName, witnessLPP.ModuleIndex)

		for k := range distWizard.LPPs {
			if !reflect.DeepEqual(distWizard.LPPs[k].ModuleNames(), moduleNames) {
				continue
			}

			moduleLPP = distWizard.LPPs[k]
			break
		}

		if moduleLPP == nil {
			t.Fatalf("module does not exists")
		}

		var (
			proverRunLPP         = wizard.RunProver(moduleLPP.Wiop, moduleLPP.GetMainProverStep(witnessLPP))
			proofLPP             = proverRunLPP.ExtractProof()
			verLPPRun, verLPPErr = wizard.VerifyWithRuntime(moduleLPP.Wiop, proofLPP)
		)

		if verLPPErr != nil {
			t.Errorf("verifier failed for segment %v, reason=%v", i, verLPPErr)
		}

		var (
			errMsg           = fmt.Sprintf("segment=%v, moduleName=%v, segment-index=%v", i, moduleNames, moduleIndex)
			logDerivativeSum = verLPPRun.GetPublicInput(distributed.LogDerivativeSumPublicInput)
			grandProduct     = verLPPRun.GetPublicInput(distributed.GrandProductPublicInput)
			hornerSum        = verLPPRun.GetPublicInput(distributed.HornerPublicInput)
			hornerN0Hash     = verLPPRun.GetPublicInput(distributed.HornerN0HashPublicInput)
			hornerN1Hash     = verLPPRun.GetPublicInput(distributed.HornerN1HashPublicInput)
			shouldBeFirst    = i == 0 || !reflect.DeepEqual(witnessLPPs[i].ModuleName, witnessLPPs[i-1].ModuleName)
		)

		if !shouldBeFirst && hornerN0Hash != prevHornerN1Hash {
			t.Error("horner-n0-hash mismatch: " + errMsg)
		}

		t.Logf("log-derivative-sum=%v grand-product=%v horner-sum=%v", logDerivativeSum.String(), grandProduct.String(), hornerSum.String())

		prevHornerN1Hash = hornerN1Hash
		allGrandProduct.Mul(&allGrandProduct, &grandProduct)
		allHornerSum.Add(&allHornerSum, &hornerSum)
		allLogDerivativeSum.Add(&allLogDerivativeSum, &logDerivativeSum)
	}

	if !allGrandProduct.IsOne() {
		t.Errorf("grand-product does not cancel")
	}

	if !allHornerSum.IsZero() {
		t.Errorf("horner does not cancel")
	}

	if !allLogDerivativeSum.IsZero() {
		t.Errorf("log-derivative-sum does not cancel. Has %v", allLogDerivativeSum.String())
	}
}

// TestBenchDistributedWizard runs the distributed wizard will all the compilations
func TestBenchDistributedWizard(t *testing.T) {

	t.Skipf("the test is a development/debug/integration test. It is not needed for CI")

	var (
		z    = zkevm.GetTestZkEVM()
		disc = &distributed.StandardModuleDiscoverer{
			TargetWeight: 1 << 28,
			Affinities:   zkevm.GetAffinities(z),
			Predivision:  1,
		}

		// This tests the compilation of the compiled-IOP
		distWizard = distributed.DistributeWizard(z.WizardIOP, disc).CompileSegments()
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
		runGLs           = runProverGLs(t, distWizard, witnessGLs)
		sharedRandomness = getSharedRandomness(runGLs)
		_                = runProverLPPs(t, distWizard, sharedRandomness, witnessLPPs)
	)
}

// DistributeTestCase is an implementation of the testcase interface. The
// testcase generates 2 triplets of columns a, b, c such that a + b = c
// and the two modules are joined by a lookup.
type DistributeTestCase struct {
	numRow int
}

// Define defines the structure of the distributed wizard. The structure is
// composed of 2 modules that are connected by a lookup. The two modules are
// identical and are defined as a + b = c. The a, b and c are each defined as
// a commit in the wizard. The lookup is defined as a global constraint that
// enforces the equality of the two modules.
func (d DistributeTestCase) Define(comp *wizard.CompiledIOP) {

	// Define the first module
	a0 := comp.InsertCommit(0, "a0", d.numRow)
	b0 := comp.InsertCommit(0, "b0", d.numRow)
	c0 := comp.InsertCommit(0, "c0", d.numRow)

	// Importantly, the second module must be slightly different than the first
	// one because else it will create a wierd edge case in the conglomeration:
	// as we would have two GL modules with the same verifying key and we would
	// not be able to infer a module from a VK.
	//
	// We differentiate the modules by adding a duplicate constraints for GL0
	a1 := comp.InsertCommit(0, "a1", d.numRow)
	b1 := comp.InsertCommit(0, "b1", d.numRow)
	c1 := comp.InsertCommit(0, "c1", d.numRow)

	comp.InsertGlobal(0, "global-0", symbolic.Sub(c0, b0, a0))
	comp.InsertGlobal(0, "global-duplicate", symbolic.Sub(c0, b0, a0))
	comp.InsertGlobal(0, "global-1", symbolic.Sub(c1, b1, a1))

	comp.InsertInclusion(0, "inclusion-0", []ifaces.Column{c0, b0, a0}, []ifaces.Column{c1, b1, a1})
}

// Assign sets up the column assignments for the DistributeTestCase
// within the ProverRuntime. It assigns constant values to six columns
// ('a0', 'b0', 'c0', 'a1', 'b1', 'c1') where each column is assigned
// a smart vector with a constant field element value and a specified
// number of rows (d.numRow). This function helps initialize the columns
// with predetermined values for the testing setup.
func (d DistributeTestCase) Assign(run *wizard.ProverRuntime) {
	run.AssignColumn("a0", smartvectors.RightZeroPadded(vector.Repeat(field.NewElement(1), d.numRow-2), d.numRow))
	run.AssignColumn("b0", smartvectors.RightZeroPadded(vector.Repeat(field.NewElement(2), d.numRow-2), d.numRow))
	run.AssignColumn("c0", smartvectors.RightZeroPadded(vector.Repeat(field.NewElement(3), d.numRow-2), d.numRow))
	run.AssignColumn("a1", smartvectors.RightZeroPadded(vector.Repeat(field.NewElement(1), d.numRow-2), d.numRow))
	run.AssignColumn("b1", smartvectors.RightZeroPadded(vector.Repeat(field.NewElement(2), d.numRow-2), d.numRow))
	run.AssignColumn("c1", smartvectors.RightZeroPadded(vector.Repeat(field.NewElement(3), d.numRow-2), d.numRow))
}

// runProverGLs executes the prover for each GL module segment. It takes in a list of
// compiled GL segments and corresponding witnesses, then runs the prover for each
// segment. The function logs the start and end times of the prover execution for each
// segment. It returns a slice of ProverRuntime instances, each representing the
// result of the prover execution for a segment.
func runProverGLs(
	t *testing.T,
	distWizard *distributed.DistributedWizard,
	witnessGLs []*distributed.ModuleWitnessGL,
) []*wizard.ProverRuntime {

	var (
		compiledGLs = distWizard.CompiledGLs
		runs        = make([]*wizard.ProverRuntime, len(witnessGLs))
	)

	for i := range witnessGLs {

		var (
			witnessGL = witnessGLs[i]
			moduleGL  *distributed.RecursedSegmentCompilation
		)

		t.Logf("segment(total)=%v module=%v segment.index=%v", i, witnessGL.ModuleName, witnessGL.ModuleIndex)
		for k := range distWizard.ModuleNames {
			if distWizard.ModuleNames[k] != witnessGLs[i].ModuleName {
				continue
			}
			moduleGL = compiledGLs[k]
		}

		if moduleGL == nil {
			t.Fatalf("module does not exists, module=%v, distWizard.ModuleNames=%v", witnessGL.ModuleName, distWizard.ModuleNames)
		}

		t.Logf("RUNNING THE GL PROVER: %v", time.Now())
		runs[i] = moduleGL.ProveSegment(witnessGL)
		t.Logf("RUNNING THE GL PROVER - DONE: %v", time.Now())
	}

	return runs
}

// runProverLPPs runs a prover for a LPP segment. It takes in a DistributedWizard
// object, a slice of RecursedSegmentCompilation objects, and a slice of
// ModuleWitnessLPP objects. It runs the prover for each segment and logs the
// time at which the prover starts and ends. It returns a slice of ProverRuntime
// instances, each representing the result of the prover execution for a segment.
func runProverLPPs(
	t *testing.T,
	distWizard *distributed.DistributedWizard,
	sharedRandomness field.Element,
	witnessLPPs []*distributed.ModuleWitnessLPP,
) []*wizard.ProverRuntime {

	var (
		runs         = make([]*wizard.ProverRuntime, len(witnessLPPs))
		compiledLPPs = distWizard.CompiledLPPs
	)

	for i := range witnessLPPs {

		var (
			witnessLPP      = witnessLPPs[i]
			moduleLPP       *distributed.RecursedSegmentCompilation
			distModuleNames = [][]distributed.ModuleName{}
		)

		witnessLPP.InitialFiatShamirState = sharedRandomness

		t.Logf("segment(total)=%v module=%v segment.index=%v", i, witnessLPP.ModuleName, witnessLPP.ModuleIndex)

	ModuleLoop:
		for k := range distWizard.LPPs {

			moduleList := distWizard.LPPs[k].ModuleNames()
			distModuleNames = append(distModuleNames, moduleList)

			for l, m := range moduleList {
				if m != witnessLPP.ModuleName[l] {
					continue ModuleLoop
				}
			}

			moduleLPP = compiledLPPs[k]
			break
		}

		if moduleLPP == nil {
			t.Fatalf("module does not exists, moduleName=%v distModuleNames=%v", witnessLPP.ModuleName, distModuleNames)
		}

		t.Logf("RUNNING THE LPP PROVER: %v", time.Now())
		runs[i] = moduleLPP.ProveSegment(witnessLPP)
		t.Logf("RUNNING THE LPP PROVER - DONE: %v", time.Now())
	}

	return runs
}
