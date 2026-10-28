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
		{
			InputFName:       "testdata/zero_modulus_all_zero_input.csv",
			NbSmallInstances: 1,
			NbLargeInstances: 1,
		},
		{
			InputFName:       "testdata/zero_modulus_256_bits_input.csv",
			NbSmallInstances: 1,
			NbLargeInstances: 1,
		},
		{
			InputFName:       "testdata/zero_modulus_4096_bits_input.csv",
			NbSmallInstances: 1,
			NbLargeInstances: 1,
		},
		{
			InputFName:       "testdata/zero_modulus_mixed_input.csv",
			NbSmallInstances: 3,
			NbLargeInstances: 2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.InputFName, func(t *testing.T) {
			if err := proveModExpAntichamber(tc.InputFName, tc.NbSmallInstances, tc.NbLargeInstances); err != nil {
				t.Fatal("proof failed", err)
			}
		})
	}
}

// TestModExpZeroModulusWrongResult checks that for a zero modulus the result
// must be 0 (EIP-198), and not base^exponent. The prover must not panic, the
// verifier must reject.
func TestModExpZeroModulusWrongResult(t *testing.T) {
	if err := proveModExpAntichamber("testdata/zero_modulus_wrong_result_input.csv", 1, 1); err == nil {
		t.Fatal("proof succeeded with a non-zero result for a zero modulus")
	} else {
		t.Log("verification failed as expected:", err)
	}
}

func proveModExpAntichamber(inputFName string, nbSmall, nbLarge int) error {

	var (
		inp   *Input
		inpCt = csvtraces.MustOpenCsvFile(inputFName)
	)

	cmp := wizard.Compile(func(build *wizard.Builder) {
		inp = &Input{
			IsModExpBase:     inpCt.GetCommit(build, "IS_MODEXP_BASE"),
			IsModExpExponent: inpCt.GetCommit(build, "IS_MODEXP_EXPONENT"),
			IsModExpModulus:  inpCt.GetCommit(build, "IS_MODEXP_MODULUS"),
			IsModExpResult:   inpCt.GetCommit(build, "IS_MODEXP_RESULT"),
			Limbs:            inpCt.GetLimbsLe(build, "LIMBS", limbs.NbLimbU128).AssertUint128(),
			Settings:         &Settings{MaxNbInstance256: nbSmall, MaxNbInstanceLarge: nbLarge},
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

	return wizard.Verify(cmp, proof)
}
