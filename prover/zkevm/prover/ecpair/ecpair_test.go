package ecpair

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils/csvtraces"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
)

type pairingDataTestCase struct {
	InputFName, ModuleFName                      string
	NbMillerLoops, NbFinalExps, NbSubgroupChecks int
}

var pairingDataTestCases = []pairingDataTestCase{
	{
		InputFName:    "testdata/ecpair_double_pair_input.csv",
		ModuleFName:   "testdata/ecpair_double_pair_module.csv",
		NbMillerLoops: 1,
		NbFinalExps:   1,
	},
	{
		InputFName:    "testdata/ecpair_two_pairings_input.csv",
		ModuleFName:   "testdata/ecpair_two_pairings_module.csv",
		NbMillerLoops: 2,
		NbFinalExps:   2,
	},
	{
		InputFName:    "testdata/ecpair_failing_double_pair_input.csv",
		ModuleFName:   "testdata/ecpair_failing_double_pair_module.csv",
		NbMillerLoops: 1,
		NbFinalExps:   1,
	},
	{
		InputFName:    "testdata/ecpair_triple_pair_input.csv",
		ModuleFName:   "testdata/ecpair_triple_pair_module.csv",
		NbMillerLoops: 2,
		NbFinalExps:   1,
	},
	{
		// test case with bigger limits than inputs. Tests that input fillers work correctly
		InputFName:    "testdata/ecpair_double_pair_input.csv",
		ModuleFName:   "testdata/ecpair_double_pair_module.csv",
		NbMillerLoops: 2,
		NbFinalExps:   2,
	},
	{
		// empty input to test edge case and input fillers
		InputFName:    "testdata/ecpair_empty.csv",
		ModuleFName:   "testdata/ecpair_empty_module.csv",
		NbMillerLoops: 2,
		NbFinalExps:   2,
	},
	{
		// trace test
		InputFName:    "testdata/ecpair_trace_input.csv",
		ModuleFName:   "testdata/ecpair_trace_module.csv",
		NbMillerLoops: 2,
		NbFinalExps:   1,
	},
	{
		// regression test in Linea Sepolia transaction 0x7afcf5eddbe09d85c8d0b1e3608b755b9baed1a59d8589ebcaf50e7603074139
		InputFName:       "testdata/ecpair_regression_1_input.csv",
		ModuleFName:      "testdata/ecpair_regression_1_module.csv",
		NbMillerLoops:    2,
		NbFinalExps:      1,
		NbSubgroupChecks: 1,
	},
}

func testModule(t *testing.T, tc pairingDataTestCase, withPairingCircuit, withG2MembershipCircuit bool, checkPairingModule, checkSubgroupModule bool) {
	t.Run(tc.InputFName, func(t *testing.T) {

		var (
			inp   *ECPairSource
			mod   *ECPair
			modCt *csvtraces.CsvTrace
			inpCt = csvtraces.MustOpenCsvFile(tc.InputFName)
		)
		if tc.ModuleFName == "" {
			checkPairingModule = false
			checkSubgroupModule = false
		}
		if checkPairingModule || checkSubgroupModule {
			modCt = csvtraces.MustOpenCsvFile(tc.ModuleFName)
		}

		limits := &Limits{
			NbMillerLoopInputInstances:   tc.NbMillerLoops,
			NbFinalExpInputInstances:     tc.NbFinalExps,
			NbG2MembershipInputInstances: tc.NbSubgroupChecks,
		}
		if tc.NbMillerLoops > 0 {
			limits.NbMillerLoopCircuits = 1
		}
		if tc.NbFinalExps > 0 {
			limits.NbFinalExpCircuits = 1
		}
		if tc.NbSubgroupChecks > 0 {
			limits.NbG2MembershipCircuits = 1
		}

		cmp := wizard.Compile(func(build *wizard.Builder) {
			inp = &ECPairSource{
				ID:                inpCt.GetCommit(build, "ECDATA_ID"),
				Index:             inpCt.GetCommit(build, "ECDATA_INDEX"),
				CsEcpairing:       inpCt.GetCommit(build, "ECDATA_CS_PAIRING"),
				CsG2Membership:    inpCt.GetCommit(build, "ECDATA_CS_G2_MEMBERSHIP"),
				IsEcPairingData:   inpCt.GetCommit(build, "ECDATA_IS_DATA"),
				IsEcPairingResult: inpCt.GetCommit(build, "ECDATA_IS_RES"),
				SuccessBit:        inpCt.GetCommit(build, "ECDATA_SUCCESS_BIT"),
				AccPairings:       inpCt.GetCommit(build, "ECDATA_ACC_PAIRINGS"),
				TotalPairings:     inpCt.GetCommit(build, "ECDATA_TOTAL_PAIRINGS"),
				Limbs:             inpCt.GetLimbsLe(build, "ECDATA_LIMB", common.NbLimbU128).AssertUint128(),
			}

			mod = newECPair(build.CompiledIOP, limits, inp)
			if withPairingCircuit {
				mod.WithPairingCircuit(build.CompiledIOP, query.PlonkRangeCheckOption(16, 1, false))
			}
			if withG2MembershipCircuit {
				mod.WithG2MembershipCircuit(build.CompiledIOP, query.PlonkRangeCheckOption(16, 1, false))
			}
		}, dummy.Compile)

		proof := wizard.Prove(cmp, func(run *wizard.ProverRuntime) {
			inpCt.Assign(run,
				inp.ID,
				inp.Index,
				inp.CsEcpairing,
				inp.CsG2Membership,
				inp.IsEcPairingData,
				inp.IsEcPairingResult,
				inp.Limbs,
				inp.SuccessBit,
				inp.AccPairings,
				inp.TotalPairings,
			)

			mod.Assign(run)

			if checkPairingModule {
				modCt.CheckAssignment(run,
					mod.IsActive,
					mod.UnalignedPairingData.IsActive,
					mod.UnalignedPairingData.Index,
					mod.UnalignedPairingData.InstanceID,
					mod.UnalignedPairingData.PairID,
					mod.UnalignedPairingData.TotalPairs,
					mod.UnalignedPairingData.IsComputed,
					mod.UnalignedPairingData.IsPulling,
					mod.UnalignedPairingData.IsFirstLineOfInstance,
					mod.UnalignedPairingData.IsAccumulatorInit,
					mod.UnalignedPairingData.IsFirstLineOfPrevAccumulator,
					mod.UnalignedPairingData.IsAccumulatorPrev,
					mod.UnalignedPairingData.IsFirstLineOfCurrAccumulator,
					mod.UnalignedPairingData.IsResultOfInstance,
					mod.UnalignedPairingData.Limbs,
					mod.UnalignedPairingData.ToMillerLoopCircuitMask,
					mod.UnalignedPairingData.ToFinalExpCircuitMask,
				)
			}
			if checkSubgroupModule {
				modCt.CheckAssignment(run,
					mod.IsActive,
					mod.UnalignedG2MembershipData.IsPulling,
					mod.UnalignedG2MembershipData.IsComputed,
					mod.UnalignedG2MembershipData.Limbs,
					mod.UnalignedG2MembershipData.ToG2MembershipCircuitMask,
					mod.UnalignedG2MembershipData.SuccessBit,
				)
			}
		})

		if err := wizard.Verify(cmp, proof); err != nil {
			t.Fatal("proof failed", err)
		}

		t.Log("proof succeeded")
	})
}

func TestPairingData(t *testing.T) {
	for _, tc := range pairingDataTestCases {
		testModule(t, tc, false, false, true, false)
	}
}

var membershipTestCases = []pairingDataTestCase{
	{
		InputFName:       "testdata/ecpair_g2_both_cases_input.csv",
		ModuleFName:      "testdata/ecpair_g2_both_cases_module.csv",
		NbSubgroupChecks: 3,
	},
	// {
	// 	// test case with bigger limits than inputs. Tests that input fillers work correctly.
	// 	InputFName:       "testdata/ecpair_g2_both_cases_input.csv",
	// 	ModuleFName:      "testdata/ecpair_g2_both_cases_module.csv",
	// 	NbSubgroupChecks: 5,
	// },
	// {
	// 	// empty input to test edge case and fillers
	// 	InputFName:       "testdata/ecpair_empty.csv",
	// 	ModuleFName:      "",
	// 	NbSubgroupChecks: 2,
	// },
}

func TestMembership(t *testing.T) {
	for _, tc := range membershipTestCases {
		testModule(t, tc, false, false, false, true)
	}
}

// this is utility function for being able to write the module output to a
// file. it is useful for testcase generation. NB! when generating testcase
// then manually check the correctness of the file before committing it.
// func writeModule(t *testing.T, _ *wizard.ProverRuntime, outFile string, _ *ECPair) {
// w, err := os.Create(outFile)
// if err != nil {
// 	t.Fatal(err)
// }
// defer w.Close()
// csvtraces.FmtCsv(w, run, []ifaces.Column{
// // module activation
// mod.IsActive,

// // source
// mod.ECPairSource.ID,
// mod.ECPairSource.Index,
// mod.ECPairSource.Limb,
// mod.ECPairSource.SuccessBit,
// mod.ECPairSource.AccPairings,
// mod.ECPairSource.TotalPairings,
// mod.ECPairSource.IsEcPairingData,
// mod.ECPairSource.IsEcPairingResult,
// mod.ECPairSource.CsEcpairing,
// mod.ECPairSource.CsG2Membership,

// // for pairing module test
// mod.UnalignedPairingData.IsActive,
// mod.UnalignedPairingData.Index,
// mod.UnalignedPairingData.InstanceID,
// mod.UnalignedPairingData.IsFirstLineOfInstance,
// mod.UnalignedPairingData.IsAccumulatorInit,
// mod.UnalignedPairingData.IsFirstLineOfPrevAccumulator,
// mod.UnalignedPairingData.IsAccumulatorPrev,
// mod.UnalignedPairingData.IsFirstLineOfCurrAccumulator,
// mod.UnalignedPairingData.IsAccumulatorCurr,
// mod.UnalignedPairingData.IsResultOfInstance,
// mod.UnalignedPairingData.IsComputed,
// mod.UnalignedPairingData.IsPulling,
// mod.UnalignedPairingData.PairID,
// mod.UnalignedPairingData.TotalPairs,
// mod.UnalignedPairingData.Limb,
// mod.UnalignedPairingData.ToMillerLoopCircuitMask,
// mod.UnalignedPairingData.ToFinalExpCircuitMask,

// // for subgroup module module test
// mod.UnalignedG2MembershipData.IsComputed,
// mod.UnalignedG2MembershipData.IsPulling,
// mod.UnalignedG2MembershipData.Limb,
// mod.UnalignedG2MembershipData.SuccessBit,
// mod.UnalignedG2MembershipData.ToG2MembershipCircuitMask,
// },
// []csvtraces.Option{csvtraces.InHex},
// )
// }
