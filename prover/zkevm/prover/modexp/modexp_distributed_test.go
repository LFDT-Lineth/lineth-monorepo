package modexp

import (
	"fmt"
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/accessors"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed"
	"github.com/consensys/linea-monorepo/prover/protocol/limbs"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils/csvtraces"
)

// TestModExpLargeDistributed checks that the large modexp module can be
// segmented by the limitless prover when it holds several instances. The input
// contains two large instances (512-bit base, exponent 1, BLS12-381 modulus)
// from a real block. The segment size must match the MODEXP_LARGE advice in
// zkevm/limitless.go.
func TestModExpLargeDistributed(t *testing.T) {

	const segmentSizeLarge = largeModExpSize

	var (
		inp   *Input
		mod   *Module
		inpCt = csvtraces.MustOpenCsvFile("testdata/two_large_instances_input.csv")
	)

	wiop := wizard.Compile(func(build *wizard.Builder) {
		inp = &Input{
			IsModExpBase:     inpCt.GetCommit(build, "IS_MODEXP_BASE"),
			IsModExpExponent: inpCt.GetCommit(build, "IS_MODEXP_EXPONENT"),
			IsModExpModulus:  inpCt.GetCommit(build, "IS_MODEXP_MODULUS"),
			IsModExpResult:   inpCt.GetCommit(build, "IS_MODEXP_RESULT"),
			Limbs:            inpCt.GetLimbsLe(build, "LIMBS", limbs.NbLimbU128).AssertUint128(),
			Settings:         &Settings{MaxNbInstance256: 1, MaxNbInstanceLarge: 2},
		}
		mod = newModule(build.CompiledIOP, inp)
	})

	disc := &distributed.StandardModuleDiscoverer{
		TargetWeight: 1 << 28,
		Advices: []*distributed.ModuleDiscoveryAdvice{
			{BaseSize: segmentSizeLarge, Cluster: "MODEXP_LARGE", Column: mod.Large.IsActive},
			{BaseSize: mod.Small.IsActive.Size(), Cluster: "MODEXP-256", Column: mod.Small.IsActive},
			{BaseSize: inp.Limbs.Size(), Cluster: "MODEXP-256", Regexp: `^(?!MODEXP_LARGE_|MODEXP_SMALL_|BIGRANGE_MODEXP_).*`},
			{BaseSize: inp.Limbs.Size(), Cluster: "MODEXP-256", Regexp: `^MODEXP_(LARGE|SMALL)_IS_(BASE|EXPONENT|MODULUS|RESULT)$`},
		},
	}

	dw := distributed.DistributeWizard(wiop, disc)
	dummy.Compile(dw.Bootstrapper)
	for i := range dw.GLs {
		dummy.CompileAtProverLvl()(dw.GLs[i].Wiop)
		for j := range 8 {
			dw.GLs[i].Wiop.InsertPublicInput(fmt.Sprintf("LPP_COLUMNS_MERKLE_ROOTS_0_%d", j), accessors.NewConstant(field.Zero()))
		}
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

	witnessGLs, _ := distributed.SegmentRuntime(runBoot, dw.Disc, dw.BlueprintGLs, dw.BlueprintLPPs, field.Octuplet{})

	for i, w := range witnessGLs {
		var moduleGL *distributed.ModuleGL
		for k := range dw.ModuleNames {
			if dw.ModuleNames[k] == w.ModuleName {
				moduleGL = dw.GLs[k]
			}
		}

		run := wizard.RunProver(moduleGL.Wiop, moduleGL.GetMainProverStep(w), false)
		if _, err := wizard.VerifyWithRuntime(moduleGL.Wiop, run.ExtractProof(), false); err != nil {
			t.Errorf("GL segment %v (module=%v) failed: %v", i, w.ModuleName, err)
		}
	}
}
