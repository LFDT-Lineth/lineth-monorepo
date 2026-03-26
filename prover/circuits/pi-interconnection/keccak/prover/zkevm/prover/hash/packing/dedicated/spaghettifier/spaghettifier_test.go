package spaghettifier

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/csvtraces"
)

func TestSpaghettify(t *testing.T) {

	var (
		inp   SpaghettificationInput
		mod   *Spaghettification
		inpCt = csvtraces.MustOpenCsvFile("testdata/input.csv")
		modCt = csvtraces.MustOpenCsvFile("testdata/mod.csv")
	)

	comp := wizard.Compile(func(build *wizard.Builder) {

		inp = SpaghettificationInput{
			Name:          "TESTING",
			SpaghettiSize: 16,
			Filter: []ifaces.Column{
				inpCt.GetCommit(build, "FILTER_A"),
				inpCt.GetCommit(build, "FILTER_B"),
				inpCt.GetCommit(build, "FILTER_C"),
			},
			ContentMatrix: [][]ifaces.Column{
				{
					inpCt.GetCommit(build, "A_0"),
					inpCt.GetCommit(build, "B_0"),
					inpCt.GetCommit(build, "C_0"),
				},
				{
					inpCt.GetCommit(build, "A_1"),
					inpCt.GetCommit(build, "B_1"),
					inpCt.GetCommit(build, "C_1"),
				},
				{
					inpCt.GetCommit(build, "A_2"),
					inpCt.GetCommit(build, "B_2"),
					inpCt.GetCommit(build, "C_2"),
				},
			},
		}

		mod = Spaghettify(build.CompiledIOP, inp)
	})

	t.Logf("the registerered columns are : %v", comp.ListCommitments())

	proof := wizard.Prove(comp, func(run *wizard.ProverRuntime) {

		inpCt.Assign(run,
			"FILTER_A",
			"FILTER_B",
			"FILTER_C",
			"A_0",
			"A_1",
			"A_2",
			"B_0",
			"B_1",
			"B_2",
			"C_0",
			"C_1",
			"C_2",
		)

		mod.Run(run)

		inpCt.CheckAssignment(run,
			"TESTING_TAGS_0",
			"TESTING_TAGS_1",
			"TESTING_TAGS_2",
		)

		modCt.CheckAssignment(run,
			"TESTING_CONTENT_SPAGHETTI_0",
			"TESTING_CONTENT_SPAGHETTI_1",
			"TESTING_CONTENT_SPAGHETTI_2",
			"TESTING_FILTERS_SPAGHETTI",
			"TESTING_TAGS_SPAGHETTI",
		)
	})

	if err := wizard.Verify(comp, proof); err != nil {
		t.Fatal("proof failed", err)
	}

	t.Log("proof succeeded")
}
