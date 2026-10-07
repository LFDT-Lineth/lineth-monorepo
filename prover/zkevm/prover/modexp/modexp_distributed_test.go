package modexp

import (
	"fmt"
	"testing"

	multisethashing "github.com/consensys/linea-monorepo/prover/crypto/multisethashing_koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/protocol/accessors"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed"
	"github.com/consensys/linea-monorepo/prover/protocol/limbs"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils/csvtraces"
)

// TestModExpDistributed checks that the modexp module can be segmented by the
// limitless prover: every GL and LPP segment must verify and the
// cross-segment Horner, log-derivative, grand-product and multiset
// accumulators must cancel. The segment sizes must match the MODEXP advices in
// zkevm/limitless.go.
func TestModExpDistributed(t *testing.T) {

	testCases := []struct {
		InputFName                         string
		NbSmallInstances, NbLargeInstances int
	}{
		{
			// two large instances (512-bit base, exponent 1, BLS12-381 base
			// field modulus) from a real block
			InputFName:       "testdata/two_large_instances_input.csv",
			NbSmallInstances: 1,
			NbLargeInstances: 2,
		},
		{
			// the first instance of the above
			InputFName:       "testdata/single_large_exponent_one_input.csv",
			NbSmallInstances: 1,
			NbLargeInstances: 1,
		},
		{
			InputFName:       "testdata/single_8192_bits_input.csv",
			NbSmallInstances: 1,
			NbLargeInstances: 1,
		},
		{
			InputFName:       "testdata/single_256_bits_input.csv",
			NbSmallInstances: 10,
			NbLargeInstances: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.InputFName, func(t *testing.T) {
			runModExpDistributed(t, tc.InputFName, tc.NbSmallInstances, tc.NbLargeInstances)
		})
	}
}

func runModExpDistributed(t *testing.T, inputFName string, nbSmall, nbLarge int) {

	const (
		segmentSizeSmall = 8192
		segmentSizeLarge = largeModExpSize
	)

	var (
		inp   *Input
		mod   *Module
		inpCt = csvtraces.MustOpenCsvFile(inputFName)
	)

	wiop := wizard.Compile(func(build *wizard.Builder) {
		inp = &Input{
			IsModExpBase:     inpCt.GetCommit(build, "IS_MODEXP_BASE"),
			IsModExpExponent: inpCt.GetCommit(build, "IS_MODEXP_EXPONENT"),
			IsModExpModulus:  inpCt.GetCommit(build, "IS_MODEXP_MODULUS"),
			IsModExpResult:   inpCt.GetCommit(build, "IS_MODEXP_RESULT"),
			Limbs:            inpCt.GetLimbsLe(build, "LIMBS", limbs.NbLimbU128).AssertUint128(),
			Settings:         &Settings{MaxNbInstance256: nbSmall, MaxNbInstanceLarge: nbLarge},
		}
		mod = newModule(build.CompiledIOP, inp)
	})

	disc := &distributed.StandardModuleDiscoverer{
		TargetWeight: 1 << 28,
		Advices: []*distributed.ModuleDiscoveryAdvice{
			{BaseSize: segmentSizeLarge, Cluster: "MODEXP_LARGE", Column: mod.Large.IsActive},
			{BaseSize: min(segmentSizeSmall, mod.Small.IsActive.Size()), Cluster: "MODEXP-256", Column: mod.Small.IsActive},
			{BaseSize: inp.Limbs.Size(), Cluster: "MODEXP-256", Regexp: `^(?!MODEXP_LARGE_|MODEXP_SMALL_|BIGRANGE_MODEXP_).*`},
			{BaseSize: inp.Limbs.Size(), Cluster: "MODEXP-256", Regexp: `^MODEXP_(LARGE|SMALL)_IS_(BASE|EXPONENT|MODULUS|RESULT)$`},
		},
	}

	dw := distributed.DistributeWizard(wiop, disc)
	dummy.Compile(dw.Bootstrapper)
	for i := range dw.GLs {
		dummy.CompileAtProverLvl()(dw.GLs[i].Wiop)
		addDummyLPPMerkleRoots(dw.GLs[i].Wiop)
	}
	for i := range dw.LPPs {
		dummy.CompileAtProverLvl()(dw.LPPs[i].Wiop)
		addDummyLPPMerkleRoots(dw.LPPs[i].Wiop)
	}

	runBoot := wizard.RunProver(dw.Bootstrapper, func(run *wizard.ProverRuntime) {
		inpCt.Assign(run,
			inp.Limbs,
			inp.IsModExpBase,
			inp.IsModExpExponent,
			inp.IsModExpModulus,
			inp.IsModExpResult,
		)
	}, false)

	if err := wizard.Verify(dw.Bootstrapper, runBoot.ExtractProof()); err != nil {
		t.Fatalf("bootstrapper failed: %v", err)
	}

	witnessGLs, witnessLPPs := distributed.SegmentRuntime(runBoot, dw.Disc, dw.BlueprintGLs, dw.BlueprintLPPs, field.Octuplet{})

	var (
		allGrandProduct     = fext.One()
		allLogDerivativeSum = fext.Element{}
		allHornerSum        = fext.Element{}
		generalMSet         = multisethashing.MSetHash{}
	)

	for i, w := range witnessGLs {
		var moduleGL *distributed.ModuleGL
		for k := range dw.ModuleNames {
			if dw.ModuleNames[k] == w.ModuleName {
				moduleGL = dw.GLs[k]
			}
		}

		run := wizard.RunProver(moduleGL.Wiop, moduleGL.GetMainProverStep(w), false)
		verRun, err := wizard.VerifyWithRuntime(moduleGL.Wiop, run.ExtractProof(), false)
		if err != nil {
			t.Errorf("GL segment %v (module=%v) failed: %v", i, w.ModuleName, err)
			continue
		}
		generalMSet.Add(multisethashing.MSetHash(distributed.GetPublicInputList(verRun, distributed.GeneralMultiSetPublicInputBase, multisethashing.MSetHashSize)))
	}

	for i, w := range witnessLPPs {
		moduleLPP := dw.LPPs[w.ModuleIndex]
		// stubbed initial Fiat-Shamir state for reproducibility
		w.InitialFiatShamirState = field.NewOctupletFromStrings([8]string{"1", "2", "3", "4", "5", "6", "7", "8"})

		run := wizard.RunProver(moduleLPP.Wiop, moduleLPP.GetMainProverStep(w), false)
		verRun, err := wizard.VerifyWithRuntime(moduleLPP.Wiop, run.ExtractProof(), false)
		if err != nil {
			t.Errorf("LPP segment %v (module=%v) failed: %v", i, w.ModuleName, err)
			continue
		}
		generalMSet.Add(multisethashing.MSetHash(distributed.GetPublicInputList(verRun, distributed.GeneralMultiSetPublicInputBase, multisethashing.MSetHashSize)))

		var (
			logDerivativeSum = verRun.GetPublicInput(distributed.LogDerivativeSumPublicInput).Ext
			grandProduct     = verRun.GetPublicInput(distributed.GrandProductPublicInput).Ext
			hornerSum        = verRun.GetPublicInput(distributed.HornerPublicInput).Ext
		)
		allGrandProduct.Mul(&allGrandProduct, &grandProduct)
		allLogDerivativeSum.Add(&allLogDerivativeSum, &logDerivativeSum)
		allHornerSum.Add(&allHornerSum, &hornerSum)
	}

	if !allHornerSum.IsZero() {
		t.Errorf("horner sum does not cancel: %v", allHornerSum.String())
	}
	if !allLogDerivativeSum.IsZero() {
		t.Errorf("log-derivative sum does not cancel: %v", allLogDerivativeSum.String())
	}
	if !allGrandProduct.IsOne() {
		t.Errorf("grand product does not cancel: %v", allGrandProduct.String())
	}
	if !generalMSet.IsEmpty() {
		t.Errorf("general multiset does not cancel")
	}
}

// addDummyLPPMerkleRoots adds the LPP merkle root public inputs which are
// normally added by the Vortex compiler.
func addDummyLPPMerkleRoots(comp *wizard.CompiledIOP) {
	for j := range 8 {
		comp.InsertPublicInput(fmt.Sprintf("LPP_COLUMNS_MERKLE_ROOTS_0_%d", j), accessors.NewConstant(field.Zero()))
	}
}
