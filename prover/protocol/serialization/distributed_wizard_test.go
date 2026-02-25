package serialization_test

import (
	"fmt"
	"path"
	"runtime"
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils/profiling"
	"github.com/consensys/linea-monorepo/prover/zkevm"
)

// TestSerdeDWPerf: This test assumes that serialization and deserialization (ser/de) have already been
// verified to be correct in the previous test (TestSerdeDW).
func TestSerdeDWPerf(t *testing.T) {

	t.Skipf("the test is a development/debug/integration test. It is not needed for CI")

	var perfLogs profiling.PerfLogs
	dw := GetDistWizard()

	t.Run("ModuleNames", func(t *testing.T) {
		perfLogs = append(perfLogs, runSerdeTestPerf(t, dw.ModuleNames, "DW.ModuleNames"))
	})

	for i := range dw.GLs {
		t.Run(fmt.Sprintf("GLModule-%d", i), func(t *testing.T) {
			perfLogs = append(perfLogs, runSerdeTestPerf(t, dw.GLs[i], fmt.Sprintf("DW.GLModule-%v", i)))
		})
	}

	for i := range dw.LPPs {
		t.Run(fmt.Sprintf("LPPModule-%d", i), func(t *testing.T) {
			perfLogs = append(perfLogs, runSerdeTestPerf(t, dw.LPPs[i], fmt.Sprintf("DW.LPPModule-%d", i)))
		})
	}

	t.Run("Bootstrapper", func(t *testing.T) {
		perfLogs = append(perfLogs, runSerdeTestPerf(t, dw.Bootstrapper, "DW.Bootstrapper"))
	})

	t.Run("Discoverer", func(t *testing.T) {
		perfLogs = append(perfLogs, runSerdeTestPerf(t, dw.Disc, "DW.Discoverer"))
	})

	for i := range dw.CompiledGLs {
		t.Run(fmt.Sprintf("CompiledGL-%v", i), func(t *testing.T) {
			perfLogs = append(perfLogs, runSerdeTestPerf(t, dw.CompiledGLs[i], fmt.Sprintf("DW.CompiledGL-%v", i)))
		})
	}

	for i := range dw.CompiledLPPs {
		t.Run(fmt.Sprintf("CompiledLPP-%v", i), func(t *testing.T) {
			perfLogs = append(perfLogs, runSerdeTestPerf(t, dw.CompiledLPPs[i], fmt.Sprintf("DW.CompiledLPP-%v", i)))
		})
	}

	// To save memory
	cong := dw.CompiledConglomeration
	dw = nil
	runtime.GC()

	t.Run("CompiledConglomeration", func(t *testing.T) {
		perfLogs = append(perfLogs, runSerdeTestPerf(t, cong, "DW.CompiledConglomeration"))
	})

	// Write performance logs to CSV
	if err := perfLogs.WritePerformanceLogsToCSV(path.Join("perf", "dw-perf-logs.csv")); err != nil {
		t.Fatalf("Error writing performance logs to csv: %v", err)
	}
}

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
func (d distributeTestCase) assign(run *wizard.ProverRuntime) {
	run.AssignColumn("a0", smartvectors.RightZeroPadded(vector.Repeat(field.NewElement(1), d.numRow-2), d.numRow))
	run.AssignColumn("b0", smartvectors.RightZeroPadded(vector.Repeat(field.NewElement(2), d.numRow-2), d.numRow))
	run.AssignColumn("c0", smartvectors.RightZeroPadded(vector.Repeat(field.NewElement(3), d.numRow-2), d.numRow))
	run.AssignColumn("a1", smartvectors.RightZeroPadded(vector.Repeat(field.NewElement(1), d.numRow-2), d.numRow))
	run.AssignColumn("b1", smartvectors.RightZeroPadded(vector.Repeat(field.NewElement(2), d.numRow-2), d.numRow))
	run.AssignColumn("c1", smartvectors.RightZeroPadded(vector.Repeat(field.NewElement(3), d.numRow-2), d.numRow))
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
			Predivision:  1,
		}

		// This tests the compilation of the compiled-IOP
		distWizard = distributed.DistributeWizard(z.WizardIOP, disc).
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
			Predivision:  1,
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
