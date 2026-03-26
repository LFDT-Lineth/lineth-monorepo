package spaghettifier

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils/csvtraces"
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
			inp.Filter[0],
			inp.Filter[1],
			inp.Filter[2],
			inp.ContentMatrix[0][0],
			inp.ContentMatrix[0][1],
			inp.ContentMatrix[0][2],
			inp.ContentMatrix[1][0],
			inp.ContentMatrix[1][1],
			inp.ContentMatrix[1][2],
			inp.ContentMatrix[2][0],
			inp.ContentMatrix[2][1],
			inp.ContentMatrix[2][2],
		)

		mod.Run(run)

		inpCt.CheckAssignment(run,
			mod.Tags[0],
			mod.Tags[1],
			mod.Tags[2],
		)

		modCt.CheckAssignment(run,
			mod.ContentSpaghetti[0],
			mod.ContentSpaghetti[1],
			mod.ContentSpaghetti[2],
			mod.FilterSpaghetti,
			mod.TagSpaghetti,
		)
	})

	if err := wizard.Verify(comp, proof); err != nil {
		t.Fatal("proof failed", err)
	}

	t.Log("proof succeeded")
}
