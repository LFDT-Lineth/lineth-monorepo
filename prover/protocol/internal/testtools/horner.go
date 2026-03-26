package testtools

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/protocol/accessors"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
)

// HornerTestcase specifies a protocol with a Horner query defined in it
// and the assignment of its columns (which may be valid or not).
type HornerTestcase struct {

	// NameStr is the name of the testcase
	NameStr string

	// SignNegativeParts indicates that parts #i has a reversed sign
	SignNegativeParts []bool

	// Coefficients are the coefficients for each parts of the query
	Coefficients [][]smartvectors.SmartVector

	// Selectors are the selectors for each parts of the query
	Selectors [][]smartvectors.SmartVector

	// N0s are the N0 values for each parts of the query
	N0s []int

	// N1s are the N1 values for each parts of the query
	N1s []int

	// Xs are the X values for each parts of the query
	Xs []fext.Element

	// FinalResult is the expected final result of the query
	FinalResult fext.Element

	// Query is the Horner query
	Query query.Horner

	// MustFailFlag indicates that the testcase must fail
	MustFailFlag bool
}

var ListOfHornerTestcasePositive = []*HornerTestcase{

	{
		NameStr:           "positive/none-selected-single",
		SignNegativeParts: []bool{false},
		Coefficients: [][]smartvectors.SmartVector{{
			smartvectors.NewConstant(field.One(), 8),
		}},
		Selectors: [][]smartvectors.SmartVector{{
			smartvectors.NewConstant(field.Zero(), 8),
		}},
		N0s:         []int{0},
		N1s:         []int{0},
		Xs:          []fext.Element{fext.One()},
		FinalResult: fext.Zero(),
	},

	// {
	// 	NameStr:           "positive/just-counting",
	// 	SignNegativeParts: []bool{false},
	// 	Coefficients: [][]smartvectors.SmartVector{{
	// 		smartvectors.NewConstant(field.One(), 8),
	// 	}},
	// 	Selectors: [][]smartvectors.SmartVector{{
	// 		smartvectors.NewConstant(field.One(), 8),
	// 	}},
	// 	N0s:         []int{0},
	// 	N1s:         []int{8},
	// 	Xs:          []fext.Element{fext.NewFromUint(2, 0, 0, 0)},
	// 	FinalResult: fext.NewFromUint(255, 0, 0, 0),
	// },

	// {
	// 	NameStr:           "positive/two-parts-cancelling",
	// 	SignNegativeParts: []bool{false, true},
	// 	Coefficients: [][]smartvectors.SmartVector{
	// 		{
	// 			RandomFromSeed(8, 1),
	// 		},
	// 		{
	// 			RandomFromSeed(8, 1),
	// 		},
	// 	},
	// 	Selectors: [][]smartvectors.SmartVector{
	// 		{
	// 			smartvectors.NewConstant(field.One(), 8),
	// 		},
	// 		{
	// 			smartvectors.NewConstant(field.One(), 8),
	// 		},
	// 	},
	// 	N0s:         []int{0, 0},
	// 	N1s:         []int{8, 8},
	// 	Xs:          []fext.Element{fext.One(), fext.One()},
	// 	FinalResult: fext.Zero(),
	// },

	// {
	// 	NameStr:           "positive/just-counting",
	// 	SignNegativeParts: []bool{false},
	// 	Coefficients: [][]smartvectors.SmartVector{{
	// 		smartvectors.NewConstant(field.One(), 8),
	// 	}},
	// 	Selectors: [][]smartvectors.SmartVector{{
	// 		smartvectors.NewConstant(field.One(), 8),
	// 	}},
	// 	N0s:         []int{0},
	// 	N1s:         []int{8},
	// 	Xs:          []fext.Element{fext.One()},
	// 	FinalResult: fext.NewFromUint(8, 0, 0, 0),
	// },

	// {
	// 	NameStr:           "positive/just-counting",
	// 	SignNegativeParts: []bool{false},
	// 	Coefficients: [][]smartvectors.SmartVector{{
	// 		smartvectors.NewConstant(field.One(), 8),
	// 	}},
	// 	Selectors: [][]smartvectors.SmartVector{{
	// 		smartvectors.NewConstant(field.One(), 8),
	// 	}},
	// 	N0s:         []int{0},
	// 	N1s:         []int{8},
	// 	Xs:          []fext.Element{fext.NewFromUint(2, 0, 0, 0)},
	// 	FinalResult: fext.NewFromUint(255, 0, 0, 0),
	// },

	// {
	// 	NameStr:           "positive/12345..7",
	// 	SignNegativeParts: []bool{false},
	// 	Coefficients: [][]smartvectors.SmartVector{{
	// 		smartvectors.ForTest(0, 1, 2, 3, 4, 5, 6, 7),
	// 	}},
	// 	Selectors: [][]smartvectors.SmartVector{{
	// 		smartvectors.NewConstant(field.One(), 8),
	// 	}},
	// 	N0s:         []int{0},
	// 	N1s:         []int{8},
	// 	Xs:          []fext.Element{fext.NewFromUint(2, 0, 0, 0)},
	// 	FinalResult: fext.NewFromUint(1538, 0, 0, 0),
	// },

	// {
	// 	NameStr:           "positive/multi-ary",
	// 	SignNegativeParts: []bool{false},
	// 	Coefficients: [][]smartvectors.SmartVector{
	// 		{
	// 			smartvectors.ForTest(1, 3, 5, 7, 9, 11, 13, 15),
	// 			smartvectors.ForTest(0, 2, 4, 6, 8, 10, 12, 14),
	// 		},
	// 	},
	// 	Selectors: [][]smartvectors.SmartVector{
	// 		{
	// 			smartvectors.NewConstant(field.One(), 8),
	// 			smartvectors.NewConstant(field.One(), 8),
	// 		},
	// 	},
	// 	N0s:         []int{0},
	// 	N1s:         []int{16},
	// 	Xs:          []fext.Element{fext.NewFromUint(2, 0, 0, 0)},
	// 	FinalResult: fext.NewFromUint(917506, 0, 0, 0),
	// },
}

var ListOfHornerTestcaseNegative = []*HornerTestcase{

	{
		NameStr:           "negative/none-selected-single/bad-count",
		SignNegativeParts: []bool{false},
		Coefficients: [][]smartvectors.SmartVector{{
			smartvectors.NewConstant(field.One(), 8),
		}},
		Selectors: [][]smartvectors.SmartVector{{
			smartvectors.NewConstant(field.Zero(), 8),
		}},
		N0s:          []int{0},
		N1s:          []int{1},
		Xs:           []fext.Element{fext.One()},
		FinalResult:  fext.Zero(),
		MustFailFlag: true,
	},

	{
		NameStr:           "negative/none-selected-single/bad-result",
		SignNegativeParts: []bool{false},
		Coefficients: [][]smartvectors.SmartVector{{
			smartvectors.NewConstant(field.One(), 8),
		}},
		Selectors: [][]smartvectors.SmartVector{{
			smartvectors.NewConstant(field.Zero(), 8),
		}},
		N0s:          []int{0},
		N1s:          []int{0},
		Xs:           []fext.Element{fext.One()},
		FinalResult:  fext.One(),
		MustFailFlag: true,
	},

	{
		NameStr:           "negative/two-parts-should-be-expected-to-cancel",
		SignNegativeParts: []bool{false, true},
		Coefficients: [][]smartvectors.SmartVector{{
			RandomFromSeed(8, 1),
			RandomFromSeed(8, 1),
		}},
		Selectors: [][]smartvectors.SmartVector{{
			smartvectors.NewConstant(field.One(), 8),
			smartvectors.NewConstant(field.One(), 8),
		}},
		N0s:          []int{0, 0},
		N1s:          []int{8, 8},
		Xs:           []fext.Element{fext.One(), fext.One()},
		FinalResult:  fext.One(),
		MustFailFlag: true,
	},

	{
		NameStr:           "negative/two-parts-should-be-expected-to-cancel",
		SignNegativeParts: []bool{false, true},
		Coefficients: [][]smartvectors.SmartVector{{
			RandomFromSeed(8, 1),
			RandomFromSeed(8, 1),
		}},
		Selectors: [][]smartvectors.SmartVector{{
			smartvectors.NewConstant(field.One(), 8),
			smartvectors.NewConstant(field.One(), 8),
		}},
		N0s:          []int{0, 0},
		N1s:          []int{8, 7},
		Xs:           []fext.Element{fext.One(), fext.One()},
		FinalResult:  fext.Zero(),
		MustFailFlag: true,
	},

	{
		NameStr:           "negative/just-counting/bad-n0",
		SignNegativeParts: []bool{false},
		Coefficients: [][]smartvectors.SmartVector{{
			smartvectors.NewConstant(field.One(), 8),
		}},
		Selectors: [][]smartvectors.SmartVector{{
			smartvectors.NewConstant(field.One(), 8),
		}},
		N0s:          []int{1},
		N1s:          []int{8},
		Xs:           []fext.Element{fext.One()},
		FinalResult:  fext.NewFromUint(8, 0, 0, 0),
		MustFailFlag: true,
	},

	{
		NameStr:           "negative/just-counting-x=2/bad-result",
		SignNegativeParts: []bool{false},
		Coefficients: [][]smartvectors.SmartVector{{
			smartvectors.NewConstant(field.One(), 8),
		}},
		Selectors: [][]smartvectors.SmartVector{{
			smartvectors.NewConstant(field.One(), 8),
		}},
		N0s:          []int{0},
		N1s:          []int{8},
		Xs:           []fext.Element{fext.NewFromUint(2, 0, 0, 0)},
		FinalResult:  fext.NewFromUint(510, 0, 0, 0),
		MustFailFlag: true,
	},
}

func (t *HornerTestcase) Define(comp *wizard.CompiledIOP) {

	parts := make([]query.HornerPart, len(t.Coefficients))
	for i := range parts {
		parts[i] = query.HornerPart{
			Name:         fmt.Sprintf("HornerPart_%d", i),
			SignNegative: t.SignNegativeParts[i],
			Coefficients: make([]*sym.Expression, len(t.Coefficients[i])),
			Selectors:    make([]ifaces.Column, len(t.Selectors[i])),
			X:            accessors.NewConstantExt(t.Xs[i]),
		}

		for j := range parts[i].Coefficients {
			parts[i].Coefficients[j] = sym.NewVariable(comp.InsertCommit(
				0,
				formatName[ifaces.ColID]("Horner", t.NameStr, "Coefficient", i, j),
				t.Coefficients[i][j].Len(),
				smartvectors.IsBase(t.Coefficients[i][j]),
			))

			parts[i].Selectors[j] = comp.InsertCommit(
				0,
				formatName[ifaces.ColID]("Horner", t.NameStr, "Selector", i, j),
				t.Selectors[i][j].Len(),
				true,
			)
		}
	}

	t.Query = comp.InsertHornerQuery(
		0,
		formatName[ifaces.QueryID]("Horner", t.NameStr, "Query"),
		parts,
	)
}

func (t *HornerTestcase) Assign(run *wizard.ProverRuntime) {

	parts := make([]query.HornerParamsPart, len(t.Coefficients))

	for i := range parts {

		parts[i] = query.HornerParamsPart{
			N0: t.N0s[i],
			N1: t.N1s[i],
		}

		for j := range t.Coefficients[i] {

			run.AssignColumn(
				formatName[ifaces.ColID]("Horner", t.NameStr, "Coefficient", i, j),
				t.Coefficients[i][j],
			)

			run.AssignColumn(
				formatName[ifaces.ColID]("Horner", t.NameStr, "Selector", i, j),
				t.Selectors[i][j],
			)
		}
	}

	run.AssignHornerParams(
		formatName[ifaces.QueryID]("Horner", t.NameStr, "Query"),
		query.HornerParams{
			Parts:       parts,
			FinalResult: t.FinalResult,
		},
	)
}

func (t *HornerTestcase) MustFail() bool {
	return t.MustFailFlag
}

func (t *HornerTestcase) Name() string {
	return t.NameStr
}
