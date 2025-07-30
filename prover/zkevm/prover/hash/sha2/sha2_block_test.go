package sha2

import (
	"strconv"
	"testing"

	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils/csvtraces"
)

type testCaseFile struct {
	ModFile, InpFile string
	WithCircuit      bool
	NbBlockLimit     int
}

func TestSha2NoCircuit(t *testing.T) {

	var testCases = []testCaseFile{
		{
			InpFile:      "testdata/input.csv",
			ModFile:      "testdata/mod.csv",
			NbBlockLimit: 10,
		},
	}

	for i := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			runTestSha2(t, testCases[i])
		})
	}
}

func runTestSha2(t *testing.T, tc testCaseFile) {

	t.Logf("testcase %++v", tc)

	var (
		inp   sha2BlocksInputs
		mod   *sha2BlockModule
		inpCt = csvtraces.MustOpenCsvFile(tc.InpFile)
		modCt = csvtraces.MustOpenCsvFile(tc.ModFile)
	)

	comp := wizard.Compile(func(build *wizard.Builder) {

		inp = sha2BlocksInputs{
			Name:                 "TESTING",
			PackedUint32:         inpCt.GetCommit(build, "PACKED_DATA"),
			Selector:             inpCt.GetCommit(build, "SELECTOR"),
			IsFirstLaneOfNewHash: inpCt.GetCommit(build, "IS_FIRST_LANE_OF_NEW_HASH"),
			MaxNbBlockPerCirc:    tc.NbBlockLimit, // 1 more than in the csv
			MaxNbCircuit:         1,
		}

		mod = newSha2BlockModule(build.CompiledIOP, &inp)

		if tc.WithCircuit {
			mod.WithCircuit(build.CompiledIOP, query.PlonkRangeCheckOption(16, 6, false))
		}

	}, dummy.Compile)

	proof := wizard.Prove(comp, func(run *wizard.ProverRuntime) {

		inpCt.Assign(run,
			"PACKED_DATA",
			"SELECTOR",
			"IS_FIRST_LANE_OF_NEW_HASH",
		)

		mod.Run(run)

		modCt.CheckAssignment(run,
			"TESTING_IS_ACTIVE",
			"TESTING_IS_EFF_BLOCK",
			"TESTING_IS_EFF_FIRST_LANE_OF_NEW_HASH",
			"TESTING_IS_EFF_LAST_LANE_OF_CURR_HASH",
			"TESTING_HASH_HI",
			"TESTING_HASH_LO",
			"TESTING_LIMBS",
		)
	})

	if err := wizard.Verify(comp, proof); err != nil {
		t.Fatal("proof failed", err)
	}

	t.Log("proof succeeded")
}
