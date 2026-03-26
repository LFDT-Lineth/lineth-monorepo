package distributed_test

import (
	"fmt"
	"testing"

	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/zkevm"
)

// MockDiscoveryWizard creates a simple wizard for testing module discovery
// without depending on the zkevm package
func MockDiscoveryWizard() *wizard.CompiledIOP {
	defFunc := func(build *wizard.Builder) {
		comp := build.CompiledIOP

		// Create multiple modules with different sizes to simulate real zkevm
		// Must include global constraints so discovery creates query-based modules
		sizes := []int{1 << 10, 1 << 12, 1 << 14, 1 << 16, 1 << 18, 1 << 20}

		for i, size := range sizes {
			a := comp.InsertCommit(0, ifaces.ColIDf("zkevm_mod%d_a", i), size, true)
			b := comp.InsertCommit(0, ifaces.ColIDf("zkevm_mod%d_b", i), size, true)
			c := comp.InsertCommit(0, ifaces.ColIDf("zkevm_mod%d_c", i), size, true)
			// Add global constraint - this creates query-based modules that need advice
			comp.InsertGlobal(0, ifaces.QueryIDf("zkevm_global_%d", i), symbolic.Sub(c, b, a))
		}

		// Add inclusion to create inter-module dependencies (like real zkevm)
		a0 := comp.Columns.GetHandle("zkevm_mod0_a")
		a1 := comp.Columns.GetHandle("zkevm_mod1_a")
		comp.InsertInclusion(0, "zkevm_inclusion_0_1", []ifaces.Column{a0}, []ifaces.Column{a1})
	}

	return wizard.Compile(defFunc)
}

func TestStandardDiscoveryOnZkEVM(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping heavy test in short mode")
	}

	var (
		z    = zkevm.GetTestZkEVM()
		disc = &distributed.StandardModuleDiscoverer{
			TargetWeight: 1 << 28,
			Advices:      zkevm.DiscoveryAdvices(z),
		}
	)

	distributed.PrecompileInitialWizard(z.InitialCompiledIOP, disc)

	// The test is to make sure that this function returns
	disc.Analyze(z.InitialCompiledIOP)

	t.Run("new size and new columns are found", func(t *testing.T) {

		allCols := z.InitialCompiledIOP.Columns.AllKeys()
		for _, colName := range allCols {
			col := z.InitialCompiledIOP.Columns.GetHandle(colName)

			var (
				nat        = col.(column.Natural)
				newSize    = distributed.NewSizeOfColumn(disc, nat)
				module     = disc.ModuleOf(nat)
				_, qbmSize = disc.QbmOf(nat)
			)

			if module == "" {
				t.Errorf("module of %v is empty", colName)
			}

			if newSize == 0 {
				t.Errorf("new-size of %v is 0", colName)
			}

			if newSize != qbmSize {
				t.Errorf("new-size of %v is %v, expected %v", colName, newSize, qbmSize)
			}
		}
	})

	t.Run("a column may only belong to a single module", func(t *testing.T) {

		for _, col := range z.InitialCompiledIOP.Columns.AllKeys() {

			var (
				nat     = z.InitialCompiledIOP.Columns.GetHandle(col).(column.Natural)
				modules = []distributed.ModuleName{}
			)

			for i := range disc.Modules {
				mod := disc.Modules[i]
				for k := range mod.SubModules {
					if mod.SubModules[k].Ds.Has(nat.ID) {
						modules = append(modules, mod.ModuleName)
					}
				}
			}

			if len(modules) == 0 {
				t.Errorf("could not match any module for %v", col)
			}

			if len(modules) > 1 {
				t.Errorf("could match more than one module for %v", col)
			}
		}
	})

	t.Logf("totalNumber of columns: %v", len(z.InitialCompiledIOP.Columns.AllKeys()))

	for _, mod := range disc.Modules {
		t.Logf("module=%v weight=%v numcol=%v\n", mod.ModuleName, mod.Weight(), disc.NumColumnOf(mod.ModuleName))
	}
}

// TestStandardDiscoveryOnMockWizardWithAdvices tests discovery with advices
func TestStandardDiscoveryOnMockWizardWithAdvices(t *testing.T) {

	var (
		wiop = MockDiscoveryWizard()
		disc = &distributed.StandardModuleDiscoverer{
			TargetWeight: 1 << 14,
			Advices:      MockDiscoveryAdvices,
		}
	)

	distributed.PrecompileInitialWizard(wiop, disc)

	// The test is to make sure that this function returns
	disc.Analyze(wiop)

	fmt.Printf("%++v\n", disc)

	allCols := wiop.Columns.AllKeys()
	for _, colName := range allCols {
		col := wiop.Columns.GetHandle(colName)

		var (
			nat     = col.(column.Natural)
			newSize = disc.NewSizeOf(nat)
			module  = disc.ModuleOf(nat)
		)

		if module == "" {
			t.Errorf("module of %v is empty", colName)
		}

		if newSize == 0 {
			t.Errorf("new-size of %v is 0", colName)
		}
	}

	for _, col := range wiop.Columns.AllKeys() {

		var (
			nat     = wiop.Columns.GetHandle(col).(column.Natural)
			modules = []distributed.ModuleName{}
		)

		for i := range disc.Modules {
			mod := disc.Modules[i]
			for k := range mod.SubModules {
				if mod.SubModules[k].Ds.Has(nat.ID) {
					modules = append(modules, mod.ModuleName)
				}
			}
		}

		if len(modules) == 0 {
			t.Errorf("could not match any module for %v", col)
		}

		if len(modules) > 1 {
			t.Errorf("could match more than one module for %v: %v", col, modules)
		}
	}

	t.Logf("totalNumber of columns: %v", len(wiop.Columns.AllKeys()))

	for _, mod := range disc.Modules {
		fmt.Printf("module=%v weight=%v numcol=%v\n", mod.ModuleName, mod.Weight(), disc.NumColumnOf(mod.ModuleName))
		for i, subModule := range mod.SubModules {

			var (
				newSize  = mod.NewSizes[i]
				allCols  = utils.SortedKeysOf(subModule.Ds.Parent, func(a, b ifaces.ColID) bool { return a < b })
				nbCols   = len(allCols)
				firstCol = allCols[0]
				weight   = subModule.Weight(newSize)
			)

			fmt.Printf("\tname=%v firstcol=%v nbCols=%v newSize=%v weight=%v\n", subModule.ModuleName, firstCol, nbCols, newSize, weight)
		}
	}
}

// MockDiscoveryAdvices - one advice per module (only one column per qbm is matched)
var MockDiscoveryAdvices = []*distributed.ModuleDiscoveryAdvice{
	{BaseSize: 1 << 10, Cluster: "ZKEVM_MOD0", Regexp: "zkevm_mod0_c"},
	{BaseSize: 1 << 12, Cluster: "ZKEVM_MOD1", Regexp: "zkevm_mod1_c"},
	{BaseSize: 1 << 14, Cluster: "ZKEVM_MOD2", Regexp: "zkevm_mod2_c"},
	{BaseSize: 1 << 16, Cluster: "ZKEVM_MOD3", Regexp: "zkevm_mod3_c"},
	{BaseSize: 1 << 18, Cluster: "ZKEVM_MOD4", Regexp: "zkevm_mod4_c"},
	{BaseSize: 1 << 20, Cluster: "ZKEVM_MOD5", Regexp: "zkevm_mod5_c"},
}
