package modexp

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/limbs"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils/csvtraces"
)

func TestModExpAntichamber(t *testing.T) {

	testCases := []struct {
		InputFName, ModuleFName            string
		NbSmallInstances, NbLargeInstances int
	}{
		{
			InputFName:       "testdata/single_256_bits_input.csv",
			NbSmallInstances: 10,
			NbLargeInstances: 1,
		},
		{
			InputFName:       "testdata/single_4096_bits_input.csv",
			NbSmallInstances: 1, // not used but include anyway
			NbLargeInstances: 1,
		},
		{
			InputFName:       "testdata/single_8192_bits_input.csv",
			NbSmallInstances: 1, // not used but include anyway
			NbLargeInstances: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.InputFName, func(t *testing.T) {

			var (
				inp   *Input
				inpCt = csvtraces.MustOpenCsvFile(tc.InputFName)
			)

			cmp := wizard.Compile(func(build *wizard.Builder) {
				inp = &Input{
					IsModExpBase:     inpCt.GetCommit(build, "IS_MODEXP_BASE"),
					IsModExpExponent: inpCt.GetCommit(build, "IS_MODEXP_EXPONENT"),
					IsModExpModulus:  inpCt.GetCommit(build, "IS_MODEXP_MODULUS"),
					IsModExpResult:   inpCt.GetCommit(build, "IS_MODEXP_RESULT"),
					Limbs:            inpCt.GetLimbsLe(build, "LIMBS", limbs.NbLimbU128).AssertUint128(),
					Settings:         &Settings{MaxNbInstance256: tc.NbSmallInstances, MaxNbInstanceLarge: tc.NbLargeInstances},
				}

				newModule(build.CompiledIOP, inp)
			}, dummy.Compile)
			proof := wizard.Prove(cmp, func(run *wizard.ProverRuntime) {
				inpCt.Assign(run,
					inp.Limbs,
					inp.IsModExpBase,
					inp.IsModExpExponent,
					inp.IsModExpModulus,
					inp.IsModExpResult,
				)
			})

			if err := wizard.Verify(cmp, proof); err != nil {
				t.Fatal("proof failed", err)
			}

			t.Log("proof succeeded")
		})
	}
}
