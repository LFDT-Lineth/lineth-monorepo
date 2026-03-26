package common

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/limbs"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils/csvtraces"
	"github.com/stretchr/testify/assert"
)

type testModuleSource struct {
	CsTest ifaces.Column
	Limbs  limbs.Limbs[limbs.BigEndian]
}

func TestFlattenColumn(t *testing.T) {
	testCases := []struct {
		InputFName  string
		ModuleFName string
		NbLimbs     int
	}{
		{
			InputFName:  "testdata/flatten_column/default_input.csv",
			ModuleFName: "testdata/flatten_column/default_module.csv",
			NbLimbs:     8,
		},
		{
			InputFName:  "testdata/flatten_column/three_cols_input.csv",
			ModuleFName: "testdata/flatten_column/three_cols_module.csv",
			NbLimbs:     3,
		},
		{
			InputFName:  "testdata/flatten_column/alternating_mask_input.csv",
			ModuleFName: "testdata/flatten_column/alternating_mask_module.csv",
			NbLimbs:     8,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.InputFName, func(t *testing.T) {

			var (
				modCt         = csvtraces.MustOpenCsvFile(tc.ModuleFName)
				inpCt         = csvtraces.MustOpenCsvFile(tc.InputFName)
				inp           *testModuleSource
				flattenColumn *FlattenColumn
			)

			cmp := wizard.Compile(func(build *wizard.Builder) {
				comp := build.CompiledIOP

				inp = &testModuleSource{
					CsTest: inpCt.GetCommit(build, "CS_TEST"),
					Limbs:  inpCt.GetLimbsBe(build, "LIMB", tc.NbLimbs),
				}

				flattenColumn = NewFlattenColumn(comp, inp.Limbs, inp.CsTest)
				flattenColumn.CsFlattenProjection(comp)
			}, dummy.Compile)

			proof := wizard.Prove(cmp, func(run *wizard.ProverRuntime) {
				inpCt.Assign(run, inp.CsTest)
				inpCt.Assign(run, inp.Limbs)

				flattenColumn.Run(run)

				modCt.CheckAssignment(run,
					flattenColumn.Limbs,
					flattenColumn.Mask,
				)
			})

			assert.NoError(t, wizard.Verify(cmp, proof), "proof failed")
		})
	}
}

func TestFlattenSharedColColumn(t *testing.T) {
	testCases := []struct {
		InputFName  string
		ModuleFName string
		NbLimbs     int
	}{
		{
			InputFName:  "testdata/flatten_column/shared_limbs_input.csv",
			ModuleFName: "testdata/flatten_column/shared_limbs_module.csv",
			NbLimbs:     8,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.InputFName, func(t *testing.T) {

			modCt := csvtraces.MustOpenCsvFile(tc.ModuleFName)
			inpCt := csvtraces.MustOpenCsvFile(tc.InputFName)

			var inpA, inpB *testModuleSource
			var flattenColumnA, flattenColumnB *FlattenColumn

			cmp := wizard.Compile(func(build *wizard.Builder) {
				comp := build.CompiledIOP

				inpA = &testModuleSource{
					CsTest: inpCt.GetCommit(build, "CS_MODULE_A"),
					Limbs:  inpCt.GetLimbsBe(build, "LIMB", tc.NbLimbs),
				}

				inpB = &testModuleSource{
					CsTest: inpCt.GetCommit(build, "CS_MODULE_B"),
					Limbs:  inpA.Limbs,
				}

				flattenColumnA = NewFlattenColumn(comp, inpA.Limbs, inpA.CsTest)
				flattenColumnA.CsFlattenProjection(comp)

				flattenColumnB = NewFlattenColumn(comp, inpB.Limbs, inpB.CsTest)
				flattenColumnB.CsFlattenProjection(comp)
			}, dummy.Compile)

			proof := wizard.Prove(cmp, func(run *wizard.ProverRuntime) {

				inpCt.Assign(run, inpA.CsTest, inpA.Limbs, inpB.CsTest)

				flattenColumnA.Run(run)
				flattenColumnB.Run(run)

				modCt.CheckAssignment(run,
					flattenColumnA.Limbs,
					flattenColumnA.Mask,
					flattenColumnB.Limbs,
					flattenColumnB.Mask,
				)
			})

			assert.NoError(t, wizard.Verify(cmp, proof), "proof failed")
		})
	}
}
