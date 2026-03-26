package testtools

import (
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
)

// ProjectionTestcase represents a test-case for a projection query
type ProjectionTestcase struct {
	NameStr          string
	FilterA, FilterB []smartvectors.SmartVector
	As, Bs           [][]smartvectors.SmartVector
	ShouldFail       bool
}

var ListOfProjectionTestcasePositive = []*ProjectionTestcase{

	{
		NameStr: "positive/selector-full-zeroes",
		FilterA: []smartvectors.SmartVector{smartvectors.NewConstant(field.Zero(), 16)},
		FilterB: []smartvectors.SmartVector{smartvectors.NewConstant(field.Zero(), 8)},
		As: [][]smartvectors.SmartVector{{
			smartvectors.PseudoRand(rng, 16),
		}},
		Bs: [][]smartvectors.SmartVector{{
			smartvectors.PseudoRand(rng, 8),
		}},
	},

	{
		NameStr: "positive/counting-values",
		FilterA: []smartvectors.SmartVector{OnesAt(16, []int{2, 4, 6, 8, 10})},
		FilterB: []smartvectors.SmartVector{OnesAt(8, []int{1, 2, 3, 4, 5})},
		As: [][]smartvectors.SmartVector{{
			CountingAt(16, 0, []int{2, 4, 6, 8, 10}),
		}},
		Bs: [][]smartvectors.SmartVector{{
			CountingAt(8, 0, []int{1, 2, 3, 4, 5}),
		}},
	},
	{
		NameStr: "positive/selector-full-zeroes-multicolumn",
		FilterA: []smartvectors.SmartVector{smartvectors.NewConstant(field.Zero(), 16)},
		FilterB: []smartvectors.SmartVector{smartvectors.NewConstant(field.Zero(), 8)},
		As: [][]smartvectors.SmartVector{{
			smartvectors.PseudoRand(rng, 16),
			smartvectors.PseudoRand(rng, 16),
		}},
		Bs: [][]smartvectors.SmartVector{{
			smartvectors.PseudoRand(rng, 8),
			smartvectors.PseudoRand(rng, 8),
		}},
	},

	{
		NameStr: "positive/counting-values-multicolumn",
		FilterA: []smartvectors.SmartVector{OnesAt(16, []int{2, 4, 6, 8, 10})},
		FilterB: []smartvectors.SmartVector{OnesAt(8, []int{1, 2, 3, 4, 5})},
		As: [][]smartvectors.SmartVector{{
			CountingAt(16, 0, []int{2, 4, 6, 8, 10}),
			CountingAt(16, 5, []int{2, 4, 6, 8, 10}),
		}},
		Bs: [][]smartvectors.SmartVector{{
			CountingAt(8, 0, []int{1, 2, 3, 4, 5}),
			CountingAt(8, 5, []int{1, 2, 3, 4, 5}),
		}},
	},

	{
		NameStr: "positive/spaghettification",
		FilterA: []smartvectors.SmartVector{smartvectors.NewConstant(field.One(), 16)},
		FilterB: []smartvectors.SmartVector{
			smartvectors.NewConstant(field.One(), 8),
			smartvectors.NewConstant(field.One(), 8),
		},
		As: [][]smartvectors.SmartVector{{
			smartvectors.ForTest(1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16),
		}},
		Bs: [][]smartvectors.SmartVector{
			{
				smartvectors.ForTest(1, 3, 5, 7, 9, 11, 13, 15),
			},
			{
				smartvectors.ForTest(2, 4, 6, 8, 10, 12, 14, 16),
			},
		},
	},

	{
		NameStr: "positive/spaghettification-with-selectors",
		FilterA: []smartvectors.SmartVector{
			smartvectors.NewConstant(field.One(), 8),
		},
		FilterB: []smartvectors.SmartVector{
			smartvectors.ForTest(1, 1, 1, 1, 0, 0, 0, 0),
			smartvectors.ForTest(1, 1, 1, 1, 0, 0, 0, 0),
		},
		As: [][]smartvectors.SmartVector{{
			smartvectors.ForTest(1, 2, 3, 4, 5, 6, 7, 8),
		}},
		Bs: [][]smartvectors.SmartVector{
			{
				smartvectors.ForTest(1, 3, 5, 7, -1, -1, -1, -1),
			},
			{
				smartvectors.ForTest(2, 4, 6, 8, -1, -1, -1, -1),
			},
		},
	},
}

var ListOfProjectionTestcaseNegative = []*ProjectionTestcase{

	{
		NameStr: "negative/full-random-with-full-ones-selectors",
		FilterA: []smartvectors.SmartVector{smartvectors.NewConstant(field.One(), 16)},
		FilterB: []smartvectors.SmartVector{smartvectors.NewConstant(field.One(), 8)},
		As: [][]smartvectors.SmartVector{{
			smartvectors.PseudoRand(rng, 16),
		}},
		Bs: [][]smartvectors.SmartVector{{
			smartvectors.PseudoRand(rng, 8),
		}},
		ShouldFail: true,
	},

	{
		NameStr: "negative/counting-too-many",
		FilterA: []smartvectors.SmartVector{OnesAt(16, []int{2, 4, 6, 8, 10})},
		FilterB: []smartvectors.SmartVector{OnesAt(8, []int{1, 2, 3, 4, 5, 6})},
		As: [][]smartvectors.SmartVector{{
			CountingAt(16, 0, []int{2, 4, 6, 8, 10}),
		}},
		Bs: [][]smartvectors.SmartVector{{
			CountingAt(8, 0, []int{1, 2, 3, 4, 5, 6}),
		}},
		ShouldFail: true,
	},

	{
		NameStr: "negative/counting-misaligned",
		FilterA: []smartvectors.SmartVector{OnesAt(16, []int{2, 4, 6, 8, 10})},
		FilterB: []smartvectors.SmartVector{OnesAt(8, []int{1, 2, 3, 4, 5})},
		As: [][]smartvectors.SmartVector{{
			CountingAt(16, 0, []int{2, 4, 6, 8, 10}),
		}},
		Bs: [][]smartvectors.SmartVector{{
			CountingAt(8, 0, []int{1, 2, 3, 4, 6}),
		}},
		ShouldFail: true,
	},
}

// Define returns a [wizard.DefineFunc] constructing
// columns and a [query.Projection] as specified by the testcase.
func (tc *ProjectionTestcase) Define(comp *wizard.CompiledIOP) {

	inp := query.ProjectionMultiAryInput{}

	for i := range tc.FilterA {

		inp.FiltersA = append(inp.FiltersA, comp.InsertCommit(
			0,
			formatName[ifaces.ColID]("Projection", tc.Name, "filterA", i),
			tc.FilterA[i].Len(),
			true,
		))

		columnsA := make([]ifaces.Column, len(tc.As[i]))
		for j := range columnsA {
			columnsA[j] = comp.InsertCommit(
				0,
				formatName[ifaces.ColID]("Projection", tc.Name, "A", i, j),
				tc.As[i][j].Len(),
				smartvectors.IsBase(tc.As[i][j]),
			)
		}

		inp.ColumnsA = append(inp.ColumnsA, columnsA)
	}

	for i := range tc.FilterB {

		inp.FiltersB = append(inp.FiltersB, comp.InsertCommit(
			0,
			formatName[ifaces.ColID]("Projection", tc.Name, "filterB", i),
			tc.FilterB[i].Len(),
			true,
		))

		columnsB := make([]ifaces.Column, len(tc.Bs[i]))
		for j := range columnsB {
			columnsB[j] = comp.InsertCommit(
				0,
				formatName[ifaces.ColID]("Projection", tc.Name, "B", i, j),
				tc.Bs[i][j].Len(),
				smartvectors.IsBase(tc.Bs[i][j]),
			)
		}

		inp.ColumnsB = append(inp.ColumnsB, columnsB)
	}

	comp.InsertProjection(
		formatName[ifaces.QueryID]("Projection", tc.Name, "Query"),
		inp,
	)
}

// proverTestcaseWithProjection returns a prover function assigning the
// columns taking place in the [query.Projection] query.
func (tc *ProjectionTestcase) Assign(run *wizard.ProverRuntime) {

	for i := range tc.FilterA {

		run.AssignColumn(
			formatName[ifaces.ColID]("Projection", tc.Name, "filterA", i),
			tc.FilterA[i],
		)

		for j := range tc.As[i] {
			run.AssignColumn(
				formatName[ifaces.ColID]("Projection", tc.Name, "A", i, j),
				tc.As[i][j],
			)
		}
	}

	for i := range tc.FilterB {

		run.AssignColumn(
			formatName[ifaces.ColID]("Projection", tc.Name, "filterB", i),
			tc.FilterB[i],
		)

		for j := range tc.Bs[i] {
			run.AssignColumn(
				formatName[ifaces.ColID]("Projection", tc.Name, "B", i, j),
				tc.Bs[i][j],
			)
		}
	}
}

func (tc *ProjectionTestcase) MustFail() bool {
	return tc.ShouldFail
}

func (tc *ProjectionTestcase) Name() string {
	return tc.NameStr
}
