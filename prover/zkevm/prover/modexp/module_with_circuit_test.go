//go:build !fuzzlight

package modexp

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils/csvtraces"
)

func TestModexpWithCircuit(t *testing.T) {

	testCases := []struct {
		InputFName, ModuleFName            string
		NbSmallInstances, NbLargeInstances int
	}{
		{
			InputFName:       "testdata/single_256_bits_input.csv",
			ModuleFName:      "testdata/single_256_bits_module.csv",
			NbSmallInstances: 1,
			NbLargeInstances: 0,
		},
		{
			InputFName:       "testdata/single_4096_bits_input.csv",
			ModuleFName:      "testdata/single_4096_bits_module.csv",
			NbSmallInstances: 1, // not used but include anyway
			NbLargeInstances: 1,
		},
		{
			InputFName:       "testdata/single_8192_bits_input.csv",
			ModuleFName:      "testdata/single_8192_bits_module.csv",
			NbSmallInstances: 1, // not used but include anyway
			NbLargeInstances: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.InputFName, func(t *testing.T) {

			var (
				inp   Input
				mod   *Module
				inpCt = csvtraces.MustOpenCsvFile(tc.InputFName)
			)

			cmp := wizard.Compile(func(build *wizard.Builder) {
				inp = Input{
					IsModExpBase:     inpCt.GetCommit(build, "IS_MODEXP_BASE"),
					IsModExpExponent: inpCt.GetCommit(build, "IS_MODEXP_EXPONENT"),
					IsModExpModulus:  inpCt.GetCommit(build, "IS_MODEXP_MODULUS"),
					IsModExpResult:   inpCt.GetCommit(build, "IS_MODEXP_RESULT"),
					Limbs:            inpCt.GetCommit(build, "LIMBS"),
					Settings:         Settings{MaxNbInstance256: tc.NbSmallInstances, MaxNbInstanceLarge: tc.NbLargeInstances, NbInstancesPerCircuitModexp256: 1, NbInstancesPerCircuitModexpLarge: 1},
				}

				mod = newModule(build.CompiledIOP, inp).
					WithCircuit(build.CompiledIOP, query.PlonkRangeCheckOption(21, 4, false))
			}, dummy.Compile)

			proof := wizard.Prove(cmp, func(run *wizard.ProverRuntime) {

				inpCt.Assign(run,
					"LIMBS",
					"IS_MODEXP_BASE",
					"IS_MODEXP_EXPONENT",
					"IS_MODEXP_MODULUS",
					"IS_MODEXP_RESULT",
				)

				mod.Assign(run)
			})

			if err := wizard.Verify(cmp, proof); err != nil {
				t.Fatal("proof failed", err)
			}

			t.Log("proof succeeded")
		})
	}

}
