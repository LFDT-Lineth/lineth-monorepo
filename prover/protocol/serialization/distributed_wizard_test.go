package serialization_test

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/consensys/linea-monorepo/prover/protocol/distributed"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/zkevm"
)

type distributeTestCase struct {
	numRow int
}

func (d distributeTestCase) define(comp *wizard.CompiledIOP) {

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

// GetDistWizard initializes a distributed wizard configuration using the
// ZkEVM's compiled IOP and a StandardModuleDiscoverer with preset parameters.
// The function compiles the necessary segments and produces a conglomerated
// distributed wizard, which is then returned.
func GetDistWizard() *distributed.DistributedWizard {
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

	return distWizard
}

func GetBasicDistWizard() *distributed.DistributedWizard {

	var (
		numRow = 1 << 10
		tc     = distributeTestCase{numRow: numRow}
		disc   = &distributed.StandardModuleDiscoverer{
			TargetWeight: 3 * numRow,
			Predivision:  1,
		}
		comp = wizard.Compile(func(build *wizard.Builder) {
			tc.define(build.CompiledIOP)
		})

		// This tests the compilation of the compiled-IOP
		distWizard = distributed.DistributeWizard(comp, disc).
				CompileSegments().
				Conglomerate(20)
	)

	return distWizard
}

func TestSerdeDistWizard(t *testing.T) {

	t.Skipf("the test is a development/debug/integration test. It is not needed for CI")

	dist := GetDistWizard()

	t.Run("ModuleNames", func(t *testing.T) {
		runSerdeTest(t, dist.ModuleNames, "DistributedWizard.ModuleNames", true, false)
	})

	for i := range dist.GLs {
		t.Run(fmt.Sprintf("GLModule-%d", i), func(t *testing.T) {
			runSerdeTest(t, dist.GLs[i], "DistributedWizard.GLs", true, false)
		})
	}

	for i := range dist.LPPs {
		t.Run(fmt.Sprintf("LPPModule-%d", i), func(t *testing.T) {
			runSerdeTest(t, dist.LPPs[i], "DistributedWizard.LPPs", true, false)
		})
	}

	t.Run("DefaultModule", func(t *testing.T) {
		runSerdeTest(t, dist.DefaultModule, "DistributedWizard.DefaultModule", true, false)
	})

	t.Run("Bootstrapper", func(t *testing.T) {
		runSerdeTest(t, dist.Bootstrapper, "DistributedWizard.Bootstrapper", true, false)
	})

	t.Run("Discoverer", func(t *testing.T) {
		runSerdeTest(t, dist.Disc, "DistributedWizard.Discoverer", true, false)
	})

	t.Run("CompiledDefault", func(t *testing.T) {
		runSerdeTest(t, dist.CompiledDefault, "DistributedWizard.CompiledDefault", true, false)
	})

	for i := range dist.CompiledGLs {
		t.Run(fmt.Sprintf("CompiledGL-%v", i), func(t *testing.T) {
			runSerdeTest(t, dist.CompiledGLs[i], fmt.Sprintf("DistributedWizard.CompiledGL-%v", i), true, false)
		})
	}

	for i := range dist.CompiledLPPs {
		t.Run(fmt.Sprintf("CompiledLPP-%v", i), func(t *testing.T) {
			runSerdeTest(t, dist.CompiledLPPs[i], fmt.Sprintf("DistributedWizard.CompiledLPP-%v", i), true, false)
		})
	}

	// To save memory
	cong := dist.CompiledConglomeration
	dist = nil
	runtime.GC()

	t.Run("CompiledConglomeration", func(t *testing.T) {
		runSerdeTest(t, cong, "DistributedWizard.CompiledConglomeration", true, false)
	})
}

func TestSerdeDWCong(t *testing.T) {

	t.Skipf("the test is a development/debug/integration test. It is not needed for CI")

	// Setup
	distWizard := GetBasicDistWizard()
	cong := distWizard.CompiledConglomeration
	distWizard = nil
	runtime.GC()

	// Subtests
	tests := []struct {
		name        string
		obj         any
		sanityCheck bool
		failfast    bool
	}{
		{name: "Wiop", obj: cong.Wiop, sanityCheck: true, failfast: true},
		{name: "Recursion", obj: cong.Recursion, sanityCheck: true, failfast: true},

		// All of these tests PASS
		{name: "MaxNbProofs", obj: cong.MaxNbProofs, sanityCheck: true, failfast: false},
		{name: "DefaultWitness", obj: cong.DefaultWitness, sanityCheck: true, failfast: true},
		{name: "DefaultIops", obj: cong.DefaultIops, sanityCheck: true, failfast: true},
		{name: "PrecomputedGLVks", obj: cong.PrecomputedGLVks, sanityCheck: true, failfast: false},
		{name: "PrecomputedLPPVks", obj: cong.PrecomputedLPPVks, sanityCheck: true, failfast: false},
		{name: "VerifyingKeyColumns", obj: cong.VerifyingKeyColumns, sanityCheck: true, failfast: false},
		{name: "HolisticLookupMappedLPPPostion", obj: cong.HolisticLookupMappedLPPPostion, sanityCheck: true, failfast: false},
		{name: "HolisticLookupMappedLPPVK", obj: cong.HolisticLookupMappedLPPVK, sanityCheck: true, failfast: false},
		{name: "IsGL", obj: cong.IsGL, sanityCheck: true, failfast: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runSerdeTest(t, tt.obj, tt.name, tt.sanityCheck, tt.failfast)
		})
	}
}
