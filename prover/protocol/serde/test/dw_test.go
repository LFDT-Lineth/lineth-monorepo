package serde_test

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/consensys/linea-monorepo/prover/protocol/distributed"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/zkevm"
)

func BenchmarkSerdeDW(b *testing.B) {

	b.Skipf("the test is a development/debug/integration test. It is not needed for CI")

	dist := GetDistWizard()
	b.ResetTimer()

	b.Run("ModuleNames", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			justserde(b, dist.ModuleNames, "DW.ModuleNames")
		}
	})

	for i := range dist.GLs {
		b.Run(fmt.Sprintf("GLModule-%d", i), func(b *testing.B) {
			for j := 0; j < b.N; j++ {
				justserde(b, dist.GLs[i], fmt.Sprintf("DW.GLModule-%d", i))
			}
		})
	}

	for i := range dist.LPPs {
		b.Run(fmt.Sprintf("LPPModule-%d", i), func(b *testing.B) {
			for j := 0; j < b.N; j++ {
				justserde(b, dist.LPPs[i], fmt.Sprintf("DW.LPPModule-%d", i))
			}
		})
	}

	b.Run("Bootstrapper", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			justserde(b, dist.Bootstrapper, "DW.Bootstrapper")
		}
	})

	b.Run("Discoverer", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			justserde(b, dist.Disc, "DW.Discoverer")
		}
	})

	for i := range dist.CompiledGLs {

		b.Run(fmt.Sprintf("CompiledGL-%v", i), func(b *testing.B) {
			for j := 0; j < b.N; j++ {
				justserde(b, dist.CompiledGLs[i], fmt.Sprintf("DW.CompiledGL-%v", i))
			}
		})
	}

	for i := range dist.CompiledLPPs {

		b.Run(fmt.Sprintf("CompiledLPP-%v", i), func(b *testing.B) {
			for j := 0; j < b.N; j++ {
				justserde(b, dist.CompiledLPPs[i], fmt.Sprintf("DW.CompiledLPP-%v", i))
			}
		})
	}

	// To save memory
	cong := dist.CompiledConglomeration
	dist = nil
	runtime.GC()

	b.Run("CompiledConglomeration", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			justserde(b, cong, "DW.CompiledConglomeration")
		}
	})
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
			Advices:      zkevm.DiscoveryAdvices(z),
		}

		// This tests the compilation of the compiled-IOP
		distWizard = distributed.DistributeWizard(z.InitialCompiledIOP, disc).
				CompileSegments(zkevm.LimitlessCompilationParams).
				Conglomerate(zkevm.LimitlessCompilationParams)
	)

	return distWizard
}

func GetBasicDistWizard() *distributed.DistributedWizard {

	var (
		numRow = 1 << 10
		tc     = distributeTestCase{numRow: numRow}
		disc   = &distributed.StandardModuleDiscoverer{
			TargetWeight: 3 * numRow,
		}
		comp = wizard.Compile(func(build *wizard.Builder) {
			tc.define(build.CompiledIOP)
		})

		// This tests the compilation of the compiled-IOP
		distWizard = distributed.DistributeWizard(comp, disc).
				CompileSegments(zkevm.LimitlessCompilationParams).
				Conglomerate(zkevm.LimitlessCompilationParams)
	)

	return distWizard
}

func TestSerdeDistWizard(t *testing.T) {

	t.Skipf("the test is a development/debug/integration test. It is not needed for CI")

	dist := GetDistWizard()

	var (
		isSanityCheck = true
		isfailFast    = true
	)

	t.Run("ModuleNames", func(t *testing.T) {
		runSerdeTest(t, dist.ModuleNames, "DW.ModuleNames", isSanityCheck, isfailFast)
	})

	for i := range dist.GLs {
		t.Run(fmt.Sprintf("DW.GLModule-%d", i), func(t *testing.T) {
			runSerdeTest(t, dist.GLs[i], fmt.Sprintf("DW.GLModule-%d", i), isSanityCheck, isfailFast)
		})
	}

	for i := range dist.LPPs {
		t.Run(fmt.Sprintf("DW.LPPModule-%d", i), func(t *testing.T) {
			runSerdeTest(t, dist.LPPs[i], fmt.Sprintf("DW.LPPModule-%d", i), isSanityCheck, isfailFast)
		})
	}

	t.Run("Bootstrapper", func(t *testing.T) {
		runSerdeTest(t, dist.Bootstrapper, "DW.Bootstrapper", isSanityCheck, isfailFast)
	})

	t.Run("Discoverer", func(t *testing.T) {
		runSerdeTest(t, dist.Disc, "DW.Discoverer", isSanityCheck, isfailFast)
	})

	for i := range dist.CompiledGLs {
		t.Run(fmt.Sprintf("CompiledGL-%v", i), func(t *testing.T) {
			runSerdeTest(t, dist.CompiledGLs[i], fmt.Sprintf("DW.CompiledGL-%v", i), isSanityCheck, isfailFast)
		})
	}

	for i := range dist.CompiledLPPs {
		t.Run(fmt.Sprintf("CompiledLPP-%v", i), func(t *testing.T) {
			runSerdeTest(t, dist.CompiledLPPs[i], fmt.Sprintf("DW.CompiledLPP-%v", i), isSanityCheck, isfailFast)
		})
	}

	// To save memory
	cong := dist.CompiledConglomeration
	dist = nil
	runtime.GC()

	t.Run("CompiledConglomeration", func(t *testing.T) {
		runSerdeTest(t, cong, "DW.CompiledConglomeration", isSanityCheck, isfailFast)
	})
}
