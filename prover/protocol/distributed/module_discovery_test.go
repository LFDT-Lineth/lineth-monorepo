package distributed_test

import (
	"fmt"
	"testing"

	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/zkevm"
)

func TestStandardDiscoveryOnZkEVM(t *testing.T) {

	var (
		z    = zkevm.GetTestZkEVM()
		disc = &distributed.StandardModuleDiscoverer{
			TargetWeight: 1 << 28,
			Advices:      zkevm.DiscoveryAdvices(z),
			Predivision:  16,
		}
	)

	distributed.PrecompileInitialWizard(z.WizardIOP, disc)

	// The test is to make sure that this function returns
	disc.Analyze(z.WizardIOP)

	allCols := z.WizardIOP.Columns.AllKeys()
	for _, colName := range allCols {
		col := z.WizardIOP.Columns.GetHandle(colName)

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

	for _, col := range z.WizardIOP.Columns.AllKeys() {

		var (
			nat     = z.WizardIOP.Columns.GetHandle(col).(column.Natural)
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

	t.Logf("totalNumber of columns: %v", len(z.WizardIOP.Columns.AllKeys()))

	for _, mod := range disc.Modules {
		t.Logf("module=%v weight=%v numcol=%v\n", mod.ModuleName, mod.Weight(z.WizardIOP), disc.NumColumnOf(mod.ModuleName))
	}
}

func TestStandardDiscoveryOnZkEVMWithAdvices(t *testing.T) {

	var (
		z    = zkevm.GetTestZkEVM()
		disc = &distributed.StandardModuleDiscoverer{
			TargetWeight: 1 << 29,
			Predivision:  1,
			Advices:      zkevm.DiscoveryAdvices(z),
		}
	)

	distributed.PrecompileInitialWizard(z.WizardIOP, disc)

	// The test is to make sure that this function returns
	disc.Analyze(z.WizardIOP)

	fmt.Printf("%++v\n", disc)

	allCols := z.WizardIOP.Columns.AllKeys()
	for _, colName := range allCols {
		col := z.WizardIOP.Columns.GetHandle(colName)

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

	for _, col := range z.WizardIOP.Columns.AllKeys() {

		var (
			nat     = z.WizardIOP.Columns.GetHandle(col).(column.Natural)
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

	t.Logf("totalNumber of columns: %v", len(z.WizardIOP.Columns.AllKeys()))

	for _, mod := range disc.Modules {
		fmt.Printf("module=%v weight=%v numcol=%v\n", mod.ModuleName, mod.Weight(z.WizardIOP), disc.NumColumnOf(mod.ModuleName))
		for i, subModule := range mod.SubModules {

			var (
				newSize  = mod.NewSizes[i]
				allCols  = utils.SortedKeysOf(subModule.Ds.Parent, func(a, b ifaces.ColID) bool { return a < b })
				nbCols   = len(allCols)
				firstCol = allCols[0]
				weight   = subModule.Weight(z.WizardIOP, newSize)
			)

			fmt.Printf("\tname=%v firstcol=%v nbCols=%v newSize=%v weight=%v\n", subModule.ModuleName, firstCol, nbCols, newSize, weight)
		}
	}
}
